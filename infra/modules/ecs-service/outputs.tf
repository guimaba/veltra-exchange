output "service_name" {
  value = aws_ecs_service.this.name
}

output "task_definition_arn" {
  value = aws_ecs_task_definition.this.arn
}

output "discovery_name" {
  description = "FQDN interno do serviço no Cloud Map."
  value       = aws_service_discovery_service.this.name
}
