package inventory
 
import (
	"time"
 
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)
 
// MovementType is the kind of stock movement.
type MovementType string
 
const (
	TypeIn         MovementType = "IN"         // stock receipt
	TypeOut        MovementType = "OUT"        // stock issue / transfer-out
	TypeAdjustment MovementType = "ADJUSTMENT" // controlled adjustment
)
 
func (t MovementType) IsValid() bool {
	switch t {
	case TypeIn, TypeOut, TypeAdjustment:
		return true
	}
	return false
}
 
// RequiresReason reports whether a reason is mandatory for this movement type.
func (t MovementType) RequiresReason() bool {
	return t == TypeOut || t == TypeAdjustment
}
 
// ---------- Adjustment kind & reason code ----------
 
// AdjustmentKind classifies an ADJUSTMENT (shortage/waste, surplus, opening balance).
type AdjustmentKind string
 
const (
	AdjShortage AdjustmentKind = "SHORTAGE" // عجز / هالك  => balance must go down
	AdjSurplus  AdjustmentKind = "SURPLUS"  // فائض جرد   => balance must go up
	AdjOpening  AdjustmentKind = "OPENING"  // تعديل افتتاحي => any direction
)
 
func (k AdjustmentKind) IsValid() bool {
	switch k {
	case AdjShortage, AdjSurplus, AdjOpening:
		return true
	}
	return false
}
 
// ReasonCode is the structured category of the reason (the free text is kept in Reason).
type ReasonCode string
 
const (
	ReasonPrepWaste       ReasonCode = "PREP_WASTE"        // هالك أثناء التحضير
	ReasonExpiredDamaged  ReasonCode = "EXPIRED_DAMAGED"   // تلف وتجاوز الصلاحية
	ReasonEntryError      ReasonCode = "ENTRY_ERROR"       // خطأ في تسجيل الإدخال
	ReasonQualitySample   ReasonCode = "QUALITY_SAMPLE"    // عينة فحص جودة
	ReasonSpillageBreakage ReasonCode = "SPILLAGE_BREAKAGE" // كسر / انسكاب أثناء النقل
	ReasonOther           ReasonCode = "OTHER"
)
 
func (c ReasonCode) IsValid() bool {
	switch c {
	case ReasonPrepWaste, ReasonExpiredDamaged, ReasonEntryError,
		ReasonQualitySample, ReasonSpillageBreakage, ReasonOther:
		return true
	}
	return false
}
 
// ---------- Stock status ----------
 
// StockStatus is derived from quantity vs. the item's limits.
type StockStatus string
 
const (
	StatusCritical  StockStatus = "CRITICAL"  // حرج:   quantity < min_stock
	StatusLow       StockStatus = "LOW"       // منخفض: quantity < reorder_level
	StatusAvailable StockStatus = "AVAILABLE" // متوفر
)
 
func (s StockStatus) IsValid() bool {
	switch s {
	case StatusCritical, StatusLow, StatusAvailable:
		return true
	}
	return false
}
 
// ---------- Roles ----------
 
// Role is the caller role passed by the API Gateway in X-User-Role.
type Role string
 
const (
	RoleCEO           Role = "CEO"
	RoleBranchManager Role = "BRANCH_MANAGER"
)
 
func (r Role) IsValid() bool {
	return r == RoleCEO || r == RoleBranchManager
}
 
// CanAccessAllBranches reports whether the role may act on any branch.
func (r Role) CanAccessAllBranches() bool {
	return r == RoleCEO
}
 
// CanAdjust reports whether the role is authorized to record inventory adjustments.
func (r Role) CanAdjust() bool {
	return r == RoleCEO || r == RoleBranchManager
}
 
// ---------- Inventory ----------
 
// StockSettings are the per-branch limits and location of one good.
type StockSettings struct {
	MinStock     decimal.Decimal
	ReorderLevel decimal.Decimal
	MaxStock     *decimal.Decimal
	LocationCode *string
}
 
