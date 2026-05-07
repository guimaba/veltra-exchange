# Etapa 1 — Descrição do Cenário

## 1.1 Contextualização

O sistema é uma **plataforma demonstrativa de blockchain financeira distribuída**, construída para fins acadêmicos na disciplina de Sistemas Distribuídos (FURB). Trata-se de uma extensão do projeto já existente (`blockchain_sistemasDistribuidos`), que implementa uma rede de nós em Go com:

- Eleição de líder via algoritmo **Bully**;
- Mineração de blocos com **Prova de Trabalho (PoW)** simplificada;
- Persistência em **MariaDB**;
- Comunicação interna entre nós via **RPC**.

Sobre essa base, esta etapa adiciona uma **camada de mensageria assíncrona com RabbitMQ** e uma **interface gráfica em Flutter Web**, permitindo que usuários finais (ou sistemas externos de auditoria, monitoramento etc.) interajam com a rede sem acoplamento direto aos nós.

### Domínio simulado: carteira de crédito

Para tornar o cenário concreto sem desviar do foco em mensageria, o domínio modelado é uma **carteira financeira simulada**:

- Cada conta possui um **saldo virtual**.
- O usuário pode **adicionar crédito** à própria conta (operação puramente simulada — não há integração com pagamento real, gateway PSP, Pix, cartão ou qualquer infraestrutura financeira de fato).
- O usuário pode **transferir valores** entre contas, gerando transações que entram no MemPool da blockchain e são mineradas em blocos.
- Antes de aceitar uma transferência, o sistema **valida o saldo** do remetente com base no histórico do ledger (soma de créditos − soma de débitos).

> ⚠️ **Aviso importante:** toda a camada financeira é demonstrativa. O foco do trabalho é demonstrar a comunicação assíncrona via RabbitMQ entre os componentes do sistema distribuído. Nenhum dinheiro real é movimentado.

### Atores do sistema

| Ator | Papel |
|---|---|
| **Usuário final** | Interage via Flutter Web: adiciona crédito, envia transações, acompanha o estado da rede em tempo real. |
| **Gateway HTTP/WebSocket** | Ponte entre o Flutter (browser) e o RabbitMQ. Expõe API REST e faz broadcast de eventos via WebSocket. |
| **Nós da blockchain (Go)** | Validam transações, mantêm o ledger, executam mineração, mantêm consenso via Bully. O nó **líder** é o único produtor de blocos. |
| **RabbitMQ** | Backbone de mensageria. Roteia comandos, eventos de domínio e mensagens de auditoria. |
| **MariaDB** | Persistência dos blocos e do ledger. |

## 1.2 Justificativa da comunicação assíncrona

A escolha por mensageria assíncrona (RabbitMQ) em vez de chamadas síncronas diretas (RPC/HTTP) entre o gateway e os nós da blockchain se justifica por **cinco motivos arquiteturais**:

### 1. Mineração é uma operação demorada e bloqueante

A mineração com PoW pode levar de milissegundos a vários segundos (mesmo com dificuldade baixa). Manter o cliente HTTP travado esperando o bloco ser minerado é inviável em escala — gera timeouts, conexões abertas e má experiência de usuário. Com mensageria, o cliente envia a transação e recebe imediatamente um *acknowledgement* "transação aceita e enfileirada"; o resultado da mineração chega depois, via WebSocket, quando o bloco for produzido.

### 2. Múltiplos consumidores reagem ao mesmo evento

Quando um bloco é minerado, vários componentes precisam reagir:

- A **interface Flutter** atualiza o dashboard;
- O **serviço de auditoria** persiste para análise futura;
- Sistemas externos de monitoramento podem ser alertados;
- Outros nós da rede precisam sincronizar (já feito via RPC, mas eventos podem complementar para clientes externos).

Comunicação síncrona ponto-a-ponto exigiria que o produtor (nó líder) conhecesse cada consumidor — acoplando rigidamente a arquitetura. Com um *exchange* RabbitMQ no estilo *publish/subscribe*, o líder publica um único evento e qualquer interessado se inscreve.

### 3. Desacoplamento temporal e tolerância a falhas

Se a interface Flutter estiver fora do ar, o líder ainda pode minerar blocos normalmente — as mensagens ficam acumuladas na fila e são entregues quando o consumidor voltar. Com chamadas síncronas, a indisponibilidade de qualquer ponto bloqueia a cadeia.

A combinação de **filas duráveis + ACK manual + Dead Letter Queue (DLQ)** garante que mensagens não sejam perdidas em caso de:

- Crash de consumidor durante o processamento;
- Erro transitório em integração externa;
- Falha de rede entre componentes.

### 4. Nivelamento de carga (load leveling)

Se o usuário disparar 1.000 transações em pico através do Flutter, o nó líder não precisa processá-las todas instantaneamente. As mensagens ficam na fila e o líder as consome no ritmo que sua capacidade de mineração permite, sem derrubar a aplicação.

### 5. Resiliência durante eleição Bully

Quando o líder cai, o algoritmo Bully elege um novo líder — esse processo leva alguns segundos. Durante esse intervalo, transações enviadas pelos usuários **não podem ser perdidas**. Com RabbitMQ, elas ficam na fila `q.leader.commands` aguardando o próximo líder consumi-las. Sem mensageria, essas requisições retornariam erro 5xx e o usuário precisaria reenviar manualmente.

## 1.3 Eventos e comandos modelados

| Categoria | Nome | Origem | Destino(s) | Descrição |
|---|---|---|---|---|
| Comando | `credit.requested` | Gateway (Flutter) | Nó líder | Pedido de adicionar crédito a uma conta. |
| Comando | `transaction.requested` | Gateway (Flutter) | Nó líder | Pedido de transferência entre contas. |
| Evento | `credit.added` | Nó líder | Gateway, auditoria | Crédito foi confirmado e registrado. |
| Evento | `transaction.received` | Nó líder | Gateway, auditoria | Transação válida adicionada ao MemPool. |
| Evento | `transaction.rejected` | Nó líder | Gateway, auditoria | Transação recusada (saldo insuficiente, conta inválida etc.). |
| Evento | `block.mined` | Nó líder | Gateway, auditoria | Novo bloco minerado e propagado. |
| Evento | `leader.changed` | Nó eleito | Gateway, auditoria | Nova eleição Bully concluída. |

A **distinção entre comando e evento** segue o padrão CQRS:

- **Comandos** representam *intenções* ("quero adicionar crédito"); têm um único consumidor (o líder).
- **Eventos** representam *fatos consumados* ("crédito foi adicionado"); podem ter múltiplos consumidores.

## 1.4 Resumo do cenário

O sistema demonstra como utilizar RabbitMQ como **espinha dorsal de eventos** de uma blockchain distribuída, permitindo:

- Interface web reativa em tempo real (Flutter Web + WebSocket);
- Desacoplamento total entre clientes externos e os nós da blockchain;
- Resiliência durante operações lentas (mineração) e troca de líder (Bully);
- Capacidade de adicionar novos consumidores (auditoria, métricas, alertas) sem modificar os produtores existentes.

A próxima etapa (Arquitetura) detalha como esses elementos se articulam concretamente em termos de exchanges, filas, bindings e fluxo de mensagens.
