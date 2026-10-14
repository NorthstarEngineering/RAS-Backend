variable "aws_region" {
  type        = string
  description = "AWS region for development resources."
}
variable "aws_profile" {
  type        = string
  description = "Optional local AWS CLI profile. Leave unset in CI to use GitHub OIDC credentials."
  default     = null
}
variable "app_secret_arn" {
  type        = string
  description = "Pre-created Secrets Manager secret ARN holding shared JWT, runtime DB passwords, and development CEO bootstrap values."
}
variable "authorizer_jwt_secret_arn" {
  type        = string
  description = "Dedicated Secrets Manager secret ARN containing only the Auth JWT signing secret."
}
variable "api_cors_allowed_origins" {
  type        = list(string)
  description = "Exact HTTPS origins allowed by API Gateway CORS. Keep empty for native-only mobile clients."
  default     = []
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
  default = "t3.small"
}
variable "database_name" {
  type    = string
  default = "restaurant_db"
}
variable "database_master_username" {
  type    = string
  default = "migration_admin"
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
  default = 7
}
