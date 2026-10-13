-- Disposable integration fixture. Runs in a transaction and always rolls back.
DO $$ BEGIN
    IF NOT has_table_privilege('branch_app', 'branches', 'SELECT,INSERT,UPDATE,DELETE') THEN RAISE EXCEPTION 'branch role grants missing'; END IF;
    IF NOT has_table_privilege('auth_app', 'users', 'SELECT,INSERT,UPDATE,DELETE') THEN RAISE EXCEPTION 'auth role grants missing'; END IF;
    IF NOT has_table_privilege('auth_app', 'branches', 'SELECT') THEN RAISE EXCEPTION 'auth branch read grant missing'; END IF;
    IF NOT has_table_privilege('attendance_app', 'employees', 'SELECT,INSERT,UPDATE,DELETE') THEN RAISE EXCEPTION 'attendance role grants missing'; END IF;
    IF has_database_privilege('auth_app', current_database(), 'CREATE') THEN RAISE EXCEPTION 'auth runtime role unexpectedly has database CREATE'; END IF;
END $$;
BEGIN;
INSERT INTO branches (id,name,type,official_start_time,late_rate) VALUES
('aaaaaaaa-0000-4000-8000-000000000001','integration-test-branch','branch','09:00',10.00);
INSERT INTO users (id,name,email,password_hash,role) VALUES
('aaaaaaaa-0000-4000-8000-000000000002','Integration Supply','integration@example.test','not-a-real-hash','SUPPLY_MANAGER');
INSERT INTO refresh_tokens (id,user_id,token_hash,expires_at) VALUES
('aaaaaaaa-0000-4000-8000-000000000003','aaaaaaaa-0000-4000-8000-000000000002',repeat('a',64),NOW()+INTERVAL '1 hour');
INSERT INTO employees (id,name,branch_id,phone,national_id,payment_type,payment_amount,hire_date) VALUES
('aaaaaaaa-0000-4000-8000-000000000004','Integration Employee','aaaaaaaa-0000-4000-8000-000000000001','01000000001','00000000000002','daily',500,DATE '2026-01-01');
INSERT INTO attendance_records (id,employee_id,branch_id,work_date,check_in_at,status,source) VALUES
('aaaaaaaa-0000-4000-8000-000000000005','aaaaaaaa-0000-4000-8000-000000000004','aaaaaaaa-0000-4000-8000-000000000001',DATE '2026-01-02',TIMESTAMPTZ '2026-01-02 09:00:00+00','open','manual');
INSERT INTO deductions (id,attendance_id,employee_id,branch_id,work_date,late_minutes,completed_hours,rate_per_hour,original_amount) VALUES
('aaaaaaaa-0000-4000-8000-000000000006','aaaaaaaa-0000-4000-8000-000000000005','aaaaaaaa-0000-4000-8000-000000000004','aaaaaaaa-0000-4000-8000-000000000001',DATE '2026-01-02',65,1,10,10);
INSERT INTO overtime (id,employee_id,branch_id,work_date,hours,rate_per_hour,amount,created_by) VALUES
('aaaaaaaa-0000-4000-8000-000000000007','aaaaaaaa-0000-4000-8000-000000000004','aaaaaaaa-0000-4000-8000-000000000001',DATE '2026-01-02',2,25,50,'aaaaaaaa-0000-4000-8000-000000000002');
DO $$ BEGIN
    IF (SELECT COUNT(*) FROM branches WHERE id='aaaaaaaa-0000-4000-8000-000000000001') <> 1 THEN RAISE EXCEPTION 'branch query failed'; END IF;
    IF (SELECT COUNT(*) FROM overtime WHERE status='pending' AND employee_id='aaaaaaaa-0000-4000-8000-000000000004') <> 1 THEN RAISE EXCEPTION 'overtime query failed'; END IF;
END $$;
ROLLBACK;
