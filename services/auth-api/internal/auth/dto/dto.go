package dto

import (
	"time"

	"github.com/google/uuid"

	"restaurant-backend/internal/auth/model"
)

type LoginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required"`
}

type ChangeEmailRequest struct {
	NewEmail        string `json:"new_email" binding:"required"`
	CurrentPassword string `json:"current_password" binding:"required"`
}

type CreateUserRequest struct {
	Name     string     `json:"name" binding:"required"`
	Email    string     `json:"email" binding:"required"`
	Password string     `json:"password" binding:"required"`
	Role     model.Role `json:"role" binding:"required"`
	BranchID *uuid.UUID `json:"branch_id"`
}

type ResetPasswordRequest struct {
	NewPassword string `json:"new_password" binding:"required"`
}

type SetActiveRequest struct {
	IsActive *bool `json:"is_active" binding:"required"`
}

type UserResponse struct {
	ID                 uuid.UUID  `json:"id"`
	Name               string     `json:"name"`
	Email              string     `json:"email"`
	Role               model.Role `json:"role"`
	BranchID           *uuid.UUID `json:"branch_id"`
	IsActive           bool       `json:"is_active"`
	MustChangePassword bool       `json:"must_change_password"`
	LockedUntil        *time.Time `json:"locked_until"`
	LastLoginAt        *time.Time `json:"last_login_at"`
	CreatedAt          time.Time  `json:"created_at"`
}

type SessionResponse struct {
	AccessToken  string       `json:"access_token"`
	TokenType    string       `json:"token_type"`
	ExpiresAt    time.Time    `json:"expires_at"`
	RefreshToken string       `json:"refresh_token"`
	User         UserResponse `json:"user"`
}

func utcPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

func ToUserResponse(u *model.User) UserResponse {
	return UserResponse{
		ID: u.ID, Name: u.Name, Email: u.Email, Role: u.Role, BranchID: u.BranchID,
		IsActive: u.IsActive, MustChangePassword: u.MustChangePassword,
		LockedUntil: utcPtr(u.LockedUntil), LastLoginAt: utcPtr(u.LastLoginAt),
		CreatedAt: u.CreatedAt.UTC(),
	}
}

func ToSessionResponse(s *model.Session) SessionResponse {
	return SessionResponse{
		AccessToken: s.AccessToken, TokenType: "Bearer", ExpiresAt: s.ExpiresAt.UTC(),
		RefreshToken: s.RefreshToken, User: ToUserResponse(s.User),
	}
}
