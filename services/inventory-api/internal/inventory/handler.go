package inventory

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

const (
	HeaderUserID   = "X-User-ID"
	HeaderUserRole = "X-User-Role"
	HeaderBranchID = "X-Branch-ID"

	actorKey     = "inventory.actor"
	maxBodyBytes = 1 << 20 // 1 MB
	dateLayout   = "2006-01-02"
)

// RequestError is a malformed request detected in the HTTP layer (bad query/body).
type RequestError struct{ Msg string }

func (e *RequestError) Error() string { return e.Msg }

func badRequest(msg string) error { return &RequestError{Msg: msg} }

type Handler struct {
	svc *Service
	log *slog.Logger
}

func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// RegisterRoutes mounts the inventory endpoints under the given group (e.g. /api/v1).
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/inventory", h.identity())
	g.GET("", h.listInventory)                           // list screen: items (search, category, status)
	g.GET("/summary", h.getSummary)                      // list screen: cards, tabs, category chips
	g.GET("/items/:good_id", h.getItem)                  // details screen
	g.PATCH("/items/:good_id/settings", h.updateSettings) // safety level / capacity / location
	g.POST("/movements", h.createMovement)               // IN / OUT / ADJUSTMENT
	g.GET("/movements", h.listMovements)                 // movements screen (audit)
	g.GET("/movements/summary", h.getMovementSummary)    // movements screen: cards
	g.GET("/value", h.getValue)                          // valuation screen
}

// ==================== Identity ====================

// identity reads the Gateway headers once and stores the Actor in the context.
func (h *Handler) identity() gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, err := actorFromHeaders(c.Request.Header)
		if err != nil {
			h.fail(c, err)
			return
		}
		c.Set(actorKey, actor)
		c.Next()
	}
}

func actorFromHeaders(hdr http.Header) (Actor, error) {
	userID, err := uuid.Parse(strings.TrimSpace(hdr.Get(HeaderUserID)))
	if err != nil || userID == uuid.Nil {
		return Actor{}, ErrMissingIdentity
	}

	role := Role(strings.ToUpper(strings.TrimSpace(hdr.Get(HeaderUserRole))))
	if !role.IsValid() {
		return Actor{}, ErrMissingIdentity
	}

	var branchID uuid.UUID
	if raw := strings.TrimSpace(hdr.Get(HeaderBranchID)); raw != "" {
		if branchID, err = uuid.Parse(raw); err != nil {
			return Actor{}, ErrMissingIdentity
		}
	}

	return Actor{UserID: userID, Role: role, BranchID: branchID}, nil
}

func actorFrom(c *gin.Context) Actor {
	return c.MustGet(actorKey).(Actor)
}

// ==================== Endpoints ====================

