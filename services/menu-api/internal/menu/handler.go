package menu

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	CtxUserID   = "user_id"
	CtxUserRole = "user_role"
	CtxBranchID = "branch_id"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes - بيسجل الـ Endpoints
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	m := r.Group("/menu")

	m.GET("/items", h.listMenuItems)
	m.GET("/items/:id", h.getMenuItem)
	m.POST("/items", h.createMenuItemDirect)
	m.POST("/requests", h.createRequest)
	m.GET("/requests", h.listRequests)
	m.PATCH("/requests/:id/approve", h.approveRequest)
	m.PATCH("/requests/:id/reject", h.rejectRequest)
	m.GET("/categories", h.listCategories)
	m.POST("/categories", h.createCategory)
	m.PATCH("/items/:id", h.updateMenuItem)

}

// @Summary      List Menu Items
// @Description  Get all items in the Master Menu.
// @Tags         Menu Items
// @Produce      json
// @Success      200  {array}   MenuItemResponse
// @Router       /menu/items [get]
func (h *Handler) listMenuItems(c *gin.Context) {
	items, err := h.svc.ListMenuItems(c.Request.Context())
	if err != nil {
		writeDomainError(c, err)
		return
	}
	resp := make([]MenuItemResponse, 0, len(items))
	for _, i := range items {
		resp = append(resp, i.ToResponse())
	}
	c.JSON(http.StatusOK, resp)
}

// @Summary      Get Menu Item by ID
// @Tags         Menu Items
// @Produce      json
// @Param        id path string true "Item ID"
// @Success      200 {object} MenuItemResponse
// @Router       /menu/items/{id} [get]
func (h *Handler) getMenuItem(c *gin.Context) {
	id := c.Param("id")
	item, err := h.svc.GetMenuItem(c.Request.Context(), id)
	if err != nil {
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, item.ToResponse())
}

// @Summary      Create Menu Item (CEO Only)
// @Tags         Menu Items
// @Accept       json
// @Produce      json
// @Param        request body CreateMenuItemDirectRequest true "Item Data"
// @Success      201 {object} MenuItemResponse
// @Router       /menu/items [post]
func (h *Handler) createMenuItemDirect(c *gin.Context) {
	var req CreateMenuItemDirectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_payload", err.Error())
		return
	}

	item, err := h.svc.CreateMenuItem(c.Request.Context(), CreateMenuItemInput{
		ActorRole:       getRole(c, req.ActorRole), // ← استبدلي السطر ده
		SKU:             req.SKU,
		Name:            req.Name,
		NameEn:          req.NameEn,
		CategoryID:      req.CategoryID,
		SellingPrice:    req.SellingPrice,
		KDSStation:      req.KDSStation,
		PrepTimeMinutes: req.PrepTimeMinutes,
		Description:     req.Description,
	})
	if err != nil {
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusCreated, item.ToResponse())
}

// @Summary      Create Menu Request (Branch Manager)
// @Tags         Requests
// @Accept       json
// @Produce      json
// @Param        request body CreateMenuRequestRequest true "Request Data"
// @Success      201 {object} MenuRequestResponse
// @Router       /menu/requests [post]
func (h *Handler) createRequest(c *gin.Context) {
	var req CreateMenuRequestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_payload", err.Error())
		return
	}

	actorID := c.GetString(CtxUserID)
	if actorID == "" {
		actorID = "99999999-9999-9999-9999-999999999999"
	}
	branchID := req.BranchID

	mr, err := h.svc.CreateRequest(c.Request.Context(), CreateRequestInput{
		BranchID:       branchID,
		RequestedBy:    actorID,
		ItemName:       req.ItemName,
		RequestedPrice: req.RequestedPrice,
		Description:    req.Description,
		Justification:  req.Justification,
	})
	if err != nil {
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusCreated, mr.ToResponse())
}

// @Summary      List Menu Requests
// @Tags         Requests
// @Produce      json
// @Success      200 {array} MenuRequestResponse
// @Router       /menu/requests [get]
func (h *Handler) listRequests(c *gin.Context) {
	reqs, err := h.svc.ListRequests(c.Request.Context())
	if err != nil {
		writeDomainError(c, err)
		return
	}
	resp := make([]MenuRequestResponse, 0, len(reqs))
	for _, r := range reqs {
		resp = append(resp, r.ToResponse())
	}
	c.JSON(http.StatusOK, resp)
}

