-- +goose Up
CREATE TABLE users (
    id UUID PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    email VARCHAR(255) NOT NULL,
    password_hash TEXT NOT NULL,
    role VARCHAR(30) NOT NULL CHECK (role IN ('CEO', 'BRANCH_MANAGER', 'SUPPLY_MANAGER')),
    branch_id UUID NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    must_change_password BOOLEAN NOT NULL DEFAULT FALSE,
    failed_login_attempts INTEGER NOT NULL DEFAULT 0,
    lockout_count INTEGER NOT NULL DEFAULT 0,
    locked_until TIMESTAMPTZ NULL,
    last_login_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT users_branch_role_chk CHECK (
        (role = 'BRANCH_MANAGER' AND branch_id IS NOT NULL)
        OR (role <> 'BRANCH_MANAGER' AND branch_id IS NULL)
    )
);
CREATE UNIQUE INDEX users_email_lower_uq ON users (LOWER(email));
CREATE UNIQUE INDEX users_single_ceo_uq ON users (role) WHERE role = 'CEO';
CREATE INDEX users_branch_idx ON users (branch_id);

-- +goose StatementBegin
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'auth_app') THEN
        EXECUTE 'GRANT SELECT, INSERT, UPDATE, DELETE ON users TO auth_app';
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
DROP TABLE users;
