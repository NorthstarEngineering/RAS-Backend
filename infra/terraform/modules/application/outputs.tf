output "instance_id" { value = aws_instance.this.id }
output "instance_private_ip" { value = aws_instance.this.private_ip }
output "ecr_repository_urls" { value = { for name, repo in aws_ecr_repository.this : name => repo.repository_url } }
