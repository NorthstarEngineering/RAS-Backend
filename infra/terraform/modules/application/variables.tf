variable "name" { type = string }
variable "ami_id" { type = string }
variable "instance_type" { type = string }
variable "public_subnet_id" { type = string }
variable "application_security_group_id" { type = string }
variable "app_secret_parameter_arn" { type = string }
variable "database_master_password_parameter_arn" { type = string }
variable "aws_region" { type = string }
variable "app_secret_parameter_name" { type = string }
variable "database_master_password_parameter_name" { type = string }
variable "database_master_username" { type = string }
variable "database_endpoint" { type = string }
variable "database_name" { type = string }
variable "ssh_allowed_cidr" {
  type     = string
  nullable = true
  default  = null
}
variable "ec2_instance_connect_cidr" {
  type        = string
  description = "EC2 Instance Connect service CIDR for the AWS region."
  default     = "35.180.112.80/29"
}
variable "ssh_key_name" {
  type     = string
  nullable = true
  default  = null
}

variable "deploy_script_content" {
  type = string
}
