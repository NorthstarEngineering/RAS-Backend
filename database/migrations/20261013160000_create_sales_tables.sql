-- +goose Up
CREATE TABLE IF NOT EXISTS shifts (
    id              UUID PRIMARY KEY,
    branch_id       UUID NOT NULL,
    manager_id      UUID NOT NULL, 
    status          TEXT NOT NULL DEFAULT 'OPEN' CHECK (status IN ('OPEN', 'REVIEW', 'CLOSED')),
    opened_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    closed_at       TIMESTAMPTZ,
    opening_float   NUMERIC(12,2) DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS sales (
    id              UUID PRIMARY KEY,
    shift_id        UUID NOT NULL REFERENCES shifts(id) ON DELETE CASCADE,
    branch_id       UUID NOT NULL,
    channel         TEXT NOT NULL CHECK (channel IN ('RESTAURANT', 'DELIVERY')),
    gross_amount    NUMERIC(12,2) NOT NULL DEFAULT 0,
    discount_amount NUMERIC(12,2) NOT NULL DEFAULT 0,
    net_amount      NUMERIC(12,2) NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by      UUID NOT NULL
);

CREATE TABLE IF NOT EXISTS sale_items (
    id              UUID PRIMARY KEY,
    sale_id         UUID NOT NULL REFERENCES sales(id) ON DELETE CASCADE,
    menu_item_id    UUID NOT NULL,
    quantity        NUMERIC(12,2) NOT NULL CHECK (quantity > 0),
    unit_price      NUMERIC(12,2) NOT NULL,
    total_price     NUMERIC(12,2) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS discounts (
    id              UUID PRIMARY KEY,
    shift_id        UUID NOT NULL REFERENCES shifts(id) ON DELETE CASCADE,
    amount          NUMERIC(12,2) NOT NULL CHECK (amount > 0),
    reason          TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'CANCELED')),
    canceled_reason TEXT,
    canceled_by     UUID,
    created_by      UUID NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_shifts_branch ON shifts (branch_id);
CREATE INDEX IF NOT EXISTS idx_sales_shift ON sales (shift_id);
CREATE INDEX IF NOT EXISTS idx_sale_items_sale ON sale_items (sale_id);
CREATE INDEX IF NOT EXISTS idx_discounts_shift ON discounts (shift_id);

-- +goose StatementBegin
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'sales_app') THEN
        EXECUTE 'GRANT SELECT, INSERT, UPDATE, DELETE ON shifts, sales, sale_items, discounts TO sales_app';
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
DROP TABLE discounts;
DROP TABLE sale_items;
DROP TABLE sales;
DROP TABLE shifts;