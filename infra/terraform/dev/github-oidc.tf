data "aws_caller_identity" "current" {}
data "aws_iam_openid_connect_provider" "github" {
  url = "https://token.actions.githubusercontent.com"
}

locals {
  publish_repository_arns = [for repository in module.application.ecr_repository_urls : "arn:aws:ecr:${var.aws_region}:${data.aws_caller_identity.current.account_id}:repository/${split("/", repository)[1]}"]
}

resource "aws_iam_role" "github_actions_ecr_publish" {
  name = "ras-backend-dev-github-ecr-publish"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Action    = "sts:AssumeRoleWithWebIdentity"
      Principal = { Federated = data.aws_iam_openid_connect_provider.github.arn }
      Condition = {
        StringEquals = { "token.actions.githubusercontent.com:aud" = "sts.amazonaws.com" }
        StringLike   = { "token.actions.githubusercontent.com:sub" = "repo:NorthstarEngineering@341364035/RAS-Backend@1414575415:environment:development-publish" }
      }
    }]
  })
  tags = { Environment = "development" }
}

resource "aws_iam_role_policy" "github_actions_ecr_publish" {
  name = "publish-and-deploy-ras-images"
  role = aws_iam_role.github_actions_ecr_publish.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      { Effect = "Allow", Action = ["ecr:GetAuthorizationToken"], Resource = "*" },
      { Effect = "Allow", Action = ["ecr:BatchCheckLayerAvailability", "ecr:InitiateLayerUpload", "ecr:UploadLayerPart", "ecr:CompleteLayerUpload", "ecr:PutImage"], Resource = local.publish_repository_arns },
      { Effect = "Allow", Action = ["ec2:DescribeInstances"], Resource = "*" },
      { Effect = "Allow", Action = ["ssm:SendCommand"], Resource = ["arn:aws:ssm:${var.aws_region}::document/AWS-RunShellScript", "arn:aws:ec2:${var.aws_region}:${data.aws_caller_identity.current.account_id}:instance/${module.application.instance_id}"] },
      { Effect = "Allow", Action = ["ssm:GetCommandInvocation"], Resource = "*" }
    ]
  })
}
