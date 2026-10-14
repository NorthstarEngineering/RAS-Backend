package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	autherrors "restaurant-backend/internal/auth/errors"
	"restaurant-backend/internal/auth/interfaces"
	"restaurant-backend/internal/auth/model"
	"restaurant-backend/internal/auth/security"
)

type qaUserRepository struct {
	interfaces.UserRepository
	getByEmailErr error
	registerErr   error
}

func (r *qaUserRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	if r.getByEmailErr != nil {
		return nil, r.getByEmailErr
	}
	return r.UserRepository.GetByEmail(ctx, email)
}

func (r *qaUserRepository) RegisterFailedAttempt(ctx context.Context, id uuid.UUID, now time.Time) (int, int, error) {
	if r.registerErr != nil {
		return 0, 0, r.registerErr
	}
	return r.UserRepository.RegisterFailedAttempt(ctx, id, now)
}

type qaBranchChecker struct {
	exists bool
	err    error
}

func (b qaBranchChecker) BranchExists(context.Context, uuid.UUID) (bool, error) {
	return b.exists, b.err
}

type qaTokenIssuer struct {
	interfaces.TokenIssuer
	issueErr error
}

func (t qaTokenIssuer) Issue(u *model.User, now time.Time) (string, time.Time, error) {
	if t.issueErr != nil {
		return "", time.Time{}, t.issueErr
	}
	return t.TokenIssuer.Issue(u, now)
}

func TestQALoginDisabledAccountAndRepositoryErrors(t *testing.T) {
	t.Run("disabled account", func(t *testing.T) {
		svc, _, user := newTestService(t)
		svc.Users.(*memUsers).byID[user.ID].IsActive = false
		if _, err := svc.Login(context.Background(), user.Email, testPassword); !errors.Is(err, autherrors.ErrAccountDisabled) {
			t.Fatalf("Login() error = %v, want ErrAccountDisabled", err)
		}
	})

	t.Run("user lookup error", func(t *testing.T) {
		svc, _, _ := newTestService(t)
		want := errors.New("user store unavailable")
		svc.Users = &qaUserRepository{UserRepository: svc.Users, getByEmailErr: want}
		if _, err := svc.Login(context.Background(), "ceo@example.com", testPassword); !errors.Is(err, want) {
			t.Fatalf("Login() error = %v, want wrapped lookup error", err)
		}
	})

	t.Run("failed attempt registration error", func(t *testing.T) {
		svc, _, _ := newTestService(t)
		want := errors.New("counter update failed")
		svc.Users = &qaUserRepository{UserRepository: svc.Users, registerErr: want}
		if _, err := svc.Login(context.Background(), "ceo@example.com", "wrong-password"); !errors.Is(err, want) {
			t.Fatalf("Login() error = %v, want wrapped counter error", err)
		}
	})
}

func TestQARefreshExpiryAndInactiveUser(t *testing.T) {
	t.Run("expired token", func(t *testing.T) {
		svc, clock, user := newTestService(t)
		store := svc.RefreshTokens.(*memRefresh)
		raw := "qa-expired-refresh-token"
		store.byHash[security.HashToken(raw)] = &model.RefreshToken{
			ID: uuid.New(), UserID: user.ID, TokenHash: security.HashToken(raw),
			ExpiresAt: clock.now,
		}
		if _, err := svc.Refresh(context.Background(), raw); !errors.Is(err, autherrors.ErrInvalidToken) {
			t.Fatalf("Refresh() error = %v, want ErrInvalidToken at expiry", err)
		}
	})

	t.Run("inactive account", func(t *testing.T) {
		svc, clock, user := newTestService(t)
		store := svc.RefreshTokens.(*memRefresh)
		raw := "qa-inactive-refresh-token"
		store.byHash[security.HashToken(raw)] = &model.RefreshToken{
			ID: uuid.New(), UserID: user.ID, TokenHash: security.HashToken(raw),
			ExpiresAt: clock.now.Add(time.Hour),
		}
		svc.Users.(*memUsers).byID[user.ID].IsActive = false
		if _, err := svc.Refresh(context.Background(), raw); !errors.Is(err, autherrors.ErrAccountDisabled) {
			t.Fatalf("Refresh() error = %v, want ErrAccountDisabled", err)
		}
	})
}

