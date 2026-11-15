package sales

import "time"

// ============================================================
// Shift - الوردية (SRS 4.15)
// ============================================================
type Shift struct {
	ID           string     `db:"id" json:"id"`
	BranchID     string     `db:"branch_id" json:"branch_id"`
	ManagerID    string     `db:"manager_id" json:"manager_id"`
	Status       string     `db:"status" json:"status"` // OPEN, REVIEW, CLOSED
	OpenedAt     time.Time  `db:"opened_at" json:"opened_at"`
	ClosedAt     *time.Time `db:"closed_at" json:"closed_at,omitempty"`
	OpeningFloat float64    `db:"opening_float" json:"opening_float"`
	CreatedAt    time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at" json:"updated_at"`
}

// ============================================================
// Sale - حركة البيع (SRS 4.14)
// ============================================================
type Sale struct {
	ID             string    `db:"id" json:"id"`
	ShiftID        string    `db:"shift_id" json:"shift_id"`
	BranchID       string    `db:"branch_id" json:"branch_id"`
	Channel        string    `db:"channel" json:"channel"` // RESTAURANT, DELIVERY
	GrossAmount    float64   `db:"gross_amount" json:"gross_amount"`
	DiscountAmount float64   `db:"discount_amount" json:"discount_amount"`
	NetAmount      float64   `db:"net_amount" json:"net_amount"`
	CreatedBy      string    `db:"created_by" json:"created_by"`
	CreatedAt      time.Time `db:"created_at" json:"created_at"`
}

// ============================================================
// SaleItem - تفاصيل الصنف المباع (SRS 4.14 - Historical Price)
// ============================================================
type SaleItem struct {
	ID         string    `db:"id" json:"id"`
	SaleID     string    `db:"sale_id" json:"sale_id"`
	MenuItemID string    `db:"menu_item_id" json:"menu_item_id"`
	Quantity   float64   `db:"quantity" json:"quantity"`
	UnitPrice  float64   `db:"unit_price" json:"unit_price"` // السعر التاريخي وقت البيع
	TotalPrice float64   `db:"total_price" json:"total_price"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
}

// ============================================================
// Discount - الخصم (SRS 4.16)
// ============================================================
type Discount struct {
	ID             string    `db:"id" json:"id"`
	ShiftID        string    `db:"shift_id" json:"shift_id"`
	Amount         float64   `db:"amount" json:"amount"`
	Reason         string    `db:"reason" json:"reason"`
	Status         string    `db:"status" json:"status"` // ACTIVE, CANCELED
	CanceledReason *string   `db:"canceled_reason" json:"canceled_reason,omitempty"`
	CanceledBy     *string   `db:"canceled_by" json:"canceled_by,omitempty"`
	CreatedBy      string    `db:"created_by" json:"created_by"`
	CreatedAt      time.Time `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time `db:"updated_at" json:"updated_at"`
}
