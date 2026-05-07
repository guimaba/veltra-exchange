# Roteiro de Testes Ponta-a-Ponta

Roteiro objetivo para validar o sistema rodando via `docker compose up`. Cada cenário tem: pré-condição, passos, resultado esperado e como inspecionar.

## Setup inicial

```powershell
# Da raiz do repositorio
.\start.ps1
```

Aguarde até ver no log dos 3 nós a mensagem `[Nó X] Eu sou o novo coordenador!` (geralmente o nó 3, por ter o maior ID).

URLs disponíveis:
- **App Flutter Web**: <http://localhost:8080>
- **Painel RabbitMQ**: <http://localhost:15672> (`admin` / `admin`)

---

## Cenário 1 — Adicionar crédito (saldo simulado)

**Pré-condição:** sistema recém-iniciado, nenhuma conta ainda.

**Passos:**
1. Abra <http://localhost:8080> e vá na aba **Carteira**.
2. Preencha: conta=`alice`, valor=`1000`. Clique em **Creditar**.

**Esperado:**
- Snackbar "Credito enfileirado (tx_id: ...)" aparece.
- Em ~1-2s, o card de saldos exibe **alice: R$ 1.000,00**.
- Aba **Monitor** mostra entrada `credit.added` com payload contendo `account=alice, amount=1000, new_balance=1000`.
- Em alguns segundos (mineração), aparece `block.mined` no Monitor e o bloco aparece na aba **Dashboard**.

**Inspeção no painel RabbitMQ:**
- Queues → `q.audit.events` mostra contadores de mensagens consumidas crescendo.
- Queues → `q.leader.commands` deve voltar a 0 mensagens (líder consumiu).

---

## Cenário 2 — Transferência válida

**Pré-condição:** `alice` com saldo R$ 1000 (Cenário 1).

**Passos:**
1. Aba **Enviar**. De: `alice`, Para: `bob`, Valor: `250`. Enviar.

**Esperado:**
- Resposta inline: `Transacao enfileirada. tx_id=...`
- Em 1-2s, Carteira mostra: alice = R$ 750, bob = R$ 250.
- Monitor: aparece `transaction.received` com `balance_after: { alice: 750, bob: 250 }`, depois `block.mined`.
- Dashboard mostra novo bloco com 1 transação de tipo `transfer`.

---

## Cenário 3 — Transferência rejeitada por saldo insuficiente

**Pré-condição:** `bob` com saldo R$ 250 (Cenário 2).

**Passos:**
1. Aba **Enviar**. De: `bob`, Para: `carol`, Valor: `500`. Enviar.

**Esperado:**
- HTTP 202 retorna normalmente (`tx_id` enfileirado).
- Em 1-2s, Monitor mostra `transaction.rejected` com `reason: INSUFFICIENT_FUNDS, current_balance: 250`.
- Saldos NÃO mudam (`bob` continua com R$ 250).
- **Nenhum bloco** é minerado para essa transação.
- DLQ continua vazia (rejeição é erro de negócio, não vai pra DLQ).

---

## Cenário 4 — Falha do líder + eleição Bully + persistência da fila

**Pré-condição:** sistema em operação, com algum saldo na blockchain.

**Passos:**
1. Identifique o líder atual no header da app (ex: "Líder: Nó 3").
2. Em outro terminal:
   ```powershell
   docker compose stop node3   # ou seja qual for o líder
   ```
3. Imediatamente, dispare uma transação pela aba Enviar (alice → bob, R$ 10).

**Esperado:**
- Logo após o stop, status no header muda para "Sem líder" por alguns segundos.
- Em 5-15s, header passa a mostrar "Líder: Nó 2" (próximo maior ID disponível).
- Monitor registra `leader.changed` com `previous_leader: 3, new_leader: 2`.
- A transação enviada **é processada após a eleição** (mensagem ficou em `q.leader.commands` durante a transição). Aparece `transaction.received` e depois `block.mined` minerado pelo Nó 2.
- No painel RabbitMQ, durante a janela de eleição, `q.leader.commands` mostrou >0 messages temporariamente.

