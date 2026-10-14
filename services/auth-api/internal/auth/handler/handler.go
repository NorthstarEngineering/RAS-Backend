package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"restaurant-backend/internal/auth/dto"
	autherrors "restaurant-backend/internal/auth/errors"
	"restaurant-backend/internal/auth/model"
	"restaurant-backend/internal/auth/service"
)

func bindJSON(c *gin.Context, dst any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<16)
	if err := c.ShouldBindJSON(dst); err != nil {
		respondError(c, autherrors.ErrValidation)
		return false
	}
	return true
}

func pathID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, autherrors.ErrValidation.WithMessage("Invalid id"))
		return uuid.Nil, false
	}
	return id, true
}

type Handler struct {
	svc     *service.Service
	mw      *Middleware
	limiter *RateLimiter
}

func New(svc *service.Service, mw *Middleware, limiter *RateLimiter) *Handler {
	return &Handler{svc: svc, mw: mw, limiter: limiter}
}

func (h *Handler) RegisterRoutes(api *gin.RouterGroup) {
	a := api.Group("/auth")
	a.POST("/login", h.limiter.Middleware(), h.Login)
	a.POST("/refresh", h.limiter.Middleware(), h.Refresh)
	a.POST("/logout", h.Logout)
	a.GET("/me", h.mw.AuthenticateAllowPasswordChange(), h.Me)
	a.POST("/change-password", h.mw.AuthenticateAllowPasswordChange(), h.ChangePassword)
	a.PATCH("/me/email", h.mw.Authenticate(), h.ChangeEmail)

	u := api.Group("/users", h.mw.Authenticate(), h.mw.RequireRoles(model.RoleCEO))
	u.POST("", h.CreateUser)
	u.GET("", h.ListUsers)
	u.PATCH("/:id/status", h.SetUserStatus)
	u.POST("/:id/reset-password", h.ResetUserPassword)
	u.POST("/:id/unlock", h.UnlockUser)
}

func (h *Handler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if !bindJSON(c, &req) {
		return
	}
	s, err := h.svc.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ToSessionResponse(s))
}

func (h *Handler) Refresh(c *gin.Context) {
	var req dto.RefreshRequest
	if !bindJSON(c, &req) {
		return
	}
	s, err := h.svc.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ToSessionResponse(s))
}

func (h *Handler) Logout(c *gin.Context) {
	var req dto.RefreshRequest
	if !bindJSON(c, &req) {
		return
	}
	if err := h.svc.Logout(c.Request.Context(), req.RefreshToken); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) Me(c *gin.Context) {
	u, err := h.svc.Me(c.Request.Context(), claimsFrom(c).UserID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ToUserResponse(u))
}

func (h *Handler) ChangePassword(c *gin.Context) {
	var req dto.ChangePasswordRequest
	if !bindJSON(c, &req) {
		return
	}
	s, err := h.svc.ChangePassword(c.Request.Context(), claimsFrom(c).UserID, req.CurrentPassword, req.NewPassword)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ToSessionResponse(s))
}

func (h *Handler) ChangeEmail(c *gin.Context) {
	var req dto.ChangeEmailRequest
	if !bindJSON(c, &req) {
		return
	}
	u, err := h.svc.ChangeEmail(c.Request.Context(), claimsFrom(c).UserID, req.NewEmail, req.CurrentPassword)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ToUserResponse(u))
}

func (h *Handler) CreateUser(c *gin.Context) {
	var req dto.CreateUserRequest
	if !bindJSON(c, &req) {
		return
	}
	u, err := h.svc.CreateUser(c.Request.Context(), claimsFrom(c), service.CreateUserInput{
		Name: req.Name, Email: req.Email, Password: req.Password, Role: req.Role, BranchID: req.BranchID,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, dto.ToUserResponse(u))
}

func (h *Handler) ListUsers(c *gin.Context) {
	users, err := h.svc.ListUsers(c.Request.Context(), claimsFrom(c))
	if err != nil {
		respondError(c, err)
		return
	}
	out := make([]dto.UserResponse, 0, len(users))
	for _, u := range users {
		out = append(out, dto.ToUserResponse(u))
	}
	c.JSON(http.StatusOK, gin.H{"users": out})
}

func (h *Handler) SetUserStatus(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req dto.SetActiveRequest
	if !bindJSON(c, &req) {
		return
	}
	if err := h.svc.SetUserActive(c.Request.Context(), claimsFrom(c), id, *req.IsActive); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) ResetUserPassword(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req dto.ResetPasswordRequest
	if !bindJSON(c, &req) {
		return
	}
	if err := h.svc.ResetUserPassword(c.Request.Context(), claimsFrom(c), id, req.NewPassword); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) UnlockUser(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.svc.UnlockUser(c.Request.Context(), claimsFrom(c), id); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
