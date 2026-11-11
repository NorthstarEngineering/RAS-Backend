package cost

import "time"

// ============================================================
// Requests
// ============================================================
type RecipeComponentRequest struct {
	GoodID   string  `json:"good_id" binding:"required,uuid"`
	Quantity float64 `json:"quantity" binding:"required,gt=0"`
	Unit     string  `json:"unit" binding:"required"`
}

type SetCostRequest struct {
	MenuItemID string                   `json:"menu_item_id" binding:"required,uuid"`
	CostAmount float64                  `json:"cost_amount" binding:"required,gte=0"`
	Components []RecipeComponentRequest `json:"components" binding:"dive"`
}

// ============================================================
// Responses
// ============================================================
type RecipeComponentResponse struct {
	GoodID   string  `json:"good_id"`
	Quantity float64 `json:"quantity"`
	Unit     string  `json:"unit"`
}

type MenuItemCostingResponse struct {
	MenuItemID   string                    `json:"menu_item_id"`
	SellingPrice float64                   `json:"selling_price"`
	CostAmount   float64                   `json:"cost_amount"`
	GrossProfit  float64                   `json:"gross_profit"`
	FoodCostPct  float64                   `json:"food_cost_pct"`
	Components   []RecipeComponentResponse `json:"components"`
	LastUpdated  string                    `json:"last_updated,omitempty"`
}

func (c *MenuItemCost) ToCostingResponse(sellingPrice float64, components []*RecipeComponent) MenuItemCostingResponse {
	grossProfit := sellingPrice - c.CostAmount
	foodCostPct := 0.0
	if sellingPrice > 0 {
		foodCostPct = (c.CostAmount / sellingPrice) * 100
	}

	var compResps []RecipeComponentResponse
	for _, comp := range components {
		compResps = append(compResps, RecipeComponentResponse{
			GoodID: comp.GoodID, Quantity: comp.Quantity, Unit: comp.Unit,
		})
	}

	return MenuItemCostingResponse{
		MenuItemID:   c.MenuItemID,
		SellingPrice: sellingPrice,
		CostAmount:   c.CostAmount,
		GrossProfit:  grossProfit,
		FoodCostPct:  foodCostPct,
		Components:   compResps,
		LastUpdated:  c.UpdatedAt.UTC().Format(time.RFC3339),
	}
}
