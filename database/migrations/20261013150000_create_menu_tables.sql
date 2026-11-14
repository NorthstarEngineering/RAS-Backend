-- +goose Up
CREATE TABLE categories (
    id          UUID PRIMARY KEY,
    name        TEXT NOT NULL,
    sort_order  INT NOT NULL DEFAULT 0,
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE menu_items (
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

CREATE TABLE menu_item_requests (
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

CREATE INDEX idx_menu_cat ON menu_items (category_id);
CREATE INDEX idx_req_status ON menu_item_requests (status);

-- +goose StatementBegin
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'menu_app') THEN
        EXECUTE 'GRANT SELECT, INSERT, UPDATE, DELETE ON categories, menu_items, menu_item_requests TO menu_app';
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
DROP TABLE menu_item_requests;
DROP TABLE menu_items;
DROP TABLE categories;