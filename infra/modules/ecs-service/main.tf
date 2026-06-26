terraform {
  required_version = ">= 1.9"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.60"
    }
  }
}

# Módulo GENÉRICO de serviço Fargate. Uma instância por componente da Veltra
# (gateway, matching, ledger, marketdata, audit), parametrizado por imagem,
# env, secrets, réplicas e (opcional) target group do ALB. Evita repetir a
# definição de task/serviço cinco vezes.

# DNS interno estável via Cloud Map (matching precisa para o RPC de eleição).
resource "aws_service_discovery_service" "this" {
  name = var.service_name
  dns_config {
    namespace_id = var.namespace_id
    dns_records {
      type = "A"
      ttl  = 10
    }
    routing_policy = "MULTIVALUE"
  }
  health_check_custom_config {
    failure_threshold = 1
  }
}

resource "aws_ecs_task_definition" "this" {
  family                   = "${var.name}-${var.service_name}"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = var.cpu
  memory                   = var.memory
  execution_role_arn       = var.execution_role_arn
  task_role_arn            = var.task_role_arn

  container_definitions = jsonencode([{
    name      = var.service_name
    image     = var.image
    essential = true

    portMappings = var.container_port > 0 ? [{
      containerPort = var.container_port
      protocol      = "tcp"
    }] : []

    environment = [for k, v in var.environment : { name = k, value = v }]
    # Segredos injetados por ARN do Secrets Manager — NUNCA em texto no state.
    secrets = [for k, arn in var.secrets : { name = k, valueFrom = arn }]

    logConfiguration = {
      logDriver = "awslogs"
      options = {
        "awslogs-group"         = var.log_group_name
        "awslogs-region"        = var.region
        "awslogs-stream-prefix" = var.service_name
      }
    }
  }])
}

resource "aws_ecs_service" "this" {
  name            = var.service_name
  cluster         = var.cluster_arn
  task_definition = aws_ecs_task_definition.this.arn
  desired_count   = var.desired_count
  propagate_tags  = "SERVICE"

  # Fargate Spot corta ~70% do custo nos serviços stateless. NÃO usar no
  # matching (interrupção + WAL = risco de estado).
  capacity_provider_strategy {
    capacity_provider = var.use_spot ? "FARGATE_SPOT" : "FARGATE"
    weight            = 1
  }

  network_configuration {
    subnets          = var.subnet_ids
    security_groups  = [var.security_group_id]
    assign_public_ip = var.assign_public_ip
  }

  service_registries {
    registry_arn = aws_service_discovery_service.this.arn
  }

  # Anexa ao ALB só quando há target group (ex.: gateway).
  dynamic "load_balancer" {
    for_each = var.target_group_arn == "" ? [] : [1]
    content {
      target_group_arn = var.target_group_arn
      container_name   = var.service_name
      container_port   = var.container_port
    }
  }

  # matching é single-writer (desired_count=1): permite parar a task velha
  # antes de subir a nova (evita dois líderes disputando o WAL).
  deployment_minimum_healthy_percent = var.desired_count == 1 ? 0 : 100
  deployment_maximum_percent         = var.desired_count == 1 ? 100 : 200
}
