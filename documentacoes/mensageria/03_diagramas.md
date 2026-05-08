# Diagramas — Etapas 1 e 2

Diagramas em **Mermaid** (renderizam nativamente no GitHub e na maioria dos editores Markdown).

---

## 1. Diagrama de Componentes (visão alto nível)

Mostra os blocos arquiteturais da solução e como se relacionam.

```mermaid
graph TB
    subgraph BROWSER["Navegador do usuário"]
        FLUTTER["Flutter Web<br/>(SPA estática)"]
    end

    subgraph DOCKER["Ambiente Docker (1 comando para subir)"]
        GATEWAY["Gateway HTTP/WS (Go)<br/>:8080<br/>- REST API<br/>- WebSocket<br/>- Static files"]

        subgraph BLOCKCHAIN["Rede Blockchain (3 nós)"]
            N1["Nó 1<br/>:8001"]
            N2["Nó 2<br/>:8002"]
            N3["Nó 3 (Líder)<br/>:8003"]
        end

        RABBIT["RabbitMQ<br/>:5672 / :15672"]
        DB[("MariaDB<br/>:3306")]
        AUDIT["Serviço de<br/>Auditoria (Go)"]
    end

    FLUTTER -->|"HTTP REST<br/>(comandos)"| GATEWAY
    FLUTTER <-.->|"WebSocket<br/>(eventos em tempo real)"| GATEWAY

    GATEWAY -->|"AMQP publish<br/>(commands)"| RABBIT
    GATEWAY <-.->|"AMQP consume<br/>(events)"| RABBIT

    RABBIT <-.->|"AMQP"| N1
    RABBIT <-.->|"AMQP"| N2
    RABBIT <-.->|"AMQP"| N3

    RABBIT <-.->|"AMQP consume"| AUDIT

    N1 <-->|"RPC<br/>(Bully + sync)"| N2
    N2 <-->|"RPC"| N3
    N3 <-->|"RPC"| N1

    N1 --> DB
    N2 --> DB
    N3 --> DB
    AUDIT --> DB

    style FLUTTER fill:#42a5f5,color:#fff
    style GATEWAY fill:#66bb6a,color:#fff
    style RABBIT fill:#ff7043,color:#fff
    style DB fill:#8d6e63,color:#fff
    style N3 fill:#fdd835
```

**Legenda:**
- Linha cheia = comunicação síncrona (HTTP, RPC).
- Linha tracejada = comunicação assíncrona (AMQP, WebSocket).
- Nó destacado em amarelo = líder atual da blockchain.

---

## 2. Topologia RabbitMQ — Exchanges, Filas e Bindings

Mostra a estrutura interna do broker.

```mermaid
graph LR
    subgraph PUBLISHERS["Publishers"]
        GW_PUB["Gateway"]
        LEADER_PUB["Nó Líder"]
    end

    subgraph EXCHANGES["Exchanges"]
        EX_CMD{"blockchain.commands<br/><i>direct</i>"}
        EX_EVT{"blockchain.events<br/><i>topic</i>"}
        EX_DLX{"blockchain.dlx<br/><i>fanout</i>"}
    end

    subgraph QUEUES["Queues"]
        Q_LEADER["q.leader.commands<br/>TTL: 60s<br/>DLX: blockchain.dlx"]
        Q_GW["q.gateway.events"]
        Q_AUDIT["q.audit.events"]
        Q_DLQ["q.dlq"]
    end

    subgraph CONSUMERS["Consumers"]
        LEADER_CON["Nó Líder"]
        GW_CON["Gateway"]
        AUDIT_CON["Auditoria"]
        DLQ_VIEW["Inspeção / Flutter"]
    end

    GW_PUB -->|"credit.requested"| EX_CMD
    GW_PUB -->|"transaction.requested"| EX_CMD

    LEADER_PUB -->|"credit.added"| EX_EVT
    LEADER_PUB -->|"transaction.received"| EX_EVT
    LEADER_PUB -->|"transaction.rejected"| EX_EVT
    LEADER_PUB -->|"block.mined"| EX_EVT
    LEADER_PUB -->|"leader.changed"| EX_EVT

    EX_CMD -->|"credit.requested<br/>transaction.requested"| Q_LEADER
    EX_EVT -->|"#"| Q_GW
    EX_EVT -->|"#"| Q_AUDIT
    EX_DLX --> Q_DLQ

    Q_LEADER -.->|"TTL expirado<br/>ou max retries"| EX_DLX

    Q_LEADER --> LEADER_CON
    Q_GW --> GW_CON
    Q_AUDIT --> AUDIT_CON
    Q_DLQ --> DLQ_VIEW

    style EX_CMD fill:#ff7043,color:#fff
    style EX_EVT fill:#ff7043,color:#fff
    style EX_DLX fill:#d32f2f,color:#fff
    style Q_DLQ fill:#ef9a9a
```

