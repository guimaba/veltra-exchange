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
  default = "16"
}

variable "instance_class" {
  type    = string
  default = "db.t4g.micro"
}

variable "allocated_storage" {
  type    = number
  default = 20
}

variable "db_name" {
  type    = string
  default = "veltra"
}

variable "db_username" {
  type    = string
  default = "veltra"
}
