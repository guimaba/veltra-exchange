# Veltra Exchange — Checklist de Execução

> Plano técnico de referência: `Veltra_Exchange_Plano_Tecnico.pdf` (versão 1.0, Maio 2026)
> Base de engenharia: projeto blockchain — Sistemas Distribuídos (Go + RabbitMQ)
>
> **Decisões travadas:**
> - Ordem de construção: **código + Docker primeiro, Terraform/Terragrunt por último**
> - Ledger: **PostgreSQL** (migração na Fase 2; MariaDB continua para a blockchain legada)
> - Frontend: **Flutter Web** (reaproveita `web/`)
> - Par inicial: **VLT/USDT-sim**
>
> **Acordo de arquitetura — usuários, autenticação e banco (2026-06-12):**
> - O sistema TERÁ organização de usuários, autenticação e gerenciamento completo no banco, aplicados **em partes** ao longo das fases
> - **PostgreSQL único** para o sistema: schemas `auth` (users, accounts, api_keys, sessions) e `ledger` (razão de dupla entrada). Criados juntos na Fase 2, mesmo que o auth só seja consumido na Fase 3
> - Modelo: `users` (senha com hash forte) → `accounts` 1:1 inicialmente (FK preparada para 1:N/subcontas) → `api_keys` para HMAC (plano §5.1)
> - **A conta passa a vir da sessão autenticada, nunca do payload** — o seletor de conta livre da UI atual e o `account` no body de `/api/orders` são TEMPORÁRIOS (demo) e serão removidos quando o login entrar
> - Faucet vira operação admin/autenticada
> - Sugestões de frontend (telas/fluxos de login, cadastro, conta) virão do time — implementar em cima delas, não inventar UX de auth

---

## ✅ Fase 0 — Fundações (CONCLUÍDA)

- [x] **T0.1 — `pkg/money`**: aritmética fixed-point sobre `int64` (8 casas decimais, `Scale = 1e8`)
  - `Parse`/`String` sem perda de precisão, `Add`/`Sub` com checagem de overflow, `Notional` (preço × qtd) via `math/big`
  - Zero `float64` em qualquer caminho de saldo/preço (plano §4.2.2); `Float64()` existe só para exibição
  - Testes: `0.1+0.2==0.3`, overflow, truncamento de notional — **passando**
- [x] **T0.2 — `pkg/exchange`**: tipos de domínio
  - `Asset` (VLT, USDT-sim), `Pair` (BASE/QUOTE), `Side`, `OrderType` (limit/market + stop reservados), `TimeInForce` (GTC/IOC/FOK), `OrderStatus`, `Order`, `Trade`
  - Prioridade FIFO ancorada em `Order.Sequence` (sequenciador), não em relógio → determinismo
- [x] **T0.3 — Topologia de mensageria Veltra** (`pkg/messaging`)
  - Exchanges `veltra.commands` / `veltra.events` (topic), separadas do domínio blockchain
  - Filas: `q.matching.commands`, `q.ledger.events`, `q.marketdata.events`, `q.exchange.audit`
  - Routing keys: `order.place|cancel`, `faucet.credit`, `order.accepted|rejected|canceled|filled`, `trade.executed`, `book.updated`, `ledger.posted`
  - Payloads tipados em `veltra_payloads.go` (valores como `int64` na menor unidade)
  - Reuso integral do publisher (confirms), consumer (ACK/retry/DLQ) e envelope existentes

## ✅ Fase 1 — Matching Engine (CONCLUÍDA)

- [x] **T1.1 — `pkg/orderbook`**: CLOB com prioridade preço-tempo
  - Bids decrescentes / asks crescentes, FIFO dentro do nível, cancelamento por OrderID, best bid/ask, spread, L2 agregado com depth
  - `AllOrders()` em ordem determinística de prioridade (para snapshots)
  - Testes: prioridade de preço, FIFO, cancel, agregação L2 — **passando**