**Recuperação:**
```powershell
docker compose start node3
```
- Nó 3 sobe, percebe que já há líder (Nó 2), entra no estado RUNNING.
- Eventualmente, ao detectar nova falha ou no próximo ciclo, pode disputar a liderança novamente (depende do algoritmo).

---

## Cenário 5 — Mensagem mal-formada → DLQ

**Pré-condição:** sistema em operação. Acesso ao painel RabbitMQ.

**Passos:**
1. Abra <http://localhost:15672> → Exchanges → `blockchain.commands`.
2. **Publish message** com:
   - Routing key: `transaction.requested`
   - Properties: `delivery_mode = 2`, `content_type = application/json`
   - Payload (string crua): `isso nao e JSON valido`
3. Clique **Publish message**.

**Esperado:**
- O líder consome a mensagem, falha no `json.Unmarshal`, classifica como erro permanente.
- Mensagem é publicada em `blockchain.dlx` → cai em `q.dlq`.
- Painel RabbitMQ: Queues → `q.dlq` mostra **1 mensagem**.
- Aba **DLQ** no Flutter mostra o link "Ir direto para q.dlq" — clicando abre a fila no painel.
- Get message no painel mostra a mensagem com headers `x-error`, `x-original-routing-key`, `x-original-exchange`.

---

## Cenário 6 — Retry com erro técnico transitório

**Pré-condição:** sistema em operação.

**Passos (simulação manual de DB indisponível):**
1. `docker compose stop mariadb`
2. Pela app, envie uma transação válida (alice → bob, R$ 10).
3. Aguarde ~2 segundos e: `docker compose start mariadb`.

**Esperado:**
- Líder tenta processar, falha por conexão com DB, classifica como `Transient`.
- Re-publica em `blockchain.retry` → fila `q.retry.5s` (TTL 5s).
- Após 5s, mensagem volta para `blockchain.commands` → `q.leader.commands`.
- Tentativas se repetem; quando MariaDB volta, processo conclui com sucesso.
- Logs do nó líder mostram cada tentativa com `tentativa N/3`.
- Auditoria registra **apenas uma vez** o evento (idempotência via `processed_messages`).

**Limite:**
- Se MariaDB ficar fora >15s (3 tentativas × 5s), mensagem cai na DLQ.

---

## Como inspecionar

### Logs dos serviços
```powershell
docker compose logs -f node1 node2 node3   # nós da blockchain
docker compose logs -f gateway              # gateway
docker compose logs -f audit                # auditoria
docker compose logs -f rabbitmq             # broker
```

### Painel RabbitMQ
- Overview de filas: <http://localhost:15672/#/queues/%2Fblockchain>
- DLQ: <http://localhost:15672/#/queues/%2Fblockchain/q.dlq>
- Exchanges: <http://localhost:15672/#/exchanges/%2Fblockchain>

### Banco de dados
```powershell
docker exec -it blockchain-mariadb mysql -uroot -proot blockchain
```
```sql
SELECT COUNT(*) FROM audit_events;
SELECT schema_id, recorded_at FROM audit_events ORDER BY id DESC LIMIT 10;
SELECT COUNT(*) FROM processed_messages;
```

### Per-node DBs
```sql
SHOW DATABASES;  -- deve mostrar blockchain_node_1, _2, _3
USE blockchain_node_3;
SELECT block_index, hash, timestamp FROM blocks ORDER BY block_index DESC LIMIT 5;
```

---

## Checklist de aceitação

- [ ] Cenário 1: crédito simulado funciona, evento aparece no Monitor, bloco minerado
- [ ] Cenário 2: transferência válida atualiza saldos, gera bloco
- [ ] Cenário 3: transferência sem saldo é rejeitada, não vira bloco
- [ ] Cenário 4: líder cai, novo líder é eleito, mensagem em backlog é processada
- [ ] Cenário 5: mensagem mal-formada vai para DLQ
- [ ] Cenário 6: erro transitório dispara retry, processo conclui após DB voltar
- [ ] Auditoria persiste todos os eventos no MariaDB
- [ ] Reinicio de gateway preserva eventos via filas duráveis (gateway perde snapshot, mas eventos novos chegam)
