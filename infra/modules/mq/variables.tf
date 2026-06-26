variable "name" {
  type = string
}

variable "private_subnet_ids" {
  type = list(string)
}

variable "data_sg_id" {
  type = string
}

variable "engine_version" {
  type    = string
  default = "3.13"
}

variable "instance_type" {
  type = string
  # RabbitMQ no Amazon MQ não oferece t3.micro; m7g.medium (Graviton) é o menor.
  default = "mq.m7g.medium"
}

variable "username" {
  type    = string
  default = "veltra"
}
