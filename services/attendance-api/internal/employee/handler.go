package employee

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// ============================================================
// Context keys — نفس اللي عند Attendance
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

// RegisterRoutes — بيسجّل الـ endpoints
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	emp := r.Group("/employees")
	{
		emp.POST("", h.create)
		emp.GET("", h.list)
		emp.GET("/:id", h.get)
		emp.PATCH("/:id", h.update)
		emp.PATCH("/:id/payment", h.changePayment)
		emp.POST("/:id/terminate", h.terminate)
	}
}

// ============================================================
// POST /employees
// Ref: FR-EMP-001, FR-EMP-002
// ============================================================

// CreateEmployee godoc
//
//	@Summary		Create a new employee
//	@Description	CEO can create employees in any branch. BM creates only in their own branch (BR-003).
//	@Tags			employee
//	@Accept			json
//	@Produce		json
//	@Param			request	body		CreateEmployeeRequest	true	"Employee data"
//	@Success		201		{object}	EmployeeResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		403		{object}	ErrorResponse
//	@Failure		409		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/employees [post]
func (h *Handler) create(c *gin.Context) {
	var req CreateEmployeeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_payload", err.Error())
		return
	}

	hireDate, err := time.Parse("2006-01-02", req.HireDate)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_hire_date",
			"hire_date must be in format YYYY-MM-DD")
		return
	}

	actorID := c.GetString(CtxUserID)
	actorRole := c.GetString(CtxUserRole)
	actorBranchID := c.GetString(CtxBranchID)

	emp, err := h.svc.Create(c.Request.Context(), CreateInput{
		Name:          req.Name,
		Phone:         req.Phone,
		NationalID:    req.NationalID,
		BranchID:      req.BranchID,
		PaymentType:   PaymentType(req.PaymentType),
		PaymentAmount: req.PaymentAmount,
		HireDate:      hireDate,
		JobTitle:      req.JobTitle,
		Email:         req.Email,
		Address:       req.Address,
		ActorID:       actorID,
		ActorRole:     actorRole,
		ActorBranchID: actorBranchID,
	})
	if err != nil {
		writeDomainError(c, err)
		return
	}

	c.JSON(http.StatusCreated, emp.ToResponse())
}

// ============================================================
// GET /employees
// Ref: FR-AUTH-004
// ============================================================

// ListEmployees godoc
//
//	@Summary		List employees
//	@Description	CEO sees all employees. BM sees only their branch.
//	@Tags			employee
//	@Accept			json
//	@Produce		json
//	@Param			branch_id	query		string	false	"Filter by branch ID (CEO only)"
//	@Param			active		query		bool	false	"Filter active only (true/false)"
//	@Success		200			{object}	ListEmployeesResponse
//	@Failure		500			{object}	ErrorResponse
//	@Router			/employees [get]
func (h *Handler) list(c *gin.Context) {
	actorRole := c.GetString(CtxUserRole)
	actorBranchID := c.GetString(CtxBranchID)

	branchID := c.Query("branch_id")
	if actorRole == "BM" {
		branchID = actorBranchID
	}

	activeOnly := c.Query("active") == "true"

	emps, err := h.svc.List(c.Request.Context(), ListInput{
		BranchID:      branchID,
		ActiveOnly:    activeOnly,
		ActorRole:     actorRole,
		ActorBranchID: actorBranchID,
	})
	if err != nil {
		writeDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, ToListResponse(emps))
}

// ============================================================
// GET /employees/:id
// ============================================================

// GetEmployee godoc
//
//	@Summary		Get employee by ID
//	@Description	CEO can fetch any employee. BM only their branch.
//	@Tags			employee
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string	true	"Employee ID (UUID)"
//	@Success		200	{object}	EmployeeResponse
//	@Failure		403	{object}	ErrorResponse
//	@Failure		404	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/employees/{id} [get]
func (h *Handler) get(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		writeError(c, http.StatusBadRequest, "missing_id", "employee id is required")
		return
	}

	emp, err := h.svc.Get(c.Request.Context(), GetInput{
		EmployeeID:    id,
		ActorRole:     c.GetString(CtxUserRole),
		ActorBranchID: c.GetString(CtxBranchID),
	})
	if err != nil {
		writeDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, emp.ToResponse())
}

// ============================================================
// PATCH /employees/:id
// ============================================================

