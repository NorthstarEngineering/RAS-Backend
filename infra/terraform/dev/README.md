# Development Terraform

This stack defines development resources only: VPC/subnets/security groups, one SSM-accessible EC2 app host, four immutable ECR repositories, and one private encrypted RDS PostgreSQL database. It has not been applied.

Requirements: Terraform >=1.10, AWS CLI credentials for an approved developer role, S3 bucket bootstrapped with versioning/encryption/public access blocked, an existing Secrets Manager app secret ARN, and an approved developer IPv4 CIDR. The backend uses S3 native state locking (`use_lockfile`) and a fixed development key. Do not put secret values in Terraform variables.

Copy `terraform.tfvars.example` to `terraform.tfvars` and supply real nonsecret values locally. Initialize with the separately provisioned state bucket:

```powershell
terraform init -backend-config="bucket=YOUR_STATE_BUCKET" -backend-config="region=YOUR_REGION"
terraform fmt -check -recursive
terraform validate
terraform plan -var-file=terraform.tfvars
```

Inspect the account, region, CIDRs, database retention, and every resource in the plan. `apply` and `destroy` are approval-gated and are not run here. The database has deletion protection and `prevent_destroy`; do not remove them for routine app cleanup. See [deployment instructions](../../../docs/deployment.md).
