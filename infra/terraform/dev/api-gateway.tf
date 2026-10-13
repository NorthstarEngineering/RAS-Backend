resource "aws_cloudwatch_log_group" "api_access" {
  name              = "/aws/apigateway/ras-backend-dev"
  retention_in_days = 14
  tags              = { Environment = "development" }
}

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

resource "aws_apigatewayv2_vpc_link" "private" {
  name               = "ras-backend-dev"
  security_group_ids = [module.network.vpc_link_security_group_id]
  subnet_ids         = module.network.private_subnet_ids
  tags               = { Environment = "development" }
}

resource "aws_lb" "internal" {
  name               = "ras-backend-dev"
  internal           = true
  load_balancer_type = "application"
  subnets            = module.network.private_subnet_ids
  security_groups    = [module.network.load_balancer_security_group_id]
  idle_timeout       = 60
  tags               = { Environment = "development" }
}

resource "aws_lb_target_group" "auth" {
  name                 = "ras-dev-auth"
  port                 = 9001
  protocol             = "HTTP"
  vpc_id               = module.network.vpc_id
  target_type          = "instance"
  deregistration_delay = 15
  health_check {
    path                = "/health"
    matcher             = "200"
    interval            = 20
    timeout             = 5
    healthy_threshold   = 2
    unhealthy_threshold = 3
  }
  tags = { Environment = "development" }
}
resource "aws_lb_target_group" "branch" {
  name                 = "ras-dev-branch"
  port                 = 9002
  protocol             = "HTTP"
  vpc_id               = module.network.vpc_id
  target_type          = "instance"
  deregistration_delay = 15
  health_check {
    path                = "/health"
    matcher             = "200"
    interval            = 20
    timeout             = 5
    healthy_threshold   = 2
    unhealthy_threshold = 3
  }
  tags = { Environment = "development" }
}
resource "aws_lb_target_group" "attendance" {
  name                 = "ras-dev-attendance"
  port                 = 9003
  protocol             = "HTTP"
  vpc_id               = module.network.vpc_id
  target_type          = "instance"
  deregistration_delay = 15
  health_check {
    path                = "/health"
    matcher             = "200"
    interval            = 20
    timeout             = 5
    healthy_threshold   = 2
    unhealthy_threshold = 3
  }
  tags = { Environment = "development" }
}

resource "aws_lb_listener" "http" {
  load_balancer_arn = aws_lb.internal.arn
  port              = 80
  protocol          = "HTTP"
  default_action {
    type = "fixed-response"
    fixed_response {
      content_type = "application/json"
      message_body = "{\"error\":\"not found\"}"
      status_code  = "404"
    }
  }
}
resource "aws_lb_listener_rule" "auth" {
  listener_arn = aws_lb_listener.http.arn
  priority     = 10
  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.auth.arn
  }
  condition {
    path_pattern { values = ["/api/v1/auth", "/api/v1/auth/*", "/api/v1/users", "/api/v1/users/*"] }
  }
}
resource "aws_lb_listener_rule" "branch" {
  listener_arn = aws_lb_listener.http.arn
  priority     = 20
  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.branch.arn
  }
  condition {
    path_pattern { values = ["/api/v1/branches", "/api/v1/branches/*"] }
  }
}
resource "aws_lb_listener_rule" "attendance" {
  listener_arn = aws_lb_listener.http.arn
  priority     = 30
  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.attendance.arn
  }
  condition {
    path_pattern { values = ["/api/v1/employees", "/api/v1/employees/*", "/api/v1/attendance", "/api/v1/attendance/*", "/api/v1/overtime", "/api/v1/overtime/*"] }
  }
}
resource "aws_lb_listener_rule" "auth_health" {
  listener_arn = aws_lb_listener.http.arn
  priority     = 40
  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.auth.arn
  }
  condition {
    path_pattern { values = ["/health"] }
  }
}

resource "aws_lb_target_group_attachment" "auth" {
  target_group_arn = aws_lb_target_group.auth.arn
  target_id        = module.application.instance_id
}
resource "aws_lb_target_group_attachment" "branch" {
  target_group_arn = aws_lb_target_group.branch.arn
  target_id        = module.application.instance_id
}
resource "aws_lb_target_group_attachment" "attendance" {
  target_group_arn = aws_lb_target_group.attendance.arn
  target_id        = module.application.instance_id
}

