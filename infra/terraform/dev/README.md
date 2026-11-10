# Free Tier development Terraform

This disposable development stack is designed around AWS services that have Free Plan offers for the account. It was deployed in `eu-west-3` on 2026-10-14 for mobile integration testing. The current API Gateway URL is available from `terraform output -raw api_gateway_base_url`; it is temporary and may be removed with the `down` command below. Free Plan limits are usage limits, not a promise of indefinite hosting, and eligible usage can consume account credits.

## Request and network path

Flutter → API Gateway HTTP API → Lambda HS256 authorizer (protected routes) → VPC-attached Lambda request proxy → private IP of one EC2 Docker host → private RDS PostgreSQL. The proxy Lambda can connect only to EC2 ports 9001–9003. The EC2 host has an automatically assigned public IPv4 only for outbound package/ECR pulls and SSM management; its security group allows application ports only from the proxy Lambda security group, with no public inbound rule. RDS is in private subnets and accepts PostgreSQL only from the application host security group.

The design removes NAT Gateway, ALB, API Gateway VPC Link, Secrets Manager, custom KMS keys, CloudWatch log groups, and RDS snapshots/backups. Lambda and API Gateway use their published request allowances. ECR keeps one image per repository and the **combined** private image storage must stay below 500 MB. Standard Parameter Store SecureString is used with the AWS-managed SSM key. Do not change parameters to Advanced tier or add paid VPC endpoints, NAT, load balancers, custom domains, WAF, or log ingestion under this Free Tier-only deployment.

## Free Tier guardrails

- EC2: `t3.micro`; use no more than the account's eligible monthly instance and public IPv4 hours. Root gp3 disk is 8 GiB; EC2 T3 CPU credits are Standard to prevent paid surplus credits.
- RDS PostgreSQL: single-AZ `db.t4g.micro`, 20 GiB gp2, private, automated backups disabled for this disposable test, no final snapshot. **Destroying the stack permanently deletes its development data.**
- API Gateway HTTP API: keep below the account's one-million-call monthly offer while eligible.
- Lambda: authorizer plus proxy share the request and compute allowance. Each protected request invokes both; no reserved concurrency is configured because this account requires at least 10 unreserved executions.
- ECR private repositories: retain one image per repo and verify total stored image size is below 500 MB before publishing.
- SSM Parameter Store: Standard SecureString only; no Advanced parameters or customer-managed KMS keys.
- Terraform state: use the secured, versioned S3 bucket `ras-backend-tfstate-109573999710-eu-west-3`; keep it separate and do not destroy it with this stack.

The active account plan is finite and currently expires on 2027-02-02; the last verified balance was $108.36. Credits are not a monthly allowance. Do not upgrade the account. Free Tier offers can expire or be exhausted, and AWS's Free plan ends when its term ends or credits are exhausted. Recheck the account plan and Free Tier usage before use. Never apply if the account is no longer on its Free plan or the selected resource/usage is not shown eligible for this account.

## Local setup and validation

Keep real secrets only in ignored `terraform.tfvars`. Copy the placeholder example, generate distinct random URL-safe values, and replace the CEO email with the intended development account. Terraform state includes sensitive values, so protect S3 access and use the DPAPI-encrypted laptop backup script. Do not commit tfvars, plans, or state.

```powershell
Set-Location C:\Users\myfin\OneDrive\Desktop\RAS-backend
Copy-Item infra\terraform\dev\terraform.tfvars.example infra\terraform\dev\terraform.tfvars
& .\.tools\terraform\terraform.exe -chdir=infra/terraform/dev init -backend-config="bucket=ras-backend-tfstate-109573999710-eu-west-3" -backend-config="region=eu-west-3" -backend-config="profile=terraform"
& .\.tools\terraform\terraform.exe fmt -check -recursive
& .\.tools\terraform\terraform.exe -chdir=infra/terraform/dev validate
& .\.tools\terraform\terraform.exe -chdir=infra/terraform/dev plan -var-file=terraform.tfvars
```

Review the account, region (`eu-west-3`), plan, all resource types, image sizes, RDS class/storage, and current Free Tier usage. The checked-in example values are placeholders for planning only; do not apply them. Apply only after a fresh plan uses the real ignored tfvars and confirms only eligible resources, and the project owner has reviewed that plan.

## One-command lifecycle

Use Windows PowerShell from the repository root. The `terraform` AWS CLI profile and Docker Desktop must be available. Keep exactly one ignored `*-secrets.tfvars` file in `.tools\terraform` (or set `RAS_TERRAFORM_SECRETS_FILE` to it). The `up` command requires committed source changes in service, database, deployment, and Terraform paths; it refuses a plan containing resource deletions or replacements.

```powershell
Set-Location C:\Users\myfin\OneDrive\Desktop\RAS-backend
.\scripts\aws-dev.ps1 up
```

The command initializes and reviews Terraform, applies only a non-destructive plan, builds and publishes immutable commit-tagged ECR images, waits for SSM, runs migrations once, deploys the three APIs, and checks gateway health.

To terminate the disposable development environment:

```powershell
.\scripts\aws-dev.ps1 down
```

This first creates a Windows-user-encrypted local state backup and displays the Terraform destroy plan. It requires typing `DELETE RAS BACKEND DEV DATA` and deletes the complete application stack, including the RDS database and its data. The separately managed, versioned Terraform state S3 bucket is retained.

To terminate and recreate only the temporary EC2 shell while retaining RDS and
all database data:

```powershell
.\scripts\aws-dev.ps1 shell-down
.\scripts\aws-dev.ps1 up
```

The EC2 bootstrap installs Docker, AWS CLI, PostgreSQL client, SSM Agent, the
deployment helper, `ras-db-shell.sh`, and `ras-host-doctor.sh`. Before images
are deployed, the lifecycle command runs the doctor, which verifies the
required commands, Docker and SSM services, the instance IAM identity, runtime
configuration, RDS DNS resolution, and TCP connectivity to RDS port 5432.
After connecting to the shell, query the database with:

```bash
/usr/local/bin/ras-db-shell.sh -c "SELECT COUNT(*) FROM users;"
```

To destroy the disposable application infrastructure while retaining the RDS
instance, database data, required VPC networking, and Terraform state bucket:

```powershell
.\scripts\aws-dev.ps1 down-preserve-data
```

This removes EC2, Docker, ECR repositories/images, API Gateway, Lambda
functions, application IAM resources, and SSM parameters. It does not remove
RDS or its PostgreSQL data. The command displays the exact targeted destroy
plan and requires typing `DELETE RAS BACKEND DEV APPLICATION`. Run
`.\scripts\aws-dev.ps1 up` afterward to recreate the application resources;
the migration runner applies migrations to the retained database without
deleting existing rows.

## Bootstrap and cleanup behavior

The app host is managed through SSM Session Manager. The lifecycle command uses the host's `ras-backend-deploy.sh migrate <commit-sha>` command once, then deploys each API by its immutable image SHA. The API Gateway and all three services were smoke-tested on 2026-10-14. This is a disposable, single-host test environment with no host-level HA.

The database and application passwords are stored as Standard SecureString SSM parameters. Terraform state also contains sensitive values; keep the encrypted server-side state and DPAPI laptop backup private. Back up state after a successful apply and again after destroy. Do not print secret parameter values.

After endpoint checks, destroy only the application Terraform stack using a reviewed `terraform plan -destroy` and `terraform destroy`. This permanently deletes the disposable RDS database without a final snapshot. The separately bootstrapped S3 state bucket and its versioned state history must remain. Do not delete that bucket as part of application cleanup.
