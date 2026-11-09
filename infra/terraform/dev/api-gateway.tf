resource "aws_apigatewayv2_api" "public" {
  name          = "ras-backend-dev"
  protocol_type = "HTTP"
  dynamic "cors_configuration" {
    for_each = length(var.api_cors_allowed_origins) > 0 ? [1] : []
    content {
      allow_origins  = var.api_cors_allowed_origins
      allow_methods  = ["GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"]
      allow_headers  = ["authorization", "content-type", "x-request-id"]
      expose_headers = ["x-request-id"]
      max_age        = 300
    }
  }
  tags = { Environment = "development" }
}

resource "aws_iam_role" "authorizer" {
  name               = "ras-backend-dev-jwt-authorizer"
  assume_role_policy = jsonencode({ Version = "2012-10-17", Statement = [{ Effect = "Allow", Principal = { Service = "lambda.amazonaws.com" }, Action = "sts:AssumeRole" }] })
  tags               = { Environment = "development" }
}
resource "aws_iam_role_policy" "authorizer_runtime" {
  name = "read-signing-parameter"
  role = aws_iam_role.authorizer.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      { Effect = "Allow", Action = ["ssm:GetParameter"], Resource = [aws_ssm_parameter.application_secrets.arn] },
      { Effect = "Allow", Action = ["kms:Decrypt"], Resource = ["*"], Condition = { StringEquals = { "kms:ViaService" = "ssm.${var.aws_region}.amazonaws.com" }, StringLike = { "kms:EncryptionContext:PARAMETER_ARN" = "arn:aws:ssm:${var.aws_region}:*:parameter/ras-backend/dev/*" } } },
    ]
  })
}
data "archive_file" "authorizer" {
  type        = "zip"
  output_path = "${path.module}/jwt-authorizer.zip"
  source {
    content  = file("${path.module}/jwt-authorizer/index.mjs")
    filename = "index.mjs"
  }
  source {
    content  = file("${path.module}/jwt-authorizer/jwt.mjs")
    filename = "jwt.mjs"
  }
}
resource "aws_lambda_function" "authorizer" {
  function_name    = "ras-backend-dev-jwt-authorizer"
  role             = aws_iam_role.authorizer.arn
  runtime          = "nodejs22.x"
  handler          = "index.handler"
  filename         = data.archive_file.authorizer.output_path
  source_code_hash = data.archive_file.authorizer.output_base64sha256
  timeout          = 5
  memory_size      = 128
  depends_on       = [aws_iam_role_policy.authorizer_runtime]
  environment { variables = { APP_SECRET_PARAMETER_NAME = aws_ssm_parameter.application_secrets.name } }
  tags = { Environment = "development" }
}
resource "aws_lambda_permission" "api_invoke_authorizer" {
  statement_id  = "AllowHttpApiInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.authorizer.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_apigatewayv2_api.public.execution_arn}/*/*"
}
resource "aws_apigatewayv2_authorizer" "jwt" {
  api_id                            = aws_apigatewayv2_api.public.id
  authorizer_type                   = "REQUEST"
  authorizer_uri                    = aws_lambda_function.authorizer.invoke_arn
  authorizer_payload_format_version = "2.0"
  enable_simple_responses           = true
  authorizer_result_ttl_in_seconds  = 0
  identity_sources                  = ["$request.header.Authorization"]
  name                              = "auth-hs256"
}

