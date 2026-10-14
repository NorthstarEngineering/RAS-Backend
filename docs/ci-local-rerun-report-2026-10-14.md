# CI run audit and local rerun report

**Checked:** 2026-10-14  
**Repository:** `NorthstarEngineering/RAS-Backend`  
**Local checkout:** `C:\Users\myfin\OneDrive\Desktop\RAS-backend`, branch `feature/tests-and-ci`, HEAD `8851e72`  
**Scope:** The 30 most recent GitHub Actions workflow runs returned by the repository API, plus checks attempted locally today. No new GitHub workflow was started. No push, commit, Terraform apply/destroy, or database reset was performed.

## Summary

- GitHub Actions history: **30 runs, 22 failed, 8 passed**.
- The 18 failing service CI runs all failed at `Check formatting`. The exact Go filenames are absent from those historical logs because the workflow used `test -z "$(gofmt -l ...)"`, which discarded the formatter output.
- Four earlier database runs failed while Goose applied a PL/pgSQL `DO $$ ... $$` block. PostgreSQL reported `unterminated dollar-quoted string` (`SQLSTATE 42601`): Goose had split the block at embedded semicolons. Goose statement markers were added in commit `8851e72`; the two latest fresh-database runs then passed migration, schema/constraint, runtime-role, privilege, and Goose-state verification.
- On service runs that failed at formatting, **Vet, Unit tests, Build, and Build container image were skipped**. This means those steps were not tested by those runs, not that they passed.
- Pull-request publishing jobs were skipped by their configured condition: publish only on a manually dispatched run against `main`. Those are intentional release gates.
- Current local Go and PostgreSQL checks could not execute because Windows has no Go executable, Ubuntu WSL returns `Wsl/E_ACCESSDENIED`, and the Docker Linux engine is not running. Docker Desktop was launched once from this session, but the engine pipe remained unavailable.

## Every workflow run in the 30-run Actions history

“Skipped” lists skipped steps/jobs reported by GitHub. Post-action cleanup steps marked skipped are included when GitHub reports them; they are not application test cases. Run IDs link to the GitHub run page.