---

## 3. Fluxo de Mensagens — Transferência com Validação de Saldo

Sequência completa de uma transação bem-sucedida.

```mermaid
sequenceDiagram
    autonumber
    actor User as Usuário
    participant FE as Flutter Web
    participant GW as Gateway
    participant MQ as RabbitMQ
    participant L as Nó Líder
    participant AU as Auditoria

    User->>FE: Preenche form (Alice→Bob, R$50)
    FE->>GW: POST /transactions
    GW->>GW: Valida formato, gera tx_id (UUID)
    GW->>MQ: publish(blockchain.commands,<br/>transaction.requested, payload)
    MQ-->>GW: Confirm
    GW-->>FE: 202 Accepted {tx_id}

    Note over MQ,L: Mensagem roteada para q.leader.commands

    MQ->>L: deliver(transaction.requested)
    L->>L: Calcula saldo Alice<br/>(créditos − débitos no ledger)

    alt Saldo suficiente
        L->>L: Adiciona ao MemPool
        L->>MQ: publish(blockchain.events,<br/>transaction.received)
        L->>MQ: ACK
        MQ->>GW: deliver(transaction.received)
        MQ->>AU: deliver(transaction.received)
        GW-)FE: WS push: transaction.received
        FE-->>User: "Transação aceita, aguardando bloco..."
        AU->>AU: Persiste no MariaDB

        Note over L: Mineração assíncrona (goroutine)
        L->>L: PoW (encontra nonce válido)
        L->>MQ: publish(blockchain.events,<br/>block.mined)
        MQ->>GW: deliver(block.mined)
        MQ->>AU: deliver(block.mined)
        GW-)FE: WS push: block.mined
        FE-->>User: Atualiza dashboard com novo bloco
    else Saldo insuficiente
        L->>MQ: publish(blockchain.events,<br/>transaction.rejected)
        L->>MQ: ACK
        MQ->>GW: deliver(transaction.rejected)
        GW-)FE: WS push: transaction.rejected
        FE-->>User: "Saldo insuficiente"
    end
```

---

## 4. Tolerância a Falhas — Eleição Bully com Fila Persistente

Mostra como a fila preserva mensagens durante a troca de líder.

```mermaid
sequenceDiagram
    autonumber
    actor User as Usuário
    participant FE as Flutter Web
    participant GW as Gateway
    participant MQ as RabbitMQ
    participant L1 as Nó 3 (Líder)
    participant N2 as Nó 2
    participant N1 as Nó 1

    User->>FE: Envia transação
    FE->>GW: POST /transactions
    GW->>MQ: publish(transaction.requested)
    GW-->>FE: 202 Accepted

    Note over L1: 💥 Líder cai (crash)
    L1--xMQ: Conexão perdida

    Note over MQ: Mensagem permanece em<br/>q.leader.commands<br/>(durable + persistent)

    Note over N1,N2: Detectam falha de heartbeat
    N2->>N1: Inicia eleição Bully
    N1-->>N2: OK (você assume)
    N2->>N2: Vira líder
    N2->>MQ: Subscribe q.leader.commands
    N2->>MQ: publish(leader.changed)
    MQ->>GW: deliver(leader.changed)
    GW-)FE: WS push: leader.changed
    FE-->>User: "Novo líder: Nó 2"

    MQ->>N2: deliver(transaction.requested)<br/>[mensagem do backlog]
    N2->>N2: Processa transação
    N2->>MQ: publish(transaction.received)
    N2->>MQ: ACK
    MQ->>GW: deliver(transaction.received)
    GW-)FE: WS push
    FE-->>User: "Transação processada após eleição"
```

