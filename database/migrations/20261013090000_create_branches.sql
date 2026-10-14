-- +goose Up
CREATE TABLE branches (
    id UUID PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,
    type VARCHAR(20) NOT NULL CHECK (type IN ('branch', 'central_kitchen')),
    address TEXT,
    official_start_time TIME NOT NULL,
    late_rate NUMERIC(10,2) NOT NULL CHECK (late_rate >= 0),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_branches_active ON branches (is_active) WHERE is_active = TRUE;

DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'branch_app') THEN
        EXECUTE 'GRANT SELECT, INSERT, UPDATE, DELETE ON branches TO branch_app';
    END IF;
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'auth_app') THEN
        EXECUTE 'GRANT SELECT ON branches TO auth_app';
    END IF;
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'attendance_app') THEN
        EXECUTE 'GRANT SELECT ON branches TO attendance_app';
    END IF;
END $$;

-- +goose Down
DROP INDEX idx_branches_active;
DROP TABLE branches;
