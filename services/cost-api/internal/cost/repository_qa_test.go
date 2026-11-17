package cost

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func assertCostArgs(t *testing.T, got []driver.NamedValue, want ...any) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("args=%v, want %v", got, want)
	}
	for i := range want {
		if fmt.Sprint(got[i].Value) != fmt.Sprint(want[i]) {
			t.Fatalf("arg %d=%v, want %v", i, got[i].Value, want[i])
		}
	}
}

func TestCostRepositoryUpsertAndReadPaths(t *testing.T) {
	ctx := context.Background()
	state := &costTxState{}
	state.query = func(q string, _ []driver.NamedValue) (driver.Rows, error) {
		switch {
		case strings.Contains(q, "FROM menu_item_costs"):
			return &costRows{cols: []string{"menu_item_id", "cost_amount", "updated_at"}, data: [][]driver.Value{{"11111111-1111-4111-8111-111111111111", float64(12.5), time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)}}}, nil
		case strings.Contains(q, "SELECT selling_price"):
			return &costRows{cols: []string{"selling_price"}, data: [][]driver.Value{{float64(25)}}}, nil
		case strings.Contains(q, "FROM recipe_components"):
			return &costRows{cols: []string{"id", "menu_item_id", "good_id", "quantity", "unit", "created_at"}, data: [][]driver.Value{{"33333333-3333-4333-8333-333333333333", "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222", float64(2), "kg", time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)}}}, nil
		default:
			return nil, fmt.Errorf("unexpected query: %s", q)
		}
	}
	db := newCostSQLDB(t, state)
	repo := NewRepository(db)
	cost := &MenuItemCost{MenuItemID: "11111111-1111-4111-8111-111111111111", CostAmount: 12.5}
	if err := repo.UpsertCost(ctx, cost); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(state.calls[0].query, "ON CONFLICT (menu_item_id)") {
		t.Fatalf("upsert query=%s", state.calls[0].query)
	}
	assertCostArgs(t, state.calls[0].args, cost.MenuItemID, 12.5)
	gotCost, err := repo.GetCostByMenuItemID(ctx, cost.MenuItemID)
	if err != nil || gotCost == nil || gotCost.CostAmount != 12.5 {
		t.Fatalf("cost=(%+v,%v)", gotCost, err)
	}
	price, err := repo.GetMenuItemPrice(ctx, cost.MenuItemID)
	if err != nil || price != 25 {
		t.Fatalf("price=(%v,%v)", price, err)
	}
	components, err := repo.GetComponentsByMenuItem(ctx, cost.MenuItemID)
	if err != nil || len(components) != 1 || components[0].Quantity != 2 {
		t.Fatalf("components=(%+v,%v)", components, err)
	}
}

func TestCostRepositoryNoRowsAndQueryErrors(t *testing.T) {
	ctx := context.Background()
	state := &costTxState{query: func(q string, _ []driver.NamedValue) (driver.Rows, error) {
		if strings.Contains(q, "FROM menu_item_costs") {
			return &costRows{cols: []string{"menu_item_id", "cost_amount", "updated_at"}}, nil
		}
		return &costRows{cols: []string{"selling_price"}}, nil
	}}
	repo := NewRepository(newCostSQLDB(t, state))
	cost, err := repo.GetCostByMenuItemID(ctx, "missing")
	if err != nil || cost != nil {
		t.Fatalf("missing cost=(%+v,%v), want nil,nil", cost, err)
	}
	if _, err = repo.GetMenuItemPrice(ctx, "missing"); err == nil || !strings.Contains(err.Error(), "menu item not found") {
		t.Fatalf("missing menu item error=%v", err)
	}
	state.queryErr = errors.New("database unavailable")
	if _, err = repo.GetCostByMenuItemID(ctx, "id"); err == nil {
		t.Fatal("expected repository query error")
	}
	state.execErr = errors.New("write unavailable")
	if err = repo.UpsertCost(ctx, &MenuItemCost{MenuItemID: "id"}); err == nil {
		t.Fatal("expected repository write error")
	}
}

