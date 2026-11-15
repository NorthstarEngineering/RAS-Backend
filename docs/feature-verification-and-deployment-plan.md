# Feature Verification and Deployment Plan

Status: proposed execution plan
Prepared: 2026-11-15

## Execution ledger

- [x] Combined candidate branch created and candidate APIs integrated (Inventory, Menu, Transfer, Sales, Cost, Central Kitchen Supply).
- [x] Branch API test compilation defects fixed.
- [x] Combined Go test suite run across all nine service modules; final run passed. See `test-results/combined-go-test-run.txt`.
- [x] Docker Desktop Linux engine started; candidate Compose images built; isolated DB migrations completed; Auth, Branch, Attendance, and Menu health endpoints passed.
- [x] Deployment definitions, Dockerfiles, shared migration, and runtime roles added for Inventory, Transfer, Sales, Cost, and Central Kitchen.
- [x] GitHub fresh-database migration validation passed for the new shared feature schema.
- [x] GitHub full Compose build, startup, and health checks for all nine APIs passed on candidate `e8f23637d447a9a0a25839f3f0a82dd58f851e50` (run `43733645153`).
- [x] Runtime-role bootstrap, shared migrations, and service health checks passed on a clean disposable PostgreSQL database.
- [ ] Feature API/database contract matrix and authenticated business-flow compatibility review for every new service.
- [x] Add deployment definitions for Inventory, Transfer, Sales, Cost, and Central Kitchen Supply.
- [ ] Run authenticated full-stack feature flows against PostgreSQL for all services; the passing smoke proves startup, migrations, role bootstrap, and health only.
- [ ] Update CI/reporting to preserve consolidated logs and all bug metadata for the integrated suite.
- [x] Final verification report and merge to `main`; PR #24 merged at `da849f4fafe3d8f38ee0783ce01acbd05a548ca3` after every applicable PR check passed.
- [x] Integrated feature services and deployment definitions are present in GitHub `main`.
- [ ] Cloud deployment: no cloud target/environment was selected or deployed in this task. Compose verification is not a cloud deployment.

All nine services are wired into the deployment and pass clean-stack startup and health checks. Authenticated business-flow checks against each API, external mobile-app compatibility, and deployment to a cloud environment remain open and must not be counted as verified until executed.

## Goal

Verify that the backend features on the active and remote feature branches are correct, compatible with the app/API contract, and compatible with the shared PostgreSQL schema. Add or improve tests wherever coverage is missing, capture every result and defect in a structured report, then verify the Docker deployment configuration starts and exposes all currently implemented features.

## Scope and starting point

- The checked-out branch is `main` at `e119d61`; the worktree was clean when this plan was prepared.
- Local branches include the Auth, Branch, Attendance, database, deployment, mobile handoff, audit, and test/CI feature branches. Remote branches also include Menu, Inventory, Sales, Transfer, Cost, and Central Kitchen feature branches. The branch list is an inventory only; branch contents and merge status must be confirmed before treating any feature as part of the deliverable.
- The repository currently has three Go services: Auth, Branch, and Attendance. Menu, inventory, sales, transfer, cost, and central-kitchen workflows are not listed as implemented in `docs/audit.md`; inspect their branches before claiming they are available.
- Dockerfiles already exist for all three APIs and the database migration runner. `deploy/docker-compose.yml` already defines PostgreSQL, role bootstrap, migrations, and the three APIs. The task is therefore to validate and maintain the deployment setup against verified features, adding or changing Docker configuration only where evidence shows a gap.
- Existing verification assets include Go tests, `tests/integration/`, CI workflows, `scripts/generate-go-test-report.py`, and `docs/test-execution-report.md`. These are inputs to the audit, not proof that every branch or feature is currently tested end to end.

## Execution stages

### 1. Establish the branch and feature inventory

1. Record the current commit, clean/dirty status, local and remote branches, and merge bases with `main`.
2. For each feature branch, record its commits and changed files; identify whether it is merged, superseded, stale, or still a candidate for integration.
3. Map each implemented feature to its service, routes, request/response contract, authorization rules, database tables/migrations, and expected app caller.
4. Compare the mobile handoff/OpenAPI contract to actual handlers and authentication behavior. Record app-repository checks that cannot be performed from this backend checkout as an explicit external dependency.
5. Treat feature branches as candidates until they pass review and their integration order/dependencies are clear. Preserve existing branch history; do not force-push or delete branches as part of this work.

**Gate:** a feature/branch matrix exists and every in-scope feature has an owner service, API contract, schema dependency, and integration disposition.

### 2. Verify code and integration correctness

