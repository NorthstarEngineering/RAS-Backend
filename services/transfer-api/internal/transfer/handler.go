package transfer

import (
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const identityKey = "transfer.identity"

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Register mounts the routes under e.g. the /api/v1 group.
func (h *Handler) Register(r *gin.RouterGroup) {
	g := r.Group("/transfers", IdentityMiddleware())
	g.POST("", h.create)
	g.GET("", h.list)
	g.GET("/stats", h.stats)
	g.GET("/:id", h.get)
	g.POST("/:id/approve", h.approve)
	g.POST("/:id/reject", h.reject)
	g.POST("/:id/send", h.send)
	g.POST("/:id/receive", h.receive)
}

// IdentityMiddleware reads the identity injected by the API Gateway (no JWT handling here).
func IdentityMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := uuid.Parse(c.GetHeader("X-User-ID"))
		if err != nil {
			respondError(c, ErrInvalidIdentity)
			c.Abort()
			return
		}
		role := strings.ToUpper(strings.TrimSpace(c.GetHeader("X-User-Role")))
		if role != RoleCEO && role != RoleBranchManager {
			respondError(c, ErrInvalidIdentity)
			c.Abort()
			return
		}
		branchID, err := uuid.Parse(c.GetHeader("X-Branch-ID"))
		if err != nil {
			branchID = uuid.Nil
		}
		if role == RoleBranchManager && branchID == uuid.Nil {
			respondError(c, ErrInvalidIdentity)
			c.Abort()
			return
		}
		c.Set(identityKey, Identity{UserID: userID, Role: role, BranchID: branchID})
		c.Next()
	}
}

func identityFrom(c *gin.Context) Identity {
	v, _ := c.Get(identityKey)
	id, _ := v.(Identity)
	return id
}

func transferID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, ErrInvalidID)
		return uuid.Nil, false
	}
	return id, true
}

// bindOptional binds a JSON body that may be empty.
func bindOptional(c *gin.Context, dst any) bool {
	if err := c.ShouldBindJSON(dst); err != nil && !errors.Is(err, io.EOF) {
		respondBadRequest(c, err)
		return false
	}
	return true
}

func (h *Handler) create(c *gin.Context) {
	var req CreateTransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, err)
		return
	}
	out, err := h.svc.Create(c.Request.Context(), identityFrom(c), req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": out})
}

