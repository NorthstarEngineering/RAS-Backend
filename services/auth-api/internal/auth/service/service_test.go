package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	autherrors "restaurant-backend/internal/auth/errors"
	"restaurant-backend/internal/auth/model"
	"restaurant-backend/internal/auth/security"
)

// ---- fakes -----------------------------------------------------------

type fakeClock struct{ now time.Time }

func (f *fakeClock) Now() time.Time          { return f.now }
func (f *fakeClock) Advance(d time.Duration) { f.now = f.now.Add(d) }

type plainHasher struct{}

func (plainHasher) Hash(p string) (string, error) { return "h:" + p, nil }
func (plainHasher) Compare(h, p string) bool      { return h == "h:"+p }

type memUsers struct{ byID map[uuid.UUID]*model.User }

func newMemUsers() *memUsers { return &memUsers{byID: map[uuid.UUID]*model.User{}} }

func (m *memUsers) Create(_ context.Context, u *model.User) error {
	for _, e := range m.byID {
		if e.Email == u.Email {
			return autherrors.ErrEmailTaken
		}
	}
	cp := *u
	m.byID[u.ID] = &cp
	return nil
}
func (m *memUsers) GetByID(_ context.Context, id uuid.UUID) (*model.User, error) {
	u, ok := m.byID[id]
	if !ok {
		return nil, autherrors.ErrUserNotFound
	}
	cp := *u
	return &cp, nil
}
func (m *memUsers) GetByEmail(_ context.Context, email string) (*model.User, error) {
	for _, u := range m.byID {
		if u.Email == email {
			cp := *u
			return &cp, nil
		}
	}
	return nil, autherrors.ErrUserNotFound
}
func (m *memUsers) List(context.Context) ([]*model.User, error) { return nil, nil }
func (m *memUsers) ExistsByRole(_ context.Context, r model.Role) (bool, error) {
	for _, u := range m.byID {
		if u.Role == r {
			return true, nil
		}
	}
	return false, nil
}
func (m *memUsers) UpdatePassword(_ context.Context, id uuid.UUID, hash string, mustChange bool, _ time.Time) error {
	u := m.byID[id]
	u.PasswordHash, u.MustChangePassword = hash, mustChange
	u.FailedLoginAttempts, u.LockoutCount, u.LockedUntil = 0, 0, nil
	return nil
}
func (m *memUsers) UpdateEmail(_ context.Context, id uuid.UUID, email string, _ time.Time) error {
	m.byID[id].Email = email
	return nil
}
func (m *memUsers) SetActive(_ context.Context, id uuid.UUID, active bool, _ time.Time) error {
	m.byID[id].IsActive = active
	return nil
}
func (m *memUsers) RegisterFailedAttempt(_ context.Context, id uuid.UUID, _ time.Time) (int, int, error) {
	u := m.byID[id]
	u.FailedLoginAttempts++
	return u.FailedLoginAttempts, u.LockoutCount, nil
}
func (m *memUsers) Lock(_ context.Context, id uuid.UUID, until, _ time.Time) error {
	u := m.byID[id]
	u.LockedUntil, u.FailedLoginAttempts = &until, 0
	u.LockoutCount++
	return nil
}
func (m *memUsers) RegisterSuccessfulLogin(_ context.Context, id uuid.UUID, now time.Time) error {
	u := m.byID[id]
	u.FailedLoginAttempts, u.LockoutCount, u.LockedUntil, u.LastLoginAt = 0, 0, nil, &now
	return nil
}
func (m *memUsers) Unlock(_ context.Context, id uuid.UUID, _ time.Time) error {
	u := m.byID[id]
	u.FailedLoginAttempts, u.LockoutCount, u.LockedUntil = 0, 0, nil
	return nil
}

type memRefresh struct{ byHash map[string]*model.RefreshToken }

func (m *memRefresh) Create(_ context.Context, t *model.RefreshToken) error {
	cp := *t
	m.byHash[t.TokenHash] = &cp
	return nil
}
func (m *memRefresh) GetByHash(_ context.Context, h string) (*model.RefreshToken, error) {
	t, ok := m.byHash[h]
	if !ok {
		return nil, autherrors.ErrInvalidToken
	}
	cp := *t
	return &cp, nil
}
func (m *memRefresh) Revoke(_ context.Context, id uuid.UUID, now time.Time) (bool, error) {
	for _, t := range m.byHash {
		if t.ID == id && t.RevokedAt == nil {
			t.RevokedAt = &now
			return true, nil
		}
	}
	return false, nil
}
func (m *memRefresh) RevokeAllForUser(_ context.Context, uid uuid.UUID, now time.Time) error {
	for _, t := range m.byHash {
		if t.UserID == uid && t.RevokedAt == nil {
			t.RevokedAt = &now
		}
	}
	return nil
}

