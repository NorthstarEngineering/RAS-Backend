package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	autherrors "restaurant-backend/internal/auth/errors"
	"restaurant-backend/internal/auth/interfaces"
	"restaurant-backend/internal/auth/model"
	"restaurant-backend/internal/auth/security"
)

type Dependencies struct {
	Users         interfaces.UserRepository
	RefreshTokens interfaces.RefreshTokenRepository
	Branches      interfaces.BranchChecker
	Hasher        interfaces.PasswordHasher
	Tokens        interfaces.TokenIssuer
	Clock         interfaces.Clock
}

type Service struct {
	Dependencies
	cfg Settings
	// dummyHash is compared against when the email is unknown, so that
	// "unknown email" and "wrong password" take the same time.
	dummyHash string
}

func New(deps Dependencies, cfg Settings) *Service {
	if deps.Clock == nil {
		deps.Clock = SystemClock{}
	}
	dummy, _ := deps.Hasher.Hash("timing-equalizer-Password1")
	return &Service{Dependencies: deps, cfg: cfg, dummyHash: dummy}
}

// ------------------------------------------------------------------ login

func (s *Service) Login(ctx context.Context, email, password string) (*model.Session, error) {
	now := s.Clock.Now()

	user, err := s.Users.GetByEmail(ctx, normalizeEmail(email))
	if errors.Is(err, autherrors.ErrUserNotFound) {
		s.Hasher.Compare(s.dummyHash, password)
		return nil, autherrors.ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}

	// While locked we do not even check the password, so guesses are useless.
	if user.IsLocked(now) {
		return nil, &autherrors.LockedError{RetryAfter: user.LockedUntil.Sub(now)}
	}

	if !s.Hasher.Compare(user.PasswordHash, password) {
		return nil, s.registerFailure(ctx, user, now)
	}
	if !user.IsActive {
		return nil, autherrors.ErrAccountDisabled
	}

	if err := s.Users.RegisterSuccessfulLogin(ctx, user.ID, now); err != nil {
		return nil, err
	}
	user.LastLoginAt = &now
	return s.issueSession(ctx, user, now)
}

// registerFailure records a bad password and locks the account when the
// threshold is reached. The lock duration doubles with every consecutive
// lock (15m, 30m, 1h, ... capped by MaxLockout).
func (s *Service) registerFailure(ctx context.Context, user *model.User, now time.Time) error {
	attempts, lockouts, err := s.Users.RegisterFailedAttempt(ctx, user.ID, now)
	if err != nil {
		return err
	}
	if attempts < s.cfg.MaxFailedAttempts {
		return autherrors.ErrInvalidCredentials
	}
	d := s.lockDuration(lockouts)
	if err := s.Users.Lock(ctx, user.ID, now.Add(d), now); err != nil {
		return err
	}
	return &autherrors.LockedError{RetryAfter: d}
}

func (s *Service) lockDuration(previousLocks int) time.Duration {
	d := s.cfg.BaseLockout
	for i := 0; i < previousLocks && d < s.cfg.MaxLockout; i++ {
		d *= 2
	}
	if d > s.cfg.MaxLockout {
		d = s.cfg.MaxLockout
	}
	return d
}

// ---------------------------------------------------------------- session

func (s *Service) issueSession(ctx context.Context, u *model.User, now time.Time) (*model.Session, error) {
	access, expiresAt, err := s.Tokens.Issue(u, now)
	if err != nil {
		return nil, err
	}
	raw, hash, err := security.NewRefreshToken()
	if err != nil {
		return nil, err
	}
	err = s.RefreshTokens.Create(ctx, &model.RefreshToken{
		ID:        uuid.New(),
		UserID:    u.ID,
		TokenHash: hash,
		ExpiresAt: now.Add(s.cfg.RefreshTokenTTL),
		CreatedAt: now,
	})
	if err != nil {
		return nil, err
	}
	return &model.Session{AccessToken: access, ExpiresAt: expiresAt, RefreshToken: raw, User: u}, nil
}

