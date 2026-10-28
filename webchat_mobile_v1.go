package main

// WorkticAI Web Chat pilot: isolated tenant-scoped conversations for the Inbox.
// Disabled by default; requires WORKTIC_WEBCHAT_ENABLED=true and staging QA.
// AI answers are opt-in and use existing commercial metering and Inbox handoff.
import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var webChatRate = struct {
	sync.Mutex
	visits map[string]time.Time
}{visits: make(map[string]time.Time)}

func webChatOn() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("WORKTIC_WEBCHAT_ENABLED")), "true")
}
func initWebChatMobileV1(db *DB) error {
	_, err := db.Exec(`
 CREATE TABLE IF NOT EXISTS mobile_web_channels_v1(id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL,public_key TEXT NOT NULL UNIQUE,name TEXT NOT NULL,origin TEXT NOT NULL,agent_id INTEGER NOT NULL,enabled INTEGER NOT NULL DEFAULT 0,auto_ai INTEGER NOT NULL DEFAULT 0,created_at TEXT NOT NULL);
 CREATE INDEX IF NOT EXISTS idx_mobile_web_channels_tenant ON mobile_web_channels_v1(tenant_id);
 CREATE TABLE IF NOT EXISTS mobile_web_sessions_v1(id INTEGER PRIMARY KEY AUTOINCREMENT,tenant_id INTEGER NOT NULL,channel_id INTEGER NOT NULL,chat_jid TEXT NOT NULL UNIQUE,token_hash TEXT NOT NULL UNIQUE,name TEXT NOT NULL,created_at TEXT NOT NULL);
 CREATE INDEX IF NOT EXISTS idx_mobile_web_sessions_tenant ON mobile_web_sessions_v1(tenant_id,channel_id);
 `)
	if err != nil {
		return err
	}
	_, _ = db.Exec(`ALTER TABLE mobile_web_channels_v1 ADD COLUMN auto_ai INTEGER NOT NULL DEFAULT 0`)
	return nil
}
func webHash(s string) string { x := sha256.Sum256([]byte(s)); return hex.EncodeToString(x[:]) }
func webRateLimit(key string) bool {
	webChatRate.Lock()
	defer webChatRate.Unlock()
	now := time.Now()
	if last, ok := webChatRate.visits[key]; ok && now.Sub(last) < 2*time.Second {
		return false
	}
	webChatRate.visits[key] = now
	if len(webChatRate.visits) > 15000 {
		for k, v := range webChatRate.visits {
			if now.Sub(v) > 10*time.Minute {
				delete(webChatRate.visits, k)
			}
		}
	}
	return true
}
func (a *App) webChatPublicGuard(w http.ResponseWriter, r *http.Request) (int64, int64, bool) {
	if !webChatOn() {
		http.Error(w, "Chat web deshabilitado", 503)
		return 0, 0, false
	}
	key := strings.TrimSpace(r.URL.Query().Get("key"))
	origin := r.Header.Get("Origin")
	if len(key) < 20 || len(key) > 100 || origin == "" {
		http.Error(w, "Origen/canal inválido", 403)
		return 0, 0, false
	}
	var tenant, channel int64
	var allowed string
	var enabled int
	err := a.db.QueryRow(`SELECT tenant_id,id,origin,enabled FROM mobile_web_channels_v1 WHERE public_key=?`, key).Scan(&tenant, &channel, &allowed, &enabled)
	if err != nil || enabled != 1 || origin != allowed {
		http.Error(w, "Canal no autorizado", 403)
		return 0, 0, false
	}
	w.Header().Set("Access-Control-Allow-Origin", allowed)
	w.Header().Set("Vary", "Origin")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Worktic-Session")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Method == http.MethodOptions {
		w.WriteHeader(204)
		return 0, 0, false
	}
	return tenant, channel, true
}
func webReadJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		http.Error(w, "Datos inválidos", 400)
		return false
	}
	return true
}

