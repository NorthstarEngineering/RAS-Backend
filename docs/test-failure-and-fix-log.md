# Test Failure and Fix Log

Prepared: 2026-11-15  
Scope: Confirmed test/build failures found while validating the current `main` checkout.

This log records verified failures and fixes. It is a running document; add entries when the combined feature suite discovers additional issues. Do not record a suspected issue as a confirmed defect until it has reproduction evidence.

## Confirmed findings

### TF-001 — Branch service tests did not compile: nested test declarations

- **Area:** Branch API, `services/branch-api/internal/branch/service_test.go`
- **Test type:** Go unit-test compilation
- **Symptom:** `go test ./...` reported `expected '(', found TestNormalizeTimeRejectsBoundaryFormats` at line 154.
- **Cause:** Several package-level test functions had been indented and placed inside `TestBranchTypeRevenueRules`. Go permits function literals inside functions but does not permit named `func Test...` declarations nested inside another function.
- **Fix:** Moved `TestNormalizeTimeRejectsBoundaryFormats`, `TestValidateLateRateAcceptsZeroAndTwoDecimals`, `TestValidateNameTrimsUnicodeAndCountsRunes`, and `TestUpdatePropagatesRepositoryErrors` to package scope. Kept the revenue assertions inside `TestBranchTypeRevenueRules`.
- **Why this fix:** Tests must be discoverable as package-level Go test functions. Restoring package scope preserves the intended coverage and allows the compiler and `go test` to discover each case.
- **Verification:** `go test ./...` from `services/branch-api` passes after the fix.

### TF-002 — Branch service tests did not compile: duplicate helper name

- **Area:** Branch API, `services/branch-api/internal/branch/service_test.go`
- **Test type:** Go unit-test compilation
- **Symptom:** After TF-001 was corrected, compilation reported `stringPointer redeclared in this block`; the helper was also defined in `service_qa_test.go`.
- **Cause:** Go files in the same package share one package-level namespace. A helper added in a second `_test.go` file reused a name already declared by another test file.
- **Fix:** Renamed the helper in `service_test.go` to `serviceStringPointer` and updated its call sites.
- **Why this fix:** The tests need a pointer helper, but its name must be unique across all source and test files in the package. A local descriptive name avoids changing the existing QA tests or their callers.
- **Verification:** `go test ./...` from `services/branch-api` passes after the rename.

## Why these failures mattered

Both findings were test-code defects, not evidence that the Branch API runtime behavior was broken. However, either compile error prevents **all** tests in the Branch API package from running. CI would fail at the same stage, and the service would have no trustworthy test result until the test package compiled. These failures also show why an integrated suite must compile every service and feature together before we treat it as ready to merge.

## Prevention steps

1. Keep every named `TestXxx(t *testing.T)` declaration at package scope. Close the current test function before starting another one; use editor brace matching and gofmt to catch misplaced blocks.
2. Before committing, run `gofmt` on changed Go files and `go test ./...` for the affected module. For integrated changes, run the repository-wide module test target after assembling the candidate feature set.
3. Use distinctive helper names or a shared test helper file. Before adding a package-level helper, search all `.go` and `_test.go` files in that package for an existing declaration.
4. Keep tests in the same package only when they need unexported symbols; remember that all files with the same package declaration share identifiers.
5. Configure CI to compile and test the complete integrated candidate, and retain the failing test output as an artifact. Do not merge to `main` until the combined test gate passes or an explicitly accepted blocker is documented.
6. When adding multiple tests in one edit, review the whole edited region, run gofmt, and inspect the diff before running or committing the change.

## Execution environment notes

- Go 1.27.2 was installed under `D:\DevTools\Go`; Go build and module caches were placed under `D:\DevTools\Go` for this session.
- Current `main` service tests previously passed for Auth and Attendance. Branch passed after the two fixes above.
- Docker Compose runtime validation remains unverified because the Docker Desktop Linux engine was unavailable in the environment at the time it was checked. This is an environment blocker, not a confirmed application defect.

### TF-006 — Compose role bootstrap did not create feature API roles

- **Test type:** Full Compose build/runtime smoke.
- **Symptom:** The unified migration failed with `role "central_kitchen_app" does not exist`.
- **Cause:** Compose used `/bin/sh -ec` as entrypoint while passing psql and its flags as separate arguments. The shell did not run the intended psql command.
- **Fix:** Set `psql` as the entrypoint and pass its variables and SQL file as explicit command arguments.
- **Retest:** Fresh-database migration validation passed in GitHub Actions after the fix. Full Compose smoke remains pending.

### TF-007 — Existing API health checks depended on missing wget

- **Test type:** Docker/runtime health check.
- **Symptom:** Auth became unhealthy during full Compose smoke.
- **Cause:** Compose health checks invoke `wget`, but Auth, Branch, and Attendance runtime images installed CA certificates only.
- **Fix:** Added `wget` to the runtime packages in all three Dockerfiles.
- **Retest:** Full Compose CI smoke passed for all nine APIs on commit `e8f23637d447a9a0a25839f3f0a82dd58f851e50` (run `43733645153`).

### TF-008 — Role bootstrap passed literal environment placeholders to psql

