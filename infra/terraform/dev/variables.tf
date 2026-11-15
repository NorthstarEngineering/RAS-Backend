variable "aws_region" {
  type        = string
  description = "AWS region for development resources."
}
variable "aws_profile" {
  type        = string
  description = "Optional local AWS CLI profile. Leave unset in CI to use GitHub OIDC credentials."
  default     = null
}
variable "app_secret_json" {
  type        = string
  sensitive   = true
  description = "Local-only JSON with JWT_SECRET, per-service DB passwords, and bootstrap CEO credentials; stored as SSM Standard SecureString."
  validation {
    condition = can(jsondecode(var.app_secret_json)) && alltrue([
      for key in ["JWT_SECRET", "AUTH_DB_PASSWORD", "BRANCH_DB_PASSWORD", "ATTENDANCE_DB_PASSWORD", "SEED_CEO_EMAIL", "SEED_CEO_PASSWORD"] : try(length(jsondecode(var.app_secret_json)[key]) > 0, false)
      ]) && try(
      can(regex("^[A-Za-z0-9._~-]{32,}$", jsondecode(var.app_secret_json).JWT_SECRET)) &&
      alltrue([
        for key in ["AUTH_DB_PASSWORD", "BRANCH_DB_PASSWORD", "ATTENDANCE_DB_PASSWORD", "SEED_CEO_PASSWORD"] :
        can(regex("^[A-Za-z0-9._~@-]+$", jsondecode(var.app_secret_json)[key]))
      ]) &&
      can(regex("^[^[:space:]@]+@[^[:space:]@]+\\.[^[:space:]@]+$", jsondecode(var.app_secret_json).SEED_CEO_EMAIL))
    , false)
    error_message = "app_secret_json must contain a 32+ character URL-safe JWT secret, URL-safe service/CEO passwords, and a valid CEO email."
  }
}
variable "database_master_password" {
  type        = string
  sensitive   = true
  description = "Local-only disposable RDS master password; also stored in SSM Parameter Store Standard SecureString."
  validation {
    condition     = can(regex("^[A-Za-z0-9._~-]{16,128}$", var.database_master_password))
    error_message = "database_master_password must be 16-128 URL-safe characters."
  }
}
variable "api_cors_allowed_origins" {
  type        = list(string)
  description = "Exact browser origins allowed by API Gateway CORS. Add the deployed web origin when hosting the frontend."
  default     = ["http://localhost:5175", "http://127.0.0.1:5175"]
}
variable "ssh_allowed_cidr" {
  type        = string
  nullable    = true
  default     = null
  description = "Optional IPv4 CIDR allowed to SSH to the temporary application shell. Prefer the current public /32."
}
variable "ec2_instance_connect_cidr" {
  type        = string
  description = "EC2 Instance Connect service CIDR. For eu-west-3 use 35.180.112.80/29."
  default     = "35.180.112.80/29"
}
variable "ssh_key_name" {
  type        = string
  nullable    = true
  default     = null
  description = "Optional existing EC2 key pair name for SSH."
}
variable "vpc_cidr" {
  type    = string
  default = "10.40.0.0/16"
}
variable "public_subnet_cidrs" {
  type    = list(string)
  default = ["10.40.0.0/24", "10.40.1.0/24"]
}
variable "private_subnet_cidrs" {
  type    = list(string)
  default = ["10.40.10.0/24", "10.40.11.0/24"]
}
variable "application_instance_type" {
  type    = string
  default = "t3.micro"
}
variable "database_name" {
  type    = string
  default = "restaurant_db"
}
variable "database_master_username" {
  type    = string
  default = "migration_admin"
  validation {
    condition     = can(regex("^[A-Za-z][A-Za-z0-9_]{0,62}$", var.database_master_username))
    error_message = "database_master_username must start with a letter and contain only letters, numbers, and underscores."
  }
}
variable "database_engine_version" {
  type    = string
  default = "16.15"
}
variable "database_instance_class" {
  type    = string
  default = "db.t4g.micro"
}
variable "database_allocated_storage_gib" {
  type    = number
  default = 20
}
variable "database_backup_retention_days" {
  type    = number
  default = 0
  validation {
    condition     = var.database_backup_retention_days == 0
    error_message = "Disposable Free Tier test databases must not retain automated backups."
  }
}
variable "protect_database" {
  type        = bool
  default     = true
  description = "Keep RDS and its data when replacing or terminating the EC2 shell."
}
