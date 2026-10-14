# RAS-backend audit

Audit date: 2026-10-13
Workspace: `C:\Users\myfin\OneDrive\Desktop\RAS-backend`

This audit is based only on the Go source, SQL, API contract, Swagger artifacts, module files, tests, and Git metadata inside this workspace. It records implemented behavior, not all desired product scope.

## Applications and feature inventory

| Feature | Status | Evidence / gap |
|---|---|---|
| Authentication: email/password login, bcrypt, JWT access tokens, refresh-token hashing, rotation and revocation | Implemented | Auth service and security packages; refresh token hashes are persisted. |
| Login brute-force lockout and login/refresh IP rate limiting | Implemented | Auth user repository/service and limiter middleware. |
| Logout, current user, change password, change email | Implemented | Auth routes and service. |
| CEO administration: create/list users, activate/deactivate, reset password, unlock | Implemented | Auth routes restricted to CEO; service enforces CEO. |
| CEO bootstrap seed | Partially implemented | Seed service is idempotent and requires environment credentials if no CEO exists; startup invokes it against the database. It is not yet separated into dev-only initialization and logs can expose raw wrapped database errors at startup. |
| Forgot password, OTP, public registration, general user update/delete | Not implemented | No routes or flows found. |
| Branch create/list/get/update and branch field validation | Implemented | Branch service validates name, type, time, and late rate. |
| Central Kitchen | Partially implemented | Branch type accepts `central_kitchen`; no separate setup/workflow or inventory exists in this API. |
| Branch authorization | Not implemented securely | Gateway middleware trusts client-provided identity headers; route handlers do not enforce CEO-only writes. |
| Employee create/list/get/update, payment change, termination | Partially implemented | Service applies CEO/BM branch scope, but middleware identity is unverified and role values do not match the Auth JWT contract. Payment history is not modeled. |
| Attendance check-in/check-out and automatic completed-hour lateness deductions | Partially implemented | Service and repository implement persistence/calculation; no tests found and identity is accepted through unsigned headers. |
| Deduction adjustment/cancellation | Partially implemented | Handler/service support adjustment and cancellation reasons/audit fields; no independent audit log or tests found. |
| Overtime create/list/get/approve/reject | Partially implemented | Service enforces pending/approved/rejected rules and BM scope using `BM`; identity header is untrusted and API/database tests are missing. |
| Fingerprint integration | Not implemented | Only a source label/suggestion path exists; no device adapter or ingestion route. |
| Swagger/OpenAPI for Attendance | Implemented, needs contract verification | `docs/swagger.yaml`, `swagger.json`, generated `docs.go`; API contract describes gateway behavior that is absent. |
| Menu, pricing, inventory, supply, transfer, sales/shift closing, discounts, COGS, reports | Not implemented | No corresponding backend packages, routes, repositories, or tables found. |

## HTTP routes

All three applications expose `GET /health`; default ports are Auth 9001, Branch 9002, Attendance 9003. Auth and Branch have no Swagger routes. Attendance exposes Swagger UI at `/swagger/*any`.

### Auth API (`:9001`)

| Method | Path | Current access |
|---|---|---|
| POST | `/api/v1/auth/login` | Public; rate-limited |
| POST | `/api/v1/auth/refresh` | Public; rate-limited |
| POST | `/api/v1/auth/logout` | Public, refresh token required in body |
| GET | `/api/v1/auth/me` | Bearer JWT; allows first-login password-change state |
| POST | `/api/v1/auth/change-password` | Bearer JWT; allows first-login password-change state |
| PATCH | `/api/v1/auth/me/email` | Bearer JWT |
| POST | `/api/v1/users` | CEO JWT |
| GET | `/api/v1/users` | CEO JWT |
| PATCH | `/api/v1/users/:id/status` | CEO JWT |
| POST | `/api/v1/users/:id/reset-password` | CEO JWT |
| POST | `/api/v1/users/:id/unlock` | CEO JWT |

### Branch API (`:9002`)

| Method | Path | Intended access in API contract | Implemented boundary |
|---|---|---|---|
| POST | `/api/v1/branches` | CEO | No role enforcement in route/handler; only untrusted headers required |
| GET | `/api/v1/branches` | CEO, BM | No JWT verification or role allowlist |
| GET | `/api/v1/branches/:id` | CEO, BM | No JWT verification or role allowlist |
| PATCH | `/api/v1/branches/:id` | CEO | No role enforcement in route/handler |

