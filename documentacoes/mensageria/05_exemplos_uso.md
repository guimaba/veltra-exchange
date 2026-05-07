# Etapa 4 — Exemplos de Uso

Esta seção apresenta **cinco casos de uso representativos** que cobrem o ciclo completo do sistema: do crédito inicial até o tratamento de mensagens problemáticas na DLQ. Cada exemplo mostra **entradas, processamento e saídas esperadas**, com payloads JSON reais.

---

## 4.1 Caso 1 — Adicionar crédito a uma conta (saldo simulado)

**Objetivo:** Permitir que o usuário comece a operar na rede atribuindo um saldo virtual à sua conta.

### Entrada

Usuário acessa `http://localhost:8080`, vai para a tela "Carteira" e clica em **+ Adicionar Crédito**, informando:

```
Conta: alice
Valor: R$ 1000,00
```

### Processamento

**1. Flutter → Gateway (HTTP)**
```http
POST /api/accounts/credit HTTP/1.1
Host: localhost:8080
Content-Type: application/json

{
  "account": "alice",
  "amount": 1000.00
}
```

**2. Gateway → RabbitMQ (AMQP publish)**

O gateway gera UUID e publica o comando:

```
Exchange:    blockchain.commands
Routing key: credit.requested
Headers:     content-type=application/json
             message-id=<uuid>
             timestamp=<unix>
```

```json
{
  "schema": "blockchain.credit.requested.v1",
  "tx_id": "550e8400-e29b-41d4-a716-446655440000",
  "timestamp": "2025-05-08T14:30:00Z",
  "payload": {
    "account": "alice",
    "amount": 1000.00
  }
}
```

**3. Gateway → Flutter (HTTP response)**
```http
HTTP/1.1 202 Accepted
Content-Type: application/json

{
  "status": "queued",
  "tx_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

**4. RabbitMQ → Nó Líder**

A mensagem é roteada para `q.leader.commands`. O líder consome, registra o crédito no MemPool como uma transação especial (`sender=null`, `receiver=alice`, `amount=1000`), e publica:

```
Exchange:    blockchain.events
Routing key: credit.added
```

```json
{
  "schema": "blockchain.credit.added.v1",
  "tx_id": "550e8400-e29b-41d4-a716-446655440000",
  "timestamp": "2025-05-08T14:30:01Z",
  "payload": {
    "account": "alice",
    "amount": 1000.00,
    "new_balance": 1000.00
  }
}
```

**5. Eventos chegam ao Gateway (q.gateway.events) e à Auditoria (q.audit.events).**

**6. Gateway → Flutter (WebSocket push)**
```json
{ "event": "credit.added", "data": { ... } }
```

### Saída esperada

- Tela "Carteira" do Flutter mostra **saldo: R$ 1000,00** instantaneamente.
- Após mineração, aparece um novo bloco no dashboard contendo essa transação.
- No painel RabbitMQ Management (`:15672`), `q.audit.events` registrou o evento.

---

## 4.2 Caso 2 — Transação válida (saldo suficiente)

**Objetivo:** Demonstrar fluxo bem-sucedido de transferência entre contas com validação de saldo.

### Pré-condição
- Alice tem saldo R$ 1000,00 (do Caso 1).
- Bob tem saldo R$ 0,00.

### Entrada

Tela "Transferir":
```
De:    alice
Para:  bob
Valor: R$ 250,00
```

### Processamento

**1. Flutter → Gateway**
```http
POST /api/transactions
{ "sender": "alice", "receiver": "bob", "amount": 250.00 }
```

**2. Gateway publica em `blockchain.commands` (routing key: `transaction.requested`)**:
```json
{
  "schema": "blockchain.transaction.requested.v1",
  "tx_id": "7f1e2c4a-...",
  "timestamp": "2025-05-08T14:35:00Z",
  "payload": {
    "sender": "alice",
    "receiver": "bob",
    "amount": 250.00
  }
}
```

**3. Líder consome a mensagem.**

**4. Validação de saldo (lógica no nó):**

```
saldo(alice) = soma(creditos onde receiver=alice)
             − soma(debitos  onde sender=alice e tx confirmada ou no MemPool)
           = 1000.00 − 0 = 1000.00

