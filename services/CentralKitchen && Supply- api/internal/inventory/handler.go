package inventory

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
	inv := r.Group("/inventory")
	{
		inv.POST("/movements", h.recordMovement)
		inv.POST("/goods", h.createGood)
		inv.GET("/goods", h.listGoods)
		inv.GET("/goods/:id", h.getGood)
	}
	supply := r.Group("/supply")
	{
		supply.POST("/requests", h.createSupplyRequest)
		supply.GET("/requests", h.listSupplyRequests)
		supply.PATCH("/requests/:id/approve", h.approveSupplyRequest)
		supply.PATCH("/requests/:id/ship", h.shipSupplyRequest)
		supply.PATCH("/requests/:id/receive", h.receiveSupplyRequest)
		supply.PATCH("/requests/:id/reject", h.rejectSupplyRequest)
	}
}

// ============================================================
// Helpers
// ============================================================

func getRole(c *gin.Context, fallback string) string {
	role := c.GetString(CtxUserRole)
	if role == "" {
		role = fallback
	}
	if role == "" {
		role = "CEO"
	}
	return role
}

func writeError(c *gin.Context, status int, code, msg string) {
	c.JSON(status, ErrorResponse{
		Error: ErrorBody{
			Code:    code,
			Message: msg,
		},
	})
}

func writeDomainError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrInvalidQuantity),
		errors.Is(err, ErrInvalidMovementType),
		errors.Is(err, ErrReasonRequired):
		writeError(c, http.StatusBadRequest, "validation_error", err.Error())

	case errors.Is(err, ErrInsufficientStock):
		writeError(c, http.StatusConflict, "insufficient_stock", err.Error())

	case errors.Is(err, ErrUnauthorizedBranch),
		errors.Is(err, ErrForbiddenAction):
		writeError(c, http.StatusForbidden, "forbidden", err.Error())

	default:
		log.Printf("[inventory] internal error: %v", err)
		writeError(c, http.StatusInternalServerError, "internal_error", "something went wrong")
	}
}

// ============================================================
// Inventory
// ============================================================

// @Summary      Record inventory movement
// @Description  Records an IN, OUT, or ADJUSTMENT movement for a branch.
// @Tags         inventory
// @Accept       json
// @Produce      json
// @Param        request body CreateMovementRequest true "Movement details"
// @Success      201  {object}  CreateMovementResponse
// @Failure      400  {object}  ErrorResponse
// @Failure      409  {object}  ErrorResponse
// @Router       /inventory/movements [post]
func (h *Handler) recordMovement(c *gin.Context) {
	var req CreateMovementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_payload", err.Error())
		return
	}

	actorID := c.GetString(CtxUserID)
	actorRole := c.GetString(CtxUserRole)
	branchID := c.GetString(CtxBranchID)

	if actorID == "" {
		actorID = req.ActorID
	}
	if branchID == "" {
		branchID = req.BranchID
	}

	if actorID == "" {
		actorID = "99999999-9999-9999-9999-999999999999"
	}
	if branchID == "" {
		branchID = "11111111-1111-1111-1111-111111111111"
	}

	if actorRole == "BRANCH_MANAGER" && branchID == "" {
		writeError(c, http.StatusForbidden, "forbidden", "missing branch id")
		return
	}

	resp, err := h.svc.RecordMovement(c.Request.Context(), RecordMovementInput{
		BranchID: branchID,
		ActorID:  actorID,
		GoodID:   req.GoodID,
		Type:     req.Type,
		Quantity: req.Quantity,
		Reason:   req.Reason,
	})
	if err != nil {
		writeDomainError(c, err)
		return
	}

	c.JSON(http.StatusCreated, resp)
}

// @Summary      Create a new good
// @Description  Adds a new good to the master inventory catalog.
// @Tags         goods
// @Accept       json
// @Produce      json
// @Param        request body CreateGoodRequest true "Good details"
// @Success      201  {object}  Good
// @Failure      400  {object}  ErrorResponse
// @Router       /inventory/goods [post]
func (h *Handler) createGood(c *gin.Context) {
	var req CreateGoodRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_payload", err.Error())
		return
	}

	g, err := h.svc.CreateGood(c.Request.Context(), CreateGoodInput{
		SKU:          req.SKU,
		Name:         req.Name,
		Category:     req.Category,
		Description:  req.Description,
		Unit:         req.Unit,
		UnitCost:     req.UnitCost,
		ReorderLevel: req.ReorderLevel,
		MaxCapacity:  req.MaxCapacity,
		InitialQty:   req.InitialQty,
	})
	if err != nil {
		writeDomainError(c, err)
		return
	}

	c.JSON(http.StatusCreated, g)
}

// @Summary      List all goods
// @Description  Get a list of all goods in the catalog.
// @Tags         goods
// @Produce      json
// @Success      200  {array}   Good
// @Router       /inventory/goods [get]
func (h *Handler) listGoods(c *gin.Context) {
	goods, err := h.svc.ListGoods(c.Request.Context())
	if err != nil {
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, goods)
}

// @Summary      Get good by ID
// @Description  Get details of a specific good.
// @Tags         goods
// @Produce      json
// @Param        id   path      string  true  "Good ID"
// @Success      200  {object}  Good
// @Failure      404  {object}  ErrorResponse
// @Router       /inventory/goods/{id} [get]
func (h *Handler) getGood(c *gin.Context) {
	id := c.Param("id")
	g, err := h.svc.GetGoodByID(c.Request.Context(), id)
	if err != nil {
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, g)
}