For every in-scope feature branch, in dependency order:

1. Review behavior against requirements and API contracts: validation, success/error status codes, authorization, tenant/branch scoping, and backward compatibility.
2. Check service wiring and app compatibility: routes, JSON names/types, token claims/roles, ports, environment variables, and documented base URLs.
3. Check database compatibility: migration ordering and reversibility, constraints/indexes, repository queries, least-privilege grants, and fresh-schema behavior. Do not claim upgrade safety for an existing database without inspecting its state and migration history.
4. Run formatting, static analysis, builds, existing unit tests, API/handler tests, and database integration checks using the repo's supported commands and CI equivalents.
5. Fix defects on the appropriate feature branch, then rerun the failed checks. Keep unrelated features isolated and integrate only after their gates pass.

**Gate:** all in-scope branches pass their applicable checks, or have a documented blocker with evidence and a clear owner/action. No unresolved critical integration/security defect is accepted as ready.

### 3. Find coverage gaps and add tests

1. Inventory test files and CI execution for each service/module and each feature branch. Mark unit, handler/API, database integration, contract, migration, authorization, and container/runtime coverage separately.
2. Identify repositories/packages with no tests and important behavior with only shallow tests; prioritize data writes, auth boundaries, branch scope, error paths, and migration interactions.
3. Add focused tests at the narrowest useful layer, plus integration tests where behavior depends on PostgreSQL or cross-service contracts.
4. Ensure tests are deterministic, isolated, and runnable in CI. Add/update CI jobs only where needed so new tests run on pull requests and `main`.
5. Rerun the complete applicable suite and retain machine-readable logs, coverage output, and CI links/artifacts.

**Gate:** every in-scope module has an explicit test inventory; previously untested repositories have tests for meaningful behavior; CI executes them; failures are fixed or reported as blockers.

### 4. Produce the verification and bug report

Maintain a report (update `docs/test-execution-report.md` or create a dated companion report) with one entry per finding. Each bug entry must include:

- Stable ID and concise title
- Affected branch, service/repository, file/route, and feature
- Environment, commit SHA, and date/time
- Severity/priority and status/owner
- Preconditions and exact reproduction steps
- Expected result and actual result
- Evidence: test name/log excerpt, request/response, SQL result, CI run, or screenshot as applicable
- Test type: unit, handler/API, contract, integration/database, migration/schema, auth/security, build/static analysis, or Docker/runtime/smoke
- User/app/database impact, suspected cause, and proposed fix
- Retest result and the commit that resolves it

Also report passed checks and skipped checks with reasons. Separate confirmed defects from coverage gaps, environment blockers, and external app checks that require access to another repository/environment. Do not record secrets or personal data in evidence.

**Gate:** report allows another engineer to reproduce each open defect and distinguish it from a test gap or environmental limitation.

### 5. Maintain and verify Docker deployment

1. Review each existing Dockerfile for build reproducibility, minimal runtime contents, non-root execution where feasible, health-check compatibility, and required CA/timezone/runtime dependencies.
2. Review Compose service dependencies, migration-once ordering, database health, runtime database roles, environment variables, ports, persistent volume behavior, and secret handling.
3. Update Dockerfiles/Compose and deployment docs only for verified gaps; keep database migrations separate from API startup and preserve data on normal shutdown.
4. Build all images and start the full stack from a clean local test environment. Verify service health endpoints and representative flows for every implemented feature, including Auth → protected API requests → PostgreSQL writes/reads.
5. Validate schema/migration checks, service-to-database connectivity under restricted roles, restart behavior, and logs. Verify feature parity against the feature matrix so no implemented service is omitted from deployment.
6. Record image build details, commands, environment shape (without values), health/smoke results, failures, and report links.

**Gate:** the complete stack builds and starts, all services become healthy, representative feature flows pass against PostgreSQL, and the deployment contains every feature classified as implemented. Any feature branch not integrated is clearly excluded from the deployed feature set.

## Final completion criteria

- Branch/feature inventory and integration decisions are documented.
- App/API and database compatibility checks pass for the in-scope feature set.
- Test gaps are closed or explicitly tracked; CI runs the added tests.
- A reproducible test/bug report includes the required attributes and test types.
- Docker/Compose is maintained, builds, starts, and passes the feature smoke checks against PostgreSQL.
- Remaining blockers and unverified external dependencies are explicit; no AWS apply or production deployment is implied by local Docker validation.

## Execution order at a glance

`Inventory branches → map features/contracts/schema → verify integrations → add missing tests → rerun and report → maintain Docker/Compose → full-stack smoke verification`



