-- +goose Up
-- 1. Goods Table
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

-- 2. Inventories Table
CREATE TABLE IF NOT EXISTS inventories (
    id           UUID PRIMARY KEY,
    branch_id    UUID NOT NULL,
    good_id      UUID NOT NULL REFERENCES goods(id) ON DELETE RESTRICT,
    quantity     NUMERIC(12,2) NOT NULL DEFAULT 0 CHECK (quantity >= 0),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_branch_good UNIQUE (branch_id, good_id)
);

-- 3. Inventory Movements Table
CREATE TABLE IF NOT EXISTS inventory_movements (
    id            UUID PRIMARY KEY,
    inventory_id  UUID NOT NULL REFERENCES inventories(id) ON DELETE CASCADE,
    type          TEXT NOT NULL CHECK (type IN ('IN', 'OUT', 'ADJUSTMENT')),
    quantity      NUMERIC(12,2) NOT NULL CHECK (quantity > 0),
    reason        TEXT,
    created_by    UUID NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 4. Supply Requests Table
CREATE TABLE IF NOT EXISTS supply_requests (
    id               UUID PRIMARY KEY,
    branch_id        UUID NOT NULL REFERENCES branches(id),
    status           TEXT NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'APPROVED', 'SHIPPED', 'FULFILLED', 'REJECTED')),
    created_by       UUID NOT NULL,
    notes            TEXT,
    priority         TEXT DEFAULT 'NORMAL',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 5. Supply Request Items Table
CREATE TABLE IF NOT EXISTS supply_request_items (
    id                 UUID PRIMARY KEY,
    request_id         UUID NOT NULL REFERENCES supply_requests(id) ON DELETE CASCADE,
    good_id            UUID NOT NULL REFERENCES goods(id),
    requested_qty      NUMERIC(12,2) NOT NULL CHECK (requested_qty > 0),
    approved_qty       NUMERIC(12,2),
    received_qty       NUMERIC(12,2),
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_inv_branch ON inventories (branch_id);
CREATE INDEX IF NOT EXISTS idx_mov_inventory ON inventory_movements (inventory_id);
CREATE INDEX IF NOT EXISTS idx_sup_req_branch ON supply_requests (branch_id);
CREATE INDEX IF NOT EXISTS idx_sup_item_req ON supply_request_items (request_id);

-- +goose StatementBegin
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'inventory_app') THEN
        EXECUTE 'GRANT SELECT, INSERT, UPDATE, DELETE ON goods, inventories, inventory_movements, supply_requests, supply_request_items TO inventory_app';
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
DROP TABLE supply_request_items;
DROP TABLE supply_requests;
DROP TABLE inventory_movements;
DROP TABLE inventories;
DROP TABLE goods;