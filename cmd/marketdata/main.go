// Serviço Market Data da Veltra Exchange.
//
// Consulta preços reais de criptomoedas na CoinGecko (free API, sem auth) a
// cada 30 segundos, mantém histórico de candles OHLCV (5 min, até 200 por
// moeda) em memória, e publica eventos market.update no exchange veltra.events.
//
// O preço do token VLT é derivado do ATOM (Cosmos): VLT_USD = ATOM_USD * 0.47.
// ATOM não é publicado na lista de moedas — é apenas uma referência interna.
//
// Variáveis de ambiente:
//
//	AMQP_URL   (obrigatório) — URL do RabbitMQ
//	FETCH_INTERVAL (opcional) — intervalo de fetch em segundos (padrão: 30)
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/messaging"
)

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// MarketCoin representa o snapshot de preço de uma moeda.
type MarketCoin struct {
	Symbol       string  `json:"symbol"`
	Name         string  `json:"name"`
	PriceUSD     float64 `json:"price_usd"`
	PriceBRL     float64 `json:"price_brl"`
	PriceEUR     float64 `json:"price_eur"`
	Change24h    float64 `json:"change_24h"`
	Volume24hUSD float64 `json:"volume_24h_usd"`
	MarketCapUSD float64 `json:"market_cap_usd"`
}

// quoteCurrencies sao as moedas fiat de cotacao: cada cripto e semeada com
// liquidez em pares contra cada uma delas (BASE/USD, BASE/BRL, BASE/EUR).
var quoteCurrencies = []string{"USD", "BRL", "EUR"}

// Candle representa uma vela OHLCV de 5 minutos.
type Candle struct {
	T int64   `json:"t"` // unix seconds (abertura da vela)
	O float64 `json:"o"`
	H float64 `json:"h"`
	L float64 `json:"l"`
	C float64 `json:"c"`
	V float64 `json:"v"` // volume em USD
}

// MarketUpdatePayload é o payload publicado no evento market.update.
type MarketUpdatePayload struct {
	Coins     []MarketCoin        `json:"coins"`
	Candles   map[string][]Candle `json:"candles"` // symbol -> últimas 50 velas
	UpdatedAt int64               `json:"updated_at"` // unix ms
}

// ---------------------------------------------------------------------------
// Coin definitions
// ---------------------------------------------------------------------------

type coinDef struct {
	geckoID string
	symbol  string
	name    string
}

// coinDefs lista as 32 moedas publicadas pelo serviço.
var coinDefs = []coinDef{
	{"bitcoin", "BTC", "Bitcoin"},
	{"ethereum", "ETH", "Ethereum"},
	{"binancecoin", "BNB", "BNB"},
	{"solana", "SOL", "Solana"},
	{"ripple", "XRP", "XRP"},
	{"cardano", "ADA", "Cardano"},
	{"dogecoin", "DOGE", "Dogecoin"},
	{"polkadot", "DOT", "Polkadot"},
	{"avalanche-2", "AVAX", "Avalanche"},
	{"polygon-ecosystem-token", "POL", "Polygon"},
	{"chainlink", "LINK", "Chainlink"},
	{"uniswap", "UNI", "Uniswap"},
	{"litecoin", "LTC", "Litecoin"},
	{"filecoin", "FIL", "Filecoin"},
	{"algorand", "ALGO", "Algorand"},
	{"stellar", "XLM", "Stellar"},
	{"near", "NEAR", "NEAR Protocol"},
	{"internet-computer", "ICP", "Internet Computer"},
	{"aptos", "APT", "Aptos"},
	{"arbitrum", "ARB", "Arbitrum"},
	{"optimism", "OP", "Optimism"},
	{"injective-protocol", "INJ", "Injective"},
	{"sei-network", "SEI", "Sei"},
	{"sui", "SUI", "Sui"},
	{"celestia", "TIA", "Celestia"},
	{"pepe", "PEPE", "Pepe"},
	{"shiba-inu", "SHIB", "Shiba Inu"},
	{"dogwifcoin", "WIF", "dogwifhat"},
	{"jupiter-exchange-solana", "JUP", "Jupiter"},
	{"bonk", "BONK", "Bonk"},
	{"the-open-network", "TON", "Toncoin"},
	{"tron", "TRX", "TRON"},
}

// cosmosGeckoID é a referência oculta para derivar o preço VLT.
const cosmosGeckoID = "cosmos"

// vltMultiplier é o multiplicador para derivar o preço VLT a partir do ATOM.
const vltMultiplier = 0.47

