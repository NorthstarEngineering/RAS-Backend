// Package autherrors holds the auth module's domain errors.
package autherrors

import (
	"fmt"
	"time"
)

// Error is a domain error with a stable, machine-readable code.
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

// WithMessage returns a copy of the error with a more specific message.
func (e *Error) WithMessage(msg string) *Error { return &Error{Code: e.Code, Message: msg} }

var (
	ErrInvalidCredentials = &Error{"INVALID_CREDENTIALS", "Invalid email or password"}
	ErrAccountDisabled    = &Error{"ACCOUNT_DISABLED", "This account has been disabled"}
	ErrUnauthorized       = &Error{"UNAUTHORIZED", "Authentication required"}
	ErrInvalidToken       = &Error{"INVALID_TOKEN", "Invalid or expired token"}
	ErrForbidden          = &Error{"FORBIDDEN", "You are not allowed to perform this action"}
	ErrPasswordChange     = &Error{"PASSWORD_CHANGE_REQUIRED", "You must change your password before continuing"}

	ErrUserNotFound = &Error{"USER_NOT_FOUND", "User not found"}
	ErrEmailTaken   = &Error{"EMAIL_ALREADY_EXISTS", "Email is already in use"}

	ErrValidation    = &Error{"VALIDATION_ERROR", "Invalid request"}
	ErrInvalidEmail  = &Error{"INVALID_EMAIL", "Email address is not valid"}
	ErrWeakPassword  = &Error{"WEAK_PASSWORD", "Password does not meet the policy"}
	ErrWrongPassword = &Error{"WRONG_CURRENT_PASSWORD", "Current password is incorrect"}

	ErrInvalidRole      = &Error{"INVALID_ROLE", "Role is not allowed for this operation"}
	ErrBranchRequired   = &Error{"BRANCH_REQUIRED", "A Branch Manager must be assigned to a branch"}
	ErrBranchNotAllowed = &Error{"BRANCH_NOT_ALLOWED", "This role cannot be assigned to a branch"}
	ErrBranchNotFound   = &Error{"BRANCH_NOT_FOUND", "Branch not found"}
)

// LockedError is returned while an account is temporarily locked.
type LockedError struct {
	RetryAfter time.Duration
}

func (e *LockedError) Error() string {
	return fmt.Sprintf("ACCOUNT_LOCKED: retry after %s", e.RetryAfter.Round(time.Second))
}
