-- Seed data for development
-- Ref: Mock data matching cmd/api/main.go

BEGIN;

-- Branch 1: starts 09:00, rate = 50 EGP/hour
INSERT INTO branches (id, official_start_time, lateness_rate_per_hour, is_central_kitchen)
VALUES (
    '11111111-1111-1111-1111-111111111111',
    '09:00',
    50.00,
    FALSE
)
ON CONFLICT (id) DO NOTHING;

-- Employee 1: active, in branch 1
INSERT INTO employees (id, branch_id, active)
VALUES (
    '22222222-2222-2222-2222-222222222222',
    '11111111-1111-1111-1111-111111111111',
    TRUE
)
ON CONFLICT (id) DO NOTHING;

-- Employee 2: inactive
INSERT INTO employees (id, branch_id, active)
VALUES (
    '33333333-3333-3333-3333-333333333333',
    '11111111-1111-1111-1111-111111111111',
    FALSE
)
ON CONFLICT (id) DO NOTHING;

COMMIT;