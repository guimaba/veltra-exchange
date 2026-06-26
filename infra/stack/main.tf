terraform {
  required_version = ">= 1.9"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.60"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.6"
    }
  }
}

# ============================================================================
# Stack completo da Veltra Exchange na AWS. Composição dos módulos; recebe
# inputs por ambiente (dev/demo) via Terragrunt. Mapa Compose → AWS:
#   rabbitmq  → Amazon MQ (RabbitMQ)        matching1-3 → 1 task Fargate (single-writer)
#   postgres  → RDS PostgreSQL              gateway     → Fargate + ALB
#   ledger/marketdata → Fargate             audit       → Fargate (+ MariaDB Fargate)
# ============================================================================

locals {
  name = "veltra-${var.env}"
}

module "network" {
  source           = "../modules/network"
  name             = local.name
  region           = var.region
  vpc_cidr         = var.vpc_cidr
  enable_nat       = var.enable_nat
  enable_endpoints = true
}

module "ecr" {
  source = "../modules/ecr"
  name   = local.name
}

module "rds" {
  source             = "../modules/rds"
  name               = local.name
  private_subnet_ids = module.network.private_subnet_ids
  data_sg_id         = module.network.data_sg_id
  instance_class     = var.rds_instance_class
}

module "mq" {
  source             = "../modules/mq"
  name               = local.name
  private_subnet_ids = module.network.private_subnet_ids
  data_sg_id         = module.network.data_sg_id
  instance_type      = var.mq_instance_type
}

# Segredo JWT do gateway (gerado, nunca em variável/state em texto plano).
resource "random_password" "jwt" {
  length  = 48
  special = false
}

resource "aws_secretsmanager_secret" "jwt" {
  name                    = "${local.name}/jwt-secret"
  recovery_window_in_days = 0
}

resource "aws_secretsmanager_secret_version" "jwt" {
  secret_id     = aws_secretsmanager_secret.jwt.id
  secret_string = random_password.jwt.result
}

module "ecs_cluster" {
  source      = "../modules/ecs-cluster"
  name        = local.name
  vpc_id      = module.network.vpc_id
  namespace   = "${var.env}.veltra.local"
  secret_arns = [module.rds.dsn_secret_arn, module.mq.amqp_secret_arn, aws_secretsmanager_secret.jwt.arn]
}

module "alb" {
  source            = "../modules/alb"
  name              = local.name
  vpc_id            = module.network.vpc_id
  public_subnet_ids = module.network.public_subnet_ids
  alb_sg_id         = module.network.alb_sg_id
}

# ---- Atributos comuns aos serviços ----
locals {
  svc_common = {
    name               = local.name
    region             = var.region
    cluster_arn        = module.ecs_cluster.cluster_arn
    namespace_id       = module.ecs_cluster.namespace_id
    execution_role_arn = module.ecs_cluster.execution_role_arn
    task_role_arn      = module.ecs_cluster.task_role_arn
    log_group_name     = module.ecs_cluster.log_group_name
    security_group_id  = module.network.tasks_sg_id
  }
  amqp_secret = { AMQP_URL = module.mq.amqp_secret_arn }
  pg_secret   = { POSTGRES_DSN = module.rds.dsn_secret_arn }
}

# Gateway: borda HTTP/WS, atrás do ALB, em subnet privada.
module "gateway" {
  source             = "../modules/ecs-service"
  service_name       = "gateway"
  image              = "${module.ecr.repository_urls["gateway"]}:${var.image_tag}"
  subnet_ids         = module.network.private_subnet_ids
  container_port     = 8080
  target_group_arn   = module.alb.target_group_arn
  desired_count      = 1
  environment        = { GATEWAY_PORT = "8080" }
  secrets            = merge(local.amqp_secret, local.pg_secret, { VELTRA_JWT_SECRET = aws_secretsmanager_secret.jwt.arn })
  name               = local.svc_common.name
  region             = local.svc_common.region
  cluster_arn        = local.svc_common.cluster_arn
  namespace_id       = local.svc_common.namespace_id
  execution_role_arn = local.svc_common.execution_role_arn
  task_role_arn      = local.svc_common.task_role_arn
  log_group_name     = local.svc_common.log_group_name
  security_group_id  = local.svc_common.security_group_id
}

