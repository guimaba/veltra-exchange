<div align="center">

UNIVERSIDADE REGIONAL DE BLUMENAU — FURB
Disciplina de Sistemas Distribuídos

# VELTRA EXCHANGE

### Exchange Spot Simulada com Matching Engine Determinístico, Integridade Contábil de Dupla Entrada e Infraestrutura como Código

Guilherme Maba
Enzo Rocha
Igor Schmidt

Blumenau
2026

</div>

---

## RESUMO

Este trabalho apresenta a **Veltra Exchange**, uma plataforma de negociação spot simulada desenvolvida como projeto final da disciplina de Sistemas Distribuídos. O sistema implementa um motor de casamento de ordens determinístico (*Central Limit Order Book* — CLOB) com prioridade preço-tempo, eleição de líder pelo algoritmo Bully e persistência tolerante a falhas via *Write-Ahead Log* (WAL) com *snapshots* e *replay*. A camada financeira adota um **ledger de dupla entrada** em PostgreSQL com **invariante de soma zero**, **idempotência** por par (conta, referência) resistente a reentregas do *broker*, **liquidação atômica** de cada negociação e **reserva de saldo pré-negociação (*hold*) atômica**, eliminando condições de corrida. A integridade é auditável por **Merkle root** periódica. Toda a comunicação é assíncrona via RabbitMQ no padrão *Event Sourcing + CQRS*. O sistema oferece autenticação JWT, dados de mercado reais de 33 criptomoedas (CoinGecko), interface Flutter Web completa e **infraestrutura como código (Terraform + Terragrunt)** para implantação em AWS (ECS Fargate, Amazon MQ, RDS PostgreSQL). O escopo entregue privilegia a **profundidade do núcleo distribuído e a integridade contábil** em detrimento da largura de funcionalidades de produto.

**Palavras-chave:** Sistemas Distribuídos. Matching Engine. CLOB. Event Sourcing. Idempotência. Dupla Entrada. Algoritmo Bully. Write-Ahead Log. RabbitMQ. Infraestrutura como Código.

---

## SUMÁRIO

1. INTRODUÇÃO
2. DESENVOLVIMENTO
   - 2.1 Arquitetura Geral
   - 2.2 Motor de Casamento (CLOB)
   - 2.3 Tolerância a Falhas: Bully + WAL
   - 2.4 Ledger de Dupla Entrada e Integridade Contábil
   - 2.5 Idempotência e Consistência Eventual
   - 2.6 Auditoria e Merkle Root
   - 2.7 Sistema de Mensageria
   - 2.8 Market Data e Liquidez
   - 2.9 Autenticação e Gateway
   - 2.10 Interface Flutter Web
   - 2.11 Infraestrutura como Código (AWS)
3. CONCLUSÕES
   - 3.1 Resultados
   - 3.2 Limitações e Fronteiras de Escopo
   - 3.3 Dificuldades Técnicas
   - 3.4 Trabalho Futuro
- REFERÊNCIAS

---

## 1 INTRODUÇÃO

Exchanges de criptomoedas estão entre os sistemas distribuídos mais exigentes em produção: demandam processamento determinístico de ordens, consistência entre réplicas, persistência financeira auditável e latência mínima. A Veltra Exchange foi concebida como simulador educacional que replica essa arquitetura com ativos fictícios (VLT e USDT-sim) e dados de mercado reais da API pública CoinGecko, **sem dinheiro real e sem custódia real** — todo saldo é crédito virtual emitido internamente.

O objetivo foi aplicar, de forma integrada, os conceitos centrais da disciplina: **eleição de líder, tolerância a falhas, *event sourcing*, mensageria assíncrona, idempotência e consistência eventual**. A implementação evoluiu um projeto blockchain anterior (Go + RabbitMQ), reaproveitando a camada de mensageria, a eleição Bully e o padrão *command/event*, e os adaptou para uma *Centralized Exchange* (CEX).

**Delimitação de escopo (decisão de engenharia).** Exchanges reais possuem dezenas de funcionalidades de produto (ordens *stop-loss*/OCO, *circuit breakers*, *surveillance* de manipulação, chaves de API com HMAC, *rate limiting*, cache distribuído). Em vez de implementar uma fatia rasa e ampla, o projeto priorizou **três eixos verticais e profundos**, que são precisamente onde residem os desafios de Sistemas Distribuídos:

