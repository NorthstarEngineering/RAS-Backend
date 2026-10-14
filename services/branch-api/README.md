# Branch API

Independent Go module for branch records and operating configuration. It listens on port 9002 and exposes `GET /health`.

This service owns `branches`, enforces CEO-only create/update operations, and limits Branch Manager reads to the branch in the signed Auth-issued JWT. Caller-supplied identity headers are ignored. See [the API reference](../../docs/api.md) and [security notes](../../docs/security.md).

Run through root Docker Compose so the shared Goose migration history is applied first. See [development docs](../../docs/development.md). Schema changes belong in the central `database/migrations/` directory.
