package overtime

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// ============================================================
// Context keys — نفس اللي عند Attendance و Employee
// ============================================================

const (
	CtxUserID   = "user_id"
	CtxUserRole = "user_role"
	CtxBranchID = "branch_id"
)

// ============================================================
// Handler
// ============================================================

type Handler struct {
	svc *Service

	// للحصول على فرع الموظف
	employeeBranchLookup func(ctx context.Context, employeeID string) (string, error)
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// SetEmployeeBranchLookup — لإدخال دالة جلب فرع الموظف
// (الـ wiring في main.go)
func (h *Handler) SetEmployeeBranchLookup(fn func(ctx context.Context, employeeID string) (string, error)) {
	h.employeeBranchLookup = fn
}

// RegisterRoutes — بيسجّل الـ endpoints
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	ot := r.Group("/overtime")
	{
		ot.POST("", h.create)
		ot.GET("", h.list)
		ot.GET("/:id", h.get)
		ot.PATCH("/:id/approve", h.approve)
		ot.PATCH("/:id/reject", h.reject)
	}
}

// ============================================================
// POST /overtime
// Ref: FR-OT-001, FR-OT-002
// ============================================================

// CreateOvertime godoc
//
//	@Summary		Create overtime record
//	@Description	CEO can create for any branch. BM only their own branch. Record starts as pending.
//	@Tags			overtime
//	@Accept			json
//	@Produce		json
//	@Param			request	body		CreateOvertimeRequest	true	"Overtime data"
//	@Success		201		{object}	OvertimeResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		403		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/overtime [post]
func (h *Handler) create(c *gin.Context) {
	var req CreateOvertimeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_payload", err.Error())
		return
	}

	// Parse work_date
	workDate, err := time.Parse("2006-01-02", req.WorkDate)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_work_date",
			"work_date must be in format YYYY-MM-DD")
		return
	}

	// للحصول على فرع الموظف
	var employeeBranchID string
	if h.employeeBranchLookup != nil {
		branchID, err := h.employeeBranchLookup(c.Request.Context(), req.EmployeeID)
		if err != nil {
			slog.Error("employee branch lookup failed", "service", "attendance-api", "request_id", c.GetString("request_id"), "area", "overtime")
			writeError(c, http.StatusInternalServerError, "internal_error", "something went wrong")
			return
		}
		employeeBranchID = branchID
	}

	ot, err := h.svc.Create(c.Request.Context(), CreateInput{
		EmployeeID:       req.EmployeeID,
		WorkDate:         workDate,
		Hours:            req.Hours,
		RatePerHour:      req.RatePerHour,
		Source:           Source(req.Source),
		ActorID:          c.GetString(CtxUserID),
		ActorRole:        c.GetString(CtxUserRole),
		ActorBranchID:    c.GetString(CtxBranchID),
		EmployeeBranchID: employeeBranchID,
	})
	if err != nil {
		writeDomainError(c, err)
		return
	}

	c.JSON(http.StatusCreated, ot.ToResponse())
}

// ============================================================
// GET /overtime
// ============================================================

// ListOvertime godoc
//
//	@Summary		List overtime records
//	@Description	CEO sees all. BM only their branch. Filter by employee, status, date range.
//	@Tags			overtime
//	@Accept			json
//	@Produce		json
//	@Param			employee_id	query		string	false	"Filter by employee ID"
//	@Param			status		query		string	false	"Filter by status (pending/approved/rejected)"
//	@Param			from		query		string	false	"From date (YYYY-MM-DD)"
//	@Param			to			query		string	false	"To date (YYYY-MM-DD)"
//	@Success		200			{object}	ListOvertimeResponse
//	@Failure		500			{object}	ErrorResponse
//	@Router			/overtime [get]
func (h *Handler) list(c *gin.Context) {
	list, err := h.svc.List(c.Request.Context(), ListInput{
		EmployeeID:    c.Query("employee_id"),
		Status:        c.Query("status"),
		FromDate:      c.Query("from"),
		ToDate:        c.Query("to"),
		ActorRole:     c.GetString(CtxUserRole),
		ActorBranchID: c.GetString(CtxBranchID),
	})
	if err != nil {
		writeDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, ToListResponse(list))
}

// ============================================================
// GET /overtime/:id
// ============================================================

// GetOvertime godoc
//
//	@Summary		Get overtime by ID
//	@Description	CEO any. BM only their branch.
//	@Tags			overtime
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"Overtime ID (UUID)"
//	@Success		200	{object}	OvertimeResponse
//	@Failure		403	{object}	ErrorResponse
//	@Failure		404	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/overtime/{id} [get]
func (h *Handler) get(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		writeError(c, http.StatusBadRequest, "missing_id", "overtime id is required")
		return
	}

	ot, err := h.svc.Get(c.Request.Context(), GetInput{
		OvertimeID:    id,
		ActorRole:     c.GetString(CtxUserRole),
		ActorBranchID: c.GetString(CtxBranchID),
	})
	if err != nil {
		writeDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, ot.ToResponse())
}

