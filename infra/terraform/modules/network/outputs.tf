output "vpc_id" { value = aws_vpc.this.id }
output "public_subnet_ids" { value = [for key in sort(keys(aws_subnet.public)) : aws_subnet.public[key].id] }
output "private_subnet_ids" { value = [for key in sort(keys(aws_subnet.private)) : aws_subnet.private[key].id] }
output "application_security_group_id" { value = aws_security_group.application.id }
output "load_balancer_security_group_id" { value = aws_security_group.load_balancer.id }
output "vpc_link_security_group_id" { value = aws_security_group.vpc_link.id }
output "private_route_table_id" { value = aws_route_table.private.id }
