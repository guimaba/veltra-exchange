output "alb_url" {
  description = "URL pública do gateway (Flutter Web + API + WS)."
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
