# Etapa 2 — Arquitetura da Solução

## 2.1 Visão geral

A solução adiciona uma camada de mensageria assíncrona (RabbitMQ) entre a interface Flutter Web e a rede de nós da blockchain Go. Os nós continuam se comunicando entre si por **RPC** (eleição Bully, sincronização de blocos), enquanto o **RabbitMQ assume o papel de barramento de eventos e comandos** entre o mundo externo (Flutter, auditoria) e o líder da blockchain.

### Componentes

| Componente | Tecnologia | Papel | Quantidade |
|---|---|---|---|
| **Nó Blockchain** | Go | Mantém ledger, executa Bully, minera blocos. Apenas o líder produz blocos. | 3 (escalável) |
| **MariaDB** | MariaDB 10+ | Persistência do ledger e metadados. | 1 (compartilhado) |
| **RabbitMQ** | RabbitMQ 3.12+ com plugin management | Roteamento de mensagens (exchanges, filas, DLQ). | 1 |
| **Gateway** | Go (`net/http` + `gorilla/websocket`) | Expõe REST/WS para o Flutter; faz ponte com Rabbit. Serve também os arquivos estáticos do Flutter Web. | 1 (escalável horizontalmente) |
| **Flutter Web** | Flutter 3.x (build web) | Interface gráfica no navegador. | N (clientes) |
| **Auditoria** | Go (consumer simples) | Persiste todos os eventos para análise/relatório. | 1 |

## 2.2 Topologia RabbitMQ

### Exchanges

| Nome | Tipo | Propósito |
|---|---|---|
| `blockchain.commands` | `direct` | Recebe comandos do Gateway destinados ao líder. Roteamento por chave exata. |
| `blockchain.events` | `topic` | Publica eventos de domínio (`block.mined`, `transaction.received`, `leader.changed`, etc.). Consumidores se inscrevem por padrões (`block.*`, `transaction.*`, `#`). |
| `blockchain.dlx` | `fanout` | Dead Letter Exchange — recebe mensagens descartadas após esgotar retries. |

A separação **commands vs. events** segue o estilo CQRS: comandos são pedidos com um único processador (o líder); eventos são fatos consumidos por N partes interessadas.

### Filas

| Fila | Bind | Consumidor | Características |
|---|---|---|---|
| `q.leader.commands` | `blockchain.commands` (`credit.requested`, `transaction.requested`) | Nó líder | `durable=true`, `x-dead-letter-exchange=blockchain.dlx`, `x-message-ttl=60000` |
| `q.gateway.events` | `blockchain.events` (`#`) | Gateway (todos os instances) | `durable=true`, exclusive=false, prefetch=10 |
| `q.audit.events` | `blockchain.events` (`#`) | Auditoria | `durable=true`, prefetch=50 |
| `q.dlq` | `blockchain.dlx` (fanout) | Inspeção manual / tela DLQ no Flutter | `durable=true` |

### Diagrama lógico

```
                ┌─────────────────────────┐
   POST /tx     │                         │   credit.requested
  ┌──────────►──┤   blockchain.commands   ├──────────────────────► q.leader.commands ──► [Nó Líder]
  │             │       (direct)          │   transaction.requested                          │
  │             └─────────────────────────┘                                                  │
  │                                                                                          │
  │                                                                                          │ publica
  │             ┌─────────────────────────┐                                                  ▼
  │             │                         │  block.mined          ┌──────────────────┐
  │ WebSocket   │   blockchain.events     │  transaction.received │                  │
  │ ◄───────────┤        (topic)          │  transaction.rejected │                  │
  │             │                         │  credit.added         │                  │
  │             │                         │  leader.changed       │                  │
  │             └────────────┬────────────┘                       │                  │
  │                          │                                    │                  │
  │                          ├──► q.gateway.events ──► [Gateway] ─┘                  │
  │                          ├──► q.audit.events ───► [Auditoria]                    │
  │                          │                                                       │
  │             ┌────────────▼────────────┐                                          │
  │             │   blockchain.dlx        │                                          │
  │             │      (fanout)           │                                          │
  │             └────────────┬────────────┘                                          │
  │                          │                                                       │
  │                          └──► q.dlq ──► [Inspeção / Flutter]                     │
  │                                                                                  │
  └─────────────── [Flutter Web] ────────────────────────────────────────────────────┘
```

