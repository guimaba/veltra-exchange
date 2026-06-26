# Plano de Conclusão — Veltra Exchange

> Escopo de "completo" decidido: **núcleo de negociação + integridade contábil + IaC**.
> Síntese de um conselho de revisão (2026-06-25). Este arquivo é interno (não faz parte do relatório).

## 1. Onde o projeto está

No **código**, o escopo está substancialmente completo e verificado em `go build`, `go vet`, `go test`, `terraform validate`/`fmt` e `docker compose config`:

| Eixo | Item | Estado |
|---|---|---|
| Núcleo | CLOB determinístico, WAL, failover Bully | ✅ (smoke test 06/12) |
| Núcleo | Todos os pares no motor real (sem `simulateTrade`) | ✅ código / ⏳ smoke |
| Integridade | Notional sem truncamento (`money.Notional`) | ✅ |
| Integridade | Idempotência `UNIQUE(conta, ref)` + `ON CONFLICT` | ✅ código / ⏳ smoke |
| Integridade | Settlement atômico (1 transação) | ✅ código / ⏳ smoke |
| Integridade | Hold pré-trade atômico (anti-TOCTOU) | ✅ código / ⏳ smoke |
| Integridade | Merkle determinístico + endpoint | ✅ código / ⏳ smoke |
| Auditoria | Consumer `q.exchange.audit` | ✅ código / ⏳ smoke |
| IaC | Terraform + Terragrunt (dev/demo) | ✅ validate (sem apply) |

## 2. O que FALTA para "completo de verdade" (passos verificáveis)

Em ordem. Só o passo 1 é bloqueante; os demais são de coerência/segurança.

1. **[BLOQUEANTE] Smoke test pós-mudanças.** Nada de hoje rodou em runtime e o schema mudou (índice UNIQUE).
   ```bash
   cd veltra-exchange
   docker compose down -v && docker compose up --build
   ```
   Validar, nesta ordem:
   - 13 contêineres sobem; eleição elege um líder de matching;
   - `marketdata` (logs) financia a conta `liquidity` e publica ordens resting;
   - uma ordem **não-VLT** (ex.: BTC/USDT-sim) pela UI gera `trade.executed` **real** contra a liquidez semeada;
   - `ledger.posted` com notional correto; `SUM(postings.amount)=0` no trade (não só no faucet);
   - reservar além do saldo retorna **HTTP 400** (hold atômico); release do hold em ordem cancelada;
   - `GET /api/veltra/merkle` retorna raiz; `q.exchange.audit` gravou eventos.

2. **[Risco do seeder] Fallback de preço.** O semeador só roda se o fetch da CoinGecko tiver sucesso (`len(lastCoins) > 0`). Se a rede/free-tier falhar, os books ficam vazios e nada casa. Mitigar: garantir rede no smoke **ou** adicionar preços estáticos de fallback no `seedLiquidity`.

3. **[Coerência UI×backend] Narrativa VLT-ao-vivo + catálogo.** A UI ainda desenha um **book sintético** para pares não-VLT e não permite cancelá-los (a projeção do `trading_state.dart` só assina `VLT/USDT-sim`). As ordens **já vão** ao motor real, mas a *visualização* do book não é a real. Caminho de custo zero (adotado no relatório): tratar **VLT/USDT-sim como o par totalmente ao vivo** e os 32 demais como **catálogo negociável com profundidade ilustrativa**. Alternativa (trabalho real, opcional): assinar `book.updated` por par e habilitar cancelamento — só se quiser 33 books ao vivo.

4. **[Pureza inteira] Seeder em float.** `cmd/marketdata` calcula quantidades em `float64` antes de escalar (`scaleOf`). Contraria a tese "zero float no caminho de dinheiro". Opcional: trocar por `money.Parse`/`money.Notional`. Enquanto não fizer, o relatório não reivindica pureza inteira no seeder (só no core).

5. **[Entrega] Commitar tudo.** Código + `infra/` estão como modificado/untracked. Não é "entregue" até commitado.

6. **[Doc] Atualizar `VELTRA_CHECKLIST.md`** com a data do smoke test do passo 1.

## 3. O que NÃO fazer (fora de escopo — consenso do conselho)

Não movem a nota e adicionam risco/custo: Fase 5 (stop-loss/OCO, circuit breakers, surveillance, teste de carga), HMAC/API keys, rate limiting, Redis, fencing/Raft, e `terraform apply` real na AWS. Ficam como **Trabalho Futuro** (§3.4 do relatório).

## 4. Checklist honesto — ANTES de entregar o relatório

O `RELATORIO_FINAL.md` foi escrito em **estado-alvo (como se completo)**, conforme combinado. Estas afirmações do relatório só ficam 100% verdadeiras **depois** de você executar o smoke test do passo 1:

- [ ] "validado experimentalmente" para idempotência, hold atômico, settlement atômico, Merkle e liquidez — **rode o passo 1** (hoje está verificado só em build/test).
- [ ] "todos os pares negociam pelo motor real" — verdadeiro no backend; confirme um trade não-VLF no smoke e mantenha a narrativa VLT-ao-vivo/catálogo da §2.10.
- [x] "IaC validada (`terraform validate`)" — **verdadeiro**; o relatório já diz explicitamente que o `apply` foi omitido (não afirme "implantado").
- [x] "failover por parada do líder (<15s)" — **verdadeiro** (smoke 06/12); o relatório já restringe a *crash-stop* e manda split-brain para trabalho futuro.
- [ ] Número de contêineres no relatório = **13** (9 Veltra + 4 legados). Confirme se o seu compose final ainda tem os 4 legados; se remover, ajuste o número.

> Em resumo: o relatório está pronto e honesto **no fraseado**; o único passo que converte "estado-alvo" em "fato" é o **smoke test do passo 1**. Recomendo fortemente rodá-lo antes de submeter.
