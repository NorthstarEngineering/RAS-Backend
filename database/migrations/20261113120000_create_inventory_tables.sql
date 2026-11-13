-- +goose Up
CREATE TABLE goods (
    id UUID PRIMARY KEY,
    sku TEXT NOT NULL,
    name TEXT NOT NULL,
    unit TEXT NOT NULL
);

CREATE TABLE inventories (
    id UUID PRIMARY KEY,
    branch_id UUID NOT NULL REFERENCES branches (id),
    good_id UUID NOT NULL REFERENCES goods (id),
    quantity NUMERIC(12,2) NOT NULL DEFAULT 0,
    unit_cost NUMERIC(12,2) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    min_stock NUMERIC(12,2) NOT NULL DEFAULT 0,
    reorder_level NUMERIC(12,2) NOT NULL DEFAULT 0,
    max_stock NUMERIC(12,2),
    location_code TEXT,
    CONSTRAINT uq_inventories_branch_good UNIQUE (branch_id, good_id),
    CONSTRAINT chk_inventories_quantity_non_negative CHECK (quantity >= 0),
    CONSTRAINT chk_inventories_unit_cost_non_negative CHECK (unit_cost >= 0),
    CONSTRAINT chk_inventories_limits CHECK (
        min_stock >= 0 AND reorder_level >= 0 AND
        (max_stock IS NULL OR max_stock >= GREATEST(min_stock, reorder_level))
    )
);
CREATE INDEX idx_inventories_branch ON inventories (branch_id);

CREATE SEQUENCE inventory_reference_seq START 1000;

CREATE TABLE inventory_movements (
    id UUID PRIMARY KEY,
    inventory_id UUID NOT NULL REFERENCES inventories (id),
    branch_id UUID NOT NULL REFERENCES branches (id),
    good_id UUID NOT NULL REFERENCES goods (id),
    type TEXT NOT NULL,
    quantity NUMERIC(12,2) NOT NULL,
    quantity_before NUMERIC(12,2) NOT NULL,
    quantity_after NUMERIC(12,2) NOT NULL,
    unit_cost NUMERIC(12,2),
    destination_branch_id UUID REFERENCES branches (id),
    reason TEXT,
    created_by UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reference_no TEXT,
    reason_code TEXT,
    adjustment_kind TEXT,
    counterparty TEXT,
    CONSTRAINT uq_mov_reference_no UNIQUE (reference_no),
    CONSTRAINT chk_mov_type CHECK (type IN ('IN', 'OUT', 'ADJUSTMENT')),
    CONSTRAINT chk_mov_quantity CHECK (
        (type IN ('IN', 'OUT') AND quantity > 0) OR
        (type = 'ADJUSTMENT' AND quantity >= 0)
    ),
    CONSTRAINT chk_mov_after_non_negative CHECK (quantity_after >= 0),
    CONSTRAINT chk_mov_math CHECK (
        (type = 'IN' AND quantity_after = quantity_before + quantity) OR
        (type = 'OUT' AND quantity_after = quantity_before - quantity) OR
        (type = 'ADJUSTMENT' AND quantity_after = quantity)
    ),
    CONSTRAINT chk_mov_reason CHECK (type = 'IN' OR (reason IS NOT NULL AND btrim(reason) <> '')),
    CONSTRAINT chk_mov_destination CHECK (
        destination_branch_id IS NULL OR (type = 'OUT' AND destination_branch_id <> branch_id)
    ),
    CONSTRAINT chk_mov_unit_cost CHECK (unit_cost IS NULL OR unit_cost >= 0),
    CONSTRAINT chk_mov_reason_code CHECK (
        reason_code IS NULL OR (
            type IN ('OUT', 'ADJUSTMENT') AND
            reason_code IN ('PREP_WASTE', 'EXPIRED_DAMAGED', 'ENTRY_ERROR',
                            'QUALITY_SAMPLE', 'SPILLAGE_BREAKAGE', 'OTHER')
        )
    ),
    CONSTRAINT chk_mov_adjustment_kind CHECK (
        adjustment_kind IS NULL OR
        (type = 'ADJUSTMENT' AND adjustment_kind IN ('SHORTAGE', 'SURPLUS', 'OPENING'))
    ),
    CONSTRAINT chk_mov_counterparty_len CHECK (
        counterparty IS NULL OR char_length(counterparty) <= 120
    )
);
CREATE INDEX idx_mov_branch_created ON inventory_movements (branch_id, created_at DESC);
CREATE INDEX idx_mov_branch_good_created ON inventory_movements (branch_id, good_id, created_at DESC);
CREATE INDEX idx_mov_branch_type ON inventory_movements (branch_id, type);
CREATE INDEX idx_mov_inventory_created ON inventory_movements (inventory_id, created_at DESC);

-- +goose StatementBegin
CREATE FUNCTION inventory_movements_immutable() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'inventory_movements is append-only: % is not allowed', TG_OP;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER trg_inventory_movements_immutable
    BEFORE DELETE OR UPDATE ON inventory_movements
    FOR EACH ROW EXECUTE FUNCTION inventory_movements_immutable();

-- +goose StatementBegin
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'inventory_app') THEN
        EXECUTE 'GRANT SELECT ON goods TO inventory_app';
        EXECUTE 'GRANT SELECT, INSERT, UPDATE ON inventories TO inventory_app';
        EXECUTE 'GRANT SELECT, INSERT ON inventory_movements TO inventory_app';
        EXECUTE 'GRANT USAGE, SELECT ON SEQUENCE inventory_reference_seq TO inventory_app';
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER IF EXISTS trg_inventory_movements_immutable ON inventory_movements;
DROP FUNCTION IF EXISTS inventory_movements_immutable();
DROP TABLE inventory_movements;
DROP TABLE inventories;
DROP TABLE goods;
DROP SEQUENCE inventory_reference_seq;