## 2.3 Produtores e consumidores

### Produtores

| Produtor | Publica em | Routing keys |
|---|---|---|
| **Gateway** | `blockchain.commands` | `credit.requested`, `transaction.requested` |
| **Nó líder** | `blockchain.events` | `credit.added`, `transaction.received`, `transaction.rejected`, `block.mined` |
| **Qualquer nó eleito** | `blockchain.events` | `leader.changed` |

### Consumidores

| Consumidor | Fila | Comportamento |
|---|---|---|
| **Nó líder** | `q.leader.commands` | Processa comandos: valida saldo (no caso de transação), adiciona ao MemPool, dispara mineração. Faz ACK só após gravar no MemPool. |
| **Gateway** | `q.gateway.events` | Repassa todos os eventos via WebSocket para os clientes Flutter conectados. Stateless — pode haver múltiplas instâncias. |
| **Auditoria** | `q.audit.events` | Persiste cada evento no MariaDB com timestamp para relatório/análise. |

## 2.4 Fluxo principal — Transferência de valores

Cenário: usuário Alice transfere R$ 50 para Bob.

```
1. [Flutter] Usuário preenche form "Transferir 50 para Bob" e clica "Enviar"
2. [Flutter] HTTP POST /transactions {sender: "alice", receiver: "bob", amount: 50}
3. [Gateway] Recebe POST, valida formato, gera UUID, publica em blockchain.commands
              routing_key=transaction.requested, body={tx_id, sender, receiver, amount, timestamp}
4. [Gateway] Retorna 202 Accepted {tx_id} imediatamente para o Flutter
5. [RabbitMQ] Roteia mensagem para q.leader.commands
6. [Nó Líder] Consome a mensagem
7. [Nó Líder] Calcula saldo de Alice a partir do ledger (créditos − débitos)
8. [Nó Líder] CASO saldo suficiente:
              - Adiciona transação ao MemPool
              - Publica evento transaction.received em blockchain.events
              - ACK para o Rabbit
              - Dispara mineração em goroutine separada
9. [Nó Líder] CASO saldo insuficiente:
              - Publica evento transaction.rejected com motivo
              - ACK (não retorna pra fila — mensagem foi processada com falha de negócio)
10.[RabbitMQ] Faz fanout do evento para q.gateway.events e q.audit.events
11.[Gateway] Recebe evento, broadcast via WebSocket para todos os clientes conectados
12.[Flutter] Recebe via WS, atualiza UI ("Transação enfileirada" ou "Saldo insuficiente")
13.[Nó Líder] Mineração conclui, publica block.mined com lista de transações inclusas
14.[RabbitMQ] Roteia para q.gateway.events e q.audit.events
15.[Gateway] Broadcast WebSocket
16.[Flutter] Atualiza dashboard com novo bloco e saldos atualizados
```

Fluxo análogo se aplica para `credit.requested → credit.added` (sem validação de saldo, já que crédito é simulado).

## 2.5 Estratégias de escalabilidade

### Horizontal — Gateway

O gateway é **stateless**. Múltiplas instâncias podem consumir simultaneamente da fila `q.gateway.events` em modo **competing consumers** (load balance natural do AMQP). Cada instância mantém suas próprias conexões WebSocket com clientes Flutter.

> Em produção real, seria necessário pinning ou broker pub/sub interno (ex.: Redis) para garantir que um evento chegue em todos os clientes mesmo se conectados a gateways diferentes. Para o trabalho, basta uma instância única.

### Vertical — Nó líder

Apenas o líder consome `q.leader.commands`. Para aumentar throughput:

- **Prefetch count** ajustável (default: 1, para garantir ordenação) — pode ser elevado se a aplicação tolerar processamento concorrente.
- **Batching de transações** no MemPool antes de minerar (já implementado no projeto base).
- **Dificuldade de PoW** baixa (3) — calibrado para máquinas locais.

### Auditoria desacoplada

Adicionar novos consumidores (alertas, métricas, ML) é trivial: basta criar uma nova fila com bind no exchange `blockchain.events`. Nenhum código existente precisa mudar.

