package transfer

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

const transferCols = `t.id, t.code, t.source_branch_id, t.dest_branch_id, t.good_id,
	t.requested_qty, t.approved_qty, t.sent_qty, t.received_qty, t.shortage_qty, t.unit_cost,
	t.priority, t.reason, t.status, t.decision_note,
	t.vehicle_type, t.driver_name, t.driver_phone, t.expected_departure, t.temperature_c, t.receive_note,
	t.requested_by, t.requested_at, t.decided_by, t.decided_at,
	t.sent_by, t.sent_at, t.received_by, t.received_at, t.updated_at`

const viewCols = transferCols + `,
	src.name AS source_branch_name, src.code AS source_branch_code,
	dst.name AS dest_branch_name, dst.code AS dest_branch_code,
	g.name AS good_name, g.sku AS good_sku, g.unit AS good_unit`

const fromClause = ` FROM transfers t
	JOIN branches src ON src.id = t.source_branch_id
	JOIN branches dst ON dst.id = t.dest_branch_id
	JOIN goods g ON g.id = t.good_id`

type inventoryRow struct {
	ID       uuid.UUID `db:"id"`
	Quantity float64   `db:"quantity"`
	UnitCost float64   `db:"unit_cost"`
}

// Movement is one row of the append-only inventory_movements ledger.
type Movement struct {
	InventoryID  uuid.UUID
	BranchID     uuid.UUID
	GoodID       uuid.UUID
	Type         string // IN | OUT
	Quantity     float64
	Before       float64
	After        float64
	UnitCost     *float64
	DestBranchID *uuid.UUID // only for OUT (transfer-out)
	Reason       string
	UserID       uuid.UUID
}

// Repository holds SQL only. Every method takes a sqlx.ExtContext so it can run
// either on the pool (*sqlx.DB) or inside a transaction (*sqlx.Tx).
type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

// ---------- lookups ----------

func (r *Repository) BranchActive(ctx context.Context, q sqlx.ExtContext, id uuid.UUID) (bool, error) {
	var ok bool
	err := sqlx.GetContext(ctx, q, &ok,
		`SELECT EXISTS (SELECT 1 FROM branches WHERE id = $1 AND is_active)`, id)
	return ok, err
}

func (r *Repository) GoodExists(ctx context.Context, q sqlx.ExtContext, id uuid.UUID) (bool, error) {
	var ok bool
	err := sqlx.GetContext(ctx, q, &ok, `SELECT EXISTS (SELECT 1 FROM goods WHERE id = $1)`, id)
	return ok, err
}

// ---------- transfers ----------

// Insert creates a PENDING transfer and fills t.Code from the DB default.
func (r *Repository) Insert(ctx context.Context, q sqlx.ExtContext, t *Transfer) error {
	return sqlx.GetContext(ctx, q, &t.Code, `
		INSERT INTO transfers
			(id, source_branch_id, dest_branch_id, good_id, requested_qty, priority, reason, requested_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING code`,
		t.ID, t.SourceBranchID, t.DestBranchID, t.GoodID, t.RequestedQty,
		string(t.Priority), t.Reason, t.RequestedBy)
}

// GetForUpdate locks the transfer row until the transaction ends.
func (r *Repository) GetForUpdate(ctx context.Context, q sqlx.ExtContext, id uuid.UUID) (*Transfer, error) {
	var t Transfer
	err := sqlx.GetContext(ctx, q, &t,
		`SELECT `+transferCols+` FROM transfers t WHERE t.id = $1 FOR UPDATE OF t`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrTransferNotFound
	}
	return &t, err
}