---

## 5. Deployment Docker (execução única)

Topologia física dos containers em uma única máquina.

```mermaid
graph TB
    subgraph HOST["Máquina do usuário"]
        BROWSER["Navegador<br/>localhost:8080"]

        subgraph COMPOSE["docker-compose up"]
            subgraph NET["rede docker: blockchain-net"]
                C_GW["container: gateway<br/>image: blockchain-gateway<br/>porta: 8080→8080<br/>serve Flutter Web + API"]

                C_N1["container: node1<br/>image: blockchain-node<br/>id: 1, porta: 8001"]
                C_N2["container: node2<br/>image: blockchain-node<br/>id: 2, porta: 8002"]
                C_N3["container: node3<br/>image: blockchain-node<br/>id: 3, porta: 8003"]

                C_RMQ["container: rabbitmq<br/>image: rabbitmq:3.12-management<br/>portas: 5672, 15672"]

                C_DB["container: mariadb<br/>image: mariadb:10<br/>porta: 3306<br/>volume: db-data"]

                C_AUDIT["container: audit<br/>image: blockchain-audit"]
            end
        end
    end

    BROWSER -->|HTTP/WS| C_GW
    C_GW <--> C_RMQ
    C_N1 <--> C_RMQ
    C_N2 <--> C_RMQ
    C_N3 <--> C_RMQ
    C_AUDIT <--> C_RMQ
    C_N1 <--> C_DB
    C_N2 <--> C_DB
    C_N3 <--> C_DB
    C_AUDIT <--> C_DB
    C_N1 <-.RPC.-> C_N2
    C_N2 <-.RPC.-> C_N3
    C_N3 <-.RPC.-> C_N1

    style C_GW fill:#66bb6a,color:#fff
    style C_RMQ fill:#ff7043,color:#fff
    style C_DB fill:#8d6e63,color:#fff
```

**URLs expostas ao usuário:**

| URL | Conteúdo |
|---|---|
| `http://localhost:8080` | Flutter Web (interface principal) |
| `http://localhost:8080/api/*` | API REST do Gateway |
| `ws://localhost:8080/ws` | Stream de eventos em tempo real |
| `http://localhost:15672` | Painel de management do RabbitMQ (`guest/guest` em dev) |

---

## 6. Pipeline de Retry e DLQ

Como uma mensagem com falha transita até chegar na DLQ.

```mermaid
graph LR
    PUB["Publisher"] --> EX1["blockchain.commands"]
    EX1 --> Q1["q.leader.commands<br/>TTL=60s<br/>x-dead-letter-exchange=<br/>blockchain.dlx"]
    Q1 --> CONSUMER["Consumer<br/>(Nó Líder)"]
    CONSUMER -->|"NACK<br/>(erro transitório)"| Q1
    Q1 -->|"TTL expirado<br/>ou rejeitada"| EX_DLX["blockchain.dlx<br/>(fanout)"]
    EX_DLX --> DLQ["q.dlq"]
    DLQ --> INSPECT["Inspeção manual<br/>ou tela DLQ no Flutter"]

    style EX_DLX fill:#d32f2f,color:#fff
    style DLQ fill:#ef9a9a
```

**Fluxo:**
1. Publisher envia mensagem para `blockchain.commands`.
2. Roteada para `q.leader.commands`.
3. Consumer processa; em caso de erro recuperável, faz NACK (mensagem volta).
4. Após N tentativas ou TTL de 60 s, RabbitMQ aplica DLX → roteia para `blockchain.dlx`.
5. Mensagem cai em `q.dlq` para inspeção.
6. Operador analisa via tela "DLQ" no Flutter ou painel RabbitMQ.

---

> Os diagramas acima cobrem os requisitos das Etapas 1 e 2. Diagramas mais específicos (configuração detalhada de bindings, payload examples) aparecem nas Etapas 3 e 4.