// ============================================================
// Supply Requests
// ============================================================

// @Summary      Create Supply Request (Branch Manager)
// @Tags         Supply Requests
// @Accept       json
// @Produce      json
// @Param        request body CreateSupplyRequestReq true "Supply Request Data"
// @Success      201 {object} SupplyRequestResponse
// @Router       /supply/requests [post]
func (h *Handler) createSupplyRequest(c *gin.Context) {
	var req CreateSupplyRequestReq
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_payload", err.Error())
		return
	}

	actorID := c.GetString(CtxUserID)
	if actorID == "" {
		actorID = "99999999-9999-9999-9999-999999999999"
	}

	var items []SupplyRequestItemInput
	for _, item := range req.Items {
		items = append(items, SupplyRequestItemInput{GoodID: item.GoodID, RequestedQty: item.RequestedQty})
	}

	sr, err := h.svc.CreateSupplyRequest(c.Request.Context(), CreateSupplyRequestInput{
		BranchID: req.BranchID, CreatedBy: actorID, Notes: req.Notes, Priority: req.Priority, Items: items,
	})
	if err != nil {
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusCreated, sr.ToResponse())
}

// @Summary      List Supply Requests
// @Tags         Supply Requests
// @Produce      json
// @Success      200 {array} SupplyRequestResponse
// @Router       /supply/requests [get]
func (h *Handler) listSupplyRequests(c *gin.Context) {
	list, err := h.svc.ListSupplyRequests(c.Request.Context())
	if err != nil {
		writeDomainError(c, err)
		return
	}
	resp := make([]SupplyRequestResponse, 0, len(list))
	for _, r := range list {
		resp = append(resp, r.ToResponse())
	}
	c.JSON(http.StatusOK, resp)
}

// @Summary      Approve Supply Request (Supply Manager)
// @Tags         Supply Requests
// @Accept       json
// @Produce      json
// @Param        id   path string true "Request ID"
// @Param        request body ApproveSupplyRequestReq true "Approve Data"
// @Success      200 {object} SupplyRequestResponse
// @Router       /supply/requests/{id}/approve [patch]
func (h *Handler) approveSupplyRequest(c *gin.Context) {
	id := c.Param("id")
	var req ApproveSupplyRequestReq
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_payload", err.Error())
		return
	}

	actorRole := getRole(c, req.ActorRole)
	var appItems []ApproveItemInput
	for _, i := range req.Items {
		appItems = append(appItems, ApproveItemInput{ItemID: i.ItemID, ApprovedQty: i.ApprovedQty})
	}

	sr, err := h.svc.ApproveSupplyRequest(c.Request.Context(), ApproveSupplyRequestInput{
		RequestID: id, ActorRole: actorRole, Items: appItems,
	})
	if err != nil {
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, sr.ToResponse())
}

// @Summary      Ship Supply Request (Supply Manager)
// @Tags         Supply Requests
// @Produce      json
// @Param        id   path string true "Request ID"
// @Param        request body string false "actor_role"
// @Success      200 {object} map[string]string
// @Router       /supply/requests/{id}/ship [patch]
func (h *Handler) shipSupplyRequest(c *gin.Context) {
	id := c.Param("id")
	role := c.GetString(CtxUserRole)
	if role == "" {
		role = "SUPPLY_MANAGER"
	}

	err := h.svc.ShipSupplyRequest(c.Request.Context(), role, id)
	if err != nil {
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "shipped"})
}

// @Summary      Receive Supply Request (Branch Manager)
// @Tags         Supply Requests
// @Accept       json
// @Produce      json
// @Param        id   path string true "Request ID"
// @Param        request body ReceiveSupplyRequestReq true "Receive Data"
// @Success      200 {object} map[string]string
// @Router       /supply/requests/{id}/receive [patch]
func (h *Handler) receiveSupplyRequest(c *gin.Context) {
	id := c.Param("id")

	var req ReceiveSupplyRequestReq
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_payload", err.Error())
		return
	}

	var recItems []ReceiveItemInput
	for _, i := range req.Items {
		recItems = append(recItems, ReceiveItemInput{ItemID: i.ItemID, ReceivedQty: i.ReceivedQty})
	}

	err := h.svc.ReceiveSupplyRequest(c.Request.Context(), ReceiveSupplyRequestInput{
		RequestID: id,
		ActorRole: getRole(c, req.ActorRole),
		Items:     recItems,
	})
	if err != nil {
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "fulfilled"})
}

// @Summary      Reject Supply Request (Supply Manager)
// @Tags         Supply Requests
// @Accept       json
// @Produce      json
// @Param        id   path string true "Request ID"
// @Param        request body RejectSupplyRequestReq true "Reject Data"
// @Success      200 {object} SupplyRequestResponse
// @Router       /supply/requests/{id}/reject [patch]
func (h *Handler) rejectSupplyRequest(c *gin.Context) {
	id := c.Param("id")
	var req RejectSupplyRequestReq
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_payload", err.Error())
		return
	}

	sr, err := h.svc.RejectSupplyRequest(c.Request.Context(), RejectSupplyRequestInput{
		RequestID: id,
		ActorRole: getRole(c, req.ActorRole),
		Reason:    req.Reason,
	})
	if err != nil {
		writeDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, sr.ToResponse())
}
