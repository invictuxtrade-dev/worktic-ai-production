package main

// B1 V27.36.13: Server-side Meta OAuth and Page authorization for Messenger.
// Feature remains disabled until Worktic explicitly configures the application
// webhook, Facebook Login for Business and a verified deployment in Render.

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const messengerSaaSWebhookPath = "/webhooks/meta/messenger/saas"
const messengerOAuthCallbackPath = "/integrations/meta/messenger/callback"
const messengerSelfServiceAPIPath = "/api/messenger/selfservice/"

func messengerSelfServiceEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("WORKTIC_MESSENGER_SELFSERVICE_ENABLED")), "true")
}
func messengerWebhookConfigured() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("WORKTIC_MESSENGER_APP_WEBHOOK_READY")), "true")
}
func messengerLoginConfigID() string {
	return strings.TrimSpace(os.Getenv("WORKTIC_META_MESSENGER_CONFIG_ID"))
}
func messengerSaaSVerifyToken() string {
	return strings.TrimSpace(os.Getenv("WORKTIC_MESSENGER_SAAS_VERIFY_TOKEN"))
}
func messengerOwnerOrAdmin(u *User) bool {
	return u != nil && (u.Role == "owner" || u.Role == "admin" || u.Role == "superadmin")
}

func initMessengerSelfServiceSchema(db *DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS messenger_oauth_states_v273613 (
 state_hash TEXT PRIMARY KEY, tenant_id INTEGER NOT NULL, user_id INTEGER NOT NULL,
 connection_id INTEGER NOT NULL, expires_at INTEGER NOT NULL, used_at INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS messenger_oauth_users_v273613 (
 tenant_id INTEGER NOT NULL, user_id INTEGER NOT NULL, connection_id INTEGER NOT NULL,
 encrypted_token TEXT NOT NULL, expires_at INTEGER NOT NULL, created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,user_id,connection_id)
);
CREATE INDEX IF NOT EXISTS idx_messenger_oauth_states_expires ON messenger_oauth_states_v273613(expires_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_messenger_connected_page_v273613 ON channel_connections(external_account_id) WHERE type='messenger' AND status='connected' AND external_account_id<>'';
`)
	return err
}

func messengerStateHash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func (a *App) messengerSelfServiceBase() (string, error) {
	base := strings.TrimRight(strings.TrimSpace(a.cfg.BaseURL), "/")
	if !strings.HasPrefix(base, "https://") {
		return "", errors.New("Configura BASE_URL con HTTPS antes de conectar Messenger")
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("BASE_URL público inválido")
	}
	return base, nil
}

func (a *App) messengerSelfServiceChannel(tid, id int64) (ChannelConnection, error) {
	var c ChannelConnection
	err := a.db.QueryRow(`SELECT id,tenant_id,public_id,type,name,status,external_account_id,assigned_agent_id,config_json,encrypted_credentials,last_connected_at,last_disconnected_at,last_message_at,last_error,created_at,updated_at FROM channel_connections WHERE tenant_id=? AND id=? AND type='messenger'`, tid, id).Scan(&c.ID, &c.TenantID, &c.PublicID, &c.Type, &c.Name, &c.Status, &c.ExternalAccountID, &c.AssignedAgentID, &c.ConfigJSON, &c.EncryptedCredentials, &c.LastConnectedAt, &c.LastDisconnectedAt, &c.LastMessageAt, &c.LastError, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return c, errors.New("canal Messenger no encontrado dentro de tu empresa")
	}
	return c, nil
}

func (a *App) messengerOAuthStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	if !messengerSelfServiceEnabled() {
		writeError(w, errors.New("conexión automática Messenger aún no habilitada"), 503)
		return
	}
	tid, u, err := a.tenantForRequest(r)
	if err != nil || !messengerOwnerOrAdmin(u) {
		writeError(w, errors.New("requiere administrador de empresa"), 403)
		return
	}
	if a.cfg.MetaAppID == "" || a.cfg.MetaAppSecret == "" || messengerLoginConfigID() == "" {
		writeError(w, errors.New("soporte Worktic debe configurar Facebook Login for Business"), 503)
		return
	}
	base, err := a.messengerSelfServiceBase()
	if err != nil {
		writeError(w, err, 503)
		return
	}
	var q struct {
		ConnectionID int64 `json:"connection_id"`
	}
	if json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&q) != nil || q.ConnectionID < 1 {
		writeError(w, errors.New("identificador de canal inválido"), 400)
		return
	}
	c, err := a.messengerSelfServiceChannel(tid, q.ConnectionID)
	if err != nil {
		writeError(w, err, 404)
		return
	}
	if c.Status == "connected" {
		writeError(w, errors.New("el canal ya está conectado; desconéctalo antes de cambiar página"), 409)
		return
	}
	state := randomToken(32)
	now := time.Now().UTC().Unix()
	// Retain only a SHA-256 digest. A used or expired state never authorizes a second callback.
	_, err = a.db.Exec(`INSERT INTO messenger_oauth_states_v273613(state_hash,tenant_id,user_id,connection_id,expires_at) VALUES(?,?,?,?,?)`, messengerStateHash(state), tid, u.ID, c.ID, now+600)
	if err != nil {
		writeError(w, errors.New("no se pudo crear autorización"), 500)
		return
	}
	redirect := base + messengerOAuthCallbackPath
	values := url.Values{
		"client_id": {a.cfg.MetaAppID}, "redirect_uri": {redirect}, "response_type": {"code"}, "state": {state},
		"config_id": {messengerLoginConfigID()},
	}
	writeJSON(w, map[string]any{"ok": true, "authorization_url": "https://www.facebook.com/" + a.messengerGraphVersion() + "/dialog/oauth?" + values.Encode()})
}

func (a *App) messengerOAuthExchange(ctx context.Context, code string) (string, int64, error) {
	base, err := a.messengerSelfServiceBase()
	if err != nil {
		return "", 0, err
	}
	values := url.Values{
		"client_id": {a.cfg.MetaAppID}, "client_secret": {a.cfg.MetaAppSecret},
		"redirect_uri": {base + messengerOAuthCallbackPath}, "code": {code},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://graph.facebook.com/"+a.messengerGraphVersion()+"/oauth/access_token?"+values.Encode(), nil)
	if err != nil {
		return "", 0, err
	}
	response, err := (&http.Client{Timeout: 25 * time.Second}).Do(req)
	if err != nil {
		return "", 0, err
	}
	defer response.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if response.StatusCode/100 != 2 {
		return "", 0, fmt.Errorf("Meta no autorizó el inicio de sesión: %s", messengerMetaErrorText(response.StatusCode, raw))
	}
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if json.Unmarshal(raw, &out) != nil || strings.TrimSpace(out.AccessToken) == "" {
		return "", 0, errors.New("Meta no devolvió un token válido")
	}
	expires := time.Now().UTC().Add(15 * time.Minute).Unix()
	if out.ExpiresIn > 0 && out.ExpiresIn < 900 {
		expires = time.Now().UTC().Add(time.Duration(out.ExpiresIn) * time.Second).Unix()
	}
	return out.AccessToken, expires, nil
}

func (a *App) messengerOAuthCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", 405)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	base, err := a.messengerSelfServiceBase()
	if err != nil {
		http.Error(w, "No disponible", 503)
		return
	}
	fail := func() { http.Redirect(w, r, base+"/app.html?messenger_oauth=error", http.StatusSeeOther) }
	if !messengerSelfServiceEnabled() {
		fail()
		return
	}
	state := strings.TrimSpace(r.URL.Query().Get("state"))
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if len(state) < 32 || len(state) > 200 || code == "" || len(code) > 4096 {
		fail()
		return
	}
	now := time.Now().UTC().Unix()
	var tenantID, userID, connectionID, expiresAt, usedAt int64
	err = a.db.QueryRow(`SELECT tenant_id,user_id,connection_id,expires_at,used_at FROM messenger_oauth_states_v273613 WHERE state_hash=?`, messengerStateHash(state)).Scan(&tenantID, &userID, &connectionID, &expiresAt, &usedAt)
	if err != nil || usedAt != 0 || expiresAt <= now {
		fail()
		return
	}
	res, err := a.db.Exec(`UPDATE messenger_oauth_states_v273613 SET used_at=? WHERE state_hash=? AND used_at=0 AND expires_at>?`, now, messengerStateHash(state), now)
	if err != nil {
		fail()
		return
	}
	n, err := res.RowsAffected()
	if err != nil || n != 1 {
		fail()
		return
	}
	if _, err = a.messengerSelfServiceChannel(tenantID, connectionID); err != nil {
		fail()
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	token, expires, err := a.messengerOAuthExchange(ctx, code)
	if err != nil {
		fail()
		return
	}
	// Store short-lived encrypted authorization for the same user, tenant and draft channel only.
	_, err = a.db.Exec(`INSERT INTO messenger_oauth_users_v273613(tenant_id,user_id,connection_id,encrypted_token,expires_at,created_at) VALUES(?,?,?,?,?,?) ON CONFLICT(tenant_id,user_id,connection_id) DO UPDATE SET encrypted_token=excluded.encrypted_token,expires_at=excluded.expires_at,created_at=excluded.created_at`, tenantID, userID, connectionID, encryptLocal(token, a.cfg.ChannelEncryptionKey), expires, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		fail()
		return
	}
	http.Redirect(w, r, base+"/app.html?messenger_oauth=ready&connection_id="+strconv.FormatInt(connectionID, 10), http.StatusSeeOther)
}

type messengerOAuthPage struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Tasks       []string `json:"tasks"`
	AccessToken string   `json:"access_token"`
}
type messengerOAuthPageResult struct {
	Data   []messengerOAuthPage `json:"data"`
	Paging struct {
		Next string `json:"next"`
	} `json:"paging"`
}

func messengerCanManagePage(p messengerOAuthPage) bool {
	if p.ID == "" || p.AccessToken == "" {
		return false
	}
	hasMessage, hasManage := false, false
	for _, task := range p.Tasks {
		switch task {
		case "MESSAGING":
			hasMessage = true
		case "MANAGE":
			hasManage = true
		}
	}
	return hasMessage && hasManage
}

func (a *App) messengerSelfServiceAuth(tid, userID, connectionID int64) (string, error) {
	var enc string
	var expires int64
	err := a.db.QueryRow(`SELECT encrypted_token,expires_at FROM messenger_oauth_users_v273613 WHERE tenant_id=? AND user_id=? AND connection_id=?`, tid, userID, connectionID).Scan(&enc, &expires)
	if err != nil || expires <= time.Now().UTC().Unix() {
		return "", errors.New("autorización vencida; vuelve a conectar Facebook")
	}
	token := decryptLocal(enc, a.cfg.ChannelEncryptionKey)
	if token == "" {
		return "", errors.New("autorización no disponible")
	}
	return token, nil
}
func (a *App) messengerSelfServicePages(token string) ([]messengerOAuthPage, error) {
	// Bound list; never fetch arbitrary paging URLs or trust a client-selected Page token.
	var out messengerOAuthPageResult
	err := a.messengerGraph(http.MethodGet, "/me/accounts", token, url.Values{"fields": {"id,name,tasks,access_token"}, "limit": {"100"}}, &out)
	if err != nil {
		return nil, err
	}
	pages := make([]messengerOAuthPage, 0, len(out.Data))
	for _, page := range out.Data {
		if messengerCanManagePage(page) {
			pages = append(pages, page)
		}
	}
	return pages, nil
}

func (a *App) messengerSelfServiceHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !messengerSelfServiceEnabled() {
		writeError(w, errors.New("Messenger está en preparación"), 503)
		return
	}
	tid, u, err := a.tenantForRequest(r)
	if err != nil || !messengerOwnerOrAdmin(u) {
		writeError(w, errors.New("se necesita un administrador de la empresa"), 403)
		return
	}
	suffix := strings.TrimPrefix(r.URL.Path, messengerSelfServiceAPIPath)
	switch suffix {
	case "start":
		a.messengerOAuthStart(w, r)
		return
	case "pages":
		if r.Method != http.MethodGet {
			http.Error(w, "Método no permitido", 405)
			return
		}
		id, err := strconv.ParseInt(r.URL.Query().Get("connection_id"), 10, 64)
		if err != nil || id < 1 {
			writeError(w, errors.New("canal inválido"), 400)
			return
		}
		if _, err = a.messengerSelfServiceChannel(tid, id); err != nil {
			writeError(w, err, 404)
			return
		}
		token, err := a.messengerSelfServiceAuth(tid, u.ID, id)
		if err != nil {
			writeError(w, err, 403)
			return
		}
		pages, err := a.messengerSelfServicePages(token)
		if err != nil {
			writeError(w, err, 502)
			return
		}
		safe := make([]map[string]any, 0, len(pages))
		for _, p := range pages {
			safe = append(safe, map[string]any{"id": p.ID, "name": p.Name, "tasks": p.Tasks})
		}
		writeJSON(w, map[string]any{"ok": true, "pages": safe})
		return
	case "activate":
		if r.Method != http.MethodPost {
			http.Error(w, "Método no permitido", 405)
			return
		}
		if !messengerWebhookConfigured() || len(messengerSaaSVerifyToken()) < 32 {
			writeError(w, errors.New("soporte Worktic debe verificar el webhook de Messenger antes de activar páginas"), 503)
			return
		}
		var q struct {
			ConnectionID int64  `json:"connection_id"`
			PageID       string `json:"page_id"`
			AgentID      int64  `json:"agent_id"`
		}
		if json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&q) != nil || q.ConnectionID < 1 || q.PageID == "" {
			writeError(w, errors.New("datos incompletos"), 400)
			return
		}
		if _, err := strconv.ParseInt(q.PageID, 10, 64); err != nil {
			writeError(w, errors.New("Page ID inválido"), 400)
			return
		}
		c, err := a.messengerSelfServiceChannel(tid, q.ConnectionID)
		if err != nil {
			writeError(w, err, 404)
			return
		}
		if c.Status == "connected" {
			writeError(w, errors.New("el canal ya está conectado"), 409)
			return
		}
		if q.AgentID > 0 {
			if err = a.securityTenantAgentActive(tid, q.AgentID); err != nil {
				writeError(w, err, 403)
				return
			}
		}
		var already int
		if err = a.db.QueryRow(`SELECT COUNT(*) FROM channel_connections WHERE type='messenger' AND external_account_id=? AND status='connected' AND id<>?`, q.PageID, c.ID).Scan(&already); err != nil {
			writeError(w, err, 500)
			return
		}
		if already > 0 {
			writeError(w, errors.New("esta página ya está vinculada a otra conexión"), 409)
			return
		}
		shortToken, err := a.messengerSelfServiceAuth(tid, u.ID, c.ID)
		if err != nil {
			writeError(w, err, 403)
			return
		}
		pages, err := a.messengerSelfServicePages(shortToken)
		if err != nil {
			writeError(w, err, 502)
			return
		}
		match := false
		for _, p := range pages {
			if p.ID == q.PageID && messengerCanManagePage(p) {
				match = true
				break
			}
		}
		if !match {
			writeError(w, errors.New("esa página no fue autorizada para esta cuenta"), 403)
			return
		}
		// Uses established encrypted long-token storage and existing token checks.
		_, err = a.persistMessengerLongToken(r, c, shortToken, q.PageID, a.cfg.MetaAppID, a.cfg.MetaAppSecret)
		if err != nil {
			writeError(w, err, 502)
			return
		}
		// Prevent false connected state when Meta subscription cannot be completed.
		fields := "messages,messaging_postbacks,message_deliveries,message_reads"
		var subscribed struct {
			Success bool `json:"success"`
		}
		fresh, err := a.messengerSelfServiceChannel(tid, c.ID)
		if err == nil {
			cred, ce := a.messengerCredentialsFor(fresh)
			if ce != nil {
				err = ce
			} else {
				err = a.messengerGraph(http.MethodPost, "/"+url.PathEscape(q.PageID)+"/subscribed_apps", cred.PageToken, url.Values{"subscribed_fields": {fields}}, &subscribed)
			}
		}
		if err != nil || !subscribed.Success {
			// Deliberately fail closed. Existing token remains encrypted for secure retry.
			_, _ = a.db.Exec(`UPDATE channel_connections SET status='draft',last_error=?,updated_at=? WHERE id=? AND tenant_id=?`, "La suscripción a Meta no quedó confirmada", time.Now().UTC().Format(time.RFC3339Nano), c.ID, tid)
			writeError(w, errors.New("Meta no confirmó la suscripción a eventos; revisa el webhook de la aplicación"), 502)
			return
		}
		base, _ := a.messengerSelfServiceBase()
		var cfg map[string]any
		_ = json.Unmarshal([]byte(fresh.ConfigJSON), &cfg)
		if cfg == nil {
			cfg = map[string]any{}
		}
		cfg["webhook_url"] = base + messengerSaaSWebhookPath
		cfg["webhook_mode"] = "app_shared_page_id"
		cfg["subscribed_fields"] = strings.Split(fields, ",")
		configJSON, _ := json.Marshal(cfg)
		now := time.Now().UTC().Format(time.RFC3339Nano)
		_, err = a.db.Exec(`UPDATE channel_connections SET config_json=?,status='connected',assigned_agent_id=?,last_error='',updated_at=? WHERE id=? AND tenant_id=?`, string(configJSON), q.AgentID, now, c.ID, tid)
		if err != nil {
			writeError(w, errors.New("no se pudo guardar la página"), 500)
			return
		}
		_, _ = a.db.Exec(`DELETE FROM messenger_oauth_users_v273613 WHERE tenant_id=? AND user_id=? AND connection_id=?`, tid, u.ID, c.ID)
		_, _ = a.db.Exec(`INSERT INTO channel_audit(tenant_id,connection_id,user_id,action,detail,created_at) VALUES(?,?,?,?,?,?)`, tid, c.ID, u.ID, "messenger_selfservice_connected", q.PageID, now)
		writeJSON(w, map[string]any{"ok": true, "page_id": q.PageID, "status": "connected", "webhook_subscribed": true})
		return
	default:
		http.NotFound(w, r)
	}
}

// A single callback is registered once at Meta app-level. Its authenticated
// entries are split by page_id before they reach the existing per-tenant handler.
// An unknown or duplicated Page ID never routes to a tenant.
func messengerSplitPageEntries(raw []byte) (map[string][][]byte, error) {
	var root struct {
		Object string            `json:"object"`
		Entry  []json.RawMessage `json:"entry"`
	}
	if json.Unmarshal(raw, &root) != nil || root.Object != "page" {
		return nil, errors.New("no es un evento de páginas de Meta")
	}
	out := map[string][][]byte{}
	for _, entry := range root.Entry {
		var e struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(entry, &e) != nil || e.ID == "" {
			return nil, errors.New("evento sin ID de página")
		}
		if _, err := strconv.ParseInt(e.ID, 10, 64); err != nil {
			return nil, errors.New("ID de página inválido")
		}
		out[e.ID] = append(out[e.ID], append([]byte(nil), entry...))
	}
	return out, nil
}
func messengerSignedBody(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
func (a *App) messengerSaaSWebhookHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !messengerSelfServiceEnabled() || len(messengerSaaSVerifyToken()) < 32 || a.cfg.MetaAppSecret == "" {
		http.Error(w, "no disponible", 503)
		return
	}
	if r.Method == http.MethodGet {
		if r.URL.Query().Get("hub.mode") == "subscribe" && hmac.Equal([]byte(r.URL.Query().Get("hub.verify_token")), []byte(messengerSaaSVerifyToken())) && r.URL.Query().Get("hub.challenge") != "" {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = w.Write([]byte(r.URL.Query().Get("hub.challenge")))
			return
		}
		http.Error(w, "verificación incorrecta", 403)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	if !messengerWebhookConfigured() {
		http.Error(w, "webhook todavía no activado", http.StatusServiceUnavailable)
		return
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 2<<20))
	if err != nil {
		http.Error(w, "cuerpo inválido", 400)
		return
	}
	received := r.Header.Get("X-Hub-Signature-256")
	expected := messengerSignedBody(a.cfg.MetaAppSecret, raw)
	if !hmac.Equal([]byte(received), []byte(expected)) {
		http.Error(w, "firma incorrecta", 403)
		return
	}
	pages, err := messengerSplitPageEntries(raw)
	if err != nil {
		http.Error(w, "evento inválido", 400)
		return
	}
	failedDispatch := false
	for pageID, entries := range pages {
		var c ChannelConnection
		var count int
		// Hard failure on duplicate claims avoids routing messages across tenants.
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM channel_connections WHERE type='messenger' AND status='connected' AND external_account_id=?`, pageID).Scan(&count)
		if count != 1 {
			continue
		}
		err = a.db.QueryRow(`SELECT id,tenant_id,public_id,type,name,status,external_account_id,assigned_agent_id,config_json,encrypted_credentials,last_connected_at,last_disconnected_at,last_message_at,last_error,created_at,updated_at FROM channel_connections WHERE type='messenger' AND status='connected' AND external_account_id=?`, pageID).Scan(&c.ID, &c.TenantID, &c.PublicID, &c.Type, &c.Name, &c.Status, &c.ExternalAccountID, &c.AssignedAgentID, &c.ConfigJSON, &c.EncryptedCredentials, &c.LastConnectedAt, &c.LastDisconnectedAt, &c.LastMessageAt, &c.LastError, &c.CreatedAt, &c.UpdatedAt)
		if err != nil || c.PublicID == "" {
			continue
		}
		var cfg map[string]any
		_ = json.Unmarshal([]byte(c.ConfigJSON), &cfg)
		if cfg["webhook_mode"] != "app_shared_page_id" {
			continue
		}
		packet, _ := json.Marshal(map[string]any{"object": "page", "entry": entries})
		// Re-sign only AFTER the original Meta signature is verified and the
		// entry is isolated to this Page. Reuse the mature tenant pipeline.
		subreq := httptest.NewRequest(http.MethodPost, "https://internal.invalid/webhooks/messenger/"+url.PathEscape(c.PublicID), bytes.NewReader(packet))
		subreq.Header.Set("X-Hub-Signature-256", messengerSignedBody(a.cfg.MetaAppSecret, packet))
		subreq.Header.Set("Content-Type", "application/json")
		sink := httptest.NewRecorder()
		a.messengerTenantWebhookHandler(sink, subreq)
		if sink.Code != 200 {
			failedDispatch = true
			a.recordMessengerChannelEvent(c, "messenger_saas_dispatch", "error", map[string]any{"status": sink.Code})
		}
	}
	if failedDispatch {
		http.Error(w, "retry later", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("EVENT_RECEIVED"))
}
