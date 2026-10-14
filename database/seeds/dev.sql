BEGIN;
DO $$ BEGIN
    IF current_database() <> 'restaurant_db' THEN RAISE EXCEPTION 'development seeds may only target restaurant_db'; END IF;
    IF current_setting('app.environment', true) IS DISTINCT FROM 'development' THEN RAISE EXCEPTION 'development seeds require app.environment=development'; END IF;
END $$;

INSERT INTO branches (id, name, type, address, official_start_time, late_rate, is_active)
VALUES ('11111111-1111-1111-1111-111111111111', 'Development Branch', 'branch', 'Local development only', '09:00', 50.00, TRUE)
ON CONFLICT (id) DO NOTHING;

INSERT INTO employees (id, name, branch_id, phone, national_id, payment_type, payment_amount, hire_date, active)
VALUES ('22222222-2222-2222-2222-222222222222', 'Development Employee', '11111111-1111-1111-1111-111111111111', '01000000000', '00000000000001', 'daily', 500.00, DATE '2026-01-01', TRUE)
ON CONFLICT (id) DO NOTHING;
COMMIT;
