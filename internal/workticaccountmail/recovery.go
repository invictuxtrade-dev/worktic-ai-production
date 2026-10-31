// Package workticaccountmail provides secure reset links and transactional notifications.
// It deliberately never guesses Worktic AI's account schema or hashing algorithm.
package workticaccountmail

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"sync"
	"time"
)

type Account struct{ ID, Email, Name, Locale string }
type Accounts interface {
	// Return (Account{}, false, nil) when no account matches.
	FindByEmail(ctx context.Context, email string) (Account, bool, error)
	// MUST use the hash algorithm already accepted by Worktic AI login.
	HashPassword(password string) (string, error)
	// Must update the existing account record using tx, and not commit tx.
	UpdatePassword(ctx context.Context, tx *sql.Tx, accountID, hash string) error
}
type Sender interface {
	Send(ctx context.Context, to, subject, html, plain string) error
}

type Service struct {
	DB       *sql.DB
	Accounts Accounts
	Mail     Sender
	BaseURL  string
	Logger   *log.Logger
	mu       sync.Mutex
	requests map[string][]time.Time
}

func New(db *sql.DB, accounts Accounts, sender Sender, baseURL string) (*Service, error) {
	if db == nil || accounts == nil || sender == nil {
		return nil, errors.New("database, account adapter and mail provider are required")
	}
	u, err := url.Parse(baseURL)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("BASE_URL must be a clean https URL")
	}
	return &Service{DB: db, Accounts: accounts, Mail: sender, BaseURL: strings.TrimRight(baseURL, "/"), requests: make(map[string][]time.Time)}, nil
}

// Mount these routes on the existing Worktic AI mux. Do not create a second server.
func (s *Service) Mount(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/auth/password/reset/request", s.Request)
	mux.HandleFunc("POST /api/auth/password/reset/confirm", s.Confirm)
}

