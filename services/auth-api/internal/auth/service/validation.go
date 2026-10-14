package service

import (
	"net/mail"
	"strings"
	"unicode"

	autherrors "restaurant-backend/internal/auth/errors"
)

const (
	minPasswordLen = 10
	maxPasswordLen = 72 // bcrypt ignores everything after 72 bytes
)

// ValidatePasswordPolicy: 10-72 bytes, at least one letter and one digit.
func ValidatePasswordPolicy(p string) error {
	if len(p) < minPasswordLen {
		return autherrors.ErrWeakPassword.WithMessage("Password must be at least 10 characters")
	}
	if len(p) > maxPasswordLen {
		return autherrors.ErrWeakPassword.WithMessage("Password must be at most 72 characters")
	}
	var hasLetter, hasDigit bool
	for _, r := range p {
		switch {
		case unicode.IsLetter(r):
			hasLetter = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		return autherrors.ErrWeakPassword.WithMessage("Password must contain at least one letter and one digit")
	}
	return nil
}

func normalizeEmail(e string) string { return strings.ToLower(strings.TrimSpace(e)) }

func validateEmail(raw string) (string, error) {
	e := normalizeEmail(raw)
	if e == "" || len(e) > 255 {
		return "", autherrors.ErrInvalidEmail
	}
	addr, err := mail.ParseAddress(e)
	if err != nil || addr.Address != e {
		return "", autherrors.ErrInvalidEmail
	}
	return e, nil
}
