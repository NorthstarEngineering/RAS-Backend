variable "name" { type = string }
variable "ami_id" { type = string }
variable "instance_type" { type = string }
variable "private_subnet_id" { type = string }
variable "application_security_group_id" { type = string }
variable "app_secret_arn" { type = string }
variable "database_master_secret_arn" { type = string }

variable "deploy_script_content" {
  type = string
}