## 2.6 Confiabilidade

| Mecanismo | Aplicação |
|---|---|
| **Filas duráveis** (`durable=true`) | Mensagens sobrevivem a restart do RabbitMQ. |
| **Mensagens persistentes** (`delivery_mode=2`) | Gravadas em disco, não só na RAM. |
| **Publisher confirms** | Garantia de que o broker recebeu a mensagem. Cliente Go usa `channel.Confirm(false)`. |
| **ACK manual** | Consumidor só dá ACK após processar com sucesso. Crash durante processamento → mensagem é re-entregue. |
| **Idempotência** | Cada comando tem `tx_id` (UUID). Se a mesma mensagem chegar duas vezes (re-entrega), o consumidor verifica se já foi processada antes de aplicar efeito. |
| **TLS** entre clientes e broker | Em produção. Localmente (Docker) usa porta 5672 sem TLS para simplicidade. |
| **Autenticação** | Usuários distintos para gateway, nós e auditoria, com permissões mínimas (vhost separado). |

## 2.7 Tolerância a falhas

### Falha do consumidor

- Mensagem volta automaticamente para a fila se o canal fechar sem ACK.
- Após N tentativas falhadas (TTL ou retry counter), mensagem vai para `q.dlq` via DLX.
- Operador inspeciona a DLQ via UI Flutter ou painel RabbitMQ.

### Falha do líder (Bully)

1. Líder atual cai (kill -9 ou crash).
2. Outros nós detectam falha de heartbeat (já existente).
3. Bully elege novo líder.
4. **Mensagens em `q.leader.commands` permanecem na fila** durante a eleição.
5. Novo líder se conecta como consumidor de `q.leader.commands` e processa o backlog.
6. Novo líder publica `leader.changed` em `blockchain.events` para informar Gateway/Auditoria.

> **Detalhe importante:** apenas o líder consome `q.leader.commands`. Para implementar isso, cada nó verifica seu estado interno (`isLeader`) — quando vira líder, registra um consumidor; quando perde a liderança, cancela. Garante exclusividade sem precisar de lock externo.

### Retry com backoff

- Exchange auxiliar `blockchain.retry` (TTL fixo, ex: 5 s) com DLX apontando de volta para a fila original.
- Em vez de NACK direto, publicador envia para `blockchain.retry`; após o TTL, mensagem volta para a fila normal.
- Após 3 retries, mensagem segue para `q.dlq`.

### Falha do RabbitMQ

- Filas duráveis + mensagens persistentes garantem que nada é perdido em restart limpo.
- Cliente Go (gateway, nós) implementa **reconexão automática** com backoff exponencial.
- Em produção real, usar cluster RabbitMQ com mirrored queues. Para o trabalho, instância única.

## 2.8 Padrões de mensagem

Todas as mensagens são **JSON UTF-8** com schema versionado:

```json
{
  "schema": "blockchain.transaction.requested.v1",
  "tx_id": "550e8400-e29b-41d4-a716-446655440000",
  "timestamp": "2025-05-08T14:32:11Z",
  "payload": {
    "sender": "alice",
    "receiver": "bob",
    "amount": 50.0
  }
}
```

A justificativa da escolha de JSON (vs. Avro/Protobuf) está detalhada na Etapa 5.

## 2.9 Resumo arquitetural

A solução combina:

- **RPC** (já existente) entre nós da blockchain para consenso e sincronização de blocos.
- **AMQP/RabbitMQ** entre clientes externos (Gateway/Flutter) e o líder, e entre o líder e consumidores reativos (Auditoria, UI).
- **HTTP REST** para comandos one-shot do Flutter para o Gateway.
- **WebSocket** para push de eventos do Gateway para o Flutter em tempo real.

Essa estratificação separa claramente:

- **Consenso interno** (RPC, alto controle, baixa latência) →
- **Comandos externos** (HTTP/AMQP, idempotência, retry) →
- **Notificação reativa** (AMQP/WebSocket, fan-out, real-time UX).

A próxima seção (Diagramas) ilustra visualmente os fluxos descritos aqui. As Etapas 3, 4 e 5 detalham configuração concreta, casos de uso e considerações técnicas finais.
