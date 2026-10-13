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

resource "aws_eip" "nat" {
  domain = "vpc"
  tags   = { Name = "${var.name}-nat", Environment = "development" }
}
resource "aws_nat_gateway" "this" {
  allocation_id = aws_eip.nat.id
  subnet_id     = aws_subnet.public["0"].id
  depends_on    = [aws_internet_gateway.this]
  tags          = { Name = "${var.name}-nat", Environment = "development" }
}
resource "aws_route_table" "private" {
  vpc_id = aws_vpc.this.id
  route {
    cidr_block     = "0.0.0.0/0"
    nat_gateway_id = aws_nat_gateway.this.id
  }
  tags = { Name = "${var.name}-private", Environment = "development" }
}
resource "aws_route_table_association" "private" {
  for_each       = aws_subnet.private
  subnet_id      = each.value.id
  route_table_id = aws_route_table.private.id
}

resource "aws_security_group" "application" {
  name        = "${var.name}-application"
  description = "Private API workloads. Incoming API traffic is accepted only from the internal ALB."
  vpc_id      = aws_vpc.this.id
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
  tags = { Name = "${var.name}-application", Environment = "development" }
}
resource "aws_security_group" "load_balancer" {
  name        = "${var.name}-internal-alb"
  description = "Private ALB accepts HTTP only from the API Gateway VPC Link."
  vpc_id      = aws_vpc.this.id
  egress      = []
  tags        = { Name = "${var.name}-internal-alb", Environment = "development" }
}
resource "aws_security_group" "vpc_link" {
  name        = "${var.name}-api-vpc-link"
  description = "API Gateway VPC Link to internal ALB."
  vpc_id      = aws_vpc.this.id
  egress      = []
  tags        = { Name = "${var.name}-api-vpc-link", Environment = "development" }
}

resource "aws_vpc_security_group_ingress_rule" "application_from_alb" {
  security_group_id            = aws_security_group.application.id
  referenced_security_group_id = aws_security_group.load_balancer.id
  ip_protocol                  = "tcp"
  from_port                    = 9001
  to_port                      = 9003
  description                  = "API listener ports from internal ALB only"
}
resource "aws_vpc_security_group_ingress_rule" "alb_from_vpc_link" {
  security_group_id            = aws_security_group.load_balancer.id
  referenced_security_group_id = aws_security_group.vpc_link.id
  ip_protocol                  = "tcp"
  from_port                    = 80
  to_port                      = 80
  description                  = "HTTP from API Gateway VPC Link only"
}
resource "aws_vpc_security_group_egress_rule" "alb_to_application" {
  security_group_id            = aws_security_group.load_balancer.id
  referenced_security_group_id = aws_security_group.application.id
  ip_protocol                  = "tcp"
  from_port                    = 9001
  to_port                      = 9003
  description                  = "Forward to API containers"
}
resource "aws_vpc_security_group_egress_rule" "vpc_link_to_alb" {
  security_group_id            = aws_security_group.vpc_link.id
  referenced_security_group_id = aws_security_group.load_balancer.id
  ip_protocol                  = "tcp"
  from_port                    = 80
  to_port                      = 80
  description                  = "VPC Link to internal ALB"
}