// Refresh rotates the refresh token. Presenting an already-used token is
// treated as theft: every session of that user is revoked.
func (s *Service) Refresh(ctx context.Context, rawToken string) (*model.Session, error) {
	now := s.Clock.Now()

	stored, err := s.RefreshTokens.GetByHash(ctx, security.HashToken(rawToken))
	if err != nil {
		return nil, err
	}
	if stored.RevokedAt != nil {
		_ = s.RefreshTokens.RevokeAllForUser(ctx, stored.UserID, now)
		return nil, autherrors.ErrInvalidToken
	}
	if !now.Before(stored.ExpiresAt) {
		return nil, autherrors.ErrInvalidToken
	}
	revoked, err := s.RefreshTokens.Revoke(ctx, stored.ID, now)
	if err != nil {
		return nil, err
	}
	if !revoked { // lost a race with another use of the same token
		_ = s.RefreshTokens.RevokeAllForUser(ctx, stored.UserID, now)
		return nil, autherrors.ErrInvalidToken
	}

	user, err := s.Users.GetByID(ctx, stored.UserID)
	if err != nil {
		return nil, autherrors.ErrInvalidToken
	}
	if !user.IsActive {
		return nil, autherrors.ErrAccountDisabled
	}
	return s.issueSession(ctx, user, now)
}