func jsonResponse(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func decode(r *http.Request, dest any) error {
	if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(strings.ToLower(ct), "application/json") {
		return errors.New("content type required")
	}
	d := json.NewDecoder(io.LimitReader(r.Body, 4096))
	d.DisallowUnknownFields()
	if err := d.Decode(dest); err != nil { return err }; var extra any; if err := d.Decode(&extra); err != io.EOF { return errors.New("extra JSON data") }; return nil
}
func validEmail(s string) bool {
	if len(s) < 3 || len(s) > 254 || strings.ContainsAny(s, "\r\n ") {
		return false
	}
	a, err := mail.ParseAddress(s)
	return err == nil && a.Address == s
}
func (s *Service) rateLimit(key string, max int, per time.Duration) bool {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.requests) > 20000 {
		s.requests = make(map[string][]time.Time)
	}
	times := s.requests[key]
	valid := times[:0]
	for _, t := range times {
		if now.Sub(t) < per {
			valid = append(valid, t)
		}
	}
	if len(valid) >= max {
		s.requests[key] = valid
		return false
	}
	s.requests[key] = append(valid, now)
	return true
}
func (s *Service) warn(msg string, err error) {
	if s.Logger != nil {
		s.Logger.Printf("account email: %s: %v", msg, err)
	}
}
func randomToken() (string, []byte, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	h := sha256.Sum256([]byte(token))
	return token, h[:], nil
}
func digestToken(token string) ([]byte, bool) {
	if len(token) != 43 || strings.ContainsAny(token, "= /+\r\n") {
		return nil, false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(decoded) != 32 {
		return nil, false
	}
	h := sha256.Sum256([]byte(token))
	return h[:], true
}

var accepted = map[string]string{"message": "Si el correo está registrado, recibirás un enlace para restablecer tu contraseña."}

func (s *Service) Request(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", 405)
		return
	}
	var body struct {
		Email string `json:"email"`
	}
	if err := decode(r, &body); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "Solicitud no válida"})
		return
	}
	email := strings.ToLower(strings.TrimSpace(body.Email))
	if !validEmail(email) {
		jsonResponse(w, 400, map[string]string{"error": "Introduce un correo válido"})
		return
	}
	// Generic response for unknown addresses AND limited requests: no account enumeration.
	ip, _, splitErr := net.SplitHostPort(r.RemoteAddr)
	if splitErr != nil {
		ip = r.RemoteAddr
	}
	if !s.rateLimit("ip:"+ip, 8, time.Hour) || !s.rateLimit("email:"+email, 3, time.Hour) {
		jsonResponse(w, 200, accepted)
		return
	}
	account, ok, err := s.Accounts.FindByEmail(r.Context(), email)
	if err != nil {
		s.warn("lookup", err)
		jsonResponse(w, 200, accepted)
		return
	}
	if !ok {
		jsonResponse(w, 200, accepted)
		return
	}
	token, digest, err := randomToken()
	if err != nil {
		s.warn("token generation", err)
		jsonResponse(w, 503, map[string]string{"error": "Servicio no disponible"})
		return
	}
	// DB throttle protects against multiple application replicas.
	var recent bool
	_ = s.DB.QueryRowContext(r.Context(), `SELECT EXISTS (SELECT 1 FROM worktic_password_reset_tokens WHERE account_id=$1 AND created_at>NOW()-interval '90 seconds')`, account.ID).Scan(&recent)
	if recent {
		jsonResponse(w, 200, accepted)
		return
	}
	_, err = s.DB.ExecContext(r.Context(), `INSERT INTO worktic_password_reset_tokens(token_hash,account_id,email,created_at,expires_at) VALUES ($1,$2,$3,NOW(),NOW()+interval '20 minutes')`, digest, account.ID, account.Email)
	if err != nil {
		s.warn("save token", err)
		jsonResponse(w, 200, accepted)
		return
	}
	resetURL := s.BaseURL + "/login.html#reset=" + url.QueryEscape(token)
	subject, html, plain := ResetEmail(account.Name, resetURL, account.Locale)
	if err := s.Mail.Send(r.Context(), account.Email, subject, html, plain); err != nil {
		s.warn("send reset email", err)
		_, _ = s.DB.ExecContext(context.WithoutCancel(r.Context()), `DELETE FROM worktic_password_reset_tokens WHERE token_hash=$1`, digest)
		jsonResponse(w, 200, accepted)
		return
	}
	jsonResponse(w, 200, accepted)
}
func (s *Service) Confirm(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", 405)
		return
	}
	var body struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := decode(r, &body); err != nil {
		jsonResponse(w, 400, map[string]string{"error": "Solicitud no válida"})
		return
	}
	digest, ok := digestToken(body.Token)
	if !ok || len(body.Password) < 12 || len(body.Password) > 128 {
		jsonResponse(w, 400, map[string]string{"error": "Enlace o contraseña inválidos (mínimo 12 caracteres)"})
		return
	}
	hash, err := s.Accounts.HashPassword(body.Password)
	if err != nil {
		s.warn("hash", err)
		jsonResponse(w, 503, map[string]string{"error": "No se pudo actualizar"})
		return
	}
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		jsonResponse(w, 503, map[string]string{"error": "No se pudo actualizar"})
		return
	}
	defer tx.Rollback()
	var id, email string
	err = tx.QueryRowContext(r.Context(), `SELECT account_id,email FROM worktic_password_reset_tokens WHERE token_hash=$1 AND consumed_at IS NULL AND expires_at>NOW() FOR UPDATE`, digest).Scan(&id, &email)
	if err != nil {
		jsonResponse(w, 400, map[string]string{"error": "El enlace ha caducado o ya fue utilizado"})
		return
	}
	if err = s.Accounts.UpdatePassword(r.Context(), tx, id, hash); err != nil {
		s.warn("update password", err)
		jsonResponse(w, 503, map[string]string{"error": "No se pudo actualizar"})
		return
	}
	_, err = tx.ExecContext(r.Context(), `UPDATE worktic_password_reset_tokens SET consumed_at=NOW() WHERE account_id=$1 AND consumed_at IS NULL`, id)
	if err != nil {
		s.warn("consume token", err)
		jsonResponse(w, 503, map[string]string{"error": "No se pudo actualizar"})
		return
	}
	if err = tx.Commit(); err != nil {
		s.warn("commit", err)
		jsonResponse(w, 503, map[string]string{"error": "No se pudo actualizar"})
		return
	}
	subject, html, plain := PasswordChangedEmail("", s.BaseURL, "")
	mailCtx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 10*time.Second)
	defer cancel()
	if err := s.Mail.Send(mailCtx, email, subject, html, plain); err != nil {
		s.warn("send password changed", err)
	}
	jsonResponse(w, 200, map[string]string{"message": "Contraseña actualizada. Ya puedes ingresar."})
}

// Call AFTER the existing successful-registration database transaction commits.
// This method does not create accounts and never sends an initial password.
func (s *Service) SendWelcome(ctx context.Context, name, email, locale string) error {
	if !validEmail(email) {
		return errors.New("invalid recipient")
	}
	subject, html, plain := WelcomeEmail(name, s.BaseURL, locale)
	return s.Mail.Send(ctx, email, subject, html, plain)
}
