package main

// Email IA P1D.3: alta multiempresa mediante Google OAuth existente.
// La conexión OAuth NO da permiso para sincronizar hasta que el administrador
// autorice explícitamente cada buzón. Los envíos automáticos P1D.2 siguen
// estrictamente limitados al piloto anterior; esta función no los amplía.
import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func emailP1D3Open() bool { return emailP1Flag("WORKTIC_EMAIL_P1D3_SELF_SERVICE_ENABLED") }

func emailP1D3Schema(db *DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS email_p1d3_accounts (
        tenant_id BIGINT NOT NULL, account_id BIGINT NOT NULL,
        sync_enabled INTEGER NOT NULL DEFAULT 0, enabled_at TEXT NOT NULL DEFAULT '',
        updated_by BIGINT NOT NULL DEFAULT 0, updated_at TEXT NOT NULL,
        PRIMARY KEY (tenant_id, account_id))`)
	return err
}

// Nunca se consulta un permiso sin verificar tenant, cuenta y proveedor.
func (a *App) emailP1D3AccountEnabled(tenant, account int64) bool {
	if !emailP1D3Open() || tenant <= 0 || account <= 0 {
		return false
	}
	var enabled int
	err := a.db.QueryRow(`SELECT p.sync_enabled FROM email_p1d3_accounts p
       JOIN email_p1_accounts a ON a.tenant_id=p.tenant_id AND a.id=p.account_id
       WHERE p.tenant_id=? AND p.account_id=? AND a.provider='google' AND a.status='connected'`, tenant, account).Scan(&enabled)
	return err == nil && enabled == 1
}

func (a *App) emailP1D3SyncAllowed(tenant, account int64, provider, address string) bool {
	if provider != "google" {
		return false
	}
	if emailP1BAllowedAccount(account, address) {
		return true
	} // P1B, sin cambios.
	return emailP1D3Open() && a.emailP1D3AccountEnabled(tenant, account)
}
func (a *App) emailP1C1AccountEnabled(tenant, account int64, address string) bool {
	return emailP1Flag("WORKTIC_EMAIL_P1C1_ENABLED") && emailP1Flag("WORKTIC_EMAIL_P1B_FAST_SYNC_ENABLED") && a.emailP1D3SyncAllowed(tenant, account, "google", address)
}
func (a *App) emailP1C2AccountEnabled(tenant, account int64, address string) bool {
	return emailP1Flag("WORKTIC_EMAIL_P1C2_ENABLED") && a.emailP1C1AccountEnabled(tenant, account, address)
}
func (a *App) emailP1D1AccountEnabled(tenant, account int64, address string) bool {
	return emailP1Flag("WORKTIC_EMAIL_P1D1_ENABLED") && a.emailP1C2AccountEnabled(tenant, account, address)
}

// El endpoint sólo cambia el buzón del tenant autenticado; no altera tokens,
// OAuth, cuentas de otras empresas, cuotas ni reglas de envío.
func (a *App) emailP1D3SyncSettings(w http.ResponseWriter, r *http.Request) {
	tenant, ok := a.emailP1Tenant(w, r)
	if !ok {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	if !a.emailP1CanAdmin(r) || !emailP1Origin(r) {
		writeError(w, errors.New("operación reservada a administradores"), 403)
		return
	}
	if !emailP1D3Open() || !emailP1Flag("WORKTIC_EMAIL_P1B_FAST_SYNC_ENABLED") || !emailP1Flag("WORKTIC_EMAIL_P1_SYNC_ENABLED") {
		writeError(w, errors.New("sincronización multiempresa no habilitada"), 403)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 2048)
	var input struct {
		AccountID int64 `json:"account_id"`
		Enabled   bool  `json:"enabled"`
	}
	if json.NewDecoder(r.Body).Decode(&input) != nil || input.AccountID <= 0 {
		writeError(w, errors.New("cuenta inválida"), 400)
		return
	}
	var provider, status, address string
	if err := a.db.QueryRow(`SELECT provider,status,email FROM email_p1_accounts WHERE tenant_id=? AND id=?`, tenant, input.AccountID).Scan(&provider, &status, &address); err != nil || provider != "google" || status != "connected" {
		writeError(w, errors.New("buzón Gmail no conectado para esta empresa"), 404)
		return
	}
	// El piloto histórico se administra por su configuración anterior; no
	// permitir aparentar pausarlo con una preferencia que no lo detendría.
	if emailP1BAllowedAccount(input.AccountID, address) {
		writeError(w, errors.New("el buzón piloto conserva su configuración administrada"), 409)
		return
	}
	if err := emailP1D3Schema(a.db); err != nil {
		writeError(w, errors.New("almacenamiento no disponible"), 503)
		return
	}
	enabled := 0
	if input.Enabled {
		enabled = 1
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	user := a.currentUser(r)
	if user == nil {
		writeError(w, errors.New("sesión vencida"), 401)
		return
	}
	_, err := a.db.Exec(`INSERT INTO email_p1d3_accounts(tenant_id,account_id,sync_enabled,enabled_at,updated_by,updated_at) VALUES(?,?,?,?,?,?)
        ON CONFLICT(tenant_id,account_id) DO UPDATE SET sync_enabled=excluded.sync_enabled,enabled_at=excluded.enabled_at,updated_by=excluded.updated_by,updated_at=excluded.updated_at`, tenant, input.AccountID, enabled, now, user.ID, now)
	if err != nil {
		writeError(w, errors.New("no se pudo guardar"), 503)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "account_id": input.AccountID, "sync_enabled": input.Enabled, "automatic_send_enabled": false})
}

// El horario de activación evita procesar retrospectivamente mensajes de otros días.
func (a *App) emailP1D3StartTime(tenant, account int64) (time.Time, bool) {
	if !a.emailP1D3AccountEnabled(tenant, account) {
		return time.Time{}, false
	}
	var at string
	if a.db.QueryRow(`SELECT enabled_at FROM email_p1d3_accounts WHERE tenant_id=? AND account_id=? AND sync_enabled=1`, tenant, account).Scan(&at) != nil {
		return time.Time{}, false
	}
	when, err := time.Parse(time.RFC3339Nano, at)
	return when, err == nil
}
func emailP1D3NewMessage(received string, activation time.Time) bool {
	when, err := time.Parse(time.RFC3339Nano, received)
	return err == nil && !when.Before(activation)
}

func emailP1D3ValidateAccountID(raw string) bool {
	id, e := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	return e == nil && id > 0
}
