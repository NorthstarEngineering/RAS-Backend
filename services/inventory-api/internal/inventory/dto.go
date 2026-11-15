package inventory
 
import (
	"strings"
	"time"
 
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)
 
// ---------- Identity ----------
 
// Actor is the caller identity extracted from Gateway headers.
type Actor struct {
	UserID   uuid.UUID
	Role     Role
	BranchID uuid.UUID
}
 
// ---------- Requests ----------
 
// CreateMovementRequest is the body of POST /api/v1/inventory/movements.
//
// Quantity semantics:
//   - IN / OUT:   quantity = amount moved (> 0)
//   - ADJUSTMENT: send EITHER quantity (the resulting balance)
//     OR delta (signed change, e.g. -3). delta is safer when the screen
//     was opened a while ago; pair it with expected_quantity_before.
type CreateMovementRequest struct {
	BranchID               *uuid.UUID       `json:"branch_id"` // CEO only; ignored for BRANCH_MANAGER
	GoodID                 uuid.UUID        `json:"good_id" binding:"required"`
	Type                   MovementType     `json:"type" binding:"required"`
	Quantity               *decimal.Decimal `json:"quantity"`
	Delta                  *decimal.Decimal `json:"delta"`                    // ADJUSTMENT only
	ExpectedQuantityBefore *decimal.Decimal `json:"expected_quantity_before"` // optional optimistic check (any type)
	UnitCost               *decimal.Decimal `json:"unit_cost"`                // IN only, optional
	DestinationBranchID    *uuid.UUID       `json:"destination_branch_id"`    // OUT only => transfer-out
	Counterparty           *string          `json:"counterparty"`             // department / supplier / vehicle
	Reason                 *string          `json:"reason"`                   // required for OUT and ADJUSTMENT
	ReasonCode             *ReasonCode      `json:"reason_code"`              // OUT / ADJUSTMENT
	AdjustmentKind         *AdjustmentKind  `json:"adjustment_kind"`          // ADJUSTMENT only
}
 
// Normalize trims text fields, uppercases codes and turns blank strings into nil.
func (r *CreateMovementRequest) Normalize() {
	r.Type = MovementType(strings.ToUpper(strings.TrimSpace(string(r.Type))))
	r.Reason = trimPtr(r.Reason)
	r.Counterparty = trimPtr(r.Counterparty)
 
	if r.ReasonCode != nil {
		c := ReasonCode(strings.ToUpper(strings.TrimSpace(string(*r.ReasonCode))))
		if c == "" {
			r.ReasonCode = nil
		} else {
			r.ReasonCode = &c
		}
	}
	if r.AdjustmentKind != nil {
		k := AdjustmentKind(strings.ToUpper(strings.TrimSpace(string(*r.AdjustmentKind))))
		if k == "" {
			r.AdjustmentKind = nil
		} else {
			r.AdjustmentKind = &k
		}
	}
}
 
// StockSettingsRequest is the body of PATCH /api/v1/inventory/items/:good_id/settings.
// Only the fields that are sent are changed.
//   - max_stock: 0 clears the capacity.
//   - location_code: "" clears the location.
type StockSettingsRequest struct {
	MinStock     *decimal.Decimal `json:"min_stock"`
	ReorderLevel *decimal.Decimal `json:"reorder_level"`
	MaxStock     *decimal.Decimal `json:"max_stock"`
	LocationCode *string          `json:"location_code"`
}
 
func (r *StockSettingsRequest) Normalize() {
	if r.LocationCode != nil {
		t := strings.TrimSpace(*r.LocationCode)
		r.LocationCode = &t // keep the pointer: "" means "clear"
	}
}
 
func trimPtr(p *string) *string {
	if p == nil {
		return nil
	}
	t := strings.TrimSpace(*p)
	if t == "" {
		return nil
	}
	return &t
}
 
// InventoryQuery is the parsed query for GET /api/v1/inventory.
type InventoryQuery struct {
	Search   string       // name / SKU / barcode / category (resolved through the catalog)
	Category string       // exact category
	Status   *StockStatus // CRITICAL | LOW | AVAILABLE
	Page     int
	Limit    int
}
 
// InventoryFilter is what the repository receives.
type InventoryFilter struct {
	BranchID      uuid.UUID
	FilterByGoods bool // when true, only GoodIDs are returned (an empty list matches nothing)
	GoodIDs       []uuid.UUID
	Status        *StockStatus
	Page          int
	Limit         int
}
 
// MovementFilter is the parsed query for GET /api/v1/inventory/movements.
type MovementFilter struct {
	BranchID       uuid.UUID
	GoodID         *uuid.UUID
	Type           *MovementType
	From           *time.Time
	To             *time.Time
	Search         string      // reference number (partial) and, via the catalog, good name / SKU
	SearchGoodIDs  []uuid.UUID // filled by the service
	Page           int
	Limit          int
}
 
