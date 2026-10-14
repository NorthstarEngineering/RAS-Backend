# Restaurant Backend API Contract

Version: 1.1 (security correction)
Last updated: 2026-10-13

## Services

| Service | Port | Health | Base path |
|---|---:|---|---|
| Auth API | 9001 | `GET /health` | `/api/v1` |
| Branch API | 9002 | `GET /health` | `/api/v1` |
| Attendance API | 9003 | `GET /health` | `/api/v1` |

The repository contains no API gateway. Protected Branch and Attendance operations therefore verify the Auth API's HS256 bearer JWT directly. The identity headers `X-User-ID`, `X-User-Role`, and `X-Branch-ID` are not accepted as authentication. A gateway may be added later only if it verifies tokens and strips client-provided identity headers.

## JWT contract

- Algorithm: HS256 only.
- Issuer: `restaurant-backend`.
- Required: `sub` UUID, `role`, `iss`, `exp`.
- Roles: `CEO`, `BRANCH_MANAGER`, `SUPPLY_MANAGER`.
- Branch managers require `branch_id` UUID; other roles must omit it.
- `mcp=true` tokens cannot access Branch or Attendance until the password is changed.
- All three applications require the same `JWT_SECRET` (at least 32 characters). Keep it in an approved secret store in shared development.
- Send `Authorization: Bearer <access-token>` to protected endpoints.

The token is locally verified by Branch and Attendance using the shared secret; they do not call Auth on every request. Disabling users and refresh-token revocation do not revoke already-issued access tokens before their 15-minute expiry. This is an explicit residual limitation of stateless access tokens.

## Routes

### Auth API

| Method | Path | Access |
|---|---|---|
| POST | `/api/v1/auth/login` | Public; rate-limited |
| POST | `/api/v1/auth/refresh` | Public; rate-limited |
| POST | `/api/v1/auth/logout` | Public; refresh token in request body |
| GET | `/api/v1/auth/me` | Bearer token |
| POST | `/api/v1/auth/change-password` | Bearer token; allowed during required first password change |
| PATCH | `/api/v1/auth/me/email` | Bearer token |
| POST | `/api/v1/users` | CEO |
| GET | `/api/v1/users` | CEO |
| PATCH | `/api/v1/users/:id/status` | CEO |
| POST | `/api/v1/users/:id/reset-password` | CEO |
| POST | `/api/v1/users/:id/unlock` | CEO |

### Branch API

| Method | Path | Access |
|---|---|---|
| POST | `/api/v1/branches` | CEO |
| GET | `/api/v1/branches` | CEO, Branch Manager |
| GET | `/api/v1/branches/:id` | CEO, Branch Manager |
| PATCH | `/api/v1/branches/:id` | CEO |

### Attendance API

| Method | Path | Access |
|---|---|---|
| POST | `/api/v1/attendance/check-in` | CEO, Branch Manager; service checks BM token branch against employee branch |
| POST | `/api/v1/attendance/check-out` | CEO, Branch Manager; service checks BM token branch against employee branch |
| PATCH | `/api/v1/attendance/deductions/:id` | CEO, Branch Manager; BM branch scope |
| POST | `/api/v1/employees` | CEO, Branch Manager; BM branch inherited |
| GET | `/api/v1/employees` | CEO, Branch Manager; BM branch scope |
| GET | `/api/v1/employees/:id` | CEO, Branch Manager; BM branch scope |
| PATCH | `/api/v1/employees/:id` | CEO, Branch Manager; BM branch scope |
| PATCH | `/api/v1/employees/:id/payment` | CEO, Branch Manager; BM branch scope |
| POST | `/api/v1/employees/:id/terminate` | CEO, Branch Manager; BM branch scope |
| POST | `/api/v1/overtime` | CEO, Branch Manager; BM branch scope |
| GET | `/api/v1/overtime` | CEO, Branch Manager; BM branch scope |
| GET | `/api/v1/overtime/:id` | CEO, Branch Manager; BM branch scope |
| PATCH | `/api/v1/overtime/:id/approve` | CEO, Branch Manager; BM branch scope |
| PATCH | `/api/v1/overtime/:id/reject` | CEO, Branch Manager; BM branch scope |

Attendance Swagger UI is available at `/swagger/index.html`. The generated OpenAPI files describe request/response schemas; this route table records the server-side bearer authorization added in this release.

The Branch Manager check-in/check-out branch-scope guard and focused service tests are included. The user-provided WSL run reports the Attendance package passed; this session could not independently rerun the suite.

## Unimplemented product scope

Forgot-password/OTP recovery, fingerprint-device ingestion, menu and pricing, inventory/supply/transfers, sales/shift closing, discounts, costing/COGS, and reporting have no API routes in the current code. The broader product backlog does not imply these endpoints exist.