// @Summary      Approve Menu Request (CEO Only)
// @Tags         Requests
// @Accept       json
// @Produce      json
// @Param        id   path string true "Request ID"
// @Param        request body ApproveRequestRequest true "Approve Data"
// @Success      201 {object} MenuItemResponse
// @Router       /menu/requests/{id}/approve [patch]
func (h *Handler) approveRequest(c *gin.Context) {
	id := c.Param("id")
	var req ApproveRequestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_payload", err.Error())
		return
	}

	item, err := h.svc.ApproveRequest(c.Request.Context(), ApproveRequestInput{
		RequestID: id,
		ActorRole: getRole(c, req.ActorRole), // ← استبدلي السطر ده
		SKU:       req.SKU,
	})
	if err != nil {
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusCreated, item.ToResponse())
}

// @Summary      Reject Menu Request (CEO Only)
// @Tags         Requests
// @Accept       json
// @Produce      json
// @Param        id   path string true "Request ID"
// @Param        request body RejectRequestRequest true "Reject Data"
// @Success      200 {object} MenuRequestResponse
// @Router       /menu/requests/{id}/reject [patch]
func (h *Handler) rejectRequest(c *gin.Context) {
	id := c.Param("id")
	var req RejectRequestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_payload", err.Error())
		return
	}

	mr, err := h.svc.RejectRequest(c.Request.Context(), RejectRequestInput{
		RequestID: id,
		ActorRole: getRole(c, req.ActorRole), // ← استبدلي السطر ده
		Reason:    req.Reason,
	})
	if err != nil {
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, mr.ToResponse())
}

// ============================================================
// Error Handling
// ============================================================

func writeDomainError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrUnauthorized):
		writeError(c, http.StatusForbidden, "forbidden", err.Error())
	case errors.Is(err, ErrRequestNotFound), errors.Is(err, ErrItemNotFound):
		writeError(c, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, ErrInvalidStatus):
		writeError(c, http.StatusConflict, "invalid_status", err.Error())
	default:
		log.Printf("[menu] internal error: %v", err)
		writeError(c, http.StatusInternalServerError, "internal_error", "something went wrong")
	}
}

func writeError(c *gin.Context, status int, code, msg string) {
	c.JSON(status, ErrorResponse{
		Error: ErrorBody{Code: code, Message: msg},
	})
}
func getRole(c *gin.Context, fallback string) string {
	role := c.GetString(CtxUserRole)
	if role == "" {
		role = fallback
	}
	if role == "" {
		role = "CEO" // افتراضي عشان الاختبار
	}
	return role
}

// @Summary      List Categories
// @Tags         Categories
// @Produce      json
// @Success      200  {array}  CategoryResponse
// @Router       /menu/categories [get]
func (h *Handler) listCategories(c *gin.Context) {
	cats, err := h.svc.ListCategories(c.Request.Context())
	if err != nil {
		writeDomainError(c, err)
		return
	}
	resp := make([]CategoryResponse, 0, len(cats))
	for _, cat := range cats {
		resp = append(resp, cat.ToResponse())
	}
	c.JSON(http.StatusOK, resp)
}

// @Summary      Create Category
// @Tags         Categories
// @Accept       json
// @Produce      json
// @Param        request body CreateCategoryRequest true "Category Data"
// @Success      201  {object}  CategoryResponse
// @Router       /menu/categories [post]
func (h *Handler) createCategory(c *gin.Context) {
	var req CreateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_payload", err.Error())
		return
	}
	cat, err := h.svc.CreateCategory(c.Request.Context(), CreateCategoryInput{
		Name: req.Name, SortOrder: req.SortOrder,
	})
	if err != nil {
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusCreated, cat.ToResponse())
}

// @Summary      Update Menu Item
// @Tags         Menu Items
// @Accept       json
// @Produce      json
// @Param        id   path string true "Item ID"
// @Param        request body UpdateMenuItemRequest true "Update Data"
// @Success      200  {object} MenuItemResponse
// @Router       /menu/items/{id} [patch]
func (h *Handler) updateMenuItem(c *gin.Context) {
	id := c.Param("id")
	var req UpdateMenuItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_payload", err.Error())
		return
	}
	item, err := h.svc.UpdateMenuItem(c.Request.Context(), UpdateMenuItemInput{
		ActorRole: getRole(c, req.ActorRole), ID: id,
		Name: req.Name, NameEn: req.NameEn,
		CategoryID: req.CategoryID, SellingPrice: req.SellingPrice,
		KDSStation: req.KDSStation, PrepTimeMinutes: req.PrepTimeMinutes,
		Description: req.Description, IsActive: req.IsActive,
	})
	if err != nil {
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, item.ToResponse())
}
