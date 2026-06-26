output "amqp_secret_arn" {
  description = "ARN do segredo com a URL AMQPS do broker (para ECS secrets)."
  value       = aws_secretsmanager_secret.amqp.arn
}

output "console_url" {
  value = aws_mq_broker.this.instances[0].console_url
}
