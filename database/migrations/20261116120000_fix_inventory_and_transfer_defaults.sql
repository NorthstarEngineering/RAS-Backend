-- +goose Up
ALTER TABLE inventories ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
ALTER TABLE transfers ALTER COLUMN code SET DEFAULT ('TR-' || gen_random_uuid()::text);

-- +goose Down
ALTER TABLE transfers ALTER COLUMN code DROP DEFAULT;
ALTER TABLE inventories DROP COLUMN created_at;
