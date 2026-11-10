data "aws_caller_identity" "current" {}
locals {
  repository_names    = toset(["ras-auth-api", "ras-branch-api", "ras-attendance-api", "ras-db-migrations"])
  ecr_repository_urls = { for name in local.repository_names : name => "${data.aws_caller_identity.current.account_id}.dkr.ecr.${var.aws_region}.amazonaws.com/${name}" }
}

resource "aws_ecr_repository" "this" {
  for_each             = local.repository_names
  name                 = each.value
  image_tag_mutability = "IMMUTABLE"
  image_scanning_configuration { scan_on_push = false }
  encryption_configuration { encryption_type = "AES256" }
  tags = { Environment = "development", Name = each.value }
}

resource "aws_ecr_lifecycle_policy" "this" {
  for_each   = aws_ecr_repository.this
  repository = each.value.name
  policy = jsonencode({
    rules = [{
      rulePriority = 1
      description  = "Keep only the newest image for each development repository"
      selection    = { tagStatus = "any", countType = "imageCountMoreThan", countNumber = 1 }
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
      { Effect = "Allow", Action = ["ssm:GetParameter"], Resource = [var.app_secret_parameter_arn, var.database_master_password_parameter_arn] },
      { Effect = "Allow", Action = ["kms:Decrypt"], Resource = ["*"], Condition = { StringEquals = { "kms:ViaService" = "ssm.${var.aws_region}.amazonaws.com" }, StringLike = { "kms:EncryptionContext:PARAMETER_ARN" = "arn:aws:ssm:${var.aws_region}:*:parameter/ras-backend/dev/*" } } }
    ]
  })
}

resource "aws_vpc_security_group_ingress_rule" "ssh" {
  count             = var.ssh_allowed_cidr == null ? 0 : 1
  security_group_id = var.application_security_group_id
  cidr_ipv4         = var.ssh_allowed_cidr
  ip_protocol       = "tcp"
  from_port         = 22
  to_port           = 22
  description       = "Temporary administrative SSH access"
}

resource "aws_vpc_security_group_ingress_rule" "ec2_instance_connect" {
  security_group_id = var.application_security_group_id
  cidr_ipv4         = var.ec2_instance_connect_cidr
  ip_protocol       = "tcp"
  from_port         = 22
  to_port           = 22
  description       = "EC2 Instance Connect service"
}

resource "aws_iam_instance_profile" "this" {
  name = "${var.name}-dev-host"
  role = aws_iam_role.instance.name
}

resource "aws_instance" "this" {
  ami           = var.ami_id
  instance_type = var.instance_type
  key_name      = var.ssh_key_name
  credit_specification {
    cpu_credits = "standard"
  }
  subnet_id                   = var.public_subnet_id
  vpc_security_group_ids      = [var.application_security_group_id]
  iam_instance_profile        = aws_iam_instance_profile.this.name
  associate_public_ip_address = true
  user_data = replace(<<-USERDATA
#!/bin/bash
set -euo pipefail
apt-get update
apt-get install -y ca-certificates docker.io jq curl unzip postgresql-client
curl -fsSL https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip -o /tmp/awscliv2.zip
unzip -q -o /tmp/awscliv2.zip -d /tmp
/tmp/aws/install
rm -rf /tmp/aws /tmp/awscliv2.zip
systemctl enable --now docker
usermod -aG docker ubuntu
snap install amazon-ssm-agent --classic || true
systemctl enable --now snap.amazon-ssm-agent.amazon-ssm-agent.service || true
cat > /usr/local/bin/ras-backend-deploy.sh <<'RAS_DEPLOY_SCRIPT'
${replace(var.deploy_script_content, "\r\n", "\n")}
RAS_DEPLOY_SCRIPT
chmod 0700 /usr/local/bin/ras-backend-deploy.sh
cat >/etc/ras-backend-deploy.env <<'ENVVARS'
AWS_REGION=${var.aws_region}
APP_SECRET_PARAMETER_NAME=${var.app_secret_parameter_name}
DATABASE_MASTER_PASSWORD_PARAMETER_NAME=${var.database_master_password_parameter_name}
DATABASE_MASTER_USERNAME=${var.database_master_username}
DATABASE_ENDPOINT=${var.database_endpoint}
DB_NAME=${var.database_name}
AUTH_IMAGE_REPOSITORY=${local.ecr_repository_urls["ras-auth-api"]}
BRANCH_IMAGE_REPOSITORY=${local.ecr_repository_urls["ras-branch-api"]}
ATTENDANCE_IMAGE_REPOSITORY=${local.ecr_repository_urls["ras-attendance-api"]}
MIGRATION_IMAGE_REPOSITORY=${local.ecr_repository_urls["ras-db-migrations"]}
ENVVARS
chmod 0644 /etc/ras-backend-deploy.env
cat > /usr/local/bin/ras-host-doctor.sh <<'DOCTOR'
#!/bin/bash
set -euo pipefail
source /etc/ras-backend-deploy.env

for command_name in aws docker jq psql curl; do
  command -v "$command_name" >/dev/null || { echo "Missing required command: $command_name" >&2; exit 1; }
done
systemctl is-active --quiet docker || { echo "Docker is not active." >&2; exit 1; }
systemctl is-active --quiet snap.amazon-ssm-agent.amazon-ssm-agent.service || { echo "SSM Agent is not active." >&2; exit 1; }
aws sts get-caller-identity --query Account --output text --region "$AWS_REGION" >/dev/null
test -r /etc/ras-backend-deploy.env
test -x /usr/local/bin/ras-backend-deploy.sh
test -x /usr/local/bin/ras-db-shell.sh
getent hosts "$DATABASE_ENDPOINT" >/dev/null
timeout 5 bash -c "</dev/tcp/$DATABASE_ENDPOINT/5432" || { echo "RDS port 5432 is unreachable." >&2; exit 1; }
docker info >/dev/null
echo "RAS EC2 host is ready."
DOCTOR
chmod 0755 /usr/local/bin/ras-host-doctor.sh
cat > /usr/local/bin/ras-db-shell.sh <<'DBSHELL'
#!/bin/bash
set -euo pipefail
source /etc/ras-backend-deploy.env
export PGPASSWORD="$(aws ssm get-parameter --name "$DATABASE_MASTER_PASSWORD_PARAMETER_NAME" --with-decryption --query Parameter.Value --output text --region "$AWS_REGION")"
exec psql --host "$DATABASE_ENDPOINT" --port 5432 --username "$DATABASE_MASTER_USERNAME" --dbname "$DB_NAME" "$@"
DBSHELL
chmod 0755 /usr/local/bin/ras-db-shell.sh
  USERDATA
  , "\r", "")
  user_data_replace_on_change = false

  metadata_options {
    http_endpoint               = "enabled"
    http_tokens                 = "required"
    http_put_response_hop_limit = 1
  }
  root_block_device {
    volume_type           = "gp3"
    volume_size           = 8
    encrypted             = true
    delete_on_termination = true
  }
  tags = { Name = "${var.name}-dev-app-host", Environment = "development" }
}
