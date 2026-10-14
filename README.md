# RAS Backend

Three independently buildable Go APIs share one PostgreSQL database. The repository is in development; no production deployment is configured.

## Services

| Service | Port | Health | Responsibilities |
|---|---:|---|---|
| Auth API | 9001 | `GET /health` | Authentication, users, refresh tokens |
| Branch API | 9002 | `GET /health` | Branch records and settings |
| Attendance API | 9003 | `GET /health` | Employees, attendance, deductions, overtime |

The source audit and scope gaps are in [docs/audit.md](docs/audit.md). See [architecture](docs/architecture.md), [API reference](docs/api.md), [database workflow](docs/database.md), [development](docs/development.md), [security](docs/security.md), and [deployment](docs/deployment.md).

## Local start

Install Go 1.27.2, Docker Desktop with Compose v2, Git, and optionally GNU Make. Copy `.env.example` to `.env` only if `.env` does not already exist, then set unique local passwords and a random JWT secret.

```powershell
docker compose -f deploy/docker-compose.yml up --build -d
docker compose -f deploy/docker-compose.yml ps
docker compose -f deploy/docker-compose.yml logs -f --tail=100
docker compose -f deploy/docker-compose.yml down
```

Normal shutdown retains PostgreSQL data in its named volume. To deliberately erase only the disposable local volume and replay schema, set `APP_ENV=development` and run `make db-reset-local`; this deletes all data in that local volume. See [database reset and migration details](docs/database.md).

## Checks

Run `make check vet test build`. Run `make integration` with the Compose database available. Central Goose migrations are under `database/migrations/`; apply them with `make db-up`. Do not have APIs apply schema at startup.

## Git and AWS boundaries

The original three Git histories are preserved locally under `.git-preservation/` and excluded from this repository. The monorepo uses the `NorthstarEngineering/RAS-Backend` GitHub repository. Changes are organized as stacked feature branches and should be merged through pull requests in dependency order. Terraform contains development definitions only; no AWS infrastructure has been applied. Do not apply/destroy infrastructure or reset shared development data without explicit approval.
