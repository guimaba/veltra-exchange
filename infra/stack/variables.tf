variable "env" {
  description = "Ambiente (dev | demo)."
  type        = string
}

variable "region" {
  description = "Região AWS."
  type        = string
  default     = "us-east-1"
}

variable "vpc_cidr" {
  type    = string
  default = "10.42.0.0/16"
}

variable "enable_nat" {
  description = "NAT Gateway (caro). Padrão usa VPC Endpoints."
  type        = bool
  default     = false
}

variable "image_tag" {
  description = "Tag das imagens no ECR."
  type        = string
  default     = "latest"
}

variable "pairs" {
  description = "Pares negociados (CSV) para o matching engine."
  type        = string
  default     = "VLT/USDT-sim"
}

variable "rds_instance_class" {
  type    = string
  default = "db.t4g.micro"
}

variable "mq_instance_type" {
  type    = string
  default = "mq.t3.micro"
}
