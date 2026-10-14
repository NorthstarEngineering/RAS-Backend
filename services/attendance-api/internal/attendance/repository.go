package attendance

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

// ============================================================
// Repository Interface — العقد مع طبقة الـ DB
// ============================================================

type Repository interface {
	// ---------- AttendanceRecord ----------

	// GetOpenRecord — يرجع أي سجل مفتوح للموظف
	GetOpenRecord(ctx context.Context, employeeID string) (*AttendanceRecord, error)

	// GetRecordByDate — يرجع سجل الموظف في تاريخ معين
	GetRecordByDate(ctx context.Context, employeeID string, date time.Time) (*AttendanceRecord, error)

	// GetRecordByID — يرجع سجل بالـ ID
	GetRecordByID(ctx context.Context, id string) (*AttendanceRecord, error)

	// CreateRecord — ينشئ سجل حضور جديد (داخل transaction)
	CreateRecord(ctx context.Context, tx *sqlx.Tx, rec *AttendanceRecord) error

	// UpdateCheckOut — يسجّل وقت الانصراف ويقفل السجل
	UpdateCheckOut(ctx context.Context, tx *sqlx.Tx, id string, at time.Time) error

	// ---------- Deduction ----------

	// CreateDeduction — ينشئ خصم جديد (داخل transaction)
	CreateDeduction(ctx context.Context, tx *sqlx.Tx, d *Deduction) error

	// GetDeductionByID — يرجع خصم بالـ ID
	GetDeductionByID(ctx context.Context, id string) (*Deduction, error)

	// GetDeductionByAttendance — يرجع خصم مرتبط بسجل حضور
	GetDeductionByAttendance(ctx context.Context, attendanceID string) (*Deduction, error)

	// UpdateDeduction — يحدّث تعديل/إلغاء خصم (داخل transaction)
	UpdateDeduction(ctx context.Context, tx *sqlx.Tx, d *Deduction) error
}

// ============================================================
// PostgreSQL Implementation
// ============================================================

type postgresRepo struct {
	db *sqlx.DB
}

// NewRepository — constructor
func NewRepository(db *sqlx.DB) Repository {
	return &postgresRepo{db: db}
}

// ============================================================
// AttendanceRecord Queries
// ============================================================

func (r *postgresRepo) GetOpenRecord(ctx context.Context, employeeID string) (*AttendanceRecord, error) {
	const q = `
		SELECT
			id, employee_id, branch_id, work_date,
			check_in_at, check_out_at,
			status, source,
			created_at, updated_at
		FROM attendance_records
		WHERE employee_id = $1
		  AND status = 'open'
		ORDER BY check_in_at DESC
		LIMIT 1
	`
	var rec AttendanceRecord
	err := r.db.GetContext(ctx, &rec, q, employeeID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil // مش موجود
	}
	if err != nil {
		return nil, fmt.Errorf("get open record: %w", err)
	}
	return &rec, nil
}

func (r *postgresRepo) GetRecordByDate(ctx context.Context, employeeID string, date time.Time) (*AttendanceRecord, error) {
	const q = `
		SELECT
			id, employee_id, branch_id, work_date,
			check_in_at, check_out_at,
			status, source,
			created_at, updated_at
		FROM attendance_records
		WHERE employee_id = $1
		  AND work_date = $2::date
		LIMIT 1
	`
	var rec AttendanceRecord
	err := r.db.GetContext(ctx, &rec, q, employeeID, date)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get record by date: %w", err)
	}
	return &rec, nil
}

func (r *postgresRepo) GetRecordByID(ctx context.Context, id string) (*AttendanceRecord, error) {
	const q = `
		SELECT
			id, employee_id, branch_id, work_date,
			check_in_at, check_out_at,
			status, source,
			created_at, updated_at
		FROM attendance_records
		WHERE id = $1
	`
	var rec AttendanceRecord
	err := r.db.GetContext(ctx, &rec, q, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get record by id: %w", err)
	}
	return &rec, nil
}

