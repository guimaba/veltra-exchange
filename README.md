# ⚡ Veltra Exchange

Exchange spot simulada com matching engine determinístico, ledger de dupla entrada e dados reais de mercado.

**Disciplina de Sistemas Distribuídos — FURB 2026**  
Guilherme Maba · Enzo Gabriel da Rocha · Igor Zafriel Schmidt

---

## Sobre o projeto

A Veltra Exchange é uma plataforma de negociação spot educacional que replica a arquitetura de uma exchange real:

- **Motor de casamento (CLOB)** com prioridade preço-tempo, determinístico e tolerante a falhas
- **Eleição de líder** via algoritmo Bully com failover automático e recuperação via WAL
- **Ledger de dupla entrada** em PostgreSQL com settlement atômico
- **33 pares de trading** com preços reais da [CoinGecko](https://www.coingecko.com)
- **Interface Flutter Web** com tema dark inspirado em exchanges profissionais

---

## Pré-requisitos

| Ferramenta | Versão mínima |
|---|---|
| [Docker Desktop](https://www.docker.com/products/docker-desktop/) | 24+ |
| [Docker Compose](https://docs.docker.com/compose/) | 2.20+ (incluído no Docker Desktop) |

> Dart, Flutter e Go **não precisam** estar instalados localmente — tudo compila dentro dos containers.

---

## Como executar

### 1. Clone o repositório

```bash
git clone https://github.com/guimaba/veltra-exchange.git
cd veltra-exchange
```

### 2. Suba o stack completo

```bash
docker compose up --build
```

> A primeira execução baixa todas as imagens e compila o Flutter (~5–10 min dependendo da conexão). As próximas serão mais rápidas graças ao cache do Docker.

### 3. Acesse a aplicação

| Serviço | URL |
|---|---|
| **Veltra Exchange (UI)** | http://localhost:8080 |
| **API REST** | http://localhost:8080/api |
| **WebSocket** | ws://localhost:8080/ws |
| **RabbitMQ Management** | http://localhost:15672 (admin / admin) |

### 4. Credenciais padrão

O usuário **admin** é criado automaticamente no primeiro boot:

```
Usuário: admin
Senha:   admin
```

Para criar uma conta normal, use a tela de cadastro em http://localhost:8080.

---

## Estrutura do projeto

```
.
├── cmd/
│   ├── gateway/          # Gateway HTTP/WS, autenticação JWT, OMS
│   ├── matching-engine/  # CLOB, Bully election, WAL
│   ├── ledger/           # Settlement de dupla entrada
│   ├── marketdata/       # CoinGecko, candles OHLCV
│   ├── audit/            # Trilha de auditoria
│   └── node/             # Nó da blockchain legada
├── pkg/
│   ├── money/            # Aritmética fixed-point int64 (scale 1e8)
│   ├── exchange/         # Tipos Order, Trade, Pair, Side...
│   ├── matching/         # Engine + WAL
│   ├── orderbook/        # CLOB implementation
│   ├── messaging/        # RabbitMQ (publisher, consumer, envelope)
│   ├── ledger/           # Motor de dupla entrada
│   └── pgstore/          # Pool de conexões PostgreSQL
├── web/                  # Interface Flutter Web
│   └── lib/
│       ├── screens/      # Trading, Market, Wallet, Admin...
│       └── ...
├── docker/
│   ├── Dockerfile.*      # Um por serviço Go
│   ├── postgres/         # Migrations SQL (schemas auth + ledger)
│   └── rabbitmq/         # definitions.json com topologia
└── docker-compose.yml
```

---

## Arquitetura

```
┌──────────────────────────────────────────────────────┐
│                   Flutter Web (UI)                    │
└───────────────────────┬──────────────────────────────┘
                        │ HTTP / WebSocket
┌───────────────────────▼──────────────────────────────┐
│              Gateway (cmd/gateway)                    │
│  REST · JWT auth · OMS pré-trade · Simulador trades  │
└──────────┬────────────────────────────┬──────────────┘
           │ veltra.commands            │ veltra.events
    ┌──────▼──────┐              ┌──────▼──────┐
    │  Matching   │              │   Ledger    │
    │  Engine ×3  │──events──►  │ (PostgreSQL)│
    │ Bully + WAL │              └─────────────┘
    └─────────────┘
           │
    ┌──────▼──────────────────────┐
    │         RabbitMQ            │
    │ veltra.commands/events      │
    │ 6 filas · retry · DLQ       │
    └─────────────────────────────┘
```

### Fluxo de uma ordem (VLT/USDT-sim)

```
UI → POST /api/orders
  → Gateway valida saldo (ledger.available)
  → Publica order.place em veltra.commands
  → Matching Engine (líder) processa, grava WAL, executa
  → Emite trade.executed em veltra.events
  → Ledger consume → 4 postings atômicos no Postgres
  → Gateway consume → atualiza projeção in-memory
  → WebSocket broadcast → UI atualiza em tempo real
```

---

## Serviços Docker

| Container | Porta exposta |
|---|---|
| `blockchain-gateway` (Flutter + Go) | 8080 |
| `blockchain-rabbitmq` | 5672, 15672 |
| `veltra-postgres` | 5432 |
| `blockchain-mariadb` | 3306 |
| `veltra-matching1/2/3` | — (interno) |
| `veltra-ledger` | — (interno) |
| `veltra-marketdata` | — (interno) |

---

## Endpoints principais da API

| Método | Endpoint | Auth | Descrição |
|---|---|---|---|
| POST | `/api/auth/register` | — | Criar conta |
| POST | `/api/auth/login` | — | Login → JWT |
| GET | `/api/balance` | JWT | Saldos do usuário (Postgres) |
| POST | `/api/deposit` | JWT | Depósito simulado (PIX/cartão) |
| POST | `/api/orders` | JWT | Enviar ordem |
| DELETE | `/api/orders/{id}` | JWT | Cancelar ordem |
| GET | `/api/market` | — | 33 moedas com preços ao vivo |
| GET | `/api/market/{sym}/candles` | — | Candles OHLCV |
| GET | `/api/admin/users` | JWT+Admin | Usuários com saldos |
| GET | `/api/admin/stats` | JWT+Admin | KPIs do sistema |

---

## Executar testes unitários

```bash
go test ./...
```

Os pacotes com testes são: `pkg/money`, `pkg/orderbook` e `pkg/matching`.

---

## Tecnologias

| Categoria | Tecnologia |
|---|---|
| Backend | Go 1.25 |
| Frontend | Flutter 3.24 (Web) |
| Broker | RabbitMQ 3.12 |
| Banco principal | PostgreSQL 16 |
| Banco legado | MariaDB 10.11 |
| Containerização | Docker Compose |
| Autenticação | JWT HS256 + bcrypt |
| Market data | CoinGecko API (free) |

---

## Licença

Projeto acadêmico — FURB 2026. Uso educacional.
