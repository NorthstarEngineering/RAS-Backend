# Development deployment

This Terraform environment is for development only. The AWS checks completed during preparation found no project-tagged VPC, EC2 host, or RDS instance. Nothing has been applied.

## Request and network path

Flutter calls the API Gateway HTTP API over its AWS-managed HTTPS endpoint at `https://<api-id>.execute-api.<region>.amazonaws.com/dev`. A private integration path mapping removes the stage prefix and preserves the existing `/api/v1` backend paths. API Gateway sends traffic through a VPC Link to an internal ALB. ALB path rules route Auth and user administration to port 9001, Branch to 9002, and Employee, Attendance, and Overtime to 9003. `/health` routes to Auth. The EC2 host and RDS are private; the host has no public IP and accepts API traffic only from the ALB security group. RDS accepts PostgreSQL only from the application security group.

The Lambda REQUEST authorizer validates the Auth API's existing HS256 issuer, expiry, subject, role, branch, and first-login claim. It preserves the first-login flag so the Auth API can allow only its existing self/profile and password-change routes; Branch and Attendance middleware continue rejecting first-login tokens and enforce role/branch rules. The authorizer reads a dedicated JWT-only secret from Secrets Manager, caches it in a warm Lambda process for at most 60 seconds, and has API Gateway result caching disabled. Gateway context is not converted into trusted identity headers. Current logout and refresh revoke refresh-token state; already-issued access JWTs remain valid until expiry (currently 15 minutes).

Public gateway routes are login, refresh, logout, and health because that matches the existing handlers. Other current route families require the authorizer. HTTP inside the private VPC is unencrypted between API Gateway's VPC Link and the ALB; the client-facing connection uses API Gateway HTTPS. CORS is omitted by default for native mobile use; configure exact browser origins only when a browser client needs CORS. Access logs are retained for 14 days and exclude credentials and request bodies. Gateway throttling starts at 25 requests/second, burst 50.

Private subnets use one NAT Gateway for EC2 image/package pulls, Secrets Manager, and SSM traffic. It avoids public backend instances but adds continuous hourly/data-processing costs and is a single-AZ egress dependency. The design has one EC2 host, one ALB, one NAT Gateway, and single-AZ RDS; it has single points of failure and no host-level scaling or HA guarantee. API Gateway, VPC Link, ALB, NAT Gateway, EC2, RDS, CloudWatch Logs, ECR, and backups have ongoing or usage-based costs. Check current prices for the selected region before approval.

## Persistent database

RDS PostgreSQL is private, encrypted, uses a managed master secret, retains seven days of automated backups, and has deletion protection plus Terraform `prevent_destroy`. The RDS database and backups must outlive temporary API compute. Routine application cleanup must retain RDS, backups, state, and Secrets Manager values. Database deletion requires a separate reviewed procedure that verifies resource identity and restorable backups first. Replaying Goose migrations restores schema, not historical business data.

The three API pools are limited to five open/two idle connections each. PostgreSQL runtime roles are capped at five sessions apiece, limiting normal API use to at most 15 application sessions plus the migration/admin connection. The small development RDS instance may queue requests when those connections are busy; change limits only after checking `max_connections`, reserved capacity, and load.

## Prerequisites

1. Approved AWS identity/profile and region. The local `terraform` CLI profile successfully returned an AWS account identity. The project region is `eu-west-3`.
2. A remote state bucket in a separate bootstrap step with versioning enabled, default encryption, and every S3 public-access-block setting enabled. Terraform uses native S3 lockfiles (Terraform >=1.10). The owner-provided CLI output for candidate bucket `takwa231018545` shows versioning disabled (empty versioning response), all four public-access-block settings `false`, and default SSE-S3/AES256 encryption enabled. Do not use it for Terraform state until versioning and all public-access-block settings are enabled and reverified. No bucket settings were changed.
3. A Secrets Manager app secret JSON containing `JWT_SECRET`, `AUTH_DB_PASSWORD`, `BRANCH_DB_PASSWORD`, `ATTENDANCE_DB_PASSWORD`, `SEED_CEO_NAME`, `SEED_CEO_EMAIL`, and `SEED_CEO_PASSWORD`, plus a dedicated JWT-only secret containing the raw signing key. The Lambda authorizer role can read only the JWT-only secret. The read-only check found no matching app secret. Generate separate values and store them only in Secrets Manager.
4. GitHub OIDC role, protected `development-publish` environment, and ECR repository URI variables before image publishing. These are not assumed to exist.

Copy `terraform.tfvars.example` to ignored `terraform.tfvars`, set the region and actual app-secret ARN, and do not commit that file. Example commands:

```powershell
Set-Location infra/terraform/dev
terraform init -backend-config="bucket=SECURE_STATE_BUCKET" -backend-config="region=STATE_BUCKET_REGION"
terraform fmt -check -recursive
terraform validate
terraform plan -var-file=terraform.tfvars
```

Review the AWS identity, region, state key, all subnets and security groups, RDS retention, cost drivers, and every resource in the complete plan. Do not apply until that plan is reviewed and explicitly approved. RDS deletion protection and `prevent_destroy` block ordinary teardown. Never remove those protections to clean up an app host.

## Publish and deploy an individual API

Merge the reviewed change to protected `main`, then dispatch only the affected API's workflow. CI completes before the protected `development-publish` environment permits ECR publishing. Configure `AWS_REGION`, `AWS_ROLE_TO_ASSUME`, and the corresponding immutable ECR repository URI (`ECR_AUTH_REPOSITORY`, `ECR_BRANCH_REPOSITORY`, or `ECR_ATTENDANCE_REPOSITORY`). Scope the OIDC role trust to this repository and environment and ECR permissions to the required repository. Images are tagged with the commit SHA.

Database changes use `.github/workflows/database.yml` to validate the complete Goose history on disposable PostgreSQL and publish the migration runner by SHA after protected manual dispatch. The deployment operator should confirm the migration runner SHA, run central migrations once, then deploy only the affected API image SHA through `/usr/local/bin/ras-backend-deploy.sh` over SSM. Verify that API's target health and the public Gateway health route before declaring success. Roll forward on application failures; never automatically roll back the shared schema.

The deploy command retrieves runtime values from Secrets Manager under the EC2 instance role. Never paste secret values into shell history, logs, Terraform variables, or GitHub Actions output. Do not publish from pull requests or deploy to production.
