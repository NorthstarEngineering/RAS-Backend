SELECT format('CREATE ROLE auth_app LOGIN PASSWORD %L', :'auth_password')
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'auth_app')
\gexec
SELECT format('ALTER ROLE auth_app WITH LOGIN PASSWORD %L', :'auth_password')
\gexec
ALTER ROLE auth_app CONNECTION LIMIT 5;

SELECT format('CREATE ROLE branch_app LOGIN PASSWORD %L', :'branch_password')
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'branch_app')
\gexec
SELECT format('ALTER ROLE branch_app WITH LOGIN PASSWORD %L', :'branch_password')
\gexec
ALTER ROLE branch_app CONNECTION LIMIT 5;

SELECT format('CREATE ROLE attendance_app LOGIN PASSWORD %L', :'attendance_password')
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'attendance_app')
\gexec
SELECT format('ALTER ROLE attendance_app WITH LOGIN PASSWORD %L', :'attendance_password')
\gexec
ALTER ROLE attendance_app CONNECTION LIMIT 5;

-- Menu App Role
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'menu_app') THEN
        CREATE ROLE menu_app LOGIN PASSWORD 'menu_ci_password';
    END IF;
END
$$;
ALTER ROLE menu_app CONNECTION LIMIT 5;

-- Sales App Role
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'sales_app') THEN
        CREATE ROLE sales_app LOGIN PASSWORD 'sales_ci_password';
    END IF;
END
$$;
ALTER ROLE sales_app CONNECTION LIMIT 5;

-- Inventory App Role
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'inventory_app') THEN
        CREATE ROLE inventory_app LOGIN PASSWORD 'inventory_ci_password';
    END IF;
END
$$;
ALTER ROLE inventory_app CONNECTION LIMIT 5;

REVOKE CREATE ON SCHEMA public FROM PUBLIC;
GRANT USAGE ON SCHEMA public TO auth_app, branch_app, attendance_app, menu_app, sales_app, inventory_app;
GRANT CONNECT ON DATABASE restaurant_db TO auth_app, branch_app, attendance_app, menu_app, sales_app, inventory_app;