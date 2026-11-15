-- +goose Up
-- Shared inventory schema supports Central Kitchen, Inventory, and Transfer APIs.
CREATE TABLE goods (
    id UUID PRIMARY KEY, sku TEXT NOT NULL UNIQUE, name TEXT NOT NULL,
    category TEXT, description TEXT, unit TEXT NOT NULL,
    unit_cost NUMERIC(12,2) NOT NULL CHECK (unit_cost >= 0),
    reorder_level NUMERIC(12,2) NOT NULL DEFAULT 0, max_capacity NUMERIC(12,2),
    is_active BOOLEAN NOT NULL DEFAULT TRUE, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE inventories (
    id UUID PRIMARY KEY, branch_id UUID NOT NULL REFERENCES branches(id),
    good_id UUID NOT NULL REFERENCES goods(id), quantity NUMERIC(12,2) NOT NULL DEFAULT 0 CHECK (quantity >= 0),
    unit_cost NUMERIC(12,2) NOT NULL DEFAULT 0, min_stock NUMERIC(12,2) NOT NULL DEFAULT 0,
    reorder_level NUMERIC(12,2) NOT NULL DEFAULT 0, max_stock NUMERIC(12,2), location_code TEXT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), UNIQUE(branch_id, good_id),
    CHECK (max_stock IS NULL OR max_stock >= GREATEST(min_stock, reorder_level))
);
CREATE TABLE inventory_movements (
    id UUID PRIMARY KEY, inventory_id UUID NOT NULL REFERENCES inventories(id) ON DELETE CASCADE,
    branch_id UUID NOT NULL REFERENCES branches(id), good_id UUID NOT NULL REFERENCES goods(id),
    type TEXT NOT NULL CHECK (type IN ('IN','OUT','ADJUSTMENT')),
    quantity NUMERIC(12,2) NOT NULL CHECK (quantity <> 0),
    quantity_before NUMERIC(12,2) NOT NULL, quantity_after NUMERIC(12,2) NOT NULL,
    unit_cost NUMERIC(12,2), destination_branch_id UUID REFERENCES branches(id), reason TEXT,
    reference_no TEXT UNIQUE, reason_code TEXT, adjustment_kind TEXT, counterparty TEXT,
    created_by UUID NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE SEQUENCE inventory_reference_seq START 1000;
CREATE INDEX idx_inventory_movements_branch_good_created ON inventory_movements(branch_id, good_id, created_at DESC);

CREATE TABLE supply_requests (
    id UUID PRIMARY KEY, branch_id UUID NOT NULL REFERENCES branches(id),
    status TEXT NOT NULL DEFAULT 'PENDING' CHECK(status IN ('PENDING','APPROVED','SHIPPED','FULFILLED','REJECTED')),
    created_by UUID NOT NULL, notes TEXT, priority TEXT DEFAULT 'NORMAL',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE supply_request_items (
    id UUID PRIMARY KEY, request_id UUID NOT NULL REFERENCES supply_requests(id) ON DELETE CASCADE,
    good_id UUID NOT NULL REFERENCES goods(id), requested_qty NUMERIC(12,2) NOT NULL CHECK(requested_qty > 0),
    approved_qty NUMERIC(12,2), received_qty NUMERIC(12,2), created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Cost and sales schemas from their feature-service contracts.
CREATE TABLE menu_item_costs(menu_item_id UUID PRIMARY KEY, cost_amount NUMERIC(12,2) NOT NULL CHECK(cost_amount >= 0), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW());
CREATE TABLE recipe_components(id UUID PRIMARY KEY, menu_item_id UUID NOT NULL, good_id UUID NOT NULL REFERENCES goods(id), quantity NUMERIC(12,2) NOT NULL CHECK(quantity > 0), unit TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW());
CREATE TABLE shifts(id UUID PRIMARY KEY, branch_id UUID NOT NULL REFERENCES branches(id), manager_id UUID NOT NULL, status TEXT NOT NULL DEFAULT 'OPEN' CHECK(status IN ('OPEN','REVIEW','CLOSED')), opened_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), closed_at TIMESTAMPTZ, opening_float NUMERIC(12,2) DEFAULT 0, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW());
CREATE TABLE sales(id UUID PRIMARY KEY, shift_id UUID NOT NULL REFERENCES shifts(id) ON DELETE CASCADE, branch_id UUID NOT NULL REFERENCES branches(id), channel TEXT NOT NULL CHECK(channel IN ('RESTAURANT','DELIVERY')), gross_amount NUMERIC(12,2) NOT NULL DEFAULT 0, discount_amount NUMERIC(12,2) NOT NULL DEFAULT 0, net_amount NUMERIC(12,2) NOT NULL DEFAULT 0, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), created_by UUID NOT NULL);
CREATE TABLE sale_items(id UUID PRIMARY KEY, sale_id UUID NOT NULL REFERENCES sales(id) ON DELETE CASCADE, menu_item_id UUID NOT NULL REFERENCES menu_items(id), quantity NUMERIC(12,2) NOT NULL CHECK(quantity > 0), unit_price NUMERIC(12,2) NOT NULL, total_price NUMERIC(12,2) NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW());
CREATE TABLE discounts(id UUID PRIMARY KEY, shift_id UUID NOT NULL REFERENCES shifts(id) ON DELETE CASCADE, amount NUMERIC(12,2) NOT NULL CHECK(amount > 0), reason TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK(status IN ('ACTIVE','CANCELED')), canceled_reason TEXT, canceled_by UUID, created_by UUID NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW());

CREATE TABLE transfers(
 id UUID PRIMARY KEY, code TEXT NOT NULL UNIQUE, source_branch_id UUID NOT NULL REFERENCES branches(id), dest_branch_id UUID NOT NULL REFERENCES branches(id), good_id UUID NOT NULL REFERENCES goods(id),
 requested_qty NUMERIC(12,2) NOT NULL CHECK(requested_qty > 0), approved_qty NUMERIC(12,2), sent_qty NUMERIC(12,2), received_qty NUMERIC(12,2), shortage_qty NUMERIC(12,2) NOT NULL DEFAULT 0, unit_cost NUMERIC(12,2),
 priority TEXT NOT NULL DEFAULT 'NORMAL', reason TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'PENDING', decision_note TEXT, vehicle_type TEXT, driver_name TEXT, driver_phone TEXT, expected_departure TIMESTAMPTZ, temperature_c NUMERIC(5,2), receive_note TEXT,
 requested_by UUID NOT NULL, requested_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), decided_by UUID, decided_at TIMESTAMPTZ, sent_by UUID, sent_at TIMESTAMPTZ, received_by UUID, received_at TIMESTAMPTZ, updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW());
CREATE TABLE transfer_events(id UUID PRIMARY KEY, transfer_id UUID NOT NULL REFERENCES transfers(id) ON DELETE CASCADE, action TEXT NOT NULL, actor_id UUID NOT NULL, note TEXT, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW());

GRANT SELECT, INSERT, UPDATE, DELETE ON goods, inventories, inventory_movements, supply_requests, supply_request_items TO central_kitchen_app, inventory_app, transfer_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON goods, inventories, inventory_movements TO central_kitchen_app, inventory_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON menu_item_costs, recipe_components TO cost_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON shifts, sales, sale_items, discounts TO sales_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON transfers, transfer_events TO transfer_app;
GRANT USAGE, SELECT ON SEQUENCE inventory_reference_seq TO inventory_app, central_kitchen_app, transfer_app;
GRANT SELECT ON branches, goods, menu_items TO transfer_app, cost_app, sales_app, inventory_app, central_kitchen_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON menu_items, categories TO sales_app;

-- +goose Down
DROP TABLE transfer_events, transfers, discounts, sale_items, sales, shifts, recipe_components, menu_item_costs, supply_request_items, supply_requests, inventory_movements, inventories, goods CASCADE;
DROP SEQUENCE inventory_reference_seq;