1. **Núcleo de negociação determinístico** — CLOB *single-threaded* por par, com *replay* exato a partir do log;
2. **Integridade contábil auditável** — dupla entrada com invariante de soma zero, *hold* atômico, idempotência e prova por Merkle root;
3. **Infraestrutura como código reprodutível** — implantação distribuída em AWS descrita e validada via Terraform/Terragrunt.

As funcionalidades fora desses eixos são apresentadas explicitamente como fronteiras de escopo na Seção 3.2 e como trabalho futuro na Seção 3.4.

O resultado é um sistema funcional orquestrado por Docker Compose (13 contêineres, sendo 9 da exchange e 4 da base blockchain legada reaproveitada), com aproximadamente 16,9 mil linhas de código (≈ 8,5 mil Go + 7,1 mil Dart + 1,3 mil de IaC), motor de matching, ledger contábil, autenticação, 33 pares de negociação com preços reais, interface web completa e infraestrutura AWS codificada.

---

## 2 DESENVOLVIMENTO

### 2.1 Arquitetura Geral

O sistema adota arquitetura orientada a eventos com serviços Go independentes, orquestrados por Docker Compose. A comunicação é inteiramente assíncrona via RabbitMQ, seguindo o padrão *Command/Event Sourcing + CQRS* (KLEPPMANN, 2017): comandos são publicados em uma *exchange* de comandos e consumidos pelo matching engine; este, **única fonte da verdade**, emite eventos imutáveis que alimentam todas as projeções (ledger, *market data*, auditoria, interface).

| Camada | Serviço | Responsabilidade |
|---|---|---|
| Borda | Gateway HTTP/WS | REST, WebSocket, autenticação JWT, OMS pré-negociação |
| Núcleo | Matching Engine (×3) | CLOB, eleição Bully, WAL, *failover* |
| Persistência | Ledger | Dupla entrada, liquidação atômica, idempotência, Merkle |
| Dados | Market Data | CoinGecko (30 s), 33 moedas, candles OHLCV, semeador de liquidez |
| Auditoria | Audit | Trilha imutável de todos os eventos da exchange |
| Infraestrutura | RabbitMQ + PostgreSQL | Mensageria e persistência financeira |

<div align="center"><sub>Quadro 1 — Serviços da Veltra Exchange e suas responsabilidades</sub></div>

A comunicação usa dois *topic exchanges* no RabbitMQ: `veltra.commands` (ordens e cancelamentos) e `veltra.events` (`trade.executed`, `order.filled`, `book.updated`, `ledger.posted`, `market.update`, `faucet.credit`). Cada serviço assina apenas o subconjunto de eventos de que precisa, desacoplando completamente as camadas (conforme o Quadro 2, na Seção 2.7).

### 2.2 Motor de Casamento (CLOB)

O coração do sistema é um *Central Limit Order Book* com prioridade **preço-tempo**. Os *bids* são mantidos em ordem decrescente de preço e os *asks* em ordem crescente; dentro de um mesmo nível de preço, a fila é **FIFO estrita ancorada no campo `Order.Sequence`** — um contador monotônico atribuído na ingestão, **nunca no relógio do sistema**. Isso garante determinismo total: a mesma sequência de entradas produz exatamente os mesmos *trades* em qualquer réplica.

O motor suporta os tipos `limit` e `market`, as políticas de tempo GTC (*Good Till Cancel*), IOC (*Immediate or Cancel*) e FOK (*Fill or Kill*), além de **prevenção de auto-negociação** (*self-trade prevention*): quando *taker* e *maker* pertencem à mesma conta, o *maker* é cancelado e o *taker* prossegue. A execução ocorre ao preço do *maker* (*price improvement* para o *taker*).

```go
func (e *Engine) match(taker *exchange.Order) []exchange.Trade {
    var trades []exchange.Trade
    for {
        best := e.book.BestOpposite(taker.Side)
        if best == nil || !crosses(taker, best) { break }
        fill := min(taker.RemainingQty(), best.RemainingQty())
        trades = append(trades, makeTrade(best, taker, fill))
        if taker.IsFilled() { break }
    }
    return trades
}
```

<div align="center"><sub>Código 1 — Núcleo do algoritmo de casamento (simplificado)</sub></div>

