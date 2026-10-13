# API reference

The stable route inventory, access requirements, JWT claims, and documented security correction are maintained in [Attendance API contract](../services/attendance-api/docs/API_CONTRACT.md). The current code-backed feature inventory is in [audit](audit.md).

- Auth API: port 9001, `GET /health`, routes under `/api/v1/auth` and `/api/v1/users`.
- Branch API: port 9002, `GET /health`, branch routes under `/api/v1/branches`.
- Attendance API: port 9003, `GET /health`, employees/attendance/overtime under `/api/v1`; Swagger at `/swagger/index.html`.

Branch and Attendance protected routes require `Authorization: Bearer <access-token>` and verify Auth's HS256 JWT. Caller-supplied identity headers are ignored. The generated Attendance Swagger JSON/YAML document the bearer scheme and request/response shapes. Auth and Branch have no OpenAPI files yet.

Public API contracts have been preserved except that Branch/Attendance no longer accept unsigned identity headers. This is a security correction because no trusted gateway exists in the repository. It is documented in the service API contract and security decision.
