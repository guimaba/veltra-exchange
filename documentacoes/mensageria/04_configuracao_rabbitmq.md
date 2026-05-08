# Etapa 3 — Configuração do RabbitMQ

Esta seção detalha **toda** a configuração necessária para reproduzir a topologia descrita na Etapa 2: vhost, usuários, exchanges, filas, bindings, política de retry com DLQ, parâmetros e segurança.

A configuração é aplicada de forma **declarativa** via arquivo `definitions.json` (formato nativo do RabbitMQ Management Plugin), importado automaticamente quando o container sobe. Isso garante reprodutibilidade total — não há comandos imperativos a rodar manualmente.

---

## 3.1 Visão geral

| Item | Quantidade | Observação |
|---|---|---|
| Virtual Host | 1 (`/blockchain`) | Isolamento lógico |
| Usuários | 4 | `gateway`, `node`, `audit`, `admin` |
| Exchanges | 4 | `commands`, `events`, `dlx`, `retry` |
| Filas | 5 | leader.commands, gateway.events, audit.events, dlq, retry buffer |
| Bindings | ~10 | Detalhados abaixo |

---

## 3.2 Virtual Host

```
Nome: /blockchain
Tracing: false
```

Toda a topologia vive dentro deste vhost. Permite isolar o trabalho de outros serviços que eventualmente compartilhem o broker.

---

## 3.3 Usuários e Permissões

Princípio: **menor privilégio possível**. Cada componente tem usuário próprio com permissões mínimas.

| Usuário | Senha (dev) | Tags | Configure | Write | Read |
|---|---|---|---|---|---|
| `admin` | `admin` | administrator | `.*` | `.*` | `.*` |
| `gateway` | `gateway-pw` | – | `^$` | `^blockchain\.commands$` | `^q\.gateway\.events$` |
| `node` | `node-pw` | – | `^$` | `^blockchain\.events$` | `^q\.leader\.commands$` |
| `audit` | `audit-pw` | – | `^$` | `^blockchain\.(retry\|dlx)$` | `^q\.audit\.events$` |

**Notas:**

- `configure=^$` impede que o usuário crie/destrua exchanges/filas — toda a topologia já vem pré-criada via `definitions.json`. Se o cliente tentar declarar passivamente um recurso que já existe (com mesmos parâmetros), a operação é permitida via `passive=true` no AMQP.
- `gateway` só pode publicar em `blockchain.commands` e ler `q.gateway.events`. Não consegue acessar comandos do líder nem a DLQ.
- `node` só publica eventos em `blockchain.events` e consome comandos de `q.leader.commands`. Não tem acesso à fila de auditoria.
- `audit` consome de `q.audit.events` e pode publicar **apenas** em `blockchain.retry` e `blockchain.dlx` (necessário para reportar falhas técnicas via DLQ — não pode publicar eventos de domínio).
- Em produção, senhas viriam de secrets (Vault/AWS Secrets Manager). Para o trabalho, ficam em variáveis de ambiente do `docker-compose`.

---

## 3.4 Exchanges

| Nome | Tipo | Durable | Auto-delete | Internal | Argumentos |
|---|---|---|---|---|---|
| `blockchain.commands` | `direct` | true | false | false | – |
| `blockchain.events` | `topic` | true | false | false | – |
| `blockchain.dlx` | `fanout` | true | false | false | – |
| `blockchain.retry` | `direct` | true | false | true | – |

**Justificativas:**

- **`direct` para `commands`** — comandos têm consumidor único e roteamento exato (`credit.requested`, `transaction.requested`). Topic seria overkill.
- **`topic` para `events`** — múltiplos consumidores podem se inscrever em padrões (`block.*`, `transaction.*`, `#` para tudo). Permite extensão futura sem alterar publisher.
- **`fanout` para `dlx`** — qualquer mensagem que chega na DLX deve ir para `q.dlq`. Não há decisão de roteamento.
- **`internal=true` para `retry`** — exchange interna; só pode receber mensagens do próprio broker (via DLX), não de clientes diretos. Evita uso indevido.
- Todas `durable=true` — sobrevivem a restart do broker.

---

## 3.5 Filas

### `q.leader.commands`

Comandos vindos do gateway, consumidos exclusivamente pelo nó líder.

| Parâmetro | Valor | Por quê |
|---|---|---|
| `durable` | `true` | Sobrevive a restart |
| `x-message-ttl` | `60000` (60 s) | Após 60 s sem ser consumida, vai pra DLX |
| `x-dead-letter-exchange` | `blockchain.dlx` | Onde as mensagens expiradas/rejeitadas vão |
| `x-max-length` | `10000` | Proteção contra crescimento descontrolado |
| `x-overflow` | `reject-publish` | Se cheia, rejeita novas publicações em vez de descartar antigas |

