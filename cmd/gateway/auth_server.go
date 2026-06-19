package main

// auth_server.go: endpoints de autenticação da Veltra Exchange.
//
//	POST /api/auth/register  - cria usuário + conta de trading
//	POST /api/auth/login     - autentica, retorna JWT
//	POST /api/auth/logout    - invalida sessão (server-side)
//
// O token JWT é stateless (HS256, TTL 24h) com um claim extra "sid" que
// aponta para a linha em auth.sessions — logout invalida essa linha.

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/pgstore"
)

// jwtSecret é lido do env VELTRA_JWT_SECRET; fallback para dev only.
var jwtSecret = func() []byte {
	if s := os.Getenv("VELTRA_JWT_SECRET"); s != "" {
		return []byte(s)
	}
	return []byte("dev-insecure-secret-change-in-prod")
}()

var usernameRe = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,32}$`)

// AuthServer encapsula o DB de auth; fica como campo do Server.
type AuthServer struct {
	db *pgstore.DB
}

func NewAuthServer(db *pgstore.DB) *AuthServer {
	return &AuthServer{db: db}
}

// SeedDefaultAdmin cria o usuário admin/admin se ainda não existir.
// Chamado no startup do gateway; idempotente.
func (a *AuthServer) SeedDefaultAdmin(ctx context.Context) {
	var exists bool
	a.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM auth.users WHERE username = 'admin')`,
	).Scan(&exists)
	if exists {
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
	if err != nil {
		return
	}

	tx, err := a.db.Conn().BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()

	var userID int64
	if err := tx.QueryRowContext(ctx,
		`INSERT INTO auth.users (username, email, password_hash, is_admin)
		 VALUES ('admin', 'admin@veltra.local', $1, true) RETURNING id`,
		string(hash),
	).Scan(&userID); err != nil {
		return
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO auth.accounts (user_id, name) VALUES ($1, 'admin')`,
		userID,
	); err != nil {
		return
	}

	tx.Commit()
}

// authClaims é o payload do JWT.
type authClaims struct {
	UserID    int64  `json:"uid"`
	AccountID int64  `json:"aid"`
	Username  string `json:"sub"`
	Email     string `json:"email"`
	IsAdmin   bool   `json:"admin"`
	SessionID int64  `json:"sid"`
	jwt.RegisteredClaims
}

// authRoutes registra as rotas de auth no mux.
func (s *Server) authRoutes(mux *http.ServeMux) {
	if s.auth == nil {
		return
	}
	mux.HandleFunc("/api/auth/register", s.auth.handleRegister)
	mux.HandleFunc("/api/auth/login", s.auth.handleLogin)
	mux.HandleFunc("/api/auth/logout", s.auth.handleLogout)
	mux.HandleFunc("/api/auth/me", s.auth.handleMe)
}

// ===== Register =====

type registerReq struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (a *AuthServer) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}
	var req registerReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "JSON invalido: "+err.Error())
		return
	}
	if !usernameRe.MatchString(req.Username) {
		writeError(w, http.StatusBadRequest, "username: 3-32 caracteres, apenas letras/números/_/-")
		return
	}
	if !strings.Contains(req.Email, "@") {
		writeError(w, http.StatusBadRequest, "email invalido")
		return
	}
	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "senha deve ter ao menos 8 caracteres")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "erro ao processar senha")
		return
	}

	ctx := r.Context()

	// Transação: cria user + account
	tx, err := a.db.Conn().BeginTx(ctx, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "erro interno")
		return
	}
	defer tx.Rollback()

	var userID int64
	err = tx.QueryRowContext(ctx,
		`INSERT INTO auth.users (username, email, password_hash)
		 VALUES ($1, $2, $3) RETURNING id`,
		req.Username, req.Email, string(hash),
	).Scan(&userID)
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "usuário ou e-mail já existe")
			return
		}
		writeError(w, http.StatusInternalServerError, "erro ao criar usuário")
		return
	}

	var accountID int64
	err = tx.QueryRowContext(ctx,
		`INSERT INTO auth.accounts (user_id, name) VALUES ($1, $2) RETURNING id`,
		userID, req.Username,
	).Scan(&accountID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "erro ao criar conta")
		return
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "erro ao confirmar registro")
		return
	}

	token, sessionID, err := a.issueToken(ctx, userID, accountID, req.Username, req.Email, false, r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "erro ao gerar token")
		return
	}
	_ = sessionID

	writeJSON(w, http.StatusCreated, map[string]any{
		"token":      token,
		"user_id":    userID,
		"account_id": accountID,
		"username":   req.Username,
		"email":      req.Email,
		"is_admin":   false,
	})
}

// ===== Login =====

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (a *AuthServer) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}
	var req loginReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "JSON invalido: "+err.Error())
		return
	}
	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "username e password obrigatórios")
		return
	}

	ctx := r.Context()

	var userID int64
	var accountID int64
	var email string
	var hash string
	var isAdmin bool

	err := a.db.QueryRowContext(ctx, `
		SELECT u.id, u.email, u.password_hash, u.is_admin, a.id
		FROM auth.users u
		JOIN auth.accounts a ON a.user_id = u.id
		WHERE u.username = $1
		LIMIT 1`,
		req.Username,
	).Scan(&userID, &email, &hash, &isAdmin, &accountID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "usuário ou senha incorretos")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)); err != nil {
		writeError(w, http.StatusUnauthorized, "usuário ou senha incorretos")
		return
	}

	token, _, err := a.issueToken(ctx, userID, accountID, req.Username, email, isAdmin, r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "erro ao gerar token")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"token":      token,
		"user_id":    userID,
		"account_id": accountID,
		"username":   req.Username,
		"email":      email,
		"is_admin":   isAdmin,
	})
}

// ===== Logout =====

func (a *AuthServer) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}
	claims, err := a.claimsFromRequest(r)
	if err != nil {
		// Logout sem token válido: ok, apenas retornar 200
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}
	// Invalida a sessão no DB
	a.db.ExecContext(r.Context(),
		`UPDATE auth.sessions SET expires_at = CURRENT_TIMESTAMP WHERE id = $1`,
		claims.SessionID)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ===== Me (verifica token) =====

func (a *AuthServer) handleMe(w http.ResponseWriter, r *http.Request) {
	claims, err := a.claimsFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "token inválido ou expirado")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"user_id":    claims.UserID,
		"account_id": claims.AccountID,
		"username":   claims.Username,
		"email":      claims.Email,
		"is_admin":   claims.IsAdmin,
	})
}

// ===== Middleware: RequireAuth =====

// RequireAuth é um middleware que valida o JWT e injeta os claims no contexto.
// Usado para proteger endpoints de trading na Fase 3.
func (a *AuthServer) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, err := a.claimsFromRequest(r)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "autenticação obrigatória")
			return
		}
		ctx := context.WithValue(r.Context(), ctxKeyClaims{}, claims)
		next(w, r.WithContext(ctx))
	}
}

type ctxKeyClaims struct{}

// ClaimsFromContext extrai os claims do contexto da requisição.
func ClaimsFromContext(ctx context.Context) (*authClaims, bool) {
	c, ok := ctx.Value(ctxKeyClaims{}).(*authClaims)
	return c, ok
}

// ===== helpers internos =====

func (a *AuthServer) issueToken(ctx context.Context, userID, accountID int64, username, email string, isAdmin bool, r *http.Request) (string, int64, error) {
	expiresAt := time.Now().Add(24 * time.Hour)

	// Reserva um session ID sem token ainda (token placeholder único por timestamp)
	placeholder := fmt.Sprintf("pending-%d-%d", accountID, time.Now().UnixNano())
	var sessionID int64
	err := a.db.QueryRowContext(ctx,
		`INSERT INTO auth.sessions (account_id, session_token, ip_address, user_agent, expires_at)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		accountID, placeholder, r.RemoteAddr, r.UserAgent(), expiresAt,
	).Scan(&sessionID)
	if err != nil {
		return "", 0, err
	}

	claims := authClaims{
		UserID:    userID,
		AccountID: accountID,
		Username:  username,
		Email:     email,
		IsAdmin:   isAdmin,
		SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "veltra-exchange",
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := t.SignedString(jwtSecret)
	if err != nil {
		return "", 0, err
	}

	// Atualiza session_token com o JWT real
	a.db.ExecContext(ctx,
		`UPDATE auth.sessions SET session_token = $1 WHERE id = $2`,
		signed, sessionID)

	return signed, sessionID, nil
}

func (a *AuthServer) claimsFromRequest(r *http.Request) (*authClaims, error) {
	raw := r.Header.Get("Authorization")
	raw = strings.TrimPrefix(raw, "Bearer ")
	if raw == "" {
		return nil, errors.New("sem token")
	}
	t, err := jwt.ParseWithClaims(raw, &authClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("algoritmo inesperado")
		}
		return jwtSecret, nil
	})
	if err != nil || !t.Valid {
		return nil, errors.New("token inválido")
	}
	c, ok := t.Claims.(*authClaims)
	if !ok {
		return nil, errors.New("claims inválidos")
	}
	// Verifica sessão não foi invalidada por logout
	var expiresAt time.Time
	err = a.db.QueryRowContext(r.Context(),
		`SELECT expires_at FROM auth.sessions WHERE id = $1`, c.SessionID,
	).Scan(&expiresAt)
	if err != nil || expiresAt.Before(time.Now()) {
		return nil, errors.New("sessão expirada")
	}
	return c, nil
}

// isUniqueViolation detecta violação de constraint unique do Postgres (código 23505).
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "23505") ||
		strings.Contains(err.Error(), "unique") ||
		strings.Contains(err.Error(), "duplicate")
}