const (
	DefaultPage  = 1
	DefaultLimit = 20
	MaxLimit     = 100
)
 
// NormalizePaging applies defaults and bounds to page/limit.
func NormalizePaging(page, limit int) (int, int) {
	if page < 1 {
		page = DefaultPage
	}
	if limit < 1 {
		limit = DefaultLimit
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}
	return page, limit
}
 
// ---------- Responses ----------
 
// GoodSummary is catalog data attached to responses (nil when the catalog has no answer).
type GoodSummary struct {
	Name     string `json:"name"`
	SKU      string `json:"sku"`
	Category string `json:"category"`
	Unit     string `json:"unit"`
	Barcode  string `json:"barcode,omitempty"`
}
 
type InventoryResponse struct {
	InventoryID    uuid.UUID    `json:"inventory_id"`
	BranchID       uuid.UUID    `json:"branch_id"`
	GoodID         uuid.UUID    `json:"good_id"`
	Good           *GoodSummary `json:"good"`
	Quantity       string       `json:"quantity"`
	UnitCost       string       `json:"unit_cost"`
	Value          string       `json:"value"`
	Status         StockStatus  `json:"status"`
	MinStock       string       `json:"min_stock"`
	ReorderLevel   string       `json:"reorder_level"`
	MaxStock       *string      `json:"max_stock"`
	Shortage       string       `json:"shortage"`        // how far below min_stock (0 if not)
	UtilizationPct *string      `json:"utilization_pct"` // quantity / max_stock × 100
	LocationCode   *string      `json:"location_code"`
	UpdatedAt      time.Time    `json:"updated_at"`
}
 
type MovementResponse struct {
	ID                  uuid.UUID       `json:"id"`
	ReferenceNo         *string         `json:"reference_no"`
	InventoryID         uuid.UUID       `json:"inventory_id"`
	BranchID            uuid.UUID       `json:"branch_id"`
	GoodID              uuid.UUID       `json:"good_id"`
	Good                *GoodSummary    `json:"good"`
	Type                MovementType    `json:"type"`
	IsTransfer          bool            `json:"is_transfer"`
	Quantity            string          `json:"quantity"`
	QuantityBefore      string          `json:"quantity_before"`
	QuantityAfter       string          `json:"quantity_after"`
	Delta               string          `json:"delta"`
	UnitCost            *string         `json:"unit_cost"`
	FinancialImpact     *string         `json:"financial_impact"` // delta × unit_cost
	Counterparty        *string         `json:"counterparty"`
	DestinationBranchID *uuid.UUID      `json:"destination_branch_id"`
	Reason              *string         `json:"reason"`
	ReasonCode          *string         `json:"reason_code"`
	AdjustmentKind      *string         `json:"adjustment_kind"`
	CreatedBy           uuid.UUID       `json:"created_by"`
	CreatedByName       *string         `json:"created_by_name"`
	CreatedAt           time.Time       `json:"created_at"`
}
 
type CreateMovementResponse struct {
	Inventory InventoryResponse `json:"inventory"`
	Movement  MovementResponse  `json:"movement"`
}
 
type InventoryItemResponse struct {
	InventoryResponse
	AvgDailyConsumption *string            `json:"avg_daily_consumption"` // last 30 days, non-transfer OUT
	DaysOfCover         *string            `json:"days_of_cover"`
	EstimatedStockoutAt *time.Time         `json:"estimated_stockout_at"`
	LastReceipt         *MovementResponse  `json:"last_receipt"`
	RecentMovements     []MovementResponse `json:"recent_movements"`
}
 
// ---- inventory summary (list screen cards + chips) ----
 
type StatusCounts struct {
	Critical  int `json:"critical"`
	Low       int `json:"low"`
	Available int `json:"available"`
}
 
type CategoryCount struct {
	Category string `json:"category"`
	Count    int    `json:"count"`
}
 
type InventorySummaryResponse struct {
	BranchID        uuid.UUID       `json:"branch_id"`
	TotalItems      int             `json:"total_items"`
	CategoriesCount int             `json:"categories_count"`
	Statuses        StatusCounts    `json:"statuses"`
	Categories      []CategoryCount `json:"categories"`
	BookValue       string          `json:"book_value"`
	LastUpdatedAt   *time.Time      `json:"last_updated_at"`
	GeneratedAt     time.Time       `json:"generated_at"`
}
 
// ---- movements summary ----
 
type MovementTypeSummary struct {
	Count       int    `json:"count"`
	NetQuantity string `json:"net_quantity"` // signed; mixes units (kg + L + pcs)
	NetValue    string `json:"net_value"`    // signed; delta × unit cost, safe to add up
}
 