### `q.gateway.events`

Eventos consumidos pelo gateway para broadcast WebSocket.

| Parâmetro | Valor | Por quê |
|---|---|---|
| `durable` | `true` | – |
| `x-max-length` | `5000` | Eventos antigos não são úteis para a UI |
| `x-overflow` | `drop-head` | Descarta os mais antigos quando cheia (UI quer eventos recentes) |

### `q.audit.events`

Mesmos eventos, persistidos pelo serviço de auditoria.

| Parâmetro | Valor | Por quê |
|---|---|---|
| `durable` | `true` | – |
| `x-message-ttl` | `86400000` (24 h) | Auditoria já persistiu no MariaDB; após 24 h, mensagem já não precisa ficar na fila |

### `q.dlq`

Mensagens com falha crônica, para inspeção manual.

| Parâmetro | Valor | Por quê |
|---|---|---|
| `durable` | `true` | – |
| `x-message-ttl` | `604800000` (7 dias) | Tempo razoável para investigar antes de descartar |

### `q.retry.5s`

Buffer intermediário para retry com delay de 5 s. Mensagens NACKeadas pelo consumidor são re-publicadas aqui; após o TTL, voltam à fila original via DLX.

| Parâmetro | Valor | Por quê |
|---|---|---|
| `durable` | `true` | – |
| `x-message-ttl` | `5000` (5 s) | Backoff fixo |
| `x-dead-letter-exchange` | `blockchain.commands` | Volta ao pipeline normal após o delay |

> Para um backoff mais sofisticado (exponencial: 5 s → 30 s → 5 min), criariam-se filas adicionais (`q.retry.30s`, `q.retry.5m`). Para o trabalho, um único nível de retry é suficiente.

---

## 3.6 Bindings

| # | Source Exchange | Routing Key | Destination | Tipo |
|---|---|---|---|---|
| 1 | `blockchain.commands` | `credit.requested` | `q.leader.commands` | direct |
| 2 | `blockchain.commands` | `transaction.requested` | `q.leader.commands` | direct |
| 3 | `blockchain.events` | `#` (todos) | `q.gateway.events` | topic |
| 4 | `blockchain.events` | `#` (todos) | `q.audit.events` | topic |
| 5 | `blockchain.dlx` | (sem chave – fanout) | `q.dlq` | fanout |
| 6 | `blockchain.retry` | `credit.requested` | `q.retry.5s` | direct |
| 7 | `blockchain.retry` | `transaction.requested` | `q.retry.5s` | direct |

**Como o retry funciona na prática:**

1. Consumer (líder) recebe `transaction.requested`.
2. Erro transitório (ex.: indisponibilidade momentânea do MariaDB).
3. Em vez de NACK simples, consumer publica a mensagem em `blockchain.retry` com a routing key original. **Incrementa header `x-retry-count`**.
4. RabbitMQ roteia para `q.retry.5s` via binding 6 ou 7.
5. Após 5 s (TTL), mensagem expira e via DLX volta para `blockchain.commands`.
6. Mensagem é re-entregue ao consumer.
7. Se `x-retry-count >= 3`, consumer publica em `blockchain.dlx` direto (vai pra DLQ).

---

## 3.7 Política de Retry — Resumo

| Situação | Ação |
|---|---|
| Erro transitório (rede, DB momentâneo) | Re-publish em `blockchain.retry` (delay 5 s), incrementa `x-retry-count`. ACK na mensagem original. |
| `x-retry-count >= 3` | Publish direto em `blockchain.dlx`. ACK. Operador inspeciona. |
| Erro de negócio (saldo insuficiente, conta inválida) | **Não retry**. Publica `transaction.rejected` como evento. ACK. |
| Mensagem mal-formada (JSON inválido) | Publish em `blockchain.dlx` imediatamente. ACK. Não tem como melhorar com retry. |

A distinção entre **erro técnico (retry)** e **erro de negócio (rejeitar com evento)** é essencial — retry de erro de negócio causaria reprocessamento infinito.

---

## 3.8 Dead Letter Queue (DLQ)

A `q.dlq` recebe mensagens via `blockchain.dlx` (fanout). Qualquer mensagem que entra lá indica falha que precisa de atenção humana.

**Como uma mensagem chega à DLQ:**

