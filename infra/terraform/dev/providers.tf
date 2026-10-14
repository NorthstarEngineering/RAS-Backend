terraform {
  required_version = ">= 1.10.0"
  backend "s3" {
    key          = "ras-backend/dev/terraform.tfstate"
    encrypt      = true
    use_lockfile = true
  }
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 5.80.0, < 7.0.0"
    }
  }
}