func (h *Handler) approve(c *gin.Context) {
	id, ok := transferID(c)
	if !ok {
		return
	}
	var req ApproveRequest
	if !bindOptional(c, &req) {
		return
	}
	out, err := h.svc.Approve(c.Request.Context(), identityFrom(c), id, req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

func (h *Handler) reject(c *gin.Context) {
	id, ok := transferID(c)
	if !ok {
		return
	}
	var req RejectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, err)
		return
	}
	out, err := h.svc.Reject(c.Request.Context(), identityFrom(c), id, req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

func (h *Handler) send(c *gin.Context) {
	id, ok := transferID(c)
	if !ok {
		return
	}
	var req SendRequest
	if !bindOptional(c, &req) {
		return
	}
	out, err := h.svc.Send(c.Request.Context(), identityFrom(c), id, req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

func (h *Handler) receive(c *gin.Context) {
	id, ok := transferID(c)
	if !ok {
		return
	}
	var req ReceiveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, err)
		return
	}
	out, err := h.svc.Receive(c.Request.Context(), identityFrom(c), id, req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

func (h *Handler) get(c *gin.Context) {
	id, ok := transferID(c)
	if !ok {
		return
	}
	out, err := h.svc.Get(c.Request.Context(), identityFrom(c), id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

// GET /transfers?direction=incoming|outgoing&status=&q=&from=2026-10-01&to=2026-10-31&branch_id=&page=&limit=
func (h *Handler) list(c *gin.Context) {
	f, err := parseFilter(c)
	if err != nil {
		respondError(c, err)
		return
	}
	items, err := h.svc.List(c.Request.Context(), identityFrom(c), f)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": ListResponse{Items: items, Page: f.Page, Limit: f.Limit}})
}

// GET /transfers/stats - same filters as the list (cards on TR-01 and TR-06).
func (h *Handler) stats(c *gin.Context) {
	f, err := parseFilter(c)
	if err != nil {
		respondError(c, err)
		return
	}
	st, err := h.svc.Stats(c.Request.Context(), identityFrom(c), f)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": st})
}

func parseFilter(c *gin.Context) (ListFilter, error) {
	var f ListFilter

	switch d := Direction(strings.ToLower(c.Query("direction"))); d {
	case "", DirectionIncoming, DirectionOutgoing:
		f.Direction = d
	default:
		return f, errors.Join(ErrInvalidRequest, errors.New("direction must be incoming or outgoing"))
	}

	if s := Status(strings.ToUpper(c.Query("status"))); s != "" {
		if !s.Valid() {
			return f, errors.Join(ErrInvalidRequest, errors.New("unknown status"))
		}
		f.Status = s
	}

	f.Search = strings.TrimSpace(c.Query("q"))

	if v := c.Query("branch_id"); v != "" { // used by CEO only; managers are scoped in the service
		b, err := uuid.Parse(v)
		if err != nil {
			return f, errors.Join(ErrInvalidRequest, errors.New("invalid branch_id"))
		}
		f.BranchID = &b
	}

	if v := c.Query("from"); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			return f, errors.Join(ErrInvalidRequest, errors.New("from must be YYYY-MM-DD"))
		}
		f.From = &t
	}
	if v := c.Query("to"); v != "" { // inclusive day
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			return f, errors.Join(ErrInvalidRequest, errors.New("to must be YYYY-MM-DD"))
		}
		t = t.Add(24 * time.Hour)
		f.To = &t
	}

	f.Page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	if f.Page < 1 {
		f.Page = 1
	}
	f.Limit, _ = strconv.Atoi(c.DefaultQuery("limit", "20"))
	if f.Limit < 1 {
		f.Limit = 20
	}
	if f.Limit > 100 {
		f.Limit = 100
	}
	return f, nil
}

func respondBadRequest(c *gin.Context, err error) {
	c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
}

func respondError(c *gin.Context, err error) {
	status, code := statusFor(err)
	msg := err.Error()
	if status == http.StatusInternalServerError {
		log.Printf("transfer: internal error: %v", err)
		msg = "internal server error"
	}
	c.JSON(status, gin.H{"error": gin.H{"code": code, "message": msg}})
}

func statusFor(err error) (int, string) {
	switch {
	case errors.Is(err, ErrInvalidIdentity):
		return http.StatusUnauthorized, "UNAUTHENTICATED"
	case errors.Is(err, ErrForbidden):
		return http.StatusForbidden, "FORBIDDEN"
	case errors.Is(err, ErrTransferNotFound):
		return http.StatusNotFound, "TRANSFER_NOT_FOUND"
	case errors.Is(err, ErrBranchNotFound):
		return http.StatusNotFound, "BRANCH_NOT_FOUND"
	case errors.Is(err, ErrGoodNotFound):
		return http.StatusNotFound, "GOOD_NOT_FOUND"
	case errors.Is(err, ErrInvalidState):
		return http.StatusConflict, "INVALID_STATE"
	case errors.Is(err, ErrInsufficientStock):
		return http.StatusBadRequest, "INSUFFICIENT_STOCK"
	case errors.Is(err, ErrSameBranch):
		return http.StatusBadRequest, "SAME_BRANCH"
	case errors.Is(err, ErrInvalidQuantity),
		errors.Is(err, ErrApprovedExceedsRequested),
		errors.Is(err, ErrReceivedExceedsSent):
		return http.StatusBadRequest, "INVALID_QUANTITY"
	case errors.Is(err, ErrReasonRequired):
		return http.StatusBadRequest, "REASON_REQUIRED"
	case errors.Is(err, ErrInvalidRequest), errors.Is(err, ErrInvalidID):
		return http.StatusBadRequest, "INVALID_REQUEST"
	default:
		return http.StatusInternalServerError, "INTERNAL_ERROR"
	}
}