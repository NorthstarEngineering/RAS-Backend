package interfaces

import (
	"context"
	"time"

	"github.com/google/uuid"

	"restaurant-backend/internal/auth/model"
)

type Clock interface {
	Now() time.Time
}

// BranchChecker is what auth needs from the branch module (wired in cmd/api).
type BranchChecker interface {
	BranchExists(ctx context.Context, id uuid.UUID) (bool, error)
}

type PasswordHasher interface {
	Hash(plain string) (string, error)
	Compare(hash, plain string) bool
}

type TokenIssuer interface {
	Issue(u *model.User, now time.Time) (token string, expiresAt time.Time, err error)
	Parse(token string, now time.Time) (*model.Claims, error)
}

type UserRepository interface {
	Create(ctx context.Context, u *model.User) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	List(ctx context.Context) ([]*model.User, error)
	ExistsByRole(ctx context.Context, role model.Role) (bool, error)

	UpdatePassword(ctx context.Context, id uuid.UUID, hash string, mustChange bool, now time.Time) error
	UpdateEmail(ctx context.Context, id uuid.UUID, email string, now time.Time) error
	SetActive(ctx context.Context, id uuid.UUID, active bool, now time.Time) error

	// Brute-force protection.
	RegisterFailedAttempt(ctx context.Context, id uuid.UUID, now time.Time) (attempts, lockoutCount int, err error)
	Lock(ctx context.Context, id uuid.UUID, until, now time.Time) error
	RegisterSuccessfulLogin(ctx context.Context, id uuid.UUID, now time.Time) error
	Unlock(ctx context.Context, id uuid.UUID, now time.Time) error
}

type RefreshTokenRepository interface {
	Create(ctx context.Context, t *model.RefreshToken) error
	GetByHash(ctx context.Context, hash string) (*model.RefreshToken, error)
	// Revoke returns true only if this call revoked an active token.
	Revoke(ctx context.Context, id uuid.UUID, now time.Time) (bool, error)
	RevokeAllForUser(ctx context.Context, userID uuid.UUID, now time.Time) error
}