resource "aws_iam_role" "request_proxy" {
  name               = "ras-backend-dev-request-proxy"
  assume_role_policy = jsonencode({ Version = "2012-10-17", Statement = [{ Effect = "Allow", Principal = { Service = "lambda.amazonaws.com" }, Action = "sts:AssumeRole" }] })
  tags               = { Environment = "development" }
}
resource "aws_iam_role_policy" "request_proxy_network" {
  name   = "manage-lambda-vpc-interfaces"
  role   = aws_iam_role.request_proxy.id
  policy = jsonencode({ Version = "2012-10-17", Statement = [{ Effect = "Allow", Action = ["ec2:CreateNetworkInterface", "ec2:DescribeNetworkInterfaces", "ec2:DeleteNetworkInterface", "ec2:AssignPrivateIpAddresses", "ec2:UnassignPrivateIpAddresses"], Resource = ["*"] }] })
}
data "archive_file" "request_proxy" {
  type        = "zip"
  source_file = "${path.module}/request-proxy.mjs"
  output_path = "${path.module}/request-proxy.zip"
}
resource "aws_lambda_function" "request_proxy" {
  function_name    = "ras-backend-dev-request-proxy"
  role             = aws_iam_role.request_proxy.arn
  runtime          = "nodejs22.x"
  handler          = "request-proxy.handler"
  filename         = data.archive_file.request_proxy.output_path
  source_code_hash = data.archive_file.request_proxy.output_base64sha256
  timeout          = 28
  memory_size      = 128
  vpc_config {
    subnet_ids         = module.network.private_subnet_ids
    security_group_ids = [module.network.api_proxy_security_group_id]
  }
  environment {
    variables = { BACKEND_PRIVATE_IP = module.application.instance_private_ip }
  }
  depends_on = [aws_iam_role_policy.request_proxy_network]
  tags       = { Environment = "development" }
}
resource "aws_lambda_permission" "api_invoke_proxy" {
  statement_id  = "AllowHttpApiInvokeProxy"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.request_proxy.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_apigatewayv2_api.public.execution_arn}/*/*"
}
resource "aws_apigatewayv2_integration" "lambda_proxy" {
  api_id                 = aws_apigatewayv2_api.public.id
  integration_type       = "AWS_PROXY"
  integration_method     = "POST"
  integration_uri        = aws_lambda_function.request_proxy.invoke_arn
  payload_format_version = "2.0"
  timeout_milliseconds   = 30000
}
resource "aws_apigatewayv2_route" "public_auth" {
  for_each  = toset(["POST /api/v1/auth/login", "POST /api/v1/auth/refresh", "POST /api/v1/auth/logout", "OPTIONS /api/v1/auth/login", "OPTIONS /api/v1/auth/refresh", "OPTIONS /api/v1/auth/logout", "OPTIONS /api/v1/{proxy+}", "GET /health"])
  api_id    = aws_apigatewayv2_api.public.id
  route_key = each.value
  target    = "integrations/${aws_apigatewayv2_integration.lambda_proxy.id}"
}
resource "aws_apigatewayv2_route" "protected" {
  for_each           = toset(["ANY /api/v1/auth/{proxy+}", "ANY /api/v1/users/{proxy+}", "ANY /api/v1/branches/{proxy+}", "ANY /api/v1/employees/{proxy+}", "ANY /api/v1/attendance/{proxy+}", "ANY /api/v1/overtime/{proxy+}", "GET /api/v1/users", "POST /api/v1/users", "GET /api/v1/branches", "POST /api/v1/branches", "GET /api/v1/employees", "POST /api/v1/employees", "GET /api/v1/attendance", "POST /api/v1/attendance", "GET /api/v1/overtime", "POST /api/v1/overtime"])
  api_id             = aws_apigatewayv2_api.public.id
  route_key          = each.value
  authorization_type = "CUSTOM"
  authorizer_id      = aws_apigatewayv2_authorizer.jwt.id
  target             = "integrations/${aws_apigatewayv2_integration.lambda_proxy.id}"
}
resource "aws_apigatewayv2_stage" "dev" {
  api_id      = aws_apigatewayv2_api.public.id
  name        = "dev"
  auto_deploy = true
  default_route_settings {
    throttling_burst_limit   = 10
    throttling_rate_limit    = 5
    detailed_metrics_enabled = false
  }
  tags = { Environment = "development" }
}

output "api_gateway_base_url" { value = "${aws_apigatewayv2_api.public.api_endpoint}/dev" }