// Configuration is available only to authenticated tenant staff; write methods
// are restricted to tenant owners/admins. This endpoint manages the Web channel
// without altering WhatsApp or Messenger credentials and runtimes.
func (a *App) webChatChannelsV1(w http.ResponseWriter, r *http.Request) {
	if !webChatOn() {
		writeError(w, errors.New("Chat Web deshabilitado. Activa WORKTIC_WEBCHAT_ENABLED tras QA"), http.StatusServiceUnavailable)
		return
	}
	tenant, u, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, http.StatusUnauthorized)
		return
	}
	if r.Method == http.MethodGet {
		rows, e := a.db.Query(`SELECT id,public_key,name,origin,agent_id,enabled,auto_ai FROM mobile_web_channels_v1 WHERE tenant_id=? ORDER BY id DESC`, tenant)
		if e != nil {
			writeError(w, e, 500)
			return
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var id, agent int64
			var key, name, origin string
			var enabled, ai int
			if rows.Scan(&id, &key, &name, &origin, &agent, &enabled, &ai) == nil {
				out = append(out, map[string]any{"id": id, "public_key": key, "name": name, "origin": origin, "agent_id": agent, "enabled": enabled == 1, "auto_ai": ai == 1})
			}
		}
		writeJSON(w, map[string]any{"channels": out, "feature_enabled": true, "ai_available": webChatAIOn() && a.openAIKey() != ""})
		return
	}
	if u.Role != "owner" && u.Role != "admin" && u.Role != "superadmin" {
		writeError(w, errors.New("No tienes permiso para configurar canales"), 403)
		return
	}
	if r.Method == http.MethodDelete {
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		if id <= 0 {
			writeError(w, errors.New("Canal inválido"), 400)
			return
		}
		res, e := a.db.Exec(`UPDATE mobile_web_channels_v1 SET enabled=0,auto_ai=0 WHERE id=? AND tenant_id=?`, id, tenant)
		if e != nil {
			writeError(w, e, 500)
			return
		}
		count, _ := res.RowsAffected()
		if count == 0 {
			writeError(w, errors.New("Canal no encontrado"), 404)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "status": "disabled"})
		return
	}
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		http.Error(w, "Método no permitido", 405)
		return
	}
	var q struct {
		ID      int64  `json:"id"`
		Name    string `json:"name"`
		Origin  string `json:"origin"`
		Agent   int64  `json:"agent_id"`
		Enabled bool   `json:"enabled"`
		AutoAI  bool   `json:"auto_ai"`
	}
	if !webReadJSON(w, r, &q) {
		return
	}
	q.Name = strings.TrimSpace(q.Name)
	q.Origin = strings.TrimSuffix(strings.TrimSpace(q.Origin), "/")
	uri, e := url.Parse(q.Origin)
	if e != nil || uri.Scheme != "https" || uri.Hostname() == "" || uri.User != nil || uri.Path != "" || uri.RawQuery != "" || uri.Fragment != "" || len(q.Name) < 3 || len(q.Name) > 80 || strings.Contains(uri.Host, "@") || uri.Port() != "" {
		writeError(w, errors.New("Nombre o dominio HTTPS inválido. Ejemplo: https://miempresa.com"), 400)
		return
	}
	if q.Agent <= 0 {
		writeError(w, errors.New("Selecciona un agente IA activo"), 400)
		return
	}
	var status string
	if a.db.QueryRow(`SELECT status FROM ai_agents WHERE id=? AND tenant_id=?`, q.Agent, tenant).Scan(&status) != nil || status != "active" {
		writeError(w, errors.New("Agente no disponible en esta empresa"), 400)
		return
	}
	if q.AutoAI && (!webChatAIOn() || a.openAIKey() == "") {
		writeError(w, errors.New("La respuesta IA automática aún no está habilitada por el administrador"), 403)
		return
	}
	enabled, ai := 0, 0
	if q.Enabled {
		enabled = 1
	}
	if q.AutoAI {
		ai = 1
	}
	if r.Method == http.MethodPut {
		if q.ID <= 0 {
			writeError(w, errors.New("ID de canal requerido"), 400)
			return
		}
		res, e := a.db.Exec(`UPDATE mobile_web_channels_v1 SET name=?,origin=?,agent_id=?,enabled=?,auto_ai=? WHERE id=? AND tenant_id=?`, q.Name, q.Origin, q.Agent, enabled, ai, q.ID, tenant)
		if e != nil {
			writeError(w, e, 500)
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			writeError(w, errors.New("Canal no encontrado"), 404)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "id": q.ID, "status": "updated"})
		return
	}
	// The total Web + other channel count is checked inside a transaction.
	// In hardening mode we take the same tenant lock as the other channel APIs.
	p, _, planErr := a.activePlan(a.billingAccountUserID(u))
	if tenantHardeningV2730Enabled() {
		p, planErr = a.tenantEffectivePlanV2730(tenant)
	}
	if planErr != nil {
		writeError(w, errors.New("No se pudo verificar el plan comercial"), 503)
		return
	}
	tx, e := a.db.Begin()
	if e != nil {
		writeError(w, e, 500)
		return
	}
	defer tx.Rollback()
	if tenantHardeningV2730Enabled() {
		if e = lockConsumptionTenantV2723(tx, a.db.IsPostgres(), tenant); e != nil {
			writeError(w, e, 500)
			return
		}
	}
	var existing, total int
	if e = tx.QueryRow(`SELECT COUNT(*) FROM mobile_web_channels_v1 WHERE tenant_id=?`, tenant).Scan(&existing); e != nil {
		writeError(w, e, 500)
		return
	}
	if e = tx.QueryRow(`SELECT COUNT(*) FROM channel_connections WHERE tenant_id=? AND status NOT IN ('revoked','deleted')`, tenant).Scan(&total); e != nil {
		writeError(w, e, 500)
		return
	}
	if existing >= 2 || (u.Role != "superadmin" && total+existing >= p.MaxChannels) {
		writeError(w, fmt.Errorf("Límite de canales del plan alcanzado"), 403)
		return
	}
	key := randomToken(24)
	now := time.Now().UTC().Format(time.RFC3339)
	res, e := tx.Exec(`INSERT INTO mobile_web_channels_v1(tenant_id,public_key,name,origin,agent_id,enabled,auto_ai,created_at) VALUES(?,?,?,?,?,?,?,?)`, tenant, key, q.Name, q.Origin, q.Agent, enabled, ai, now)
	if e != nil {
		writeError(w, e, 500)
		return
	}
	id, e := res.LastInsertId()
	if e != nil {
		writeError(w, e, 500)
		return
	}
	if e = tx.Commit(); e != nil {
		writeError(w, e, 500)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "id": id, "public_key": key, "status": "created"})
}
func webChatAIOn() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("WORKTIC_WEBCHAT_AUTO_AI")), "true")
}
func (a *App) webChatStartV1(w http.ResponseWriter, r *http.Request) {
	tenant, channel, ok := a.webChatPublicGuard(w, r)
	if !ok {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	remote, _, _ := net.SplitHostPort(r.RemoteAddr)
	if !webRateLimit(remote + "/" + strWebChannel(channel)) {
		http.Error(w, "Espera antes de continuar", 429)
		return
	}
	var q struct {
		Name    string `json:"name"`
		Consent bool   `json:"consent"`
	}
	if !webReadJSON(w, r, &q) {
		return
	}
	q.Name = strings.TrimSpace(q.Name)
	if len(q.Name) < 2 || len(q.Name) > 100 || !q.Consent {
		http.Error(w, "Nombre y aceptación de privacidad requeridos", 400)
		return
	}
	plan, planErr := a.tenantPlanV2723(tenant)
	if tenantHardeningV2730Enabled() {
		plan, planErr = a.tenantEffectivePlanV2730(tenant)
	}
	if planErr != nil {
		writeError(w, errors.New("No se pudo verificar el límite de contactos de la empresa"), 503)
		return
	}
	token := randomToken(32)
	chat := "webchat:" + randomToken(18)
	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := a.db.Begin()
	if err != nil {
		writeError(w, err, 500)
		return
	}
	defer tx.Rollback()
	if tenantHardeningV2730Enabled() {
		if err = lockConsumptionTenantV2723(tx, a.db.IsPostgres(), tenant); err != nil {
			writeError(w, err, 500)
			return
		}
	}
	if _, err = tx.Exec(`INSERT INTO mobile_web_sessions_v1(tenant_id,channel_id,chat_jid,token_hash,name,created_at) VALUES(?,?,?,?,?,?)`, tenant, channel, chat, webHash(token), q.Name, now); err != nil {
		writeError(w, err, 500)
		return
	}
	if _, err = tx.Exec(`INSERT INTO worktic_contacts(tenant_id,chat_jid,phone,name,unread,updated_at,channel) VALUES(?,?,?,?,?,?,?)`, tenant, chat, "", q.Name, 0, now, "web"); err != nil {
		writeError(w, err, 500)
		return
	}
	var crmCount int
	if err = tx.QueryRow(`SELECT COUNT(*) FROM crm_contacts WHERE tenant_id=? AND COALESCE(deleted_at,'')=''`, tenant).Scan(&crmCount); err != nil {
		writeError(w, err, 500)
		return
	}
	if plan.MaxContacts <= 0 || crmCount >= plan.MaxContacts {
		writeError(w, errors.New("Límite de contactos Web alcanzado"), 429)
		return
	}
	if _, err = tx.Exec(`INSERT INTO crm_contacts(tenant_id,name,phone,email,channel,stage,tags,notes,updated_at,source,external_id,first_seen_at,last_activity_at,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, tenant, q.Name, "", "", "web", "Nuevo", "", "", now, "chat_web", chat, now, now, now); err != nil {
		writeError(w, err, 500)
		return
	}
	if err = tx.Commit(); err != nil {
		writeError(w, err, 500)
		return
	}
	a.ensureInboxConversation(tenant, chat)
	var autoAI int
	_ = a.db.QueryRow(`SELECT auto_ai FROM mobile_web_channels_v1 WHERE tenant_id=? AND id=?`, tenant, channel).Scan(&autoAI)
	if !webChatAIOn() || autoAI != 1 {
		_, _ = a.db.Exec(`UPDATE inbox_conversations SET mode='human' WHERE tenant_id=? AND chat_jid=?`, tenant, chat)
	}
	writeJSON(w, map[string]any{"session": token})
}
func strWebChannel(id int64) string { return strconv.FormatInt(id, 10) }
func (a *App) webChatSession(w http.ResponseWriter, r *http.Request, tenant, channel int64) (string, bool) {
	token := r.Header.Get("X-Worktic-Session")
	if len(token) < 20 || len(token) > 150 {
		http.Error(w, "Sesión inválida", 401)
		return "", false
	}
	var chat string
	err := a.db.QueryRow(`SELECT chat_jid FROM mobile_web_sessions_v1 WHERE token_hash=? AND tenant_id=? AND channel_id=? AND created_at>?`, webHash(token), tenant, channel, time.Now().UTC().Add(-7*24*time.Hour).Format(time.RFC3339)).Scan(&chat)
	if err != nil {
		http.Error(w, "Sesión no encontrada", 401)
		return "", false
	}
	return chat, true
}
func (a *App) webChatMessageV1(w http.ResponseWriter, r *http.Request) {
	tenant, channel, ok := a.webChatPublicGuard(w, r)
	if !ok {
		return
	}
	chat, ok := a.webChatSession(w, r, tenant, channel)
	if !ok {
		return
	}
	if r.Method == http.MethodGet {
		rows, e := a.db.Query(`SELECT direction,text,timestamp FROM worktic_messages WHERE tenant_id=? AND chat_jid=? ORDER BY id DESC LIMIT 80`, tenant, chat)
		if e != nil {
			writeError(w, e, 500)
			return
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var direction, text, at string
			if rows.Scan(&direction, &text, &at) == nil {
				out = append(out, map[string]any{"direction": direction, "text": text, "timestamp": at})
			}
		}
		writeJSON(w, map[string]any{"messages": out})
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	if !webRateLimit(chat) {
		http.Error(w, "Envía los mensajes más despacio", 429)
		return
	}
	var q struct {
		Text string `json:"text"`
	}
	if !webReadJSON(w, r, &q) {
		return
	}
	q.Text = strings.TrimSpace(q.Text)
	if len(q.Text) == 0 || len(q.Text) > 1600 {
		http.Error(w, "Mensaje inválido", 400)
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := a.db.Exec(`INSERT INTO worktic_messages(tenant_id,channel,wa_id,chat_jid,sender_jid,direction,message_type,text,status,timestamp) VALUES(?,?,?,?,?,'in','text',?,'received',?)`, tenant, "web", "", chat, chat, q.Text, now)
	if err != nil {
		writeError(w, err, 500)
		return
	}
	_, _ = a.db.Exec(`UPDATE worktic_contacts SET unread=unread+1,updated_at=? WHERE tenant_id=? AND chat_jid=?`, now, tenant, chat)
	_, _ = a.db.Exec(`UPDATE crm_contacts SET last_activity_at=?,updated_at=? WHERE tenant_id=? AND external_id=?`, now, now, tenant, chat)
	a.emitInboxEvent(tenant, "conversation.message_received", map[string]any{"chat": chat, "channel": "web", "channel_id": channel})
	if webChatAIOn() && webRateLimit("ai:"+chat) {
		var agent int64
		var auto int
		if a.db.QueryRow(`SELECT agent_id,auto_ai FROM mobile_web_channels_v1 WHERE id=? AND tenant_id=? AND enabled=1`, channel, tenant).Scan(&agent, &auto) == nil && auto == 1 {
			go a.webChatAutoRespond(tenant, channel, agent, chat, q.Text)
		}
	}
	writeJSON(w, map[string]any{"ok": true})
}
func (a *App) webChatSendHuman(tenant int64, chat, text string) error {
	text = strings.TrimSpace(text)
	if !webChatOn() {
		return errors.New("Chat Web deshabilitado")
	}
	if !strings.HasPrefix(chat, "webchat:") || len(text) == 0 || len([]rune(text)) > 1600 {
		return errors.New("Mensaje inválido")
	}
	var count int
	err := a.db.QueryRow(`SELECT COUNT(*) FROM mobile_web_sessions_v1 s JOIN mobile_web_channels_v1 c ON c.id=s.channel_id AND c.tenant_id=s.tenant_id WHERE s.tenant_id=? AND s.chat_jid=? AND c.enabled=1`, tenant, chat).Scan(&count)
	if err != nil || count != 1 {
		return errors.New("Conversación no encontrada o canal inactivo")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = a.db.Exec(`INSERT INTO worktic_messages(tenant_id,channel,wa_id,chat_jid,sender_jid,direction,message_type,text,status,timestamp) VALUES(?,?,?,?,?,'out','text',?,'sent',?)`, tenant, "web", "", chat, "human", text, now)
	return err
}

// AI replies are opt-in at both service and channel level, and never bypass
// the existing commercial metering or human takeover of Inbox V21.
func (a *App) webChatAutoRespond(tenant, channel, agent int64, chat, text string) {
	if !webChatAIOn() || a.openAIKey() == "" || !a.inboxAIAllowed(tenant, chat) {
		return
	}
	var name, objective, tone, language, instructions, knowledge string
	err := a.db.QueryRow(`SELECT name,objective,tone,language,instructions,knowledge FROM ai_agents WHERE id=? AND tenant_id=? AND status='active'`, agent, tenant).Scan(&name, &objective, &tone, &language, &instructions, &knowledge)
	if err != nil {
		return
	}
	system := fmt.Sprintf("Eres %s. Objetivo: %s. Tono: %s. Idioma: %s. Instrucciones: %s. Conocimiento verificado: %s. No inventes información. Responde brevemente y no compartas datos de otros usuarios.", name, objective, tone, language, instructions, knowledge)
	if context := a.agentV22Context(tenant, agent, chat); context != "" {
		system += "\n" + context
	}
	reply, callErr := a.callOpenAI(system, text, tenant)
	period := time.Now().UTC().Format("2006-01")
	if callErr != nil {
		_, _ = a.db.Exec(`INSERT INTO ai_agent_usage(tenant_id,agent_id,period,channel,conversations,responses,errors) VALUES(?,?,?,'web',1,0,1) ON CONFLICT(tenant_id,agent_id,period,channel) DO UPDATE SET conversations=conversations+1,errors=errors+1`, tenant, agent, period)
		return
	}
	if strings.TrimSpace(reply) == "" || !a.inboxAIAllowed(tenant, chat) {
		return
	}
	var enabled, auto int
	if a.db.QueryRow(`SELECT enabled,auto_ai FROM mobile_web_channels_v1 WHERE tenant_id=? AND id=?`, tenant, channel).Scan(&enabled, &auto) != nil || enabled != 1 || auto != 1 {
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err = a.db.Exec(`INSERT INTO worktic_messages(tenant_id,channel,wa_id,chat_jid,sender_jid,direction,message_type,text,status,timestamp) VALUES(?,?,?,?,?,'out','text',?,'ai_sent',?)`, tenant, "web", "", chat, "ai", reply, now); err != nil {
		return
	}
	a.agentV22Remember(tenant, agent, chat, text, reply)
	_, _ = a.db.Exec(`INSERT INTO ai_agent_usage(tenant_id,agent_id,period,channel,conversations,responses) VALUES(?,?,?,'web',1,1) ON CONFLICT(tenant_id,agent_id,period,channel) DO UPDATE SET conversations=conversations+1,responses=responses+1`, tenant, agent, period)
}
func (a *App) webChatReplyV1(w http.ResponseWriter, r *http.Request) {
	tenant, _, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	var q struct {
		Chat string `json:"chat"`
		Text string `json:"text"`
	}
	if !webReadJSON(w, r, &q) {
		return
	}
	if err = a.webChatSendHuman(tenant, q.Chat, q.Text); err != nil {
		writeError(w, err, 400)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}