1. Consumer da `q.leader.commands` faz NACK com `requeue=false`.
2. RabbitMQ aplica DLX da fila → publica em `blockchain.dlx`.
3. Fanout → `q.dlq`.

**Outras formas:**

- Mensagem expira por TTL na fila original (60 s sem consumo).
- Fila excede `x-max-length` com `x-overflow=reject-publish` (publish falha — produtor decide o que fazer).
- Consumer publica explicitamente em `blockchain.dlx` (após esgotar retries).

**Inspeção:**

- Tela "DLQ" no Flutter (Etapa de implementação) lista mensagens com payload + headers (incluindo causa da morte: `x-first-death-reason`).
- Painel RabbitMQ Management em `:15672` permite browse manual.

**Operações sobre a DLQ:**

| Ação | Como |
|---|---|
| Visualizar | UI Flutter ou Management Plugin |
| Reprocessar | Mover de volta para `blockchain.commands` (botão na UI) |
| Descartar | Purge da fila (apaga todas) |

---

## 3.9 Segurança

### 3.9.1 Autenticação

- **Mecanismo:** SASL PLAIN (usuário + senha).
- **Em desenvolvimento:** senhas em variáveis de ambiente do `docker-compose.yml`.
- **Em produção:** secrets externos + considerar SASL EXTERNAL com certificados client-side.
- **Listener anônimo desabilitado:** sem `loopback_users.guest = false`, a única forma de acessar é com credenciais válidas.

```ini
# rabbitmq.conf
loopback_users = none
default_user = admin
default_pass = admin
default_vhost = /blockchain
```

### 3.9.2 Autorização

Já detalhada em **3.3** — cada usuário tem regex de permissões `configure/write/read` que limita os recursos acessíveis.

**Exemplo de teste de autorização:** se o serviço de auditoria (`audit`) tentar publicar uma mensagem (em qualquer exchange), o broker rejeita a operação com `ACCESS_REFUSED`. Isso garante que componentes comprometidos não conseguem extrapolar seu papel.

### 3.9.3 Criptografia em trânsito

| Ambiente | TLS | Justificativa |
|---|---|---|
| **Desenvolvimento (Docker local)** | Não | Tráfego entre containers na mesma rede privada Docker; complexidade de gerenciar certificados não compensa. |
| **Produção** | TLS 1.2+ obrigatório | Listener `5671` (AMQPS) substitui `5672`. Certificados emitidos por CA interna ou Let's Encrypt. |

Configuração TLS de exemplo (para referência no relatório, não usada em dev):

```ini
listeners.tcp = none
listeners.ssl.default = 5671
ssl_options.cacertfile = /etc/rabbitmq/certs/ca.crt
ssl_options.certfile   = /etc/rabbitmq/certs/server.crt
ssl_options.keyfile    = /etc/rabbitmq/certs/server.key
ssl_options.verify     = verify_peer
ssl_options.fail_if_no_peer_cert = true
```

### 3.9.4 Criptografia em repouso

- **Mensagens persistentes** (delivery_mode=2) ficam em disco. Em produção, volume cifrado (LUKS, EBS encryption).
- Para o trabalho, persistência vai em volume Docker padrão sem criptografia adicional.

### 3.9.5 Hardening adicional

- **Management plugin** exposto apenas internamente em produção (não publicar porta 15672 para internet).
- **Rate limiting** via política `max-length` nas filas — protege contra publishers maliciosos enchendo o broker.
- **Monitoramento** de filas com profundidade anormal (DLQ crescendo = problema).

---

## 3.10 Arquivo `definitions.json` para import automático

Resumo declarativo de toda a topologia. Vai em `docker/rabbitmq/definitions.json` e é montado no container.

