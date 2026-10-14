# Decision 002: one shared PostgreSQL database and Goose history

**Status:** Accepted for development baseline
**Date:** 2026-10-13

All three applications connect to `restaurant_db` on one PostgreSQL server. A single Goose history in `database/migrations/` owns ordered schema changes. Each table has one service owner; migration execution is centralized and performed with a controlled role. Runtime database roles receive only required table privileges.

The existing source contains duplicate/conflicting definitions of branches and attendance-owned tables, plus repeated overtime SQL and ordering dependencies. Original SQL is archived for audit; the central migration chain defines the fresh-install schema. Existing database migration records were not available, so adoption requires inspection and an explicitly reviewed baseline. Shared foreign keys and cross-service branch reads remain release coupling.

**Consequences:** all schema changes require coordinated review; deployment order may depend on forward-compatible migrations; API containers do not run DDL. Down migrations are not an automatic recovery strategy because they may destroy data.
