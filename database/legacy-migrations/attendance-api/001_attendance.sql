-- Migration: 001_attendance.sql
-- Module:    Attendance, Lateness & Automatic Deductions
-- Date:      2026-10-10
-- Ref: FR-ATT-001..004, FR-DED-001..009, NFR-DATA-001, BR-006..010
-- NOTE: Stub tables (employees, branches) are temporary for standalone dev.

BEGIN;

-- ============================================================
-- TEMPORARY STUBS
-- ============================================================

CREATE TABLE IF NOT EXISTS employees (
    id         UUID PRIMARY KEY,
    branch_id  UUID NOT NULL,
    active     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS branches (
    id                     UUID PRIMARY KEY,
    official_start_time    TEXT NOT NULL DEFAULT '09:00',
    lateness_rate_per_hour NUMERIC(12,2) NOT NULL DEFAULT 0,
    is_central_kitchen     BOOLEAN NOT NULL DEFAULT FALSE,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ============================================================
-- attendance_records
-- Ref: FR-ATT-001, FR-ATT-002
-- ============================================================

CREATE TABLE IF NOT EXISTS attendance_records (
    id           UUID PRIMARY KEY,
    employee_id  UUID NOT NULL REFERENCES employees(id),
    branch_id    UUID NOT NULL REFERENCES branches(id),
    work_date    DATE NOT NULL,
    check_in_at  TIMESTAMPTZ NOT NULL,
    check_out_at TIMESTAMPTZ,
    status       TEXT NOT NULL CHECK (status IN ('open', 'closed')),
    source       TEXT NOT NULL CHECK (source IN ('fingerprint', 'manual')),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_employee_workdate UNIQUE (employee_id, work_date),
    CONSTRAINT chk_checkout_after_checkin
        CHECK (check_out_at IS NULL OR check_out_at > check_in_at)
);

CREATE INDEX IF NOT EXISTS idx_att_branch_date
    ON attendance_records (branch_id, work_date);

CREATE INDEX IF NOT EXISTS idx_att_employee_date
    ON attendance_records (employee_id, work_date);

CREATE INDEX IF NOT EXISTS idx_att_open
    ON attendance_records (employee_id)
    WHERE status = 'open';

-- ============================================================
-- deductions
-- Ref: FR-DED-001..009
-- ============================================================

CREATE TABLE IF NOT EXISTS deductions (
    id                UUID PRIMARY KEY,
    attendance_id     UUID NOT NULL REFERENCES attendance_records(id) ON DELETE CASCADE,
    employee_id       UUID NOT NULL REFERENCES employees(id),
    branch_id         UUID NOT NULL REFERENCES branches(id),
    work_date         DATE NOT NULL,

    late_minutes      INT NOT NULL CHECK (late_minutes >= 0),
    completed_hours   INT NOT NULL CHECK (completed_hours >= 0),
    rate_per_hour     NUMERIC(12,2) NOT NULL CHECK (rate_per_hour >= 0),
    original_amount   NUMERIC(12,2) NOT NULL CHECK (original_amount >= 0),
    source            TEXT NOT NULL DEFAULT 'automatic'
                      CHECK (source IN ('automatic', 'manual')),

    adjusted_amount   NUMERIC(12,2)
                      CHECK (adjusted_amount IS NULL OR adjusted_amount >= 0),
    adjustment_reason TEXT,
    adjusted_by       UUID,
    adjusted_at       TIMESTAMPTZ,

    cancelled         BOOLEAN NOT NULL DEFAULT FALSE,
    cancelled_by      UUID,
    cancelled_at      TIMESTAMPTZ,
    cancel_reason     TEXT,

    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_adjustment_complete CHECK (
        (adjusted_amount IS NULL AND adjustment_reason IS NULL
            AND adjusted_by IS NULL AND adjusted_at IS NULL)
        OR
        (adjusted_amount IS NOT NULL AND adjustment_reason IS NOT NULL
            AND adjusted_by IS NOT NULL AND adjusted_at IS NOT NULL)
    ),

    CONSTRAINT chk_cancel_complete CHECK (
        (cancelled = FALSE AND cancelled_by IS NULL
            AND cancelled_at IS NULL AND cancel_reason IS NULL)
        OR
        (cancelled = TRUE AND cancelled_by IS NOT NULL
            AND cancelled_at IS NOT NULL AND cancel_reason IS NOT NULL)
    ),

       CONSTRAINT chk_cancel_complete CHECK (
        (cancelled = FALSE AND cancelled_by IS NULL
            AND cancelled_at IS NULL AND cancel_reason IS NULL)
        OR
        (cancelled = TRUE AND cancelled_by IS NOT NULL
            AND cancelled_at IS NOT NULL AND cancel_reason IS NOT NULL)
    )

);

CREATE INDEX IF NOT EXISTS idx_ded_employee_date
    ON deductions (employee_id, work_date);

CREATE INDEX IF NOT EXISTS idx_ded_branch_date
    ON deductions (branch_id, work_date);

CREATE INDEX IF NOT EXISTS idx_ded_attendance
    ON deductions (attendance_id);

CREATE INDEX IF NOT EXISTS idx_ded_active
    ON deductions (branch_id, work_date)
    WHERE cancelled = FALSE;

COMMIT;