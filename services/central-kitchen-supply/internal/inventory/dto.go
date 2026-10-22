package inventory

import "time"

// ============================================================
// Request DTOs
// ============================================================

// CreateMovementRequest - POST /api/v1/inventory/movements
// Ref: FR-INV-002, FR-INV-003
type CreateMovementRequest struct {
	BranchID string `json:"branch_id"` // ← ضيفي السطر ده
	ActorID  string `json:"actor_id"`  // ← ضيفي السطر ده

	GoodID   string  `json:"good_id"   binding:"required,uuid"`
	Type     string  `json:"type"      binding:"required,oneof=IN OUT ADJUSTMENT"`
	Quantity float64 `json:"quantity"  binding:"required,gt=0"`
	Reason   string  `json:"reason"`
}

// ============================================================
// Response DTOs
// ============================================================

type InventoryResponse struct {
	ID        string  `json:"id"`
	BranchID  string  `json:"branch_id"`
	GoodID    string  `json:"good_id"`
	Quantity  float64 `json:"quantity"`
	UpdatedAt string  `json:"updated_at"`
}

type MovementResponse struct {
	ID          string  `json:"id"`
	InventoryID string  `json:"inventory_id"`
	Type        string  `json:"type"`
	Quantity    float64 `json:"quantity"`
	Reason      string  `json:"reason"`
	CreatedBy   string  `json:"created_by"`
	CreatedAt   string  `json:"created_at"`
}

// CreateMovementResponse - الرد بعد عمل حركة مخزون
type CreateMovementResponse struct {
	Inventory InventoryResponse `json:"inventory"`
	Movement  MovementResponse  `json:"movement"`
}

// ============================================================
// Error Response
// ============================================================

type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ============================================================
// Mappers
// ============================================================

func (i *Inventory) ToResponse() InventoryResponse {
	return InventoryResponse{
		ID:        i.ID,
		BranchID:  i.BranchID,
		GoodID:    i.GoodID,
		Quantity:  i.Quantity,
		UpdatedAt: i.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func (m *InventoryMovement) ToResponse() MovementResponse {
	return MovementResponse{
		ID:          m.ID,
		InventoryID: m.InventoryID,
		Type:        string(m.Type),
		Quantity:    m.Quantity,
		Reason:      m.Reason,
		CreatedBy:   m.CreatedBy,
		CreatedAt:   m.CreatedAt.UTC().Format(time.RFC3339),
	}
}

type CreateGoodRequest struct {
	SKU          string  `json:"sku"           binding:"required"`
	Name         string  `json:"name"          binding:"required"`
	Category     string  `json:"category"`
	Description  string  `json:"description"`
	Unit         string  `json:"unit"          binding:"required"`
	UnitCost     float64 `json:"unit_cost"     binding:"required,gte=0"`
	ReorderLevel float64 `json:"reorder_level"`
	MaxCapacity  float64 `json:"max_capacity"`
	InitialQty   float64 `json:"initial_qty"`
}

// ============================================================
// Supply Requests DTOs
// ============================================================

type CreateSupplyRequestReq struct {
	BranchID string              `json:"branch_id" binding:"required,uuid"`
	Notes    string              `json:"notes"`
	Priority string              `json:"priority"`
	Items    []SupplyRequestItem `json:"items" binding:"required,min=1,dive"`
}

type ApproveSupplyRequestReq struct {
	ActorRole string           `json:"actor_role"`
	Items     []ApproveItemReq `json:"items" binding:"required"`
}

type ApproveItemReq struct {
	ItemID      string  `json:"item_id" binding:"required,uuid"`
	ApprovedQty float64 `json:"approved_qty" binding:"required,gte=0"`
}

type SupplyRequestResponse struct {
	ID        string                  `json:"id"`
	BranchID  string                  `json:"branch_id"`
	Status    string                  `json:"status"`
	CreatedBy string                  `json:"created_by"`
	Notes     *string                 `json:"notes,omitempty"`
	Priority  string                  `json:"priority"`
	Items     []SupplyRequestItemResp `json:"items,omitempty"`
	CreatedAt string                  `json:"created_at"`
}

type SupplyRequestItemResp struct {
	ID           string   `json:"id"`
	GoodID       string   `json:"good_id"`
	RequestedQty float64  `json:"requested_qty"`
	ApprovedQty  *float64 `json:"approved_qty,omitempty"`
	ReceivedQty  *float64 `json:"received_qty,omitempty"`
}

func (r *SupplyRequest) ToResponse() SupplyRequestResponse {
	var items []SupplyRequestItemResp
	for _, i := range r.Items {
		items = append(items, SupplyRequestItemResp{
			ID: i.ID, GoodID: i.GoodID, RequestedQty: i.RequestedQty,
			ApprovedQty: i.ApprovedQty, ReceivedQty: i.ReceivedQty,
		})
	}
	return SupplyRequestResponse{
		ID: r.ID, BranchID: r.BranchID, Status: r.Status, CreatedBy: r.CreatedBy,
		Notes: r.Notes, Priority: r.Priority, Items: items,
		CreatedAt: r.CreatedAt.UTC().Format(time.RFC3339),
	}
}

type RejectSupplyRequestReq struct {
	Reason    string `json:"reason" binding:"required"`
	ActorRole string `json:"actor_role"`
}
type ReceiveSupplyRequestReq struct {
	ActorRole string           `json:"actor_role"`
	Items     []ReceiveItemReq `json:"items"`
}

type ReceiveItemReq struct {
	ItemID      string  `json:"item_id" binding:"required"`
	ReceivedQty float64 `json:"received_qty" binding:"required,gte=0"`
}
