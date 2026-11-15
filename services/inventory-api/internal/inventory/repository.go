package inventory
 
import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
 
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/shopspring/decimal"
)
 
const inventoryColumns = `id, branch_id, good_id, quantity, unit_cost,
	min_stock, reorder_level, max_stock, location_code, created_at, updated_at`
 
const movementColumns = `id, inventory_id, branch_id, good_id, type, quantity, quantity_before,
	quantity_after, unit_cost, destination_branch_id, reason, reference_no, reason_code,
	adjustment_kind, counterparty, created_by, created_at`
 
// TxStore holds the operations that must run inside a single transaction.
type TxStore interface {
	LockOrCreateInventory(ctx context.Context, branchID, goodID uuid.UUID) (Inventory, error)
	UpdateInventory(ctx context.Context, id uuid.UUID, quantity, unitCost decimal.Decimal) (Inventory, error)
	UpdateSettings(ctx context.Context, id uuid.UUID, s StockSettings) (Inventory, error)
	NextReference(ctx context.Context) (int64, error)
	InsertMovement(ctx context.Context, m *InventoryMovement) error
}
 
// Store is the persistence contract used by the service.
type Store interface {
	WithTx(ctx context.Context, fn func(tx TxStore) error) error
	ListInventories(ctx context.Context, f InventoryFilter) ([]Inventory, int, error)
	GetInventory(ctx context.Context, branchID, goodID uuid.UUID) (Inventory, error)
	ListAllInventories(ctx context.Context, branchID uuid.UUID) ([]Inventory, error)
	ListMovements(ctx context.Context, f MovementFilter) ([]InventoryMovement, int, error)
	MovementSummary(ctx context.Context, f MovementFilter) ([]TypeAggregate, error)
	OutflowSince(ctx context.Context, branchID, goodID uuid.UUID, since time.Time) (decimal.Decimal, error)
}
 
var (
	_ Store   = (*Repository)(nil)
	_ TxStore = (*txRepository)(nil)
)
 
// ---------- small SQL helpers ----------
 
type whereBuilder struct {
	conds []string
	args  []any
}
 
// add appends a condition with one placeholder; cond must contain exactly one %d.
func (w *whereBuilder) add(cond string, v any) {
	w.args = append(w.args, v)
	w.conds = append(w.conds, fmt.Sprintf(cond, len(w.args)))
}
 
func (w *whereBuilder) raw(cond string) { w.conds = append(w.conds, cond) }
 
func (w *whereBuilder) sql() string { return strings.Join(w.conds, " AND ") }
 
func uuidStrings(ids []uuid.UUID) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, id.String())
	}
	return out
}
 
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
 
func escapeLike(s string) string { return likeEscaper.Replace(s) }
 
// Status conditions — must mirror Inventory.Status() in model.go.
const (
	sqlIsCritical = `(min_stock > 0 AND quantity < min_stock)`
	sqlIsLow      = `(NOT ` + sqlIsCritical + ` AND quantity < GREATEST(min_stock, reorder_level))`
	sqlIsAvail    = `(NOT ` + sqlIsCritical + ` AND quantity >= GREATEST(min_stock, reorder_level))`
)
 
// ---------- Repository (non-transactional reads + tx factory) ----------
 
type Repository struct {
	db *sqlx.DB
}
 
func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}
 
// WithTx runs fn inside a transaction. It commits if fn returns nil,
// and rolls back on error or panic.
func (r *Repository) WithTx(ctx context.Context, fn func(tx TxStore) error) (err error) {
	tx, err := r.db.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
 
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback()
		}
	}()
 
	if err = fn(&txRepository{tx: tx}); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
 
func (r *Repository) ListInventories(ctx context.Context, f InventoryFilter) ([]Inventory, int, error) {
	w := &whereBuilder{}
	w.add("branch_id = $%d", f.BranchID)
	if f.FilterByGoods {
		w.add("good_id = ANY($%d::uuid[])", pq.Array(uuidStrings(f.GoodIDs)))
	}
	if f.Status != nil {
		switch *f.Status {
		case StatusCritical:
			w.raw(sqlIsCritical)
		case StatusLow:
			w.raw(sqlIsLow)
		case StatusAvailable:
			w.raw(sqlIsAvail)
		}
	}
	whereSQL := w.sql()
 
	var total int
	if err := r.db.GetContext(ctx, &total,
		`SELECT COUNT(*) FROM inventories WHERE `+whereSQL, w.args...); err != nil {
		return nil, 0, fmt.Errorf("count inventories: %w", err)
	}
 
	offset := (f.Page - 1) * f.Limit
	pageArgs := append(append([]any{}, w.args...), f.Limit, offset)
	q := fmt.Sprintf(`SELECT %s
		FROM inventories
		WHERE %s
		ORDER BY updated_at DESC, id
		LIMIT $%d OFFSET $%d`,
		inventoryColumns, whereSQL, len(w.args)+1, len(w.args)+2)
 
	items := make([]Inventory, 0)
	if err := r.db.SelectContext(ctx, &items, q, pageArgs...); err != nil {
		return nil, 0, fmt.Errorf("list inventories: %w", err)
	}
	return items, total, nil
}
 
