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
  type    = string
  default = "mq.t3.micro"
}

variable "username" {
  type    = string
  default = "veltra"
}
