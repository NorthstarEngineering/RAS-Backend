-- +goose Up
CREATE TABLE employees (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    branch_id UUID NOT NULL REFERENCES branches(id),
    phone TEXT NOT NULL,
    national_id TEXT NOT NULL,
    job_title TEXT,
    email TEXT,
    address TEXT,
    payment_type TEXT NOT NULL CHECK (payment_type IN ('daily', 'monthly')),
    payment_amount NUMERIC(12,2) NOT NULL CHECK (payment_amount > 0),
    hire_date DATE NOT NULL,
    termination_date DATE,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_national_id_format CHECK (national_id ~ '^[0-9]{14}$'),
    CONSTRAINT chk_phone_format CHECK (phone ~ '^[0-9]{10,15}$'),
    CONSTRAINT chk_termination_after_hire CHECK (termination_date IS NULL OR termination_date >= hire_date),
    CONSTRAINT chk_active_termination_consistency CHECK (
        (active = TRUE AND termination_date IS NULL)
        OR (active = FALSE AND termination_date IS NOT NULL)
    )
);
CREATE UNIQUE INDEX uq_employees_national_id ON employees (national_id);
CREATE INDEX idx_employees_branch ON employees (branch_id);
CREATE INDEX idx_employees_active ON employees (active) WHERE active = TRUE;

CREATE TABLE attendance_records (
    id UUID PRIMARY KEY,
    employee_id UUID NOT NULL REFERENCES employees(id),
    branch_id UUID NOT NULL REFERENCES branches(id),
    work_date DATE NOT NULL,
    check_in_at TIMESTAMPTZ NOT NULL,
    check_out_at TIMESTAMPTZ,
    status TEXT NOT NULL CHECK (status IN ('open', 'closed')),
    source TEXT NOT NULL CHECK (source IN ('fingerprint', 'manual')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_employee_workdate UNIQUE (employee_id, work_date),
    CONSTRAINT chk_checkout_after_checkin CHECK (check_out_at IS NULL OR check_out_at > check_in_at)
);
CREATE INDEX idx_att_branch_date ON attendance_records (branch_id, work_date);
CREATE INDEX idx_att_employee_date ON attendance_records (employee_id, work_date);
CREATE INDEX idx_att_open ON attendance_records (employee_id) WHERE status = 'open';

CREATE TABLE deductions (
    id UUID PRIMARY KEY,
    attendance_id UUID NOT NULL REFERENCES attendance_records(id) ON DELETE CASCADE,
    employee_id UUID NOT NULL REFERENCES employees(id),
    branch_id UUID NOT NULL REFERENCES branches(id),
    work_date DATE NOT NULL,
    late_minutes INT NOT NULL CHECK (late_minutes >= 0),
    completed_hours INT NOT NULL CHECK (completed_hours >= 0),
    rate_per_hour NUMERIC(12,2) NOT NULL CHECK (rate_per_hour >= 0),
    original_amount NUMERIC(12,2) NOT NULL CHECK (original_amount >= 0),
    source TEXT NOT NULL DEFAULT 'automatic' CHECK (source IN ('automatic', 'manual')),
    adjusted_amount NUMERIC(12,2) CHECK (adjusted_amount IS NULL OR adjusted_amount >= 0),
    adjustment_reason TEXT,
    adjusted_by UUID,
    adjusted_at TIMESTAMPTZ,
    cancelled BOOLEAN NOT NULL DEFAULT FALSE,
    cancelled_by UUID,
    cancelled_at TIMESTAMPTZ,
    cancel_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_adjustment_complete CHECK (
        (adjusted_amount IS NULL AND adjustment_reason IS NULL AND adjusted_by IS NULL AND adjusted_at IS NULL)
        OR (adjusted_amount IS NOT NULL AND adjustment_reason IS NOT NULL AND adjusted_by IS NOT NULL AND adjusted_at IS NOT NULL)
    ),
    CONSTRAINT chk_cancel_complete CHECK (
        (cancelled = FALSE AND cancelled_by IS NULL AND cancelled_at IS NULL AND cancel_reason IS NULL)
        OR (cancelled = TRUE AND cancelled_by IS NOT NULL AND cancelled_at IS NOT NULL AND cancel_reason IS NOT NULL)
    )
);
CREATE INDEX idx_ded_employee_date ON deductions (employee_id, work_date);
CREATE INDEX idx_ded_branch_date ON deductions (branch_id, work_date);
CREATE INDEX idx_ded_attendance ON deductions (attendance_id);
CREATE INDEX idx_ded_active ON deductions (branch_id, work_date) WHERE cancelled = FALSE;

CREATE TABLE overtime (
    id UUID PRIMARY KEY,
    employee_id UUID NOT NULL REFERENCES employees(id),
    branch_id UUID NOT NULL REFERENCES branches(id),
    work_date DATE NOT NULL,
    hours NUMERIC(6,2) NOT NULL CHECK (hours > 0),
    rate_per_hour NUMERIC(12,2) NOT NULL CHECK (rate_per_hour > 0),
    amount NUMERIC(12,2) NOT NULL CHECK (amount > 0),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected')),
    source TEXT NOT NULL DEFAULT 'manual' CHECK (source IN ('manual', 'fingerprint')),
    approved_by UUID,
    approved_at TIMESTAMPTZ,
    rejected_by UUID,
    rejected_at TIMESTAMPTZ,
    rejection_reason TEXT,
    created_by UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_approval_consistency CHECK (
        (status = 'approved' AND approved_by IS NOT NULL AND approved_at IS NOT NULL AND rejected_by IS NULL AND rejected_at IS NULL)
        OR (status <> 'approved' AND approved_by IS NULL AND approved_at IS NULL)
    ),
    CONSTRAINT chk_rejection_consistency CHECK (
        (status = 'rejected' AND rejected_by IS NOT NULL AND rejected_at IS NOT NULL AND rejection_reason IS NOT NULL)
        OR (status <> 'rejected' AND rejected_by IS NULL AND rejected_at IS NULL AND rejection_reason IS NULL)
    )
);
CREATE INDEX idx_ot_employee_date ON overtime (employee_id, work_date);
CREATE INDEX idx_ot_branch_date ON overtime (branch_id, work_date);
CREATE INDEX idx_ot_status ON overtime (status);
CREATE INDEX idx_ot_pending ON overtime (branch_id, created_at) WHERE status = 'pending';

DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'attendance_app') THEN
        EXECUTE 'GRANT SELECT, INSERT, UPDATE, DELETE ON employees, attendance_records, deductions, overtime TO attendance_app';
    END IF;
END $$;

-- +goose Down
DROP TABLE overtime;
DROP TABLE deductions;
DROP TABLE attendance_records;
DROP TABLE employees;
