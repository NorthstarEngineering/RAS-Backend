-- +goose Up
CREATE TABLE IF NOT EXISTS menu_item_costs (
    menu_item_id    UUID PRIMARY KEY, 
    cost_amount     NUMERIC(12,2) NOT NULL CHECK (cost_amount >= 0),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS recipe_components (
    id              UUID PRIMARY KEY,
    menu_item_id    UUID NOT NULL,
    good_id         UUID NOT NULL,
    quantity        NUMERIC(12,2) NOT NULL CHECK (quantity > 0),
    unit            TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_recipe_menu ON recipe_components (menu_item_id);

-- +goose StatementBegin
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'cost_app') THEN
        EXECUTE 'GRANT SELECT, INSERT, UPDATE, DELETE ON menu_item_costs, recipe_components TO cost_app';
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
DROP TABLE recipe_components;
DROP TABLE menu_item_costs;