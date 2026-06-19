package messaging

// Payloads tipados da Veltra Exchange. Mantidos desacoplados dos pacotes de
// dominio (exchange/money): todo valor monetario trafega como int64 na menor
// unidade (money.Amount escalado por money.Scale), e enums como string. O
// matching engine faz o mapeamento entre estes payloads e os tipos de dominio.
//
// Cada payload corresponde a um Schema declarado em topology.go.

// ----- Comandos -----

// OrderPlacePayload e o comando para registrar uma nova ordem.
type OrderPlacePayload struct {
	ClientOrderID string `json:"client_order_id"`
	Account       string `json:"account"`
	Pair          string `json:"pair"` // "BASE/QUOTE"
	Side          string `json:"side"` // "buy" | "sell"
	Type          string `json:"type"` // "limit" | "market"
	TimeInForce   string `json:"time_in_force"`
	Price         int64  `json:"price"`    // menor unidade; 0 para market
	Quantity      int64  `json:"quantity"` // menor unidade, em BASE
}

// OrderCancelPayload e o comando para cancelar uma ordem em repouso.
type OrderCancelPayload struct {
	OrderID       string `json:"order_id,omitempty"`
	ClientOrderID string `json:"client_order_id,omitempty"`
	Account       string `json:"account"`
	Pair          string `json:"pair"`
}

// FaucetCreditPayload emite saldo virtual para uma conta (admin/faucet).
type FaucetCreditPayload struct {
	Account string `json:"account"`
	Asset   string `json:"asset"`
	Amount  int64  `json:"amount"` // menor unidade
}

// ----- Eventos -----

// OrderAcceptedPayload confirma que a ordem foi sequenciada e entrou no motor.
type OrderAcceptedPayload struct {
	OrderID       string `json:"order_id"`
	ClientOrderID string `json:"client_order_id"`
	Account       string `json:"account"`
	Pair          string `json:"pair"`
	Side          string `json:"side"`
	Type          string `json:"type"`
	Price         int64  `json:"price"`
	Quantity      int64  `json:"quantity"`
	Sequence      uint64 `json:"sequence"`
}

// OrderRejectedPayload sinaliza recusa pre-trade ou no motor.
type OrderRejectedPayload struct {
	ClientOrderID string `json:"client_order_id"`
	Account       string `json:"account"`
	Pair          string `json:"pair"`
	Reason        string `json:"reason"`
}

// OrderCanceledPayload confirma cancelamento de uma ordem em repouso.
type OrderCanceledPayload struct {
	OrderID string `json:"order_id"`
	Account string `json:"account"`
	Pair    string `json:"pair"`
}

// OrderFilledPayload e o execution report de um fill (parcial ou total).
type OrderFilledPayload struct {
	OrderID          string `json:"order_id"`
	ClientOrderID    string `json:"client_order_id"`
	Account          string `json:"account"`
	Pair             string `json:"pair"`
	Side             string `json:"side"`
	Price            int64  `json:"price"`             // preco do fill
	FillQuantity     int64  `json:"fill_quantity"`     // quantidade deste fill
	CumulativeFilled int64  `json:"cumulative_filled"` // total executado da ordem
	RemainingQty     int64  `json:"remaining_qty"`
	Status           string `json:"status"`
}

// TradeExecutedPayload e o evento imutavel de casamento (fonte da verdade para
// ledger e market data).
type TradeExecutedPayload struct {
	TradeID      string `json:"trade_id"`
	Pair         string `json:"pair"`
	Price        int64  `json:"price"`
	Quantity     int64  `json:"quantity"`
	TakerOrderID string `json:"taker_order_id"`
	MakerOrderID string `json:"maker_order_id"`
	TakerAccount string `json:"taker_account"`
	MakerAccount string `json:"maker_account"`
	TakerSide    string `json:"taker_side"`
	Sequence     uint64 `json:"sequence"`
	TimestampMs  int64  `json:"timestamp_ms"`
}

// BookLevel e um nivel agregado do order book L2 (preco -> quantidade total).
type BookLevel struct {
	Price    int64 `json:"price"`
	Quantity int64 `json:"quantity"`
}

// BookUpdatedPayload publica um snapshot/delta do order book L2.
type BookUpdatedPayload struct {
	Pair     string      `json:"pair"`
	Bids     []BookLevel `json:"bids"`
	Asks     []BookLevel `json:"asks"`
	Sequence uint64      `json:"sequence"`
}

// LedgerEntry e um lancamento de dupla entrada (debito negativo, credito positivo).
type LedgerEntry struct {
	Account string `json:"account"`
	Asset   string `json:"asset"`
	Delta   int64  `json:"delta"` // negativo = debito, positivo = credito
}

// LedgerPostedPayload confirma um conjunto balanceado de lancamentos (soma=0
// por ativo), com a Merkle root do periodo para auditoria.
type LedgerPostedPayload struct {
	RefTxID    string        `json:"ref_tx_id"` // tx/trade que originou o lancamento
	Entries    []LedgerEntry `json:"entries"`
	MerkleRoot string        `json:"merkle_root,omitempty"`
}
