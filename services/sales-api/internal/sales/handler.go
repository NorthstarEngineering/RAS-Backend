package sales

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ============================================================
// Context keys
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

// RegisterRoutes - بيسجل الـ Endpoints
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	sales := r.Group("/sales")
	{
		// Shifts
		sales.POST("/shifts/open", h.openShift)
		sales.PATCH("/shifts/:id/close", h.closeShift)
		sales.GET("/shifts/:id/summary", h.getShiftSummary)
		// Sales
		sales.POST("/", h.recordSale)

		// Discounts
		sales.POST("/discounts", h.createDiscount)
		sales.PATCH("/discounts/:id/cancel", h.cancelDiscount)

	}
}

// ============================================================
// Helpers
// ============================================================
func writeError(c *gin.Context, status int, code, msg string) {
	c.JSON(status, gin.H{
		"error": gin.H{
			"code":    code,
			"message": msg,
		},
	})
}

func writeDomainError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrShiftNotFound),
		errors.Is(err, ErrMenuItemNotFound),
		errors.Is(err, ErrDiscountNotFound):
		writeError(c, http.StatusNotFound, "not_found", err.Error())

	case errors.Is(err, ErrShiftNotOpen),
		errors.Is(err, ErrShiftNotClosed):
		writeError(c, http.StatusConflict, "invalid_state", err.Error())

	case errors.Is(err, ErrInvalidChannel):
		writeError(c, http.StatusBadRequest, "invalid_channel", err.Error())

	default:
		log.Printf("[sales] internal error: %v", err)
		writeError(c, http.StatusInternalServerError, "internal_error", "something went wrong")
	}
}

// ============================================================
// Shifts
// ============================================================

// @Summary      Open Shift
// @Tags         Shifts
// @Accept       json
// @Produce      json
// @Param        request body OpenShiftRequest true "Open Shift Data"
// @Success      201  {object}  ShiftResponse
// @Router       /sales/shifts/open [post]
func (h *Handler) openShift(c *gin.Context) {
	var req OpenShiftRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_payload", err.Error())
		return
	}

	shift, err := h.svc.OpenShift(c.Request.Context(), req)
	if err != nil {
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusCreated, shift.ToResponse())
}

// @Summary      Close Shift
// @Tags         Shifts
// @Accept       json
// @Produce      json
// @Param        id path string true "Shift ID"
// @Success      200  {object}  ShiftResponse
// @Router       /sales/shifts/{id}/close [patch]
func (h *Handler) closeShift(c *gin.Context) {
	id := c.Param("id")

	shift, err := h.svc.CloseShift(c.Request.Context(), CloseShiftRequest{ShiftID: id})
	if err != nil {
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, shift.ToResponse())
}

// ============================================================
// Sales
// ============================================================

// @Summary      Record Sale
// @Tags         Sales
// @Accept       json
// @Produce      json
// @Param        request body RecordSaleRequest true "Sale Data"
// @Success      201  {object}  SaleResponse
// @Router       /sales [post]
func (h *Handler) recordSale(c *gin.Context) {
	var req RecordSaleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_payload", err.Error())
		return
	}

	actorID := c.GetString(CtxUserID)
	if actorID == "" {
		actorID = "99999999-9999-9999-9999-999999999999" // Default للاختبار
	}

	sale, items, err := h.svc.RecordSale(c.Request.Context(), req, actorID)
	if err != nil {
		writeDomainError(c, err)
		return
	}

	c.JSON(http.StatusCreated, sale.ToResponse(items))
}

// ============================================================
// Discounts
// ============================================================

// @Summary      Create Discount
// @Tags         Discounts
// @Accept       json
// @Produce      json
// @Param        request body CreateDiscountRequest true "Discount Data"
// @Success      201  {object}  DiscountResponse
// @Router       /sales/discounts [post]
func (h *Handler) createDiscount(c *gin.Context) {
	var req CreateDiscountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_payload", err.Error())
		return
	}

	actorID := c.GetString(CtxUserID)
	if actorID == "" {
		actorID = "99999999-9999-9999-9999-999999999999"
	}

	discount, err := h.svc.CreateDiscount(c.Request.Context(), req, actorID)
	if err != nil {
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusCreated, discount.ToResponse())
}

// @Summary      Cancel Discount
// @Tags         Discounts
// @Accept       json
// @Produce      json
// @Param        id path string true "Discount ID"
// @Param        request body CancelDiscountRequest true "Cancel Reason"
// @Success      200  {object}  DiscountResponse
// @Router       /sales/discounts/{id}/cancel [patch]
func (h *Handler) cancelDiscount(c *gin.Context) {
	id := c.Param("id")

	var req CancelDiscountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_payload", err.Error())
		return
	}

	actorID := c.GetString(CtxUserID)
	if actorID == "" {
		actorID = "99999999-9999-9999-9999-999999999999"
	}

	discount, err := h.svc.CancelDiscount(c.Request.Context(), id, req, actorID)
	if err != nil {
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, discount.ToResponse())
}

// @Summary      Get Shift Summary
// @Tags         Shifts
// @Produce      json
// @Param        id path string true "Shift ID"
// @Success      200  {object}  ShiftSummaryResponse
// @Router       /sales/shifts/{id}/summary [get]
func (h *Handler) getShiftSummary(c *gin.Context) {
	id := c.Param("id")

	summary, err := h.svc.GetShiftSummary(c.Request.Context(), id)
	if err != nil {
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, summary)
}