Uma decisão central foi **proibir `time.Now()` e `math/rand` com semente temporal** no pacote `pkg/matching`. Qualquer fonte de não-determinismo quebraria a propriedade de *replay* — essencial para a recuperação via WAL. *Timestamp* e identificador de *trade* derivam da sequência da ordem, não do relógio.

### 2.3 Tolerância a Falhas: Bully + WAL

Três réplicas do matching engine competem pela liderança via **algoritmo Bully** (GARCIA-MOLINA, 1982): o nó com maior ID vence a eleição. O líder é o único a consumir a fila `q.matching.commands`, com `prefetch=1`, garantindo **serialização total** (uma mensagem por vez). Os *standbys* aguardam com o *consumer* pausado, prontos para assumir.

Toda ordem é gravada no **Write-Ahead Log** com `fsync` **antes** de ser aplicada ao livro de ordens. *Snapshots* periódicos truncam o WAL para evitar crescimento ilimitado. Na promoção de um *standby*, `OpenJournal` restaura o último *snapshot* e faz *replay* dos registros remanescentes, reconstruindo o estado exato.

O mecanismo foi validado experimentalmente: após a parada do contêiner líder, a réplica seguinte venceu a eleição em menos de 15 segundos, recuperou o WAL e executou corretamente uma ordem de mercado contra a ordem parcialmente preenchida existente antes da falha; o contador de *trades* permaneceu contínuo. O WAL reside em volume compartilhado entre as réplicas — não como escrita concorrente (apenas o líder escreve), mas como mecanismo de **transferência de estado na promoção**.

### 2.4 Ledger de Dupla Entrada e Integridade Contábil

Todo movimento financeiro segue o princípio contábil da **dupla entrada**: cada operação gera um par débito/crédito que **soma zero**. O invariante `SUM(postings.amount) = 0` por ativo é a garantia de integridade do sistema e foi validado experimentalmente.

O serviço `cmd/ledger` consome `trade.executed` do RabbitMQ e executa a liquidação. Três propriedades foram tratadas com rigor:

**a) Liquidação atômica.** Um *trade* gera quatro lançamentos (comprador recebe *base*, comprador paga *quote*, vendedor recebe *quote*, vendedor entrega *base*). As pernas *base* e *quote* são gravadas em uma **única transação ACID** — ou ambas entram, ou nenhuma —, nunca um meio-*trade*.

**b) Aritmética inteira.** Preço e quantidade trafegam como `int64` escalado (`scale = 10⁸`). O notional é calculado por `money.Notional`, que multiplica **antes** de dividir pela escala usando `math/big`, eliminando qualquer truncamento de preço.

**c) *Hold* pré-negociação atômico.** Antes de encaminhar uma ordem ao motor, o gateway **reserva** o saldo necessário de forma **atômica e condicional**, em uma única instrução:

```sql
UPDATE ledger.accounts
   SET reserved = reserved + $1
 WHERE id = $2 AND (balance - reserved) >= $1;
```

Como a verificação de suficiência e a reserva ocorrem na mesma instrução, **elimina-se a janela de corrida (TOCTOU)**: duas ordens concorrentes não conseguem reservar o mesmo saldo. O saldo disponível é a coluna derivada `available = balance - reserved`. A reserva é liberada quando a ordem atinge estado terminal (preenchida, cancelada ou rejeitada), evento detectado pelo gateway e propagado por `client_order_id`.

### 2.5 Idempotência e Consistência Eventual

RabbitMQ entrega mensagens com semântica *at-least-once*: reentregas (reconexão, ACK perdido, falha entre a aplicação e a confirmação) são o caso normal, não a exceção. Sem proteção, uma reentrega de `trade.executed` liquidaria o mesmo *trade* duas vezes, corrompendo saldos silenciosamente.

A solução é **idempotência no nível do banco**. A tabela de lançamentos possui o índice único `UNIQUE(ledger_account_id, reference_id)`, e toda inserção usa `ON CONFLICT DO NOTHING`. Como débito e crédito de um mesmo par compartilham a `reference_id` em **contas diferentes**, eles não colidem entre si; já uma reentrega da mesma operação colide e é descartada sem reaplicar saldo. As ordens também carregam `client_order_id` idempotente, herdado do mecanismo de `TxID` do projeto base. O *pipeline* de retry republica com *backoff* de 5 s por até três tentativas; após isso, a mensagem é roteada para a *Dead Letter Queue*.

