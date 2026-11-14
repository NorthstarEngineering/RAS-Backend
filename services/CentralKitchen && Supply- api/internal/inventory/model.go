package inventory

import "time"

// ============================================================
// Inventory — الرصيد الحالي للصنف في الفرع (أو المطبخ المركزي)
// Ref: FR-INV-001..005
// ============================================================

type Inventory struct {
	ID        string    `db:"id" json:"id"`
	BranchID  string    `db:"branch_id" json:"branch_id"`
	GoodID    string    `db:"good_id" json:"good_id"`
	Quantity  float64   `db:"quantity" json:"quantity"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// ============================================================
// MovementType — نوع حركة المخزون
// Ref: FR-INV-002, FR-INV-003
// ============================================================

type MovementType string

const (
	// TypeIn — إضافة مخزون (توريد/استلام)
	TypeIn MovementType = "IN"

	// TypeOut — صرف مخزون (استهلاك/تحويل خارج)
	TypeOut MovementType = "OUT"

	// TypeAdjustment — تسوية جرد (تعديل يدوي)
	TypeAdjustment MovementType = "ADJUSTMENT"
)

// IsValid — يتحقق إن النوع صحيح
func (m MovementType) IsValid() bool {
	return m == TypeIn || m == TypeOut || m == TypeAdjustment
}

// ============================================================
// InventoryMovement — سجل حركة المخزون
// Ref: FR-INV-002, FR-INV-003, NFR-SEC-005 (audit)
// ============================================================

type InventoryMovement struct {
	ID          string       `db:"id" json:"id"`
	InventoryID string       `db:"inventory_id" json:"inventory_id"`
	Type        MovementType `db:"type" json:"type"`
	Quantity    float64      `db:"quantity" json:"quantity"`
	Reason      string       `db:"reason" json:"reason"`
	CreatedBy   string       `db:"created_by" json:"created_by"`
	CreatedAt   time.Time    `db:"created_at" json:"created_at"`
}

// ============================================================
// SupplyRequest - طلب التوريد (من الفرع للمطبخ المركزي)
// Ref: FR-SUP-001, FR-SUP-002
// ============================================================

type SupplyRequest struct {
	ID        string    `db:"id" json:"id"`
	BranchID  string    `db:"branch_id" json:"branch_id"`
	Status    string    `db:"status" json:"status"` // PENDING, APPROVED, SHIPPED, FULFILLED, REJECTED
	CreatedBy string    `db:"created_by" json:"created_by"`
	Notes     *string   `db:"notes" json:"notes,omitempty"`
	Priority  string    `db:"priority" json:"priority"` // NORMAL, URGENT
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`

	Items []*SupplyRequestItem `db:"-" json:"items,omitempty"`
}

type SupplyRequestItem struct {
	ID           string   `db:"id" json:"id"`
	RequestID    string   `db:"request_id" json:"request_id"`
	GoodID       string   `db:"good_id" json:"good_id"`
	RequestedQty float64  `db:"requested_qty" json:"requested_qty"`
	ApprovedQty  *float64 `db:"approved_qty" json:"approved_qty,omitempty"`
	ReceivedQty  *float64 `db:"received_qty" json:"received_qty,omitempty"` // ← موجود

	CreatedAt time.Time `db:"created_at" json:"created_at"`
}