// GET /api/v1/inventory?branch_id=&q=&category=&status=&page=&limit=
func (h *Handler) listInventory(c *gin.Context) {
	branchID, err := optionalUUID(c, "branch_id")
	if err != nil {
		h.fail(c, err)
		return
	}
	page, limit, err := paging(c)
	if err != nil {
		h.fail(c, err)
		return
	}

	q := InventoryQuery{
		Search:   strings.TrimSpace(c.Query("q")),
		Category: strings.TrimSpace(c.Query("category")),
		Page:     page,
		Limit:    limit,
	}
	if raw := strings.TrimSpace(c.Query("status")); raw != "" {
		st := StockStatus(strings.ToUpper(raw))
		q.Status = &st
	}

	res, err := h.svc.ListInventory(c.Request.Context(), actorFrom(c), branchID, q)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// GET /api/v1/inventory/summary?branch_id=
func (h *Handler) getSummary(c *gin.Context) {
	branchID, err := optionalUUID(c, "branch_id")
	if err != nil {
		h.fail(c, err)
		return
	}

	res, err := h.svc.GetInventorySummary(c.Request.Context(), actorFrom(c), branchID)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// GET /api/v1/inventory/items/:good_id?branch_id=
func (h *Handler) getItem(c *gin.Context) {
	goodID, err := uuid.Parse(c.Param("good_id"))
	if err != nil {
		h.fail(c, badRequest("good_id must be a valid UUID"))
		return
	}
	branchID, err := optionalUUID(c, "branch_id")
	if err != nil {
		h.fail(c, err)
		return
	}

	res, err := h.svc.GetInventoryItem(c.Request.Context(), actorFrom(c), branchID, goodID)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// PATCH /api/v1/inventory/items/:good_id/settings?branch_id=
func (h *Handler) updateSettings(c *gin.Context) {
	goodID, err := uuid.Parse(c.Param("good_id"))
	if err != nil {
		h.fail(c, badRequest("good_id must be a valid UUID"))
		return
	}
	branchID, err := optionalUUID(c, "branch_id")
	if err != nil {
		h.fail(c, err)
		return
	}

	var req StockSettingsRequest
	if err := decodeBody(c, &req); err != nil {
		h.fail(c, err)
		return
	}

	res, err := h.svc.UpdateStockSettings(c.Request.Context(), actorFrom(c), branchID, goodID, req)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// POST /api/v1/inventory/movements
func (h *Handler) createMovement(c *gin.Context) {
	var req CreateMovementRequest
	if err := decodeBody(c, &req); err != nil {
		h.fail(c, err)
		return
	}

	res, err := h.svc.RecordMovement(c.Request.Context(), actorFrom(c), req)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, res)
}

// GET /api/v1/inventory/movements?branch_id=&good_id=&type=&from=&to=&q=&page=&limit=
func (h *Handler) listMovements(c *gin.Context) {
	branchID, f, err := movementQuery(c)
	if err != nil {
		h.fail(c, err)
		return
	}
	if f.Page, f.Limit, err = paging(c); err != nil {
		h.fail(c, err)
		return
	}

	res, err := h.svc.ListMovements(c.Request.Context(), actorFrom(c), branchID, f)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// GET /api/v1/inventory/movements/summary?branch_id=&good_id=&type=&from=&to=&q=
func (h *Handler) getMovementSummary(c *gin.Context) {
	branchID, f, err := movementQuery(c)
	if err != nil {
		h.fail(c, err)
		return
	}

	res, err := h.svc.GetMovementSummary(c.Request.Context(), actorFrom(c), branchID, f)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// GET /api/v1/inventory/value?branch_id=&top=
func (h *Handler) getValue(c *gin.Context) {
	branchID, err := optionalUUID(c, "branch_id")
	if err != nil {
		h.fail(c, err)
		return
	}
	top, err := intQuery(c, "top", 0)
	if err != nil {
		h.fail(c, err)
		return
	}

	res, err := h.svc.GetInventoryValue(c.Request.Context(), actorFrom(c), branchID, top)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// ==================== Body / query helpers ====================

// decodeBody reads exactly one JSON object, rejecting unknown fields and oversized bodies.
func decodeBody(c *gin.Context, dst any) error {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBodyBytes)

	dec := json.NewDecoder(c.Request.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return badRequest("request body is required")
		}
		return badRequest("invalid JSON body: " + err.Error())
	}
	if dec.More() {
		return badRequest("request body must contain a single JSON object")
	}
	return nil
}

// movementQuery parses the filters shared by the movements list and its summary.
func movementQuery(c *gin.Context) (*uuid.UUID, MovementFilter, error) {
	var f MovementFilter

	branchID, err := optionalUUID(c, "branch_id")
	if err != nil {
		return nil, f, err
	}
	if f.GoodID, err = optionalUUID(c, "good_id"); err != nil {
		return nil, f, err
	}
	if raw := strings.TrimSpace(c.Query("type")); raw != "" {
		t := MovementType(strings.ToUpper(raw))
		f.Type = &t
	}
	if f.From, err = optionalTime(c, "from", false); err != nil {
		return nil, f, err
	}
	if f.To, err = optionalTime(c, "to", true); err != nil {
		return nil, f, err
	}
	f.Search = strings.TrimSpace(c.Query("q"))
	return branchID, f, nil
}

func optionalUUID(c *gin.Context, key string) (*uuid.UUID, error) {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return nil, nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return nil, badRequest(key + " must be a valid UUID")
	}
	return &id, nil
}

func paging(c *gin.Context) (int, int, error) {
	page, err := intQuery(c, "page", DefaultPage)
	if err != nil {
		return 0, 0, err
	}
	limit, err := intQuery(c, "limit", DefaultLimit)
	if err != nil {
		return 0, 0, err
	}
	return page, limit, nil
}

func intQuery(c *gin.Context, key string, def int) (int, error) {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return def, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, badRequest(key + " must be an integer")
	}
	return n, nil
}

// optionalTime accepts RFC3339 ("2026-10-20T10:00:00Z") or a date ("2026-10-20").
// For "to" with a date only, the whole day is included (upper bound is exclusive).
func optionalTime(c *gin.Context, key string, endOfDay bool) (*time.Time, error) {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return nil, nil
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return &t, nil
	}
	t, err := time.Parse(dateLayout, raw)
	if err != nil {
		return nil, badRequest(key + " must be RFC3339 or YYYY-MM-DD")
	}
	if endOfDay {
		t = t.AddDate(0, 0, 1)
	}
	return &t, nil
}

// ==================== Error mapping ====================

type errorMapping struct {
	target error
	status int
	code   string
}

var errorMappings = []errorMapping{
	{ErrMissingIdentity, http.StatusUnauthorized, "UNAUTHENTICATED"},
	{ErrUnauthorizedBranch, http.StatusForbidden, "BRANCH_FORBIDDEN"},
	{ErrAdjustmentNotAllowed, http.StatusForbidden, "ADJUSTMENT_NOT_ALLOWED"},
	{ErrInventoryNotFound, http.StatusNotFound, "INVENTORY_NOT_FOUND"},
	{ErrStaleBalance, http.StatusConflict, "STALE_BALANCE"},
	{ErrInsufficientStock, http.StatusBadRequest, "INSUFFICIENT_STOCK"},
	{ErrInvalidMovementType, http.StatusBadRequest, "INVALID_MOVEMENT_TYPE"},
	{ErrReasonRequired, http.StatusBadRequest, "REASON_REQUIRED"},
	{ErrInvalidDestination, http.StatusBadRequest, "INVALID_DESTINATION"},
	{ErrInvalidReasonCode, http.StatusBadRequest, "INVALID_REASON_CODE"},
	{ErrInvalidAdjustmentKind, http.StatusBadRequest, "INVALID_ADJUSTMENT_KIND"},
	{ErrAdjustmentKindMismatch, http.StatusBadRequest, "ADJUSTMENT_KIND_MISMATCH"},
	{ErrInvalidStatus, http.StatusBadRequest, "VALIDATION_ERROR"},
	{ErrGoodRequired, http.StatusBadRequest, "VALIDATION_ERROR"},
	{ErrQuantityRequired, http.StatusBadRequest, "VALIDATION_ERROR"},
	{ErrQuantityAndDelta, http.StatusBadRequest, "VALIDATION_ERROR"},
	{ErrDeltaNotAllowed, http.StatusBadRequest, "VALIDATION_ERROR"},
	{ErrZeroDelta, http.StatusBadRequest, "VALIDATION_ERROR"},
	{ErrNegativeBalance, http.StatusBadRequest, "VALIDATION_ERROR"},
	{ErrInvalidQuantity, http.StatusBadRequest, "VALIDATION_ERROR"},
	{ErrNegativeQuantity, http.StatusBadRequest, "VALIDATION_ERROR"},
	{ErrInvalidPrecision, http.StatusBadRequest, "VALIDATION_ERROR"},
	{ErrQuantityTooLarge, http.StatusBadRequest, "VALIDATION_ERROR"},
	{ErrInvalidUnitCost, http.StatusBadRequest, "VALIDATION_ERROR"},
	{ErrUnitCostNotAllowed, http.StatusBadRequest, "VALIDATION_ERROR"},
	{ErrReasonCodeNotAllowed, http.StatusBadRequest, "VALIDATION_ERROR"},
	{ErrAdjustmentKindNotAllowed, http.StatusBadRequest, "VALIDATION_ERROR"},
	{ErrTextTooLong, http.StatusBadRequest, "VALIDATION_ERROR"},
	{ErrNoSettingsProvided, http.StatusBadRequest, "VALIDATION_ERROR"},
	{ErrNegativeSetting, http.StatusBadRequest, "VALIDATION_ERROR"},
	{ErrInvalidStockSettings, http.StatusBadRequest, "VALIDATION_ERROR"},
	{ErrInvalidBranch, http.StatusBadRequest, "VALIDATION_ERROR"},
	{ErrInvalidDateRange, http.StatusBadRequest, "VALIDATION_ERROR"},
}

func (h *Handler) fail(c *gin.Context, err error) {
	status, code, msg := classifyError(err)
	if status >= http.StatusInternalServerError {
		h.log.ErrorContext(c.Request.Context(), "inventory request failed",
			"err", err, "method", c.Request.Method, "path", c.FullPath())
	}
	c.AbortWithStatusJSON(status, ErrorResponse{Error: ErrorBody{Code: code, Message: msg}})
}

func classifyError(err error) (int, string, string) {
	var reqErr *RequestError
	if errors.As(err, &reqErr) {
		return http.StatusBadRequest, "VALIDATION_ERROR", reqErr.Msg
	}

	for _, m := range errorMappings {
		if errors.Is(err, m.target) {
			return m.status, m.code, err.Error()
		}
	}

	// Safety net: database constraints catch anything the service missed.
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		switch pqErr.Code {
		case "23514": // check_violation
			return http.StatusBadRequest, "VALIDATION_ERROR", "request violates an inventory rule"
		case "23505": // unique_violation
			return http.StatusConflict, "CONFLICT", "resource already exists"
		case "40001", "40P01": // serialization failure / deadlock
			return http.StatusConflict, "CONFLICT", "concurrent update, please retry"
		}
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return http.StatusGatewayTimeout, "TIMEOUT", "request timed out"
	}

	return http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error"
}