- [x] **T1.2 — `pkg/matching`**: motor de casamento
  - Single-threaded por par (sem locks no caminho de matching), determinístico: **sem `time.Now`/random no motor** — timestamp herdado da ordem, TradeID derivado da sequência
  - Limit + market, fills parciais, IOC (cancela resto), FOK (tudo ou nada, sem tocar o book se insuficiente)
  - **Self-trade prevention**: cancela o maker da própria conta e continua (plano §4.2.2/§5.2)
  - Execução ao preço do maker (price improvement ao taker)
  - Testes: fill total/parcial, price-time, market sweep, IOC, FOK, self-trade, **replay determinístico** (mesma entrada ⇒ mesmos trades) — **passando**
- [x] **T1.3 — WAL + snapshots** (`pkg/matching/wal.go`)
  - `Journal`: `Append` com fsync **antes** de aplicar ao motor; `Snapshot` atômico (tmp+rename) que trunca o WAL; `OpenJournal` recupera snapshot + replay do WAL
  - Tolerância a linha corrompida no fim do WAL (crash no meio do append)
  - Testes: recovery pós-"crash", snapshot+truncamento+recovery — **passando**
- [x] **T1.4 — `cmd/matching-engine`**: serviço integrado
  - Eleição Bully própria (`election.go`, RPC), heartbeat do líder, promoção/rebaixamento
  - Só o líder consome `q.matching.commands` (prefetch=1 ⇒ serializado); standby fica pronto
  - Sequencia → grava WAL → processa → emite eventos (`order.accepted/rejected/canceled/filled`, `trade.executed`)
  - Snapshot a cada N comandos (`SNAPSHOT_EVERY`)
  - Config via env: `NODE_ID`, `NODE_PORT`, `PEERS`, `AMQP_URL`, `PAIRS`, `WAL_DIR`, `SNAPSHOT_EVERY`
- [x] **Docker**
  - `docker/Dockerfile.matching` (multi-stage, alpine, non-root)
  - 3 réplicas no `docker-compose.yml` (`matching1..3`) com **volume de WAL compartilhado** (`matching-wal`) para failover com recuperação de estado
  - `definitions.json`: usuários/permissões (`matching`, `ledger`, `marketdata`), exchanges, filas e bindings da Veltra — JSON validado, compose validado

### ✅ Validação da Fase 1 — smoke test executado em 2026-06-12
- [x] **Stack completo no Docker**: `docker compose up --build` ok (incl. compilação do Flutter Web); 10 containers saudáveis
- [x] **Eleição**: matching3 (maior ID) virou líder e consumiu `q.matching.commands`; 1 e 2 em standby
- [x] **Trading ponta a ponta via API REST**: faucet (alice 100k USDT-sim, bob 1k VLT) → bob sell 2 VLT@100 → alice buy 1 VLT@100 → `trade.executed` t1; book com resto do bob (1 VLT@100, partially_filled); saldos exatos (alice 99.900 USDT + 1 VLT; bob 100 USDT + 999 VLT)
- [x] **Failover com recuperação via WAL**: `docker stop veltra-matching3` → matching2 venceu eleição e recuperou estado do WAL compartilhado (`lastSeq=2, ordens=1`) → market buy da alice casou contra a ordem RECUPERADA (trade t2 @ 100, contador de trades contínuo) → book zerado, saldos finais exatos (alice 99.800 + 2 VLT; bob 200 + 998 VLT)
- [x] **UI Flutter servida** em `http://localhost:8080` (HTTP 200)

---

## ✅ Fase 2 — Ledger de dupla entrada (PostgreSQL) — CONCLUÍDA 2026-06-18

