DO $$
BEGIN
    IF NOT has_database_privilege('auth_app', 'restaurant_db', 'CONNECT')
       OR NOT has_schema_privilege('auth_app', 'public', 'USAGE')
       OR has_schema_privilege('auth_app', 'public', 'CREATE')
       OR NOT has_table_privilege('auth_app', 'users', 'SELECT')
       OR NOT has_table_privilege('auth_app', 'users', 'INSERT')
       OR NOT has_table_privilege('auth_app', 'users', 'UPDATE')
       OR NOT has_table_privilege('auth_app', 'users', 'DELETE')
       OR NOT has_table_privilege('auth_app', 'branches', 'SELECT')
       OR has_table_privilege('auth_app', 'employees', 'SELECT') THEN
        RAISE EXCEPTION 'auth_app grants do not match the least-privilege contract';
    END IF;

    IF NOT has_table_privilege('branch_app', 'branches', 'SELECT')
       OR NOT has_table_privilege('branch_app', 'branches', 'INSERT')
       OR NOT has_table_privilege('branch_app', 'branches', 'UPDATE')
       OR NOT has_table_privilege('branch_app', 'branches', 'DELETE')
       OR has_table_privilege('branch_app', 'users', 'SELECT')
       OR has_table_privilege('branch_app', 'employees', 'SELECT') THEN
        RAISE EXCEPTION 'branch_app grants do not match the least-privilege contract';
    END IF;

    IF NOT has_table_privilege('attendance_app', 'employees', 'SELECT')
       OR NOT has_table_privilege('attendance_app', 'employees', 'INSERT')
       OR NOT has_table_privilege('attendance_app', 'employees', 'UPDATE')
       OR NOT has_table_privilege('attendance_app', 'employees', 'DELETE')
       OR NOT has_table_privilege('attendance_app', 'attendance_records', 'SELECT')
       OR NOT has_table_privilege('attendance_app', 'attendance_records', 'INSERT')
       OR NOT has_table_privilege('attendance_app', 'attendance_records', 'UPDATE')
       OR NOT has_table_privilege('attendance_app', 'attendance_records', 'DELETE')
       OR NOT has_table_privilege('attendance_app', 'branches', 'SELECT')
       OR has_table_privilege('attendance_app', 'users', 'SELECT')
       OR has_schema_privilege('attendance_app', 'public', 'CREATE') THEN
        RAISE EXCEPTION 'attendance_app grants do not match the least-privilege contract';
    END IF;

    IF (SELECT rolsuper OR rolcreatedb OR rolcreaterole OR rolreplication OR rolbypassrls
        FROM pg_roles WHERE rolname = 'auth_app')
       OR (SELECT rolconnlimit <> 5 FROM pg_roles WHERE rolname = 'auth_app') THEN
        RAISE EXCEPTION 'auth_app role attributes are not restricted';
    END IF;
END $$;
