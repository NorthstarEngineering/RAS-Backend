package sales

import "time"

// ============================================================
// Requests (اللي بتيجي من الموبايل)
// ============================================================

// OpenShiftRequest - فتح وردية جديدة
type OpenShiftRequest struct {
	BranchID     string  `json:"branch_id" binding:"required,uuid"`
	ManagerID    string  `json:"manager_id" binding:"required,uuid"`
	OpeningFloat float64 `json:"opening_float"`
}

// RecordSaleRequest - تسجيل بيعة (مطعم أو دليفري)
type RecordSaleRequest struct {
	ShiftID string            `json:"shift_id" binding:"required,uuid"`
	Channel string            `json:"channel" binding:"required,oneof=RESTAURANT DELIVERY"`
	Items   []SaleItemRequest `json:"items" binding:"required,min=1,dive"`
}

type SaleItemRequest struct {
	MenuItemID string  `json:"menu_item_id" binding:"required,uuid"`
	Quantity   float64 `json:"quantity" binding:"required,gt=0"`
}

// CreateDiscountRequest - إضافة خصم
type CreateDiscountRequest struct {
	ShiftID string  `json:"shift_id" binding:"required,uuid"`
	Amount  float64 `json:"amount" binding:"required,gt=0"`
	Reason  string  `json:"reason" binding:"required"`
}

// CancelDiscountRequest - إلغاء خصم
type CancelDiscountRequest struct {
	Reason string `json:"reason" binding:"required"`
}

// CloseShiftRequest - إغلاق الوردية
type CloseShiftRequest struct {
	ShiftID string `json:"shift_id" binding:"required,uuid"`
}

// ============================================================
// Responses (اللي هترجع للموبايل)
// ============================================================

type ShiftResponse struct {
	ID           string  `json:"id"`
	BranchID     string  `json:"branch_id"`
	ManagerID    string  `json:"manager_id"`
	Status       string  `json:"status"`
	OpenedAt     string  `json:"opened_at"`
	ClosedAt     *string `json:"closed_at,omitempty"`
	OpeningFloat float64 `json:"opening_float"`
}

type SaleResponse struct {
	ID             string             `json:"id"`
	ShiftID        string             `json:"shift_id"`
	Channel        string             `json:"channel"`
	GrossAmount    float64            `json:"gross_amount"`
	DiscountAmount float64            `json:"discount_amount"`
	NetAmount      float64            `json:"net_amount"`
	Items          []SaleItemResponse `json:"items"`
	CreatedAt      string             `json:"created_at"`
}

type SaleItemResponse struct {
	MenuItemID string  `json:"menu_item_id"`
	Quantity   float64 `json:"quantity"`
	UnitPrice  float64 `json:"unit_price"`
	TotalPrice float64 `json:"total_price"`
}

type DiscountResponse struct {
	ID             string  `json:"id"`
	ShiftID        string  `json:"shift_id"`
	Amount         float64 `json:"amount"`
	Reason         string  `json:"reason"`
	Status         string  `json:"status"`
	CanceledReason *string `json:"canceled_reason,omitempty"`
	CreatedAt      string  `json:"created_at"`
}

// ============================================================
// Mappers (تحويل من Model لـ Response)
// ============================================================

func (s *Shift) ToResponse() ShiftResponse {
	var closedAt *string
	if s.ClosedAt != nil {
		t := s.ClosedAt.Format(time.RFC3339)
		closedAt = &t
	}
	return ShiftResponse{
		ID: s.ID, BranchID: s.BranchID, ManagerID: s.ManagerID,
		Status: s.Status, OpenedAt: s.OpenedAt.Format(time.RFC3339),
		ClosedAt: closedAt, OpeningFloat: s.OpeningFloat,
	}
}

func (s *Sale) ToResponse(items []*SaleItem) SaleResponse {
	var itemResps []SaleItemResponse
	for _, item := range items {
		itemResps = append(itemResps, SaleItemResponse{
			MenuItemID: item.MenuItemID, Quantity: item.Quantity,
			UnitPrice: item.UnitPrice, TotalPrice: item.TotalPrice,
		})
	}
	return SaleResponse{
		ID: s.ID, ShiftID: s.ShiftID, Channel: s.Channel,
		GrossAmount: s.GrossAmount, DiscountAmount: s.DiscountAmount, NetAmount: s.NetAmount,
		Items: itemResps, CreatedAt: s.CreatedAt.Format(time.RFC3339),
	}
}

func (d *Discount) ToResponse() DiscountResponse {
	return DiscountResponse{
		ID: d.ID, ShiftID: d.ShiftID, Amount: d.Amount, Reason: d.Reason,
		Status: d.Status, CanceledReason: d.CanceledReason,
		CreatedAt: d.CreatedAt.Format(time.RFC3339),
	}
}

// ============================================================
// Shift Summary (لشاشات الإغلاق والتقارير)
// ============================================================

type ChannelSummary struct {
	Channel    string  `db:"channel" json:"channel"`
	TotalSales float64 `db:"total_sales" json:"total_sales"`
	OrderCount int     `db:"order_count" json:"order_count"`
}

type ItemSummary struct {
	MenuItemID    string  `db:"menu_item_id" json:"menu_item_id"`
	ItemName      string  `db:"item_name" json:"item_name"`
	TotalQuantity float64 `db:"total_quantity" json:"total_quantity"`
	TotalSales    float64 `db:"total_sales" json:"total_sales"`
}

type ShiftSummaryResponse struct {
	ShiftID        string           `json:"shift_id"`
	Status         string           `json:"status"`
	GrossSales     float64          `json:"gross_sales"`
	TotalDiscounts float64          `json:"total_discounts"`
	NetSales       float64          `json:"net_sales"`
	TotalOrders    int              `json:"total_orders"`
	Channels       []ChannelSummary `json:"channels"`
	Items          []ItemSummary    `json:"items"`
}
