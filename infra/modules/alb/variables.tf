variable "name" {
  type = string
}

variable "vpc_id" {
  type = string
}

variable "public_subnet_ids" {
  type = list(string)
}

variable "alb_sg_id" {
  type = string
}

variable "target_port" {
  type    = number
  default = 8080
}

variable "health_path" {
  type    = string
  default = "/api/health"
}
