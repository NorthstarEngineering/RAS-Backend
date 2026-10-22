-- Migration: 001_menu.sql
-- Module: Master Menu & Pricing

BEGIN;

-- Categories
CREATE TABLE IF NOT EXISTS categories (
    id          UUID PRIMARY KEY,
    name        TEXT NOT NULL,
    sort_order  INT NOT NULL DEFAULT 0,
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Menu Items (Master Menu)
CREATE TABLE IF NOT EXISTS menu_items (
    id                    UUID PRIMARY KEY,
    sku                   TEXT NOT NULL UNIQUE,
    name                  TEXT NOT NULL,
    name_en               TEXT,
    category_id           UUID REFERENCES categories(id),
    selling_price         NUMERIC(12,2) NOT NULL CHECK (selling_price >= 0),
    kds_station           TEXT,
    prep_time_minutes     INT,
    description           TEXT,
    is_active             BOOLEAN NOT NULL DEFAULT TRUE,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Menu Item Requests (From Branch Managers)
CREATE TABLE IF NOT EXISTS menu_item_requests (
    id                UUID PRIMARY KEY,
    branch_id         UUID NOT NULL,
    requested_by      UUID NOT NULL,
    item_name         TEXT NOT NULL,
    requested_price   NUMERIC(12,2) NOT NULL,
    description       TEXT,
    justification     TEXT,
    status            TEXT NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'APPROVED', 'REJECTED')),
    rejection_reason  TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_menu_cat ON menu_items (category_id);
CREATE INDEX IF NOT EXISTS idx_req_status ON menu_item_requests (status);

COMMIT;