func TestCostRepositoryComponentWritesUseTransaction(t *testing.T) {
	state := &costTxState{}
	db := newCostSQLDB(t, state)
	repo := NewRepository(db)
	tx, err := db.BeginTxx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	component := &RecipeComponent{ID: "33333333-3333-4333-8333-333333333333", MenuItemID: "11111111-1111-4111-8111-111111111111", GoodID: "22222222-2222-4222-8222-222222222222", Quantity: 1, Unit: "ea"}
	if err = repo.DeleteComponentsByMenuItem(context.Background(), tx, component.MenuItemID); err != nil {
		t.Fatal(err)
	}
	if err = repo.CreateComponent(context.Background(), tx, component); err != nil {
		t.Fatal(err)
	}
	if len(state.calls) != 2 || !strings.Contains(state.calls[0].query, "DELETE FROM recipe_components") || !strings.Contains(state.calls[1].query, "INSERT INTO recipe_components") {
		t.Fatalf("transaction calls=%+v", state.calls)
	}
	assertCostArgs(t, state.calls[0].args, component.MenuItemID)
	assertCostArgs(t, state.calls[1].args, component.ID, component.MenuItemID, component.GoodID, 1, "ea")
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

func TestSetCostCostWriteUsesComponentTransaction(t *testing.T) {
	state := &costTxState{failComponentInsert: true}
	db := newCostSQLDB(t, state)
	db.SetMaxOpenConns(2)
	repo := NewRepository(db)
	err := NewService(db, repo).SetCost(context.Background(), SetCostRequest{
		MenuItemID: "11111111-1111-4111-8111-111111111111",
		CostAmount: 12,
		Components: []RecipeComponentRequest{{GoodID: "22222222-2222-4222-8222-222222222222", Quantity: 1, Unit: "ea"}},
	})
	if err == nil || !strings.Contains(err.Error(), "component insert failed") {
		t.Fatalf("error=%v, want component failure", err)
	}
	if state.rollbacks != 1 {
		t.Fatalf("rollback count=%d, want 1", state.rollbacks)
	}
	if state.upsertConnID != state.txConnID {
		t.Fatalf("cost upsert used connection %d while components used transaction connection %d; cost is outside the transaction", state.upsertConnID, state.txConnID)
	}
}

func TestCostReportRepositoriesMapAggregatesAndFilters(t *testing.T) {
	state := &costTxState{}
	state.query = func(q string, _ []driver.NamedValue) (driver.Rows, error) {
		switch {
		case strings.Contains(q, "FROM sales s") && strings.Contains(q, "sale_items"):
			return &costRows{cols: []string{"total_revenue", "total_cogs", "gross_profit"}, data: [][]driver.Value{{float64(100), float64(35), float64(65)}}}, nil
		case strings.Contains(q, "FROM sale_items si"):
			return &costRows{cols: []string{"total_cogs"}, data: [][]driver.Value{{float64(35)}}}, nil
		case strings.Contains(q, "FROM employees e"):
			return &costRows{cols: []string{"labor_cost"}, data: [][]driver.Value{{float64(20)}}}, nil
		default:
			return nil, fmt.Errorf("unexpected query: %s", q)
		}
	}
	repo := NewRepository(newCostSQLDB(t, state))
	f := ReportFilter{BranchID: "branch-1", From: "2026-01-01", To: "2026-01-31"}
	cogs, err := repo.GetCOGSReport(context.Background(), f)
	if err != nil || cogs.TotalCOGS != 35 || cogs.From != f.From || cogs.To != f.To || cogs.BranchID != f.BranchID {
		t.Fatalf("COGS=(%+v,%v)", cogs, err)
	}
	assertCostArgs(t, state.calls[0].args, f.BranchID, f.From, f.To)
	gp, err := repo.GetGrossProfitReport(context.Background(), f)
	if err != nil || gp.TotalRevenue != 100 || gp.TotalCOGS != 35 || gp.GrossProfit != 65 {
		t.Fatalf("gross profit=(%+v,%v)", gp, err)
	}
	contribution, err := repo.GetEstimatedContribution(context.Background(), f)
	if err != nil || contribution.GrossProfit != 65 || contribution.LaborCost != 20 || contribution.EstimatedContribution != 45 {
		t.Fatalf("contribution=(%+v,%v)", contribution, err)
	}
	laborCall := state.calls[len(state.calls)-1]
	if !strings.Contains(laborCall.query, "e.branch_id") {
		t.Fatalf("labor query lacks branch filter: %s", laborCall.query)
	}
}

func TestCostReportRepositoriesPropagateQueryErrors(t *testing.T) {
	ctx := context.Background()
	f := ReportFilter{BranchID: "branch-1", From: "2026-01-01", To: "2026-01-31"}
	state := &costTxState{queryErr: errors.New("report query failed")}
	repo := NewRepository(newCostSQLDB(t, state))
	if _, err := repo.GetCOGSReport(ctx, f); err == nil {
		t.Fatal("expected COGS query error")
	}
	if _, err := repo.GetGrossProfitReport(ctx, f); err == nil {
		t.Fatal("expected gross profit query error")
	}
	if _, err := repo.GetEstimatedContribution(ctx, f); err == nil {
		t.Fatal("expected gross profit error from contribution report")
	}

	laborState := &costTxState{query: func(q string, _ []driver.NamedValue) (driver.Rows, error) {
		if strings.Contains(q, "FROM employees e") {
			return nil, errors.New("labor query failed")
		}
		return &costRows{cols: []string{"total_revenue", "total_cogs", "gross_profit"}, data: [][]driver.Value{{float64(10), float64(3), float64(7)}}}, nil
	}}
	if _, err := NewRepository(newCostSQLDB(t, laborState)).GetEstimatedContribution(ctx, f); err == nil || !strings.Contains(err.Error(), "labor query failed") {
		t.Fatalf("labor query error=%v", err)
	}
}

func TestEstimatedContributionLaborQueryFiltersSelectedPeriod(t *testing.T) {
	state := &costTxState{}
	state.query = func(q string, _ []driver.NamedValue) (driver.Rows, error) {
		if strings.Contains(q, "FROM employees e") {
			return &costRows{cols: []string{"labor_cost"}, data: [][]driver.Value{{float64(20)}}}, nil
		}
		return &costRows{cols: []string{"total_revenue", "total_cogs", "gross_profit"}, data: [][]driver.Value{{float64(100), float64(35), float64(65)}}}, nil
	}
	repo := NewRepository(newCostSQLDB(t, state))
	_, err := repo.GetEstimatedContribution(context.Background(), ReportFilter{BranchID: "branch-1", From: "2026-01-01", To: "2026-01-31"})
	if err != nil {
		t.Fatal(err)
	}
	laborQuery := state.calls[len(state.calls)-1].query
	if !strings.Contains(laborQuery, "work_date >=") || !strings.Contains(laborQuery, "work_date <=") {
		t.Fatalf("labor query does not apply the selected report period: %s", laborQuery)
	}
}
