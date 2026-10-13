# auth module

## Run first-time setup
1. `go get github.com/golang-jwt/jwt/v5 golang.org/x/crypto github.com/google/uuid && go mod tidy`
2. Add `JWT_SECRET`, `SEED_CEO_EMAIL`, `SEED_CEO_PASSWORD` to `.env` (see `.env.example`).
3. Run migrations in order: `auth_001.sql`, `auth_002.sql`, ... and `auth_003_branch_fk.sql` **after** the branch module creates `branches`.
4. `go test ./internal/auth/...` then `go run ./cmd/api`.

## Endpoints (`/api/v1`)
| Method | Path | Who |
|---|---|---|
| POST | /auth/login | public (IP rate-limited) |
| POST | /auth/refresh | public (IP rate-limited) |
| POST | /auth/logout | public (body: refresh_token) |
| GET | /auth/me | any logged-in user |
| POST | /auth/change-password | any logged-in user (works while must_change_password) |
| PATCH | /auth/me/email | any logged-in user (needs current password) |
| POST | /users | CEO - create Branch Manager / Supply Manager |
| GET | /users | CEO |
| PATCH | /users/:id/status | CEO - `{"is_active": false}` |
| POST | /users/:id/reset-password | CEO |
| POST | /users/:id/unlock | CEO |

## For other modules (no imports from auth!)
Protect a route group in `cmd/api/main.go`:

    api.Group("/sales", authModule.Middleware.Authenticate(),
        authModule.Middleware.RequireRoles(model.RoleBranchManager))

Inside your handler read the identity from the gin context:

    role     := c.GetString("role")       // CEO | BRANCH_MANAGER | SUPPLY_MANAGER
    userID   := c.GetString("user_id")
    branchID := c.GetString("branch_id")  // "" for CEO / SUPPLY_MANAGER

Branch scope rule (FR-AUTH-004): if role == BRANCH_MANAGER, always use `branch_id`
from the context, never a branch id sent by the client.
