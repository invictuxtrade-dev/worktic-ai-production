package workticactivation

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"html"
	"net/url"
	"strings"
	"time"

	"worktic-omnichannel-ai-v3/internal/workticaccountmail"
)

type Verification struct {
	DB      *sql.DB
	Sender  workticaccountmail.Sender
	BaseURL string
}

func (s Verification) Validate() error {
	if s.DB == nil || s.Sender == nil {
		return errors.New("DB and sender are required")
	}
	u, err := url.Parse(s.BaseURL)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("invalid BASE_URL")
	}
	return nil
}

// Only additive tables, with no alterations to existing user/session/tenant records.
func (s Verification) InitSchema(ctx context.Context) error {
	if err := s.Validate(); err != nil {
		return err
	}
	statements := []string{
		`CREATE TABLE IF NOT EXISTS worktic_email_activation_state (
            user_id BIGINT PRIMARY KEY, tenant_id BIGINT NOT NULL, email TEXT NOT NULL,
            name TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            activated_at TIMESTAMPTZ, purged_at TIMESTAMPTZ,
            resend_count INT NOT NULL DEFAULT 0, resend_day DATE NOT NULL DEFAULT CURRENT_DATE,
            reminder_sent_at TIMESTAMPTZ,
            last_sent_at TIMESTAMPTZ, purge_after TIMESTAMPTZ NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS worktic_email_activation_tokens (
            token_hash BYTEA PRIMARY KEY, user_id BIGINT NOT NULL,
            expires_at TIMESTAMPTZ NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            consumed_at TIMESTAMPTZ)`,
		`ALTER TABLE worktic_email_activation_state ADD COLUMN IF NOT EXISTS reminder_sent_at TIMESTAMPTZ`,
		`CREATE INDEX IF NOT EXISTS idx_worktic_activation_user ON worktic_email_activation_tokens(user_id,expires_at)`,
		`CREATE INDEX IF NOT EXISTS idx_worktic_activation_purge ON worktic_email_activation_state(purge_after) WHERE activated_at IS NULL AND purged_at IS NULL`,
		`CREATE TABLE IF NOT EXISTS worktic_captcha_challenges (
            id_hash BYTEA PRIMARY KEY, correct_index INT NOT NULL,
            expires_at TIMESTAMPTZ NOT NULL, consumed_at TIMESTAMPTZ,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE INDEX IF NOT EXISTS idx_worktic_captcha_expiry ON worktic_captcha_challenges(expires_at)`,
	}
	for _, stmt := range statements {
		if _, err := s.DB.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func newToken() (string, []byte, error) {
	b, err := secureRandom(32)
	if err != nil {
		return "", nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	d := sha256.Sum256([]byte(token))
	return token, d[:], nil
}

// Call before COMMIT in the existing registration transaction.
// No welcome email is sent here. Email is sent only after successful activation.
func (s Verification) CreatePending(ctx context.Context, tx *sql.Tx, userID, tenantID int64, name, email string) (string, error) {
	if userID <= 0 || tenantID <= 0 {
		return "", errors.New("invalid account")
	}
	token, digest, err := newToken()
	if err != nil {
		return "", err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO worktic_email_activation_state(user_id,tenant_id,name,email,purge_after) VALUES($1,$2,$3,$4,NOW()+INTERVAL '72 hours')`, userID, tenantID, strings.TrimSpace(name), strings.ToLower(strings.TrimSpace(email)))
	if err != nil {
		return "", err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO worktic_email_activation_tokens(token_hash,user_id,expires_at) VALUES($1,$2,NOW()+INTERVAL '24 hours')`, digest, userID)
	return token, err
}

func (s Verification) SendActivation(ctx context.Context, name, email, token string) error {
	if err := s.Validate(); err != nil {
		return err
	}
	link := strings.TrimRight(s.BaseURL, "/") + "/login.html#activate=" + url.QueryEscape(token)
	safe := html.EscapeString(name)
	if safe == "" {
		safe = "" + "Hola"
	}
	subject := "Activa tu cuenta de Worktic AI"
	body := `<!DOCTYPE html><html lang="es"><meta charset="utf-8"><body style="background:#f5f7fd;padding:32px;font-family:Arial,sans-serif;color:#101c40"><div style="max-width:560px;margin:auto;background:#fff;border-radius:22px;padding:38px;border:1px solid #dde3fb"><div style="font-size:26px;font-weight:bold;color:#3754ed">Worktic <span style="color:#d92fea">AI</span></div><h1 style="font-size:25px">Confirma tu correo</h1><p>Hola, ` + safe + `. Para comenzar a usar Worktic AI, primero confirma tu dirección de correo electrónico.</p><p style="margin:32px 0"><a href="` + html.EscapeString(link) + `" style="padding:16px 27px;border-radius:13px;background:#5544ef;color:#fff;text-decoration:none;font-weight:bold">Activar mi cuenta</a></p><p>Este enlace es de un solo uso y vence en 24 horas. Si no creaste esta cuenta, ignora este mensaje.</p><p style="font-size:12px;color:#7280a3">Si no activas tu cuenta en 72 horas, el registro se eliminará.</p></div></body></html>`
	plain := "Hola, " + name + ". Activa tu cuenta de Worktic AI: " + link + "\nEste enlace vence en 24 horas."
	return s.Sender.Send(ctx, email, subject, body, plain)
}

// Atomic activation in PostgreSQL. The caller may send the welcome email only AFTER success.
func (s Verification) Activate(ctx context.Context, token string) (string, string, error) {
	if len(token) != 43 {
		return "", "", errors.New("invalid or expired token")
	}
	if _, err := base64.RawURLEncoding.DecodeString(token); err != nil {
		return "", "", errors.New("invalid or expired token")
	}
	digest := sha256.Sum256([]byte(token))
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return "", "", err
	}
	defer tx.Rollback()
	var userID int64
	err = tx.QueryRowContext(ctx, `SELECT user_id FROM worktic_email_activation_tokens WHERE token_hash=$1 AND consumed_at IS NULL AND expires_at>NOW() FOR UPDATE`, digest[:]).Scan(&userID)
	if err != nil {
		return "", "", errors.New("invalid or expired token")
	}
	var email, name string
	err = tx.QueryRowContext(ctx, `SELECT email,name FROM worktic_email_activation_state WHERE user_id=$1 AND activated_at IS NULL AND purged_at IS NULL AND purge_after>NOW() FOR UPDATE`, userID).Scan(&email, &name)
	if err != nil {
		return "", "", errors.New("account no longer pending")
	}
	res, err := tx.ExecContext(ctx, `UPDATE app_users SET active=1 WHERE id=$1 AND active=0`, userID)
	if err != nil {
		return "", "", err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return "", "", errors.New("account unavailable")
	}
	if _, err = tx.ExecContext(ctx, `UPDATE worktic_email_activation_state SET activated_at=NOW() WHERE user_id=$1`, userID); err != nil {
		return "", "", err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE worktic_email_activation_tokens SET consumed_at=NOW() WHERE user_id=$1 AND consumed_at IS NULL`, userID); err != nil {
		return "", "", err
	}
	if err = tx.Commit(); err != nil {
		return "", "", err
	}
	return name, email, nil
}

// Resend returns the same generic result regardless of account existence.
func (s Verification) Resend(ctx context.Context, email string) (string, string, string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if len(email) > 254 || !strings.Contains(email, "@") {
		return "", "", "", nil
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return "", "", "", err
	}
	defer tx.Rollback()
	var id int64
	var name, registeredEmail string
	var count int
	var last sql.NullTime
	var day time.Time
	err = tx.QueryRowContext(ctx, `SELECT user_id,name,email,resend_count,resend_day,last_sent_at FROM worktic_email_activation_state WHERE lower(email)=lower($1) AND activated_at IS NULL AND purged_at IS NULL AND purge_after>NOW() FOR UPDATE`, email).Scan(&id, &name, &registeredEmail, &count, &day, &last)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", "", nil
	}
	if err != nil {
		return "", "", "", err
	}
	now := time.Now().UTC()
	if last.Valid && now.Sub(last.Time) < time.Minute {
		return "", "", "", nil
	}
	if day.Format("2006-01-02") != now.Format("2006-01-02") {
		count = 0
	}
	if count >= 5 {
		return "", "", "", nil
	}
	token, digest, err := newToken()
	if err != nil {
		return "", "", "", err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE worktic_email_activation_tokens SET consumed_at=NOW() WHERE user_id=$1 AND consumed_at IS NULL`, id); err != nil {
		return "", "", "", err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO worktic_email_activation_tokens(token_hash,user_id,expires_at) VALUES($1,$2,NOW()+INTERVAL '24 hours')`, digest, id); err != nil {
		return "", "", "", err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE worktic_email_activation_state SET resend_count=$2,resend_day=CURRENT_DATE,last_sent_at=NOW() WHERE user_id=$1`, id, count+1); err != nil {
		return "", "", "", err
	}
	if err = tx.Commit(); err != nil {
		return "", "", "", err
	}
	return name, registeredEmail, token, nil
}

// A cleanup operation never touches accounts that have successfully activated.
// Pending accounts older than 72 hours are disabled and anonymized without breaking tenant foreign keys.
// Physical removal of tenant dependencies requires a reviewed migration.
// This action irreversibly removes personal data and denies access at 72h.
func (s Verification) AnonymizeExpired(ctx context.Context) (int64, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT user_id FROM worktic_email_activation_state WHERE activated_at IS NULL AND purged_at IS NULL AND purge_after<NOW() FOR UPDATE SKIP LOCKED LIMIT 50`)
	if err != nil {
		return 0, err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()
	for _, id := range ids {
		pseudo := fmt.Sprintf("unverified-deleted-%d@invalid.workticai.local", id)
		if _, err = tx.ExecContext(ctx, `UPDATE app_users SET active=0,email=$2,name='Cuenta eliminada',company='',password_hash='DISABLED' WHERE id=$1 AND active=0`, id, pseudo); err != nil {
			return 0, err
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM worktic_email_activation_tokens WHERE user_id=$1`, id); err != nil {
			return 0, err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE worktic_email_activation_state SET email='',name='',purged_at=NOW() WHERE user_id=$1`, id); err != nil {
			return 0, err
		}
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return int64(len(ids)), nil
}

// RemindDue issues ONE replacement link after 18 hours. The prior link is
// invalidated. The replacement remains valid for 24 hours, but the 72-hour
// registration deadline is never extended. Safe for multiple running instances.
func (s Verification) RemindDue(ctx context.Context, max int) (int, error) {
	if max <= 0 || max > 50 {
		max = 20
	}
	count := 0
	for count < max {
		tx, err := s.DB.BeginTx(ctx, nil)
		if err != nil {
			return count, err
		}
		var id int64
		var name, email string
		err = tx.QueryRowContext(ctx, `SELECT user_id,name,email FROM worktic_email_activation_state
    WHERE activated_at IS NULL AND purged_at IS NULL
      AND created_at <= NOW()-INTERVAL '18 hours'
      AND purge_after > NOW()
      AND reminder_sent_at IS NULL
    ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1`).Scan(&id, &name, &email)
		if errors.Is(err, sql.ErrNoRows) {
			_ = tx.Rollback()
			return count, nil
		}
		if err != nil {
			_ = tx.Rollback()
			return count, err
		}
		token, digest, err := newToken()
		if err != nil {
			_ = tx.Rollback()
			return count, err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE worktic_email_activation_state SET reminder_sent_at=NOW() WHERE user_id=$1`, id); err != nil {
			_ = tx.Rollback()
			return count, err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE worktic_email_activation_tokens SET consumed_at=NOW() WHERE user_id=$1 AND consumed_at IS NULL`, id); err != nil {
			_ = tx.Rollback()
			return count, err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO worktic_email_activation_tokens(token_hash,user_id,expires_at) VALUES($1,$2,NOW()+INTERVAL '24 hours')`, digest, id); err != nil {
			_ = tx.Rollback()
			return count, err
		}
		if err = tx.Commit(); err != nil {
			return count, err
		}
		// Only one automatic reminder, best-effort. A user can request a new link.
		if err = s.SendReminder(ctx, name, email, token); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func (s Verification) SendReminder(ctx context.Context, name, email, token string) error {
	link := strings.TrimRight(s.BaseURL, "/") + "/login.html#activate=" + url.QueryEscape(token)
	title := "Tu cuenta Worktic AI sigue pendiente"
	body := `<!doctype html><html lang="es"><meta charset="utf-8"><body style="background:#f3f6fc;padding:32px;font-family:Arial,sans-serif;color:#142348"><div style="max-width:550px;margin:auto;background:#fff;border-radius:22px;padding:36px"><h2 style="color:#5544ef">Worktic AI</h2><h1>Activa tu cuenta</h1><p>Hola, ` + html.EscapeString(name) + `. Todavía no has confirmado tu correo.</p><p><a href="` + html.EscapeString(link) + `" style="display:inline-block;background:#5544ef;color:white;padding:15px 22px;border-radius:12px;text-decoration:none">Activar mi cuenta</a></p><p>Este enlace nuevo es válido durante 24 horas. El registro provisional se elimina si no se activa dentro de las 72 horas siguientes al registro inicial.</p></div></body></html>`
	return s.Sender.Send(ctx, email, title, body, "Activa tu cuenta Worktic AI: "+link+"\nEl registro provisional caduca a las 72 horas desde su creación.")
}

// Housekeeping keeps token/captcha tables bounded. Run periodically and at startup.
func (s Verification) CleanupAuxiliary(ctx context.Context) error {
	for _, q := range []string{
		`DELETE FROM worktic_captcha_challenges WHERE expires_at < NOW()-INTERVAL '1 hour' OR consumed_at < NOW()-INTERVAL '1 hour'`,
		`DELETE FROM worktic_email_activation_tokens WHERE (expires_at < NOW()-INTERVAL '2 days') OR (consumed_at < NOW()-INTERVAL '2 days')`,
	} {
		if _, err := s.DB.ExecContext(ctx, q); err != nil {
			return err
		}
	}
	return nil
}