func TestQAChangePasswordRotatesAndRevokesSessions(t *testing.T) {
	svc, clock, user := newTestService(t)
	ctx := context.Background()
	oldSession, err := svc.Login(ctx, user.Email, testPassword)
	if err != nil {
		t.Fatal(err)
	}
	store := svc.RefreshTokens.(*memRefresh)
	secondRaw := "qa-second-active-session"
	secondHash := security.HashToken(secondRaw)
	if err := store.Create(ctx, &model.RefreshToken{
		ID: uuid.New(), UserID: user.ID, TokenHash: secondHash,
		ExpiresAt: clock.now.Add(time.Hour), CreatedAt: clock.now,
	}); err != nil {
		t.Fatal(err)
	}
	oldTokens := []*model.RefreshToken{
		store.byHash[security.HashToken(oldSession.RefreshToken)],
		store.byHash[secondHash],
	}

	newPassword := "NewPassword123"
	session, err := svc.ChangePassword(ctx, user.ID, testPassword, newPassword)
	if err != nil {
		t.Fatalf("ChangePassword() error = %v", err)
	}
	if session.RefreshToken == oldSession.RefreshToken || session.AccessToken == "" {
		t.Fatal("ChangePassword() did not issue a fresh session")
	}
	for i, oldToken := range oldTokens {
		if oldToken == nil || oldToken.RevokedAt == nil {
			t.Fatalf("preexisting session %d was not revoked", i+1)
		}
	}
	if _, err := svc.Refresh(ctx, oldSession.RefreshToken); !errors.Is(err, autherrors.ErrInvalidToken) {
		t.Fatalf("old refresh token error = %v, want ErrInvalidToken", err)
	}
	got, err := svc.Users.GetByID(ctx, user.ID)
	if err != nil || got.MustChangePassword || got.PasswordHash != "h:"+newPassword {
		t.Fatalf("updated user = %+v, error = %v", got, err)
	}
	if !session.ExpiresAt.Equal(clock.now.Add(15 * time.Minute)) {
		t.Fatalf("session expiry = %v, want clock-based expiry", session.ExpiresAt)
	}
}

func TestQAChangePasswordRejectsWrongCurrentPassword(t *testing.T) {
	svc, _, user := newTestService(t)
	_, err := svc.ChangePassword(context.Background(), user.ID, "wrong-password", "NewPassword123")
	if !errors.Is(err, autherrors.ErrWrongPassword) {
		t.Fatalf("ChangePassword() error = %v, want ErrWrongPassword", err)
	}
}

func TestQALockDurationRespectsConfiguredMaximum(t *testing.T) {
	svc, _, _ := newTestService(t)
	if got := svc.lockDuration(20); got != time.Hour {
		t.Fatalf("lockDuration() = %v, want configured maximum 1h", got)
	}
}

func TestQAChangeEmailNormalizesAndRejectsInvalidAddress(t *testing.T) {
	svc, _, user := newTestService(t)
	got, err := svc.ChangeEmail(context.Background(), user.ID, " New.Name@Example.COM ", testPassword)
	if err != nil {
		t.Fatalf("ChangeEmail() error = %v", err)
	}
	if got.Email != "new.name@example.com" {
		t.Fatalf("updated email = %q, want normalized address", got.Email)
	}
	if _, err := svc.ChangeEmail(context.Background(), user.ID, "not-an-email", testPassword); !errors.Is(err, autherrors.ErrInvalidEmail) {
		t.Fatalf("ChangeEmail() invalid address error = %v, want ErrInvalidEmail", err)
	}
}

func TestQALogoutIsIdempotentAndRevokesKnownToken(t *testing.T) {
	svc, clock, user := newTestService(t)
	ctx := context.Background()
	if err := svc.Logout(ctx, "unknown-token"); err != nil {
		t.Fatalf("Logout() unknown token error = %v, want nil", err)
	}

	raw := "qa-logout-token"
	store := svc.RefreshTokens.(*memRefresh)
	hash := security.HashToken(raw)
	token := &model.RefreshToken{ID: uuid.New(), UserID: user.ID, TokenHash: hash, ExpiresAt: clock.now.Add(time.Hour)}
	store.byHash[hash] = token
	if err := svc.Logout(ctx, raw); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if token.RevokedAt == nil || !token.RevokedAt.Equal(clock.now) {
		t.Fatalf("revoked_at = %v, want injected clock time %v", token.RevokedAt, clock.now)
	}
}

