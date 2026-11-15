\getenv auth_password AUTH_DB_PASSWORD
\getenv branch_password BRANCH_DB_PASSWORD
\getenv attendance_password ATTENDANCE_DB_PASSWORD
\getenv menu_password MENU_DB_PASSWORD
\getenv inventory_password INVENTORY_DB_PASSWORD
\getenv central_kitchen_password CENTRAL_KITCHEN_DB_PASSWORD
\getenv cost_password COST_DB_PASSWORD
\getenv sales_password SALES_DB_PASSWORD
\getenv transfer_password TRANSFER_DB_PASSWORD

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

-- أضفنا الـ Menu App هنا
SELECT format('CREATE ROLE menu_app LOGIN PASSWORD %L', :'menu_password')
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'menu_app')
\gexec
SELECT format('ALTER ROLE menu_app WITH LOGIN PASSWORD %L', :'menu_password')
\gexec
ALTER ROLE menu_app NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS CONNECTION LIMIT 5;

SELECT format('CREATE ROLE inventory_app LOGIN PASSWORD %L', :'inventory_password') WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'inventory_app')
\gexec
SELECT format('ALTER ROLE inventory_app WITH LOGIN PASSWORD %L', :'inventory_password')
\gexec
ALTER ROLE inventory_app NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS CONNECTION LIMIT 5;
SELECT format('CREATE ROLE central_kitchen_app LOGIN PASSWORD %L', :'central_kitchen_password') WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'central_kitchen_app')
\gexec
SELECT format('ALTER ROLE central_kitchen_app WITH LOGIN PASSWORD %L', :'central_kitchen_password')
\gexec
ALTER ROLE central_kitchen_app NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS CONNECTION LIMIT 5;
SELECT format('CREATE ROLE cost_app LOGIN PASSWORD %L', :'cost_password') WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'cost_app')
\gexec
SELECT format('ALTER ROLE cost_app WITH LOGIN PASSWORD %L', :'cost_password')
\gexec
ALTER ROLE cost_app NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS CONNECTION LIMIT 5;
SELECT format('CREATE ROLE sales_app LOGIN PASSWORD %L', :'sales_password') WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'sales_app')
\gexec
SELECT format('ALTER ROLE sales_app WITH LOGIN PASSWORD %L', :'sales_password')
\gexec
ALTER ROLE sales_app NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS CONNECTION LIMIT 5;
SELECT format('CREATE ROLE transfer_app LOGIN PASSWORD %L', :'transfer_password') WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'transfer_app')
\gexec
SELECT format('ALTER ROLE transfer_app WITH LOGIN PASSWORD %L', :'transfer_password')
\gexec
ALTER ROLE transfer_app NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS CONNECTION LIMIT 5;

REVOKE CREATE ON SCHEMA public FROM PUBLIC;
GRANT USAGE ON SCHEMA public TO auth_app, branch_app, attendance_app, menu_app, inventory_app, central_kitchen_app, cost_app, sales_app, transfer_app;
GRANT CONNECT ON DATABASE restaurant_db TO auth_app, branch_app, attendance_app, menu_app, inventory_app, central_kitchen_app, cost_app, sales_app, transfer_app;

DO $$
BEGIN
  IF (SELECT COUNT(*) FROM pg_roles WHERE rolname IN ('auth_app', 'branch_app', 'attendance_app', 'menu_app', 'inventory_app', 'central_kitchen_app', 'cost_app', 'sales_app', 'transfer_app')) <> 9 THEN
    RAISE EXCEPTION 'Runtime role bootstrap did not create all 9 expected roles';
  END IF;
END
$$;