func (r *Repository) GetInventory(ctx context.Context, branchID, goodID uuid.UUID) (Inventory, error) {
	var inv Inventory
	q := `SELECT ` + inventoryColumns + ` FROM inventories WHERE branch_id = $1 AND good_id = $2`
	if err := r.db.GetContext(ctx, &inv, q, branchID, goodID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Inventory{}, ErrInventoryNotFound
		}
		return Inventory{}, fmt.Errorf("get inventory: %w", err)
	}
	return inv, nil
}
 
// ListAllInventories returns every tracked good of a branch (valuation, summary).
func (r *Repository) ListAllInventories(ctx context.Context, branchID uuid.UUID) ([]Inventory, error) {
	items := make([]Inventory, 0)
	q := `SELECT ` + inventoryColumns + `
		FROM inventories
		WHERE branch_id = $1
		ORDER BY (quantity * unit_cost) DESC, id`
	if err := r.db.SelectContext(ctx, &items, q, branchID); err != nil {
		return nil, fmt.Errorf("list all inventories: %w", err)
	}
	return items, nil
}
 
// movementWhere builds the WHERE clause shared by the list and the summary.
func movementWhere(f MovementFilter) *whereBuilder {
	w := &whereBuilder{}
	w.add("branch_id = $%d", f.BranchID)
	if f.GoodID != nil {
		w.add("good_id = $%d", *f.GoodID)
	}
	if f.Type != nil {
		w.add("type = $%d", string(*f.Type))
	}
	if f.From != nil {
		w.add("created_at >= $%d", *f.From)
	}
	if f.To != nil {
		w.add("created_at < $%d", *f.To)
	}
	if f.Search != "" {
		w.args = append(w.args, "%"+escapeLike(f.Search)+"%")
		cond := fmt.Sprintf("reference_no ILIKE $%d", len(w.args))
		if len(f.SearchGoodIDs) > 0 {
			w.args = append(w.args, pq.Array(uuidStrings(f.SearchGoodIDs)))
			cond += fmt.Sprintf(" OR good_id = ANY($%d::uuid[])", len(w.args))
		}
		w.raw("(" + cond + ")")
	}
	return w
}
 
func (r *Repository) ListMovements(ctx context.Context, f MovementFilter) ([]InventoryMovement, int, error) {
	w := movementWhere(f)
	whereSQL := w.sql()
 
	var total int
	if err := r.db.GetContext(ctx, &total,
		`SELECT COUNT(*) FROM inventory_movements WHERE `+whereSQL, w.args...); err != nil {
		return nil, 0, fmt.Errorf("count movements: %w", err)
	}
 
	offset := (f.Page - 1) * f.Limit
	pageArgs := append(append([]any{}, w.args...), f.Limit, offset)
 
	q := fmt.Sprintf(`SELECT %s
		FROM inventory_movements
		WHERE %s
		ORDER BY created_at DESC, id DESC
		LIMIT $%d OFFSET $%d`,
		movementColumns, whereSQL, len(w.args)+1, len(w.args)+2)
 
	items := make([]InventoryMovement, 0)
	if err := r.db.SelectContext(ctx, &items, q, pageArgs...); err != nil {
		return nil, 0, fmt.Errorf("list movements: %w", err)
	}
	return items, total, nil
}
 
// MovementSummary aggregates movements per type with the same filters as the list
// (paging is ignored). Quantity and value are signed net changes.
func (r *Repository) MovementSummary(ctx context.Context, f MovementFilter) ([]TypeAggregate, error) {
	w := movementWhere(f)
	q := `SELECT type,
			COUNT(*) AS count,
			COALESCE(SUM(quantity_after - quantity_before), 0) AS quantity,
			COALESCE(SUM((quantity_after - quantity_before) * COALESCE(unit_cost, 0)), 0) AS value
		FROM inventory_movements
		WHERE ` + w.sql() + `
		GROUP BY type`
 
	rows := make([]TypeAggregate, 0, 3)
	if err := r.db.SelectContext(ctx, &rows, q, w.args...); err != nil {
		return nil, fmt.Errorf("movement summary: %w", err)
	}
	return rows, nil
}
 