func TestQADeactivatingUserRevokesAllRefreshTokens(t *testing.T) {
	svc, clock, ceo := newTestService(t)
	ctx := context.Background()
	branchID := uuid.New()
	target := &model.User{
		ID: uuid.New(), Name: "Manager", Email: "manager@example.com", PasswordHash: "hash",
		Role: model.RoleBranchManager, BranchID: &branchID, IsActive: true,
	}
	if err := svc.Users.Create(ctx, target); err != nil {
		t.Fatal(err)
	}
	raw := "qa-deactivation-token"
	hash := security.HashToken(raw)
	store := svc.RefreshTokens.(*memRefresh)
	store.byHash[hash] = &model.RefreshToken{ID: uuid.New(), UserID: target.ID, TokenHash: hash, ExpiresAt: clock.now.Add(time.Hour)}

	actor := &model.Claims{UserID: ceo.ID, Role: model.RoleCEO}
	if err := svc.SetUserActive(ctx, actor, target.ID, false); err != nil {
		t.Fatalf("SetUserActive() error = %v", err)
	}
	got, err := svc.Users.GetByID(ctx, target.ID)
	if err != nil || got.IsActive {
		t.Fatalf("deactivated user = %+v, error = %v", got, err)
	}
	if store.byHash[hash].RevokedAt == nil {
		t.Fatal("deactivation left the user's refresh token active")
	}
}

func TestQACreateUserBranchLookupAndTokenIssueFailures(t *testing.T) {
	ctx := context.Background()
	svc, _, ceo := newTestService(t)
	actor := &model.Claims{UserID: ceo.ID, Role: model.RoleCEO}
	branchID := uuid.New()
	input := CreateUserInput{Name: "Manager", Email: "manager@example.com", Password: "ManagerPass123", Role: model.RoleBranchManager, BranchID: &branchID}

	svc.Branches = qaBranchChecker{}
	if _, err := svc.CreateUser(ctx, actor, input); !errors.Is(err, autherrors.ErrBranchNotFound) {
		t.Fatalf("CreateUser() error = %v, want ErrBranchNotFound", err)
	}
	wantBranchErr := errors.New("branch lookup failed")
	svc.Branches = qaBranchChecker{exists: true, err: wantBranchErr}
	if _, err := svc.CreateUser(ctx, actor, input); !errors.Is(err, wantBranchErr) {
		t.Fatalf("CreateUser() error = %v, want branch lookup error", err)
	}

	wantTokenErr := errors.New("token issue failed")
	svc.Tokens = qaTokenIssuer{TokenIssuer: svc.Tokens, issueErr: wantTokenErr}
	if _, err := svc.Login(ctx, ceo.Email, testPassword); !errors.Is(err, wantTokenErr) {
		t.Fatalf("Login() error = %v, want token issuance error", err)
	}
}

func TestQAAdministrativeServiceAuthorizationAndRestrictions(t *testing.T) {
	svc, _, ceo := newTestService(t)
	ctx := context.Background()
	bm := &model.Claims{UserID: uuid.New(), Role: model.RoleBranchManager}

	if _, err := svc.ListUsers(ctx, bm); !errors.Is(err, autherrors.ErrForbidden) {
		t.Fatalf("ListUsers() error = %v, want ErrForbidden", err)
	}
	if err := svc.SetUserActive(ctx, bm, ceo.ID, false); !errors.Is(err, autherrors.ErrForbidden) {
		t.Fatalf("SetUserActive() error = %v, want ErrForbidden", err)
	}
	if err := svc.ResetUserPassword(ctx, &model.Claims{Role: model.RoleCEO}, ceo.ID, "NewPassword123"); !errors.Is(err, autherrors.ErrForbidden) {
		t.Fatalf("ResetUserPassword() CEO target error = %v, want ErrForbidden", err)
	}
	if err := svc.UnlockUser(ctx, bm, ceo.ID); !errors.Is(err, autherrors.ErrForbidden) {
		t.Fatalf("UnlockUser() error = %v, want ErrForbidden", err)
	}
}

func TestQASeedCEOIsIdempotentAndRequiresCredentialsWhenMissing(t *testing.T) {
	svc, _, _ := newTestService(t)
	if err := svc.SeedCEO(context.Background(), "", "", ""); err != nil {
		t.Fatalf("SeedCEO() with an existing CEO error = %v, want nil", err)
	}

	users := newMemUsers()
	svc.Users = users
	if err := svc.SeedCEO(context.Background(), "", "", ""); err == nil {
		t.Fatal("SeedCEO() without an existing CEO or credentials succeeded")
	}
	if err := svc.SeedCEO(context.Background(), "", " CEO@Example.com ", "SeedPass123"); err != nil {
		t.Fatalf("SeedCEO() error = %v", err)
	}
	if len(users.byID) != 1 {
		t.Fatalf("seeded users = %d, want 1", len(users.byID))
	}
	for _, seeded := range users.byID {
		if seeded.Name != "CEO" || seeded.Email != "ceo@example.com" || !seeded.MustChangePassword {
			t.Fatalf("unexpected seeded CEO: %+v", seeded)
		}
	}
}
