variable "name" {
  type = string
}

variable "vpc_id" {
  type = string
}

variable "namespace" {
  description = "Namespace DNS privado do Cloud Map (ex.: veltra.local)."
  type        = string
  default     = "veltra.local"
}

variable "log_retention_days" {
  type    = number
  default = 7
}

variable "secret_arns" {
  description = "ARNs dos segredos que a execution role pode ler (DSN, AMQP, JWT)."
  type        = list(string)
}