| Run ID | Workflow | Branch / commit | Overall | Failed step | Skipped checks/jobs |
|---:|---|---|---|---|---|
| 38526734518 | Database migration validation | feature/tests-and-ci @ `8851e72` | success | — | publish-migration-runner job |
| 38526734425 | branch-api CI and image publishing | feature/tests-and-ci @ `8851e72` | failure | validate: Check formatting | validate/Vet, validate/Unit tests, validate/Build, validate/Build container image, validate/Post Run actions/setup-go@v5, publish job |
| 38526734372 | Terraform and authorizer validation | feature/tests-and-ci @ `8851e72` | success | — | none |
| 38526734358 | auth-api CI and image publishing | feature/tests-and-ci @ `8851e72` | failure | validate: Check formatting | validate/Vet, validate/Unit tests, validate/Build, validate/Build container image, validate/Post Run actions/setup-go@v5, publish job |
| 38526734336 | attendance-api CI and image publishing | feature/tests-and-ci @ `8851e72` | failure | validate: Check formatting | validate/Vet, validate/Unit tests, validate/Build, validate/Build container image, validate/Post Run actions/setup-go@v5, publish job |
| 38526734304 | API contract validation | feature/tests-and-ci @ `8851e72` | success | — | none |
| 38526733415 | branch-api CI and image publishing | feature/tests-and-ci @ `8851e72` | failure | validate: Check formatting | validate/Vet, validate/Unit tests, validate/Build, validate/Build container image, validate/Post Run actions/setup-go@v5, publish job |
| 38526733403 | API contract validation | feature/tests-and-ci @ `8851e72` | success | — | none |
| 38526733371 | attendance-api CI and image publishing | feature/tests-and-ci @ `8851e72` | failure | validate: Check formatting | validate/Vet, validate/Unit tests, validate/Build, validate/Build container image, validate/Post Run actions/setup-go@v5, publish job |
| 38526733363 | Database migration validation | feature/tests-and-ci @ `8851e72` | success | — | publish-migration-runner job |
| 38526733341 | Terraform and authorizer validation | feature/tests-and-ci @ `8851e72` | success | — | none |
| 38526733331 | auth-api CI and image publishing | feature/tests-and-ci @ `8851e72` | failure | validate: Check formatting | validate/Vet, validate/Unit tests, validate/Build, validate/Build container image, validate/Post Run actions/setup-go@v5, publish job |
| 38510615717 | attendance-api CI and image publishing | qa/attendance-api-tests @ `f301ac4` | failure | validate: Check formatting | validate/Vet, validate/Unit tests, validate/Build, validate/Build container image, validate/Post Run actions/setup-go@v5, publish job |
| 38510615701 | auth-api CI and image publishing | qa/attendance-api-tests @ `f301ac4` | failure | validate: Check formatting | validate/Vet, validate/Unit tests, validate/Build, validate/Build container image, validate/Post Run actions/setup-go@v5, publish job |
| 38510615634 | Database migration validation | qa/attendance-api-tests @ `f301ac4` | failure | migrate-fresh-and-verify: Apply complete migration history to fresh database | migrate-fresh-and-verify/Verify repository tables and constraints with disposable inserts, migrate-fresh-and-verify/Verify runtime roles and table privileges, migrate-fresh-and-verify/Verify Goose migration state, migrate-fresh-and-verify/Post Run actions/setup-go@v5, publish-migration-runner job |
| 38510615621 | branch-api CI and image publishing | qa/attendance-api-tests @ `f301ac4` | failure | validate: Check formatting | validate/Vet, validate/Unit tests, validate/Build, validate/Build container image, validate/Post Run actions/setup-go@v5, publish job |
| 38508145499 | Database migration validation | qa/attendance-api-tests @ `86ef4f3` | failure | migrate-fresh-and-verify: Apply complete migration history to fresh database | migrate-fresh-and-verify/Verify repository tables and constraints with disposable inserts, migrate-fresh-and-verify/Verify runtime roles and table privileges, migrate-fresh-and-verify/Verify Goose migration state, migrate-fresh-and-verify/Post Run actions/setup-go@v5, publish-migration-runner job |
| 38508145496 | auth-api CI and image publishing | qa/attendance-api-tests @ `86ef4f3` | failure | validate: Check formatting | validate/Vet, validate/Unit tests, validate/Build, validate/Build container image, validate/Post Run actions/setup-go@v5, publish job |
| 38508145429 | attendance-api CI and image publishing | qa/attendance-api-tests @ `86ef4f3` | failure | validate: Check formatting | validate/Vet, validate/Unit tests, validate/Build, validate/Build container image, validate/Post Run actions/setup-go@v5, publish job |
| 38503787905 | Database migration validation | qa/attendance-api-tests @ `d98630e` | failure | migrate-fresh-and-verify: Apply complete migration history to fresh database | migrate-fresh-and-verify/Verify repository tables and constraints with disposable inserts, migrate-fresh-and-verify/Verify runtime roles and table privileges, migrate-fresh-and-verify/Verify Goose migration state, migrate-fresh-and-verify/Post Run actions/setup-go@v5, publish-migration-runner job |
| 38503787890 | attendance-api CI and image publishing | qa/attendance-api-tests @ `d98630e` | failure | validate: Check formatting | validate/Vet, validate/Unit tests, validate/Build, validate/Build container image, validate/Post Run actions/setup-go@v5, publish job |
| 38438811143 | attendance-api CI and image publishing | feature/backend-audit-docs @ `8d60982` | failure | validate: Check formatting | validate/Vet, validate/Unit tests, validate/Build, validate/Build container image, validate/Post Run actions/setup-go@v5, publish job |
| 38438811080 | branch-api CI and image publishing | feature/backend-audit-docs @ `8d60982` | failure | validate: Check formatting | validate/Vet, validate/Unit tests, validate/Build, validate/Build container image, validate/Post Run actions/setup-go@v5, publish job |
| 38438811078 | auth-api CI and image publishing | feature/backend-audit-docs @ `8d60982` | failure | validate: Check formatting | validate/Vet, validate/Unit tests, validate/Build, validate/Build container image, validate/Post Run actions/setup-go@v5, publish job |
| 38438792967 | attendance-api CI and image publishing | feature/tests-and-ci @ `be3a7b1` | failure | validate: Check formatting | validate/Vet, validate/Unit tests, validate/Build, validate/Build container image, validate/Post Run actions/setup-go@v5, publish job |
| 38438792877 | auth-api CI and image publishing | feature/tests-and-ci @ `be3a7b1` | failure | validate: Check formatting | validate/Vet, validate/Unit tests, validate/Build, validate/Build container image, validate/Post Run actions/setup-go@v5, publish job |
| 38438792781 | Database migration validation | feature/tests-and-ci @ `be3a7b1` | failure | migrate-fresh-and-verify: Apply complete migration history to fresh database | migrate-fresh-and-verify/Verify repository tables and constraints with disposable inserts, migrate-fresh-and-verify/Verify runtime roles and table privileges, migrate-fresh-and-verify/Verify Goose migration state, migrate-fresh-and-verify/Post Run actions/setup-go@v5, publish-migration-runner job |
| 38438792752 | branch-api CI and image publishing | feature/tests-and-ci @ `be3a7b1` | failure | validate: Check formatting | validate/Vet, validate/Unit tests, validate/Build, validate/Build container image, validate/Post Run actions/setup-go@v5, publish job |
| 38438792677 | Terraform and authorizer validation | feature/tests-and-ci @ `be3a7b1` | success | — | none |
| 38438792673 | API contract validation | feature/tests-and-ci @ `be3a7b1` | success | — | none |

