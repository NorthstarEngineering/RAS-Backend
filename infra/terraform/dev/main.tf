provider "aws" {
  region = var.aws_region
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
  instance_class                = var.database_instance_class
  engine_version                = var.database_engine_version
  allocated_storage_gib         = var.database_allocated_storage_gib
  backup_retention_days         = var.database_backup_retention_days
}

module "application" {
  source                        = "../modules/application"
  name                          = "ras-backend"
  ami_id                        = data.aws_ami.ubuntu.id
  instance_type                 = var.application_instance_type
  private_subnet_id             = module.network.private_subnet_ids[0]
  application_security_group_id = module.network.application_security_group_id
  app_secret_arn                = var.app_secret_arn
  database_master_secret_arn    = module.database.master_secret_arn
  deploy_script_content         = file("${path.module}/../../../deploy/aws-dev-deploy.sh")
}
