-- ===== Balance table: stock limits + physical location =====
ALTER TABLE inventories
    ADD COLUMN IF NOT EXISTS min_stock     NUMERIC(12,2) NOT NULL DEFAULT 0,   -- safety level: below it => CRITICAL
    ADD COLUMN IF NOT EXISTS reorder_level NUMERIC(12,2) NOT NULL DEFAULT 0,   -- below it (but above safety) => LOW
    ADD COLUMN IF NOT EXISTS max_stock     NUMERIC(12,2),                      -- storage capacity (optional)
    ADD COLUMN IF NOT EXISTS location_code TEXT;                               -- e.g. BOH-01
 
ALTER TABLE inventories
    ADD CONSTRAINT chk_inventories_limits CHECK (
        min_stock >= 0 AND reorder_level >= 0 AND
        (max_stock IS NULL OR max_stock >= GREATEST(min_stock, reorder_level))
    );
 
-- ===== Movement ledger: reference number, reason code, counterparty =====
CREATE SEQUENCE IF NOT EXISTS inventory_reference_seq START 1000;
 
ALTER TABLE inventory_movements
    ADD COLUMN IF NOT EXISTS reference_no    TEXT,   -- REC-1001 / OUT-1002 / TRF-1003 / ADJ-2026-1004
    ADD COLUMN IF NOT EXISTS reason_code     TEXT,   -- structured reason (the free text stays in "reason")
    ADD COLUMN IF NOT EXISTS adjustment_kind TEXT,   -- SHORTAGE / SURPLUS / OPENING
    ADD COLUMN IF NOT EXISTS counterparty    TEXT;   -- department / supplier / vehicle (free text)
 
-- Existing rows keep NULL here: the ledger is append-only, so we do not backfill.
ALTER TABLE inventory_movements
    ADD CONSTRAINT uq_mov_reference_no UNIQUE (reference_no),
    ADD CONSTRAINT chk_mov_reason_code CHECK (
        reason_code IS NULL OR (
            type IN ('OUT', 'ADJUSTMENT') AND
            reason_code IN ('PREP_WASTE', 'EXPIRED_DAMAGED', 'ENTRY_ERROR',
                            'QUALITY_SAMPLE', 'SPILLAGE_BREAKAGE', 'OTHER')
        )
    ),
    ADD CONSTRAINT chk_mov_adjustment_kind CHECK (
        adjustment_kind IS NULL OR
        (type = 'ADJUSTMENT' AND adjustment_kind IN ('SHORTAGE', 'SURPLUS', 'OPENING'))
    ),
    ADD CONSTRAINT chk_mov_counterparty_len CHECK (
        counterparty IS NULL OR char_length(counterparty) <= 120
    );
 
CREATE INDEX IF NOT EXISTS idx_mov_branch_good_created
    ON inventory_movements (branch_id, good_id, created_at DESC);