Essa combinação — log de eventos imutável como fonte da verdade, projeções reconstruíveis e idempotência sob reentrega — concretiza a **consistência eventual** entre o motor, o ledger e a interface.

### 2.6 Auditoria e Merkle Root

A trilha de auditoria materializa a propriedade "o log de eventos permite reconstruir e auditar qualquer estado". O serviço `cmd/audit` consome **todos** os eventos da exchange (binding `#` na fila `q.exchange.audit`) e persiste os eventos de negócio.

Periodicamente, o ledger computa a **Merkle root** dos lançamentos de cada período (SHA-256, estilo Bitcoin) e a persiste em `ledger.merkle_roots`. A folha da árvore é **determinística** — composta pelos campos do lançamento, **sem o `created_at`** (relógio de parede) —, de modo que o mesmo conjunto de lançamentos sempre produz a mesma raiz, reproduzível por *replay*. O endpoint `GET /api/veltra/merkle` expõe as raízes por período, habilitando **provas de auditoria sem revelar o razão completo**.

### 2.7 Sistema de Mensageria

O RabbitMQ organiza o domínio em dois *topic exchanges*. *Bindings* por fila garantem que cada serviço receba apenas o necessário.

| Fila | Consumidor | Eventos roteados |
|---|---|---|
| `q.matching.commands` | Matching Engine (líder) | `order.place`, `order.cancel` |
| `q.ledger.events` | Ledger | `trade.executed`, `faucet.credit` |
| `q.marketdata.events` | Gateway | `book.updated`, `trade.executed`, `market.update`, … |
| `q.exchange.audit` | Audit | todos os eventos (`#`) |

<div align="center"><sub>Quadro 2 — Filas RabbitMQ e seus consumidores</sub></div>

Toda mensagem é encapsulada em um *Envelope* com UUID (`TxID`), *schema* versionado, *timestamp* e *payload* JSON. Erros permanentes vão direto à DLQ sem *retry*; erros transitórios entram no *pipeline* de *backoff*.

### 2.8 Market Data e Liquidez

O serviço `cmd/marketdata` consulta a API gratuita da CoinGecko a cada 30 segundos para 33 símbolos, obtendo preço em USD/BRL, variação de 24 h, volume e *market cap*. O parâmetro `precision=full` é essencial para micro-preços (PEPE, SHIB, BONK) sem truncamento. O preço do VLT é derivado do ATOM (Cosmos) por um fator fixo, referência interna que ancora o token em um ativo real sem expô-lo na listagem. *Candles* OHLCV de 5 minutos são geradas por *random walk* determinístico (semente por símbolo).

**Todos os pares são negociados pelo matching engine real** — não há fills simulados fora do motor. Para que os pares do catálogo possuam liquidez desde a inicialização, um **semeador de liquidez** financia uma conta `liquidity` via *faucet* e publica ordens-limite *resting* em ambos os lados de cada par em torno do preço de referência. Assim, uma ordem de um usuário casa contra liquidez real no CLOB, gerando `trade.executed` legítimo, com WAL e liquidação de dupla entrada — exatamente como o par nativo VLT/USDT-sim.

### 2.9 Autenticação e Gateway

O gateway é o único ponto de entrada HTTP e WebSocket. A autenticação usa **JWT HS256** com TTL de 24 h; senhas são armazenadas com **bcrypt**. Sessões são registradas no PostgreSQL e invalidadas no *logout*. O token carrega os *claims* `username`, `account_id` e `is_admin`, eliminando *lookups* adicionais por requisição. **A conta de negociação vem sempre da sessão autenticada, nunca do *payload***, fechando a superfície para falsificação de identidade. O OMS pré-negociação (Seção 2.4) opera neste serviço.

### 2.10 Interface Flutter Web

A interface foi construída em Flutter Web com tema *dark* inspirado em exchanges profissionais. Telas principais: **Landing** (preços ao vivo + login); **Trading** (par nativo VLT/USDT-sim com *order book* real, gráfico, formulário de compra/venda com validação de saldo, ordens abertas e cancelamento; demais pares do catálogo negociáveis pelo motor com visualização de profundidade de referência); **Mercado** (33 moedas com *sparklines*, filtros *gainers/losers*); **Carteira** (saldos lidos diretamente do PostgreSQL, portfólio em BRL); **Depósito simulado**; e **Admin** (KPIs, usuários, *faucet*, monitor de eventos ao vivo). O estado de saldo é gerenciado por um `BalanceState` que busca dados do PostgreSQL, resolvendo a volatilidade da projeção *in-memory* do gateway entre reinicializações.

