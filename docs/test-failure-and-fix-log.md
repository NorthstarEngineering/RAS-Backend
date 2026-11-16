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

### TF-015 — Reports API CI credentials, database privileges, and branch scope

- **Affected branch/service:** `feature/reports-api`, Reports API, database bootstrap and migrations, Docker stack and database CI workflows.
- **Environment/commit/date:** Local repair branch based on PR #26 head `ee6a6e5`; Windows amd64, Go 1.27.2; 2026-11-16.
- **Severity/status:** High; fix implemented, full migration and combined-stack retest pending.
- **Test types:** CI failure diagnosis, handler/API authorization, Go unit tests, database role/migration integration, Docker runtime smoke.
- **Reproduction/evidence:** PR #26 Docker stack validation run `43745690720` failed before Compose startup because its generated `.env` omitted required `REPORTS_DB_PASSWORD`. After fixing that, run `43823503673` revealed the Reports Dockerfile used Go 1.23.12 while the module requires Go 1.27.2. Inspection also found `roles.sql` hardcoded `reports_ci_password`, the smoke health loop skipped port 9009, Reports had no tests, reporting tables lacked SELECT grants for `reports_app`, and branch/query filters were not enforced by handlers. Reports attendance SQL counted `open` as absent even though an open record represents a successful check-in.
- **Expected/actual impact:** A fresh stack could fail during role bootstrap or use credentials different from the API; report endpoints could fail on permission denied; branch managers could request cross-branch sales/inventory data; reports queries could return incorrect or unscoped counts.
- **Fix:** Use `REPORTS_DB_PASSWORD` for create and alter role operations and enforce restricted role attributes; add the credential to both CI workflows; health-check port 9009; align the Reports builder with Go 1.27.2 and its matching static build; grant `reports_app` read-only access to report source tables; require CEO/branch-manager/supply-manager role as appropriate and force branch-manager scope from trusted auth context; scope sales, inventory, attendance, and approval SQL; fix attendance/overtime aggregations; safely encode database URL credentials and support configured SSL mode; add handler tests for authentication, branch scope, and audit role restrictions plus integration checks for read-only grants.
- **Retest:** Reports API `go test ./...` passed after handler changes. Fresh migration/privilege validation and full ten-service Compose smoke are pending.
- **Prevention:** Every new runtime role must get its password from the same environment variable in local Compose, CI, and cloud deployment; every reporting query must have an explicit authorization/scope rule and corresponding handler and database tests; full-stack health loops must cover every service port.

### TF-016 — Audit actor attribution and transactional integrity

- **Affected branch/services:** `feature/audit-integration`; Sales, Menu, Central Kitchen Supply.
- **Environment/commit/date:** Local repair branch based on PR #27 head `177a799`; Windows amd64, Go 1.27.2; 2026-11-16.
- **Severity/status:** High; fixes implemented and targeted module tests pass; full combined suite and CI pending.
- **Test types:** Code review, unit/service tests, handler/API validation, transaction failure-path tests.
- **Reproduction/evidence:** Review of PR #27 found Sales shift closure logged `shift.ManagerID` as the actor, Menu approval logged `req.RequestedBy`, and supply approval/rejection/receipt logged the requester; shipment logged a hardcoded central-kitchen UUID. Sales and all four supply operations logged audit insert errors and still committed their business transactions. Audit routes also accepted role fallbacks from request payloads and did not require an authenticated actor ID.
- **Expected/actual impact:** Audit history could identify the wrong person, omit a real approver/receiver, or record a service placeholder as the actor. A successful business write could commit without a corresponding audit event, weakening traceability.
- **Fix:** Populate audit actor ID and role only from the trusted authenticated request context; validate the actor UUID; reject unauthenticated/unauthorized audit actions; use the actual actor for inventory movement metadata; return audit write failures before commit so transaction rollback preserves atomicity. Add tests that verify actor identity and that audit insert failures roll back Sales and Supply transactions; add Menu tests for CEO and actor ID requirements.
- **Retest:** `go test ./...` passed in Sales, Menu, and Central Kitchen Supply modules after the fixes. Full combined run is pending.
- **Prevention:** Audit event columns must come from authenticated claims, never the record creator or request body. For audited mutations, the business update and audit insert must share one transaction and the audit insert must be required for commit.


### TF-017 — AWS deploy automation and complete service matrix

