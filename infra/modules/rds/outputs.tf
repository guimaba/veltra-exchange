output "endpoint" {
  value = aws_db_instance.this.endpoint
}

output "dsn_secret_arn" {
  description = "ARN do segredo com o DSN completo do Postgres (para ECS secrets)."
  value       = aws_secretsmanager_secret.dsn.arn
}
