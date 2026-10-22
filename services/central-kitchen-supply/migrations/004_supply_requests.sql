-- Migration: 004_supply_requests.sql
-- Module: Supply Requests

BEGIN;

-- Supply Requests
CREATE TABLE IF NOT EXISTS supply_requests (
    id               UUID PRIMARY KEY,
    branch_id        UUID NOT NULL REFERENCES branches(id),
    status           TEXT NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'APPROVED', 'SHIPPED', 'FULFILLED', 'REJECTED')),
    created_by       UUID NOT NULL,
    notes            TEXT,
    priority         TEXT DEFAULT 'NORMAL', -- NORMAL, URGENT
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Supply Request Items (الأصناف اللي جوه الطلب)
CREATE TABLE IF NOT EXISTS supply_request_items (
    id                 UUID PRIMARY KEY,
    request_id         UUID NOT NULL REFERENCES supply_requests(id) ON DELETE CASCADE,
    good_id            UUID NOT NULL REFERENCES goods(id),
    requested_qty      NUMERIC(12,2) NOT NULL CHECK (requested_qty > 0),
    approved_qty       NUMERIC(12,2),
    received_qty       NUMERIC(12,2),
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sup_req_branch ON supply_requests (branch_id);
CREATE INDEX IF NOT EXISTS idx_sup_req_status ON supply_requests (status);
CREATE INDEX IF NOT EXISTS idx_sup_item_req ON supply_request_items (request_id);

COMMIT;