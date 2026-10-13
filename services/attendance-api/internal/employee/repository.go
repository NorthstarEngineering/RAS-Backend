package employee

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
	// Create — ينشئ موظف جديد
	Create(ctx context.Context, tx *sqlx.Tx, emp *Employee) error

	// GetByID — يرجع موظف بالـ ID
	GetByID(ctx context.Context, id string) (*Employee, error)

	// GetByNationalID — يرجع موظف بالرقم القومي
	GetByNationalID(ctx context.Context, nationalID string) (*Employee, error)

	// ListByBranch — يرجع موظفين فرع معين
	// لو branchID فاضي، يرجّع كل الفروع (للـ CEO)
	ListByBranch(ctx context.Context, branchID string) ([]*Employee, error)

	// ListActiveByBranch — يرجع الموظفين الفعّالين بس
	ListActiveByBranch(ctx context.Context, branchID string) ([]*Employee, error)

	// Update — يحدّث بيانات موظف
	Update(ctx context.Context, tx *sqlx.Tx, emp *Employee) error

	// ExistsNationalID — يتحقق إن الرقم القومي موجود (لمنع التكرار)
	// لو excludeID موجود، يستثنيه من الفحص (للتعديل)
	ExistsNationalID(ctx context.Context, nationalID string, excludeID string) (bool, error)
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

const employeeColumns = `
	id, name, branch_id,
	phone, national_id,
	job_title, email, address,
	payment_type, payment_amount,
	hire_date, termination_date,
	active, created_at, updated_at
`

// Create — Ref: FR-EMP-001, FR-EMP-002, FR-EMP-003
func (r *postgresRepo) Create(ctx context.Context, tx *sqlx.Tx, emp *Employee) error {
	const q = `
		INSERT INTO employees (
			id, name, branch_id,
			phone, national_id,
			job_title, email, address,
			payment_type, payment_amount,
			hire_date, termination_date,
			active, created_at, updated_at
		) VALUES (
			$1, $2, $3,
			$4, $5,
			$6, $7, $8,
			$9, $10,
			$11, $12,
			$13, $14, $15
		)
	`
	_, err := tx.ExecContext(ctx, q,
		emp.ID, emp.Name, emp.BranchID,
		emp.Phone, emp.NationalID,
		emp.JobTitle, emp.Email, emp.Address,
		emp.PaymentType, emp.PaymentAmount,
		emp.HireDate, emp.TerminationDate,
		emp.Active, emp.CreatedAt, emp.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create employee: %w", err)
	}
	return nil
}

// GetByID — Ref: FR-EMP-001 (for attendance module)
func (r *postgresRepo) GetByID(ctx context.Context, id string) (*Employee, error) {
	q := `SELECT ` + employeeColumns + ` FROM employees WHERE id = $1`

	var emp Employee
	err := r.db.GetContext(ctx, &emp, q, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get employee by id: %w", err)
	}
	return &emp, nil
}

// GetByNationalID
func (r *postgresRepo) GetByNationalID(ctx context.Context, nationalID string) (*Employee, error) {
	q := `SELECT ` + employeeColumns + ` FROM employees WHERE national_id = $1`

	var emp Employee
	err := r.db.GetContext(ctx, &emp, q, nationalID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get employee by national_id: %w", err)
	}
	return &emp, nil
}

// ListByBranch — Ref: FR-AUTH-004 (BM branch scope)
func (r *postgresRepo) ListByBranch(ctx context.Context, branchID string) ([]*Employee, error) {
	var (
		q    string
		args []any
	)

	if branchID == "" {
		// CEO — كل الفروع
		q = `SELECT ` + employeeColumns + ` FROM employees ORDER BY created_at DESC`
	} else {
		// BM — فرعه بس
		q = `SELECT ` + employeeColumns + ` FROM employees WHERE branch_id = $1 ORDER BY created_at DESC`
		args = append(args, branchID)
	}

	var emps []*Employee
	if err := r.db.SelectContext(ctx, &emps, q, args...); err != nil {
		return nil, fmt.Errorf("list employees: %w", err)
	}
	return emps, nil
}

// ListActiveByBranch
func (r *postgresRepo) ListActiveByBranch(ctx context.Context, branchID string) ([]*Employee, error) {
	var (
		q    string
		args []any
	)

	if branchID == "" {
		q = `SELECT ` + employeeColumns + ` FROM employees WHERE active = TRUE ORDER BY created_at DESC`
	} else {
		q = `SELECT ` + employeeColumns + ` FROM employees WHERE branch_id = $1 AND active = TRUE ORDER BY created_at DESC`
		args = append(args, branchID)
	}

	var emps []*Employee
	if err := r.db.SelectContext(ctx, &emps, q, args...); err != nil {
		return nil, fmt.Errorf("list active employees: %w", err)
	}
	return emps, nil
}

// Update — Ref: FR-EMP-006 (change payment type)
func (r *postgresRepo) Update(ctx context.Context, tx *sqlx.Tx, emp *Employee) error {
	const q = `
		UPDATE employees SET
			name              = $2,
			branch_id         = $3,
			phone             = $4,
			national_id       = $5,
			job_title         = $6,
			email             = $7,
			address           = $8,
			payment_type      = $9,
			payment_amount    = $10,
			hire_date         = $11,
			termination_date  = $12,
			active            = $13,
			updated_at        = $14
		WHERE id = $1
	`
	res, err := tx.ExecContext(ctx, q,
		emp.ID,
		emp.Name, emp.BranchID,
		emp.Phone, emp.NationalID,
		emp.JobTitle, emp.Email, emp.Address,
		emp.PaymentType, emp.PaymentAmount,
		emp.HireDate, emp.TerminationDate,
		emp.Active, emp.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update employee: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrEmployeeNotFound
	}
	return nil
}

// ExistsNationalID — Ref: unique constraint uq_employees_national_id
func (r *postgresRepo) ExistsNationalID(ctx context.Context, nationalID string, excludeID string) (bool, error) {
	var q string
	var args []any

	if excludeID == "" {
		q = `SELECT EXISTS(SELECT 1 FROM employees WHERE national_id = $1)`
		args = []any{nationalID}
	} else {
		q = `SELECT EXISTS(SELECT 1 FROM employees WHERE national_id = $1 AND id != $2)`
		args = []any{nationalID, excludeID}
	}

	var exists bool
	if err := r.db.GetContext(ctx, &exists, q, args...); err != nil {
		return false, fmt.Errorf("exists national_id: %w", err)
	}
	return exists, nil
}

// ============================================================
// Compile-time check
// ============================================================

var _ Repository = (*postgresRepo)(nil)