- **Test type:** Terraform validation, API Gateway proxy unit tests, GitHub Actions workflow integration, full Docker Compose smoke.
- **Symptom:** Development infrastructure published only part of the service set and had no automatic post-smoke path to apply migrations and roll out all APIs. Reports was absent from the deployment inventory.
- **Fix:** Expanded ECR to all ten API images plus migration runner; added Reports credentials, repository, port 9009 health check, and deploy command; configured OIDC-limited SSM permissions and a success-triggered deployment workflow that runs migrations first, then starts and health-checks all ten APIs.
- **Verification:** Terraform validation passed; request-proxy tests passed (4/4); Compose configuration validation passed. End-to-end AWS apply and deployment remain pending infrastructure state and successful GitHub publication workflow.
- **Prevention:** Keep Compose smoke as the deployment prerequisite, publish immutable commit-SHA images, run schema migration before application rollout, and require SSM health confirmation before reporting success.

### TF-018 — Legacy feature tests used obsolete actor and audit assumptions

- **Affected areas:** Salvaged tests from legacy Menu and Sales feature PRs.
- **Test type:** Combined Go unit-test suite (`go test ./...` across all service modules).
- **Symptom:** CI reported 11 failures in Menu and Sales after those legacy handler/service tests were copied into the integrated candidate. Handler requests had no authenticated actor context, some service cases omitted actor identity, and the fake SQL connectors rejected the required audit insert with `not supported`.
- **Cause:** The test suites predated the audit hardening merged in PR #27. They expected actor role/ID fallbacks from request JSON or implicit defaults and used mocks without support for transactional audit writes.
- **Fix:** Removed those obsolete imported suites instead of weakening authenticated actor requirements or audit transaction behavior. Kept the integrated audit-focused tests (`audit_test.go`, `rass93_qa_test.go`) and the existing feature QA tests.
- **Retest:** Fresh combined Go and feature-service workflows are running on PR #28 after removal. Do not treat the earlier candidate run as green.
- **Prevention:** When salvaging tests from older branches, update auth context and transaction fakes to match current audited contracts, then run them with the complete module suite before retaining them.
### TF-019 — AWS deployment identity and Terraform example inputs are not provisioned

- **Test type:** GitHub environment and secret inventory, AWS STS identity check, Terraform variable validation.
- **Symptom:** The repository has no `development-publish` environment variables or secrets, and `aws sts get-caller-identity` returns exit 255 in the current machine. The Terraform example also omitted `REPORTS_DB_PASSWORD`, which the application-secret validation now requires.
- **Cause:** AWS OIDC role bootstrap and its region/role variables have not been provisioned in this repository/account context; there is no AWS CLI credential available to initialize infrastructure or write GitHub environment variables. The sample secrets JSON had not been extended when Reports was added.
- **Fix:** Added `REPORTS_DB_PASSWORD` to the Terraform example. The deployment workflow uses GitHub OIDC and the Terraform-managed role without static AWS keys; its required variables are `AWS_ROLE_TO_ASSUME` and `AWS_REGION` in the `development-publish` environment.
- **Verification/status:** Terraform validate passes. Live infrastructure apply, ECR publish, and service deployment cannot start until a bootstrap AWS identity creates/applies the OIDC deployment role and the two environment variables point to it and `eu-west-3`.
- **Prevention:** Verify AWS STS and GitHub environment prerequisites before triggering a cloud rollout; keep the sample secret schema synchronized with every runtime role; never claim cloud deployment based on CI-only container smoke.
### TF-020 — Reports API was not reachable through the app gateway

- **Test type:** API Gateway request-proxy route integration test and deployment code review.
- **Symptom:** `/api/v1/reports/*` had no proxy target, so the Lambda proxy returned 404 even though Reports was present in Compose, ECR, and the EC2 deploy script.
- **Cause:** The Reports service was added after the request-proxy route matrix and that matrix had no Reports case.
- **Fix:** Route `/api/v1/reports` and descendants to port 9009, preserving the path; added the Reports sales endpoint to the proxy forwarding test matrix.
- **Retest:** `node --test infra/terraform/dev/request-proxy.test.mjs` rerun and fresh PR CI pending.
- **Prevention:** Every newly deployed API must have an API Gateway route mapping and a proxy test in addition to its container health check.
### TF-021 — Reports Compose DSN did not encode password characters