// candleInterval é o tamanho de cada vela em segundos (5 minutos).
const candleInterval int64 = 5 * 60

// maxCandles é o número máximo de velas mantidas por moeda em memória.
const maxCandles = 200

// publishCandles é quantas velas são incluídas no payload publicado.
const publishCandles = 50

// ---------------------------------------------------------------------------
// CoinGecko response
// ---------------------------------------------------------------------------

// geckoPrice representa os campos que extraímos da resposta da CoinGecko.
type geckoPrice struct {
	USD        float64 `json:"usd"`
	BRL        float64 `json:"brl"`
	EUR        float64 `json:"eur"`
	USD24hVol  float64 `json:"usd_24h_vol"`
	USD24hChg  float64 `json:"usd_24h_change"`
	USDMarket  float64 `json:"usd_market_cap"`
}

// fetchPrices consulta a API gratuita da CoinGecko e retorna um mapa
// geckoID -> geckoPrice. ctx controla o timeout da requisição HTTP.
func fetchPrices(ctx context.Context, ids []string) (map[string]geckoPrice, error) {
	joined := strings.Join(ids, ",")
	url := fmt.Sprintf(
		"https://api.coingecko.com/api/v3/simple/price?ids=%s&vs_currencies=usd,brl,eur&include_24hr_change=true&include_24hr_vol=true&include_market_cap=true&precision=full",
		joined,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar requisição: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "veltra-marketdata/1.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("erro na requisição HTTP: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("CoinGecko retornou status %d", resp.StatusCode)
	}

	// A resposta é um objeto { "bitcoin": { "usd": ..., "brl": ... }, ... }
	var raw map[string]geckoPrice
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("erro ao decodificar resposta: %w", err)
	}
	return raw, nil
}

// ---------------------------------------------------------------------------
// Candle store
// ---------------------------------------------------------------------------

// candleStore mantém as velas OHLCV de todas as moedas em memória.
type candleStore struct {
	mu      sync.RWMutex
	candles map[string][]Candle // symbol -> candles ordenadas por tempo (mais antiga primeiro)
	rng     map[string]*rand.Rand
}

func newCandleStore() *candleStore {
	return &candleStore{
		candles: make(map[string][]Candle),
		rng:     make(map[string]*rand.Rand),
	}
}

// rngFor retorna (ou cria) um gerador de números aleatórios dedicado para
// uma moeda. A seed é derivada do símbolo para garantir reproducibilidade
// na geração do histórico sintético.
func (s *candleStore) rngFor(symbol string) *rand.Rand {
	if r, ok := s.rng[symbol]; ok {
		return r
	}
	h := fnv.New64a()
	h.Write([]byte(symbol))
	r := rand.New(rand.NewSource(int64(h.Sum64()))) //nolint:gosec // reproducible seed
	s.rng[symbol] = r
	return r
}

// initHistory gera 200 velas sintéticas históricas para uma moeda a partir
// do preço atual, caminhando para trás no tempo.
func (s *candleStore) initHistory(symbol string, currentPrice float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.candles[symbol]; exists {
		return // já inicializado
	}

	r := s.rngFor(symbol)
	now := time.Now().Unix()
	// Alinha ao início do candle de 5 min atual.
	openTime := (now / candleInterval) * candleInterval

	candles := make([]Candle, maxCandles)
	price := currentPrice

	// Gera do mais recente para o mais antigo, depois inverte.
	for i := 0; i < maxCandles; i++ {
		t := openTime - int64(i)*candleInterval

		// Variação entre -1.5% e +1.5% para cada vela (random walk).
		factor := 1.0 + (r.Float64()*0.03 - 0.015)
		open := price / factor // preço de abertura desta vela
		close := price         // preço de fechamento

		high := math.Max(open, close) * (1.0 + r.Float64()*0.003)
		low := math.Min(open, close) * (1.0 - r.Float64()*0.003)

		// Volume em USD: entre 0.5x e 2x o preço (simulado para ter grandeza razoável).
		vol := price * (0.5 + r.Float64()*1.5) * 1_000_000

		candles[i] = Candle{T: t, O: open, H: high, L: low, C: close, V: vol}
		price = open
	}

	// Inverte: índice 0 = mais antigo, último = mais recente.
	for i, j := 0, len(candles)-1; i < j; i, j = i+1, j-1 {
		candles[i], candles[j] = candles[j], candles[i]
	}

	s.candles[symbol] = candles
}

