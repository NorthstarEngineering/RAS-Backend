# Database

One PostgreSQL database named `restaurant_db` serves all three applications. `database/migrations/` is the centralized Goose history. `database/legacy-migrations/` holds original SQL for audit and preservation only; it must never be applied as a second active history.

## Ownership and migration order

Branch owns `branches`; Auth owns `users` and `refresh_tokens`; Attendance owns `employees`, `attendance_records`, `deductions`, and `overtime`. The present order is branches, users, refresh tokens, user-to-branch foreign key, then attendance-owned tables. The FK and attendance tables depend on branches. Attendance records depend on employees and branches; deductions depend on attendance records, employees, and branches; overtime depends on employees and branches. Auth's user records intentionally do not FK to employee records.

Use one controlled migration role/job. Runtime roles `auth_app`, `branch_app`, and `attendance_app` are provisioned separately with no superuser, database/role creation, replication, or RLS-bypass capability; each has a five-connection limit. They receive only `USAGE` on `public`, `CONNECT` on `restaurant_db`, and table grants required by their owner. Auth and Attendance receive read-only access to Branch-owned `branches` for existing cross-service checks. These read grants are real database coupling, not service isolation. Application pools are capped at five open/two idle per service; overall expected API capacity is at most 15 runtime sessions, leaving RDS connection capacity for migration/admin tasks. Existing installations must be inventoried and explicitly baselined before the new history is run; migration compatibility against an existing database is not verified here.

`database/bootstrap/roles.sql` must be run as the controlled database administrator before migrations. Migration SQL conditionally grants only owner DML and the documented Branch reads. PostgreSQL runtime roles do not own the tables or get DDL privileges. Re-run the role bootstrap through an approved process when rotating runtime passwords. The current RDS definition enables encryption, private networking, seven-day automated backups, a final snapshot, deletion protection, and `prevent_destroy`; confirm backup retention and recovery with an actual restore exercise before relying on it.

## Goose commands

The migration runner image uses Goose v3.28.0. Locally, with Compose running and `.env` configured:

```powershell
docker compose -f deploy/docker-compose.yml run --rm migrate status
docker compose -f deploy/docker-compose.yml run --rm migrate up
```

Or install Goose and set its documented environment configuration explicitly:

```powershell
$env:GOOSE_DRIVER = 'postgres'
$env:GOOSE_DBSTRING = $env:DATABASE_URL
$env:GOOSE_MIGRATION_DIR = 'database/migrations'
$env:GOOSE_TABLE = 'goose_db_version'
goose up
```

Never have each API independently run migrations. CI applies the complete history to a disposable PostgreSQL database. Every new migration uses a timestamped descriptive name and explicit Goose Up/Down sections. A Down migration can destroy data; prefer a reviewed forward fix for shared development.

## Local database

Normal `docker compose down` preserves the named local PostgreSQL volume. To deliberately erase this disposable local database, verify `APP_ENV=development` and run `make db-reset-local`. This removes the Compose volume and every row in it, then recreates the local stack. Never use this procedure against shared development. Apply migrations with `make db-up`; fixtures are separate and loaded with `make db-seed` only when `APP_ENV=development`.

## Shared development data and recovery

RDS is configured with encryption, seven-day automated backups, deletion protection, `prevent_destroy`, and a final snapshot. This provides recoverable backups, not zero data loss or high availability. EC2 replacement does not delete RDS. Before an intentional recovery, identify account, region, instance, and snapshot timestamp; restore a selected snapshot to a new database, verify it, then deliberately update the endpoint. Replaying migrations on an empty database restores schema only, not historical business data. Do not import production data without authorization and privacy review. See [deployment.md](deployment.md).
