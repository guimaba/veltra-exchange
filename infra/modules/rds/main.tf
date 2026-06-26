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

# PostgreSQL gerenciado (ledger + auth). Single-AZ + instância pequena para
# manter custo baixo em ambiente acadêmico. A senha NUNCA é passada como
# variável (vazaria no state): é gerada aqui e guardada no Secrets Manager;
# as tasks ECS a leem por ARN.

resource "random_password" "db" {
  length  = 24
  special = false # evita caracteres problemáticos em DSN/URL
}

resource "aws_db_subnet_group" "this" {
  name       = "${var.name}-pg"
  subnet_ids = var.private_subnet_ids
  tags       = { Name = "${var.name}-pg" }
}

resource "aws_db_instance" "this" {
  identifier     = "${var.name}-pg"
  engine         = "postgres"
  engine_version = var.engine_version
  instance_class = var.instance_class

  allocated_storage = var.allocated_storage
  storage_type      = "gp3"
  storage_encrypted = true

  db_name  = var.db_name
  username = var.db_username
  password = random_password.db.result
  port     = 5432

  db_subnet_group_name   = aws_db_subnet_group.this.name
  vpc_security_group_ids = [var.data_sg_id]
  multi_az               = false
  publicly_accessible    = false

  # Ambiente acadêmico: facilita destroy e evita custo de snapshots/backup.
  skip_final_snapshot     = true
  backup_retention_period = 0
  deletion_protection     = false
  apply_immediately       = true

  tags = { Name = "${var.name}-pg" }
}

# DSN completo (formato lib/pq) guardado no Secrets Manager.
resource "aws_secretsmanager_secret" "dsn" {
  name                    = "${var.name}/postgres-dsn"
  recovery_window_in_days = 0 # permite recriar imediatamente em demos
}

resource "aws_secretsmanager_secret_version" "dsn" {
  secret_id = aws_secretsmanager_secret.dsn.id
  secret_string = format(
    "postgres://%s:%s@%s/%s?sslmode=require",
    var.db_username,
    random_password.db.result,
    aws_db_instance.this.endpoint,
    var.db_name,
  )
}
