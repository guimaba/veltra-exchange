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

# Amazon MQ for RabbitMQ — backbone de eventos (plano §4.4: "log de eventos
# sobre Amazon MQ"). SINGLE_INSTANCE para custo baixo na demo; em produção
# usar CLUSTER_MULTI_AZ. Credenciais geradas e guardadas no Secrets Manager.

resource "random_password" "mq" {
  length  = 24
  special = false
}

resource "aws_mq_broker" "this" {
  broker_name         = "${var.name}-rabbit"
  engine_type         = "RabbitMQ"
  engine_version      = var.engine_version
  host_instance_type  = var.instance_type
  deployment_mode     = "SINGLE_INSTANCE"
  publicly_accessible = false

  subnet_ids      = [var.private_subnet_ids[0]] # SINGLE_INSTANCE usa 1 subnet
  security_groups = [var.data_sg_id]

  user {
    username = var.username
    password = random_password.mq.result
  }

  logs {
    general = true
  }
}

# URL AMQPS (TLS, porta 5671) montada a partir do endpoint do broker.
resource "aws_secretsmanager_secret" "amqp" {
  name                    = "${var.name}/amqp-url"
  recovery_window_in_days = 0
}

resource "aws_secretsmanager_secret_version" "amqp" {
  secret_id = aws_secretsmanager_secret.amqp.id
  # Ex.: amqps://user:pass@b-xxxx.mq.region.amazonaws.com:5671
  secret_string = format(
    "amqps://%s:%s@%s",
    var.username,
    random_password.mq.result,
    replace(tolist(aws_mq_broker.this.instances[0].endpoints)[0], "amqp+ssl://", ""),
  )
}
