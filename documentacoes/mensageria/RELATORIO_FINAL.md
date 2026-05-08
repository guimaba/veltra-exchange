# Relatório Final — Trabalho Prático de Mensageria

**Disciplina:** Sistemas Distribuídos — FURB
**Tema:** Comunicação assíncrona com RabbitMQ sobre uma rede de blockchain distribuída
**Repositório:** <https://github.com/guimaba/blockchain_sistemasDistribuidos>

---

## Sumário executivo

Este trabalho implementa uma camada de mensageria assíncrona com **RabbitMQ** sobre o projeto base de blockchain distribuída em Go (com eleição Bully, mineração PoW e persistência em MariaDB). Sobre essa base, foram adicionados:

- **Pacote `pkg/messaging`** em Go com publisher/consumer, reconexão automática, ACK manual, retry com TTL e Dead Letter Queue.
- **Gateway HTTP/WebSocket** (`cmd/gateway`) que expõe API REST para o frontend e faz broadcast de eventos em tempo real.
- **Serviço de auditoria** (`cmd/audit`) que persiste todos os eventos no MariaDB com idempotência.
- **Interface gráfica em Flutter Web** (`web/`) com 5 telas (Carteira, Dashboard, Enviar, Monitor, DLQ).
- **Orquestração Docker** completa (`docker-compose.yml`) que sobe tudo (3 nós + Rabbit + MariaDB + gateway + auditoria + Flutter Web) com **um único comando**.

> ⚠️ **Camada financeira simulada.** O sistema demonstra arquitetura de mensageria assíncrona sobre uma blockchain didática. A operação "adicionar crédito" injeta saldo virtual — não há integração com pagamento real, gateway financeiro, Pix ou cartão. O foco do trabalho é a comunicação assíncrona.

---

## Mapa das entregas

| Etapa do enunciado | Pontuação | Documento(s) | Status |
|---|---|---|---|
| 1. Descrição do Cenário | parte de 2,5 pts | [01_cenario.md](./01_cenario.md) | ✅ |
| 2. Arquitetura da Solução | parte de 2,5 pts | [02_arquitetura.md](./02_arquitetura.md) | ✅ |
| Diagramas (1 e 2) | parte de 2,5 pts | [03_diagramas.md](./03_diagramas.md) | ✅ |
| 3. Configuração do RabbitMQ | parte de 3,5 pts | [04_configuracao_rabbitmq.md](./04_configuracao_rabbitmq.md) | ✅ |
| 4. Exemplos de Uso | parte de 3,5 pts | [05_exemplos_uso.md](./05_exemplos_uso.md) | ✅ |
| 5. Considerações Técnicas | parte de 3,5 pts | [06_consideracoes_tecnicas.md](./06_consideracoes_tecnicas.md) | ✅ |
| Roteiro de testes ponta-a-ponta | — | [07_roteiro_testes.md](./07_roteiro_testes.md) | ✅ |
| Implementação técnica | — | Veja seção "Implementação" abaixo | ✅ |
| Relatório final | 2,5 pts | **este arquivo** | ✅ |

---

## Como executar

