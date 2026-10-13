# Database workflow

`migrations/` is the single Goose migration history for `restaurant_db`. `seeds/dev.sql` contains disposable development fixtures. `bootstrap/roles.sql` provisions the three limited runtime roles and is run as a controlled migration/database administrator operation. `legacy-migrations/` is retained for audit and must not be executed.

Create migrations with Goose tooling into `database/migrations/`, review ownership/dependencies, and test on disposable PostgreSQL. Do not edit an applied migration, manually alter shared schema, or run the full migration history from each API at startup.

See [database operations](../docs/database.md) for migration commands, ownership, reset precautions, and recovery.