## Failure detail and root cause

### Go service formatting — 18 failed runs

For Auth, Branch, and Attendance, the failing command in Actions was:

```sh
test -z "$(gofmt -l $(find services/<service> -name '*.go'))"
```

The command exits 1 when `gofmt -l` finds unformatted files. Because its output is captured inside command substitution and never printed, the logs identify the service but **do not identify the files**. That limitation is why filenames cannot responsibly be listed from these historical runs.

For each affected service run, the skipped steps were:

- Vet
- Unit tests
- Build
- Build container image

The workflow currently present in the local checkout has been edited to print the filenames and to let these checks execute even if formatting fails, then fail the validation job if any check failed. This is a diagnostic workflow improvement; it does not itself format any Go source. It is local and uncommitted.

### Goose migration parsing — 4 failed runs

The database job failed at `Apply complete migration history to fresh database`. The log reported:

```text
failed to run SQL migration: failed to execute SQL query "DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'branch_app') THEN
        EXECUTE 'GRANT SELECT, INSERT, UPDATE, DELETE ON branches TO branch_app';
": ERROR: unterminated dollar-quoted string ... (SQLSTATE 42601)
```

Goose parsed a PL/pgSQL block as multiple statements because the block contains semicolons. The four affected migrations were marked using Goose's statement-boundary directives in `8851e72`. The two latest Actions migration jobs succeeded through all of their verification steps. The local database job was not rerun because Docker is unavailable here.

### Intentionally skipped publishing

Each PR service workflow has a `publish` job conditioned on both a manual `workflow_dispatch` event and `refs/heads/main`. The migration-runner publishing job is also not a PR validation job. Its PR skip is expected and protects publishing from pull-request runs; it is not a missing test.

## Local checks attempted on 2026-10-14

