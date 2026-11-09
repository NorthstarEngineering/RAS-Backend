output "endpoint" { value = aws_db_instance.this.address }
output "port" { value = aws_db_instance.this.port }
output "database_name" { value = aws_db_instance.this.db_name }
output "master_password_parameter_name" { value = aws_ssm_parameter.master_password.name }
output "master_password_parameter_arn" { value = aws_ssm_parameter.master_password.arn }
output "security_group_id" { value = aws_security_group.database.id }
