# Security notes

## Authentication and authorization

Auth uses bcrypt password hashing, HS256 access JWTs, 15-minute access expiry, hashed refresh tokens with rotation/revocation, account lockout, and login/refresh IP rate limiting. Auth validates issuer, algorithm, signature, expiration, subject UUID, and role. Protected operations enforce roles in middleware and business services.

Branch and Attendance now verify signed Bearer JWTs locally. They do not trust `X-User-ID`, `X-User-Role`, or `X-Branch-ID`. Branch route writes require CEO; Branch Managers can read only their assigned branch. Attendance accepts CEO/Branch Manager; employee and overtime operations scope Branch Managers to the signed branch claim and employee creation inherits that branch. Check-in and check-out now also compare the authenticated manager branch with the employee branch before attendance repository access. Focused tests were added for this guard; the user-provided WSL test output reports the Attendance package passed. I could not independently rerun it from this session. A `mcp=true` token is denied outside Auth password-change routes.

## Important remaining limitations

- HS256 requires the same secret in all three APIs. Any compromised service that knows it can forge tokens. A future migration to asymmetric signing should be separately designed and rolled out compatibly.
- Access tokens are stateless and remain valid until expiry after account disablement or refresh-token revocation (maximum 15 minutes under current defaults).
- Password reset via OTP, account recovery, MFA, and audit-log storage are not implemented.
- The API Gateway and Lambda authorizer are Terraform-defined but not deployed. Gateway must not trust caller identity headers.
- No CORS policy is configured for the native mobile app by default; add exact origins if browser clients are introduced.

## AWS edge authentication (Terraform-defined, not deployed)

Auth currently issues HS256 access JWTs with issuer `restaurant-backend`, `sub`, `iat`, `exp`, `jti`, `role`, optional `branch_id`, and optional `mcp`. It has no `aud`, so the native HTTP API JWT authorizer cannot validate this token as-is. The development Terraform uses a Lambda REQUEST authorizer without adding a second issuer or user database. The Lambda reads only a dedicated JWT-only Secrets Manager secret, validates the current claims, and returns a boolean plus minimal context. It preserves first-login `mcp` tokens so existing Auth self/profile and password-change routes work; Auth handlers limit their use, while Branch and Attendance reject them. Authorizer-result caching is disabled; a warm Lambda process caches the signing secret for at most 60 seconds. Backend Branch and Attendance continue signature verification and authorization. Gateway identity is not forwarded as trusted headers.

Logout revokes refresh-token records but does not revoke an already-issued stateless access JWT; a logged-out access token remains usable until its 15-minute expiry. If immediate access-token revocation becomes required, implement and review an explicit revocation/session-version design or migrate token claims/signing contract with compatibility handling. Gateway custom-authorizer denials may use a gateway-generated 403 response; direct application middleware returns its documented 401 for invalid/expired bearer tokens. Flutter should handle both at the gateway boundary.

## Configuration and logging

- `JWT_SECRET` must be at least 32 characters and shared across APIs.
- Do not commit `.env`; examples contain placeholders only.
- Development CEO bootstrap only runs with `APP_ENV=development`; it requires explicit email/password when a CEO does not already exist.
- API logs are JSON and include service, request ID, route, method, status, and duration. Passwords, token values, secret values, and database DSNs are not logged.
- Internal Attendance errors return generic 500 messages; unexpected database details are not returned to clients.

## PostgreSQL and AWS

Runtime database users are separate (`auth_app`, `branch_app`, `attendance_app`) and receive only required DML/read grants. Goose runs with a controlled migration account; APIs never run schema DDL. AWS EC2 uses SSM rather than inbound SSH; RDS is private and accepts traffic only from the application security group. The app host can read only the specified Secrets Manager secrets and ECR images.

Local PostgreSQL passwords are development-only. Shared AWS secrets must be pre-created in Secrets Manager and accessed through least-privilege IAM. CI should use GitHub OIDC, protected environments, and an AWS trust policy restricted to this repository and `main`.