**Pré-requisito único:** [Docker Desktop](https://www.docker.com/products/docker-desktop/) instalado e rodando.

**Comando:**
```powershell
.\start.ps1            # Windows
./start.sh             # Linux/macOS
```

ou diretamente:
```bash
docker compose up --build
```

**URLs:**
| URL | Conteúdo |
|---|---|
| <http://localhost:8080> | Aplicação Flutter Web |
| <http://localhost:8080/api/state> | Snapshot do estado |
| ws://localhost:8080/ws | Stream de eventos |
| <http://localhost:15672> | Painel RabbitMQ (`admin/admin`) |

Detalhes em [README.md](../../README.md).

---

## Arquitetura — visão consolidada

```
                                              minera/valida
                            ┌──────────────────────────────────┐
                            │                                  │
   Flutter Web ──HTTP──▶ Gateway ──AMQP──▶ blockchain.commands ──▶ Nó Líder
       ▲                    │                                       │
       └──── WebSocket ─────┤                                       │ publica eventos
                            │                                       ▼
                            │                                blockchain.events (topic)
                            │                                  │           │
                            │              ┌───────────────────┘           │
                            └─────AMQP─────┤                               │
                                           ▼                               ▼
                                    q.gateway.events              q.audit.events
                                                                          │
                                                                          ▼
                                                                MariaDB (audit_events)
```

- **HTTP** (síncrono) entre Flutter e gateway para enviar comandos com confirmação imediata (HTTP 202).
- **WebSocket** (assíncrono) push do gateway para Flutter, alimentado pelos eventos consumidos do Rabbit.
- **AMQP/RabbitMQ** entre gateway, líder, auditoria. Tudo desacoplado.
- **RPC interno** entre nós (já existente) para o consenso Bully e propagação de blocos.

Diagramas detalhados (componentes, sequência, deployment Docker, pipeline DLQ) em [03_diagramas.md](./03_diagramas.md).

---

## Topologia RabbitMQ — resumo

| Tipo | Nome | Função |
|---|---|---|
| Exchange | `blockchain.commands` (direct) | Comandos do gateway pro líder (`credit.requested`, `transaction.requested`) |
| Exchange | `blockchain.events` (topic) | Eventos de domínio (`block.mined`, `transaction.received/rejected`, `credit.added`, `leader.changed`) |
| Exchange | `blockchain.dlx` (fanout) | Dead Letter Exchange |
| Exchange | `blockchain.retry` (direct, internal) | Buffer de retry com TTL 5s |
| Fila | `q.leader.commands` | Consumida pelo líder atual; TTL 60s, DLX configurada |
| Fila | `q.gateway.events` | Consumida pelo gateway (broadcast WS) |
| Fila | `q.audit.events` | Consumida pelo serviço de auditoria |
| Fila | `q.dlq` | Dead Letter Queue |
| Fila | `q.retry.5s` | Buffer de retry; após TTL devolve à fila original |

**Toda a topologia é declarativa** via [docker/rabbitmq/definitions.json](../../docker/rabbitmq/definitions.json), importada automaticamente no startup do broker. Reprodutibilidade total.

Detalhes em [04_configuracao_rabbitmq.md](./04_configuracao_rabbitmq.md).

---

## Implementação — mapa do código

### Backend Go

```
cmd/
├── node/main.go            # Nó da blockchain (env vars, peers Docker, publisher integrado, consumer de comandos quando líder)
├── gateway/                # Gateway HTTP/WebSocket
│   ├── main.go             #   bootstrap
│   ├── server.go           #   API REST + WS + SPA fallback
│   ├── state.go            #   snapshot em memória dos saldos/blocos/líder
│   └── hub.go              #   broadcast WebSocket
└── audit/
    ├── main.go             # Consumer de q.audit.events
    └── store.go            # Persistência MariaDB com idempotência

pkg/
├── messaging/              # ★ Pacote central da mensageria
│   ├── topology.go         #   constantes (espelho do definitions.json)
│   ├── envelope.go         #   envelope JSON + payloads tipados
│   ├── errors.go           #   Business / Transient / Permanent
│   ├── client.go           #   conexão com reconexão automática
│   ├── publisher.go        #   publish com confirms + timeout
│   └── consumer.go         #   ACK manual + retry + DLQ
├── blockchain/             # Existente, estendido com TxID/Kind, GetBalance, HasTransaction
├── bully/                  # Existente
├── network/rpc.go          # Refatorado: aceita peers Docker, EventEmitter opcional
└── database/mariadb.go     # Existente
```

### Frontend Flutter Web

```
web/
├── pubspec.yaml
├── web/index.html, manifest.json
└── lib/
    ├── main.dart
    ├── theme.dart
    ├── api.dart            # Cliente HTTP + WebSocket
    ├── state.dart          # AppState (Provider) que aplica eventos do WS
    └── screens/
        ├── home.dart       # NavigationBar com 5 abas
        ├── wallet.dart     # Carteira + adicionar crédito
        ├── dashboard.dart  # Blocos minerados + estatísticas + líder
        ├── send.dart       # Form de transferência com feedback de saldo
        ├── monitor.dart    # Stream de eventos brutos
        └── dlq.dart        # DLQ + link pro painel RabbitMQ
```

### Infraestrutura

```
docker-compose.yml          # 6 serviços orquestrados
docker/
├── Dockerfile.node         # Multi-stage; imagem ~20 MB
├── Dockerfile.gateway      # 3 stages: Flutter Web + Go + runtime; ~30 MB
├── Dockerfile.audit
├── rabbitmq/
│   ├── rabbitmq.conf
│   └── definitions.json    # Topologia AMQP declarativa
└── mariadb/
    └── init.sql            # Schema inicial: blocks, transactions, processed_messages, audit_events

start.ps1, start.sh         # Inicialização com um comando
```

---

## Decisões técnicas — destaques

### 1. Distinção entre erro de negócio e erro técnico

- **Negócio** (saldo insuficiente, conta inválida): publica evento `*.rejected` + ACK. **Não retry.**
- **Transitório** (DB momentaneamente fora, rede instável): re-publica em `blockchain.retry` (TTL 5s). Após 3 tentativas, vai pra DLQ.
- **Permanente** (JSON mal-formado, schema desconhecido): direto pra DLQ, sem retry.

A distinção é crítica: retry de erro de negócio causa loop infinito; erro permanente em retry gasta slots e atrasa a DLQ.

### 2. Idempotência

Toda mensagem tem `tx_id` (UUID v4) gerado pelo gateway. Antes de processar, consumers fazem `INSERT IGNORE` em `processed_messages` (PK composta `tx_id + consumer`). Mensagens duplicadas (re-entregas após crash) são detectadas e ignoradas.

### 3. Publisher confirms + ACK manual

- Publisher usa `channel.Confirm(false)` e aguarda confirmação do broker em até 5s.
- Consumer **nunca** usa auto-ack; ACK só após processar com sucesso.
- Combinado: garantia de **at-least-once delivery** com **exactly-once effect** (graças à idempotência).

### 4. Liderança Bully + consumer dinâmico

O nó só consome `q.leader.commands` quando é líder. Implementação:

- Goroutine de monitoramento checa `node.IsLeader()` a cada 2s.
- Quando vira líder, registra um consumer; quando perde, cancela.
- Mensagens em backlog durante eleição **não são perdidas** — ficam na fila durável até o novo líder se conectar.

### 5. Validação de saldo

- Calculada em runtime no líder a partir de `bc.Blocks` (confirmados) + `bc.MemPool` (pendentes).
- Permite simulação fiel: se o usuário enviar duas transações em sequência, a segunda já considera o débito da primeira pendente.
- Erro de saldo é classificado como `Business` → ACK + evento `transaction.rejected`.

Mais decisões em [06_consideracoes_tecnicas.md](./06_consideracoes_tecnicas.md).

---

## Validação ponta-a-ponta

Roteiro com 6 cenários de teste documentado em [07_roteiro_testes.md](./07_roteiro_testes.md):

1. Adicionar crédito simulado
2. Transferência válida
3. Transferência rejeitada por saldo insuficiente
4. Falha do líder + eleição Bully + persistência da fila
5. Mensagem mal-formada → DLQ
6. Erro transitório → retry com sucesso

Cada cenário lista: pré-condição, passos, resultado esperado, como inspecionar (logs, painel RabbitMQ, banco).

---

## Atendimento aos requisitos do enunciado

| Requisito | Onde foi atendido |
|---|---|
| Descrição do cenário com contextualização e justificativa de assincronia | [01_cenario.md](./01_cenario.md) — 5 motivos arquiteturais detalhados |
| Identificação de produtores e consumidores | [02_arquitetura.md](./02_arquitetura.md) §2.3 |
| Definição do fluxo de mensagens, tópicos e filas | [02_arquitetura.md](./02_arquitetura.md) §2.2 e §2.4 |
| Estratégias de escalabilidade, confiabilidade, tolerância a falhas | [02_arquitetura.md](./02_arquitetura.md) §2.5–2.7 |
| Diagramas ilustrativos | [03_diagramas.md](./03_diagramas.md) — 6 diagramas Mermaid |
| Exchanges, filas, bindings configurados | [04_configuracao_rabbitmq.md](./04_configuracao_rabbitmq.md) §3.4–3.6 |
| Políticas de retry e DLQ | [04_configuracao_rabbitmq.md](./04_configuracao_rabbitmq.md) §3.7–3.8 |
| Autenticação, autorização, criptografia | [04_configuracao_rabbitmq.md](./04_configuracao_rabbitmq.md) §3.9 |
| Casos de uso representativos com entrada/processamento/saída | [05_exemplos_uso.md](./05_exemplos_uso.md) — 6 casos completos com payloads JSON |
| Tecnologias e linguagens | [06_consideracoes_tecnicas.md](./06_consideracoes_tecnicas.md) §5.1 — Go, Flutter Web, RabbitMQ, MariaDB, Docker |
| Padrões de mensagens (JSON/Avro/Protobuf) | [06_consideracoes_tecnicas.md](./06_consideracoes_tecnicas.md) §5.2 — JSON com schema versionado, justificativa vs. alternativas |
| Boas práticas para desempenho e integridade | [06_consideracoes_tecnicas.md](./06_consideracoes_tecnicas.md) §5.3 — 11 práticas aplicadas |

---

## Limitações conhecidas

- **Senhas em texto plano** no `docker-compose.yml`. Em produção: secrets manager.
- **Sem TLS** no broker localmente. Em produção: AMQPS obrigatório (config de exemplo no §3.9.3).
- **Single-broker** (sem cluster). Suficiente para o trabalho; em produção, cluster RabbitMQ com Quorum Queues.
- **Snapshot do gateway é em memória** — reiniciar o gateway zera o dashboard até novos eventos chegarem. Em produção, persistir ou rebuildar a partir de `audit_events`.
- **Camada financeira é simulada** (escopo intencional do trabalho).

Possíveis evoluções listadas em [06_consideracoes_tecnicas.md](./06_consideracoes_tecnicas.md) §5.4.

---

## Conclusão

A solução demonstra na prática como o RabbitMQ atua como **espinha dorsal de eventos** de uma rede de blockchain distribuída, permitindo:

- Interface web reativa em tempo real (Flutter Web + WebSocket).
- Desacoplamento total entre clientes externos e os nós da blockchain.
- Resiliência durante operações lentas (mineração) e troca de líder (Bully).
- Capacidade de adicionar novos consumidores (auditoria, métricas, alertas) sem modificar produtores.
- Tratamento robusto de falhas com pipeline retry + DLQ + idempotência.

A escolha de stack (Go + Flutter Web + RabbitMQ + Docker) prioriza simplicidade de execução (um comando para subir tudo), reutilização do projeto base existente, e adequação aos requisitos do enunciado.

A separação clara entre **comandos** (single consumer) e **eventos** (multi consumer), entre **erros de negócio** e **erros técnicos**, e a aplicação consistente de práticas como ACK manual, publisher confirms e idempotência via UUID + tabela de processados, garantem um sistema com semântica **at-least-once delivery, exactly-once effect** — o padrão correto para sistemas distribuídos confiáveis.