| Check | Command / action | Result |
|---|---|---|
| Go executable | `go version` in PowerShell | **Blocked:** `go` is not recognized; no Windows Go executable is on PATH. |
| WSL Go executable | `wsl.exe -d Ubuntu /usr/bin/go version` | **Blocked:** `Wsl/E_ACCESSDENIED`. |
| Docker daemon | `docker info --format 
'{{.ServerVersion}}'` | **Blocked:** Docker Desktop Linux engine pipe `//./pipe/dockerDesktopLinuxEngine` does not exist. Starting Docker Desktop from this session did not bring the engine online. |
| Go formatting / vet / tests / builds | Per-service Go commands | **Not run:** blocked by unavailable Go runtime. |
| Local database migration/integration | Docker Compose/PostgreSQL | **Not run:** blocked by unavailable Docker engine. No database was reset or altered. |
| Docker Compose syntax and interpolation | Set dummy process-only values, then `docker compose -f deploy/docker-compose.yml config --quiet` | **Passed** (exit 0). Dummy values were not written to a file. |
| JWT authorizer unit tests | `node --test infra/terraform/dev/jwt-authorizer/jwt.test.mjs` | **Passed:** 5 passed, 0 failed, 0 skipped. |
| Terraform formatting | `.tools\terraform\terraform.exe fmt -check -recursive infra\terraform` | **Passed** (exit 0). |
| Terraform validation | `.tools\terraform\terraform.exe -chdir=infra/terraform/dev validate` | **Blocked by provider processes:** could not load schemas; HashiCorp AWS and archive providers returned “Plugin did not respond.” This is not a successful validate result. |
| Workflow YAML parse | PyYAML safe-load of `.github/workflows/*.yml` | **Passed:** 6 workflow files parsed. Python emitted a launcher warning about `C:\Python314\python.exe`, but the parser completed successfully. |
| Patch whitespace check | `git diff --check` | **Passed** (exit 0; Git printed line-ending conversion warnings). |
| OpenAPI lint | No compatible local validator was available in the checkout; no package install/network fetch was attempted. | **Not rerun locally.** The latest GitHub API contract run passed. |

## Local working tree

The following tracked files have local edits that have **not** been committed or pushed:

- `.github/workflows/auth-api.yml`
- `.github/workflows/branch-api.yml`
- `.github/workflows/attendance-api.yml`

They make the formatter print filenames, run vet/tests/build/image build even after a previous check fails, and summarize failures at the end. The pre-existing untracked `RAS-Backend/` directory was not modified or staged.

The report itself is a new untracked file. No other Git changes were made during this rerun.

## How to complete the blocked local checks

1. Start Docker Desktop and wait until `docker info` succeeds; alternatively restore Ubuntu WSL access.
2. Make Go available to that environment.
3. In each module, run:
   ```sh
   cd services/auth-api && gofmt -w $(find . -name '*.go') && go vet ./... && go test ./... && go build ./cmd/api
   cd ../branch-api && gofmt -w $(find . -name '*.go') && go vet ./... && go test ./... && go build ./cmd/api
   cd ../attendance-api && gofmt -w $(find . -name '*.go') && go vet ./... && go test ./... && go build ./cmd/api
   ```
   Review the resulting source diff before keeping formatting changes.
4. Copy `.env.example` to the local ignored `.env` and provide development-only values yourself. Do not place secrets in this report or commit the local file.
5. Run the disposable local checks using `docker compose -f deploy/docker-compose.yml up --build -d`, inspect health and logs, then run the documented integration checks. Normal `docker compose down` retains the named database volume. Do **not** use `down -v` unless you intentionally want to delete disposable local DB data.
6. Run Terraform validation again after repairing/reinstalling the local provider plugins; use only `validate`/plan checks and do not apply or destroy infrastructure.
7. After local fixes are reviewed, push from your normal shell when you choose. That will trigger CI; no Actions run was started by this audit.

## Verification limits

This report distinguishes the historical GitHub outcomes from checks run locally on the current checkout. Historical green results do not prove the current uncommitted workflow edits or Go source pass. The source Go formatting issue remains uncorrected and unverified locally because this machine session cannot access Go. No claim is made that the whole CI suite passes.

