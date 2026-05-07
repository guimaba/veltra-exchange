package messaging

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Envelope e o formato unificado de toda mensagem que trafega pelo Rabbit.
// Documentado na Etapa 5 (consideracoes tecnicas).
type Envelope struct {
	Schema    string          `json:"schema"`
	TxID      string          `json:"tx_id"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

// NewEnvelope monta um envelope para um payload arbitrario, gerando tx_id
// automaticamente se nao fornecido.
func NewEnvelope(schema, txID string, payload any) (Envelope, error) {
	if txID == "" {
		txID = uuid.NewString()
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, err
	}
	return Envelope{
		Schema:    schema,
		TxID:      txID,
		Timestamp: time.Now().UTC(),
		Payload:   body,
	}, nil
}

// MarshalJSON serializa o envelope em JSON, formato que vai pro corpo da mensagem AMQP.
func (e Envelope) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

// Unmarshal extrai o payload tipado de um envelope.
func (e Envelope) Unmarshal(into any) error {
	return json.Unmarshal(e.Payload, into)
}

// ParseEnvelope decodifica os bytes recebidos do AMQP.
func ParseEnvelope(body []byte) (Envelope, error) {
	var env Envelope
	err := json.Unmarshal(body, &env)
	return env, err
}

// ----- Payloads tipados -----
//
// Cada payload abaixo corresponde a um Schema em topology.go. O campo TxID e
// redundante com o do envelope mas facilita consumers que so olham o payload.

type CreditRequestedPayload struct {
	Account string  `json:"account"`
	Amount  float64 `json:"amount"`
}

type CreditAddedPayload struct {
	Account    string  `json:"account"`
	Amount     float64 `json:"amount"`
	NewBalance float64 `json:"new_balance"`
}

type TransactionRequestedPayload struct {
	Sender   string  `json:"sender"`
	Receiver string  `json:"receiver"`
	Amount   float64 `json:"amount"`
}

type TransactionReceivedPayload struct {
	Sender        string             `json:"sender"`
	Receiver      string             `json:"receiver"`
	Amount        float64            `json:"amount"`
	BalanceAfter  map[string]float64 `json:"balance_after,omitempty"`
}

type TransactionRejectedPayload struct {
	Sender         string  `json:"sender"`
	Receiver       string  `json:"receiver"`
	Amount         float64 `json:"amount"`
	Reason         string  `json:"reason"`
	CurrentBalance float64 `json:"current_balance,omitempty"`
}

// BlockMinedTransaction e a representacao reduzida de uma transacao dentro do
// payload de block.mined. Mantida separada do tipo blockchain.Transaction para
// nao acoplar o pacote messaging ao pacote blockchain.
type BlockMinedTransaction struct {
	TxID     string  `json:"tx_id"`
	Sender   string  `json:"sender"`
	Receiver string  `json:"receiver"`
	Amount   float64 `json:"amount"`
	Kind     string  `json:"kind"`
}

type BlockMinedPayload struct {
	Index        int                     `json:"index"`
	PrevHash     string                  `json:"previous_hash"`
	Hash         string                  `json:"hash"`
	Nonce        int                     `json:"nonce"`
	Transactions []BlockMinedTransaction `json:"transactions"`
	MinerNodeID  int                     `json:"miner_node_id"`
}

type LeaderChangedPayload struct {
	PreviousLeader int    `json:"previous_leader"`
	NewLeader      int    `json:"new_leader"`
	Reason         string `json:"reason"`
}