```json
{
  "rabbit_version": "3.12.0",
  "users": [
    {"name": "admin",   "password": "admin",      "tags": ["administrator"]},
    {"name": "gateway", "password": "gateway-pw", "tags": []},
    {"name": "node",    "password": "node-pw",    "tags": []},
    {"name": "audit",   "password": "audit-pw",   "tags": []}
  ],
  "vhosts": [
    {"name": "/blockchain"}
  ],
  "permissions": [
    {"user": "admin",   "vhost": "/blockchain", "configure": ".*", "write": ".*", "read": ".*"},
    {"user": "gateway", "vhost": "/blockchain", "configure": "^$", "write": "^blockchain\\.commands$", "read": "^q\\.gateway\\.events$"},
    {"user": "node",    "vhost": "/blockchain", "configure": "^$", "write": "^blockchain\\.(events|retry|dlx)$", "read": "^q\\.leader\\.commands$"},
    {"user": "audit",   "vhost": "/blockchain", "configure": "^$", "write": "^blockchain\\.(retry|dlx)$", "read": "^q\\.audit\\.events$"}
  ],
  "exchanges": [
    {"name": "blockchain.commands", "vhost": "/blockchain", "type": "direct", "durable": true, "auto_delete": false, "internal": false, "arguments": {}},
    {"name": "blockchain.events",   "vhost": "/blockchain", "type": "topic",  "durable": true, "auto_delete": false, "internal": false, "arguments": {}},
    {"name": "blockchain.dlx",      "vhost": "/blockchain", "type": "fanout", "durable": true, "auto_delete": false, "internal": false, "arguments": {}},
    {"name": "blockchain.retry",    "vhost": "/blockchain", "type": "direct", "durable": true, "auto_delete": false, "internal": true,  "arguments": {}}
  ],
  "queues": [
    {
      "name": "q.leader.commands", "vhost": "/blockchain", "durable": true, "auto_delete": false,
      "arguments": {
        "x-message-ttl": 60000,
        "x-dead-letter-exchange": "blockchain.dlx",
        "x-max-length": 10000,
        "x-overflow": "reject-publish"
      }
    },
    {
      "name": "q.gateway.events", "vhost": "/blockchain", "durable": true, "auto_delete": false,
      "arguments": {"x-max-length": 5000, "x-overflow": "drop-head"}
    },
    {
      "name": "q.audit.events", "vhost": "/blockchain", "durable": true, "auto_delete": false,
      "arguments": {"x-message-ttl": 86400000}
    },
    {
      "name": "q.dlq", "vhost": "/blockchain", "durable": true, "auto_delete": false,
      "arguments": {"x-message-ttl": 604800000}
    },
    {
      "name": "q.retry.5s", "vhost": "/blockchain", "durable": true, "auto_delete": false,
      "arguments": {
        "x-message-ttl": 5000,
        "x-dead-letter-exchange": "blockchain.commands"
      }
    }
  ],
  "bindings": [
    {"source": "blockchain.commands", "vhost": "/blockchain", "destination": "q.leader.commands", "destination_type": "queue", "routing_key": "credit.requested",      "arguments": {}},
    {"source": "blockchain.commands", "vhost": "/blockchain", "destination": "q.leader.commands", "destination_type": "queue", "routing_key": "transaction.requested", "arguments": {}},
    {"source": "blockchain.events",   "vhost": "/blockchain", "destination": "q.gateway.events",  "destination_type": "queue", "routing_key": "#", "arguments": {}},
    {"source": "blockchain.events",   "vhost": "/blockchain", "destination": "q.audit.events",    "destination_type": "queue", "routing_key": "#", "arguments": {}},
    {"source": "blockchain.dlx",      "vhost": "/blockchain", "destination": "q.dlq",             "destination_type": "queue", "routing_key": "",  "arguments": {}},
    {"source": "blockchain.retry",    "vhost": "/blockchain", "destination": "q.retry.5s",        "destination_type": "queue", "routing_key": "credit.requested",      "arguments": {}},
    {"source": "blockchain.retry",    "vhost": "/blockchain", "destination": "q.retry.5s",        "destination_type": "queue", "routing_key": "transaction.requested", "arguments": {}}
  ]
}
```

---

## 3.11 Outros parâmetros de runtime

```ini
# rabbitmq.conf
# Heartbeat AMQP — detecta conexões mortas
heartbeat = 30

# Prefetch global limite — protege contra consumer goloso
channel_max = 256

# Limite de memória — broker para de aceitar publishes se exceder
vm_memory_high_watermark.relative = 0.6

# Limite de disco — broker bloqueia se faltar espaço
disk_free_limit.absolute = 1GB
```

---

## 3.12 Resumo da Etapa 3

| Requisito do enunciado | Onde foi tratado |
|---|---|
| Exchanges | 3.4 |
| Filas | 3.5 |
| Bindings | 3.6 |
| Políticas de retry | 3.7 |
| Dead Letter Queues | 3.8 |
| Autenticação | 3.9.1 |
| Autorização | 3.9.2 |
| Criptografia | 3.9.3 e 3.9.4 |

Toda a configuração é **declarativa e versionada** — o arquivo `definitions.json` está sob controle de versão e é importado automaticamente pelo container RabbitMQ no startup. Garante que qualquer pessoa que clone o repositório obtenha exatamente a mesma topologia ao subir o sistema.