## Terraform profile and plan follow-up (2026-10-14)

The configured AWS profile was verified with:

```powershell
aws sts get-caller-identity --profile terraform
```

STS succeeded, confirming the profile credentials and AWS identity are usable. The profile's configured region is `eu-west-3`. No AWS resources were changed.

Terraform initialization was retried with:

```powershell
.tools\terraform\terraform.exe -chdir=infra/terraform/dev init -backend=false
```

It failed because this environment could not resolve `registry.terraform.io` (`lookup registry.terraform.io: no such host`), so it could not query/download the AWS and archive provider packages. Consequently, `terraform validate` still cannot load provider schemas, and a plan cannot be generated here. This is now isolated to Terraform Registry network/DNS/provider initialization, not AWS profile credentials.

There are also prerequisites for a real plan that are intentionally not fabricated: `terraform.tfvars` does not exist; the required application and JWT Secrets Manager ARNs are placeholders; and the S3 remote-state bucket must be bootstrapped and passed to `terraform init` as documented in `infra/terraform/dev/README.md`. At the time of this follow-up, the project owner confirmed `eu-west-3` as the project region; the Terraform example was updated accordingly. No plan, apply, or destroy was run.

The follow-up attempted read-only discovery of S3 bucket names and Secrets Manager secret names in `eu-west-3`. Both AWS CLI calls failed before returning data because the session could not connect to `s3.eu-west-3.amazonaws.com` or `secretsmanager.eu-west-3.amazonaws.com`. Registry DNS resolution also returned `Access is denied`. Thus, neither the state bucket nor the required secret ARNs could be verified. The successful STS call only verifies credential identity against STS; it does not establish network access or authorization to the S3, Secrets Manager, or Terraform Registry endpoints.

The project owner then confirmed that `eu-west-3` is the required region and provided local AWS CLI output showing a candidate S3 bucket named `takwa231018545` and no listed Secrets Manager secret names. The Terraform example now uses `eu-west-3`, including its Secrets Manager ARN placeholders. The existing deployment documentation records that the candidate bucket had versioning and public-access-block settings disabled in an earlier audit, so it remains unsafe for state until those settings are rechecked and corrected. The required app/JWT secrets are still absent according to the provided output. These are records of user-provided CLI output, not fresh reads performed by this session. No AWS settings were modified.

The owner later supplied the bucket configuration commands' output: `get-bucket-versioning` returned an empty response (versioning not enabled); `get-public-access-block` returned `false` for `BlockPublicAcls`, `IgnorePublicAcls`, `BlockPublicPolicy`, and `RestrictPublicBuckets`; `get-bucket-encryption` confirmed default SSE-S3/AES256 encryption. This independently confirms the candidate bucket is not ready for Terraform state. Enable versioning and all public-access-block controls, then recheck them before considering it. This session did not change the bucket.

The project owner requested a local Terraform profile file. The provider now accepts optional `aws_profile`; the ignored local `infra/terraform/dev/terraform.tfvars` contains only `aws_region = "eu-west-3"` and `aws_profile = "terraform"`. `.gitignore` already ignores all `*.tfvars` files, confirmed with `git check-ignore`. The tracked example and Terraform README document the local profile and the OIDC-compatible CI behavior. Required secret ARNs remain unset, and no AWS resources were changed.

## Retest after adding the local profile (2026-10-14)

