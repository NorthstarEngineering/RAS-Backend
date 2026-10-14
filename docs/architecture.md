# Architecture

## Applications and capabilities

The repository contains three independently built Go modules. Auth owns sign-in, signed access tokens, refresh sessions, password changes, and CEO account administration. Branch owns branch records and branch-specific operating settings. Attendance owns employees, check-in/out, deductions, and overtime workflows. The source audit and public routes are in [audit.md](audit.md) and [api.md](api.md).

Each service has its own `cmd/api`, `internal` packages, module files, tests, and container image. Handlers translate HTTP input/output; service packages implement business rules; repositories perform SQL access. No service imports another service's `internal` package.

## Communication and coupling

Clients authenticate with an Auth-issued signed JWT. Branch and Attendance validate the signature locally using the shared secret and do not call Auth for each request. Request IDs are accepted or generated for correlation. Service URLs are environment configuration for future cross-service HTTP calls; the current source has no established internal HTTP client workflow. Auth currently reads branch records to validate branch assignment, and Attendance reads branch records directly. These are explicit shared-database dependencies, not isolated microservices.

## Database ownership

| Table | Owner | Current readers/writers |
|---|---|---|
| `branches` | Branch | Branch writes; Auth and Attendance read |
| `users`, `refresh_tokens` | Auth | Auth only |
| `employees`, `attendance_records`, `deductions`, `overtime` | Attendance | Attendance only |

The central Goose history is the only authoritative schema history. Changes to shared foreign-key targets must be coordinated across the affected APIs. See [database.md](database.md).

## Runtime

Docker Compose runs PostgreSQL and all three APIs locally. AWS development uses one SSM-managed EC2 host for the containers, ECR for images, and one private RDS PostgreSQL database. There is no Kubernetes, production environment, high-availability claim, or infrastructure deployment performed by this task.

## Known limitations

The audit is source-based; legacy databases have not been inspected or baselined. AWS resources are definitions only. Auth and the other APIs must share identical JWT signing configuration. Cross-service business operations currently use the shared database, and database permissions enforce read/write ownership at table level rather than separate schemas.
