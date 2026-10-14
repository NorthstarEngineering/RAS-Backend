# Auth API

Independent Go module for authentication, users, and refresh-token sessions. It listens on port 9001 and exposes `GET /health`.

The service owns `users` and `refresh_tokens`; it reads Branch-owned `branches` to validate assignments. It serves the auth and CEO user-management routes listed in [the API reference](../../docs/api.md). Passwords use bcrypt; access tokens use HS256 and refresh tokens are rotated and stored hashed.

Run it through the root Compose setup so the central migration history is applied first. Direct local development instructions, required settings, and checks are in [development docs](../../docs/development.md). Schema changes belong in `database/migrations/`; this service never runs DDL at startup.
