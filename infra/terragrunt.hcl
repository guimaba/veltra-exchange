# Configuração raiz do Terragrunt (DRY). Cada ambiente em live/<env> faz
# `include` deste arquivo e reusa o MESMO módulo infra/stack, variando só os
# inputs. Aqui ficam: estado remoto (S3), geração do provider e tags padrão.

locals {
  region       = "us-east-1"
  project      = "veltra-exchange"
  state_bucket = "veltra-exchange-tfstate" # deve ser globalmente único (ajuste se colidir)
}

# Estado remoto em S3 com lock nativo (use_lockfile dispensa DynamoDB; requer
# Terraform >= 1.10 / provider AWS >= 5.x). Criptografado e versionado.
remote_state {
  backend = "s3"
  generate = {
    path      = "backend.tf"
    if_exists = "overwrite_terragrunt"
  }
  config = {
    bucket       = local.state_bucket
    key          = "${path_relative_to_include()}/terraform.tfstate"
    region       = local.region
    encrypt      = true
    use_lockfile = true
  }
}

# Provider gerado para todos os ambientes, com default_tags (rastreio de custo).
generate "provider" {
  path      = "provider.tf"
  if_exists = "overwrite_terragrunt"
  contents  = <<EOF
provider "aws" {
  region = "${local.region}"
  default_tags {
    tags = {
      Project    = "${local.project}"
      ManagedBy  = "terragrunt"
      Owner      = "furb-academic"
      CostCenter = "sistemas-distribuidos"
    }
  }
}
EOF
}

# Inputs comuns a todos os ambientes (sobrescritos por live/<env>).
inputs = {
  region = local.region
}
