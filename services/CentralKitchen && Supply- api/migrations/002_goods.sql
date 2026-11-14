-- Migration: 002_goods.sql
-- Module: Goods (Central Kitchen Catalog)

BEGIN;

-- Table: goods
CREATE TABLE IF NOT EXISTS goods (
    id               UUID PRIMARY KEY,
    sku              TEXT NOT NULL UNIQUE,
    name             TEXT NOT NULL,
    category         TEXT,
    description      TEXT,
    unit             TEXT NOT NULL,
    unit_cost        NUMERIC(12,2) NOT NULL CHECK (unit_cost >= 0),
    reorder_level    NUMERIC(12,2) NOT NULL DEFAULT 0,
    max_capacity     NUMERIC(12,2),
    is_active        BOOLEAN NOT NULL DEFAULT TRUE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_goods_sku ON goods (sku);
CREATE INDEX IF NOT EXISTS idx_goods_active ON goods (is_active) WHERE is_active = TRUE;

-- Add FK from inventories to goods
ALTER TABLE inventories 
    ADD CONSTRAINT fk_inv_good 
    FOREIGN KEY (good_id) REFERENCES goods(id) ON DELETE RESTRICT;

COMMIT;