# Infraestrutura — Veltra Exchange na AWS (Terraform + Terragrunt)

IaC do plano técnico §4.1/§6. **Terraform modular** (reuso via módulos) com uma
**camada Terragrunt fina** (`live/dev`, `live/demo`) que reusa o mesmo stack
variando só os inputs — DRY sem duplicar HCL.

## Mapa Docker Compose → AWS

| Compose | AWS | Notas |
|---|---|---|
| `rabbitmq` | **Amazon MQ (RabbitMQ)** | `SINGLE_INSTANCE` na demo; `CLUSTER_MULTI_AZ` em prod |
| `postgres` | **RDS PostgreSQL** | single-AZ `db.t4g.micro`, encryption at rest |
| `gateway` | **Fargate + ALB** | borda HTTP/WS; ALB faz WebSocket nativo |
| `matching1-3` | **1 task Fargate** | single-writer (ver decisão abaixo) |
| `ledger` | **Fargate (Spot)** | stateless |
| `marketdata` | **Fargate (Spot, subnet pública)** | precisa de internet (CoinGecko) |
| `audit` | — | fora do cloud: depende do MariaDB legado (não faz parte do plano §6) |
| `mariadb` + `node1-3` | — | blockchain legada, fora do escopo cloud |

## Estrutura

```
infra/
  terragrunt.hcl          # raiz: estado remoto S3, provider gerado, default_tags
  live/
    dev/terragrunt.hcl    # par único, footprint mínimo
    demo/terragrunt.hcl   # catálogo de pares, RDS um degrau acima
  stack/                  # composição: chama todos os módulos (1 root, N ambientes)
  modules/
    network/      # VPC, 2 AZs, subnets pub/priv, SGs, VPC Endpoints (sem NAT)
    ecr/          # repositórios das 5 imagens + lifecycle policy
    rds/          # PostgreSQL + senha no Secrets Manager
    mq/           # Amazon MQ RabbitMQ + URL no Secrets Manager
    ecs-cluster/  # cluster Fargate, Cloud Map, log group, roles IAM least-privilege
    ecs-service/  # MÓDULO GENÉRICO de serviço (reusado 4×)
    alb/          # ALB + target group + listener
```

## Boas práticas aplicadas

- **Secrets nunca no state**: senha do RDS e credenciais do MQ são geradas via
  `random_password` e guardadas no **Secrets Manager**; as tasks ECS as recebem
  por **ARN** (campo `secrets`, não `environment`). O JWT do gateway idem.
- **IAM least-privilege**: execution role lê **apenas** os ARNs dos segredos da
  Veltra (não `*`); task role mínima (a app não chama nenhuma API AWS).
- **Rede privada**: RDS, MQ e a maioria das tasks ficam em subnets privadas;
  só o ALB (e o marketdata, por causa da CoinGecko) são expostos.
- **Sem NAT por padrão**: saída para ECR/S3/Logs/Secrets via **VPC Endpoints**
  (mais barato e seguro que NAT Gateway). `enable_nat=true` se precisar.
- **Estado remoto**: S3 criptografado + **lock nativo** (`use_lockfile`, dispensa
  DynamoDB; requer Terraform ≥ 1.10).
- **Custo**: single-AZ, instâncias menores, Fargate **Spot** nos stateless,
  `default_tags` para rastreio, tudo `terraform destroy`-able (`force_delete`,
  `skip_final_snapshot`, `recovery_window_in_days=0`).

## Decisão consciente: matching em ECS

No Compose, as **3 réplicas** do matching usam eleição **Bully** + **WAL em volume
compartilhado** como mecanismo de HA. Em ECS isso não traduz bem: EFS no caminho
de `fsync` do WAL adiciona latência (mata o alvo de ms do plano §1) e a eleição
sem fencing pode dar split-brain. **Na nuvem rodamos 1 task** (single-writer — o
que o motor exige de qualquer forma) e a HA vem do ECS reiniciando a task,
recuperando do WAL local. As 3 réplicas seguem como demonstração do Bully no
Compose.

## Custo estimado (se aplicado, ~24/7)

Amazon MQ `mq.t3.micro` (~US$15-20) + RDS `db.t4g.micro` (~US$12-15) + ALB
(~US$16) + Fargate (varia). **Sem NAT** (economia de ~US$32/mês). Total da ordem
de ~US$50-70/mês se ficar ligado — **destrua após a demo**.

## Uso

A entrega acadêmica é **`validate` + `plan`** (não precisa subir — evita custo):

```bash
# Pré-requisitos: criar o bucket de state uma vez (nome global único):
aws s3 mb s3://veltra-exchange-tfstate --region us-east-1
aws s3api put-bucket-versioning --bucket veltra-exchange-tfstate \
  --versioning-configuration Status=Enabled

cd infra/live/dev
terragrunt init
terragrunt plan        # mostra os recursos a criar — a prova de boas práticas
# terragrunt apply     # só se for subir de verdade
# terragrunt destroy   # destrói tudo após a demo
```

Sem Terragrunt instalado, valide os módulos com Terraform puro:

```bash
cd infra/stack && terraform init -backend=false && terraform validate
```

## Pipeline de deploy (T6.3 — resumo)

Build das imagens → push ECR → atualizar `image_tag` → `terragrunt apply`:

```bash
aws ecr get-login-password | docker login --username AWS --password-stdin <acct>.dkr.ecr.us-east-1.amazonaws.com
for s in gateway matching ledger marketdata; do
  docker build -f docker/Dockerfile.$s -t <acct>.dkr.ecr.us-east-1.amazonaws.com/veltra-dev/$s:latest .
  docker push <acct>.dkr.ecr.us-east-1.amazonaws.com/veltra-dev/$s:latest
done
```