type allBranches struct{}

func (allBranches) BranchExists(context.Context, uuid.UUID) (bool, error) { return true, nil }

// ---- helpers ---------------------------------------------------------

const testPassword = "Correct1234"

func newTestService(t *testing.T) (*Service, *fakeClock, *model.User) {
	t.Helper()
	clock := &fakeClock{now: time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)}
	secret := []byte("0123456789abcdef0123456789abcdef")
	users := newMemUsers()
	svc := New(Dependencies{
		Users:         users,
		RefreshTokens: &memRefresh{byHash: map[string]*model.RefreshToken{}},
		Branches:      allBranches{},
		Hasher:        plainHasher{},
		Tokens:        security.NewJWTManager(secret, "test", 15*time.Minute),
		Clock:         clock,
	}, Settings{
		RefreshTokenTTL:   time.Hour,
		MaxFailedAttempts: 5,
		BaseLockout:       15 * time.Minute,
		MaxLockout:        time.Hour,
	})

	u := &model.User{
		ID: uuid.New(), Name: "CEO", Email: "ceo@example.com", PasswordHash: "h:" + testPassword,
		Role: model.RoleCEO, IsActive: true, CreatedAt: clock.now, UpdatedAt: clock.now,
	}
	if err := users.Create(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	return svc, clock, u
}

func isLocked(err error) (*autherrors.LockedError, bool) {
	var l *autherrors.LockedError
	return l, errors.As(err, &l)
}

// ---- tests -----------------------------------------------------------

func TestLoginSuccess(t *testing.T) {
	svc, _, _ := newTestService(t)
	s, err := svc.Login(context.Background(), " CEO@Example.com ", testPassword)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	if s.AccessToken == "" || s.RefreshToken == "" {
		t.Fatal("expected both tokens")
	}
	claims, err := svc.VerifyAccessToken(s.AccessToken)
	if err != nil || claims.Role != model.RoleCEO {
		t.Fatalf("bad claims: %+v, %v", claims, err)
	}
}

func TestUnknownEmailAndWrongPasswordLookTheSame(t *testing.T) {
	svc, _, _ := newTestService(t)
	_, e1 := svc.Login(context.Background(), "nobody@example.com", "whatever12345")
	_, e2 := svc.Login(context.Background(), "ceo@example.com", "wrong-password1")
	if !errors.Is(e1, autherrors.ErrInvalidCredentials) || !errors.Is(e2, autherrors.ErrInvalidCredentials) {
		t.Fatalf("expected identical INVALID_CREDENTIALS, got %v / %v", e1, e2)
	}
}

func TestAccountLocksAfterMaxFailuresAndProgressivelyLonger(t *testing.T) {
	svc, clock, _ := newTestService(t)
	ctx := context.Background()

	for i := 1; i <= 4; i++ {
		if _, err := svc.Login(ctx, "ceo@example.com", "bad"); !errors.Is(err, autherrors.ErrInvalidCredentials) {
			t.Fatalf("attempt %d: want invalid credentials, got %v", i, err)
		}
	}
	_, err := svc.Login(ctx, "ceo@example.com", "bad")
	l, ok := isLocked(err)
	if !ok || l.RetryAfter != 15*time.Minute {
		t.Fatalf("5th failure should lock for 15m, got %v", err)
	}

	// Correct password is rejected while locked.
	if _, err := svc.Login(ctx, "ceo@example.com", testPassword); !func() bool { _, ok := isLocked(err); return ok }() {
		t.Fatalf("locked account must not authenticate, got %v", err)
	}

	// After the lock expires, a second round of failures locks for 30m.
	clock.Advance(16 * time.Minute)
	for i := 1; i <= 4; i++ {
		_, _ = svc.Login(ctx, "ceo@example.com", "bad")
	}
	_, err = svc.Login(ctx, "ceo@example.com", "bad")
	l, ok = isLocked(err)
	if !ok || l.RetryAfter != 30*time.Minute {
		t.Fatalf("second lock should be 30m, got %v", err)
	}
}