func (r *Repository) GetView(ctx context.Context, q sqlx.ExtContext, id uuid.UUID) (*TransferView, error) {
	var v TransferView
	err := sqlx.GetContext(ctx, q, &v, `SELECT `+viewCols+fromClause+` WHERE t.id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrTransferNotFound
	}
	return &v, err
}

func (r *Repository) Events(ctx context.Context, q sqlx.ExtContext, transferID uuid.UUID) ([]TransferEvent, error) {
	events := []TransferEvent{}
	err := sqlx.SelectContext(ctx, q, &events, `
		SELECT id, transfer_id, action, actor_id, note, created_at
		FROM transfer_events
		WHERE transfer_id = $1
		ORDER BY created_at ASC, id ASC`, transferID)
	return events, err
}

func (r *Repository) InsertEvent(ctx context.Context, q sqlx.ExtContext,
	transferID uuid.UUID, action Action, actorID uuid.UUID, note *string) error {
	_, err := q.ExecContext(ctx, `
		INSERT INTO transfer_events (id, transfer_id, action, actor_id, note)
		VALUES ($1, $2, $3, $4, $5)`,
		uuid.New(), transferID, string(action), actorID, note)
	return err
}

func (r *Repository) MarkApproved(ctx context.Context, q sqlx.ExtContext,
	id uuid.UUID, approvedQty float64, note *string, userID uuid.UUID) error {
	_, err := q.ExecContext(ctx, `
		UPDATE transfers
		SET status = 'APPROVED', approved_qty = $2, decision_note = $3,
		    decided_by = $4, decided_at = NOW(), updated_at = NOW()
		WHERE id = $1`, id, approvedQty, note, userID)
	return err
}

func (r *Repository) MarkRejected(ctx context.Context, q sqlx.ExtContext,
	id uuid.UUID, reason string, userID uuid.UUID) error {
	_, err := q.ExecContext(ctx, `
		UPDATE transfers
		SET status = 'REJECTED', decision_note = $2,
		    decided_by = $3, decided_at = NOW(), updated_at = NOW()
		WHERE id = $1`, id, reason, userID)
	return err
}

func (r *Repository) MarkSent(ctx context.Context, q sqlx.ExtContext, id uuid.UUID, qty, unitCost float64,
	vehicle, driver, phone *string, departure *time.Time, temp *float64, userID uuid.UUID) error {
	_, err := q.ExecContext(ctx, `
		UPDATE transfers
		SET status = 'SENT', sent_qty = $2,
		    vehicle_type = $3, driver_name = $4, driver_phone = $5,
		    expected_departure = $6, temperature_c = $7,
		    sent_by = $8, sent_at = NOW(), unit_cost = $9, updated_at = NOW()
		WHERE id = $1`, id, qty, vehicle, driver, phone, departure, temp, userID, unitCost)
	return err
}

func (r *Repository) MarkReceived(ctx context.Context, q sqlx.ExtContext, id uuid.UUID,
	received, shortage float64, note *string, userID uuid.UUID) error {
	_, err := q.ExecContext(ctx, `
		UPDATE transfers
		SET status = 'RECEIVED', received_qty = $2, shortage_qty = $3, receive_note = $4,
		    received_by = $5, received_at = NOW(), updated_at = NOW()
		WHERE id = $1`, id, received, shortage, note, userID)
	return err
}

// ---------- inventory ----------

// LockInventory returns the (branch, good) stock row locked for update, or nil if none exists.
func (r *Repository) LockInventory(ctx context.Context, q sqlx.ExtContext,
	branchID, goodID uuid.UUID) (*inventoryRow, error) {
	var inv inventoryRow
	err := sqlx.GetContext(ctx, q, &inv, `
		SELECT id, quantity, unit_cost FROM inventories
		WHERE branch_id = $1 AND good_id = $2
		FOR UPDATE`, branchID, goodID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &inv, err
}

func (r *Repository) DecreaseInventory(ctx context.Context, q sqlx.ExtContext, inventoryID uuid.UUID, qty float64) error {
	_, err := q.ExecContext(ctx,
		`UPDATE inventories SET quantity = quantity - $2, updated_at = NOW() WHERE id = $1`,
		inventoryID, qty)
	return err
}

// EnsureInventory creates an empty (branch, good) row when none exists yet.
// Race-safe thanks to the uq_inventories_branch_good constraint.
func (r *Repository) EnsureInventory(ctx context.Context, q sqlx.ExtContext,
	branchID, goodID uuid.UUID, unitCost float64) error {
	_, err := q.ExecContext(ctx, `
		INSERT INTO inventories (id, branch_id, good_id, quantity, unit_cost)
		VALUES ($1, $2, $3, 0, $4)
		ON CONFLICT (branch_id, good_id) DO NOTHING`,
		uuid.New(), branchID, goodID, unitCost)
	return err
}

// SetInventory writes the new quantity and unit cost of a row locked by LockInventory.
func (r *Repository) SetInventory(ctx context.Context, q sqlx.ExtContext,
	inventoryID uuid.UUID, quantity, unitCost float64) error {
	_, err := q.ExecContext(ctx,
		`UPDATE inventories SET quantity = $2, unit_cost = $3, updated_at = NOW() WHERE id = $1`,
		inventoryID, quantity, unitCost)
	return err
}

func (r *Repository) InsertMovement(ctx context.Context, q sqlx.ExtContext, m Movement) error {
	_, err := q.ExecContext(ctx, `
		INSERT INTO inventory_movements
			(id, inventory_id, branch_id, good_id, type, quantity, quantity_before, quantity_after,
			 unit_cost, destination_branch_id, reason, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		uuid.New(), m.InventoryID, m.BranchID, m.GoodID, m.Type, m.Quantity, m.Before, m.After,
		m.UnitCost, m.DestBranchID, m.Reason, m.UserID)
	return err
}

// ---------- list & stats ----------

func buildWhere(f ListFilter) (string, []any) {
	var conds []string
	var args []any
	add := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	if f.BranchID != nil {
		p := add(*f.BranchID)
		switch f.Direction {
		case DirectionIncoming:
			conds = append(conds, "t.dest_branch_id = "+p)
		case DirectionOutgoing:
			conds = append(conds, "t.source_branch_id = "+p)
		default:
			conds = append(conds, "(t.source_branch_id = "+p+" OR t.dest_branch_id = "+p+")")
		}
	}
	if f.Status != "" {
		conds = append(conds, "t.status = "+add(string(f.Status)))
	}
	if f.From != nil {
		conds = append(conds, "t.requested_at >= "+add(*f.From))
	}
	if f.To != nil {
		conds = append(conds, "t.requested_at < "+add(*f.To))
	}
	if f.Search != "" {
		p := add("%" + f.Search + "%")
		conds = append(conds, "(t.code ILIKE "+p+" OR g.name ILIKE "+p+" OR g.sku ILIKE "+p+
			" OR src.name ILIKE "+p+" OR dst.name ILIKE "+p+")")
	}
	if len(conds) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

func (r *Repository) List(ctx context.Context, q sqlx.ExtContext, f ListFilter) ([]TransferView, error) {
	where, args := buildWhere(f)
	query := `SELECT ` + viewCols + fromClause + where +
		fmt.Sprintf(` ORDER BY t.requested_at DESC, t.id DESC LIMIT %d OFFSET %d`, f.Limit, (f.Page-1)*f.Limit)

	items := []TransferView{}
	err := sqlx.SelectContext(ctx, q, &items, query, args...)
	return items, err
}

func (r *Repository) Stats(ctx context.Context, q sqlx.ExtContext, f ListFilter) (*Stats, error) {
	where, args := buildWhere(f)
	query := `SELECT
		COUNT(*) AS total,
		COUNT(*) FILTER (WHERE t.status = 'PENDING') AS pending_approval,
		COUNT(*) FILTER (WHERE t.status = 'SENT') AS in_transit,
		COUNT(*) FILTER (WHERE t.status = 'RECEIVED') AS received,
		COALESCE(SUM(t.received_qty) FILTER (WHERE t.status = 'RECEIVED'), 0) AS total_quantity,
		COUNT(*) FILTER (WHERE t.status = 'RECEIVED' AND t.shortage_qty > 0) AS shortage_cases` +
		fromClause + where

	var st Stats
	if err := sqlx.GetContext(ctx, q, &st, query, args...); err != nil {
		return nil, err
	}
	return &st, nil
}