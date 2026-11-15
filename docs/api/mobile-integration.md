# Flutter mobile API integration

This guide and the OpenAPI contract are the mobile team handoff. The development API Gateway is deployed and was smoke-tested on 2026-10-14. Endpoint schemas are in [openapi.yaml](openapi.yaml); deterministic sample payloads are in [mocks/](mocks/).

## Base URLs

The services currently listen locally on Auth `9001`, Branch `9002`, and Attendance `9003`. Their paths include `/api/v1`. For example:

- Auth: `http://localhost:9001/api/v1`
- Branch: `http://localhost:9002/api/v1`
- Attendance: `http://localhost:9003/api/v1`

For mobile integration with the deployed development environment, use this API Gateway base URL:

`https://jo3c9na8mg.execute-api.eu-west-3.amazonaws.com/dev`

Append the existing API path, for example `/api/v1/auth/login` or `/api/v1/branches`. Keep `/dev` in the base URL. Do not add another stage segment. This is the current development endpoint, not a production URL; it can become unavailable when the temporary AWS environment is stopped or destroyed. Development and production are not deployed.

For local service testing, use the per-service URLs above. On a physical phone, `localhost` means the phone; use the host machine's reachable LAN address and ensure the development firewall permits the connection. Android emulator host addressing differs by emulator; configure it in the local-only environment rather than hardcoding it in app code.

The active Gateway base URL is also listed in the supplied development Postman environment. Its availability depends on the temporary development stack remaining deployed.

## Login and token handling

Call `POST /api/v1/auth/login` without a bearer token, with JSON `{ "email": "…", "password": "…" }`. A successful response is `200` with `access_token`, `token_type` (`Bearer`), `expires_at` (UTC timestamp), `refresh_token`, and `user`. Access JWTs are issued by Auth using HS256 and include issuer `restaurant-backend`, subject, role, expiry, and optional branch / first-login claims. The current token has no audience claim. Source defaults are a 15-minute access-token TTL and a 30-day refresh-token TTL; both can be configured server-side. The returned `expires_at` is authoritative for a session.

Store both tokens using platform secure storage. Never include a JWT signing secret, database credential, AWS credential, or server secret in the mobile app. Attach `Authorization: Bearer <access_token>` to protected API calls. Do not send identity headers such as `X-User-ID`, `X-User-Role`, or `X-Branch-ID` as authorization evidence.

When the access token expires, call `POST /api/v1/auth/refresh` with `{ "refresh_token": "…" }`. It returns the same session shape; persist the newly returned access and refresh tokens together because refresh tokens rotate. Serialize refresh requests (single-flight) so concurrent 401 responses do not reuse the same refresh token. If refresh fails, clear local session state and require login. `POST /api/v1/auth/logout` takes the current refresh token in the same JSON shape and returns `204`; it has no access-token middleware in the current route registration. Clear local tokens after logout.

## Authorization and branch context

Current role names include `CEO`, `BRANCH_MANAGER`, and model-defined roles. Branch API allows CEO and Branch Manager reads and CEO writes. Attendance routes globally allow CEO and Branch Manager. Business services scope employee and overtime operations by branch. The employee-create request includes `branch_id`; server-side code overrides it with the authenticated Branch Manager's branch. CEO can select a branch.

Client-side role checks may hide unavailable UI, but are not security controls. The backend must authorize every operation. Check-in and check-out compare a Branch Manager's token branch with the employee branch. The development Gateway authorizer and proxy route were deployed and smoke-tested; application-level authorization remains required.

## Errors, timeouts, and retries

Application errors usually use `{ "error": { "code": "…", "message": "…" } }`. Auth, Branch, Attendance may use different code casing/naming. A gateway authorizer can return a gateway-generated error shape that differs from application errors. Parse defensively and do not display internal error text as trusted detail.

Set finite connect and response timeouts appropriate to mobile networks. Retry safe reads only for transient network failures and selected 5xx responses, with bounded exponential backoff and jitter. Do not automatically retry mutations such as check-in, check-out, employee creation, or overtime creation unless an idempotency contract is added; the current API does not document idempotency keys. Handle `400` validation, `401` session renewal, `403` role/scope refusal, `404` missing resources, `409` state conflicts, `429` throttling/lockout (honor `Retry-After` if supplied), and `5xx` service failures.

## Postman and mocks

Import `postman-collection.json` and `environments.example.json`; import the `RAS Backend - development` environment and select it to use the deployed Gateway. For local runs, duplicate it and set each service base URL to the corresponding local service URL above. Obtain a development login from the project owner through an approved secure channel; credentials are intentionally not included here. The examples use placeholders, not credentials. Deterministic mock JSON files under `mocks/` let the UI exercise success and error states before the backend is reachable. They are fixtures, not live API responses or user accounts.

Validate the contract from the repository root with `npx --yes @redocly/cli lint --config docs/api/redocly.yaml docs/api/openapi.yaml`. The repository config suppresses style warnings for intentionally local/development servers and the absence of a project license; it does not suppress structural validation errors.

## Missing endpoints and contract limitations

There is no attendance-history/list endpoint, employee branch-transfer endpoint, fingerprint-device ingestion endpoint, payroll/history API, or password recovery/OTP flow in the verified route table. Do not implement mobile calls to these as if they exist. The current Swagger artifact is for Attendance only and is Swagger 2.0; Auth and Branch are not represented there. No API contract version beyond the existing route prefix has been changed by this handoff.
