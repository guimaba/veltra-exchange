package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gorilla/websocket"

	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/messaging"
)

var accountRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

// Server agrega tudo: state, hub, publisher, auth e static dir.
type Server struct {
	state     *State
	veltra    *VeltraState
	hub       *Hub
	publisher *messaging.Publisher
	auth      *AuthServer // nil quando Postgres não está configurado
	staticDir string
}

func NewServer(state *State, veltra *VeltraState, hub *Hub, pub *messaging.Publisher, auth *AuthServer, staticDir string) *Server {
	return &Server{state: state, veltra: veltra, hub: hub, publisher: pub, auth: auth, staticDir: staticDir}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	// API REST
	mux.HandleFunc("/api/health", s.health)
	mux.HandleFunc("/api/state", s.getState)
	mux.HandleFunc("/api/accounts/credit", s.postCredit)
	mux.HandleFunc("/api/transactions", s.postTransaction)
	mux.HandleFunc("/api/accounts/", s.getBalance) // /api/accounts/{name}/balance

	// API REST de autenticação
	s.authRoutes(mux)

	// API REST de saldo + admin
	s.balanceRoutes(mux)

	// API REST de market data
	s.marketRoutes(mux)

	// API REST da Veltra Exchange (trading)
	s.veltraRoutes(mux)

	// WebSocket
	mux.HandleFunc("/ws", s.serveWS)

	// Static (Flutter Web)
	if s.staticDir != "" {
		fs := http.FileServer(http.Dir(s.staticDir))
		mux.Handle("/", spaFallback(s.staticDir, fs))
	}

	return logging(mux)
}

// ===== Handlers REST =====

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"clients": s.hub.ClientCount(),
		"leader":  s.state.Leader(),
	})
}

func (s *Server) getState(w http.ResponseWriter, r *http.Request) {
	body, err := s.state.SnapshotJSON()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
}

func (s *Server) getBalance(w http.ResponseWriter, r *http.Request) {
	// Espera /api/accounts/{name}/balance
	rest := strings.TrimPrefix(r.URL.Path, "/api/accounts/")
	parts := strings.Split(rest, "/")
	if len(parts) != 2 || parts[1] != "balance" {
		http.NotFound(w, r)
		return
	}
	account := parts[0]
	if !accountRegex.MatchString(account) {
		writeError(w, http.StatusBadRequest, "conta invalida")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"account": account,
		"balance": s.state.Balance(account),
	})
}

type creditReq struct {
	Account string  `json:"account"`
	Amount  float64 `json:"amount"`
}

func (s *Server) postCredit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}
	var req creditReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "JSON invalido: "+err.Error())
		return
	}
	if !accountRegex.MatchString(req.Account) {
		writeError(w, http.StatusBadRequest, "conta invalida")
		return
	}
	if req.Amount <= 0 {
		writeError(w, http.StatusBadRequest, "amount deve ser > 0")
		return
	}

	env, err := messaging.NewEnvelope(messaging.SchemaCreditRequested, "", messaging.CreditRequestedPayload{
		Account: req.Account,
		Amount:  req.Amount,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := s.publisher.Publish(r.Context(), messaging.ExchangeCommands, messaging.RKCreditRequested, env, nil); err != nil {
		log.Printf("[Gateway] Falha ao publicar credit: %v", err)
		writeError(w, http.StatusServiceUnavailable, "broker indisponivel")
		return
	}
	s.state.MarkPending(env.TxID)

	writeJSON(w, http.StatusAccepted, map[string]any{
		"status": "queued",
		"tx_id":  env.TxID,
	})
}

type transactionReq struct {
	Sender   string  `json:"sender"`
	Receiver string  `json:"receiver"`
	Amount   float64 `json:"amount"`
}

func (s *Server) postTransaction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}
	var req transactionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "JSON invalido: "+err.Error())
		return
	}
	if !accountRegex.MatchString(req.Sender) {
		writeError(w, http.StatusBadRequest, "sender invalido")
		return
	}
	if !accountRegex.MatchString(req.Receiver) {
		writeError(w, http.StatusBadRequest, "receiver invalido")
		return
	}
	if req.Sender == req.Receiver {
		writeError(w, http.StatusBadRequest, "sender e receiver nao podem ser iguais")
		return
	}
	if req.Amount <= 0 {
		writeError(w, http.StatusBadRequest, "amount deve ser > 0")
		return
	}

	env, err := messaging.NewEnvelope(messaging.SchemaTransactionRequested, "", messaging.TransactionRequestedPayload{
		Sender:   req.Sender,
		Receiver: req.Receiver,
		Amount:   req.Amount,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := s.publisher.Publish(r.Context(), messaging.ExchangeCommands, messaging.RKTransactionRequested, env, nil); err != nil {
		log.Printf("[Gateway] Falha ao publicar transaction: %v", err)
		writeError(w, http.StatusServiceUnavailable, "broker indisponivel")
		return
	}
	s.state.MarkPending(env.TxID)

	writeJSON(w, http.StatusAccepted, map[string]any{
		"status": "queued",
		"tx_id":  env.TxID,
	})
}

// ===== WebSocket =====

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true }, // dev: aceita qualquer origin
}

func (s *Server) serveWS(w http.ResponseWriter, r *http.Request) {
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[Gateway] Falha no upgrade WS: %v", err)
		return
	}
	s.hub.Register <- conn

	// Envia snapshots iniciais (blockchain + veltra)
	if snap, err := s.state.SnapshotJSON(); err == nil {
		conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"snapshot","data":`+string(snap)+`}`))
	}
	if snap, err := s.veltra.SnapshotJSON(); err == nil {
		conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"veltra_snapshot","data":`+string(snap)+`}`))
	}

	// Loop de leitura: ignora payloads do cliente, so detecta close.
	go func() {
		defer func() { s.hub.Unreg <- conn }()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()
}

// ===== Helpers =====

func decodeJSON(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[HTTP] %s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

// spaFallback serve os arquivos do diretorio static. Para rotas que nao
// correspondem a um arquivo (ex: /carteira), serve index.html para que
// o roteador do Flutter Web assuma o controle.
func spaFallback(staticDir string, fs http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			path := filepath.Join(staticDir, filepath.Clean(r.URL.Path))
			info, err := os.Stat(path)
			if err != nil || info.IsDir() {
				http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
				return
			}
		}
		fs.ServeHTTP(w, r)
	})
}