- [x] **T2.1 — Infra PostgreSQL**: `veltra-postgres` no docker-compose; migrations em `docker/postgres/` criam schemas `auth` e `ledger`; `pkg/pgstore` com pool de conexões
- [x] **T2.2 — `pkg/ledger`**: motor de dupla entrada (`ledger.go`): `PostEntry` (débito/crédito atômico), `HoldAmount`/`ReleaseHold`, `GetOrCreateAccount`, `GetOrCreateTradingAccount` (auto-provisionamento pré-auth), `ValidateSufficient`
- [x] **T2.3 — Settlement pós-trade** (`cmd/ledger`): consome `q.ledger.events`; `trade.executed` → `SettleTrade` (base e quote, atomicamente); emite `ledger.posted`; `Dockerfile.ledger` + serviço no compose
- [x] **T2.4 — Faucet contábil**: consome `faucet.credit` → `SettleFaucet` (crédita contra conta sink de emissão); contas sink inicializadas no seed do schema com saldo `-MAXINT64`
- [x] **T2.5 — Merkle root por período**: `pkg/ledger/merkle.go` — `ComputeMerkleRoot` (SHA256 Bitcoin-style), `BuildAndSaveMerkleRoot` (busca postings do período, computa raiz, persiste em `ledger.merkle_roots`)

### ✅ Validação Fase 2 — smoke test executado em 2026-06-18
- [x] **Postgres inicializado**: 9 tabelas em schemas `auth` e `ledger`; contas sink seed ok (VLT/USDT-sim com `trading_account_id=0`)
- [x] **Faucet settlado**: alice +500 USDT-sim, bob +100 VLT — registrados no `ledger.postings` e balanços atualizados
- [x] **Trade settlado**: `trade.executed` → buyer base +qty, seller quote +notional (e créditos correspondentes)
- [x] **Invariante de dupla entrada**: `SUM(postings.amount) = 0` por operação (`trade` e `faucet` ambos = 0 ✅)
- [x] **`go build ./...` + `go vet ./...`**: ok

## 🟨 Fase 3 — OMS / Risco / Borda (PARCIAL — fatia vertical do frontend)

- [ ] **T3.1 — OMS pré-trade**
  - Validação de saldo + **hold** (reserva) antes de encaminhar ao motor; liberação de hold em cancel/reject; risco por conta/par (limites)
  - **Idempotência por `clientOrderId`** (mesmo mecanismo do TxID atual)
- [x] **T3.2 — Gateway de trading (REST)** *(entregue na fatia vertical do frontend)*
  - `POST /api/orders`, `DELETE /api/orders/{id}`, `POST /api/faucet`, `GET /api/veltra/state` em `cmd/gateway/veltra_server.go`
  - Preço/quantidade entram como string decimal e viram inteiro escalado via `money.Parse` (cliente nunca faz conta de dinheiro em float)
  - Publica `order.place`/`order.cancel` em `veltra.commands`; faucet publica `faucet.credit` em `veltra.events` (quando o Ledger da Fase 2 existir, ele consome esse mesmo evento)
- [x] **T3.3 — Autenticação e usuários** *(2026-06-18 — frontend + backend concluídos)*
  - `cmd/gateway/auth_server.go`: `POST /api/auth/register` (bcrypt, cria user+account em TX), `POST /api/auth/login` (verifica hash, emite JWT HS256 TTL 24h), `POST /api/auth/logout` (invalida sessão no DB), `GET /api/auth/me` (verifica token)
  - Middleware `RequireAuth`: JWT → claims no contexto; `veltraRoutes` usa middleware em orders, cancel e faucet
  - **Conta vem do JWT** em todos os endpoints de trading: `account` do payload ignorado quando autenticado
  - Deps adicionadas: `golang.org/x/crypto/bcrypt`, `github.com/golang-jwt/jwt/v5`; Dockerfiles atualizados para Go 1.25
  - Flutter: `AuthState`, `AuthGate`, `LoginScreen`, `RegisterScreen`, `_UserMenu` com logout; token enviado em `Authorization: Bearer` em todas as chamadas de trading
  - **Smoke test validado 2026-06-18**: register ✅, login ✅, /me ✅, faucet autenticado ✅, ordem autenticada (conta do JWT) ✅, requisição sem token → 401 ✅, trade executado ✅
  - Pendente (Fase 5): HMAC para API keys de bots; rate limiting; 2FA

## 🟨 Fase 4 — Market Data + Frontend (PARCIAL — núcleo entregue)

