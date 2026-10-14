locals {
  repository_names = toset(["ras-auth-api", "ras-branch-api", "ras-attendance-api", "ras-db-migrations"])
}

resource "aws_ecr_repository" "this" {
  for_each             = local.repository_names
  name                 = each.value
  image_tag_mutability = "IMMUTABLE"
  image_scanning_configuration { scan_on_push = true }
  encryption_configuration { encryption_type = "AES256" }
  tags = { Environment = "development", Name = each.value }
}

resource "aws_ecr_lifecycle_policy" "this" {
  for_each   = aws_ecr_repository.this
  repository = each.value.name
  policy = jsonencode({
    rules = [{
      rulePriority = 1
      description  = "Retain the newest 30 immutable development images"
      selection    = { tagStatus = "any", countType = "imageCountMoreThan", countNumber = 30 }
      action       = { type = "expire" }
    }]
  })
}

resource "aws_iam_role" "instance" {
  name = "${var.name}-dev-host"
  assume_role_policy = jsonencode({
    Version   = "2012-10-17"
    Statement = [{ Effect = "Allow", Principal = { Service = "ec2.amazonaws.com" }, Action = "sts:AssumeRole" }]
  })
  tags = { Environment = "development" }
}

resource "aws_iam_role_policy_attachment" "ssm" {
  role       = aws_iam_role.instance.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"
}

resource "aws_iam_role_policy" "runtime" {
  name = "${var.name}-dev-pull-and-secrets"
  role = aws_iam_role.instance.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      { Effect = "Allow", Action = ["ecr:GetAuthorizationToken"], Resource = "*" },
      { Effect = "Allow", Action = ["ecr:BatchCheckLayerAvailability", "ecr:GetDownloadUrlForLayer", "ecr:BatchGetImage"], Resource = [for repo in aws_ecr_repository.this : repo.arn] },
      { Effect = "Allow", Action = ["secretsmanager:GetSecretValue"], Resource = [var.app_secret_arn, var.database_master_secret_arn] }
    ]
  })
}

resource "aws_iam_instance_profile" "this" {
  name = "${var.name}-dev-host"
  role = aws_iam_role.instance.name
}

resource "aws_instance" "this" {
  ami                         = var.ami_id
  instance_type               = var.instance_type
  subnet_id                   = var.private_subnet_id
  vpc_security_group_ids      = [var.application_security_group_id]
  iam_instance_profile        = aws_iam_instance_profile.this.name
  associate_public_ip_address = false
  user_data                   = <<-USERDATA
    #!/bin/bash
    set -euo pipefail
    apt-get update
    apt-get install -y docker.io awscli jq
    systemctl enable --now docker
    usermod -aG docker ubuntu
    snap install amazon-ssm-agent --classic || true
    systemctl enable --now snap.amazon-ssm-agent.amazon-ssm-agent.service || true
    cat > /usr/local/bin/ras-backend-deploy.sh <<'RAS_DEPLOY_SCRIPT'
${var.deploy_script_content}
RAS_DEPLOY_SCRIPT
    chmod 0700 /usr/local/bin/ras-backend-deploy.sh
  USERDATA
  user_data_replace_on_change = false

  metadata_options {
    http_endpoint               = "enabled"
    http_tokens                 = "required"
    http_put_response_hop_limit = 1
  }
  root_block_device {
    volume_type           = "gp3"
    volume_size           = 30
    encrypted             = true
    delete_on_termination = true
  }
  tags = { Name = "${var.name}-dev-app-host", Environment = "development" }
}
