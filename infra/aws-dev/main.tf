# Root de aplicação direta (Terraform puro, sem Terragrunt) para o ambiente dev.
# State LOCAL (efêmero) — adequado para uma demo de hoje que será destruída.
# Provider + default_tags aqui (no fluxo Terragrunt, isto seria gerado).

terraform {
  required_version = ">= 1.9"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.60"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.6"
    }
  }
}

provider "aws" {
  region = "us-east-1"
  default_tags {
    tags = {
      Project    = "veltra-exchange"
      Env        = "dev"
      ManagedBy  = "terraform"
      Owner      = "furb-academic"
      CostCenter = "sistemas-distribuidos"
    }
  }
}

module "veltra" {
  source = "../stack"

  env       = "dev"
  region    = "us-east-1"
  image_tag = "latest"
  # Pares: fiat↔USDT (conversão) + cripto/USDT (negociação).
  pairs      = "USD/USDT-sim,BRL/USDT-sim,EUR/USDT-sim,GBP/USDT-sim,VLT/USDT-sim,BTC/USDT-sim,ETH/USDT-sim,SOL/USDT-sim"
  enable_nat = false
  # RabbitMQ no Amazon MQ não suporta t3.micro; menor instância é m7g.medium.
  mq_instance_type = "mq.m7g.medium"
}

output "alb_url" {
  value = module.veltra.alb_url
}

output "ecr_repository_urls" {
  value = module.veltra.ecr_repository_urls
}

output "mq_console_url" {
  value = module.veltra.mq_console_url
}

output "rds_endpoint" {
  value = module.veltra.rds_endpoint
}
