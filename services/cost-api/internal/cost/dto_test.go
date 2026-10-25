package cost

import (
	"math"
	"testing"
	"time"
)

func TestCostingResponseCalculatesProfitAndFoodCost(t *testing.T) {
	now := time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC)
	cost := MenuItemCost{MenuItemID: "menu-1", CostAmount: 40, UpdatedAt: now}
	response := cost.ToCostingResponse(100, []*RecipeComponent{{GoodID: "good-1", Quantity: 2, Unit: "kg"}})

	if response.GrossProfit != 60 || math.Abs(response.FoodCostPct-40) > 0.000001 {
		t.Fatalf("unexpected costing response: %+v", response)
	}
	if len(response.Components) != 1 || response.Components[0].GoodID != "good-1" {
		t.Fatalf("components were not mapped: %+v", response.Components)
	}
	if response.LastUpdated != now.Format(time.RFC3339) {
		t.Fatalf("LastUpdated = %q, want %q", response.LastUpdated, now.Format(time.RFC3339))
	}
}
