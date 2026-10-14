package model

import (
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleCEO           Role = "CEO"
	RoleBranchManager Role = "BRANCH_MANAGER"
	RoleSupplyManager Role = "SUPPLY_MANAGER"
)

func (r Role) Valid() bool {
	switch r {
	case RoleCEO, RoleBranchManager, RoleSupplyManager:
		return true
	}
	return false
}

type User struct {
	ID           uuid.UUID
	Name         string
	Email        string
	PasswordHash string
	Role         Role
	BranchID     *uuid.UUID

	IsActive           bool
	MustChangePassword bool

	FailedLoginAttempts int
	LockoutCount        int
	LockedUntil         *time.Time
	LastLoginAt         *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}

func (u *User) IsLocked(now time.Time) bool {
	return u.LockedUntil != nil && now.Before(*u.LockedUntil)
}

type RefreshToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash string
	ExpiresAt time.Time
	RevokedAt *time.Time
	CreatedAt time.Time
}

// Claims is the identity extracted from a verified access token.
type Claims struct {
	UserID             uuid.UUID
	Role               Role
	BranchID           *uuid.UUID
	MustChangePassword bool
}

// Session is the result of a successful login / refresh / password change.
type Session struct {
	AccessToken  string
	ExpiresAt    time.Time
	RefreshToken string
	User         *User
}
