package cost

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
)

type Repository interface {
	UpsertCost(ctx context.Context, c *MenuItemCost) error
	GetCostByMenuItemID(ctx context.Context, menuItemID string) (*MenuItemCost, error)
	GetMenuItemPrice(ctx context.Context, menuItemID string) (float64, error)
	// BOM
	CreateComponent(ctx context.Context, tx *sqlx.Tx, comp *RecipeComponent) error
	DeleteComponentsByMenuItem(ctx context.Context, tx *sqlx.Tx, menuItemID string) error
	GetComponentsByMenuItem(ctx context.Context, menuItemID string) ([]*RecipeComponent, error)
	// Reports
	GetCOGSReport(ctx context.Context, f ReportFilter) (*COGSReportResponse, error)
	GetGrossProfitReport(ctx context.Context, f ReportFilter) (*GrossProfitReportResponse, error)
	GetEstimatedContribution(ctx context.Context, f ReportFilter) (*EstimatedContributionResponse, error)
}

type postgresRepo struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &postgresRepo{db: db}
}

func (r *postgresRepo) UpsertCost(ctx context.Context, c *MenuItemCost) error {
	const q = `
		INSERT INTO menu_item_costs (menu_item_id, cost_amount, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (menu_item_id) 
		DO UPDATE SET cost_amount = EXCLUDED.cost_amount, updated_at = NOW()
	`
	_, err := r.db.ExecContext(ctx, q, c.MenuItemID, c.CostAmount)
	return err
}

func (r *postgresRepo) GetCostByMenuItemID(ctx context.Context, menuItemID string) (*MenuItemCost, error) {
	const q = `SELECT menu_item_id, cost_amount, updated_at FROM menu_item_costs WHERE menu_item_id = $1`
	var c MenuItemCost
	err := r.db.GetContext(ctx, &c, q, menuItemID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &c, err
}

func (r *postgresRepo) GetMenuItemPrice(ctx context.Context, menuItemID string) (float64, error) {
	const q = `SELECT selling_price FROM menu_items WHERE id = $1`
	var price float64
	err := r.db.GetContext(ctx, &price, q, menuItemID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("menu item not found")
	}
	return price, err
}

// --- BOM Methods ---

func (r *postgresRepo) CreateComponent(ctx context.Context, tx *sqlx.Tx, comp *RecipeComponent) error {
	const q = `
		INSERT INTO recipe_components (id, menu_item_id, good_id, quantity, unit, created_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
	`
	_, err := tx.ExecContext(ctx, q, comp.ID, comp.MenuItemID, comp.GoodID, comp.Quantity, comp.Unit)
	return err
}

func (r *postgresRepo) DeleteComponentsByMenuItem(ctx context.Context, tx *sqlx.Tx, menuItemID string) error {
	_, err := tx.ExecContext(ctx, `DELETE FROM recipe_components WHERE menu_item_id = $1`, menuItemID)
	return err
}

func (r *postgresRepo) GetComponentsByMenuItem(ctx context.Context, menuItemID string) ([]*RecipeComponent, error) {
	const q = `SELECT id, menu_item_id, good_id, quantity, unit, created_at FROM recipe_components WHERE menu_item_id = $1`
	var comps []*RecipeComponent
	err := r.db.SelectContext(ctx, &comps, q, menuItemID)
	return comps, err
}

// ============================================================
// Reports Queries
// ============================================================

func (r *postgresRepo) GetCOGSReport(ctx context.Context, f ReportFilter) (*COGSReportResponse, error) {
	query := `
		SELECT COALESCE(SUM(si.quantity * c.cost_amount), 0) as total_cogs
		FROM sale_items si
		JOIN sales s ON si.sale_id = s.id
		LEFT JOIN menu_item_costs c ON si.menu_item_id = c.menu_item_id
		WHERE 1=1`

	var args []interface{}
	argIndex := 1

	if f.BranchID != "" {
		query += fmt.Sprintf(" AND s.branch_id = $%d", argIndex)
		args = append(args, f.BranchID)
		argIndex++
	}
	if f.From != "" {
		query += fmt.Sprintf(" AND s.created_at >= $%d", argIndex)
		args = append(args, f.From)
		argIndex++
	}
	if f.To != "" {
		query += fmt.Sprintf(" AND s.created_at <= $%d", argIndex)
		args = append(args, f.To)
	}

	var result COGSReportResponse
	err := r.db.GetContext(ctx, &result, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get cogs report: %w", err)
	}
	result.From = f.From
	result.To = f.To
	result.BranchID = f.BranchID
	return &result, nil
}

func (r *postgresRepo) GetGrossProfitReport(ctx context.Context, f ReportFilter) (*GrossProfitReportResponse, error) {
	query := `
		SELECT 
			COALESCE(SUM(s.net_amount), 0) as total_revenue,
			COALESCE(SUM(si.quantity * c.cost_amount), 0) as total_cogs,
			COALESCE(SUM(s.net_amount) - SUM(si.quantity * c.cost_amount), 0) as gross_profit
		FROM sales s
		LEFT JOIN sale_items si ON s.id = si.sale_id
		LEFT JOIN menu_item_costs c ON si.menu_item_id = c.menu_item_id
		WHERE 1=1`

	var args []interface{}
	argIndex := 1

	if f.BranchID != "" {
		query += fmt.Sprintf(" AND s.branch_id = $%d", argIndex)
		args = append(args, f.BranchID)
		argIndex++
	}
	if f.From != "" {
		query += fmt.Sprintf(" AND s.created_at >= $%d", argIndex)
		args = append(args, f.From)
		argIndex++
	}
	if f.To != "" {
		query += fmt.Sprintf(" AND s.created_at <= $%d", argIndex)
		args = append(args, f.To)
	}

	var result GrossProfitReportResponse
	err := r.db.GetContext(ctx, &result, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get gross profit report: %w", err)
	}
	return &result, nil
}

func (r *postgresRepo) GetEstimatedContribution(ctx context.Context, f ReportFilter) (*EstimatedContributionResponse, error) {
	// 1. Gross Profit
	gpReport, err := r.GetGrossProfitReport(ctx, f)
	if err != nil {
		return nil, err
	}

	// 2. Labor Cost
	laborQuery := `
		SELECT COALESCE(SUM(
			CASE 
				WHEN e.payment_type = 'daily' THEN e.payment_amount
				WHEN e.payment_type = 'monthly' THEN e.payment_amount / 30
				ELSE 0
			END
		), 0) as labor_cost
		FROM employees e
		WHERE e.active = TRUE`

	var args []interface{}
	argIndex := 1
	if f.BranchID != "" {
		laborQuery += fmt.Sprintf(" AND e.branch_id = $%d", argIndex)
		args = append(args, f.BranchID)
	}

	var laborCost float64
	err = r.db.GetContext(ctx, &laborCost, laborQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("get labor cost: %w", err)
	}

	// 3. Estimated Contribution
	estimatedContribution := gpReport.GrossProfit - laborCost

	return &EstimatedContributionResponse{
		GrossProfit:           gpReport.GrossProfit,
		LaborCost:             laborCost,
		EstimatedContribution: estimatedContribution,
	}, nil
}
