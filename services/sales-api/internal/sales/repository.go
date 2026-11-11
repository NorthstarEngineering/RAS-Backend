package sales

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// ============================================================
// Repository Interface (عشان نسهل عمل الـ Mocking لو احتجنا)
// ============================================================

type Repository interface {
	// Shifts
	CreateShift(ctx context.Context, tx *sqlx.Tx, shift *Shift) error
	GetShiftByID(ctx context.Context, id string) (*Shift, error)
	UpdateShiftStatus(ctx context.Context, tx *sqlx.Tx, id, status string) error

	// Sales
	CreateSale(ctx context.Context, tx *sqlx.Tx, sale *Sale) error
	CreateSaleItem(ctx context.Context, tx *sqlx.Tx, item *SaleItem) error
	GetShiftSalesTotal(ctx context.Context, shiftID string) (float64, float64, error) // Gross, Discount

	// Discounts
	CreateDiscount(ctx context.Context, tx *sqlx.Tx, discount *Discount) error
	GetDiscountByID(ctx context.Context, id string) (*Discount, error)
	CancelDiscount(ctx context.Context, tx *sqlx.Tx, id, reason, actorID string) error

	// Menu (للحصول على السعر التاريخي)
	GetMenuItemPrice(ctx context.Context, id string) (float64, error)
	// Shift Summary
	GetShiftChannelSummary(ctx context.Context, shiftID string) ([]ChannelSummary, error)
	GetShiftItemSummary(ctx context.Context, shiftID string) ([]ItemSummary, error)
}

// ============================================================
// PostgreSQL Implementation
// ============================================================

type postgresRepo struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &postgresRepo{db: db}
}

// ============================================================
// Shifts Queries
// ============================================================

func (r *postgresRepo) CreateShift(ctx context.Context, tx *sqlx.Tx, shift *Shift) error {
	const q = `
		INSERT INTO shifts (id, branch_id, manager_id, status, opened_at, opening_float, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := tx.ExecContext(ctx, q, shift.ID, shift.BranchID, shift.ManagerID, shift.Status, shift.OpenedAt, shift.OpeningFloat, shift.CreatedAt, shift.UpdatedAt)
	return err
}

func (r *postgresRepo) GetShiftByID(ctx context.Context, id string) (*Shift, error) {
	const q = `
		SELECT id, branch_id, manager_id, status, opened_at, closed_at, opening_float, created_at, updated_at
		FROM shifts WHERE id = $1
	`
	var shift Shift
	err := r.db.GetContext(ctx, &shift, q, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get shift: %w", err)
	}
	return &shift, nil
}

func (r *postgresRepo) UpdateShiftStatus(ctx context.Context, tx *sqlx.Tx, id, status string) error {
	const q = `UPDATE shifts SET status = $2, updated_at = NOW() WHERE id = $1`
	_, err := tx.ExecContext(ctx, q, id, status)
	return err
}

// ============================================================
// Sales Queries
// ============================================================

func (r *postgresRepo) CreateSale(ctx context.Context, tx *sqlx.Tx, sale *Sale) error {
	const q = `
		INSERT INTO sales (id, shift_id, branch_id, channel, gross_amount, discount_amount, net_amount, created_by, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := tx.ExecContext(ctx, q, sale.ID, sale.ShiftID, sale.BranchID, sale.Channel, sale.GrossAmount, sale.DiscountAmount, sale.NetAmount, sale.CreatedBy, sale.CreatedAt)
	return err
}

func (r *postgresRepo) CreateSaleItem(ctx context.Context, tx *sqlx.Tx, item *SaleItem) error {
	const q = `
		INSERT INTO sale_items (id, sale_id, menu_item_id, quantity, unit_price, total_price, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := tx.ExecContext(ctx, q, item.ID, item.SaleID, item.MenuItemID, item.Quantity, item.UnitPrice, item.TotalPrice, item.CreatedAt)
	return err
}

func (r *postgresRepo) GetShiftSalesTotal(ctx context.Context, shiftID string) (float64, float64, error) {
	const q = `
		SELECT COALESCE(SUM(gross_amount), 0), COALESCE(SUM(discount_amount), 0)
		FROM sales WHERE shift_id = $1
	`
	var gross, discount float64
	err := r.db.QueryRowContext(ctx, q, shiftID).Scan(&gross, &discount)
	return gross, discount, err
}

// ============================================================
// Discounts Queries
// ============================================================

func (r *postgresRepo) CreateDiscount(ctx context.Context, tx *sqlx.Tx, discount *Discount) error {
	const q = `
		INSERT INTO discounts (id, shift_id, amount, reason, status, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := tx.ExecContext(ctx, q, discount.ID, discount.ShiftID, discount.Amount, discount.Reason, discount.Status, discount.CreatedBy, discount.CreatedAt, discount.UpdatedAt)
	return err
}

func (r *postgresRepo) GetDiscountByID(ctx context.Context, id string) (*Discount, error) {
	const q = `SELECT id, shift_id, amount, reason, status, created_by, created_at FROM discounts WHERE id = $1`
	var d Discount
	err := r.db.GetContext(ctx, &d, q, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *postgresRepo) CancelDiscount(ctx context.Context, tx *sqlx.Tx, id, reason, actorID string) error {
	const q = `
		UPDATE discounts 
		SET status = 'CANCELED', canceled_reason = $2, canceled_by = $3, updated_at = NOW() 
		WHERE id = $1
	`
	_, err := tx.ExecContext(ctx, q, id, reason, actorID)
	return err
}

// ============================================================
// Menu Queries (للسعر التاريخي)
// ============================================================

func (r *postgresRepo) GetMenuItemPrice(ctx context.Context, id string) (float64, error) {
	const q = `SELECT selling_price FROM menu_items WHERE id = $1`
	var price float64
	err := r.db.GetContext(ctx, &price, q, id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("menu item not found")
	}
	return price, err
}

// ============================================================
// Shift Summary Queries
// ============================================================

func (r *postgresRepo) GetShiftChannelSummary(ctx context.Context, shiftID string) ([]ChannelSummary, error) {
	const q = `
		SELECT channel, SUM(net_amount) as total_sales, COUNT(id) as order_count
		FROM sales
		WHERE shift_id = $1
		GROUP BY channel
	`
	var results []ChannelSummary
	err := r.db.SelectContext(ctx, &results, q, shiftID)
	return results, err
}

func (r *postgresRepo) GetShiftItemSummary(ctx context.Context, shiftID string) ([]ItemSummary, error) {
	const q = `
		SELECT si.menu_item_id, mi.name as item_name, SUM(si.quantity) as total_quantity, SUM(si.total_price) as total_sales
		FROM sale_items si
		JOIN sales s ON si.sale_id = s.id
		LEFT JOIN menu_items mi ON si.menu_item_id = mi.id
		WHERE s.shift_id = $1
		GROUP BY si.menu_item_id, mi.name
	`
	var results []ItemSummary
	err := r.db.SelectContext(ctx, &results, q, shiftID)
	return results, err
}

var _ Repository = (*postgresRepo)(nil)
