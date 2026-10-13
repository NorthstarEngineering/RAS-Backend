package overtime

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
	// Create — ينشئ سجل OT جديد
	Create(ctx context.Context, tx *sqlx.Tx, ot *Overtime) error

	// GetByID — يرجع سجل OT بالـ ID
	GetByID(ctx context.Context, id string) (*Overtime, error)

	// List — يرجع سجلات OT (مع فلترة)
	List(ctx context.Context, filter ListFilter) ([]*Overtime, error)

	// Update — يحدّث سجل OT (للـ approve/reject)
	Update(ctx context.Context, tx *sqlx.Tx, ot *Overtime) error

	// ListPayableByBranchDate — يرجع OT المعتمد لفرع في فترة (للـ Labor Cost)
	// Ref: FR-FIN-004
	ListPayableByBranchDate(ctx context.Context, branchID string, from, to string) ([]*Overtime, error)
}

// ListFilter — فلتر للقائمة
type ListFilter struct {
	BranchID   string // "" لكل الفروع (CEO)
	EmployeeID string // "" لكل الموظفين
	Status     string // "" لكل الحالات
	FromDate   string // "YYYY-MM-DD"
	ToDate     string // "YYYY-MM-DD"
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
// Columns
// ============================================================

const otColumns = `
	id, employee_id, branch_id, work_date,
	hours, rate_per_hour, amount,
	status, source,
	approved_by, approved_at,
	rejected_by, rejected_at, rejection_reason,
	created_by, created_at, updated_at
`

// ============================================================
// Create
// Ref: FR-OT-001, FR-OT-002
// ============================================================

func (r *postgresRepo) Create(ctx context.Context, tx *sqlx.Tx, ot *Overtime) error {
	const q = `
		INSERT INTO overtime (
			id, employee_id, branch_id, work_date,
			hours, rate_per_hour, amount,
			status, source,
			approved_by, approved_at,
			rejected_by, rejected_at, rejection_reason,
			created_by, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7,
			$8, $9,
			$10, $11,
			$12, $13, $14,
			$15, $16, $17
		)
	`
	_, err := tx.ExecContext(ctx, q,
		ot.ID, ot.EmployeeID, ot.BranchID, ot.WorkDate,
		ot.Hours, ot.RatePerHour, ot.Amount,
		ot.Status, ot.Source,
		ot.ApprovedBy, ot.ApprovedAt,
		ot.RejectedBy, ot.RejectedAt, ot.RejectionReason,
		ot.CreatedBy, ot.CreatedAt, ot.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create overtime: %w", err)
	}
	return nil
}

// ============================================================
// GetByID
// ============================================================

func (r *postgresRepo) GetByID(ctx context.Context, id string) (*Overtime, error) {
	q := `SELECT ` + otColumns + ` FROM overtime WHERE id = $1`

	var ot Overtime
	err := r.db.GetContext(ctx, &ot, q, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get overtime by id: %w", err)
	}
	return &ot, nil
}

// ============================================================
// List — مع فلترة ديناميكية
// ============================================================

func (r *postgresRepo) List(ctx context.Context, filter ListFilter) ([]*Overtime, error) {
	// نبني الـ query بشكل ديناميكي
	query := `SELECT ` + otColumns + ` FROM overtime WHERE 1=1`
	args := []any{}
	argIdx := 1

	if filter.BranchID != "" {
		query += fmt.Sprintf(" AND branch_id = $%d", argIdx)
		args = append(args, filter.BranchID)
		argIdx++
	}
	if filter.EmployeeID != "" {
		query += fmt.Sprintf(" AND employee_id = $%d", argIdx)
		args = append(args, filter.EmployeeID)
		argIdx++
	}
	if filter.Status != "" {
		query += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, filter.Status)
		argIdx++
	}
	if filter.FromDate != "" {
		query += fmt.Sprintf(" AND work_date >= $%d", argIdx)
		args = append(args, filter.FromDate)
		argIdx++
	}
	if filter.ToDate != "" {
		query += fmt.Sprintf(" AND work_date <= $%d", argIdx)
		args = append(args, filter.ToDate)
		argIdx++
	}

	query += ` ORDER BY created_at DESC`

	var list []*Overtime
	if err := r.db.SelectContext(ctx, &list, query, args...); err != nil {
		return nil, fmt.Errorf("list overtime: %w", err)
	}
	return list, nil
}

// ============================================================
// Update — للـ approve/reject
// Ref: FR-OT-007, FR-OT-008
// ============================================================

func (r *postgresRepo) Update(ctx context.Context, tx *sqlx.Tx, ot *Overtime) error {
	const q = `
		UPDATE overtime SET
			status            = $2,
			approved_by       = $3,
			approved_at       = $4,
			rejected_by       = $5,
			rejected_at       = $6,
			rejection_reason  = $7,
			updated_at        = $8
		WHERE id = $1
	`
	res, err := tx.ExecContext(ctx, q,
		ot.ID,
		ot.Status,
		ot.ApprovedBy, ot.ApprovedAt,
		ot.RejectedBy, ot.RejectedAt, ot.RejectionReason,
		ot.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update overtime: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrOvertimeNotFound
	}
	return nil
}

// ============================================================
// ListPayableByBranchDate — للـ Labor Cost
// Ref: FR-FIN-004
// ============================================================

func (r *postgresRepo) ListPayableByBranchDate(
	ctx context.Context,
	branchID string,
	from, to string,
) ([]*Overtime, error) {
	query := `SELECT ` + otColumns + ` FROM overtime
		WHERE status = 'approved'
		  AND work_date >= $1
		  AND work_date <= $2`
	args := []any{from, to}

	if branchID != "" {
		query += ` AND branch_id = $3`
		args = append(args, branchID)
	}

	query += ` ORDER BY work_date DESC`

	var list []*Overtime
	if err := r.db.SelectContext(ctx, &list, query, args...); err != nil {
		return nil, fmt.Errorf("list payable overtime: %w", err)
	}
	return list, nil
}

// ============================================================
// Compile-time check
// ============================================================

var _ Repository = (*postgresRepo)(nil)