// UpdateEmployee godoc
//
//	@Summary		Update employee data
//	@Description	CEO any branch. BM own branch only.
//	@Tags			employee
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string					true	"Employee ID (UUID)"
//	@Param			request	body		UpdateEmployeeRequest	true	"Update data"
//	@Success		200		{object}	EmployeeResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		403		{object}	ErrorResponse
//	@Failure		404		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/employees/{id} [patch]
func (h *Handler) update(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		writeError(c, http.StatusBadRequest, "missing_id", "employee id is required")
		return
	}

	var req UpdateEmployeeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_payload", err.Error())
		return
	}

	var pt *PaymentType
	if req.PaymentType != nil {
		p := PaymentType(*req.PaymentType)
		pt = &p
	}

	emp, err := h.svc.Update(c.Request.Context(), UpdateInput{
		EmployeeID:    id,
		Name:          req.Name,
		Phone:         req.Phone,
		JobTitle:      req.JobTitle,
		Email:         req.Email,
		Address:       req.Address,
		PaymentType:   pt,
		PaymentAmount: req.PaymentAmount,
		ActorID:       c.GetString(CtxUserID),
		ActorRole:     c.GetString(CtxUserRole),
		ActorBranchID: c.GetString(CtxBranchID),
	})
	if err != nil {
		writeDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, emp.ToResponse())
}

// ============================================================
// PATCH /employees/:id/payment
// Ref: FR-EMP-006
// ============================================================

// ChangePayment godoc
//
//	@Summary		Change employee payment (type + amount)
//	@Description	Ref: FR-EMP-006. Changes the payment type (daily/monthly) and amount.
//	@Tags			employee
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string					true	"Employee ID (UUID)"
//	@Param			request	body		ChangePaymentRequest	true	"Payment data"
//	@Success		200		{object}	EmployeeResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		403		{object}	ErrorResponse
//	@Failure		404		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/employees/{id}/payment [patch]
func (h *Handler) changePayment(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		writeError(c, http.StatusBadRequest, "missing_id", "employee id is required")
		return
	}

	var req ChangePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_payload", err.Error())
		return
	}

	emp, err := h.svc.ChangePayment(c.Request.Context(), ChangePaymentInput{
		EmployeeID:    id,
		PaymentType:   PaymentType(req.PaymentType),
		PaymentAmount: req.PaymentAmount,
		ActorID:       c.GetString(CtxUserID),
		ActorRole:     c.GetString(CtxUserRole),
		ActorBranchID: c.GetString(CtxBranchID),
	})
	if err != nil {
		writeDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, emp.ToResponse())
}

// ============================================================
// POST /employees/:id/terminate
// ============================================================

// TerminateEmployee godoc
//
//	@Summary		Terminate (deactivate) employee
//	@Description	Marks employee as inactive and sets termination date.
//	@Tags			employee
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string				true	"Employee ID (UUID)"
//	@Param			request	body		TerminateRequest	true	"Termination data"
//	@Success		200		{object}	EmployeeResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		403		{object}	ErrorResponse
//	@Failure		404		{object}	ErrorResponse
//	@Failure		409		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/employees/{id}/terminate [post]
func (h *Handler) terminate(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		writeError(c, http.StatusBadRequest, "missing_id", "employee id is required")
		return
	}

	var req TerminateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_payload", err.Error())
		return
	}

	terminationDate, err := time.Parse("2006-01-02", req.TerminationDate)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_termination_date",
			"termination_date must be in format YYYY-MM-DD")
		return
	}

	emp, err := h.svc.Terminate(c.Request.Context(), TerminateInput{
		EmployeeID:      id,
		TerminationDate: terminationDate,
		ActorID:         c.GetString(CtxUserID),
		ActorRole:       c.GetString(CtxUserRole),
		ActorBranchID:   c.GetString(CtxBranchID),
	})
	if err != nil {
		writeDomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, emp.ToResponse())
}

// ============================================================
// Error Mapping
// ============================================================

func writeDomainError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrEmployeeNotFound),
		errors.Is(err, ErrBranchNotFound):
		writeError(c, http.StatusNotFound, "not_found", err.Error())

	case errors.Is(err, ErrNationalIDDuplicate),
		errors.Is(err, ErrAlreadyTerminated),
		errors.Is(err, ErrAlreadyActive):
		writeError(c, http.StatusConflict, "conflict", err.Error())

	case errors.Is(err, ErrUnauthorizedBranch),
		errors.Is(err, ErrForbiddenAction):
		writeError(c, http.StatusForbidden, "forbidden", err.Error())

	case errors.Is(err, ErrNameRequired),
		errors.Is(err, ErrPhoneRequired),
		errors.Is(err, ErrPhoneInvalid),
		errors.Is(err, ErrNationalIDRequired),
		errors.Is(err, ErrNationalIDInvalid),
		errors.Is(err, ErrInvalidPaymentType),
		errors.Is(err, ErrInvalidPaymentAmount),
		errors.Is(err, ErrInvalidHireDate),
		errors.Is(err, ErrInvalidTerminationDate),
		errors.Is(err, ErrBranchRequired),
		errors.Is(err, ErrInvalidEmail),
		errors.Is(err, ErrCannotTerminateActive):
		writeError(c, http.StatusBadRequest, "validation_error", err.Error())

	default:
		slog.Error("internal operation failed", "service", "attendance-api", "request_id", c.GetString("request_id"), "area", "employee")
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