// ============================================================
// PATCH /overtime/:id/approve
// Ref: FR-OT-006, FR-OT-007, BR-011
// ============================================================

// ApproveOvertime godoc
//
//	@Summary		Approve overtime
//	@Description	Ref: FR-OT-007, BR-011. Makes OT payable. Only CEO or BM (own branch).
//	@Tags			overtime
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"Overtime ID (UUID)"
//	@Success		200	{object}	OvertimeResponse
//	@Failure		403	{object}	ErrorResponse
//	@Failure		404	{object}	ErrorResponse
//	@Failure		409	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/overtime/{id}/approve [patch]
func (h *Handler) approve(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		writeError(c, http.StatusBadRequest, "missing_id", "overtime id is required")
		return
	}

	ot, err := h.svc.Approve(c.Request.Context(), ApproveInput{
		OvertimeID:    id,
		ActorID:       c.GetString(CtxUserID),
		ActorRole:     c.GetString(CtxUserRole),
		ActorBranchID: c.GetString(CtxBranchID),
	})
	if err != nil {
		writeDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, ot.ToResponse())
}

// ============================================================
// PATCH /overtime/:id/reject
// Ref: FR-OT-008
// ============================================================

// RejectOvertime godoc
//
//	@Summary		Reject overtime
//	@Description	Ref: FR-OT-008. Requires reason. Only CEO or BM (own branch).
//	@Tags			overtime
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string					true	"Overtime ID (UUID)"
//	@Param			request	body		RejectOvertimeRequest	true	"Rejection reason"
//	@Success		200		{object}	OvertimeResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		403		{object}	ErrorResponse
//	@Failure		404		{object}	ErrorResponse
//	@Failure		409		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/overtime/{id}/reject [patch]
func (h *Handler) reject(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		writeError(c, http.StatusBadRequest, "missing_id", "overtime id is required")
		return
	}

	var req RejectOvertimeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_payload", err.Error())
		return
	}

	ot, err := h.svc.Reject(c.Request.Context(), RejectInput{
		OvertimeID:    id,
		Reason:        req.Reason,
		ActorID:       c.GetString(CtxUserID),
		ActorRole:     c.GetString(CtxUserRole),
		ActorBranchID: c.GetString(CtxBranchID),
	})
	if err != nil {
		writeDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, ot.ToResponse())
}

// ============================================================
// Error Mapping
// ============================================================

func writeDomainError(c *gin.Context, err error) {
	switch {
	// ---------- 404 Not Found ----------
	case errors.Is(err, ErrOvertimeNotFound),
		errors.Is(err, ErrEmployeeNotFound),
		errors.Is(err, ErrBranchNotFound):
		writeError(c, http.StatusNotFound, "not_found", err.Error())

	// ---------- 409 Conflict ----------
	case errors.Is(err, ErrAlreadyApproved),
		errors.Is(err, ErrAlreadyRejected),
		errors.Is(err, ErrNotPending),
		errors.Is(err, ErrCannotApproveRejected),
		errors.Is(err, ErrCannotRejectApproved):
		writeError(c, http.StatusConflict, "conflict", err.Error())

	// ---------- 403 Forbidden ----------
	case errors.Is(err, ErrUnauthorizedBranch),
		errors.Is(err, ErrForbiddenAction):
		writeError(c, http.StatusForbidden, "forbidden", err.Error())

	// ---------- 400 Bad Request ----------
	case errors.Is(err, ErrEmployeeRequired),
		errors.Is(err, ErrWorkDateRequired),
		errors.Is(err, ErrWorkDateFuture),
		errors.Is(err, ErrInvalidHours),
		errors.Is(err, ErrInvalidRate),
		errors.Is(err, ErrInvalidSource),
		errors.Is(err, ErrRejectionReasonRequired):
		writeError(c, http.StatusBadRequest, "validation_error", err.Error())

	// ---------- 500 Internal ----------
	default:
		slog.Error("internal operation failed", "service", "attendance-api", "request_id", c.GetString("request_id"), "area", "overtime")
		writeError(c, http.StatusInternalServerError, "internal_error", "something went wrong")
	}
}

func writeError(c *gin.Context, status int, code, msg string) {
	c.JSON(status, ErrorResponse{
		Error: ErrorBody{
			Code:    code,
			Message: msg,
		},
	})
}