- **Test type:** Deployment configuration review and Docker Compose interpolation validation.
- **Symptom:** Reports Compose built `DATABASE_URL` by string interpolation. A password containing `@` (allowed by Terraform validation) could be parsed as URL authority syntax and fail database authentication.
- **Cause:** The Reports API already supports individual `DB_*` variables and encodes credentials with `url.UserPassword`, but Compose bypassed that safe path.
- **Fix:** Pass `DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_USER`, `DB_PASSWORD`, and `DB_SSLMODE` to Reports in Compose so the service builds the DSN safely.
- **Retest:** Compose config validation and the combined ten-service Compose smoke are pending on the latest PR commit.
- **Prevention:** Pass credentials as structured environment fields and use standard URL constructors; avoid embedding raw secrets in DSN strings.

### TF-022 — OpenAPI CI requested an unpublished Redocly version

- **Affected area:** OpenAPI contract validation workflow.
- **Environment/commit/date:** GitHub Actions on merged `main` commit `5bee53dc727cc8c052c65b6b8c0e4f4a722fded8`; 2026-11-16.
- **Severity/status:** Medium; workflow fix implemented, retest pending.
- **Test type:** Main-branch CI diagnosis and OpenAPI contract lint.
- **Symptom:** Main-branch API contract run `43830158147` failed before reading the OpenAPI document with npm `ETARGET`: no matching version was found for `@redocly/cli@2.79.0`. The PR-head run had passed, showing the floating `npx` resolution can produce a different package selection.
- **Cause:** The workflow invoked `npx @redocly/cli` without pinning a known package version, so CI's resolution was not reproducible and selected an unavailable release.
- **Fix:** Pin the workflow command to the published `@latest` dist-tag so npx cannot mistake `2.79.0` for a valid package version. No API schema changes were needed.
- **Retest:** The follow-up PR OpenAPI CI run is required before marking this issue resolved.
- **Prevention:** Pin CI tool dependencies to a validated exact version in a lockfile when possible; otherwise use a valid published dist-tag and ensure the main-branch run executes the same check as the PR.

### TF-023 — Cloud deployment omitted Reports and exposed no public health probes

- **Affected area:** AWS deployment script, API Gateway routes, request proxy, and endpoint smoke workflow.
- **Environment/commit/date:** Code review of `main` deployment state; Windows PowerShell, AWS `eu-west-3`; 2026-11-16.
- **Severity/status:** High; implementation and local checks pass, live AWS retest pending valid AWS credentials.
- **Test type:** Deployment configuration review, gateway route unit test, Terraform validation, PowerShell syntax validation, endpoint smoke design.
- **Symptom:** The local image/deploy script omitted the Reports image and service despite Reports being part of the ten-service Compose stack. The cloud proxy did not rewrite service health probes to root `/health` or `/healthz`, and `/api/v1/{proxy+}` was protected, so unauthenticated health probes would fail or reach nonexistent API routes.
- **Cause:** Reports was integrated after the original cloud lifecycle script; public service health routing was not added alongside the gateway proxy.
- **Fix:** Add Reports to Terraform host environment, the image build list, SSM deploy list, and GitHub rollout; add explicit public API Gateway health routes for all ten APIs; rewrite each public health path to its service's actual local health path; verify host and gateway endpoints and record each result.
- **Retest:** Node request-proxy tests pass (4/4); Terraform validate passes; PowerShell parser passes. Live AWS endpoint retest is blocked by expired AWS profile credentials.
- **Prevention:** Keep the service inventory shared across Compose, Terraform/ECR, local build/deploy, GitHub publishing, gateway routing, and endpoint smoke; run the gateway health matrix whenever adding a service.

### TF-024 — Deleted state bucket prevented a fresh deployment

- **Affected area:** Terraform backend bootstrap and disposable cloud lifecycle.
- **Environment/commit/date:** AWS account `109573999710`, `eu-west-3`; 2026-11-16.
- **Severity/status:** High; secure bucket bootstrap and teardown automation implemented; cloud retest pending valid AWS credentials.
- **Test type:** AWS state-backend inventory and Terraform initialization diagnosis.
- **Symptom:** Terraform initialization failed with `NoSuchBucket` after the state S3 bucket had been deleted; the existing lifecycle script assumed the bucket was already present and did not recreate it.
- **Cause:** The backend is deliberately outside the Terraform stack, but there was no lifecycle code to create or remove it.
- **Fix:** Create the account/region state bucket before Terraform initialization, enable all S3 public access blocks, versioning, and AES-256 encryption; after a successful ephemeral destroy, remove all object versions/delete markers and delete the bucket.
- **Retest:** Bucket security controls were verified after bootstrap. Recovery state was loaded only to inspect the former inventory; Terraform refresh showed AWS had independently removed/replaced resources. Full clean-plan/apply/teardown retest is pending AWS reauthentication.
- **Prevention:** Treat backend setup/cleanup as explicit lifecycle stages and verify the backend bucket exists and is protected before Terraform init.