func (r *postgresRepo) CreateRecord(ctx context.Context, tx *sqlx.Tx, rec *AttendanceRecord) error {
	const q = `
		INSERT INTO attendance_records (
			id, employee_id, branch_id, work_date,
			check_in_at, check_out_at,
			status, source,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4,
			$5, $6,
			$7, $8,
			$9, $10
		)
	`
	_, err := tx.ExecContext(ctx, q,
		rec.ID, rec.EmployeeID, rec.BranchID, rec.WorkDate,
		rec.CheckInAt, rec.CheckOutAt,
		rec.Status, rec.Source,
		rec.CreatedAt, rec.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create record: %w", err)
	}
	return nil
}

func (r *postgresRepo) UpdateCheckOut(ctx context.Context, tx *sqlx.Tx, id string, at time.Time) error {
	const q = `
		UPDATE attendance_records
		SET check_out_at = $2,
		    status       = 'closed',
		    updated_at   = NOW()
		WHERE id = $1
		  AND status = 'open'
	`
	res, err := tx.ExecContext(ctx, q, id, at)
	if err != nil {
		return fmt.Errorf("update checkout: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrAlreadyCheckedOut
	}
	return nil
}

// ============================================================
// Deduction Queries
// ============================================================

func (r *postgresRepo) CreateDeduction(ctx context.Context, tx *sqlx.Tx, d *Deduction) error {
	const q = `
		INSERT INTO deductions (
			id, attendance_id, employee_id, branch_id, work_date,
			late_minutes, completed_hours, rate_per_hour, original_amount,
			adjusted_amount, adjustment_reason, adjusted_by, adjusted_at,
			cancelled, cancelled_by, cancelled_at, cancel_reason,
			source, created_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11, $12, $13,
			$14, $15, $16, $17,
			$18, $19
		)
	`
	_, err := tx.ExecContext(ctx, q,
		d.ID, d.AttendanceID, d.EmployeeID, d.BranchID, d.WorkDate,
		d.LateMinutes, d.CompletedHours, d.RatePerHour, d.OriginalAmount,
		d.AdjustedAmount, d.AdjustmentReason, d.AdjustedBy, d.AdjustedAt,
		d.Cancelled, d.CancelledBy, d.CancelledAt, d.CancelReason,
		d.Source, d.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create deduction: %w", err)
	}
	return nil
}

func (r *postgresRepo) GetDeductionByID(ctx context.Context, id string) (*Deduction, error) {
	const q = `
		SELECT
			id, attendance_id, employee_id, branch_id, work_date,
			late_minutes, completed_hours, rate_per_hour, original_amount,
			adjusted_amount, adjustment_reason, adjusted_by, adjusted_at,
			cancelled, cancelled_by, cancelled_at, cancel_reason,
			source, created_at
		FROM deductions
		WHERE id = $1
	`
	var d Deduction
	err := r.db.GetContext(ctx, &d, q, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get deduction by id: %w", err)
	}
	return &d, nil
}

func (r *postgresRepo) GetDeductionByAttendance(ctx context.Context, attendanceID string) (*Deduction, error) {
	const q = `
		SELECT
			id, attendance_id, employee_id, branch_id, work_date,
			late_minutes, completed_hours, rate_per_hour, original_amount,
			adjusted_amount, adjustment_reason, adjusted_by, adjusted_at,
			cancelled, cancelled_by, cancelled_at, cancel_reason,
			source, created_at
		FROM deductions
		WHERE attendance_id = $1
		LIMIT 1
	`
	var d Deduction
	err := r.db.GetContext(ctx, &d, q, attendanceID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get deduction by attendance: %w", err)
	}
	return &d, nil
}

func (r *postgresRepo) UpdateDeduction(ctx context.Context, tx *sqlx.Tx, d *Deduction) error {
	const q = `
		UPDATE deductions SET
			adjusted_amount   = $2,
			adjustment_reason = $3,
			adjusted_by       = $4,
			adjusted_at       = $5,
			cancelled         = $6,
			cancelled_by      = $7,
			cancelled_at      = $8,
			cancel_reason     = $9
		WHERE id = $1
	`
	res, err := tx.ExecContext(ctx, q,
		d.ID,
		d.AdjustedAmount, d.AdjustmentReason, d.AdjustedBy, d.AdjustedAt,
		d.Cancelled, d.CancelledBy, d.CancelledAt, d.CancelReason,
	)
	if err != nil {
		return fmt.Errorf("update deduction: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrDeductionNotFound
	}
	return nil
}

// ============================================================
// Compile-time check
// ============================================================

var _ Repository = (*postgresRepo)(nil)
