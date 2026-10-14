package handler

import (
	"strings"

	"github.com/gin-gonic/gin"

	autherrors "restaurant-backend/internal/auth/errors"
	"restaurant-backend/internal/auth/model"
	"restaurant-backend/internal/auth/service"
)

// Context keys set by Authenticate. Other modules must NOT import the auth
// packages; they read the identity with plain strings, e.g.
//
//	role := c.GetString("role")
//	branchID := c.GetString("branch_id") // "" for CEO / Supply Manager
const (
	CtxUserID   = "user_id"
	CtxRole     = "role"
	CtxBranchID = "branch_id"
	ctxClaims   = "auth.claims"
)

type Middleware struct{ svc *service.Service }

func NewMiddleware(svc *service.Service) *Middleware { return &Middleware{svc: svc} }

func claimsFrom(c *gin.Context) *model.Claims {
	v, _ := c.Get(ctxClaims)
	claims, _ := v.(*model.Claims)
	return claims
}

// Authenticate requires a valid access token AND a finished first-login
// password change.
func (m *Middleware) Authenticate() gin.HandlerFunc { return m.authenticate(false) }

// AuthenticateAllowPasswordChange is used only by /me and /change-password.
func (m *Middleware) AuthenticateAllowPasswordChange() gin.HandlerFunc {
	return m.authenticate(true)
}

func (m *Middleware) authenticate(allowPendingPasswordChange bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		scheme, token, found := strings.Cut(c.GetHeader("Authorization"), " ")
		if !found || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(token) == "" {
			respondError(c, autherrors.ErrUnauthorized)
			return
		}
		claims, err := m.svc.VerifyAccessToken(strings.TrimSpace(token))
		if err != nil {
			respondError(c, err)
			return
		}
		if claims.MustChangePassword && !allowPendingPasswordChange {
			respondError(c, autherrors.ErrPasswordChange)
			return
		}

		c.Set(ctxClaims, claims)
		c.Set(CtxUserID, claims.UserID.String())
		c.Set(CtxRole, string(claims.Role))
		if claims.BranchID != nil {
			c.Set(CtxBranchID, claims.BranchID.String())
		} else {
			c.Set(CtxBranchID, "")
		}
		c.Next()
	}
}

// RequireRoles must run after Authenticate.
func (m *Middleware) RequireRoles(roles ...model.Role) gin.HandlerFunc {
	allowed := make(map[model.Role]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}
	return func(c *gin.Context) {
		claims := claimsFrom(c)
		if claims == nil {
			respondError(c, autherrors.ErrUnauthorized)
			return
		}
		if _, ok := allowed[claims.Role]; !ok {
			respondError(c, autherrors.ErrForbidden)
			return
		}
		c.Next()
	}
}