### 2.11 Infraestrutura como Código (AWS)

A implantação distribuída na AWS é descrita por **Terraform modular** com uma camada **Terragrunt** fina por ambiente (`dev`, `demo`), reaproveitando o mesmo *stack* e variando apenas os *inputs* (DRY sem duplicar HCL). O mapeamento da topologia local para a nuvem segue o plano técnico (§4.1/§6):

| Docker Compose | AWS |
|---|---|
| `rabbitmq` | Amazon MQ (RabbitMQ) |
| `postgres` | RDS PostgreSQL (*encryption at rest*) |
| `gateway` | ECS Fargate + *Application Load Balancer* (HTTP/WS) |
| `matching` | ECS Fargate (tarefa única — *single-writer*) |
| `ledger`, `marketdata` | ECS Fargate (*Spot*) |

<div align="center"><sub>Quadro 3 — Mapeamento da topologia local para a AWS</sub></div>

Módulos: `network` (VPC, *subnets* públicas/privadas, *security groups*, **VPC Endpoints** que dispensam NAT Gateway), `ecr`, `rds`, `mq`, `ecs-cluster` (*Cloud Map* para *service discovery* e papéis IAM de **menor privilégio**), `ecs-service` (genérico, reutilizado por todos os serviços) e `alb`. Boas práticas aplicadas: **segredos no AWS Secrets Manager** (senha do RDS, credenciais do MQ e segredo JWT gerados via `random_password` e referenciados por ARN — **nunca no *state***); *state* remoto em S3 com *lock* nativo; `default_tags` para rastreio de custo; instâncias mínimas, *single-AZ* e Fargate *Spot* para conter gastos; recursos totalmente destrutíveis.

**Decisão consciente — matching em ECS.** No Compose, as três réplicas com Bully + WAL compartilhado demonstram *failover*. Em ECS, executa-se **uma única tarefa** de matching (o motor é *single-writer* por construção): a alta disponibilidade vem do orquestrador reiniciando a tarefa e recuperando do WAL, evitando a latência de um sistema de arquivos de rede no caminho de `fsync` e o risco de *split-brain*. A IaC foi **validada** com `terraform validate` e `terraform fmt`; o `apply` é omitido por padrão para não incorrer em custos, e o *pipeline* de *deploy* (build → push ECR → `apply`) está documentado.

---

## 3 CONCLUSÕES

### 3.1 Resultados

O projeto entregou um sistema funcional de ponta a ponta no escopo declarado (núcleo de negociação, integridade contábil e IaC). Foram implementados e verificados:

a) motor de casamento CLOB com prioridade preço-tempo, *fills* parciais, IOC/FOK e *self-trade prevention*, com testes unitários e *replay* determinístico passando;
b) tolerância a falhas via eleição Bully e WAL, com *failover* por parada do líder validado experimentalmente;
c) ledger de dupla entrada em PostgreSQL com invariante de soma zero, **liquidação atômica**, **idempotência por (conta, referência)** resistente a reentregas e **33 ativos**;
d) **OMS pré-negociação com reserva (*hold*) atômica**, eliminando a condição de corrida entre validação e reserva de saldo;
e) **auditoria imutável** de todos os eventos e **Merkle root** determinística por período, com endpoint de prova;
f) autenticação JWT com bcrypt, sessões no PostgreSQL e conta derivada da sessão;
g) *market data* ao vivo de 33 criptomoedas e **semeador de liquidez** que coloca todos os pares para negociar no motor real;
h) interface Flutter Web completa;
i) **infraestrutura como código (Terraform + Terragrunt)** para AWS, modular, com segredos gerenciados e validada.

### 3.2 Limitações e Fronteiras de Escopo

As seguintes funcionalidades estão **fora do recorte deliberado** deste trabalho, e não constituem defeitos do que foi entregue:

