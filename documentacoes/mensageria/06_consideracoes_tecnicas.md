# Etapa 5 — Considerações Técnicas

Esta seção justifica as escolhas tecnológicas, define o padrão de mensagens adotado e lista as boas práticas aplicadas para garantir desempenho e integridade.

---

## 5.1 Stack tecnológica

### Visão geral

| Camada | Tecnologia | Versão alvo |
|---|---|---|
| Backend (nós + gateway + auditoria) | **Go** | 1.21+ |
| Frontend (web SPA) | **Flutter** (build web) | 3.19+ |
| Mensageria | **RabbitMQ** | 3.12+ (com plugin Management) |
| Persistência | **MariaDB** | 10.11 |
| Comunicação browser ↔ backend | **HTTP REST + WebSocket** | – |
| Comunicação interna (consenso) | **Go RPC (`net/rpc`)** | – |
| Empacotamento e execução | **Docker + Docker Compose** | 24+ / v2 |

### 5.1.1 Por quê Go no backend

- **Já usado no projeto base.** Reaproveitamento total dos pacotes existentes (`pkg/blockchain`, `pkg/bully`, `pkg/network`, `pkg/database`).
- **Concorrência nativa via goroutines.** Mineração rodando em paralelo a consumo de fila e RPC sem complexidade de threads.
- **Cliente AMQP maduro:** [`github.com/rabbitmq/amqp091-go`](https://github.com/rabbitmq/amqp091-go) — fork oficial mantido pela RabbitMQ Team, sucessor do `streadway/amqp`.
- **Compilação estática** facilita Docker (binário único, base `scratch` ou `alpine`, imagens finais < 30 MB).

### 5.1.2 Por quê Flutter Web no frontend

- **Build single-codebase** que permite estender futuramente para Android/iOS/desktop sem reescrever.
- **Renderização canvas (CanvasKit)** dá UI consistente entre navegadores.
- **Riverpod / Provider** oferecem gerência de estado reativa, ideal para um dashboard que recebe eventos via WebSocket.
- **`flutter build web` gera HTML/JS estáticos** servidos diretamente pelo gateway Go — sem necessidade de Node.js no runtime.

### 5.1.3 Por quê RabbitMQ (vs. Kafka, NATS, Redis Streams)

| Critério | RabbitMQ | Kafka | NATS JetStream | Redis Streams |
|---|---|---|---|---|
| Roteamento flexível (topic, fanout, direct) | ✅ | ❌ (apenas tópicos) | △ | ❌ |
| DLQ nativa | ✅ | ❌ (manual) | ✅ | ❌ |
| Retry com TTL | ✅ | ❌ (lib externa) | ✅ | ❌ |
| Curva de aprendizado | Baixa | Alta | Média | Baixa |
| Footprint operacional | Pequeno | Grande | Pequeno | Pequeno |
| Adequação ao trabalho | **✅ Ideal** | Overkill | Possível | Limitado |

RabbitMQ entrega exatamente o que o trabalho pede (filas, exchanges, DLQ, retry), com baixo esforço operacional e excelente UI de management. Kafka seria desproporcional para o volume e os requisitos descritos.

### 5.1.4 Por quê Docker Compose

- **Execução única**: `docker-compose up` sobe os 6+ containers necessários.
- **Rede privada** entre containers; nenhum serviço interno precisa ser exposto além das portas necessárias (8080, 15672).
- **Reprodutibilidade total** — qualquer máquina com Docker reproduz exatamente o ambiente.
- **Healthchecks** garantem ordem correta de inicialização (Rabbit antes dos consumers, MariaDB antes dos nós).

---

## 5.2 Padrão de mensagens — JSON

### Escolha: JSON UTF-8 com schema versionado

**Justificativa da escolha vs. alternativas:**

| Critério | JSON | Avro | Protobuf |
|---|---|---|---|
| Legibilidade humana (debug) | ✅ | ❌ | ❌ |
| Suporte nativo Go | ✅ (`encoding/json`) | ⚠️ (libs externas) | ⚠️ (gerar código) |
| Suporte Flutter (Dart) | ✅ (`dart:convert`) | ❌ | ⚠️ |
| Tamanho do payload | Maior | Menor | Menor |
| Schema enforcement | Manual (validação) | ✅ (registry) | ✅ (`.proto`) |
| Evolução de schema | Versão no envelope | ✅ (built-in) | ✅ (campos opcionais) |

**Decisão:** JSON.

**Motivos:**
1. Mensagens trafegam entre Go e Dart (Flutter), e ambos têm suporte first-class.
2. Volume de mensagens é baixo (centenas/segundo no pior caso) — overhead de tamanho é irrelevante.
3. Debug é trivial: olhando a fila pelo painel RabbitMQ, qualquer pessoa entende o conteúdo.
4. Schema versioning resolvido manualmente via campo `schema` no envelope (ex.: `blockchain.transaction.requested.v1`). Quando uma mudança breaking for necessária, publica-se `v2` e consumers tratam ambas durante o período de migração.

> Em produção com altíssimo throughput ou contratos rígidos entre serviços, Protobuf seria a escolha. Para este trabalho, JSON é o ponto ótimo entre simplicidade e expressividade.

### 5.2.1 Envelope padrão

Toda mensagem segue o mesmo invólucro:

```json
{
  "schema":    "<dominio>.<tipo>.<v>",
  "tx_id":     "<UUID v4>",
  "timestamp": "<ISO-8601 UTC>",
  "payload":   { /* específico do tipo */ }
}
```

**Campos obrigatórios:**

| Campo | Tipo | Função |
|---|---|---|
| `schema` | string | Identifica tipo + versão. Permite evolução compatível. |
| `tx_id` | string (UUID v4) | Identificador único da operação. **Base da idempotência.** |
| `timestamp` | string (ISO-8601 UTC) | Momento da criação da mensagem pelo produtor. Útil para debug e ordenação. |
| `payload` | object | Conteúdo específico do tipo (varia por mensagem). |

### 5.2.2 Tipos de mensagem (catálogo)

| Schema | Direção | Descrição |
|---|---|---|
| `blockchain.credit.requested.v1` | Gateway → Líder | Pedido de crédito |
| `blockchain.credit.added.v1` | Líder → Todos | Crédito registrado |
| `blockchain.transaction.requested.v1` | Gateway → Líder | Pedido de transferência |
| `blockchain.transaction.received.v1` | Líder → Todos | Transferência válida no MemPool |
| `blockchain.transaction.rejected.v1` | Líder → Todos | Transferência inválida (saldo etc.) |
| `blockchain.block.mined.v1` | Líder → Todos | Bloco minerado e propagado |
| `blockchain.leader.changed.v1` | Eleito → Todos | Novo líder após Bully |

Schemas concretos com payloads detalhados estão na **Etapa 4 (Exemplos de Uso)**.

### 5.2.3 Headers AMQP padronizados

Além do payload JSON, cada mensagem carrega headers padronizados:

| Header | Origem | Uso |
|---|---|---|
| `content-type: application/json` | Publisher | Sinaliza formato do body |
| `message-id: <uuid>` | Publisher | Idempotência (= `tx_id` do payload) |
| `timestamp: <unix>` | Publisher | Auditoria |
| `delivery-mode: 2` | Publisher | Persistente no broker |
| `x-retry-count: <n>` | Consumer (em retry) | Controla número de tentativas |
| `x-first-death-*` | RabbitMQ (DLX) | Rastreia origem em mensagens da DLQ |

---

## 5.3 Boas práticas adotadas

### 5.3.1 Idempotência

Todo consumer **deve** verificar se a mensagem já foi processada antes de aplicar efeito. A implementação:

1. Tabela `processed_messages` no MariaDB com PK `tx_id`.
2. Antes de processar, consumer faz `INSERT ... ON DUPLICATE KEY UPDATE` ou `INSERT IGNORE`.
3. Se já existia, mensagem é ACKeada sem re-aplicar efeito.

**Por quê:** retries e re-entregas são parte natural do AMQP. Idempotência evita débitos duplicados, blocos duplicados, etc. Sem isso, qualquer crash entre processamento e ACK causa duplicação.

### 5.3.2 ACK manual

- Modo automático (`auto-ack=true`) é **proibido** neste projeto.
- Consumer só faz ACK **após** processar com sucesso (gravar no MemPool, persistir no DB, etc.).
- Em caso de erro, faz NACK com `requeue=false` e republica em `blockchain.retry` (controle explícito do retry).

### 5.3.3 Publisher confirms

- Cliente Go usa `channel.Confirm(false)` para receber acks do broker.
- Em caso de NACK do broker (ex.: fila cheia com `reject-publish`), publisher detecta e propaga erro ao chamador.
- Sem confirms, mensagens podem ser perdidas silenciosamente em falhas do broker.

### 5.3.4 Connection pooling e canais

- **Uma conexão TCP** por processo (gateway, nó, auditoria) — conexões AMQP são caras.
- **Múltiplos canais** por conexão para paralelismo (canais são leves).
- **Reconexão automática** com backoff exponencial: 1s → 2s → 4s → ... → 30s (cap).
- **Heartbeat AMQP de 30s** detecta conexões mortas que não fecharam graciosamente.

### 5.3.5 Prefetch count adequado

- `q.leader.commands`: prefetch=**1** (garante ordenação parcial — apenas uma mensagem por vez no líder).
- `q.gateway.events`: prefetch=**10** (mais paralelismo OK; eventos são independentes).
- `q.audit.events`: prefetch=**50** (auditoria faz batch insert no DB).

Prefetch alto demais em consumers lentos causa head-of-line blocking; baixo demais subutiliza throughput.

### 5.3.6 Mensagens persistentes + filas duráveis

- **Filas duráveis**: sobrevivem a restart do broker.
- **Mensagens persistentes** (`delivery_mode=2`): sobrevivem em disco, não só RAM.
- A combinação dos dois é o que garante "at-least-once delivery" mesmo com falhas.

### 5.3.7 Distinguir erro de negócio de erro técnico

Já discutido na Etapa 3, vale repetir como prática:

| Tipo | Resposta |
|---|---|
| Erro de negócio (saldo, conta inválida, schema desconhecido) | Publica evento `*.rejected` + ACK. **Não retry.** |
| Erro técnico transitório (DB fora, rede instável) | Re-publica em `blockchain.retry` com contador. ACK original. |
| Erro técnico permanente (mensagem mal-formada) | Publica em `blockchain.dlx` + ACK. |

Confundir os três causa loops infinitos ou perda de dados.

### 5.3.8 Logging estruturado

- Todo log inclui `tx_id` quando aplicável → permite correlacionar atravez de produtor/consumer/auditoria.
- Formato JSON (`zap` ou `slog` em Go) para parsing posterior.
- Logs de eventos críticos (publicação, consumo, retry, DLQ) com nível INFO.

Exemplo:
```json
{"level":"info","ts":"2025-05-08T14:35:01Z","msg":"transaction accepted","tx_id":"7f1e2c4a-...","sender":"alice","receiver":"bob","amount":250.00,"node_id":3}
```

### 5.3.9 Validação no gateway antes de publicar

O gateway **não publica cegamente** o que recebe do Flutter. Antes:

1. Valida formato JSON.
2. Valida campos obrigatórios (`sender`, `receiver`, `amount`).
3. Valida tipos (`amount` numérico positivo).
4. Valida regex de conta (alfa-numérico, sem espaços).
5. Só então gera `tx_id` e publica.

Falhas retornam HTTP 400 imediatamente, sem poluir o broker.

### 5.3.10 Healthchecks e graceful shutdown

- Cada serviço expõe `GET /health` retornando estado de dependências (DB, Rabbit).
- Docker Compose usa healthchecks para orquestrar startup.
- Em SIGTERM, consumers param de buscar mensagens novas, terminam as em curso, fecham canais e conexão limpa. Evita mensagens "perdidas no limbo".

### 5.3.11 Observabilidade

- Painel RabbitMQ Management (`:15672`) já dá métricas básicas (depth de fila, taxa de publish/consume).
- Logs estruturados permitem agregação posterior em ELK/Grafana Loki.
- Tela DLQ no Flutter monitora taxa de mensagens com falha em tempo real.

---

## 5.4 Limitações conhecidas e melhorias futuras

### Limitações deste trabalho

- **Senhas em texto plano** no `docker-compose.yml`. Em produção, secrets managers.
- **Sem TLS** no broker localmente. Em produção, AMQPS obrigatório.
- **Single-broker** (sem cluster RabbitMQ). Suficiente para o trabalho; em produção, cluster com mirrored queues ou Quorum Queues.
- **Idempotência via DB lookup** tem custo de I/O. Em throughput alto, considerar cache (Redis) com TTL.
- **Backoff fixo (5s)** no retry. Backoff exponencial (5s → 30s → 5min) seria mais sofisticado mas adiciona complexidade de filas.

### Possíveis evoluções

- **Schema registry** (ex.: Confluent Schema Registry) para validação automática de payloads.
- **Tracing distribuído** (OpenTelemetry) para visualizar fluxo completo de uma transação.
- **Métricas Prometheus** expostas pelos serviços para alertas operacionais.
- **Cluster RabbitMQ** com Quorum Queues (substitui mirrored queues, mais resiliente).
- **Compensação** (sagas) para operações multi-step que precisem ser desfeitas em caso de falha parcial.

---

## 5.5 Resumo da Etapa 5

| Item do enunciado | Onde foi tratado |
|---|---|
| Tecnologias e linguagens | 5.1 |
| Padrões de mensagens (JSON, Avro, Protobuf) | 5.2 |
| Boas práticas (desempenho e integridade) | 5.3 |

A escolha de stack (Go + Flutter Web + RabbitMQ + Docker) prioriza **simplicidade de execução** (um comando para subir tudo), **reutilização** do projeto base existente, e **adequação dos requisitos do enunciado** (mensageria com filas, exchanges, DLQ, retry, segurança).

JSON com envelope versionado oferece o equilíbrio certo entre legibilidade, suporte cross-language (Go ↔ Dart) e capacidade de evolução de schema. As boas práticas listadas (idempotência, ACK manual, publisher confirms, distinção de tipos de erro) são aplicadas de forma consistente em todo o sistema, garantindo que o pipeline seja **at-least-once em entrega e exactly-once em efeito**.
