SELECT format('CREATE ROLE auth_app LOGIN PASSWORD %L', :'auth_password')
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'auth_app')
\gexec
SELECT format('ALTER ROLE auth_app WITH LOGIN PASSWORD %L', :'auth_password')
\gexec
ALTER ROLE auth_app NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS CONNECTION LIMIT 5;

SELECT format('CREATE ROLE branch_app LOGIN PASSWORD %L', :'branch_password')
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'branch_app')
\gexec
SELECT format('ALTER ROLE branch_app WITH LOGIN PASSWORD %L', :'branch_password')
\gexec
ALTER ROLE branch_app NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS CONNECTION LIMIT 5;

SELECT format('CREATE ROLE attendance_app LOGIN PASSWORD %L', :'attendance_password')
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'attendance_app')
\gexec
SELECT format('ALTER ROLE attendance_app WITH LOGIN PASSWORD %L', :'attendance_password')
\gexec
ALTER ROLE attendance_app NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS CONNECTION LIMIT 5;

REVOKE CREATE ON SCHEMA public FROM PUBLIC;
GRANT USAGE ON SCHEMA public TO auth_app, branch_app, attendance_app;
GRANT CONNECT ON DATABASE restaurant_db TO auth_app, branch_app, attendance_app;