### TF-025 — ECR image layers blocked the repeatable teardown

- **Affected area:** Terraform ECR repository lifecycle for all API and migration images.
- **Environment/commit/date:** AWS account `109573999710`, `eu-west-3`; ephemeral run on `main` `7ad5c319304c847563d2c00ad170efe85327e7cf`; 2026-11-16.
- **Severity/status:** Medium; fix implemented, live destroy/recreate retest pending.
- **Test type:** Terraform destroy and AWS ECR integration.
- **Symptom:** The ephemeral lifecycle stopped before deployment because Terraform could not destroy four existing ECR repositories containing image tags; AWS returned `RepositoryNotEmptyException`.
- **Cause:** The repository resource did not set `force_delete`, so Terraform required an empty image repository before destroy.
- **Fix:** Set `force_delete = true` on the development ECR repositories so each disposable run can remove image layers and recreate clean repositories.
- **Retest:** Terraform formatting/validation and the next full ephemeral run are pending.
- **Prevention:** Exercise destroy behavior with populated repositories as part of infrastructure lifecycle verification, not only fresh-stack creation.

### TF-026 — Main deployment workflow ran before OIDC bootstrap

- **Affected area:** GitHub `Deploy AWS development stack` workflow.
- **Environment/commit/date:** GitHub Actions on `main` commit `0255ca1c410c714c39e534d26e71976730efd581`; 2026-11-16.
- **Severity/status:** Medium; guard implemented, retest pending.
- **Test type:** GitHub Actions deployment bootstrap diagnosis.
- **Symptom:** The post-merge `workflow_run` job attempted OIDC authentication with no AWS role configured and failed in `configure-aws-credentials` before publishing or deploying images.
- **Cause:** Automatic deployment was eligible even though the `development-publish` environment had not yet been bootstrapped with the Terraform-created role, region, and API base URL.
- **Fix:** Gate the deployment job on all three environment variables. `infra-up` creates the GitHub environment and writes these values after the AWS role and gateway exist.
- **Retest:** The workflow should skip while unconfigured and proceed after the first successful `infra-up`; AWS publishing retest remains pending.
- **Prevention:** Check bootstrap prerequisites in the job condition before requesting OIDC credentials.

### TF-027 — Legacy ECR repositories retained images during teardown

- **Affected area:** `scripts/aws-dev.ps1` ephemeral teardown and Terraform-managed development ECR repositories.
- **Environment/commit/date:** AWS account `109573999710`, `eu-west-3`; legacy repositories restored from existing state; 2026-11-16.
- **Severity/status:** Medium; automated image purge implemented, full lifecycle retest pending.
- **Test type:** AWS ECR integration and Terraform destroy.
- **Symptom:** Terraform's `force_delete = true` did not unblock deletion of repositories created before that setting was added; the legacy state had no force-delete value and AWS rejected deletion while images remained.
- **Cause:** Existing ECR repositories can outlive the Terraform option change and contain tagged or untagged image manifests.
- **Fix:** Before applying the reviewed destroy plan, enumerate image digests in each known development repository and batch-delete them, failing with the repository and ECR failure details if any remain. Fresh repositories continue to use Terraform `force_delete` as a backstop.
- **Retest:** Pending the complete ephemeral create, endpoint-smoke, and destroy cycle.
- **Prevention:** Keep ECR cleanup in the destroy path as well as provider configuration; validate teardown with repositories containing image layers.

### TF-028 — Public health probes duplicated the downstream path

- **Affected area:** `scripts/aws-dev.ps1` public API Gateway endpoint verification.
- **Environment/commit/date:** Deployment script review against the Terraform API Gateway and request-proxy route maps; 2026-11-16.
- **Severity/status:** High for deployment verification; fixed, live endpoint retest pending.
- **Test type:** API route contract and deployment smoke verification.
- **Symptom:** Public health probes constructed URLs such as `/api/v1/branches/health/health`, although API Gateway exposes `/api/v1/branches/health` and the proxy maps that exact path to the host's `/health` endpoint.
- **Cause:** The probe appended the service-local health path to an already complete public gateway route.
- **Fix:** Probe the exact public `Path`; continue using `LocalPath` only for the private host-side check.
- **Retest:** Proxy route unit tests pass; the full public/private endpoint matrix will be recorded during the ephemeral cloud run.
- **Prevention:** Keep public gateway paths and downstream service paths separate in endpoint test fixtures; verify each public URL against both the API Gateway route and proxy rewrite table.
