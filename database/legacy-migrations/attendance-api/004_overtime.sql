-- ============================================================
-- Migration 004: Overtime (Extra Hours)
-- Ref: FR-OT-001..008, BR-011, BR-012
-- ============================================================

BEGIN;

-- ============================================================
-- Overtime table
-- ============================================================

CREATE TABLE IF NOT EXISTS overtime (
    id              UUID PRIMARY KEY,
    employee_id     UUID NOT NULL REFERENCES employees(id),
    branch_id       UUID NOT NULL REFERENCES branches(id),
    work_date       DATE NOT NULL,

    -- ========================================
    -- الساعات والمبلغ
    -- ========================================
    hours           NUMERIC(6,2) NOT NULL CHECK (hours > 0),
    rate_per_hour   NUMERIC(12,2) NOT NULL CHECK (rate_per_hour > 0),
    amount          NUMERIC(12,2) NOT NULL CHECK (amount > 0),

    -- ========================================
    -- الحالة
    -- ========================================
    -- pending:  في انتظار الموافقة
    -- approved: مقبول → payable (BR-011)
    -- rejected: مرفوض → غير مدفوع
    status          TEXT NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending', 'approved', 'rejected')),

    -- ========================================
    -- المصدر
    -- ========================================
    -- manual:      إدخال يدوي من BM/CEO
    -- fingerprint: اقتراح من جهاز البصمة (BR-012)
    source          TEXT NOT NULL DEFAULT 'manual'
                    CHECK (source IN ('manual', 'fingerprint')),

    -- ========================================
    -- الموافقة
    -- ========================================
    approved_by     UUID,
    approved_at     TIMESTAMPTZ,

    -- ========================================
    -- الرفض
    -- ========================================
    rejected_by     UUID,
    rejected_at     TIMESTAMPTZ,
    rejection_reason TEXT,

    -- ========================================
    -- الإنشاء
    -- ========================================
    created_by      UUID NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- ========================================
    -- Constraints
    -- ========================================

    -- Approved: لازم يكون فيه approved_by + approved_at
    -- ومينفعش يكون فيه rejected_by
    CONSTRAINT chk_approval_consistency CHECK (
        (status = 'approved' AND approved_by IS NOT NULL AND approved_at IS NOT NULL
            AND rejected_by IS NULL AND rejected_at IS NULL)
        OR
        (status != 'approved' AND approved_by IS NULL AND approved_at IS NULL)
    ),

    -- Rejected: لازم يكون فيه rejected_by + rejected_at + reason
    CONSTRAINT chk_rejection_consistency CHECK (
        (status = 'rejected' AND rejected_by IS NOT NULL AND rejected_at IS NOT NULL
            AND rejection_reason IS NOT NULL)
        OR
        (status != 'rejected' AND rejected_by IS NULL AND rejected_at IS NULL
            AND rejection_reason IS NULL)
    )
);

-- ============================================================
-- Indexes
-- ============================================================

CREATE INDEX IF NOT EXISTS idx_ot_employee_date
    ON overtime (employee_id, work_date);

CREATE INDEX IF NOT EXISTS idx_ot_branch_date
    ON overtime (branch_id, work_date);

CREATE INDEX IF NOT EXISTS idx_ot_status
    ON overtime (status);

-- Partial index: pending بس (للداشبورد)
CREATE INDEX IF NOT EXISTS idx_ot_pending
    ON overtime (branch_id, created_at)
    WHERE status = 'pending';

COMMIT;