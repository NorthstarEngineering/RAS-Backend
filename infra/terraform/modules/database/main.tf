resource "aws_security_group" "database" {
  name        = "${var.name}-postgres"
  description = "Private PostgreSQL access only from the application host security group."
  vpc_id      = var.vpc_id
  ingress {
    description     = "PostgreSQL from the private application host"
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    security_groups = [var.application_security_group_id]
  }
  tags = { Name = "${var.name}-postgres", Environment = "development" }
}

resource "aws_db_subnet_group" "this" {
  name       = "${var.name}-private-db"
  subnet_ids = var.private_subnet_ids
  tags       = { Name = "${var.name}-private-db", Environment = "development" }
}

resource "aws_db_instance" "this" {
  identifier                 = "${var.name}-dev"
  engine                     = "postgres"
  engine_version             = var.engine_version
  instance_class             = var.instance_class
  allocated_storage          = var.allocated_storage_gib
  storage_type               = "gp2"
  storage_encrypted          = true
  db_name                    = var.database_name
  username                   = var.master_username
  password                   = var.master_password
  db_subnet_group_name       = aws_db_subnet_group.this.name
  vpc_security_group_ids     = [aws_security_group.database.id]
  publicly_accessible        = false
  multi_az                   = false
  backup_retention_period    = var.backup_retention_days
  copy_tags_to_snapshot      = true
  deletion_protection        = var.protect_database
  skip_final_snapshot        = true
  auto_minor_version_upgrade = true
  apply_immediately          = false
  tags                       = { Name = "${var.name}-dev", Environment = "development", DataClass = "development" }
}

resource "aws_ssm_parameter" "master_password" {
  name        = "/ras-backend/dev/database-master-password"
  description = "Disposable development RDS master password"
  type        = "SecureString"
  tier        = "Standard"
  value       = var.master_password
  tags        = { Environment = "development", DataClass = "secret" }
}