Como 1000.00 ≥ 250.00 → APROVADO
```

**5. Líder adiciona ao MemPool e publica `transaction.received`:**
```json
{
  "schema": "blockchain.transaction.received.v1",
  "tx_id": "7f1e2c4a-...",
  "timestamp": "2025-05-08T14:35:01Z",
  "payload": {
    "sender": "alice",
    "receiver": "bob",
    "amount": 250.00,
    "balance_after": { "alice": 750.00, "bob": 250.00 }
  }
}
```

**6. Mineração (assíncrona).** Quando termina, líder publica `block.mined`:
```json
{
  "schema": "blockchain.block.mined.v1",
  "block_id": 42,
  "timestamp": "2025-05-08T14:35:03Z",
  "payload": {
    "index": 42,
    "previous_hash": "00abcd...",
    "hash": "000ef1...",
    "nonce": 8421,
    "transactions": [
      { "tx_id": "7f1e2c4a-...", "sender": "alice", "receiver": "bob", "amount": 250.00 }
    ],
    "miner_node_id": 3
  }
}
```

### Saída esperada

- Flutter recebe `transaction.received` via WS → exibe "Transação aceita, aguardando bloco..."
- Logo após, recebe `block.mined` → atualiza dashboard com bloco 42 e novos saldos.
- Saldo de Alice: R$ 750,00. Saldo de Bob: R$ 250,00.
- Bloco persistido no MariaDB pelo serviço de auditoria.

---

## 4.3 Caso 3 — Transação rejeitada por saldo insuficiente

**Objetivo:** Demonstrar tratamento de erro de **negócio** (não de infra). Não há retry — apenas evento de rejeição.

### Pré-condição
- Bob tem saldo R$ 250,00 (do Caso 2).

### Entrada

Bob tenta transferir mais do que possui:
```
De:    bob
Para:  carol
Valor: R$ 500,00
```

### Processamento

**1-2.** Iguais ao Caso 2 (POST → publish em `blockchain.commands`).

**3. Líder consome e valida:**
```
saldo(bob) = 250.00
Solicitado: 500.00
250.00 < 500.00 → REJEITADO
```

**4. Líder publica `transaction.rejected`:**
```json
{
  "schema": "blockchain.transaction.rejected.v1",
  "tx_id": "9c8b3d...",
  "timestamp": "2025-05-08T14:40:00Z",
  "payload": {
    "sender": "bob",
    "receiver": "carol",
    "amount": 500.00,
    "reason": "INSUFFICIENT_FUNDS",
    "current_balance": 250.00
  }
}
```

**5. Líder dá ACK na mensagem (não volta para fila — não é erro técnico).**

**6. Gateway recebe via `q.gateway.events`, faz broadcast WebSocket.**

### Saída esperada

- Flutter exibe banner vermelho: **"Saldo insuficiente. Saldo atual: R$ 250,00"**.
- Saldos não mudam.
- Nenhum bloco é minerado para essa transação.
- Auditoria registra a rejeição no MariaDB para análise.

> **Nota arquitetural:** rejeição por saldo é um **fato de negócio**, não uma falha. Por isso vai como evento normal em `blockchain.events`, não para a DLQ. A DLQ é reservada para falhas técnicas (mensagem mal-formada, erro persistente de infra).

---

## 4.4 Caso 4 — Falha do líder durante processamento (resiliência via Bully + fila durável)

**Objetivo:** Demonstrar que mensagens não são perdidas durante a troca de líder.

### Pré-condição
- Rede com 3 nós: Nó 1, Nó 2, Nó 3.
- Nó 3 é o líder atual (maior ID).

### Cenário

1. Usuário envia transação via Flutter.
2. Gateway publica em `blockchain.commands`.
3. Mensagem chega em `q.leader.commands`, mas Nó 3 cai antes de consumir (ex.: `docker kill node3`).

### Processamento

**1. Mensagem permanece na fila** — `durable=true` + `delivery_mode=2` garantem persistência.

**2. Nó 1 e Nó 2 detectam falha de heartbeat.**

**3. Bully:**
- Nó 2 inicia eleição (envia ELECTION para Nó 3 — sem resposta).
- Nó 2 se proclama líder (não há nó superior).
- Nó 2 envia COORDINATOR para Nó 1.

**4. Nó 2 (novo líder) registra consumer em `q.leader.commands`.**

**5. RabbitMQ entrega a mensagem em backlog para Nó 2.**

**6. Nó 2 publica `leader.changed`:**
```json
{
  "schema": "blockchain.leader.changed.v1",
  "timestamp": "2025-05-08T14:45:30Z",
  "payload": {
    "previous_leader": 3,
    "new_leader": 2,
    "reason": "heartbeat_timeout"
  }
}
```

**7. Nó 2 processa a transação normalmente** (publica `transaction.received` e depois `block.mined`).

### Saída esperada

- Flutter recebe `leader.changed` via WS → topo da tela atualiza: "Líder atual: Nó 2".
- Flutter recebe `transaction.received` e `block.mined` em sequência → transação foi processada com sucesso, **apesar** do crash do líder.
- Tempo total observado: ~3-5 segundos (timeout de heartbeat + eleição + consumo).
- Painel RabbitMQ mostra que a fila ficou com 1 mensagem por alguns segundos antes de ser consumida.

> **Demonstração prática:** este cenário pode ser reproduzido manualmente parando o container do líder com `docker compose stop node3` durante uma transação.

---

## 4.5 Caso 5 — Mensagem mal-formada → DLQ

**Objetivo:** Demonstrar o caminho de uma mensagem com falha permanente até a Dead Letter Queue.

### Cenário

Um cliente AMQP externo (não o gateway oficial) publica uma mensagem **mal-formada** em `blockchain.commands`:

```
Exchange:    blockchain.commands
Routing key: transaction.requested
Body:        "isso não é JSON válido"
```

> Nota: o gateway oficial validaria o payload antes de publicar. Este cenário simula um cliente comprometido ou bug em outro produtor.

### Processamento

**1. RabbitMQ roteia para `q.leader.commands` normalmente** — o broker não inspeciona o payload.

**2. Líder consome a mensagem.**

**3. Tenta `json.Unmarshal(body, &TransactionCmd{})` → erro.**

**4. Líder reconhece que retry **não vai resolver** (mensagem mal-formada permanecerá mal-formada). Decisão: enviar direto para a DLQ.**

**5. Líder publica em `blockchain.dlx`:**
```json
{
  "original_payload": "isso não é JSON válido",
  "headers": {
    "x-first-death-reason": "rejected",
    "x-first-death-queue": "q.leader.commands",
    "x-first-death-exchange": "blockchain.commands",
    "x-original-routing-key": "transaction.requested",
    "x-error": "json: cannot unmarshal string into Go value of type messaging.TransactionCmd"
  }
}
```

**6. Fanout `blockchain.dlx` → `q.dlq`.**

**7. Líder dá ACK na mensagem original (não volta para a fila).**

### Saída esperada

- Mensagem aparece na tela "DLQ" do Flutter, listada com:
  - `tx_id` (ou `??` se ausente).
  - Razão: `rejected — JSON inválido`.
  - Timestamp.
  - Botão "Ver payload bruto".
- Operador pode optar por:
  - **Descartar** (purge da fila).
  - **Reprocessar** (mover de volta para `blockchain.commands` se a causa raiz foi corrigida).
- Painel RabbitMQ mostra `q.dlq` com 1 mensagem.

---

## 4.6 Caso 6 — Erro técnico transitório → Retry com sucesso

**Objetivo:** Demonstrar o pipeline de retry para falhas recuperáveis.

### Cenário

Líder consome `transaction.requested`, valida saldo OK, tenta gravar no MemPool, mas o MariaDB está temporariamente indisponível (ex.: reinício de container).

### Processamento

**1. Tentativa 1:**
- Consume mensagem (header: sem `x-retry-count`).
- Erro: `connection refused` ao MariaDB.
- Decisão: erro técnico → retry.
- Republica em `blockchain.retry` (routing key `transaction.requested`) com header `x-retry-count: 1`.
- ACK na mensagem original.

**2. Mensagem entra em `q.retry.5s` com TTL de 5 s.**

**3. Após 5 s, expira → DLX `blockchain.commands` → volta para `q.leader.commands`.**

**4. Tentativa 2 (5 s depois):**
- MariaDB ainda fora.
- Republica em `blockchain.retry` com `x-retry-count: 2`.

**5. Tentativa 3 (10 s depois):**
- MariaDB voltou.
- Grava com sucesso.
- Publica `transaction.received`.

### Saída esperada

- Transação é processada com atraso de ~10-15 s (3 tentativas × 5 s).
- Logs do nó líder mostram cada tentativa.
- Auditoria registra `transaction.received` apenas uma vez (idempotência via `tx_id`).
- Se após 3 tentativas ainda falhar, vai para DLQ.

> **Importante:** o consumer **deve** verificar `x-retry-count` antes de re-publicar. Se já está em 3, pula direto para `blockchain.dlx`.

---

## 4.7 Resumo dos casos

| # | Cenário | Tipo | Resultado | DLQ envolvida |
|---|---|---|---|---|
| 1 | Adicionar crédito | Comando bem-sucedido | Saldo atualizado, evento publicado | Não |
| 2 | Transação válida | Comando bem-sucedido | Bloco minerado | Não |
| 3 | Saldo insuficiente | Erro de negócio | Evento `transaction.rejected` | Não |
| 4 | Falha de líder | Resiliência (Bully + fila durável) | Eleição + reprocesso | Não |
| 5 | JSON mal-formado | Erro permanente | Mensagem na DLQ | **Sim** |
| 6 | DB transitório fora | Erro técnico recuperável | Retry com sucesso | Não (mas iria após 3 falhas) |

Estes casos cobrem **todos** os requisitos funcionais e não-funcionais expostos nas Etapas 1-3:
- Comunicação síncrona (HTTP) e assíncrona (AMQP, WebSocket).
- Validação de saldo.
- Eventos de domínio com fanout para múltiplos consumidores.
- Resiliência durante operações lentas e troca de líder.
- Distinção entre erros de negócio e erros técnicos.
- Pipeline de retry com backoff.
- Dead Letter Queue para inspeção de falhas.