type MovementSummaryResponse struct {
	BranchID   uuid.UUID           `json:"branch_id"`
	TotalCount int                 `json:"total_count"`
	In         MovementTypeSummary `json:"in"`
	Out        MovementTypeSummary `json:"out"`
	Adjustment MovementTypeSummary `json:"adjustment"`
	From       *time.Time          `json:"from"`
	To         *time.Time          `json:"to"`
}
 
// ---- valuation ----
 
type InventoryValueItem struct {
	InventoryID uuid.UUID    `json:"inventory_id"`
	GoodID      uuid.UUID    `json:"good_id"`
	Good        *GoodSummary `json:"good"`
	Status      StockStatus  `json:"status"`
	Quantity    string       `json:"quantity"`
	UnitCost    string       `json:"unit_cost"`
	Value       string       `json:"value"`
}
 
type CategoryValue struct {
	Category   string `json:"category"`
	ItemsCount int    `json:"items_count"`
	Quantity   string `json:"quantity"`
	Value      string `json:"value"`
	SharePct   string `json:"share_pct"`
}
 
type UnitQuantity struct {
	Unit     string `json:"unit"`
	Quantity string `json:"quantity"`
}
 
type InventoryValueResponse struct {
	BranchID       uuid.UUID            `json:"branch_id"`
	ItemsCount     int                  `json:"items_count"`
	TotalQuantity  string               `json:"total_quantity"` // deprecated: mixes units, use quantity_by_unit
	QuantityByUnit []UnitQuantity       `json:"quantity_by_unit"`
	TotalValue     string               `json:"total_value"`
	ByCategory     []CategoryValue      `json:"by_category"`
	Items          []InventoryValueItem `json:"items"` // sorted by value desc; limited by ?top=
	ValuedAt       time.Time            `json:"valued_at"`
}
 
// ---- shared ----
 
type Pagination struct {
	Page  int `json:"page"`
	Limit int `json:"limit"`
	Total int `json:"total"`
}
 
type PageResponse[T any] struct {
	Data       []T        `json:"data"`
	Pagination Pagination `json:"pagination"`
}
 
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
 
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}
 
// ---------- Mappers ----------
 
const moneyScale = 2
 
func enumPtr[T ~string](p *T) *string {
	if p == nil {
		return nil
	}
	s := string(*p)
	return &s
}
 
func fixedPtr(d *decimal.Decimal, scale int32) *string {
	if d == nil {
		return nil
	}
	s := d.StringFixed(scale)
	return &s
}
 
func ToInventoryResponse(inv Inventory) InventoryResponse {
	return InventoryResponse{
		InventoryID:    inv.ID,
		BranchID:       inv.BranchID,
		GoodID:         inv.GoodID,
		Quantity:       inv.Quantity.StringFixed(moneyScale),
		UnitCost:       inv.UnitCost.StringFixed(moneyScale),
		Value:          inv.TotalValue().StringFixed(moneyScale),
		Status:         inv.Status(),
		MinStock:       inv.MinStock.StringFixed(moneyScale),
		ReorderLevel:   inv.ReorderLevel.StringFixed(moneyScale),
		MaxStock:       fixedPtr(inv.MaxStock, moneyScale),
		Shortage:       inv.Shortage().StringFixed(moneyScale),
		UtilizationPct: fixedPtr(inv.UtilizationPct(), 1),
		LocationCode:   inv.LocationCode,
		UpdatedAt:      inv.UpdatedAt,
	}
}
 
func ToMovementResponse(m InventoryMovement) MovementResponse {
	return MovementResponse{
		ID:                  m.ID,
		ReferenceNo:         m.ReferenceNo,
		InventoryID:         m.InventoryID,
		BranchID:            m.BranchID,
		GoodID:              m.GoodID,
		Type:                m.Type,
		IsTransfer:          m.IsTransfer(),
		Quantity:            m.Quantity.StringFixed(moneyScale),
		QuantityBefore:      m.QuantityBefore.StringFixed(moneyScale),
		QuantityAfter:       m.QuantityAfter.StringFixed(moneyScale),
		Delta:               m.Delta().StringFixed(moneyScale),
		UnitCost:            fixedPtr(m.UnitCost, moneyScale),
		FinancialImpact:     fixedPtr(m.FinancialImpact(), moneyScale),
		Counterparty:        m.Counterparty,
		DestinationBranchID: m.DestinationBranchID,
		Reason:              m.Reason,
		ReasonCode:          enumPtr(m.ReasonCode),
		AdjustmentKind:      enumPtr(m.AdjustmentKind),
		CreatedBy:           m.CreatedBy,
		CreatedAt:           m.CreatedAt,
	}
}
 





