// Logout is idempotent: unknown or already revoked tokens are ignored.
func (s *Service) Logout(ctx context.Context, rawToken string) error {
	stored, err := s.RefreshTokens.GetByHash(ctx, security.HashToken(rawToken))
	if errors.Is(err, autherrors.ErrInvalidToken) {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = s.RefreshTokens.Revoke(ctx, stored.ID, s.Clock.Now())
	return err
}

func (s *Service) VerifyAccessToken(token string) (*model.Claims, error) {
	return s.Tokens.Parse(token, s.Clock.Now())
}

func (s *Service) Me(ctx context.Context, userID uuid.UUID) (*model.User, error) {
	return s.Users.GetByID(ctx, userID)
}

// ------------------------------------------------------ self-service

// verifyCurrentPassword counts wrong guesses too, so a stolen access token
// cannot be used to brute-force the password through these endpoints.
func (s *Service) verifyCurrentPassword(ctx context.Context, user *model.User, password string, now time.Time) error {
	if user.IsLocked(now) {
		return &autherrors.LockedError{RetryAfter: user.LockedUntil.Sub(now)}
	}
	if s.Hasher.Compare(user.PasswordHash, password) {
		return nil
	}
	err := s.registerFailure(ctx, user, now)
	if errors.Is(err, autherrors.ErrInvalidCredentials) {
		return autherrors.ErrWrongPassword
	}
	return err
}

func (s *Service) ChangePassword(ctx context.Context, userID uuid.UUID, current, newPassword string) (*model.Session, error) {
	now := s.Clock.Now()
	user, err := s.Users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !user.IsActive {
		return nil, autherrors.ErrAccountDisabled
	}
	if err := s.verifyCurrentPassword(ctx, user, current, now); err != nil {
		return nil, err
	}
	if err := ValidatePasswordPolicy(newPassword); err != nil {
		return nil, err
	}
	if current == newPassword {
		return nil, autherrors.ErrWeakPassword.WithMessage("New password must be different from the current one")
	}
	hash, err := s.Hasher.Hash(newPassword)
	if err != nil {
		return nil, err
	}
	if err := s.Users.UpdatePassword(ctx, user.ID, hash, false, now); err != nil {
		return nil, err
	}
	if err := s.RefreshTokens.RevokeAllForUser(ctx, user.ID, now); err != nil {
		return nil, err
	}
	user.PasswordHash = hash
	user.MustChangePassword = false
	return s.issueSession(ctx, user, now)
}

func (s *Service) ChangeEmail(ctx context.Context, userID uuid.UUID, newEmail, currentPassword string) (*model.User, error) {
	now := s.Clock.Now()
	email, err := validateEmail(newEmail)
	if err != nil {
		return nil, err
	}
	user, err := s.Users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if err := s.verifyCurrentPassword(ctx, user, currentPassword, now); err != nil {
		return nil, err
	}
	if err := s.Users.UpdateEmail(ctx, user.ID, email, now); err != nil {
		return nil, err
	}
	user.Email = email
	return user, nil
}

// ----------------------------------------------------- CEO administration

type CreateUserInput struct {
	Name     string
	Email    string
	Password string // temporary; the user must change it on first login
	Role     model.Role
	BranchID *uuid.UUID
}

func requireCEO(actor *model.Claims) error {
	if actor == nil || actor.Role != model.RoleCEO {
		return autherrors.ErrForbidden
	}
	return nil
}

func (s *Service) CreateUser(ctx context.Context, actor *model.Claims, in CreateUserInput) (*model.User, error) {
	if err := requireCEO(actor); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" || len(name) > 100 {
		return nil, autherrors.ErrValidation.WithMessage("Name is required (max 100 characters)")
	}
	email, err := validateEmail(in.Email)
	if err != nil {
		return nil, err
	}
	if err := ValidatePasswordPolicy(in.Password); err != nil {
		return nil, err
	}

	switch in.Role {
	case model.RoleBranchManager:
		if in.BranchID == nil {
			return nil, autherrors.ErrBranchRequired
		}
		ok, err := s.Branches.BranchExists(ctx, *in.BranchID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, autherrors.ErrBranchNotFound
		}
	case model.RoleSupplyManager:
		if in.BranchID != nil {
			return nil, autherrors.ErrBranchNotAllowed
		}
	default: // CEO cannot be created through the API
		return nil, autherrors.ErrInvalidRole
	}

	hash, err := s.Hasher.Hash(in.Password)
	if err != nil {
		return nil, err
	}
	now := s.Clock.Now()
	user := &model.User{
		ID:                 uuid.New(),
		Name:               name,
		Email:              email,
		PasswordHash:       hash,
		Role:               in.Role,
		BranchID:           in.BranchID,
		IsActive:           true,
		MustChangePassword: true,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := s.Users.Create(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *Service) ListUsers(ctx context.Context, actor *model.Claims) ([]*model.User, error) {
	if err := requireCEO(actor); err != nil {
		return nil, err
	}
	return s.Users.List(ctx)
}

func (s *Service) SetUserActive(ctx context.Context, actor *model.Claims, id uuid.UUID, active bool) error {
	if err := requireCEO(actor); err != nil {
		return err
	}
	target, err := s.Users.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if target.Role == model.RoleCEO && !active {
		return autherrors.ErrForbidden
	}
	now := s.Clock.Now()
	if err := s.Users.SetActive(ctx, id, active, now); err != nil {
		return err
	}
	if !active {
		return s.RefreshTokens.RevokeAllForUser(ctx, id, now)
	}
	return nil
}

func (s *Service) ResetUserPassword(ctx context.Context, actor *model.Claims, id uuid.UUID, newPassword string) error {
	if err := requireCEO(actor); err != nil {
		return err
	}
	if err := ValidatePasswordPolicy(newPassword); err != nil {
		return err
	}
	target, err := s.Users.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if target.Role == model.RoleCEO { // the CEO uses change-password
		return autherrors.ErrForbidden
	}
	hash, err := s.Hasher.Hash(newPassword)
	if err != nil {
		return err
	}
	now := s.Clock.Now()
	if err := s.Users.UpdatePassword(ctx, id, hash, true, now); err != nil {
		return err
	}
	return s.RefreshTokens.RevokeAllForUser(ctx, id, now)
}

func (s *Service) UnlockUser(ctx context.Context, actor *model.Claims, id uuid.UUID) error {
	if err := requireCEO(actor); err != nil {
		return err
	}
	return s.Users.Unlock(ctx, id, s.Clock.Now())
}

// SeedCEO creates the initial CEO account on first run. It is a no-op when a
// CEO already exists. The CEO must change this password on first login.
func (s *Service) SeedCEO(ctx context.Context, name, email, password string) error {
	exists, err := s.Users.ExistsByRole(ctx, model.RoleCEO)
	if err != nil || exists {
		return err
	}
	if email == "" || password == "" {
		return errors.New("no CEO exists: set SEED_CEO_EMAIL and SEED_CEO_PASSWORD")
	}
	email, err = validateEmail(email)
	if err != nil {
		return err
	}
	if err := ValidatePasswordPolicy(password); err != nil {
		return err
	}
	if strings.TrimSpace(name) == "" {
		name = "CEO"
	}
	hash, err := s.Hasher.Hash(password)
	if err != nil {
		return err
	}
	now := s.Clock.Now()
	return s.Users.Create(ctx, &model.User{
		ID: uuid.New(), Name: strings.TrimSpace(name), Email: email, PasswordHash: hash,
		Role: model.RoleCEO, IsActive: true, MustChangePassword: true, CreatedAt: now, UpdatedAt: now,
	})
}