func TestSuccessfulLoginResetsCounters(t *testing.T) {
	svc, _, u := newTestService(t)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		_, _ = svc.Login(ctx, "ceo@example.com", "bad")
	}
	if _, err := svc.Login(ctx, "ceo@example.com", testPassword); err != nil {
		t.Fatal(err)
	}
	got, _ := svc.Users.GetByID(ctx, u.ID)
	if got.FailedLoginAttempts != 0 || got.LockoutCount != 0 {
		t.Fatalf("counters not reset: %+v", got)
	}
}

func TestRefreshRotationAndReuseDetection(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()

	first, err := svc.Login(ctx, "ceo@example.com", testPassword)
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.Refresh(ctx, first.RefreshToken)
	if err != nil {
		t.Fatalf("refresh failed: %v", err)
	}
	// Re-using the old token is theft: it fails AND kills the new token too.
	if _, err := svc.Refresh(ctx, first.RefreshToken); !errors.Is(err, autherrors.ErrInvalidToken) {
		t.Fatalf("reuse must fail, got %v", err)
	}
	if _, err := svc.Refresh(ctx, second.RefreshToken); !errors.Is(err, autherrors.ErrInvalidToken) {
		t.Fatalf("all sessions must be revoked after reuse, got %v", err)
	}
}

func TestAccessTokenExpires(t *testing.T) {
	svc, clock, _ := newTestService(t)
	s, err := svc.Login(context.Background(), "ceo@example.com", testPassword)
	if err != nil {
		t.Fatal(err)
	}
	clock.Advance(16 * time.Minute)
	if _, err := svc.VerifyAccessToken(s.AccessToken); !errors.Is(err, autherrors.ErrInvalidToken) {
		t.Fatalf("expired token must be rejected, got %v", err)
	}
}

func TestCreateUserRules(t *testing.T) {
	svc, _, ceo := newTestService(t)
	ctx := context.Background()
	actor := &model.Claims{UserID: ceo.ID, Role: model.RoleCEO}
	branch := uuid.New()

	// Branch manager needs a branch.
	_, err := svc.CreateUser(ctx, actor, CreateUserInput{Name: "A", Email: "a@x.com", Password: "Password123", Role: model.RoleBranchManager})
	if !errors.Is(err, autherrors.ErrBranchRequired) {
		t.Fatalf("want BRANCH_REQUIRED, got %v", err)
	}
	// Supply manager must not have a branch.
	_, err = svc.CreateUser(ctx, actor, CreateUserInput{Name: "S", Email: "s@x.com", Password: "Password123", Role: model.RoleSupplyManager, BranchID: &branch})
	if !errors.Is(err, autherrors.ErrBranchNotAllowed) {
		t.Fatalf("want BRANCH_NOT_ALLOWED, got %v", err)
	}
	// CEO cannot be created through the API.
	_, err = svc.CreateUser(ctx, actor, CreateUserInput{Name: "C", Email: "c@x.com", Password: "Password123", Role: model.RoleCEO})
	if !errors.Is(err, autherrors.ErrInvalidRole) {
		t.Fatalf("want INVALID_ROLE, got %v", err)
	}
	// Weak password.
	_, err = svc.CreateUser(ctx, actor, CreateUserInput{Name: "B", Email: "b@x.com", Password: "short", Role: model.RoleSupplyManager})
	var appErr *autherrors.Error
	if !errors.As(err, &appErr) || appErr.Code != autherrors.ErrWeakPassword.Code {
		t.Fatalf("want WEAK_PASSWORD, got %v", err)
	}
	// Happy path: forced password change.
	u, err := svc.CreateUser(ctx, actor, CreateUserInput{Name: "B", Email: "B@X.com", Password: "Password123", Role: model.RoleBranchManager, BranchID: &branch})
	if err != nil || !u.MustChangePassword || u.Email != "b@x.com" {
		t.Fatalf("unexpected result: %+v, %v", u, err)
	}
	// Non-CEO is forbidden.
	bm := &model.Claims{UserID: u.ID, Role: model.RoleBranchManager, BranchID: &branch}
	if _, err := svc.CreateUser(ctx, bm, CreateUserInput{}); !errors.Is(err, autherrors.ErrForbidden) {
		t.Fatalf("want FORBIDDEN, got %v", err)
	}
}

func TestPasswordPolicy(t *testing.T) {
	for _, bad := range []string{"", "short1", "alllettersonly", "1234567890123"} {
		if ValidatePasswordPolicy(bad) == nil {
			t.Errorf("%q should be rejected", bad)
		}
	}
	if err := ValidatePasswordPolicy("GoodPass123"); err != nil {
		t.Errorf("valid password rejected: %v", err)
	}
}
