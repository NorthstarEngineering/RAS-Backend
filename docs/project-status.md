# RAS-Backend project status

Status verified: 2026-10-14. This is a development project; no AWS deployment is confirmed.

## Current state

- GitHub `main` is at `1c3e5c8` (merge of PR #10). PRs #1–#10 are merged, and there are no open pull requests. PR #10 integrated the monorepo and its 155 changed files. See [PR #10](https://github.com/NorthstarEngineering/RAS-Backend/pull/10).
- The repository contains three independent Go modules under `services/`: Auth, Branch, and Attendance. Each has its own `go.mod`, Dockerfile, and application entry point. Attendance contains Employee, Attendance, and Overtime modules.
- Central Goose migrations are under `database/migrations/`; development seed data is separate in `database/seeds/dev.sql`. The database name is `restaurant_db` in the current examples and Compose configuration.
- The mobile handoff exists under `docs/api/` with OpenAPI, Postman, example environments, and deterministic mock responses.
- Terraform's current local draft uses API Gateway, a custom JWT authorizer, a VPC-attached Lambda request proxy, one EC2 Docker host, ECR, SSM Standard parameters, and private RDS. The draft removes NAT Gateway, ALB, VPC Link, Secrets Manager, and CloudWatch log groups to fit the strict Free Tier constraint. Terraform source is not evidence of deployment.

## Completion ledger

| Area | Status | Evidence and remaining work |
|---|---|---|
| Service code and module separation | DONE for current CI scope | Independent modules and image builds are validated by the service workflows on merged `main`. End-to-end behavior against a live database is not verified locally in this session. |
| API contract and mobile handoff | DONE for current documented contract | `docs/api/openapi.yaml`, `mobile-integration.md`, Postman collection, environment example, and mocks exist. The OpenAPI lint check passed on `main`. Mobile-app integration itself is unverified because that repository was not inspected. |
| Central migrations and database checks | DONE for clean-database CI | Goose history, schema checks, and runtime-role privilege checks passed in `Database migration validation` on `main`. An RDS upgrade or restore has not been tested. |
| CI validation | DONE for merged `main` | Three service validation jobs, OpenAPI lint, migration validation, and Terraform/authorizer validation passed on commit `1c3e5c8`. Image publishing jobs were skipped intentionally: workflows require manual `workflow_dispatch` on `main`. No images have been confirmed published. |
| Terraform source validation | DONE for current local draft | Bundled Terraform 1.13.5 `fmt -check -recursive` and `validate` passed. A placeholder-only Free Tier architecture plan produced 65 resources to add, 0 change, 0 destroy after the GitHub OIDC publisher role was added. It is not an apply plan because it uses placeholder secrets and does not validate service eligibility or image sizes. No AWS apply has occurred. |
| Authorizer tests | DONE for unit tests | `node --test infra/terraform/dev/jwt-authorizer/jwt.test.mjs`: 5 passed, 0 failed. This does not establish that API Gateway has been deployed or accepts live tokens. |
| Local Go tests | BLOCKED in this session | `wsl.exe -d Ubuntu go version` returned `Wsl/E_ACCESSDENIED`; Windows Go is not installed. CI service jobs passed, but no local Go tests ran here. |
| Local Docker/PostgreSQL checks | BLOCKED in this session | Docker CLI is present, but `docker info` cannot connect to the Docker Desktop Linux engine. No local Compose or database tests ran. |
| AWS development environment | TODO / not deployed | Rechecked in `eu-west-3`: no EC2 instances, RDS instances, API Gateway APIs, load balancers, NAT gateways, ECR repositories, or Secrets Manager secrets exist. No application resources were created. |
| Terraform remote state | PARTIAL / configured | The dedicated bucket `ras-backend-tfstate-109573999710-eu-west-3` is versioned, encrypted, private, and TLS-only. It contains S3 lockfile versions but no Terraform state object because no apply occurred. Keep the bucket when destroying app resources. |
| Cost requirement | PARTIAL | Terraform draft removes the known recurring paid resources and uses Free Tier eligible service classes/offers with quotas. AWS usage limits and eligibility are account- and time-dependent; image sizes are not verified. Do not apply until those are checked and Docker images are published. |
| AWS Free account plan and cost review | DONE for read-only recheck | Account plan is `FREE`/`ACTIVE`, remaining credits $108.36, expiration 2027-02-02 15:16 UTC. Free Tier usage API reported only Glue and 7 KMS requests. No budget exists. See [deployment-cost-review.md](deployment-cost-review.md). |
| Deployment credential minimization | PARTIAL | `deploy/aws-dev-deploy.sh` was changed locally to include CEO bootstrap values only in the Auth API environment. Bash/jq were unavailable (WSL returned `Bash/E_ACCESSDENIED`; `jq` is not installed), so this edit has not been executed or syntax-validated. |

## Git and local-worktree notes

- The local checkout is `feature/tests-and-ci` at `953343b`, while GitHub `main` is `1c3e5c8`. The GitHub comparison shows the local branch is two commits behind `main`. A local `git fetch origin` could not update refs because writing `.git/FETCH_HEAD` was denied; do not force or reset to compensate.
- The local tree reports modified Go files, but `git diff --ignore-space-at-eol` showed no content changes; they appear to be line-ending/whitespace noise. Preserve them until the checkout can be safely refreshed.
- `RAS-Backend/` is a clean nested clone of the same GitHub repository on its original `main` commit (`20c9311`), containing only a short README. It is not part of the outer repository's tracked tree; leave it untouched.
- Historical feature branches are stale relative to `main`. GitHub reports no open PRs. Do not delete or force-update those branches as part of deployment work.

## Security and deployment risks to resolve

1. The private RDS design and persistent database lifecycle are appropriate goals, but the existing development stack has not been applied or smoke-tested.
2. `infra/terraform/modules/application/main.tf` grants the application host role access to both the shared application secret and the RDS master secret. Review least-privilege access and use service-specific database roles before deployment.
3. `deploy/aws-dev-deploy.sh` now conditionally writes `SEED_CEO_NAME`, `SEED_CEO_EMAIL`, and `SEED_CEO_PASSWORD` only for Auth. This local change still needs shell/jq validation before deployment.
4. Current local Terraform state stores sensitive values in the versioned S3 backend; access must remain least-privilege, and the DPAPI-encrypted laptop backup should be maintained.
5. The strict Free Tier design has reduced observability because it does not provision paid logging resources. API Gateway and Lambda usage offers are quota-limited, and the ECR 500 MB combined image limit must be verified before publishing.
6. CI image publishing is intentionally manual. After AWS prerequisites exist, publishing must be explicitly triggered from `main` with the required GitHub environment variables and OIDC role.

## Next steps

1. Review the revised plan and Free Tier usage immediately before apply. It is temporary; do not upgrade the AWS account.
2. Restore Docker Desktop, build the four images, and verify their combined ECR storage is below 500 MB before publishing.
3. Configure the GitHub `development-publish` environment's OIDC role and ECR repository variables from the Terraform outputs after infrastructure exists.
4. Apply only the reviewed, real-secret plan; run migrations once; deploy images and verify public health/login/protected business routes.
5. Back up Terraform state locally after apply and after destroy, then destroy the disposable stack without deleting the S3 state bucket.

No AWS resources, database state, GitHub branches, or pull requests were modified during this status audit.

## Deployment follow-up (2026-10-14)

- Created only the dedicated S3 state bucket `ras-backend-tfstate-109573999710-eu-west-3` in `eu-west-3`; it has versioning, AES-256 encryption, bucket-owner-enforced ownership, all public-access blocks, a TLS-only policy, and development-purpose tags. No application resources were created.
- Initialized Terraform's S3 backend successfully using Terraform 1.13.5. `terraform validate` and `fmt -check -recursive` passed. The remote bucket contains versioned `.tflock` objects but no Terraform state object because no resource apply occurred.
- Prepared `scripts/backup-terraform-state.ps1`, which writes a DPAPI-encrypted backup outside the repo under the current Windows user's LocalAppData. The backup has not been run; the state is empty and no infrastructure is applied.
- The deployment-cost review is `docs/deployment-cost-review.md`. No Budget or application secrets exist yet. Application deployment remains pending a precise Paris-region cost estimate and approved plan.
- `deploy/aws-dev-deploy.sh` limits CEO bootstrap credentials to Auth, but shell/jq validation remains pending because Bash and jq are unavailable in this session.
