variable "name" {
  description = "Prefixo do ambiente (ex.: veltra-dev)."
  type        = string
}

variable "service_name" {
  description = "Nome do serviço (gateway, matching, ledger, ...)."
  type        = string
}

variable "region" {
  type = string
}

variable "image" {
  description = "URI da imagem no ECR (com tag)."
  type        = string
}

variable "cluster_arn" {
  type = string
}

variable "namespace_id" {
  type = string
}

variable "execution_role_arn" {
  type = string
}

variable "task_role_arn" {
  type = string
}

variable "log_group_name" {
  type = string
}

variable "subnet_ids" {
  type = list(string)
}

variable "security_group_id" {
  type = string
}

variable "cpu" {
  type    = number
  default = 256
}

variable "memory" {
  type    = number
  default = 512
}

variable "desired_count" {
  type    = number
  default = 1
}

variable "container_port" {
  description = "Porta exposta (0 = nenhuma)."
  type        = number
  default     = 0
}

variable "environment" {
  type    = map(string)
  default = {}
}

variable "secrets" {
  description = "Mapa nome -> ARN do segredo (Secrets Manager)."
  type        = map(string)
  default     = {}
}

variable "target_group_arn" {
  description = "ARN do target group do ALB (vazio = sem ALB)."
  type        = string
  default     = ""
}

variable "use_spot" {
  type    = bool
  default = false
}

variable "assign_public_ip" {
  description = "IP público (necessário sem NAT para serviços que acessam a internet)."
  type        = bool
  default     = false
}
