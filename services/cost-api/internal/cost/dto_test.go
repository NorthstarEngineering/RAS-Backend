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
	if len(response.Components) != 1 || response.Components[0].GoodID != "good-1" || response.LastUpdated != now.Format(time.RFC3339) {
		t.Fatalf("costing response mapping is incorrect: %+v", response)
	}
}

func TestCostingResponseAvoidsDivisionByZero(t *testing.T) {
	cost := MenuItemCost{MenuItemID: "menu-1", CostAmount: 10}
	response := cost.ToCostingResponse(0, nil)
	if response.FoodCostPct != 0 || response.GrossProfit != -10 {
		t.Fatalf("zero-price costing response = %+v", response)
	}
}

func TestCostingResponseSupportsZeroCost(t *testing.T) {
	cost := MenuItemCost{MenuItemID: "menu-1", CostAmount: 0}
	response := cost.ToCostingResponse(25, nil)
	if response.FoodCostPct != 0 || response.GrossProfit != 25 {
		t.Fatalf("zero-cost response = %+v", response)
	}
}

func TestCostingResponsePreservesComponentOrder(t *testing.T) {
	components := []*RecipeComponent{{GoodID: "good-1", Quantity: 1, Unit: "kg"}, {GoodID: "good-2", Quantity: 0.5, Unit: "l"}}
	cost := MenuItemCost{MenuItemID: "menu-1", CostAmount: 5}
	response := cost.ToCostingResponse(10, components)
	if len(response.Components) != 2 || response.Components[0].GoodID != "good-1" || response.Components[1].GoodID != "good-2" {
		t.Fatalf("component order was not preserved: %+v", response.Components)
	}
}

func TestCostingResponseUsesUTCForLastUpdated(t *testing.T) {
	localTime := time.Date(2026, 10, 25, 3, 0, 0, 0, time.FixedZone("test", 3*60*60))
	cost := MenuItemCost{UpdatedAt: localTime}
	response := cost.ToCostingResponse(10, nil)
	if response.LastUpdated != "2026-10-25T00:00:00Z" {
		t.Fatalf("LastUpdated = %q", response.LastUpdated)
	}
}
