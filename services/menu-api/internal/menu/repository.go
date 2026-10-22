package menu

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// ============================================================
// Repository Interface
// ============================================================

type Repository interface {
	// Categories
	CreateCategory(ctx context.Context, tx *sqlx.Tx, c *Category) error
	ListCategories(ctx context.Context) ([]*Category, error)

	// Menu Items
	CreateMenuItem(ctx context.Context, tx *sqlx.Tx, item *MenuItem) error
	GetMenuItemByID(ctx context.Context, id string) (*MenuItem, error)
	ListMenuItems(ctx context.Context) ([]*MenuItem, error)
	UpdateMenuItem(ctx context.Context, tx *sqlx.Tx, item *MenuItem) error

	// Menu Item Requests
	CreateMenuItemRequest(ctx context.Context, tx *sqlx.Tx, req *MenuItemRequest) error
	GetMenuItemRequestByID(ctx context.Context, id string) (*MenuItemRequest, error)
	ListMenuItemRequests(ctx context.Context) ([]*MenuItemRequest, error)
	UpdateMenuItemRequest(ctx context.Context, tx *sqlx.Tx, req *MenuItemRequest) error
}

// ============================================================
// Implementation
// ============================================================

type postgresRepo struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &postgresRepo{db: db}
}

// ============================================================
// Categories
// ============================================================

func (r *postgresRepo) CreateCategory(ctx context.Context, tx *sqlx.Tx, c *Category) error {
	const q = `
		INSERT INTO categories (id, name, sort_order, is_active, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := tx.ExecContext(ctx, q, c.ID, c.Name, c.SortOrder, c.IsActive, c.CreatedAt)
	return err
}

func (r *postgresRepo) ListCategories(ctx context.Context) ([]*Category, error) {
	var cats []*Category
	err := r.db.SelectContext(ctx, &cats, `SELECT id, name, sort_order, is_active, created_at FROM categories ORDER BY sort_order`)
	if err != nil {
		return nil, err
	}
	return cats, nil
}

// ============================================================
// Menu Items
// ============================================================

func (r *postgresRepo) CreateMenuItem(ctx context.Context, tx *sqlx.Tx, item *MenuItem) error {
	const q = `
		INSERT INTO menu_items (
			id, sku, name, name_en, category_id, selling_price,
			kds_station, prep_time_minutes, description, is_active,
			created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
	`
	_, err := tx.ExecContext(ctx, q,
		item.ID, item.SKU, item.Name, item.NameEn, item.CategoryID, item.SellingPrice,
		item.KDSStation, item.PrepTimeMinutes, item.Description, item.IsActive,
		item.CreatedAt, item.UpdatedAt,
	)
	return err
}

func (r *postgresRepo) GetMenuItemByID(ctx context.Context, id string) (*MenuItem, error) {
	const q = `
		SELECT id, sku, name, name_en, category_id, selling_price, kds_station,
		       prep_time_minutes, description, is_active, created_at, updated_at
		FROM menu_items WHERE id = $1
	`
	var item MenuItem
	err := r.db.GetContext(ctx, &item, q, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get menu item: %w", err)
	}
	return &item, nil
}

func (r *postgresRepo) ListMenuItems(ctx context.Context) ([]*MenuItem, error) {
	const q = `
		SELECT id, sku, name, name_en, category_id, selling_price, kds_station,
		       prep_time_minutes, description, is_active, created_at, updated_at
		FROM menu_items ORDER BY name
	`
	var items []*MenuItem
	err := r.db.SelectContext(ctx, &items, q)
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (r *postgresRepo) UpdateMenuItem(ctx context.Context, tx *sqlx.Tx, item *MenuItem) error {
	const q = `
		UPDATE menu_items
		SET name = $2, name_en = $3, category_id = $4, selling_price = $5,
		    kds_station = $6, prep_time_minutes = $7, description = $8,
		    is_active = $9, updated_at = $10
		WHERE id = $1
	`
	res, err := tx.ExecContext(ctx, q,
		item.ID, item.Name, item.NameEn, item.CategoryID, item.SellingPrice,
		item.KDSStation, item.PrepTimeMinutes, item.Description,
		item.IsActive, item.UpdatedAt,
	)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return errors.New("menu item not found")
	}
	return nil
}

// ============================================================
// Menu Item Requests
// ============================================================

func (r *postgresRepo) CreateMenuItemRequest(ctx context.Context, tx *sqlx.Tx, req *MenuItemRequest) error {
	const q = `
		INSERT INTO menu_item_requests (
			id, branch_id, requested_by, item_name, requested_price,
			description, justification, status, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	`
	_, err := tx.ExecContext(ctx, q,
		req.ID, req.BranchID, req.RequestedBy, req.ItemName, req.RequestedPrice,
		req.Description, req.Justification, req.Status, req.CreatedAt, req.UpdatedAt,
	)
	return err
}

func (r *postgresRepo) GetMenuItemRequestByID(ctx context.Context, id string) (*MenuItemRequest, error) {
	const q = `
		SELECT id, branch_id, requested_by, item_name, requested_price,
		       description, justification, status, rejection_reason,
		       created_at, updated_at
		FROM menu_item_requests WHERE id = $1
	`
	var req MenuItemRequest
	err := r.db.GetContext(ctx, &req, q, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get menu request: %w", err)
	}
	return &req, nil
}

func (r *postgresRepo) ListMenuItemRequests(ctx context.Context) ([]*MenuItemRequest, error) {
	const q = `
		SELECT id, branch_id, requested_by, item_name, requested_price,
		       description, justification, status, rejection_reason,
		       created_at, updated_at
		FROM menu_item_requests ORDER BY created_at DESC
	`
	var reqs []*MenuItemRequest
	err := r.db.SelectContext(ctx, &reqs, q)
	if err != nil {
		return nil, err
	}
	return reqs, nil
}

func (r *postgresRepo) UpdateMenuItemRequest(ctx context.Context, tx *sqlx.Tx, req *MenuItemRequest) error {
	const q = `
		UPDATE menu_item_requests
		SET status = $2, rejection_reason = $3, updated_at = $4
		WHERE id = $1
	`
	res, err := tx.ExecContext(ctx, q, req.ID, req.Status, req.RejectionReason, req.UpdatedAt)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return errors.New("menu request not found")
	}
	return nil
}

var _ Repository = (*postgresRepo)(nil)