| Check | Result |
|---|---|
| Go executable (`go version`) | **Blocked:** PowerShell reports `go` is not recognized. |
| WSL Go (`wsl.exe -d Ubuntu /usr/bin/go version`) | **Blocked:** `Wsl/E_ACCESSDENIED`. |
| Docker daemon (`docker info`) | **Blocked:** Linux engine named pipe is missing. |
| JWT authorizer tests (`node --test infra/terraform/dev/jwt-authorizer/jwt.test.mjs`) | **Passed:** 5 passed, 0 failed, 0 skipped. |
| Terraform formatting (`terraform fmt -check -recursive infra/terraform`) | **Passed.** |
| Terraform validation (`terraform -chdir=infra/terraform/dev validate`) | **Failed to initialize provider schemas:** AWS and archive plugins did not respond. |
| Workflow YAML parse | **Passed:** all 6 workflow YAML files parsed. Python printed a launcher warning but returned success. |
| Docker Compose config (`docker compose -f deploy/docker-compose.yml config --quiet`) | **Passed:** configuration rendered/validated; no services were started. |
| `git diff --check` | **Passed:** only Git line-ending warnings were printed. |
| STS identity (`aws sts get-caller-identity --profile terraform`) | **Blocked in this rerun:** could not connect to `https://sts.eu-west-3.amazonaws.com/`. The owner-provided terminal output had previously verified the profile. |

The Go unit tests, vet, independent builds, Docker image builds, and PostgreSQL migration/integration tests remain unrun locally due to Go/WSL and Docker availability. No new GitHub Actions run was triggered.

## Three latest service CI failures and local fix (2026-10-14)

Inspected the user's three latest failed PR #7 runs:

| Run | CI result / exact issue |
|---|---|
| [Auth run 38534752468](https://github.com/NorthstarEngineering/RAS-Backend/actions/runs/38534752468) | `Check formatting` failed for `services/auth-api/cmd/api/main.go`, `services/auth-api/internal/auth/service/service_test.go`, and `services/auth-api/internal/database/postgres.go`. Vet, unit tests, binary build, and Docker image build passed. The aggregate validation step failed only because formatting was still marked failed. |
| [Branch run 38534752457](https://github.com/NorthstarEngineering/RAS-Backend/actions/runs/38534752457) | Formatting failed for `services/branch-api/cmd/api/main.go`, `services/branch-api/internal/branch/model.go`, `interfaces.go`, `handler.go`, `dto.go`, and `services/branch-api/internal/authn/middleware.go` plus `middleware_test.go`. Vet, tests, binary build, and Docker image build also failed because `cmd/api/main.go:36:30` referenced undefined `envInt`. |
| [Attendance run 38534752435](https://github.com/NorthstarEngineering/RAS-Backend/actions/runs/38534752435) | `Check formatting` failed for `services/attendance-api/cmd/api/main.go`, `services/attendance-api/internal/overtime/handler.go`, `services/attendance-api/internal/authn/middleware.go`, and `middleware_test.go`. Vet, unit tests, binary build, and Docker image build passed. The aggregate validation step failed only because formatting was still marked failed. |

Local changes: formatted the 14 files listed above using Go 1.27.2 `gofmt`; added an `envInt` helper to `services/branch-api/cmd/api/main.go` that parses positive integers and falls back to the configured default for empty, invalid, or nonpositive values. No business routes or API contracts were changed.

Local results after the fix:

| Service | gofmt check | `go vet ./...` | `go test ./...` | `go build ./cmd/api` |
|---|---|---|---|---|
| Auth API | Passed | Passed | Passed (`internal/auth/service`; other packages have no tests) | Passed |
| Branch API | Passed | Passed | Passed (`internal/authn`; other packages have no tests) | Passed |
| Attendance API | Passed | Passed | Passed (`internal/attendance`, `internal/authn`; other packages have no tests) | Passed |

Local Docker image build attempts could not reach BuildKit: each failed opening `C:\Users\myfin\.docker\buildx\.lock` with `Access is denied`. Earlier `docker info` also showed the Docker Desktop Linux engine pipe missing. This is a local environment limitation; it is not a successful local image-build result. In the GitHub runs before the fix, Auth and Attendance image builds passed, while Branch's image build failed from the same `undefined: envInt` compilation error. No new GitHub Actions run was triggered, and no commit or push was made.

