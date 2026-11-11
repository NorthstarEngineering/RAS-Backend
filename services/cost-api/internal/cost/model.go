package cost

import "time"

// ============================================================
// MenuItemCost - التكلفة الإجمالية للصنف
// ============================================================
type MenuItemCost struct {
	MenuItemID string    `db:"menu_item_id" json:"menu_item_id"`
	CostAmount float64   `db:"cost_amount" json:"cost_amount"`
	UpdatedAt  time.Time `db:"updated_at" json:"updated_at"`
}

// ============================================================
// RecipeComponent - مكونات الوصفة (BOM)
// ============================================================
type RecipeComponent struct {
	ID         string    `db:"id" json:"id"`
	MenuItemID string    `db:"menu_item_id" json:"menu_item_id"`
	GoodID     string    `db:"good_id" json:"good_id"`
	Quantity   float64   `db:"quantity" json:"quantity"`
	Unit       string    `db:"unit" json:"unit"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
}
