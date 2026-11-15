resource "aws_vpc" "this" {
  cidr_block           = var.vpc_cidr
  enable_dns_support   = true
  enable_dns_hostnames = true
  tags                 = { Name = "${var.name}-vpc", Environment = "development" }
}

resource "aws_internet_gateway" "this" {
  vpc_id = aws_vpc.this.id
  tags   = { Name = "${var.name}-igw", Environment = "development" }
}

locals {
  public_subnets  = { for i, cidr in var.public_subnet_cidrs : tostring(i) => cidr }
  private_subnets = { for i, cidr in var.private_subnet_cidrs : tostring(i) => cidr }
}

resource "aws_subnet" "public" {
  for_each                = local.public_subnets
  vpc_id                  = aws_vpc.this.id
  cidr_block              = each.value
  availability_zone       = var.availability_zones[tonumber(each.key)]
  map_public_ip_on_launch = true
  tags                    = { Name = "${var.name}-public-${each.key}", Environment = "development", Tier = "public" }
}

resource "aws_subnet" "private" {
  for_each          = local.private_subnets
  vpc_id            = aws_vpc.this.id
  cidr_block        = each.value
  availability_zone = var.availability_zones[tonumber(each.key)]
  tags              = { Name = "${var.name}-private-${each.key}", Environment = "development", Tier = "private" }
}

resource "aws_route_table" "public" {
  vpc_id = aws_vpc.this.id
  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.this.id
  }
  tags = { Name = "${var.name}-public", Environment = "development" }
}
resource "aws_route_table_association" "public" {
  for_each       = aws_subnet.public
  subnet_id      = each.value.id
  route_table_id = aws_route_table.public.id
}

resource "aws_security_group" "application" {
  name        = "${var.name}-application"
  description = "API containers accept traffic only from the API Gateway proxy Lambda."
  vpc_id      = aws_vpc.this.id
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
  tags = { Name = "${var.name}-application", Environment = "development" }
}
resource "aws_security_group" "api_proxy" {
  name        = "${var.name}-api-proxy"
  description = "VPC-attached Lambda proxy to the application host."
  vpc_id      = aws_vpc.this.id
  egress {
    from_port       = 9001
    to_port         = 9003
    protocol        = "tcp"
    security_groups = [aws_security_group.application.id]
    description     = "Proxy Lambda to application host API ports"
  }
  tags = { Name = "${var.name}-api-proxy", Environment = "development" }
}

resource "aws_vpc_security_group_ingress_rule" "application_from_proxy" {
  security_group_id            = aws_security_group.application.id
  referenced_security_group_id = aws_security_group.api_proxy.id
  ip_protocol                  = "tcp"
  from_port                    = 9001
  to_port                      = 9003
  description                  = "API listener ports from API Gateway proxy Lambda only"
}