- **Ordens avançadas**: *stop-loss*, *stop-limit* e OCO não foram implementadas (o motor suporta `limit` e `market`).
- **Integridade de mercado avançada**: *circuit breakers* e *surveillance* de manipulação (*spoofing*, *layering*, *wash trading*) não foram implementados.
- **Acesso programático**: chaves de API com assinatura HMAC e *rate limiting* na borda não foram implementados (o *schema* `auth.api_keys` está preparado).
- **Escala de leitura**: cache Redis para *order book*/*candles* não foi adotado (as projeções residem no gateway).
- **Isolamento de partição**: o *failover* cobre o cenário de parada do líder (*crash-stop*); o tratamento de partição de rede com *fencing* não foi implementado e é mitigado na nuvem pela tarefa única.
- **Provisionamento na nuvem**: a IaC foi escrita e validada (`terraform validate`), porém o `apply` na AWS não foi executado, para evitar custos.

### 3.3 Dificuldades Técnicas

- **Determinismo do motor**: assegurar que nenhuma fonte de não-determinismo (relógio, aleatoriedade) entrasse no caminho de execução exigiu disciplina e revisão, sob pena de quebrar o *replay* do WAL.
- **Integridade financeira de ponta a ponta**: garantir aritmética inteira em toda a cadeia (Go e Dart) e corrigir um cálculo de notional que truncava o preço exigiu rastrear o valor desde a borda até o lançamento contábil.
- **Idempotência sob reentrega**: modelar a unicidade correta (`(conta, referência)`, não apenas `referência`) para que débito e crédito do mesmo par coexistam, mas reentregas sejam rejeitadas.
- **WAL no Windows**: `os.Truncate` em arquivo aberto com `O_APPEND` retornava "Acesso negado"; a solução foi fechar o *handle* antes de truncar — comportamento distinto do Linux.

### 3.4 Trabalho Futuro

Como evolução natural, destacam-se: implementação de ordens *stop-loss*/OCO e *circuit breakers* no motor; módulo de *surveillance* sobre o log de eventos; chaves de API com HMAC e *rate limiting* para acesso de *bots*; cache Redis para escalar o *market data*; substituição do Bully por um protocolo com *fencing* (lease/Raft) para tolerância a partições; e o provisionamento efetivo na AWS via `terraform apply` com *pipeline* de integração contínua.

---

## REFERÊNCIAS

COINGECKO. **CoinGecko API Documentation**. 2026. Disponível em: https://docs.coingecko.com/. Acesso em: 25 jun. 2026.

FLUTTER TEAM. **Flutter Documentation**. Mountain View: Google, 2026. Disponível em: https://docs.flutter.dev/. Acesso em: 25 jun. 2026.

GARCIA-MOLINA, H. Elections in a Distributed Computing System. **IEEE Transactions on Computers**, v. C-31, n. 1, p. 48-59, jan. 1982. DOI: 10.1109/TC.1982.1675885.

GARCIA-MOLINA, H.; ULLMAN, J. D.; WIDOM, J. **Database Systems: The Complete Book**. 2. ed. Upper Saddle River: Pearson Prentice Hall, 2008. ISBN 978-0-13-187325-4.

GO TEAM. **The Go Programming Language Specification**. Mountain View: Google, 2026. Disponível em: https://go.dev/ref/spec. Acesso em: 25 jun. 2026.

HASHICORP. **Terraform Documentation**. San Francisco: HashiCorp, 2026. Disponível em: https://developer.hashicorp.com/terraform/docs. Acesso em: 25 jun. 2026.

HOHPE, G.; WOOLF, B. **Enterprise Integration Patterns**: Designing, Building, and Deploying Messaging Solutions. Boston: Addison-Wesley, 2003. ISBN 978-0-321-20068-6.

KLEPPMANN, M. **Designing Data-Intensive Applications**: The Big Ideas Behind Reliable, Scalable, and Maintainable Systems. Sebastopol: O'Reilly Media, 2017. ISBN 978-1-4493-7332-0.

POSTGRESQL GLOBAL DEVELOPMENT GROUP. **PostgreSQL 16 Documentation**. 2026. Disponível em: https://www.postgresql.org/docs/16/. Acesso em: 25 jun. 2026.

RABBITMQ TEAM. **RabbitMQ Documentation**. Palo Alto: Broadcom, 2026. Disponível em: https://www.rabbitmq.com/docs. Acesso em: 25 jun. 2026.
