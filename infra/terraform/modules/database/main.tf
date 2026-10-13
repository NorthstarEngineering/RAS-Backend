resource "aws_security_group" "database" {
  name        = "${var.name}-postgres"
  description = "Private PostgreSQL access only from the application host security group."
  vpc_id      = var.vpc_id
  ingress {
    description     = "PostgreSQL from application host"
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
  identifier                  = "${var.name}-dev"
  engine                      = "postgres"
  engine_version              = var.engine_version
  instance_class              = var.instance_class
  allocated_storage           = var.allocated_storage_gib
  max_allocated_storage       = max(var.allocated_storage_gib, 100)
  storage_type                = "gp3"
  storage_encrypted           = true
  db_name                     = var.database_name
  username                    = var.master_username
  manage_master_user_password = true
  db_subnet_group_name        = aws_db_subnet_group.this.name
  vpc_security_group_ids      = [aws_security_group.database.id]
  publicly_accessible         = false
  multi_az                    = false
  backup_retention_period     = var.backup_retention_days
  copy_tags_to_snapshot       = true
  deletion_protection         = true
  skip_final_snapshot         = false
  final_snapshot_identifier   = "${var.name}-dev-final"
  auto_minor_version_upgrade  = true
  apply_immediately           = false
  tags                        = { Name = "${var.name}-dev", Environment = "development", DataClass = "development" }

  lifecycle { prevent_destroy = true }
}
