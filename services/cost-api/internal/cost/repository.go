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
