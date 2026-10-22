-- Migration: 001_inventory.sql
-- Module: Inventory (Central Kitchen & Branches)
-- Ref: FR-INV-001..006, FR-GOODS-001..004

BEGIN;

-- Table: inventories
CREATE TABLE IF NOT EXISTS inventories (
    id           UUID PRIMARY KEY,
    branch_id    UUID NOT NULL,
    good_id      UUID NOT NULL,
    quantity     NUMERIC(12,2) NOT NULL DEFAULT 0 CHECK (quantity >= 0),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_branch_good UNIQUE (branch_id, good_id)
);

CREATE INDEX IF NOT EXISTS idx_inv_branch ON inventories (branch_id);
CREATE INDEX IF NOT EXISTS idx_inv_good ON inventories (good_id);


-- Table: inventory_movements
CREATE TABLE IF NOT EXISTS inventory_movements (
    id            UUID PRIMARY KEY,
    inventory_id  UUID NOT NULL REFERENCES inventories(id) ON DELETE CASCADE,
    type          TEXT NOT NULL CHECK (type IN ('IN', 'OUT', 'ADJUSTMENT')),
    quantity      NUMERIC(12,2) NOT NULL CHECK (quantity > 0),
    reason        TEXT,
    created_by    UUID NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_mov_inventory ON inventory_movements (inventory_id);
CREATE INDEX IF NOT EXISTS idx_mov_date ON inventory_movements (created_at DESC);

COMMIT;