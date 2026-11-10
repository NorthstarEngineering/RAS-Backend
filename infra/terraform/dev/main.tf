provider "aws" {
  region  = var.aws_region
  profile = var.aws_profile
  default_tags {
    tags = { Project = "RAS-backend", Environment = "development", ManagedBy = "Terraform" }
  }
}

data "aws_availability_zones" "available" { state = "available" }
data "aws_ami" "ubuntu" {
  most_recent = true
  owners      = ["099720109477"]
  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd-gp3/ubuntu-noble-24.04-amd64-server-*"]
  }
  filter {
    name   = "architecture"
    values = ["x86_64"]
  }
  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }
  filter {
    name   = "root-device-type"
    values = ["ebs"]
  }
}

module "network" {
  source               = "../modules/network"
  name                 = "ras-backend-dev"
  vpc_cidr             = var.vpc_cidr
  availability_zones   = slice(data.aws_availability_zones.available.names, 0, 2)
  public_subnet_cidrs  = var.public_subnet_cidrs
  private_subnet_cidrs = var.private_subnet_cidrs
}

module "database" {
  source                        = "../modules/database"
  name                          = "ras-backend"
  vpc_id                        = module.network.vpc_id
  private_subnet_ids            = module.network.private_subnet_ids
  application_security_group_id = module.network.application_security_group_id
  database_name                 = var.database_name
  master_username               = var.database_master_username
  master_password               = var.database_master_password
  instance_class                = var.database_instance_class
  engine_version                = var.database_engine_version
  allocated_storage_gib         = var.database_allocated_storage_gib
  backup_retention_days         = var.database_backup_retention_days
  protect_database              = var.protect_database
}

resource "aws_ssm_parameter" "application_secrets" {
  name        = "/ras-backend/dev/application-secrets"
  description = "Development runtime credentials for the three backend applications and JWT authorizer"
  type        = "SecureString"
  tier        = "Standard"
  value       = var.app_secret_json
  tags        = { Environment = "development", DataClass = "secret" }
}

module "application" {
  source                                  = "../modules/application"
  name                                    = "ras-backend"
  ami_id                                  = data.aws_ami.ubuntu.id
  instance_type                           = var.application_instance_type
  public_subnet_id                        = module.network.public_subnet_ids[0]
  application_security_group_id           = module.network.application_security_group_id
  app_secret_parameter_name               = aws_ssm_parameter.application_secrets.name
  app_secret_parameter_arn                = aws_ssm_parameter.application_secrets.arn
  database_master_password_parameter_name = module.database.master_password_parameter_name
  database_master_password_parameter_arn  = module.database.master_password_parameter_arn
  database_master_username                = var.database_master_username
  database_endpoint                       = module.database.endpoint
  database_name                           = module.database.database_name
  aws_region                              = var.aws_region
  ssh_allowed_cidr                        = var.ssh_allowed_cidr
  ec2_instance_connect_cidr               = var.ec2_instance_connect_cidr
  ssh_key_name                            = var.ssh_key_name
  deploy_script_content                   = file("${path.module}/../../../deploy/aws-dev-deploy.sh")
}
