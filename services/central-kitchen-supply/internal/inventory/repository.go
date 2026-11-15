package inventory

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

// ============================================================
// Repository Interface
// ============================================================

type Repository interface {
	// GetInventoryByBranchAndGood - يجيب رصيد صنف معين في فرع معين
	GetInventoryByBranchAndGood(ctx context.Context, branchID, goodID string) (*Inventory, error)

	// CreateInventory - ينشئ سجل مخزون جديد لصنف
	CreateInventory(ctx context.Context, tx *sqlx.Tx, inv *Inventory) error

	// UpdateInventoryQuantity - يحدث الكمية
	UpdateInventoryQuantity(ctx context.Context, tx *sqlx.Tx, id string, newQty float64, updatedAt time.Time) error

	// CreateMovement - يسجل حركة المخزون
	CreateMovement(ctx context.Context, tx *sqlx.Tx, mov *InventoryMovement) error
	CreateGood(ctx context.Context, tx *sqlx.Tx, g *Good) error
	GetGoodByID(ctx context.Context, id string) (*Good, error)
	ListGoods(ctx context.Context) ([]*Good, error) // ← ضيفي ده
	// Supply Requests
	CreateSupplyRequest(ctx context.Context, tx *sqlx.Tx, req *SupplyRequest) error
	GetSupplyRequestByID(ctx context.Context, id string) (*SupplyRequest, error)
	ListSupplyRequests(ctx context.Context) ([]*SupplyRequest, error)
	UpdateSupplyRequestStatus(ctx context.Context, tx *sqlx.Tx, id string, status string) error

	// Supply Request Items
	CreateSupplyRequestItem(ctx context.Context, tx *sqlx.Tx, item *SupplyRequestItem) error
	UpdateSupplyRequestItem(ctx context.Context, tx *sqlx.Tx, item *SupplyRequestItem) error
	ListSupplyRequestItems(ctx context.Context, requestID string) ([]*SupplyRequestItem, error)
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
// Queries
// ============================================================

func (r *postgresRepo) GetInventoryByBranchAndGood(ctx context.Context, branchID, goodID string) (*Inventory, error) {
	const q = `
		SELECT id, branch_id, good_id, quantity, updated_at
		FROM inventories
		WHERE branch_id = $1 AND good_id = $2
	`
	var inv Inventory
	err := r.db.GetContext(ctx, &inv, q, branchID, goodID)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil // مش موجود
	}
	if err != nil {
		return nil, fmt.Errorf("get inventory: %w", err)
	}
	return &inv, nil
}