// appendCandle fecha a vela corrente e abre uma nova com o preço atual.
// Se ainda não houver histórico para o símbolo, inicializa antes.
func (s *candleStore) appendCandle(symbol string, currentPrice float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	r := s.rngFor(symbol)
	now := time.Now().Unix()
	openTime := (now / candleInterval) * candleInterval

	existing := s.candles[symbol]

	// Determina o open: fecha a vela anterior com o preço atual e usa esse
	// mesmo preço como open da nova vela.
	var openPrice float64
	if len(existing) > 0 {
		last := existing[len(existing)-1]
		if last.T == openTime {
			// Ainda na mesma janela — atualiza a vela corrente.
			updated := last
			updated.C = currentPrice
			updated.H = math.Max(last.H, currentPrice)
			updated.L = math.Min(last.L, currentPrice)
			existing[len(existing)-1] = updated
			s.candles[symbol] = existing
			return
		}
		openPrice = last.C
	} else {
		openPrice = currentPrice
	}

	close := currentPrice
	high := math.Max(openPrice, close) * (1.0 + r.Float64()*0.003)
	low := math.Min(openPrice, close) * (1.0 - r.Float64()*0.003)
	vol := currentPrice * (0.5 + r.Float64()*1.5) * 1_000_000

	newCandle := Candle{T: openTime, O: openPrice, H: high, L: low, C: close, V: vol}
	existing = append(existing, newCandle)

	// Mantém no máximo maxCandles velas.
	if len(existing) > maxCandles {
		existing = existing[len(existing)-maxCandles:]
	}
	s.candles[symbol] = existing
}

// last retorna as últimas n velas do símbolo (ou todas, se houver menos que n).
func (s *candleStore) last(symbol string, n int) []Candle {
	s.mu.RLock()
	defer s.mu.RUnlock()

	all := s.candles[symbol]
	if len(all) <= n {
		out := make([]Candle, len(all))
		copy(out, all)
		return out
	}
	out := make([]Candle, n)
	copy(out, all[len(all)-n:])
	return out
}

// ---------------------------------------------------------------------------
// Market data service
// ---------------------------------------------------------------------------

type service struct {
	publisher *messaging.Publisher
	store     *candleStore
	interval  time.Duration
	httpClient *http.Client

	// IDs CoinGecko a buscar (inclui cosmos para VLT, mas não inclui na lista publicada).
	geckoIDs []string
	// Mapa geckoID -> coinDef para montar o payload.
	defByGeckoID map[string]coinDef

	// seedLiq habilita a semeadura de liquidez (ordens resting por par).
	seedLiq bool
	// lastCoins guarda o último snapshot de moedas (para a semeadura inicial).
	lastCoins []MarketCoin
}

func newService(publisher *messaging.Publisher, interval time.Duration) *service {
	ids := make([]string, 0, len(coinDefs)+1)
	defMap := make(map[string]coinDef, len(coinDefs))

	for _, cd := range coinDefs {
		ids = append(ids, cd.geckoID)
		defMap[cd.geckoID] = cd
	}
	ids = append(ids, cosmosGeckoID) // referência VLT — não publicada

	return &service{
		publisher:    publisher,
		store:        newCandleStore(),
		interval:     interval,
		httpClient:   &http.Client{Timeout: 15 * time.Second},
		geckoIDs:     ids,
		defByGeckoID: defMap,
		seedLiq:      os.Getenv("SEED_LIQUIDITY") == "true",
	}
}

// run é o loop principal do serviço. Bloqueia até ctx ser cancelado.
func (svc *service) run(ctx context.Context) {
	log.Printf("[MarketData] Iniciando. Intervalo de fetch: %s", svc.interval)

	// Inicialização: busca preços e gera histórico.
	if err := svc.tick(ctx, true); err != nil {
		log.Printf("[MarketData] Aviso: falha no tick inicial: %v", err)
	}

	// Semeadura única de liquidez (após termos preços de referência).
	if svc.seedLiq && len(svc.lastCoins) > 0 {
		svc.seedLiquidity(ctx, svc.lastCoins)
	}

	ticker := time.NewTicker(svc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("[MarketData] Contexto cancelado, encerrando loop.")
			return
		case <-ticker.C:
			if err := svc.tick(ctx, false); err != nil {
				log.Printf("[MarketData] Aviso: falha no tick: %v", err)
			}
		}
	}
}

