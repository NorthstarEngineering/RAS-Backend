package menu

import "time"

// ============================================================
// Requests (اللي بتيجي من الـ Frontend)
// ============================================================

// CreateMenuRequestRequest - طلب إضافة صنف جديد (Branch Manager)
type CreateMenuRequestRequest struct {
	BranchID       string  `json:"branch_id" binding:"required,uuid"`
	ItemName       string  `json:"item_name" binding:"required"`
	RequestedPrice float64 `json:"requested_price" binding:"required,gt=0"`
	Description    string  `json:"description"`
	Justification  string  `json:"justification"`
}

// ApproveRequestRequest - موافقة الـ CEO
type ApproveRequestRequest struct {
	SKU       string `json:"sku" binding:"required"`
	ActorRole string `json:"actor_role,omitempty"` // ← ضيفي ده

}

// RejectRequestRequest - رفض الـ CEO
type RejectRequestRequest struct {
	Reason    string `json:"reason" binding:"required"`
	ActorRole string `json:"actor_role,omitempty"` // ← ضيفي ده

}

// CreateMenuItemDirectRequest - إضافة صنف مباشرة (CEO)
type CreateMenuItemDirectRequest struct {
	SKU             string  `json:"sku" binding:"required"`
	Name            string  `json:"name" binding:"required"`
	NameEn          string  `json:"name_en"`
	CategoryID      string  `json:"category_id"`
	SellingPrice    float64 `json:"selling_price" binding:"required,gte=0"`
	KDSStation      string  `json:"kds_station"`
	PrepTimeMinutes int     `json:"prep_time_minutes"`
	Description     string  `json:"description"`
	ActorRole       string  `json:"actor_role,omitempty"` // ← ضيفي ده

}

// ============================================================
// Responses
// ============================================================

type MenuItemResponse struct {
	ID              string  `json:"id"`
	SKU             string  `json:"sku"`
	Name            string  `json:"name"`
	NameEn          *string `json:"name_en,omitempty"`
	CategoryID      *string `json:"category_id,omitempty"`
	SellingPrice    float64 `json:"selling_price"`
	KDSStation      *string `json:"kds_station,omitempty"`
	PrepTimeMinutes *int    `json:"prep_time_minutes,omitempty"`
	Description     *string `json:"description,omitempty"`
	IsActive        bool    `json:"is_active"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
}

type MenuRequestResponse struct {
	ID              string  `json:"id"`
	BranchID        string  `json:"branch_id"`
	RequestedBy     string  `json:"requested_by"`
	ItemName        string  `json:"item_name"`
	RequestedPrice  float64 `json:"requested_price"`
	Description     *string `json:"description,omitempty"`
	Justification   *string `json:"justification,omitempty"`
	Status          string  `json:"status"`
	RejectionReason *string `json:"rejection_reason,omitempty"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
}

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

func (item *MenuItem) ToResponse() MenuItemResponse {
	return MenuItemResponse{
		ID:              item.ID,
		SKU:             item.SKU,
		Name:            item.Name,
		NameEn:          item.NameEn,
		CategoryID:      item.CategoryID,
		SellingPrice:    item.SellingPrice,
		KDSStation:      item.KDSStation,
		PrepTimeMinutes: item.PrepTimeMinutes,
		Description:     item.Description,
		IsActive:        item.IsActive,
		CreatedAt:       item.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:       item.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func (req *MenuItemRequest) ToResponse() MenuRequestResponse {
	return MenuRequestResponse{
		ID:              req.ID,
		BranchID:        req.BranchID,
		RequestedBy:     req.RequestedBy,
		ItemName:        req.ItemName,
		RequestedPrice:  req.RequestedPrice,
		Description:     req.Description,
		Justification:   req.Justification,
		Status:          req.Status,
		RejectionReason: req.RejectionReason,
		CreatedAt:       req.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:       req.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

type CreateCategoryRequest struct {
	Name      string `json:"name" binding:"required"`
	SortOrder int    `json:"sort_order"`
}

type CategoryResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
	IsActive  bool   `json:"is_active"`
	CreatedAt string `json:"created_at"`
}

func (c *Category) ToResponse() CategoryResponse {
	return CategoryResponse{
		ID: c.ID, Name: c.Name, SortOrder: c.SortOrder,
		IsActive: c.IsActive, CreatedAt: c.CreatedAt.UTC().Format(time.RFC3339),
	}
}

type UpdateMenuItemRequest struct {
	Name            string  `json:"name" binding:"required"`
	NameEn          string  `json:"name_en"`
	CategoryID      string  `json:"category_id"`
	SellingPrice    float64 `json:"selling_price" binding:"required,gte=0"`
	KDSStation      string  `json:"kds_station"`
	PrepTimeMinutes int     `json:"prep_time_minutes"`
	Description     string  `json:"description"`
	IsActive        bool    `json:"is_active"`
	ActorRole       string  `json:"actor_role"`
}