// OutflowSince returns the total quantity issued (OUT, excluding transfers to
// other branches) for one good since the given time — used for average consumption.
func (r *Repository) OutflowSince(ctx context.Context, branchID, goodID uuid.UUID, since time.Time) (decimal.Decimal, error) {
	var total decimal.Decimal
	const q = `SELECT COALESCE(SUM(quantity), 0)
		FROM inventory_movements
		WHERE branch_id = $1 AND good_id = $2
		  AND type = 'OUT' AND destination_branch_id IS NULL
		  AND created_at >= $3`
	if err := r.db.GetContext(ctx, &total, q, branchID, goodID, since); err != nil {
		return decimal.Zero, fmt.Errorf("outflow since: %w", err)
	}
	return total, nil
}
 
// ---------- txRepository (writes inside a transaction) ----------
 
type txRepository struct {
	tx *sqlx.Tx
}
 
// LockOrCreateInventory returns the balance row locked with FOR UPDATE,
// creating it first if this branch has never tracked the good.
// ON CONFLICT makes concurrent first-time creation safe.
func (t *txRepository) LockOrCreateInventory(ctx context.Context, branchID, goodID uuid.UUID) (Inventory, error) {
	inv, err := t.selectForUpdate(ctx, branchID, goodID)
	if err == nil {
		return inv, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Inventory{}, fmt.Errorf("lock inventory: %w", err)
	}
 
	const insertQ = `INSERT INTO inventories (id, branch_id, good_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (branch_id, good_id) DO NOTHING`
	if _, err := t.tx.ExecContext(ctx, insertQ, uuid.New(), branchID, goodID); err != nil {
		return Inventory{}, fmt.Errorf("create inventory: %w", err)
	}
 
	inv, err = t.selectForUpdate(ctx, branchID, goodID)
	if err != nil {
		return Inventory{}, fmt.Errorf("lock new inventory: %w", err)
	}
	return inv, nil
}
 
func (t *txRepository) selectForUpdate(ctx context.Context, branchID, goodID uuid.UUID) (Inventory, error) {
	var inv Inventory
	q := `SELECT ` + inventoryColumns + `
		FROM inventories
		WHERE branch_id = $1 AND good_id = $2
		FOR UPDATE`
	err := t.tx.GetContext(ctx, &inv, q, branchID, goodID)
	return inv, err
}
 
func (t *txRepository) UpdateInventory(ctx context.Context, id uuid.UUID, quantity, unitCost decimal.Decimal) (Inventory, error) {
	var inv Inventory
	q := `UPDATE inventories
		SET quantity = $2, unit_cost = $3, updated_at = NOW()
		WHERE id = $1
		RETURNING ` + inventoryColumns
	if err := t.tx.GetContext(ctx, &inv, q, id, quantity, unitCost); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Inventory{}, ErrInventoryNotFound
		}
		return Inventory{}, fmt.Errorf("update inventory: %w", err)
	}
	return inv, nil
}
 
// UpdateSettings overwrites the four settings columns with the (already merged) values.
func (t *txRepository) UpdateSettings(ctx context.Context, id uuid.UUID, s StockSettings) (Inventory, error) {
	var inv Inventory
	q := `UPDATE inventories
		SET min_stock = $2, reorder_level = $3, max_stock = $4, location_code = $5, updated_at = NOW()
		WHERE id = $1
		RETURNING ` + inventoryColumns
	if err := t.tx.GetContext(ctx, &inv, q, id, s.MinStock, s.ReorderLevel, s.MaxStock, s.LocationCode); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Inventory{}, ErrInventoryNotFound
		}
		return Inventory{}, fmt.Errorf("update settings: %w", err)
	}
	return inv, nil
}
 
// NextReference returns the next number of the shared reference sequence.
// Sequences are not rolled back, so gaps are possible and expected.
func (t *txRepository) NextReference(ctx context.Context) (int64, error) {
	var n int64
	if err := t.tx.GetContext(ctx, &n, `SELECT nextval('inventory_reference_seq')`); err != nil {
		return 0, fmt.Errorf("next reference: %w", err)
	}
	return n, nil
}
 
func (t *txRepository) InsertMovement(ctx context.Context, m *InventoryMovement) error {
	const q = `INSERT INTO inventory_movements (
			id, inventory_id, branch_id, good_id, type, quantity,
			quantity_before, quantity_after, unit_cost,
			destination_branch_id, reason, reference_no, reason_code,
			adjustment_kind, counterparty, created_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		RETURNING created_at`
 
	err := t.tx.QueryRowxContext(ctx, q,
		m.ID, m.InventoryID, m.BranchID, m.GoodID, string(m.Type), m.Quantity,
		m.QuantityBefore, m.QuantityAfter, m.UnitCost,
		m.DestinationBranchID, m.Reason, m.ReferenceNo, enumPtr(m.ReasonCode),
		enumPtr(m.AdjustmentKind), m.Counterparty, m.CreatedBy,
	).Scan(&m.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert movement: %w", err)
	}
	return nil
}