resource "aws_iam_role" "authorizer" {
  name               = "ras-backend-dev-jwt-authorizer"
  assume_role_policy = jsonencode({ Version = "2012-10-17", Statement = [{ Effect = "Allow", Principal = { Service = "lambda.amazonaws.com" }, Action = "sts:AssumeRole" }] })
  tags               = { Environment = "development" }
}
resource "aws_cloudwatch_log_group" "authorizer" {
  name              = "/aws/lambda/ras-backend-dev-jwt-authorizer"
  retention_in_days = 14
  tags              = { Environment = "development" }
}
resource "aws_iam_role_policy" "authorizer_logs" {
  name = "write-authorizer-logs"
  role = aws_iam_role.authorizer.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = ["logs:CreateLogStream", "logs:PutLogEvents"]
      Resource = ["${aws_cloudwatch_log_group.authorizer.arn}:*"]
    }]
  })
}
resource "aws_iam_role_policy" "authorizer_secret" {
  name   = "read-auth-signing-secret"
  role   = aws_iam_role.authorizer.id
  policy = jsonencode({ Version = "2012-10-17", Statement = [{ Effect = "Allow", Action = ["secretsmanager:GetSecretValue"], Resource = [var.authorizer_jwt_secret_arn] }] })
}
data "archive_file" "authorizer" {
  type        = "zip"
  source_file = "${path.module}/jwt-authorizer/index.mjs"
  output_path = "${path.module}/jwt-authorizer.zip"
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
  depends_on       = [aws_iam_role_policy.authorizer_logs, aws_iam_role_policy.authorizer_secret, aws_cloudwatch_log_group.authorizer]
  environment { variables = { JWT_SECRET_ARN = var.authorizer_jwt_secret_arn } }
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

resource "aws_apigatewayv2_integration" "private_alb" {
  api_id                 = aws_apigatewayv2_api.public.id
  integration_type       = "HTTP_PROXY"
  integration_method     = "ANY"
  integration_uri        = aws_lb_listener.http.arn
  connection_type        = "VPC_LINK"
  connection_id          = aws_apigatewayv2_vpc_link.private.id
  payload_format_version = "1.0"
  request_parameters     = { "overwrite:path" = "$request.path" }
  timeout_milliseconds   = 30000
}

resource "aws_apigatewayv2_route" "public_auth" {
  for_each  = toset(["POST /api/v1/auth/login", "POST /api/v1/auth/refresh", "POST /api/v1/auth/logout", "GET /health"])
  api_id    = aws_apigatewayv2_api.public.id
  route_key = each.value
  target    = "integrations/${aws_apigatewayv2_integration.private_alb.id}"
}
resource "aws_apigatewayv2_route" "protected" {
  for_each           = toset(["ANY /api/v1/auth/{proxy+}", "ANY /api/v1/users/{proxy+}", "ANY /api/v1/branches/{proxy+}", "ANY /api/v1/employees/{proxy+}", "ANY /api/v1/attendance/{proxy+}", "ANY /api/v1/overtime/{proxy+}", "GET /api/v1/users", "POST /api/v1/users", "GET /api/v1/branches", "POST /api/v1/branches", "GET /api/v1/employees", "POST /api/v1/employees", "GET /api/v1/attendance", "POST /api/v1/attendance", "GET /api/v1/overtime", "POST /api/v1/overtime"])
  api_id             = aws_apigatewayv2_api.public.id
  route_key          = each.value
  authorization_type = "CUSTOM"
  authorizer_id      = aws_apigatewayv2_authorizer.jwt.id
  target             = "integrations/${aws_apigatewayv2_integration.private_alb.id}"
}
resource "aws_apigatewayv2_stage" "dev" {
  api_id      = aws_apigatewayv2_api.public.id
  name        = "dev"
  auto_deploy = true
  access_log_settings {
    destination_arn = aws_cloudwatch_log_group.api_access.arn
    format          = jsonencode({ requestId = "$context.requestId", routeKey = "$context.routeKey", status = "$context.status", sourceIp = "$context.identity.sourceIp", responseLength = "$context.responseLength", integrationError = "$context.integrationErrorMessage" })
  }
  default_route_settings {
    throttling_burst_limit   = 50
    throttling_rate_limit    = 25
    detailed_metrics_enabled = true
  }
  tags = { Environment = "development" }
}

output "api_gateway_base_url" { value = "${aws_apigatewayv2_api.public.api_endpoint}/dev" }
output "internal_alb_dns" { value = aws_lb.internal.dns_name }
