output "alb_url" {
  description = "URL pública do gateway (Flutter Web + API + WS)."
  value       = "http://${module.alb.dns_name}"
}

# Link direto do front-end (Flutter Web). É a mesma borda do ALB, mas exposto
# como saída própria para quem só quer abrir a interface.
output "frontend_url" {
  description = "URL do front-end Veltra Exchange (Flutter Web)."
  value       = "http://${module.alb.dns_name}"
}

output "ecr_repository_urls" {
  value = module.ecr.repository_urls
}

output "mq_console_url" {
  value = module.mq.console_url
}

output "rds_endpoint" {
  value = module.rds.endpoint
}
