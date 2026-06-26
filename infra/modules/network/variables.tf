variable "name" {
  description = "Prefixo de nomes dos recursos (ex.: veltra-dev)."
  type        = string
}

variable "region" {
  description = "Região AWS (para nomes de serviço dos VPC endpoints)."
  type        = string
}

variable "vpc_cidr" {
  description = "CIDR da VPC."
  type        = string
  default     = "10.42.0.0/16"
}

variable "enable_nat" {
  description = "Cria NAT Gateway (caro). Por padrão usa VPC Endpoints."
  type        = bool
  default     = false
}

variable "enable_endpoints" {
  description = "Cria VPC Endpoints (ECR/S3/Logs/Secrets) para tasks privadas sem NAT."
  type        = bool
  default     = true
}
