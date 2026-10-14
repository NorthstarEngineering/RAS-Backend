-- Module: auth | users table (owned by auth)
-- NOTE: the FK users.branch_id -> branches(id) is added in auth_003_branch_fk.sql,
-- after the branch module has created the branches table.

CREATE TABLE users (
    id UUID PRIMARY KEY,

    name  VARCHAR(100) NOT NULL,
    email VARCHAR(255) NOT NULL,
    password_hash TEXT NOT NULL,

    role VARCHAR(30) NOT NULL
        CHECK (role IN ('CEO', 'BRANCH_MANAGER', 'SUPPLY_MANAGER')),

    branch_id UUID NULL,

    is_active            BOOLEAN NOT NULL DEFAULT TRUE,
    must_change_password BOOLEAN NOT NULL DEFAULT FALSE,

    -- Brute-force protection
    failed_login_attempts INTEGER NOT NULL DEFAULT 0,
    lockout_count         INTEGER NOT NULL DEFAULT 0,
    locked_until          TIMESTAMPTZ NULL,
    last_login_at         TIMESTAMPTZ NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- A Branch Manager always belongs to a branch; other roles never do.
    CONSTRAINT users_branch_role_chk CHECK (
        (role = 'BRANCH_MANAGER' AND branch_id IS NOT NULL)
        OR (role <> 'BRANCH_MANAGER' AND branch_id IS NULL)
    )
);

-- Case-insensitive unique email.
CREATE UNIQUE INDEX users_email_lower_uq ON users (LOWER(email));

-- Only one CEO in the system.
CREATE UNIQUE INDEX users_single_ceo_uq ON users (role) WHERE role = 'CEO';

CREATE INDEX users_branch_idx ON users (branch_id);
