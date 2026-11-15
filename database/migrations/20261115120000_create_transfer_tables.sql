-- +goose Up
CREATE SEQUENCE transfer_code_seq START 1000;
CREATE TABLE transfers (
    id UUID PRIMARY KEY,
    code TEXT NOT NULL DEFAULT ('TRF-' || lpad(nextval('transfer_code_seq')::text, 6, '0')),
    source_branch_id UUID NOT NULL REFERENCES branches (id),
    dest_branch_id UUID NOT NULL REFERENCES branches (id),
    good_id UUID NOT NULL REFERENCES goods (id),
    requested_qty NUMERIC(12,2) NOT NULL,
    approved_qty NUMERIC(12,2),
    sent_qty NUMERIC(12,2),
    received_qty NUMERIC(12,2),
    shortage_qty NUMERIC(12,2) NOT NULL DEFAULT 0,
    unit_cost NUMERIC(12,2),
    priority TEXT NOT NULL DEFAULT 'NORMAL',
    reason TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'PENDING',
    decision_note TEXT,
    vehicle_type TEXT,
    driver_name TEXT,
    driver_phone TEXT,
    expected_departure TIMESTAMPTZ,
    temperature_c NUMERIC(5,2),
    receive_note TEXT,
    requested_by UUID NOT NULL,
    requested_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    decided_by UUID,
    decided_at TIMESTAMPTZ,
    sent_by UUID,
    sent_at TIMESTAMPTZ,
    received_by UUID,
    received_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_transfers_code UNIQUE (code),
    CONSTRAINT chk_transfers_branches CHECK (source_branch_id <> dest_branch_id),
    CONSTRAINT chk_transfers_status CHECK (status IN ('PENDING', 'APPROVED', 'REJECTED', 'SENT', 'RECEIVED')),
    CONSTRAINT chk_transfers_priority CHECK (priority IN ('NORMAL', 'URGENT')),
    CONSTRAINT chk_transfers_qty CHECK (
        requested_qty > 0 AND shortage_qty >= 0 AND
        (approved_qty IS NULL OR approved_qty > 0) AND
        (sent_qty IS NULL OR sent_qty > 0) AND
        (received_qty IS NULL OR received_qty >= 0)
    ),
    CONSTRAINT chk_transfers_unit_cost CHECK (unit_cost IS NULL OR unit_cost >= 0)
);
CREATE INDEX idx_transfers_source ON transfers (source_branch_id, requested_at DESC);
CREATE INDEX idx_transfers_dest ON transfers (dest_branch_id, requested_at DESC);
CREATE INDEX idx_transfers_status ON transfers (status);
CREATE TABLE transfer_events (
    id UUID PRIMARY KEY,
    transfer_id UUID NOT NULL REFERENCES transfers (id),
    action TEXT NOT NULL,
    actor_id UUID NOT NULL,
    note TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_transfer_events_action CHECK (action IN ('REQUESTED', 'APPROVED', 'REJECTED', 'SENT', 'RECEIVED'))
);
CREATE INDEX idx_transfer_events_transfer ON transfer_events (transfer_id, created_at);
-- +goose StatementBegin
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'transfer_app') THEN
        EXECUTE 'GRANT SELECT ON branches, goods TO transfer_app';
        EXECUTE 'GRANT SELECT, INSERT, UPDATE ON transfers TO transfer_app';
        EXECUTE 'GRANT SELECT, INSERT ON transfer_events TO transfer_app';
        EXECUTE 'GRANT SELECT, INSERT, UPDATE ON inventories TO transfer_app';
        EXECUTE 'GRANT SELECT, INSERT ON inventory_movements TO transfer_app';
        EXECUTE 'GRANT USAGE, SELECT ON SEQUENCE transfer_code_seq TO transfer_app';
    END IF;
END $$;
-- +goose StatementEnd
-- +goose Down
DROP TABLE transfer_events;
DROP TABLE transfers;
DROP SEQUENCE transfer_code_seq;