### Attendance API (`:9003`)

| Method | Path | Intended access in API contract | Implemented boundary |
|---|---|---|---|
| POST | `/api/v1/attendance/check-in` | CEO, BM | Reads caller-controlled identity headers |
| POST | `/api/v1/attendance/check-out` | CEO, BM | Reads caller-controlled identity headers |
| PATCH | `/api/v1/attendance/deductions/:id` | CEO, BM | Reads caller-controlled identity headers |
| POST | `/api/v1/employees` | CEO, BM | Reads caller-controlled identity headers |
| GET | `/api/v1/employees` | CEO, BM | Reads caller-controlled identity headers |
| GET | `/api/v1/employees/:id` | CEO, BM | Reads caller-controlled identity headers |
| PATCH | `/api/v1/employees/:id` | CEO, BM | Reads caller-controlled identity headers |
| PATCH | `/api/v1/employees/:id/payment` | CEO, BM | Reads caller-controlled identity headers |
| POST | `/api/v1/employees/:id/terminate` | CEO, BM | Reads caller-controlled identity headers |
| POST | `/api/v1/overtime` | CEO, BM | Reads caller-controlled identity headers |
| GET | `/api/v1/overtime` | CEO, BM | Reads caller-controlled identity headers |
| GET | `/api/v1/overtime/:id` | CEO, BM | Reads caller-controlled identity headers |
| PATCH | `/api/v1/overtime/:id/approve` | CEO, BM | Reads caller-controlled identity headers |
| PATCH | `/api/v1/overtime/:id/reject` | CEO, BM | Reads caller-controlled identity headers |

## Database schema and ownership

The complete Attendance `001_init.sql` defines the most complete schema. Auth and Branch own the following tables; Attendance owns its employee and operational tables. Every application currently connects directly to the same configured PostgreSQL database.

| Table | Owner | Important relationships |
|---|---|---|
| `branches` | Branch API | Primary table; referenced by users, employees, attendance, deductions, overtime |
| `users` | Auth API | Optional `branch_id` references `branches`; role check requires branch only for `BRANCH_MANAGER`; one CEO partial unique index |
| `refresh_tokens` | Auth API | `user_id` references users with cascade delete; stores SHA-256 token hash |
| `employees` | Attendance API | `branch_id` references branches; unique national ID; payment, hire/termination, active fields |
| `attendance_records` | Attendance API | References employees and branches; one record per employee/work date; check-in/out consistency |
| `deductions` | Attendance API | References attendance, employees, branches; adjustment/cancellation fields and consistency checks |
| `overtime` | Attendance API | References employees and branches; pending/approved/rejected state and approval/rejection consistency |

`auth-api/cmd/api/adapters.go` directly queries `branches` to validate a user assignment. Attendance directly queries branches through its adapter and uses the employee service/repository internally. All services share the database, so table grants and migration changes remain deployment-coupled.

## Migration audit

- `branch-api/migrations/branch_001.sql` creates the authoritative-looking `branches` table with `type VARCHAR(20)`, `UNIQUE(name)`, `TIME`, `late_rate NUMERIC(10,2)`, and active timestamps.
- `attendance-api/migrations/001_init.sql` also creates `branches`, this time `type VARCHAR(30)` and a separate unique index, then creates the full employees, attendance, deductions, and overtime schema. Branch service only accepts `branch` and `central_kitchen`; both varchar widths fit the values, so the narrow 20-character form is sufficient. Keep the Branch-owned definition and align canonical schema on it.
- `attendance-api/migrations/001_attendance.sql` creates conflicting incomplete stub `branches` and `employees`, followed by duplicate attendance/deduction tables. It has a repeated `chk_cancel_complete` constraint in the visible source and is not safe as a second executable migration.
- `attendance-api/migrations/004_overtime.sql` repeats the overtime table and indexes already in `001_init.sql`.
- `auth-api/migrations/auth_001.sql` creates users before branch FK; `auth_002.sql` creates refresh_tokens; `auth_003_branch_fk.sql` depends on branches. This ordering must be represented centrally.
- `attendance-api/migrations/002_seed_dev.sql` inserts columns that do not match the full canonical branch and employee tables; it cannot be reused unchanged.
- Existing migration files are plain SQL and no applied schema/version history is available in this workspace. Do not infer database state or claim an existing database can be upgraded safely until inspected and baselined.