// tick busca preços, atualiza velas e publica o evento.
// isFirst indica se é o tick inicial (gera histórico sintético).
func (svc *service) tick(ctx context.Context, isFirst bool) error {
	fetchCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	prices, err := fetchPrices(fetchCtx, svc.geckoIDs)
	if err != nil {
		return fmt.Errorf("CoinGecko indisponível: %w", err)
	}

	// Monta moedas publicadas.
	coins := make([]MarketCoin, 0, len(coinDefs)+1)

	for _, cd := range coinDefs {
		p, ok := prices[cd.geckoID]
		if !ok {
			log.Printf("[MarketData] Aviso: preço não encontrado para %s (%s)", cd.symbol, cd.geckoID)
			continue
		}

		if isFirst {
			svc.store.initHistory(cd.symbol, p.USD)
		} else {
			svc.store.appendCandle(cd.symbol, p.USD)
		}

		coins = append(coins, MarketCoin{
			Symbol:       cd.symbol,
			Name:         cd.name,
			PriceUSD:     p.USD,
			PriceBRL:     p.BRL,
			PriceEUR:     p.EUR,
			Change24h:    p.USD24hChg,
			Volume24hUSD: p.USD24hVol,
			MarketCapUSD: p.USDMarket,
		})
	}

	// Deriva preço VLT a partir do ATOM.
	atomPrice, hasAtom := prices[cosmosGeckoID]
	if hasAtom {
		vltUSD := atomPrice.USD * vltMultiplier
		vltBRL := atomPrice.BRL * vltMultiplier
		vltEUR := atomPrice.EUR * vltMultiplier

		if isFirst {
			svc.store.initHistory("VLT", vltUSD)
		} else {
			svc.store.appendCandle("VLT", vltUSD)
		}

		coins = append(coins, MarketCoin{
			Symbol:       "VLT",
			Name:         "Veltra Token",
			PriceUSD:     vltUSD,
			PriceBRL:     vltBRL,
			PriceEUR:     vltEUR,
			Change24h:    atomPrice.USD24hChg,
			Volume24hUSD: 0, // volume VLT não é real
			MarketCapUSD: 0,
		})
	} else {
		log.Printf("[MarketData] Aviso: preço de cosmos (VLT ref) não encontrado")
	}

	// Monta mapa de candles para o payload (últimas publishCandles por moeda).
	candleMap := make(map[string][]Candle, len(coins))
	for _, coin := range coins {
		candleMap[coin.Symbol] = svc.store.last(coin.Symbol, publishCandles)
	}

	svc.lastCoins = coins

	payload := MarketUpdatePayload{
		Coins:     coins,
		Candles:   candleMap,
		UpdatedAt: time.Now().UnixMilli(),
	}

	env, err := messaging.NewEnvelope("veltra.market.update.v1", "", payload)
	if err != nil {
		return fmt.Errorf("erro ao criar envelope: %w", err)
	}

	pubCtx, pubCancel := context.WithTimeout(ctx, 10*time.Second)
	defer pubCancel()

	if err := svc.publisher.Publish(pubCtx, messaging.ExchangeVeltraEvents, "market.update", env, nil); err != nil {
		return fmt.Errorf("erro ao publicar market.update: %w", err)
	}

	log.Printf("[MarketData] Publicado market.update: %d moedas, updated_at=%d", len(coins), payload.UpdatedAt)
	return nil
}

// ---------------------------------------------------------------------------
// Liquidity seeder (market maker da demo)
// ---------------------------------------------------------------------------
//
// Promove TODOS os pares ao matching engine real (plano §4.2.2 "um motor por
// par"): em vez de fills forjados no gateway, uma conta `liquidity` é financiada
// por faucet e coloca ordens limite resting nos dois lados de cada par. Quando
// um usuário negocia, ele casa contra essas ordens pelo CLOB determinístico —
// trade.executed real, com WAL e settlement de dupla entrada.
//
// Semeadura ÚNICA no startup (bounded): mantém o book com profundidade inicial
// sem churn de ordens. As ordens vão para q.matching.commands (durável), então
// são processadas assim que o líder do matching estiver ativo.

const liquidityAccount = "liquidity"

// scaleOf converte um valor decimal (USD) para int64 escalado (money.Scale=1e8).
func scaleOf(v float64) int64 { return int64(v * 1e8) }

// priceIn retorna o preço da moeda na moeda de cotação informada (USD/BRL/EUR).
func (c MarketCoin) priceIn(quote string) float64 {
	switch quote {
	case "BRL":
		return c.PriceBRL
	case "EUR":
		return c.PriceEUR
	default:
		return c.PriceUSD
	}
}

