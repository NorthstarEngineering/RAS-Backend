# Free Tier deployment review

**Updated:** 2026-10-14  
**Region:** `eu-west-3` (Europe/Paris)  
**AWS account:** `109573999710`  
**Deployment status:** no application infrastructure applied.

## Account check

The `terraform` profile authenticated successfully. AWS returned Free plan `ACTIVE`, with **$108.36** remaining and plan expiry **2027-02-02T15:16:41Z**. This is a finite account-plan balance, not a monthly allowance. The last resource inventory found no project EC2 instance, RDS instance, API Gateway API, or load balancer. The dedicated versioned S3 Terraform state bucket exists, is private/encrypted, and contains versioned `.tflock` objects but no Terraform state object. The old `takwa231018545` bucket remains untouched and is not used for state.

The AWS Free Tier API showed current Free Tier usage for Glue catalog requests and KMS requests only. It did not report usage rows for the application products because the stack has not been deployed. The last KMS usage reading was 7 requests, forecast 15.5 against the 20,000 always-free monthly allowance.

## Free Tier-only redesign

The first Terraform plan proposed 81 resources, including NAT Gateway, ALB, VPC Link, Secrets Manager, a `t3.small` host, and CloudWatch logging. Those hourly or metered items are removed from the revised design.

The local Terraform configuration now targets API Gateway HTTP API → Lambda HS256 authorizer → VPC-attached Lambda request proxy → Docker services on one `t3.micro` EC2 host → private `db.t4g.micro` RDS PostgreSQL. The host accepts service-port traffic only from the proxy Lambda security group. It has a public IPv4 for outbound ECR/package access and SSM management; the host ports have no public ingress. RDS remains private and permits only the app host security group. Standard Parameter Store SecureString replaces Secrets Manager. The temporary RDS database has no automated backups or final snapshot because the user intends to destroy it after testing.

Resources remaining in the design must stay within account-specific offers and usage limits:

| AWS component | Free Tier basis / limit to respect |
| --- | --- |
| EC2 host | `t3.micro` is eligible under the new Free plan; public IPv4 usage must remain within the EC2 Free Tier allowance. Root gp3 disk is 8 GiB; EC2 T3 CPU credits are Standard to prevent paid surplus credits. |
| RDS PostgreSQL | AWS lists `db.t3.micro` and `db.t4g.micro` for the Free plan. The deployment is single-AZ with 20 GiB gp2 storage, no automated backups, and no final snapshot. |
| API Gateway HTTP API | Up to 1 million HTTP API calls/month for eligible new customers and the published offer period. |
| Lambda | Authorizer and proxy together use the 1 million request/400,000 GB-second monthly allowance; protected requests invoke both functions. |
| ECR private repositories | 500 MB/month private storage for one year for eligible new customers. Keep one image per repo; measure all four images together before publishing. |
| Systems Manager Parameter Store | Standard parameters have no additional charge; SecureString values use AWS-managed `aws/ssm`, not a customer-managed KMS key. |
| VPC/subnets/route tables/security groups/Internet Gateway | No NAT Gateway, public EIP resource, VPC endpoint, ALB, or API Gateway VPC Link is present in the revised plan. |
| S3 Terraform state | Retain the existing secured/versioned bucket. Keep state and versions small; it is not part of app teardown. |

Published offer references: [AWS Free account plan](https://docs.aws.amazon.com/awsaccountbilling/latest/aboutv2/free-tier.html), [EC2 Free Tier eligibility](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-free-tier-usage.html), [RDS Free Tier](https://aws.amazon.com/rds/free/), [API Gateway pricing](https://aws.amazon.com/api-gateway/pricing/), [Lambda pricing](https://aws.amazon.com/lambda/pricing/), [ECR pricing](https://aws.amazon.com/ecr/pricing/), and [Parameter Store pricing](https://aws.amazon.com/systems-manager/pricing/).

These published offers are conditional on account eligibility, date, region, combined account usage, and the offer's duration. The Terraform plan cannot prove an AWS usage allowance will remain available. Recheck the account plan and usage immediately before any apply; do not upgrade the account or exceed Free Tier offers. AWS states the Free account plan ends after six months or when credits are exhausted, whichever happens first. It is not a permanent free hosting plan.

## Current validation and limits

- The current placeholder-only Terraform plan plans **65 additions, 0 changes, 0 destroys**. It contains API Gateway, Lambda, EC2 `t3.micro`, RDS `db.t4g.micro`, ECR, SSM Standard parameters, IAM, and VPC networking. It contains no NAT Gateway, ALB, VPC Link, Secrets Manager, CloudWatch log group, or EIP resource. The plan is saved under ignored `.tools/ras-freetier.tfplan`; it is not an apply plan because it uses placeholders.
- Bundled Terraform 1.13.5 `fmt -check -recursive` and `validate` passed after the latest edits.
- No application image sizes were verified because Docker Desktop's Linux engine is unavailable. ECR publishing must remain blocked until the combined stored image size is checked against 500 MB.
- The Free Tier usage and account-plan APIs were rechecked after planning. The plan still reports $108.36 remaining; usage lists 131 Glue catalog requests (forecast 290.07/1,000,000) and 7 KMS requests (forecast 15.5/20,000). No application resource was created or modified during this review.
- No budget alarm is configured. Budgets are alerts, not hard spending caps. The Free account plan itself currently prevents charges unless the account is upgraded, but use only the offers/limits above and monitor usage.

## Teardown/data handling

This RDS database is disposable and the planned Terraform destroy removes it without a final snapshot. No production or personal data may be used. The remote S3 state bucket and versioned history remain. Terraform state includes sensitive credential values; keep the S3 bucket private and the local backup DPAPI-encrypted. After destruction, verify the app stack resources are absent and preserve the S3 backend only.
