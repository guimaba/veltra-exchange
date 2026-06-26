variable "name" {
  description = "Prefixo dos repositórios (ex.: veltra-dev)."
  type        = string
}

variable "repositories" {
  description = "Lista de nomes de imagem/serviço."
  type        = list(string)
  default     = ["gateway", "matching", "ledger", "marketdata", "audit"]
}
