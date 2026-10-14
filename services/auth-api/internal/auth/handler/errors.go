package handler

import (
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	autherrors "restaurant-backend/internal/auth/errors"
)

var statusByCode = map[string]int{
	autherrors.ErrInvalidCredentials.Code: http.StatusUnauthorized,
	autherrors.ErrUnauthorized.Code:       http.StatusUnauthorized,
	autherrors.ErrInvalidToken.Code:       http.StatusUnauthorized,
	autherrors.ErrAccountDisabled.Code:    http.StatusForbidden,
	autherrors.ErrForbidden.Code:          http.StatusForbidden,
	autherrors.ErrPasswordChange.Code:     http.StatusForbidden,
	autherrors.ErrUserNotFound.Code:       http.StatusNotFound,
	autherrors.ErrBranchNotFound.Code:     http.StatusNotFound,
	autherrors.ErrEmailTaken.Code:         http.StatusConflict,
	autherrors.ErrValidation.Code:         http.StatusBadRequest,
	autherrors.ErrInvalidEmail.Code:       http.StatusBadRequest,
	autherrors.ErrWeakPassword.Code:       http.StatusBadRequest,
	autherrors.ErrWrongPassword.Code:      http.StatusBadRequest,
	autherrors.ErrInvalidRole.Code:        http.StatusBadRequest,
	autherrors.ErrBranchRequired.Code:     http.StatusBadRequest,
	autherrors.ErrBranchNotAllowed.Code:   http.StatusBadRequest,
}

// errorBody follows the shared HTTP convention:
// {"error": {"code": "...", "message": "..."}}
func errorBody(code, message string) gin.H {
	return gin.H{"error": gin.H{"code": code, "message": message}}
}

func respondError(c *gin.Context, err error) {
	var locked *autherrors.LockedError
	if errors.As(err, &locked) {
		secs := int(math.Ceil(locked.RetryAfter.Seconds()))
		c.Header("Retry-After", strconv.Itoa(secs))
		c.AbortWithStatusJSON(http.StatusTooManyRequests, errorBody("ACCOUNT_LOCKED",
			fmt.Sprintf("Too many failed attempts. Try again in %d minute(s).", (secs+59)/60)))
		return
	}
	var appErr *autherrors.Error
	if errors.As(err, &appErr) {
		status, ok := statusByCode[appErr.Code]
		if !ok {
			status = http.StatusBadRequest
		}
		c.AbortWithStatusJSON(status, errorBody(appErr.Code, appErr.Message))
		return
	}
	log.Printf("auth: internal error: %v", err)
	c.AbortWithStatusJSON(http.StatusInternalServerError, errorBody("INTERNAL_ERROR", "Something went wrong"))
}
