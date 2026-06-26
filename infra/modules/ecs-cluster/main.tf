terraform {
  required_version = ">= 1.9"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.60"
    }
  }
}

resource "aws_ecs_cluster" "this" {
  name = var.name
  setting {
    name  = "containerInsights"
    value = "disabled" # custo extra; habilite se quiser métricas detalhadas
  }
}

# Service Discovery (Cloud Map): dá DNS estável às tasks de IP dinâmico —
# necessário para o RPC de eleição Bully entre as réplicas do matching e para
# o gateway resolver os serviços internos.
resource "aws_service_discovery_private_dns_namespace" "this" {
  name = var.namespace
  vpc  = var.vpc_id
}

resource "aws_cloudwatch_log_group" "this" {
  name              = "/ecs/${var.name}"
  retention_in_days = var.log_retention_days
}

# --- IAM: execution role (puxa imagem, lê segredos, escreve logs) ---
data "aws_iam_policy_document" "ecs_assume" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["ecs-tasks.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "execution" {
  name               = "${var.name}-exec"
  assume_role_policy = data.aws_iam_policy_document.ecs_assume.json
}

resource "aws_iam_role_policy_attachment" "execution_managed" {
  role       = aws_iam_role.execution.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

# Least-privilege: leitura SÓ dos segredos da Veltra (não "*").
data "aws_iam_policy_document" "read_secrets" {
  statement {
    actions   = ["secretsmanager:GetSecretValue"]
    resources = var.secret_arns
  }
}

resource "aws_iam_role_policy" "execution_secrets" {
  name   = "${var.name}-read-secrets"
  role   = aws_iam_role.execution.id
  policy = data.aws_iam_policy_document.read_secrets.json
}

# --- IAM: task role (permissões da aplicação em runtime) ---
# Os serviços não chamam nenhuma API AWS hoje → role mínima/vazia.
resource "aws_iam_role" "task" {
  name               = "${var.name}-task"
  assume_role_policy = data.aws_iam_policy_document.ecs_assume.json
}
