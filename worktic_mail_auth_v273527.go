package main

// Worktic AI V27.35.27 — Adapts the existing authentication model; no additional database driver.
// The approved V27.35.25 login, password hashes, tenant model and Go session cookies are preserved.
import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"worktic-omnichannel-ai-v3/internal/workticaccountmail"
)

var workticRecoveryV273527 *workticaccountmail.Service
var workticWelcomeSlotsV273527 = make(chan struct{}, 6)

type workticAccountAdapterV273527 struct{ app *App }

func (ad workticAccountAdapterV273527) FindByEmail(ctx context.Context, email string) (workticaccountmail.Account, bool, error) {
	// app_users is global; use an exact normalized email, matching the existing login handler.
	// We intentionally do not filter by tenant: same identity and email namespace as login.
	var id int64
	var name, storedEmail string
	var active int
	err := ad.app.db.QueryRow(`SELECT id,name,email,active FROM app_users WHERE lower(email)=lower(?)`, strings.TrimSpace(email)).Scan(&id, &name, &storedEmail, &active)
	if errors.Is(err, sql.ErrNoRows) {
		return workticaccountmail.Account{}, false, nil
	}
	if err != nil {
		return workticaccountmail.Account{}, false, err
	}
	if active != 1 {
		return workticaccountmail.Account{}, false, nil
	}
	return workticaccountmail.Account{ID: strconv.FormatInt(id, 10), Name: name, Email: storedEmail, Locale: "es"}, true, nil
}
func (ad workticAccountAdapterV273527) HashPassword(password string) (string, error) {
	return hashPasswordSecure(password) // Argon2id, same function used by existing registerHandler.
}
func (ad workticAccountAdapterV273527) UpdatePassword(ctx context.Context, tx *sql.Tx, accountID, hash string) error {
	id, err := strconv.ParseInt(accountID, 10, 64)
	if err != nil || id < 1 {
		return errors.New("invalid account id")
	}
	query := adaptSQL(`UPDATE app_users SET password_hash=? WHERE id=? AND active=1`, ad.app.db.dialect)
	res, err := tx.ExecContext(ctx, query, hash, id)
	if err != nil {
		return err
	}
	count, err := res.RowsAffected()
	if err != nil || count != 1 {
		return errors.New("account no longer available")
	}
	// All existing sessions must be invalidated as part of the same transaction.
	if _, err = tx.ExecContext(ctx, adaptSQL(`DELETE FROM app_sessions WHERE user_id=?`, ad.app.db.dialect), id); err != nil {
		return err
	}
	return nil
}
func workticRecoveryUnavailableV273527(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = w.Write([]byte(`{"error":"El servicio de recuperación está temporalmente deshabilitado."}`))
}

// Runs at startup, registering both endpoints even when email is not yet configured.
// A missing or invalid email key cannot crash Worktic AI's existing login.
func initWorkticAccountMailV273527(a *App, mux *http.ServeMux) error {
	mux.HandleFunc("/api/auth/password/reset/request", func(w http.ResponseWriter, r *http.Request) {
		if workticRecoveryV273527 == nil {
			workticRecoveryUnavailableV273527(w, r)
			return
		}
		workticRecoveryV273527.Request(w, r)
	})
	mux.HandleFunc("/api/auth/password/reset/confirm", func(w http.ResponseWriter, r *http.Request) {
		if workticRecoveryV273527 == nil {
			workticRecoveryUnavailableV273527(w, r)
			return
		}
		workticRecoveryV273527.Confirm(w, r)
	})
	if !strings.EqualFold(strings.TrimSpace(os.Getenv("WORKTIC_MAIL_ENABLED")), "true") {
		log.Printf("WorkticAI V27.35.27: transactional email disabled; login unchanged")
		return nil
	}
	if !a.db.IsPostgres() {
		return errors.New("password recovery requires PostgreSQL; existing login remains available")
	}
	sender, err := workticaccountmail.MailConfigFromEnv().Sender()
	if err != nil {
		return fmt.Errorf("mail configuration: %w", err)
	}
	base := strings.TrimSpace(os.Getenv("BASE_URL"))
	if base == "" {
		base = "https://workticai.com"
	}
	if err := initRecoverySchemaV273527(a); err != nil {
		return fmt.Errorf("reset table: %w", err)
	}
	service, err := workticaccountmail.New(a.db.raw, workticAccountAdapterV273527{a}, sender, base)
	if err != nil {
		return err
	}
	workticRecoveryV273527 = service
	log.Printf("WorkticAI V27.35.27: email recovery and transactional messages enabled")
	return nil
}

func initRecoverySchemaV273527(a *App) error {
	// No ALTER TABLE and no migration of existing users, sessions, or businesses.
	_, err := a.db.Exec(`CREATE TABLE IF NOT EXISTS worktic_password_reset_tokens (
        token_hash BYTEA PRIMARY KEY,
        account_id TEXT NOT NULL,
        email TEXT NOT NULL,
        created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
        expires_at TIMESTAMPTZ NOT NULL,
        consumed_at TIMESTAMPTZ
    )`)
	if err != nil {
		return err
	}
	if _, err = a.db.Exec(`CREATE INDEX IF NOT EXISTS idx_worktic_reset_user ON worktic_password_reset_tokens(account_id,created_at DESC)`); err != nil {
		return err
	}
	if _, err = a.db.Exec(`CREATE INDEX IF NOT EXISTS idx_worktic_reset_expiry ON worktic_password_reset_tokens(expires_at)`); err != nil {
		return err
	}
	return nil
}

// Nonblocking and bounded: registration succeeds even if email is temporarily unavailable.
// Never sends a temporary password or stores account credentials in a queue.
func (a *App) enqueueWelcomeMailV273527(name, email string) {
	s := workticRecoveryV273527
	if s == nil {
		return
	}
	select {
	case workticWelcomeSlotsV273527 <- struct{}{}:
		go func() {
			defer func() { <-workticWelcomeSlotsV273527 }()
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			if err := s.SendWelcome(ctx, strings.TrimSpace(name), strings.ToLower(strings.TrimSpace(email)), "es"); err != nil {
				log.Printf("WorkticAI V27.35.27: welcome mail could not be delivered")
			}
		}()
	default:
		log.Printf("WorkticAI V27.35.27: welcome mail queue full")
	}
}