- [x] **T4.1a — Projeções de market data** *(vivem no gateway por ora — `cmd/gateway/veltra_state.go`)*
  - Matching engine emite `book.updated` (snapshot L2 top-20 por comando)
  - Gateway consome `q.marketdata.events`: book L2, fita de trades (100), ordens, saldos-projeção, último preço
  - Saldos são PROJEÇÃO de exibição (faucet + trades); fonte de verdade contábil chega com o Ledger (Fase 2)
- [ ] **T4.1b — `cmd/marketdata` dedicado**: extrair as projeções do gateway; candles/OHLCV por janela; cache Redis (plano §6)
- [x] **T4.2a — WebSocket fan-out (snapshot+eventos)**: hub existente reusado; `veltra_snapshot` no connect + eventos em tempo real
- [ ] **T4.2b — Deltas incrementais de book** (hoje é snapshot L2 completo por update; otimizar para deltas)
- [x] **T4.3 — Trading UI (Flutter Web)** *(`web/lib/screens/trade.dart` + `trading_state.dart`)*
  - Book ladder com barras de profundidade e clique-para-preencher-preço; fita de trades em tempo real
  - Form de ordem: buy/sell, limit/market, GTC/IOC/FOK, total estimado; ordens abertas (com cancelar) + histórico
  - Saldos por conta + dialog de faucet; seletor de conta (demo multi-conta em abas do navegador)
  - Header de mercado (último preço com direção, bid/ask/spread); tema dark de terminal de trading
  - ⚠️ Compilação Dart acontece no Docker (flutter não instalado na máquina local) — validar com `docker compose build gateway`

## 🔲 Fase 5 — Avançado / Integridade de mercado

- [ ] **T5.1 — Ordens avançadas**: stop-loss, stop-limit, OCO (plano §4.2.3)
- [ ] **T5.2 — Circuit breakers**: interromper o par em movimento extremo de preço; limites de risco por conta/par
- [ ] **T5.3 — Surveillance educacional**: detecção de spoofing/layering/wash trading sobre o log de eventos
- [ ] **T5.4 — Validação sob carga + determinismo**: teste de carga (latência alvo ms), replay completo do log comparando estados

## 🔲 Fase 6 — Infraestrutura (POR ÚLTIMO, decisão do time)

- [ ] **T6.1 — Terraform**: módulos AWS — Amazon MQ (RabbitMQ), ECS (um serviço por componente), RDS PostgreSQL, ElastiCache Redis (se usado), CloudFront/WAF na borda (plano §4.1/§6)
- [ ] **T6.2 — Terragrunt**: composição por ambiente (dev/demo), estado remoto, DRY entre ambientes
- [ ] **T6.3 — Pipeline de deploy**: build das imagens → push ECR → deploy ECS

---

## Estado dos testes

| Pacote | Status |
|---|---|
| `pkg/money` | ✅ ok |
| `pkg/orderbook` | ✅ ok |
| `pkg/matching` (engine + WAL) | ✅ ok |
| `go build ./...` | ✅ ok |
| `go vet` | ✅ ok |
| `docker compose config` | ✅ válido |
| `definitions.json` | ✅ JSON válido |
| Smoke test no Docker | ✅ ok (Fase 1: 2026-06-12, Fase 2: 2026-06-18) |

## Mapa de reuso (plano §3.1/§3.2)

| Projeto blockchain | → Veltra Exchange | Status |
|---|---|---|
| `pkg/messaging` (RabbitMQ, confirms, retry/DLQ) | Event sourcing + CQRS | ✅ reusado |
| Idempotência por TxID | `clientOrderId` idempotente | 🔲 Fase 3 (OMS) |
| `pkg/bully` (eleição) | Failover do matching engine | ✅ reusado + WAL |
| `Amount float64` | Inteiro escalado (`pkg/money`) | ✅ feito |
| Ledger de blocos imutáveis | Ledger dupla entrada + Merkle | 🔲 Fase 2 |
| Gateway REST/WS + hub | Borda + market data fan-out | 🔲 Fases 3-4 |
| `cmd/audit` | Trilha de auditoria (`q.exchange.audit`) | ✅ fila pronta; consumer 🔲 |