## Security, configuration, and operations

- Auth validates a minimum 32-character JWT secret, uses HS256 and issuer validation, expiration, bcrypt, hashed rotating refresh tokens, lockout, and rate limiting. The Auth module returns sanitized API errors.
- Branch and Attendance trust caller-provided `X-User-ID`, `X-User-Role`, and `X-Branch-ID`. No gateway exists in this repository, despite contract documentation describing one. A forged header can bypass role and branch-scope assumptions.
- Auth role claims use `BRANCH_MANAGER`, while Attendance domain services compare `BM`; that causes valid BM JWT-derived roles to fail or behave inconsistently.
- Attendance has a hardcoded fallback DSN containing a weak credential, uses `sqlx.Connect` without explicit pool/timeouts, and all three apps use `router.Run` without configured HTTP timeouts or graceful shutdown.
- Auth's `.env.example` uses `restaurant_go_db` while the agreed shared DB is `restaurant_db`; per-app credentials default to broad postgres examples. There is no root configuration contract.
- Gin's default request logger is not structured and does not propagate request IDs. Health responses include current time; readiness is not separated.
- No runtime DDL was found. Auth CEO seeding is idempotent and fails when no CEO exists and no seed credential is configured, but currently runs at every Auth startup after DB connection.

## Existing verification and gaps

- Existing test: `auth-api/internal/auth/service/service_test.go` only.
- No Branch or Attendance unit/API/database/integration tests found.
- No root Compose, Dockerfiles, Terraform, GitHub Actions, or root docs found.
- Go toolchain was not available on PATH at audit time; baseline tests/vet could not run. Recheck after locating a toolchain.
- Swagger artifacts exist only for Attendance; verify route/security/status/schema parity after fixes.

## Changes applied after the baseline audit

The applications have been consolidated under `services/` while preserving individual Go modules and entry points. Original repository histories are retained locally as verified bundles and Git directories under the ignored `.git-preservation/` directory. The monorepo is connected to the existing GitHub repository and is organized into feature branches for pull request review.

The active Goose history now has five timestamped migrations. Original migrations are preserved under `database/legacy-migrations/` and are not active. Ownership is branches (Branch), users/refresh_tokens (Auth), and employees/attendance_records/deductions/overtime (Attendance). Fresh-database migration execution and adoption against any existing database remain unverified because PostgreSQL was unavailable.

Branch and Attendance now validate signed Bearer JWTs rather than identity headers, enforce service roles, and scope branch-manager reads/writes. This intentionally closes an unauthenticated identity spoofing defect; clients that relied on unsigned `X-User-*` headers must send Auth-issued Bearer tokens. HTTP timeouts, graceful shutdown, request IDs, structured logs, environment-driven database connection settings, Compose, Dockerfiles, Terraform definitions, independent CI workflows, and documentation have been added. These changes have not been compiled or runtime-tested in this environment.

## Baseline prioritized implementation plan

1. Preserve each clean nested repository history and record refs/remotes; establish the root Git repository safely.
2. Relocate modules to `services/` while preserving independent `go.mod`, entry points, tests, and Swagger.
3. Establish one Goose migration history with Branch first, Auth tables and branch FK next, then Attendance tables; move dev seeds out of migration history.
4. Replace unsigned identity headers with strict HS256 bearer-token verification in Branch and Attendance, canonicalize roles, enforce route authorization and branch scope.
5. Remove the Attendance fallback DSN, align `restaurant_db`, configure database pools and HTTP timeouts/shutdown, and add request-ID structured logging.
6. Make CEO bootstrap safe and environment-driven; test auth and authorization boundaries.
7. Add Dockerfiles, explicit one-shot migration Compose service, local dev configuration, Make targets, and docs.
8. Add AWS development Terraform and separate CI workflows; do not apply/destroy AWS or publish images without approval.
9. Run formatting, vet, unit/API tests, migration/database checks, Compose builds, Terraform validation, and secret/diff review; report environmental blockers precisely.
