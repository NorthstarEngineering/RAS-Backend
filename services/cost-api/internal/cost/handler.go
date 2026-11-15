package cost

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	c := r.Group("/cost")
	{
		c.POST("/items", h.setCost)
		c.GET("/items/:id", h.getCosting)
		// Reports
		c.GET("/reports/cogs", h.getCOGSReport)
		c.GET("/reports/gross-profit", h.getGrossProfitReport)
		c.GET("/reports/estimated-contribution", h.getEstimatedContribution)
	}
}

// @Summary      Set Menu Item Cost
// @Tags         Costing
// @Accept       json
// @Produce      json
// @Param        request body SetCostRequest true "Cost & BOM Data"
// @Success      200
// @Router       /cost/items [post]
func (h *Handler) setCost(c *gin.Context) {
	var req SetCostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.svc.SetCost(c.Request.Context(), req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "saved"})
}

// @Summary      Get Menu Item Costing
// @Tags         Costing
// @Produce      json
// @Param        id path string true "Menu Item ID"
// @Success      200  {object}  MenuItemCostingResponse
// @Router       /cost/items/{id} [get]
func (h *Handler) getCosting(c *gin.Context) {
	id := c.Param("id")
	resp, err := h.svc.GetMenuItemCosting(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if resp == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no cost found for this item"})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// @Summary      Get COGS Report
// @Tags         Reports
// @Produce      json
// @Param        branch_id query string false "Filter by branch ID"
// @Param        from query string false "From date (YYYY-MM-DD)"
// @Param        to query string false "To date (YYYY-MM-DD)"
// @Success      200 {object} COGSReportResponse
// @Router       /cost/reports/cogs [get]
func (h *Handler) getCOGSReport(c *gin.Context) {
	filter := ReportFilter{
		BranchID: c.Query("branch_id"),
		From:     c.Query("from"),
		To:       c.Query("to"),
	}
	resp, err := h.svc.GetCOGSReport(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// @Summary      Get Gross Profit Report
// @Tags         Reports
// @Produce      json
// @Param        branch_id query string false "Filter by branch ID"
// @Param        from query string false "From date (YYYY-MM-DD)"
// @Param        to query string false "To date (YYYY-MM-DD)"
// @Success      200 {object} GrossProfitReportResponse
// @Router       /cost/reports/gross-profit [get]
func (h *Handler) getGrossProfitReport(c *gin.Context) {
	filter := ReportFilter{
		BranchID: c.Query("branch_id"),
		From:     c.Query("from"),
		To:       c.Query("to"),
	}
	resp, err := h.svc.GetGrossProfitReport(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// @Summary      Get Estimated Contribution
// @Tags         Reports
// @Produce      json
// @Param        branch_id query string false "Filter by branch ID"
// @Param        from query string false "From date (YYYY-MM-DD)"
// @Param        to query string false "To date (YYYY-MM-DD)"
// @Success      200 {object} EstimatedContributionResponse
// @Router       /cost/reports/estimated-contribution [get]
func (h *Handler) getEstimatedContribution(c *gin.Context) {
	filter := ReportFilter{
		BranchID: c.Query("branch_id"),
		From:     c.Query("from"),
		To:       c.Query("to"),
	}
	resp, err := h.svc.GetEstimatedContribution(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}
