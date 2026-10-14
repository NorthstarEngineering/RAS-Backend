output "application_instance_id" { value = module.application.instance_id }
output "database_endpoint" { value = module.database.endpoint }
output "database_port" { value = module.database.port }
output "database_name" { value = module.database.database_name }
output "database_master_secret_arn" { value = module.database.master_secret_arn }
output "ecr_repository_urls" { value = module.application.ecr_repository_urls }
