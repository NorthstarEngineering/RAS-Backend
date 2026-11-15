package inventory

import (
	"time"
)

// ============================================================
// Good — الصنف (منيو المطبخ المركزي)
// Ref: FR-GOODS-001, FR-GOODS-002
// ============================================================

type Good struct {
	ID           string    `db:"id" json:"id"`
	SKU          string    `db:"sku" json:"sku"`
	Name         string    `db:"name" json:"name"`
	Category     string    `db:"category" json:"category"`
	Description  string    `db:"description" json:"description"`
	Unit         string    `db:"unit" json:"unit"`
	UnitCost     float64   `db:"unit_cost" json:"unit_cost"`
	ReorderLevel float64   `db:"reorder_level" json:"reorder_level"`
	MaxCapacity  float64   `db:"max_capacity" json:"max_capacity"`
	IsActive     bool      `db:"is_active" json:"is_active"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at" json:"updated_at"`
}
