output "cluster_id" {
  value = aws_ecs_cluster.this.id
}

output "cluster_arn" {
  value = aws_ecs_cluster.this.arn
}

output "namespace_id" {
  value = aws_service_discovery_private_dns_namespace.this.id
}

output "log_group_name" {
  value = aws_cloudwatch_log_group.this.name
}

output "execution_role_arn" {
  value = aws_iam_role.execution.arn
}

output "task_role_arn" {
  value = aws_iam_role.task.arn
}
