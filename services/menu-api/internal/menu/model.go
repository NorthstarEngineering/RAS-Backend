package menu

import "time"

// ============================================================
// Category - فئة المنيو (زي برجر، مشروبات)
// ============================================================
type Category struct {
	ID        string    `db:"id" json:"id"`
	Name      string    `db:"name" json:"name"`
	SortOrder int       `db:"sort_order" json:"sort_order"`
	IsActive  bool      `db:"is_active" json:"is_active"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// ============================================================
// MenuItem - الصنف في المنيو الرئيسي
// Ref: FR-MENU-001, FR-MENU-002
// ============================================================
type MenuItem struct {
	ID              string    `db:"id" json:"id"`
	SKU             string    `db:"sku" json:"sku"`
	Name            string    `db:"name" json:"name"`
	NameEn          *string   `db:"name_en" json:"name_en,omitempty"`
	CategoryID      *string   `db:"category_id" json:"category_id,omitempty"`
	SellingPrice    float64   `db:"selling_price" json:"selling_price"`
	KDSStation      *string   `db:"kds_station" json:"kds_station,omitempty"`
	PrepTimeMinutes *int      `db:"prep_time_minutes" json:"prep_time_minutes,omitempty"`
	Description     *string   `db:"description" json:"description,omitempty"`
	IsActive        bool      `db:"is_active" json:"is_active"`
	CreatedAt       time.Time `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time `db:"updated_at" json:"updated_at"`
}

// ============================================================
// MenuItemRequest - طلب إضافة صنف من الـ Branch Manager
// Ref: FR-MENU-004, FR-MENU-005
// ============================================================
type MenuItemRequest struct {
	ID              string    `db:"id" json:"id"`
	BranchID        string    `db:"branch_id" json:"branch_id"`
	RequestedBy     string    `db:"requested_by" json:"requested_by"`
	ItemName        string    `db:"item_name" json:"item_name"`
	RequestedPrice  float64   `db:"requested_price" json:"requested_price"`
	Description     *string   `db:"description" json:"description,omitempty"`
	Justification   *string   `db:"justification" json:"justification,omitempty"`
	Status          string    `db:"status" json:"status"` // PENDING, APPROVED, REJECTED
	RejectionReason *string   `db:"rejection_reason" json:"rejection_reason,omitempty"`
	CreatedAt       time.Time `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time `db:"updated_at" json:"updated_at"`
}