- **Test type:** Combined Docker Compose build and runtime smoke across all APIs.
- **Symptom:** The next combined run failed the new role-bootstrap invocation with exit code 127 before migration could start.
- **Cause:** Adding a mounted shell wrapper introduced another command-resolution boundary and still left credentials dependent on command/entrypoint behavior.
- **Fix:** Removed the wrapper. The shared psql bootstrap file now reads all nine passwords directly from its process environment using psql `\\getenv` commands, and it raises an error unless all expected roles exist. Both Compose and the fresh-database workflow use the same direct `psql -f roles.sql` path.
- **Why this fix:** psql itself handles environment-to-variable transfer, so no shell interpolation or argument forwarding is needed. The role postcondition makes silent bootstrap success impossible.
- **Retest:** Fresh-database role and migration validation passed, and the all-service Compose smoke passed on commit `e8f23637d447a9a0a25839f3f0a82dd58f851e50` (GitHub Actions run `43733645153`).
- **Prevention:** Keep database bootstrap behavior identical between CI and Compose, and validate the resulting role state inside the bootstrap transaction/path before allowing dependent migrations to run.

### TF-009 — Inventory API attempted DDL with its restricted runtime role

- **Test type:** Full Compose startup and database migration smoke.
- **Symptom:** Role creation and Goose migrations succeeded, but Inventory became unhealthy with `permission denied for schema public` while creating its embedded migration tracking table.
- **Cause:** Inventory re-ran DDL migrations during API startup using `inventory_app`. Runtime roles intentionally lack schema ownership and `CREATE` privileges, and the same inventory schema is already provisioned by the privileged shared migration runner.
- **Fix:** Removed the redundant embedded migration call from Inventory startup. Deployment's Goose service remains the single DDL migration owner and Compose starts Inventory only after that service succeeds.
- **Why this fix:** Schema changes run once under the dedicated migration account; API connections retain least privilege for their runtime queries.
- **Retest:** All nine API images built and the full Compose stack became healthy; all health endpoints passed on commit `e8f23637d447a9a0a25839f3f0a82dd58f851e50` (run `43733645153`).
- **Prevention:** Keep schema migration execution in the dedicated deploy migration stage; runtime services should only connect and serve requests under their restricted database roles.

### TF-003 — Integrated feature modules failed the combined Go suite

- **Environment/commit:** Integrated candidate branch; Windows amd64; Go 1.27.2; 2026-11-15.
- **Test type:** Combined unit tests (`go test ./...` in every `services/**/go.mod` module).
- **Initial failures:** Central Kitchen returned internal errors for missing goods and malformed supply request items; negative received/approved quantities were not rejected before repository/database calls. Standalone Inventory recorded an adjustment's resulting balance instead of its signed delta. Sales allowed discount cancellation on a closed shift; a valid cancellation fixture also omitted its expected open shift.
- **Fixes:** Added appropriate Central Kitchen 404/400 mappings and request item validation before DB access; preserved the signed inventory adjustment delta as movement quantity; required an active open shift before Sales discount cancellation; made the valid test fixture represent an open shift.
- **Follow-up:** The handler contract tests distinguished `validation_error` for a missing movement reason from `invalid_payload` for malformed requests. The mapping now preserves that distinction.
- **Retest:** Combined run passed across all nine Go modules. Output: `test-results/combined-go-test-run.txt`.

### TF-004 — Docker engine absent at first runtime check

- **Test type:** Docker engine / Compose build / runtime smoke.
- **Cause:** Docker Desktop was installed but not running; its Linux named-pipe endpoint did not exist.
- **Fix:** Started Docker Desktop. `docker info` then reported server 29.8.0, Linux.
- **Retest:** Compose image builds succeeded for Auth, Branch, Attendance, Menu, and the Goose migration runner. On an isolated smoke database, Goose migrations completed and all four API containers reported healthy; `/health` returned 200 on ports 9001, 9002, 9003, and 9005.
- **Additional issue and handling:** Menu's Dockerfile used Go 1.23 despite its module requiring 1.27.2; updated the builder/runtime stages and rebuilt successfully. An initial smoke environment also had an occupied host port and mismatched development seed settings; used a separate smoke volume/port and production mode for the disposable health-only Auth check. Existing local database data was retained.
- **Scope limitation:** Current Compose still does not include Inventory, Transfer, Sales, Cost, or Central Kitchen services. Their integration into deployment and feature-level DB flows remain open.

### TF-010 — Final integrated candidate verification

- **Environment/commit:** GitHub Actions `ubuntu-latest`; PR #24 head `e8f23637d447a9a0a25839f3f0a82dd58f851e50`; 2026-11-15.
- **Test types:** Combined Go unit/build suite, feature service tests, OpenAPI lint, fresh-database migration/schema/role validation, service-specific CI validation, Docker image build, Compose runtime startup, and HTTP health smoke.
- **Result:** PASS. All nine API services built and became healthy after role bootstrap and shared migrations. Health endpoints passed for ports 9001–9008 and 9010. Combined Go tests, feature service tests, migration checks, OpenAPI lint, and existing API validations passed.
- **Evidence:** [Full Compose Actions run](https://github.com/NorthstarEngineering/RAS-Backend/actions/runs/43733645153); [PR #24](https://github.com/NorthstarEngineering/RAS-Backend/pull/24).
- **Remaining coverage boundary:** This verifies deployment startup, DB migrations/roles, and API liveness. It does not execute authenticated business transactions through every feature API or validate an external mobile app repository. Those remain separate integration coverage items.


