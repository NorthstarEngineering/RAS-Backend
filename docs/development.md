# Local development

## Required tools

- Go 1.27.2 (the three `go.mod` files are aligned).
- Docker Desktop with Docker Compose v2.
- Git.
- GNU Make for the convenience targets; commands below can also be run directly.
- PostgreSQL client and Goose v3.28.0 only when connecting/migrating outside Compose.

## Configure and start

From the RAS-backend root, copy `.env.example` to `.env` only if `.env` does not already exist. Fill in unique random alphanumeric passwords and a JWT secret of at least 32 random characters. Do not commit `.env`.

```powershell
if (-not (Test-Path .env)) { Copy-Item .env.example .env }
docker compose -f deploy/docker-compose.yml up --build -d
docker compose -f deploy/docker-compose.yml ps
```

Compose waits for PostgreSQL health, creates restricted service login roles, runs the single `migrate` job, and starts each API only after that job exits successfully. API base ports are 9001/9002/9003; each exposes `GET /health`. Attendance Swagger UI is at `http://localhost:9003/swagger/index.html` and uses a bearer token.

To view logs or stop:

```powershell
docker compose -f deploy/docker-compose.yml logs -f --tail=100
docker compose -f deploy/docker-compose.yml down
```

`down` keeps the named database volume. `make db-reset-local` is explicitly destructive: it removes the local volume, all local data, and recreates schema. Never use it for shared AWS development.

## Run an individual application

Compose is the supported default because it supplies database roles and applies schema first. For direct Go execution, first start local PostgreSQL, create the roles, and apply migrations. Copy the relevant `services/<app>/.env.example` to `.env` in that app directory, provide its role password and shared JWT key, then run:

```powershell
Set-Location services/auth-api
go run ./cmd/api
```

Use the analogous `services/branch-api` and `services/attendance-api` directories. Auth creates the initial CEO only when `APP_ENV=development`, no CEO exists, and explicit seed values are present. There is no hardcoded default password.

## Go checks and builds

```powershell
make check
make vet
make test
make build-auth
make build-branch
make build-attendance
```

These operate independently in each module. The modules remain buildable without a `go.work` file. `make integration` runs disposable SQL inserts/queries in a transaction against the local Compose database.

## Migrations and seeds

Create a timestamped migration with Goose from the repository root:

```powershell
goose -dir database/migrations create add_employee_field sql
```

Use explicit Up/Down sections, update the owner service's repository/domain/API as needed, and test the full migration history on disposable PostgreSQL. Check status with `make db-status`, apply with `make db-up`. Load fixtures only with `APP_ENV=development make db-seed` after Compose is running. See [database workflow](database.md).

## API contract and errors

The route inventory is in [API docs](api.md); machine-readable Attendance schemas are in `services/attendance-api/docs/`. Auth and Branch schemas are not currently available as OpenAPI documents. Do not infer missing endpoints from the wider product backlog.

## Configuration inventory

| Variable | Required by | Notes |
|---|---|---|
| `DATABASE_URL` | All APIs, optional alternative | PostgreSQL URL; if absent, all of `DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_USER`, and `DB_PASSWORD` are required. |
| `DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_USER`, `DB_PASSWORD` | All APIs when no URL | Compose uses host `postgres`, database `restaurant_db`, and a distinct service role/password. |
| `DB_SSLMODE` | All APIs, optional | Defaults to `disable` locally; AWS deploy sets `require`. |
| `JWT_SECRET` | All APIs | Same secret across APIs; minimum 32 characters. |
| `PORT` | Each API, optional | Defaults to 9001/9002/9003. |
| `DB_MAX_OPEN_CONNS`, `DB_MAX_IDLE_CONNS` | Auth and Attendance, optional | Positive pool sizes; service defaults apply otherwise. |
| `AUTH_ACCESS_TTL`, `AUTH_REFRESH_TTL` | Auth, optional | Positive Go durations; default access 15 minutes and refresh 30 days. |
| `AUTH_MAX_FAILED_ATTEMPTS`, `AUTH_MAX_LOCKOUT`, `AUTH_BASE_LOCKOUT`, `AUTH_BCRYPT_COST`, `AUTH_LOGIN_RATE_PER_MIN` | Auth, optional | Positive integer/duration settings for lockout, password hashing, and per-IP login rate limits; defaults are defined in `internal/auth/config.go`. |
| `APP_ENV` | Auth bootstrap / local safety | Set `development` to enable explicit CEO bootstrap and local-only reset/seed guards. |
| `SEED_CEO_NAME`, `SEED_CEO_EMAIL`, `SEED_CEO_PASSWORD` | Auth in development when CEO absent | Explicit local fixture values; never put real secrets in tracked examples. |
| `MIGRATION_DB_USER`, `MIGRATION_DB_PASSWORD` | Compose migration/bootstrap | Controlled local migration role; never an API runtime credential. |
| `AUTH_DB_PASSWORD`, `BRANCH_DB_PASSWORD`, `ATTENDANCE_DB_PASSWORD` | Compose/runtime role bootstrap | Separate credentials for the three runtime roles. |
| `DB_PORT_FORWARD` | Compose, optional | Host port for PostgreSQL, default 5432. |
| `AUTH_API_URL`, `BRANCH_API_URL` | Currently unused | Reserve for future explicit internal HTTP clients; no current code consumes these variables. |

AWS operator configuration is listed in [deployment](deployment.md); runtime secrets are fetched from Secrets Manager on the host and are not stored in Terraform source.
