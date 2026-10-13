package attendance

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ============================================================
// Context keys — نفس المفاتيح اللي بنستخدمها مع الـ middleware
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
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes — بيسجّل الـ endpoints على Gin router group
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	att := r.Group("/attendance")
	{
		att.POST("/check-in", h.checkIn)
		att.POST("/check-out", h.checkOut)
		att.PATCH("/deductions/:id", h.adjustDeduction)
	}
}

// ============================================================
// POST /attendance/check-in
// Ref: FR-ATT-001, FR-ATT-003
// ============================================================

// CheckIn godoc
//
//	@Summary		Record employee check-in
//	@Description	Records an employee check-in and automatically calculates lateness deduction if applicable
//	@Tags			attendance
//	@Accept			json
//	@Produce		json
//	@Param			request	body		CheckInRequest	true	"Check-in data"
//	@Success		201		{object}	CheckInResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		404		{object}	ErrorResponse
//	@Failure		409		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/attendance/check-in [post]
func (h *Handler) checkIn(c *gin.Context) {
	var req CheckInRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_payload", err.Error())
		return
	}

	actorID := c.GetString(CtxUserID)

	result, err := h.svc.CheckIn(c.Request.Context(), CheckInInput{
		EmployeeID:    req.EmployeeID,
		ActorID:       actorID,
		ActorRole:     c.GetString(CtxUserRole),
		ActorBranchID: c.GetString(CtxBranchID),
		Source:        EventSource(req.Source),
	})
	if err != nil {
		writeDomainError(c, err)
		return
	}

	resp := CheckInResponse{
		Record: result.Record.ToResponse(),
	}
	if result.Deduction != nil {
		d := result.Deduction.ToResponse()
		resp.Deduction = &d
	}
	c.JSON(http.StatusCreated, resp)
}

// ============================================================
// POST /attendance/check-out
// Ref: FR-ATT-002
// ============================================================

// CheckOut godoc
//
//	@Summary		Record employee check-out
//	@Description	Records check-out for the currently open attendance record
//	@Tags			attendance
//	@Accept			json
//	@Produce		json
//	@Param			request	body		CheckOutRequest	true	"Check-out data"
//	@Success		200		{object}	AttendanceRecordResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		404		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/attendance/check-out [post]
func (h *Handler) checkOut(c *gin.Context) {
	var req CheckOutRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_payload", err.Error())
		return
	}

	actorID := c.GetString(CtxUserID)

	record, err := h.svc.CheckOut(c.Request.Context(), CheckOutInput{
		EmployeeID:    req.EmployeeID,
		ActorID:       actorID,
		ActorRole:     c.GetString(CtxUserRole),
		ActorBranchID: c.GetString(CtxBranchID),
	})
	if err != nil {
		writeDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, record.ToResponse())
}

// ============================================================
// PATCH /attendance/deductions/:id
// Ref: FR-DED-006..009
// ============================================================

// AdjustDeduction godoc
//
//	@Summary		Adjust or cancel a deduction
//	@Description	Allows CEO or Branch Manager to adjust (increase/decrease) or cancel a deduction. BM is limited to their own branch.
//	@Tags			attendance
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string					true	"Deduction ID (UUID)"
//	@Param			request	body		AdjustDeductionRequest	true	"Adjustment data"
//	@Success		200		{object}	DeductionResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		403		{object}	ErrorResponse
//	@Failure		404		{object}	ErrorResponse
//	@Failure		409		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/attendance/deductions/{id} [patch]
func (h *Handler) adjustDeduction(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		writeError(c, http.StatusBadRequest, "missing_id", "deduction id is required")
		return
	}

	var req AdjustDeductionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_payload", err.Error())
		return
	}

	// Validation إضافي مش covered by binding tags
	if !req.Cancel && req.NewAmount == nil {
		writeError(c, http.StatusBadRequest, "no_adjustment",
			"provide either new_amount or cancel=true")
		return
	}

	actorID := c.GetString(CtxUserID)
	actorRole := c.GetString(CtxUserRole)
	actorBranchID := c.GetString(CtxBranchID)

	d, err := h.svc.AdjustDeduction(c.Request.Context(), AdjustDeductionInput{
		DeductionID:   id,
		ActorID:       actorID,
		ActorRole:     actorRole,
		ActorBranchID: actorBranchID,
		NewAmount:     req.NewAmount,
		Cancel:        req.Cancel,
		Reason:        req.Reason,
	})
	if err != nil {
		writeDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, d.ToResponse())
}

// ============================================================
// Error Mapping — Domain Error → HTTP Status
// ============================================================

func writeDomainError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrEmployeeNotFound),
		errors.Is(err, ErrBranchNotFound),
		errors.Is(err, ErrDeductionNotFound),
		errors.Is(err, ErrNotCheckedIn):
		writeError(c, http.StatusNotFound, "not_found", err.Error())

	case errors.Is(err, ErrAlreadyCheckedIn),
		errors.Is(err, ErrAlreadyCheckedOut),
		errors.Is(err, ErrAlreadyCancelled):
		writeError(c, http.StatusConflict, "conflict", err.Error())

	case errors.Is(err, ErrUnauthorizedBranch),
		errors.Is(err, ErrForbiddenAction):
		writeError(c, http.StatusForbidden, "forbidden", err.Error())

	case errors.Is(err, ErrEmployeeInactive),
		errors.Is(err, ErrCentralKitchenNotAllowed),
		errors.Is(err, ErrReasonRequired),
		errors.Is(err, ErrInvalidAmount),
		errors.Is(err, ErrNoAdjustmentSpecified):
		writeError(c, http.StatusBadRequest, "validation_error", err.Error())

	default:
		slog.Error("internal operation failed", "service", "attendance-api", "request_id", c.GetString("request_id"), "area", "attendance")
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