// seedLiquidity financia a conta de liquidez e coloca quotes nos dois lados de
// cada par BASE/{USD,BRL,EUR} a partir dos preços de referência por moeda.
func (svc *service) seedLiquidity(ctx context.Context, coins []MarketCoin) {
	const (
		levels         = 3        // níveis de profundidade por lado
		notionalPerLvl = 25_000.0 // ~unidades da quote por nível (define a quantidade)
		fundQuote      = 50_000_000.0
	)

	// 1. Financia a conta de liquidez com cada moeda de cotação (USD/BRL/EUR)...
	for _, q := range quoteCurrencies {
		svc.faucet(ctx, liquidityAccount, q, scaleOf(fundQuote))
	}
	// ...e com cada ativo base (inventário p/ os asks, dimensionado em USD).
	for _, c := range coins {
		if c.PriceUSD <= 0 {
			continue
		}
		baseQty := (notionalPerLvl * float64(levels) * 4 * float64(len(quoteCurrencies))) / c.PriceUSD
		svc.faucet(ctx, liquidityAccount, c.Symbol, scaleOf(baseQty))
	}

	// 2. Coloca quotes resting em cada par BASE/QUOTE, com preço na moeda da quote.
	pairs := 0
	for _, c := range coins {
		for _, q := range quoteCurrencies {
			px := c.priceIn(q)
			if px <= 0 {
				continue
			}
			pair := c.Symbol + "/" + q
			qtyPerLvl := notionalPerLvl / px
			for i := 1; i <= levels; i++ {
				off := 1.0 + float64(i)*0.002 // ±0.2%, 0.4%, 0.6%
				svc.placeLimit(ctx, pair, "buy", scaleOf(px/off), scaleOf(qtyPerLvl))
				svc.placeLimit(ctx, pair, "sell", scaleOf(px*off), scaleOf(qtyPerLvl))
			}
			pairs++
		}
	}
	log.Printf("[MarketData] Liquidez semeada: %d pares, %d níveis/lado", pairs, levels)
}

// faucet publica faucet.credit (emissão de saldo virtual) para a conta.
func (svc *service) faucet(ctx context.Context, account, asset string, amount int64) {
	if amount <= 0 {
		return
	}
	env, err := messaging.NewEnvelope(messaging.SchemaFaucetCredit, "", messaging.FaucetCreditPayload{
		Account: account, Asset: asset, Amount: amount,
	})
	if err != nil {
		return
	}
	_ = svc.publisher.Publish(ctx, messaging.ExchangeVeltraEvents, messaging.RKFaucetCredit, env, nil)
}

// placeLimit publica order.place (GTC) no exchange de comandos da Veltra.
func (svc *service) placeLimit(ctx context.Context, pair, side string, price, qty int64) {
	env, err := messaging.NewEnvelope(messaging.SchemaOrderPlace, "", messaging.OrderPlacePayload{
		Account:     liquidityAccount,
		Pair:        pair,
		Side:        side,
		Type:        "limit",
		TimeInForce: "gtc",
		Price:       price,
		Quantity:    qty,
	})
	if err != nil {
		return
	}
	_ = svc.publisher.Publish(ctx, messaging.ExchangeVeltraCommands, messaging.RKOrderPlace, env, nil)
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	amqpURL := os.Getenv("AMQP_URL")
	if amqpURL == "" {
		log.Fatal("[MarketData] AMQP_URL é obrigatório")
	}

	interval := 30 * time.Second
	if v := os.Getenv("FETCH_INTERVAL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			interval = time.Duration(n) * time.Second
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Conecta ao RabbitMQ com retry.
	log.Printf("[MarketData] Conectando ao RabbitMQ...")
	bootCtx, bootCancel := context.WithTimeout(ctx, 60*time.Second)
	client, err := messaging.NewClient(bootCtx, amqpURL)
	bootCancel()
	if err != nil {
		log.Fatalf("[MarketData] Falha ao conectar ao RabbitMQ: %v", err)
	}
	defer client.Close()

	// Declara a topologia (Amazon MQ não importa definitions.json). Idempotente.
	if err := client.DeclareTopology(); err != nil {
		log.Printf("[MarketData] Aviso: topologia nao declarada (%v) - ok se ja existir", err)
	}

	publisher := messaging.NewPublisher(client)
	defer publisher.Close()

	svc := newService(publisher, interval)

	// Graceful shutdown.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		log.Printf("[MarketData] Sinal %v recebido. Encerrando...", sig)
		cancel()
	}()

	svc.run(ctx)
	log.Printf("[MarketData] Encerrado.")
}
