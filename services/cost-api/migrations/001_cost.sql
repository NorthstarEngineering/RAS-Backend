-- Migration: 001_cost.sql
-- Module: Cost & Profitability

BEGIN;

-- 1. جدول تكاليف أصناف المنيو الإجمالية
CREATE TABLE IF NOT EXISTS menu_item_costs (
    menu_item_id    UUID PRIMARY KEY, 
    cost_amount     NUMERIC(12,2) NOT NULL CHECK (cost_amount >= 0),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 2. جدول مكونات الوصفة (BOM)
CREATE TABLE IF NOT EXISTS recipe_components (
    id              UUID PRIMARY KEY,
    menu_item_id    UUID NOT NULL,
    good_id         UUID NOT NULL, -- مربوط بجدول الـ goods في الـ Inventory API
    quantity        NUMERIC(12,2) NOT NULL CHECK (quantity > 0),
    unit            TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_recipe_menu ON recipe_components (menu_item_id);

COMMIT;