# Matching engine: SINGLE task (single-writer). O failover Bully + WAL
# compartilhado das 3 réplicas é o mecanismo de HA do Docker Compose; em ECS a
# HA vem do orquestrador reiniciando a task única — evita split-brain e o EFS
# lento no caminho de fsync do WAL.
module "matching" {
  source        = "../modules/ecs-service"
  service_name  = "matching"
  image         = "${module.ecr.repository_urls["matching"]}:${var.image_tag}"
  subnet_ids    = module.network.private_subnet_ids
  desired_count = 1
  cpu           = 512
  memory        = 1024
  environment = {
    NODE_ID        = "1"
    NODE_PORT      = "9101"
    PEERS          = "" # task única → líder por padrão
    PAIRS          = var.pairs
    WAL_DIR        = "/data/matching"
    SNAPSHOT_EVERY = "100"
  }
  secrets            = local.amqp_secret
  name               = local.svc_common.name
  region             = local.svc_common.region
  cluster_arn        = local.svc_common.cluster_arn
  namespace_id       = local.svc_common.namespace_id
  execution_role_arn = local.svc_common.execution_role_arn
  task_role_arn      = local.svc_common.task_role_arn
  log_group_name     = local.svc_common.log_group_name
  security_group_id  = local.svc_common.security_group_id
}

# Ledger: settlement de dupla entrada (stateless → Spot).
module "ledger" {
  source             = "../modules/ecs-service"
  service_name       = "ledger"
  image              = "${module.ecr.repository_urls["ledger"]}:${var.image_tag}"
  subnet_ids         = module.network.private_subnet_ids
  desired_count      = 1
  use_spot           = true
  secrets            = merge(local.amqp_secret, local.pg_secret)
  name               = local.svc_common.name
  region             = local.svc_common.region
  cluster_arn        = local.svc_common.cluster_arn
  namespace_id       = local.svc_common.namespace_id
  execution_role_arn = local.svc_common.execution_role_arn
  task_role_arn      = local.svc_common.task_role_arn
  log_group_name     = local.svc_common.log_group_name
  security_group_id  = local.svc_common.security_group_id
}

# MariaDB (legado): banco do serviço de auditoria. Imagem stock do Docker Hub,
# então roda em subnet pública com IP público para conseguir puxar a imagem
# (sem NAT). Dados efêmeros — a trilha de auditoria é um log, recriável.
# Descoberto via Cloud Map em mariadb.${var.env}.veltra.local:3306.
module "mariadb" {
  source           = "../modules/ecs-service"
  service_name     = "mariadb"
  image            = "mariadb:10.11"
  subnet_ids       = module.network.public_subnet_ids
  assign_public_ip = true
  container_port   = 3306
  desired_count    = 1
  cpu              = 256
  memory           = 512
  environment = {
    MARIADB_ROOT_PASSWORD = "root"
    MARIADB_DATABASE      = "blockchain"
    MARIADB_USER          = "blockchain"
    MARIADB_PASSWORD      = "blockchain"
  }
  name               = local.svc_common.name
  region             = local.svc_common.region
  cluster_arn        = local.svc_common.cluster_arn
  namespace_id       = local.svc_common.namespace_id
  execution_role_arn = local.svc_common.execution_role_arn
  task_role_arn      = local.svc_common.task_role_arn
  log_group_name     = local.svc_common.log_group_name
  security_group_id  = local.svc_common.security_group_id
}

# Audit (legado): consome todos os eventos e persiste a trilha no MariaDB.
# Cria as próprias tabelas no boot (idempotente), então o MariaDB stock basta.
module "audit" {
  source        = "../modules/ecs-service"
  service_name  = "audit"
  image         = "${module.ecr.repository_urls["audit"]}:${var.image_tag}"
  subnet_ids    = module.network.private_subnet_ids
  desired_count = 1
  use_spot      = true
  environment = {
    DB_DSN = "blockchain:blockchain@tcp(mariadb.${var.env}.veltra.local:3306)/blockchain?parseTime=true"
  }
  secrets            = local.amqp_secret
  name               = local.svc_common.name
  region             = local.svc_common.region
  cluster_arn        = local.svc_common.cluster_arn
  namespace_id       = local.svc_common.namespace_id
  execution_role_arn = local.svc_common.execution_role_arn
  task_role_arn      = local.svc_common.task_role_arn
  log_group_name     = local.svc_common.log_group_name
  security_group_id  = local.svc_common.security_group_id
}

# Market data: precisa de internet (CoinGecko) → subnet pública + IP público.
module "marketdata" {
  source             = "../modules/ecs-service"
  service_name       = "marketdata"
  image              = "${module.ecr.repository_urls["marketdata"]}:${var.image_tag}"
  subnet_ids         = module.network.public_subnet_ids
  assign_public_ip   = true
  desired_count      = 1
  use_spot           = true
  environment        = { SEED_LIQUIDITY = "true" }
  secrets            = local.amqp_secret
  name               = local.svc_common.name
  region             = local.svc_common.region
  cluster_arn        = local.svc_common.cluster_arn
  namespace_id       = local.svc_common.namespace_id
  execution_role_arn = local.svc_common.execution_role_arn
  task_role_arn      = local.svc_common.task_role_arn
  log_group_name     = local.svc_common.log_group_name
  security_group_id  = local.svc_common.security_group_id
}
