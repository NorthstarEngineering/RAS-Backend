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