// Inventory is the current balance of one good in one branch.
type Inventory struct {
	ID           uuid.UUID        `db:"id"`
	BranchID     uuid.UUID        `db:"branch_id"`
	GoodID       uuid.UUID        `db:"good_id"`
	Quantity     decimal.Decimal  `db:"quantity"`
	UnitCost     decimal.Decimal  `db:"unit_cost"`
	MinStock     decimal.Decimal  `db:"min_stock"`
	ReorderLevel decimal.Decimal  `db:"reorder_level"`
	MaxStock     *decimal.Decimal `db:"max_stock"`
	LocationCode *string          `db:"location_code"`
	CreatedAt    time.Time        `db:"created_at"`
	UpdatedAt    time.Time        `db:"updated_at"`
}
 
// TotalValue returns quantity × unit cost.
func (i Inventory) TotalValue() decimal.Decimal {
	return i.Quantity.Mul(i.UnitCost)
}
 
// isCritical: a safety level of 0 means "not configured", so it never triggers.
func (i Inventory) isCritical() bool {
	return i.MinStock.IsPositive() && i.Quantity.LessThan(i.MinStock)
}
 
// Status mirrors the SQL filter in Repository.ListInventories — keep them in sync.
func (i Inventory) Status() StockStatus {
	if i.isCritical() {
		return StatusCritical
	}
	if i.Quantity.LessThan(decimal.Max(i.MinStock, i.ReorderLevel)) {
		return StatusLow
	}
	return StatusAvailable
}
 
// Shortage is how far the balance is below the safety level (0 if not below).
func (i Inventory) Shortage() decimal.Decimal {
	if i.Quantity.LessThan(i.MinStock) {
		return i.MinStock.Sub(i.Quantity)
	}
	return decimal.Zero
}
 
// UtilizationPct is quantity as a percentage of capacity (nil when no capacity is set).
func (i Inventory) UtilizationPct() *decimal.Decimal {
	if i.MaxStock == nil || !i.MaxStock.IsPositive() {
		return nil
	}
	p := i.Quantity.Div(*i.MaxStock).Mul(decimal.NewFromInt(100))
	return &p
}
 
// ---------- Movements ----------
 
// InventoryMovement is one immutable entry in the stock ledger.
type InventoryMovement struct {
	ID                  uuid.UUID        `db:"id"`
	InventoryID         uuid.UUID        `db:"inventory_id"`
	BranchID            uuid.UUID        `db:"branch_id"`
	GoodID              uuid.UUID        `db:"good_id"`
	Type                MovementType     `db:"type"`
	Quantity            decimal.Decimal  `db:"quantity"`
	QuantityBefore      decimal.Decimal  `db:"quantity_before"`
	QuantityAfter       decimal.Decimal  `db:"quantity_after"`
	UnitCost            *decimal.Decimal `db:"unit_cost"`
	DestinationBranchID *uuid.UUID       `db:"destination_branch_id"`
	Reason              *string          `db:"reason"`
	ReferenceNo         *string          `db:"reference_no"`
	ReasonCode          *ReasonCode      `db:"reason_code"`
	AdjustmentKind      *AdjustmentKind  `db:"adjustment_kind"`
	Counterparty        *string          `db:"counterparty"`
	CreatedBy           uuid.UUID        `db:"created_by"`
	CreatedAt           time.Time        `db:"created_at"`
}
 
// IsTransfer reports whether this OUT movement is a transfer to another branch.
func (m InventoryMovement) IsTransfer() bool {
	return m.Type == TypeOut && m.DestinationBranchID != nil
}
 
// Delta returns the signed change applied to the balance.
func (m InventoryMovement) Delta() decimal.Decimal {
	return m.QuantityAfter.Sub(m.QuantityBefore)
}
 
// FinancialImpact is delta × unit cost at movement time (nil if no cost snapshot).
func (m InventoryMovement) FinancialImpact() *decimal.Decimal {
	if m.UnitCost == nil {
		return nil
	}
	v := m.Delta().Mul(*m.UnitCost)
	return &v
}
 
// TypeAggregate is one row of the movements summary.
type TypeAggregate struct {
	Type     string          `db:"type"`
	Count    int             `db:"count"`
	Quantity decimal.Decimal `db:"quantity"` // signed net change
	Value    decimal.Decimal `db:"value"`    // signed net change × unit cost
}