func (r *postgresRepo) CreateInventory(ctx context.Context, tx *sqlx.Tx, inv *Inventory) error {
	const q = `
		INSERT INTO inventories (id, branch_id, good_id, quantity, updated_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := tx.ExecContext(ctx, q, inv.ID, inv.BranchID, inv.GoodID, inv.Quantity, inv.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create inventory: %w", err)
	}
	return nil
}

func (r *postgresRepo) UpdateInventoryQuantity(ctx context.Context, tx *sqlx.Tx, id string, newQty float64, updatedAt time.Time) error {
	const q = `
		UPDATE inventories
		SET quantity = $2, updated_at = $3
		WHERE id = $1
	`
	res, err := tx.ExecContext(ctx, q, id, newQty, updatedAt)
	if err != nil {
		return fmt.Errorf("update inventory qty: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrInventoryNotFound
	}
	return nil
}

func (r *postgresRepo) CreateMovement(ctx context.Context, tx *sqlx.Tx, mov *InventoryMovement) error {
	const q = `
		INSERT INTO inventory_movements (id, inventory_id, type, quantity, reason, created_by, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := tx.ExecContext(ctx, q, mov.ID, mov.InventoryID, mov.Type, mov.Quantity, mov.Reason, mov.CreatedBy, mov.CreatedAt)
	if err != nil {
		return fmt.Errorf("create movement: %w", err)
	}
	return nil
}

// ============================================================
// Goods Queries
// ============================================================

func (r *postgresRepo) CreateGood(ctx context.Context, tx *sqlx.Tx, g *Good) error {
	const q = `
		INSERT INTO goods (id, sku, name, category, description, unit, unit_cost, reorder_level, max_capacity, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`
	_, err := tx.ExecContext(ctx, q,
		g.ID, g.SKU, g.Name, g.Category, g.Description, g.Unit,
		g.UnitCost, g.ReorderLevel, g.MaxCapacity, g.IsActive, g.CreatedAt, g.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create good: %w", err)
	}
	return nil
}

func (r *postgresRepo) GetGoodByID(ctx context.Context, id string) (*Good, error) {
	const q = `
		SELECT id, sku, name, category, description, unit, unit_cost, reorder_level, max_capacity, is_active, created_at, updated_at
		FROM goods
		WHERE id = $1
	`
	var g Good
	err := r.db.GetContext(ctx, &g, q, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get good: %w", err)
	}
	return &g, nil
}
func (r *postgresRepo) ListGoods(ctx context.Context) ([]*Good, error) {
	const q = `
		SELECT id, sku, name, category, description, unit, unit_cost, reorder_level, max_capacity, is_active, created_at, updated_at
		FROM goods
		ORDER BY created_at DESC
	`
	var goods []*Good
	err := r.db.SelectContext(ctx, &goods, q)
	if err != nil {
		return nil, fmt.Errorf("list goods: %w", err)
	}
	return goods, nil
}
func (r *postgresRepo) CreateSupplyRequest(ctx context.Context, tx *sqlx.Tx, req *SupplyRequest) error {
	const q = `
		INSERT INTO supply_requests (id, branch_id, status, created_by, notes, priority, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := tx.ExecContext(ctx, q, req.ID, req.BranchID, req.Status, req.CreatedBy, req.Notes, req.Priority, req.CreatedAt, req.UpdatedAt)
	return err
}

func (r *postgresRepo) GetSupplyRequestByID(ctx context.Context, id string) (*SupplyRequest, error) {
	const q = `
		SELECT id, branch_id, status, created_by, notes, priority, created_at, updated_at
		FROM supply_requests
		WHERE id = $1
	`
	var req SupplyRequest
	err := r.db.GetContext(ctx, &req, q, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get supply request: %w", err)
	}

	// جلب العناصر
	items, err := r.ListSupplyRequestItems(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	req.Items = items

	return &req, nil
}

func (r *postgresRepo) ListSupplyRequests(ctx context.Context) ([]*SupplyRequest, error) {
	const q = `
		SELECT id, branch_id, status, created_by, notes, priority, created_at, updated_at
		FROM supply_requests
		ORDER BY created_at DESC
	`
	var reqs []*SupplyRequest
	if err := r.db.SelectContext(ctx, &reqs, q); err != nil {
		return nil, err
	}
	return reqs, nil
}

func (r *postgresRepo) UpdateSupplyRequestStatus(ctx context.Context, tx *sqlx.Tx, id string, status string) error {
	const q = `UPDATE supply_requests SET status = $2, updated_at = NOW() WHERE id = $1`
	_, err := tx.ExecContext(ctx, q, id, status)
	return err
}

func (r *postgresRepo) CreateSupplyRequestItem(ctx context.Context, tx *sqlx.Tx, item *SupplyRequestItem) error {
	const q = `
		INSERT INTO supply_request_items (id, request_id, good_id, requested_qty, approved_qty, received_qty, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := tx.ExecContext(ctx, q, item.ID, item.RequestID, item.GoodID, item.RequestedQty, item.ApprovedQty, item.ReceivedQty, item.CreatedAt)
	return err
}

func (r *postgresRepo) UpdateSupplyRequestItem(ctx context.Context, tx *sqlx.Tx, item *SupplyRequestItem) error {
	const q = `UPDATE supply_request_items SET approved_qty = $2, received_qty = $3 WHERE id = $1`
	_, err := tx.ExecContext(ctx, q, item.ID, item.ApprovedQty, item.ReceivedQty)
	return err
}

func (r *postgresRepo) ListSupplyRequestItems(ctx context.Context, requestID string) ([]*SupplyRequestItem, error) {
	const q = `
		SELECT id, request_id, good_id, requested_qty, approved_qty, received_qty, created_at
		FROM supply_request_items
		WHERE request_id = $1
	`
	var items []*SupplyRequestItem
	err := r.db.SelectContext(ctx, &items, q, requestID)
	if err != nil {
		return nil, err
	}
	return items, nil
}

var _ Repository = (*postgresRepo)(nil)
