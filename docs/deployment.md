# Development deployment

The development stack was deployed in `eu-west-3` on 2026-10-14 for mobile integration. The API Gateway URL is `https://jo3c9na8mg.execute-api.eu-west-3.amazonaws.com/dev`; obtain the current value with `terraform output -raw api_gateway_base_url`. The endpoint is temporary and remains available until the explicit teardown command is run. Eligible usage may consume the account's Free Plan credits.

## Request path

Flutter uses the API Gateway HTTP API HTTPS endpoint. Protected routes run the existing HS256 JWT check in a Lambda REQUEST authorizer. The request proxy Lambda forwards requests inside the VPC to the existing Go services running as Docker containers on one EC2 host. A proxy security group is the only permitted source to the host's API ports. The RDS PostgreSQL database is private and accepts traffic only from the EC2 application security group. No API Gateway VPC Link, load balancer, or NAT Gateway is used.

The EC2 host receives an automatically assigned public IPv4 address for outbound package/ECR access and SSM management. It has no internet-facing application ingress rule. The one host is a single point of failure and does not scale independently. Lambda has no reserved concurrency because AWS requires at least 10 executions to remain unreserved in this account. There are no CloudWatch log groups in this Free Plan configuration; inspect API responses and EC2 container logs through SSM, and avoid printing credentials or tokens.

## Eligible usage and limits

The account last reported Free plan `ACTIVE`, with $108.36 remaining and expiration `2027-02-02`. Recheck both Free Tier usage and eligibility before every apply. Relevant published offers include EC2 `t3.micro` and public IPv4 allowance, RDS PostgreSQL `db.t4g.micro` on the Free plan, API Gateway's monthly HTTP API request offer, Lambda's monthly request/compute allowance, ECR private storage up to 500 MB/month for eligible new customers, and no-additional-charge SSM Parameter Store Standard parameters. These offers have eligibility periods and usage limits; this is not a monthly credit budget. Keep combined ECR image storage under 500 MB, protect API/Lambda usage limits, and do not upgrade the account. [AWS Free account plan](https://docs.aws.amazon.com/awsaccountbilling/latest/aboutv2/free-tier.html), [RDS Free Tier](https://aws.amazon.com/rds/free/), [API Gateway pricing](https://aws.amazon.com/api-gateway/pricing/), [Lambda pricing](https://aws.amazon.com/lambda/pricing/), [ECR pricing](https://aws.amazon.com/ecr/pricing/), [Parameter Store pricing](https://aws.amazon.com/systems-manager/pricing/).

The stack intentionally has no NAT Gateway, ALB, VPC Link, Secrets Manager, custom KMS key, CloudWatch log group, or RDS backup/snapshot. Do not add a resource unless its account-specific Free Tier eligibility and usage allowance are verified first. AWS Free plan access ends when its term expires or credits are exhausted; do not rely on this as permanent hosting.

## Credentials and data

Use Standard SecureString parameters in Parameter Store with the AWS-managed SSM key. Terraform state also contains sensitive values because the RDS password is managed by Terraform. The remote S3 state bucket is versioned, encrypted, private, and must remain after the application stack is destroyed. Maintain the DPAPI-encrypted laptop state backup; never commit state, plans, or real `terraform.tfvars`.

RDS is single-AZ, private, `db.t4g.micro`, 20 GiB gp2, and has automated backups disabled for this disposable test. Destroying the application stack deletes `restaurant_db` permanently with no final snapshot. Run no production or personal data in it. Goose migrations are applied once through the migration-runner image before the APIs are started.

## Prepare and plan

From PowerShell:

```powershell
Set-Location C:\Users\myfin\OneDrive\Desktop\RAS-backend
Copy-Item infra\terraform\dev\terraform.tfvars.example infra\terraform\dev\terraform.tfvars
# Replace placeholders in the ignored tfvars with unique development-only values.
& .\.tools\terraform\terraform.exe -chdir=infra/terraform/dev init -backend-config="bucket=ras-backend-tfstate-109573999710-eu-west-3" -backend-config="region=eu-west-3" -backend-config="profile=terraform"
& .\.tools\terraform\terraform.exe fmt -check -recursive
& .\.tools\terraform\terraform.exe -chdir=infra/terraform/dev validate
& .\.tools\terraform\terraform.exe -chdir=infra/terraform/dev plan -var-file=terraform.tfvars
```

The example file contains placeholders and must not be applied. Before apply, confirm the current account identity and Free plan, inspect the complete plan for unexpected resource types, and ensure the ECR images are within their combined storage offer. A plan is not proof that resources can be deployed or that all features remain eligible.

## Publish and deploy

The four immutable images tagged `1c3e5c8ad19c5add7bd25f617f0796dc82c9e0aa` were published to ECR. Their combined repository storage is subject to the account's Free Plan allowance. GitHub image publishing remains separate and requires the documented OIDC role and repository variables.

The deployment script builds the migration connection string from the URL-safe RDS credentials retrieved from SSM. The EC2 bootstrap installs the PostgreSQL client required by the deployment tooling, and temporary migration environment files are removed by an exit trap. If ECR repositories were created outside the current Terraform state, import each existing repository before applying so Terraform does not attempt to recreate it.

The deployment and smoke checks completed:

1. The single EC2 host is accessible through Systems Manager.
2. Five Goose migrations were applied through version `20261013100000`.
3. Auth, Branch, and Attendance were started from immutable ECR images.
4. `GET /health`, login, initial password change, and authenticated `GET /api/v1/branches` returned HTTP 200.
5. Unauthenticated requests to `/api/v1/auth/me`, `/api/v1/branches`, `/api/v1/employees`, `/api/v1/attendance`, and `/api/v1/overtime` returned HTTP 401.

Do not send real passwords, tokens, or database values to chat or shell output.

## Destroy the disposable application environment

After mobile integration testing, run `.\scripts\aws-dev.ps1 down`. It creates an encrypted laptop state backup, shows the exact destroy plan, and requires typing `DELETE RAS BACKEND DEV DATA`. It removes EC2, RDS, ECR repositories/images, Lambda functions, API Gateway, SSM parameters, networking, and IAM resources. **RDS data is permanently deleted** because this temporary stack has no backup retention or final snapshot. The versioned S3 state bucket and state history are outside the application stack and remain.

To remove the disposable application resources while preserving the RDS
instance and all database rows, run
`.\scripts\aws-dev.ps1 down-preserve-data`. It targets every managed resource
except `module.database` and `module.network`, displays the exact plan, and
requires `DELETE RAS BACKEND DEV APPLICATION`. This preserves the database,
its required subnet/security-group resources, VPC networking, and the remote
Terraform state bucket while removing EC2, ECR, API Gateway, Lambda, SSM, and
application IAM resources. Re-run `.\scripts\aws-dev.ps1 up` to recreate the
application layer against the retained database.

To replace only the temporary shell and retain the RDS database, use
`.\scripts\aws-dev.ps1 shell-down` followed by `.\scripts\aws-dev.ps1 up`.
The new EC2 bootstrap installs all runtime and database-query tools before the
deployment command reports the host ready.
