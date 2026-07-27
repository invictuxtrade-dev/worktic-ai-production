package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/skip2/go-qrcode"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
)

type ChannelConnection struct {
	ID                   int64  `json:"id"`
	TenantID             int64  `json:"tenant_id"`
	PublicID             string `json:"public_id"`
	Type                 string `json:"type"`
	Name                 string `json:"name"`
	Status               string `json:"status"`
	ExternalAccountID    string `json:"external_account_id"`
	AssignedAgentID      int64  `json:"assigned_agent_id"`
	ConfigJSON           string `json:"config_json"`
	EncryptedCredentials string `json:"-"`
	LastConnectedAt      string `json:"last_connected_at"`
	LastDisconnectedAt   string `json:"last_disconnected_at"`
	LastMessageAt        string `json:"last_message_at"`
	LastError            string `json:"last_error"`
	CreatedAt            string `json:"created_at"`
	UpdatedAt            string `json:"updated_at"`
}

type channelRuntime struct {
	conn      ChannelConnection
	wa        *whatsmeow.Client
	qrDataURL string
	mu        sync.RWMutex
}

type ChannelManager struct {
	app      *App
	mu       sync.RWMutex
	runtimes map[int64]*channelRuntime
}

func NewChannelManager(app *App) *ChannelManager {
	return &ChannelManager{app: app, runtimes: map[int64]*channelRuntime{}}
}

func initChannelTenantSchema(db *sql.DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS tenants (
 id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL, account_type TEXT NOT NULL DEFAULT 'business',
 owner_user_id INTEGER NOT NULL UNIQUE, status TEXT NOT NULL DEFAULT 'active', created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS tenant_users (
 tenant_id INTEGER NOT NULL, user_id INTEGER NOT NULL UNIQUE, role TEXT NOT NULL, created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,user_id)
);
CREATE TABLE IF NOT EXISTS channel_connections (
 id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL, public_id TEXT NOT NULL UNIQUE,
 type TEXT NOT NULL, name TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'draft', external_account_id TEXT NOT NULL DEFAULT '',
 encrypted_credentials TEXT NOT NULL DEFAULT '', encryption_key_version INTEGER NOT NULL DEFAULT 1,
 assigned_agent_id INTEGER NOT NULL DEFAULT 0, config_json TEXT NOT NULL DEFAULT '{}', session_reference TEXT NOT NULL DEFAULT '',
 last_connected_at TEXT NOT NULL DEFAULT '', last_disconnected_at TEXT NOT NULL DEFAULT '', last_message_at TEXT NOT NULL DEFAULT '',
 last_error TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_channel_connections_tenant ON channel_connections(tenant_id,type,status);
CREATE UNIQUE INDEX IF NOT EXISTS idx_channel_external ON channel_connections(type,external_account_id) WHERE external_account_id<>'';
CREATE TABLE IF NOT EXISTS channel_audit (
 id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL, connection_id INTEGER NOT NULL,
 user_id INTEGER NOT NULL, action TEXT NOT NULL, detail TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS channel_events (
 id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL, connection_id INTEGER NOT NULL,
 event_type TEXT NOT NULL, status TEXT NOT NULL, detail TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL
);
`)
	if err != nil {
		return err
	}
	for _, stmt := range []string{
		`ALTER TABLE worktic_messages ADD COLUMN tenant_id INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE worktic_messages ADD COLUMN channel_connection_id INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE worktic_contacts ADD COLUMN tenant_id INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE worktic_contacts ADD COLUMN channel_connection_id INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE app_users ADD COLUMN tenant_id INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE billing_plans ADD COLUMN max_whatsapp INTEGER NOT NULL DEFAULT 1`,
		`ALTER TABLE billing_plans ADD COLUMN max_telegram INTEGER NOT NULL DEFAULT 1`,
		`ALTER TABLE billing_plans ADD COLUMN max_messenger INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE billing_plans ADD COLUMN max_agents INTEGER NOT NULL DEFAULT 1`,
	} {
		_, _ = db.Exec(stmt)
	}
	var planLimitsMigrated string
	_ = db.QueryRow(`SELECT value FROM worktic_settings WHERE key='channel_plan_limits_v2'`).Scan(&planLimitsMigrated)
	if planLimitsMigrated == "" {
		_, _ = db.Exec(`UPDATE billing_plans SET max_channels=1,max_whatsapp=1,max_telegram=1,max_messenger=0,max_agents=1 WHERE code='free'`)
		_, _ = db.Exec(`UPDATE billing_plans SET max_channels=2,max_whatsapp=1,max_telegram=2,max_messenger=1,max_agents=2 WHERE code='personal'`)
		_, _ = db.Exec(`UPDATE billing_plans SET max_channels=5,max_whatsapp=3,max_telegram=5,max_messenger=3,max_agents=5 WHERE code='business'`)
		_, _ = db.Exec(`UPDATE billing_plans SET max_channels=15,max_whatsapp=10,max_telegram=15,max_messenger=10,max_agents=15 WHERE code='enterprise'`)
		_, _ = db.Exec(`INSERT OR REPLACE INTO worktic_settings(key,value) VALUES('channel_plan_limits_v2','done')`)
	}
	return migrateTenants(db)
}

func migrateTenants(db *sql.DB) error {
	rows, err := db.Query(`SELECT id,name,company,role,tenant_id FROM app_users ORDER BY id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	type ur struct {
		id                  int64
		name, company, role string
		tenant              int64
	}
	var users []ur
	for rows.Next() {
		var u ur
		_ = rows.Scan(&u.id, &u.name, &u.company, &u.role, &u.tenant)
		users = append(users, u)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	companyTenant := map[string]int64{}
	for _, u := range users {
		key := strings.ToLower(strings.TrimSpace(u.company))
		if key == "" {
			key = fmt.Sprintf("personal-%d", u.id)
		}
		if u.tenant > 0 {
			companyTenant[key] = u.tenant
			continue
		}
		tid := companyTenant[key]
		if tid == 0 {
			var existing int64
			_ = db.QueryRow(`SELECT id FROM tenants WHERE lower(name)=? ORDER BY id LIMIT 1`, key).Scan(&existing)
			if existing > 0 {
				tid = existing
			} else {
				res, _ := db.Exec(`INSERT INTO tenants(name,account_type,owner_user_id,status,created_at,updated_at) VALUES(?,?,?,?,?,?)`, firstNonEmpty(u.company, u.name), map[bool]string{true: "personal", false: "business"}[u.company == ""], u.id, "active", now, now)
				tid, _ = res.LastInsertId()
			}
			companyTenant[key] = tid
		}
		_, _ = db.Exec(`UPDATE app_users SET tenant_id=? WHERE id=?`, tid, u.id)
		_, _ = db.Exec(`INSERT OR REPLACE INTO tenant_users(tenant_id,user_id,role,created_at) VALUES(?,?,?,?)`, tid, u.id, u.role, now)
	}
	// Legacy rows are assigned to the first tenant only, never duplicated. Administrators can review them.
	var firstTenant int64
	_ = db.QueryRow(`SELECT id FROM tenants ORDER BY id LIMIT 1`).Scan(&firstTenant)
	if firstTenant > 0 {
		_, _ = db.Exec(`UPDATE worktic_messages SET tenant_id=? WHERE tenant_id=0`, firstTenant)
		_, _ = db.Exec(`UPDATE worktic_contacts SET tenant_id=? WHERE tenant_id=0`, firstTenant)
	}
	return nil
}

func firstNonEmpty(v ...string) string {
	for _, x := range v {
		if strings.TrimSpace(x) != "" {
			return strings.TrimSpace(x)
		}
	}
	return "Mi espacio"
}

func (a *App) tenantForRequest(r *http.Request) (int64, *User, error) {
	u := a.currentUser(r)
	if u == nil {
		return 0, nil, errors.New("sesión requerida")
	}
	var tid int64
	if err := a.db.QueryRow(`SELECT tenant_id FROM app_users WHERE id=?`, u.ID).Scan(&tid); err != nil || tid == 0 {
		return 0, u, errors.New("cuenta sin tenant asignado")
	}
	return tid, u, nil
}

func channelTypeLimit(p Plan, typ string) int {
	switch p.Code {
	case "personal":
		if typ == "whatsapp_qr" {
			return 1
		}
		if typ == "telegram" {
			return 2
		}
		if typ == "messenger" {
			return 1
		}
	case "business":
		if typ == "whatsapp_qr" {
			return 3
		}
		if typ == "telegram" {
			return 5
		}
		if typ == "messenger" {
			return 3
		}
	case "enterprise":
		if typ == "whatsapp_qr" {
			return 10
		}
		if typ == "telegram" {
			return 15
		}
		if typ == "messenger" {
			return 10
		}
	default:
		if typ == "whatsapp_qr" || typ == "telegram" {
			return 1
		}
		return 0
	}
	return 0
}

func (a *App) channelTypeLimitForPlan(p Plan, typ string) int {
	var wa, tg, msg int
	if err := a.db.QueryRow(`SELECT max_whatsapp,max_telegram,max_messenger FROM billing_plans WHERE code=?`, p.Code).Scan(&wa, &tg, &msg); err == nil {
		switch typ {
		case "whatsapp_qr":
			return wa
		case "telegram":
			return tg
		case "messenger":
			return msg
		}
	}
	return channelTypeLimit(p, typ)
}

func (cm *ChannelManager) restoreActive() {
	rows, err := cm.app.db.Query(`SELECT id,tenant_id,public_id,type,name,status,external_account_id,assigned_agent_id,config_json,encrypted_credentials,last_connected_at,last_disconnected_at,last_message_at,last_error,created_at,updated_at FROM channel_connections WHERE status IN ('connected','reconnecting','connecting') ORDER BY id`)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var c ChannelConnection
		_ = rows.Scan(&c.ID, &c.TenantID, &c.PublicID, &c.Type, &c.Name, &c.Status, &c.ExternalAccountID, &c.AssignedAgentID, &c.ConfigJSON, &c.EncryptedCredentials, &c.LastConnectedAt, &c.LastDisconnectedAt, &c.LastMessageAt, &c.LastError, &c.CreatedAt, &c.UpdatedAt)
		if c.Type == "whatsapp_qr" {
			go func(x ChannelConnection) {
				time.Sleep(time.Duration(x.ID%8) * time.Second)
				_ = cm.startWhatsApp(x, false)
			}(c)
		}
	}
}

func (cm *ChannelManager) startWhatsApp(c ChannelConnection, needQR bool) error {
	base := filepath.Join(cm.app.cfg.DataDir, "wa_sessions", fmt.Sprintf("tenant_%d", c.TenantID))
	_ = os.MkdirAll(base, 0700)
	dsn := "file:" + filepath.ToSlash(filepath.Join(base, fmt.Sprintf("channel_%d.db", c.ID))) + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	container, err := sqlstore.New(context.Background(), "sqlite", dsn, waLog.Stdout(fmt.Sprintf("WA-%d", c.ID), "WARN", true))
	if err != nil {
		return err
	}
	device, err := container.GetFirstDevice(context.Background())
	if err != nil {
		return err
	}
	cli := whatsmeow.NewClient(device, waLog.Stdout(fmt.Sprintf("WA-%d", c.ID), "INFO", true))
	rt := &channelRuntime{conn: c, wa: cli}
	cm.mu.Lock()
	cm.runtimes[c.ID] = rt
	cm.mu.Unlock()
	cli.AddEventHandler(func(evt interface{}) { cm.handleWAEvent(c.ID, evt) })
	if device.ID == nil || needQR {
		qr, err := cli.GetQRChannel(context.Background())
		if err != nil {
			return err
		}
		if err = cli.Connect(); err != nil {
			return err
		}
		go func() {
			for ev := range qr {
				if ev.Event == "code" {
					if img, e := qrDataURL(ev.Code); e == nil {
						rt.mu.Lock()
						rt.qrDataURL = img
						rt.mu.Unlock()
						cm.updateStatus(c.ID, "waiting_qr", "")
					}
				} else if ev.Event == "success" {
					cm.updateStatus(c.ID, "connected", "")
				} else if ev.Event == "timeout" {
					cm.updateStatus(c.ID, "error", "QR vencido")
				}
			}
		}()
		return nil
	}
	cm.updateStatus(c.ID, "reconnecting", "")
	if err = cli.Connect(); err != nil {
		cm.updateStatus(c.ID, "error", err.Error())
		return err
	}
	return nil
}

func qrDataURL(code string) (string, error) {
	png, err := qrcodeEncode(code)
	if err != nil {
		return "", err
	}
	return "data:image/png;base64," + png, nil
}

func (cm *ChannelManager) handleWAEvent(id int64, evt interface{}) {
	rt := cm.runtime(id)
	if rt == nil {
		return
	}
	switch v := evt.(type) {
	case *events.Message:
		if v.Info.IsFromMe {
			return
		}
		text, typ := extractContent(v.Message)
		if strings.TrimSpace(text) == "" {
			text = "[Mensaje " + typ + "]"
		}
		chat := fmt.Sprintf("t%d:c%d:%s", rt.conn.TenantID, id, v.Info.Chat.String())
		sender := v.Info.Sender.String()
		now := v.Info.Timestamp.UTC().Format(time.RFC3339)
		_, _ = cm.app.db.Exec(`INSERT OR IGNORE INTO worktic_messages(tenant_id,channel_connection_id,channel,wa_id,chat_jid,sender_jid,direction,message_type,text,status,timestamp) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, rt.conn.TenantID, id, "whatsapp", v.Info.ID, chat, sender, "in", typ, text, "received", now)
		_, _ = cm.app.db.Exec(`INSERT INTO worktic_contacts(tenant_id,channel_connection_id,chat_jid,channel,phone,name,unread,updated_at) VALUES(?,?,?,?,?,?,1,?) ON CONFLICT(chat_jid) DO UPDATE SET unread=worktic_contacts.unread+1,name=CASE WHEN excluded.name<>'' THEN excluded.name ELSE worktic_contacts.name END,updated_at=excluded.updated_at`, rt.conn.TenantID, id, chat, "whatsapp", shortJID(v.Info.Chat.String()), strings.TrimSpace(v.Info.PushName), now)
		_, _ = cm.app.db.Exec(`UPDATE channel_connections SET last_message_at=?,updated_at=? WHERE id=?`, now, now, id)
		go cm.maybeTenantAIReply(rt, v.Info.Chat, chat, text)
	case *events.Connected:
		ext := ""
		if rt.wa.Store.ID != nil {
			ext = rt.wa.Store.ID.String()
		}
		_, _ = cm.app.db.Exec(`UPDATE channel_connections SET status='connected',external_account_id=?,last_connected_at=?,last_error='',updated_at=? WHERE id=?`, ext, time.Now().UTC().Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339), id)
	case *events.Disconnected:
		cm.updateStatus(id, "reconnecting", "")
		go func() {
			for n := 1; n <= 5; n++ {
				time.Sleep(time.Duration(n*4) * time.Second)
				if rt.wa.IsConnected() {
					return
				}
				if rt.wa.Connect() == nil {
					return
				}
			}
			cm.updateStatus(id, "error", "No fue posible reconectar")
		}()
	case *events.LoggedOut:
		cm.updateStatus(id, "revoked", "La sesión fue cerrada desde WhatsApp")
	}
}

func (cm *ChannelManager) maybeTenantAIReply(rt *channelRuntime, recipient types.JID, storedChat, text string) {
	if rt == nil || rt.wa == nil || strings.TrimSpace(text) == "" {
		return
	}
	if cm.app.openAIKey() == "" {
		_, _ = cm.app.db.Exec(`UPDATE channel_connections SET last_error=?,updated_at=? WHERE id=?`, "Falta configurar OPENAI_API_KEY", time.Now().UTC().Format(time.RFC3339), rt.conn.ID)
		return
	}
	log.Printf("[WA-AI] recibido tenant=%d conexion=%d chat=%s", rt.conn.TenantID, rt.conn.ID, storedChat)
	key := fmt.Sprintf("tenant-ai:%d:%d:%s", rt.conn.TenantID, rt.conn.ID, storedChat)
	cm.app.mu.Lock()
	if t, ok := cm.app.autoLast[key]; ok && time.Since(t) < time.Duration(cm.app.cfg.AutoReplyCooldownSeconds)*time.Second {
		cm.app.mu.Unlock()
		return
	}
	cm.app.autoLast[key] = time.Now()
	cm.app.mu.Unlock()

	agentID := rt.conn.AssignedAgentID
	if agentID == 0 {
		resolved, err := cm.app.resolveAgent(rt.conn.TenantID, "whatsapp", "", text, 0, 0, 0)
		if err == nil {
			agentID = resolved
		}
	}
	history := cm.tenantRecentHistory(rt.conn.TenantID, rt.conn.ID, storedChat, 10)
	system := ""
	if agentID > 0 {
		var ag AIAgent
		var isDefault int
		err := cm.app.db.QueryRow(`SELECT id,tenant_id,name,type,description,objective,tone,language,instructions,knowledge,greeting,away_message,handoff_rules,tools,channels,status,is_default,monthly_budget,created_at,updated_at FROM ai_agents WHERE id=? AND tenant_id=? AND status='active'`, agentID, rt.conn.TenantID).Scan(
			&ag.ID, &ag.TenantID, &ag.Name, &ag.Type, &ag.Description, &ag.Objective, &ag.Tone, &ag.Language, &ag.Instructions, &ag.Knowledge, &ag.Greeting, &ag.AwayMessage, &ag.HandoffRules, &ag.Tools, &ag.Channels, &ag.Status, &isDefault, &ag.MonthlyBudget, &ag.CreatedAt, &ag.UpdatedAt,
		)
		if err == nil {
			system = fmt.Sprintf("Eres %s, agente especializado de tipo %s. Objetivo: %s. Tono: %s. Idioma: %s. Instrucciones: %s. Conocimiento verificado: %s. Herramientas permitidas: %s. No inventes datos y responde de forma humana, clara y breve. Historial reciente:\n%s", ag.Name, ag.Type, ag.Objective, ag.Tone, ag.Language, ag.Instructions, ag.Knowledge, ag.Tools, history)
		} else {
			// Una asignación antigua o inválida nunca debe dejar al canal sin respuesta.
			// Se limpia la asignación y se usa el Asistente Principal como respaldo.
			agentID = 0
			_, _ = cm.app.db.Exec(`UPDATE channel_connections SET assigned_agent_id=0,last_error=?,updated_at=? WHERE id=?`, "Agente asignado inválido; usando Asistente Principal", time.Now().UTC().Format(time.RFC3339), rt.conn.ID)
		}
	}
	if system == "" {
		legacy := cm.app.loadAgent()
		if !legacy.Enabled {
			_, _ = cm.app.db.Exec(`UPDATE channel_connections SET last_error=?,updated_at=? WHERE id=?`, "Activa el Asistente Principal o asigna un Agente Especializado activo", time.Now().UTC().Format(time.RFC3339), rt.conn.ID)
			return
		}
		system = fmt.Sprintf("Eres %s, asistente principal de %s. Objetivo: %s. Tono: %s. Instrucciones: %s. Conocimiento verificado: %s. No inventes datos y responde de forma humana, clara y breve. Historial reciente:\n%s", legacy.Name, legacy.Company, legacy.Objective, legacy.Tone, legacy.Instructions, legacy.Knowledge, history)
	}
	log.Printf("[WA-AI] generando respuesta tenant=%d conexion=%d agente=%d", rt.conn.TenantID, rt.conn.ID, agentID)
	reply, err := cm.app.callOpenAI(system, text)
	period := time.Now().UTC().Format("2006-01")
	if err != nil {
		if agentID > 0 {
			_, _ = cm.app.db.Exec(`INSERT INTO ai_agent_usage(tenant_id,agent_id,period,channel,conversations,responses,errors) VALUES(?,?,?,'whatsapp',1,0,1) ON CONFLICT(tenant_id,agent_id,period,channel) DO UPDATE SET conversations=conversations+1,errors=errors+1`, rt.conn.TenantID, agentID, period)
		}
		_, _ = cm.app.db.Exec(`UPDATE channel_connections SET last_error=?,updated_at=? WHERE id=?`, err.Error(), time.Now().UTC().Format(time.RFC3339), rt.conn.ID)
		return
	}
	if strings.TrimSpace(reply) == "" {
		return
	}
	time.Sleep(700 * time.Millisecond)
	resp, err := rt.wa.SendMessage(context.Background(), recipient, &waProto.Message{Conversation: proto.String(reply)})
	if err != nil {
		_, _ = cm.app.db.Exec(`UPDATE channel_connections SET last_error=?,updated_at=? WHERE id=?`, err.Error(), time.Now().UTC().Format(time.RFC3339), rt.conn.ID)
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, _ = cm.app.db.Exec(`INSERT OR IGNORE INTO worktic_messages(tenant_id,channel_connection_id,channel,wa_id,chat_jid,sender_jid,direction,message_type,text,status,timestamp) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, rt.conn.TenantID, rt.conn.ID, "whatsapp", resp.ID, storedChat, "ai", "out", "text", reply, "ai_sent", now)
	if agentID > 0 {
		_, _ = cm.app.db.Exec(`INSERT INTO ai_agent_usage(tenant_id,agent_id,period,channel,conversations,responses) VALUES(?,?,?,'whatsapp',1,1) ON CONFLICT(tenant_id,agent_id,period,channel) DO UPDATE SET conversations=conversations+1,responses=responses+1`, rt.conn.TenantID, agentID, period)
	}
	_, _ = cm.app.db.Exec(`UPDATE channel_connections SET last_error='',updated_at=? WHERE id=?`, now, rt.conn.ID)
	log.Printf("[WA-AI] respuesta enviada tenant=%d conexion=%d mensaje=%s", rt.conn.TenantID, rt.conn.ID, resp.ID)
}

func (cm *ChannelManager) tenantRecentHistory(tenantID, connectionID int64, chat string, limit int) string {
	rows, err := cm.app.db.Query(`SELECT direction,text FROM worktic_messages WHERE tenant_id=? AND channel_connection_id=? AND chat_jid=? ORDER BY id DESC LIMIT ?`, tenantID, connectionID, chat, limit)
	if err != nil {
		return ""
	}
	defer rows.Close()
	var lines []string
	for rows.Next() {
		var direction, value string
		if rows.Scan(&direction, &value) != nil {
			continue
		}
		who := "Cliente"
		if direction == "out" {
			who = "Asistente"
		}
		lines = append([]string{who + ": " + value}, lines...)
	}
	return strings.Join(lines, "\n")
}

func (cm *ChannelManager) runtime(id int64) *channelRuntime {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.runtimes[id]
}
func (cm *ChannelManager) updateStatus(id int64, status, detail string) {
	now := time.Now().UTC().Format(time.RFC3339)
	_, _ = cm.app.db.Exec(`UPDATE channel_connections SET status=?,last_error=?,updated_at=? WHERE id=?`, status, detail, now, id)
}

func (a *App) channelConnectionsHandler(w http.ResponseWriter, r *http.Request) {
	tid, u, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, 409)
		return
	}
	switch r.Method {
	case http.MethodGet:
		rows, e := a.db.Query(`SELECT id,tenant_id,public_id,type,name,status,external_account_id,assigned_agent_id,config_json,encrypted_credentials,last_connected_at,last_disconnected_at,last_message_at,last_error,created_at,updated_at FROM channel_connections WHERE tenant_id=? ORDER BY id DESC`, tid)
		if e != nil {
			writeError(w, e, 500)
			return
		}
		defer rows.Close()
		out := []ChannelConnection{}
		for rows.Next() {
			var c ChannelConnection
			_ = rows.Scan(&c.ID, &c.TenantID, &c.PublicID, &c.Type, &c.Name, &c.Status, &c.ExternalAccountID, &c.AssignedAgentID, &c.ConfigJSON, &c.EncryptedCredentials, &c.LastConnectedAt, &c.LastDisconnectedAt, &c.LastMessageAt, &c.LastError, &c.CreatedAt, &c.UpdatedAt)
			out = append(out, c)
		}
		p, _, _ := a.activePlan(a.billingAccountUserID(u))
		writeJSON(w, map[string]any{"connections": out, "max_total": p.MaxChannels, "limits": map[string]int{"whatsapp_qr": a.channelTypeLimitForPlan(p, "whatsapp_qr"), "telegram": a.channelTypeLimitForPlan(p, "telegram"), "messenger": a.channelTypeLimitForPlan(p, "messenger")}})
	case http.MethodPost:
		if u.Role != "owner" && u.Role != "admin" && u.Role != "superadmin" {
			writeError(w, errors.New("sin permiso para conectar canales"), 403)
			return
		}
		var q ChannelConnection
		_ = json.NewDecoder(r.Body).Decode(&q)
		q.Type = strings.ToLower(strings.TrimSpace(q.Type))
		if q.Type != "whatsapp_qr" && q.Type != "telegram" && q.Type != "messenger" {
			writeError(w, errors.New("tipo de canal no soportado"), 400)
			return
		}
		p, _, _ := a.activePlan(a.billingAccountUserID(u))
		var total, byType int
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM channel_connections WHERE tenant_id=? AND status<>'deleted'`, tid).Scan(&total)
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM channel_connections WHERE tenant_id=? AND type=? AND status<>'deleted'`, tid, q.Type).Scan(&byType)
		if total >= p.MaxChannels {
			writeError(w, fmt.Errorf("tu plan permite %d conexiones", p.MaxChannels), 403)
			return
		}
		if byType >= a.channelTypeLimitForPlan(p, q.Type) {
			writeError(w, fmt.Errorf("límite alcanzado para %s", q.Type), 403)
			return
		}
		now := time.Now().UTC().Format(time.RFC3339)
		pub := randomToken(12)
		if q.Name == "" {
			q.Name = map[string]string{"whatsapp_qr": "WhatsApp", "telegram": "Telegram", "messenger": "Messenger"}[q.Type]
		}
		res, e := a.db.Exec(`INSERT INTO channel_connections(tenant_id,public_id,type,name,status,assigned_agent_id,config_json,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?)`, tid, pub, q.Type, q.Name, "draft", q.AssignedAgentID, firstNonEmpty(q.ConfigJSON, "{}"), now, now)
		if e != nil {
			writeError(w, e, 500)
			return
		}
		id, _ := res.LastInsertId()
		_, _ = a.db.Exec(`INSERT INTO channel_audit(tenant_id,connection_id,user_id,action,detail,created_at) VALUES(?,?,?,?,?,?)`, tid, id, u.ID, "create", q.Type, now)
		writeJSON(w, map[string]any{"ok": true, "id": id, "public_id": pub})
	case http.MethodDelete:
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		var typ string
		if a.db.QueryRow(`SELECT type FROM channel_connections WHERE id=? AND tenant_id=?`, id, tid).Scan(&typ) != nil {
			writeError(w, errors.New("conexión no encontrada"), 404)
			return
		}
		if typ == "telegram" {
			a.deleteTelegramWebhook(id)
		}
		if rt := a.channelManager.runtime(id); rt != nil && rt.wa != nil {
			rt.wa.Disconnect()
		}
		_, _ = a.db.Exec(`DELETE FROM channel_connections WHERE id=? AND tenant_id=?`, id, tid)
		writeJSON(w, map[string]any{"ok": true})
	default:
		http.Error(w, "Método no permitido", 405)
	}
}

func (a *App) channelActionHandler(w http.ResponseWriter, r *http.Request) {
	tid, u, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, 409)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	var q struct {
		ID         int64  `json:"id"`
		Action     string `json:"action"`
		Token      string `json:"token"`
		ExternalID string `json:"external_id"`
		AgentID    int64  `json:"agent_id"`
		AppSecret  string `json:"app_secret"`
	}
	_ = json.NewDecoder(r.Body).Decode(&q)
	var c ChannelConnection
	if a.db.QueryRow(`SELECT id,tenant_id,public_id,type,name,status,external_account_id,assigned_agent_id,config_json,encrypted_credentials,last_connected_at,last_disconnected_at,last_message_at,last_error,created_at,updated_at FROM channel_connections WHERE id=? AND tenant_id=?`, q.ID, tid).Scan(&c.ID, &c.TenantID, &c.PublicID, &c.Type, &c.Name, &c.Status, &c.ExternalAccountID, &c.AssignedAgentID, &c.ConfigJSON, &c.EncryptedCredentials, &c.LastConnectedAt, &c.LastDisconnectedAt, &c.LastMessageAt, &c.LastError, &c.CreatedAt, &c.UpdatedAt) != nil {
		writeError(w, errors.New("conexión no encontrada"), 404)
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	switch q.Action {
	case "connect":
		if c.Type == "whatsapp_qr" {
			if err := a.channelManager.startWhatsApp(c, true); err != nil {
				writeError(w, err, 500)
				return
			}
		}
		if c.Type == "telegram" {
			if strings.TrimSpace(q.Token) == "" {
				writeError(w, errors.New("token obligatorio"), 400)
				return
			}
			bot, info, webhookSecret, hookErr := a.configureTelegramWebhook(r, c, strings.TrimSpace(q.Token))
			if hookErr != nil {
				_, _ = a.db.Exec(`UPDATE channel_connections SET status='error',last_error=?,updated_at=? WHERE id=? AND tenant_id=?`, hookErr.Error(), now, q.ID, tid)
				writeError(w, hookErr, 502)
				return
			}
			creds := encryptTelegramCredentials(telegramChannelCredentials{Token: strings.TrimSpace(q.Token), WebhookSecret: webhookSecret}, a.cfg.ChannelEncryptionKey)
			cfg, _ := json.Marshal(map[string]any{"external_id": bot.Username, "bot_id": bot.ID, "bot_name": bot.FirstName, "webhook_url": info.URL})
			_, _ = a.db.Exec(`UPDATE channel_connections SET encrypted_credentials=?,external_account_id=?,config_json=?,status='connected',assigned_agent_id=?,last_connected_at=?,last_error='',updated_at=? WHERE id=? AND tenant_id=?`, creds, bot.Username, string(cfg), q.AgentID, now, now, q.ID, tid)
		}
		if c.Type == "messenger" {
			if strings.TrimSpace(q.Token) == "" {
				writeError(w, errors.New("Page Access Token obligatorio"), 400)
				return
			}
			result, cfgErr := a.configureMessenger(r, c, strings.TrimSpace(q.Token), strings.TrimSpace(q.ExternalID), strings.TrimSpace(q.AppSecret))
			if cfgErr != nil {
				writeError(w, cfgErr, 502)
				return
			}
			_, _ = a.db.Exec(`UPDATE channel_connections SET assigned_agent_id=? WHERE id=? AND tenant_id=?`, q.AgentID, q.ID, tid)
			writeJSON(w, result)
			return
		}
	case "test":
		if c.Type == "telegram" {
			result, testErr := a.testTelegramConnection(r, c)
			if testErr != nil {
				_, _ = a.db.Exec(`UPDATE channel_connections SET last_error=?,updated_at=? WHERE id=? AND tenant_id=?`, testErr.Error(), now, q.ID, tid)
				writeError(w, testErr, 502)
				return
			}
			if ok, _ := result["ok"].(bool); ok {
				_, _ = a.db.Exec(`UPDATE channel_connections SET status='connected',last_error='',updated_at=? WHERE id=? AND tenant_id=?`, now, q.ID, tid)
			} else {
				detail, _ := result["last_error"].(string)
				if detail == "" {
					detail = "El webhook no coincide con esta conexión"
				}
				_, _ = a.db.Exec(`UPDATE channel_connections SET last_error=?,updated_at=? WHERE id=? AND tenant_id=?`, detail, now, q.ID, tid)
			}
			_, _ = a.db.Exec(`INSERT INTO channel_audit(tenant_id,connection_id,user_id,action,detail,created_at) VALUES(?,?,?,?,?,?)`, tid, q.ID, u.ID, "test", c.Type, now)
			writeJSON(w, result)
			return
		}
		if c.Type == "messenger" {
			result, testErr := a.testMessengerConnection(r, c)
			if testErr != nil {
				writeError(w, testErr, 502)
				return
			}
			writeJSON(w, result)
			return
		}
		writeJSON(w, map[string]any{"ok": c.Status == "connected", "platform": c.Type, "status": c.Status})
		return
	case "disconnect":
		if c.Type == "telegram" {
			a.deleteTelegramWebhook(q.ID)
		}
		if rt := a.channelManager.runtime(q.ID); rt != nil && rt.wa != nil {
			rt.wa.Disconnect()
		}
		_, _ = a.db.Exec(`UPDATE channel_connections SET status='disconnected',last_disconnected_at=?,updated_at=? WHERE id=? AND tenant_id=?`, now, now, q.ID, tid)
	case "pause":
		_, _ = a.db.Exec(`UPDATE channel_connections SET status='paused',updated_at=? WHERE id=? AND tenant_id=?`, now, q.ID, tid)
	default:
		writeError(w, errors.New("acción no soportada"), 400)
		return
	}
	_, _ = a.db.Exec(`INSERT INTO channel_audit(tenant_id,connection_id,user_id,action,detail,created_at) VALUES(?,?,?,?,?,?)`, tid, q.ID, u.ID, q.Action, c.Type, now)
	writeJSON(w, map[string]any{"ok": true})
}

func (a *App) channelQRHandler(w http.ResponseWriter, r *http.Request) {
	tid, _, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, 409)
		return
	}
	id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	var owner int64
	if a.db.QueryRow(`SELECT tenant_id FROM channel_connections WHERE id=?`, id).Scan(&owner) != nil || owner != tid {
		writeError(w, errors.New("conexión no encontrada"), 404)
		return
	}
	rt := a.channelManager.runtime(id)
	if rt == nil {
		writeJSON(w, map[string]any{"state": "idle"})
		return
	}
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	writeJSON(w, map[string]any{"data_url": rt.qrDataURL, "connected": rt.wa != nil && rt.wa.IsConnected() && rt.wa.IsLoggedIn()})
}

func (a *App) channelHealthHandler(w http.ResponseWriter, r *http.Request) {
	tid, _, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, 409)
		return
	}
	rows, e := a.db.Query(`SELECT status,COUNT(*) FROM channel_connections WHERE tenant_id=? GROUP BY status`, tid)
	if e != nil {
		writeError(w, e, 500)
		return
	}
	defer rows.Close()
	m := map[string]int{}
	for rows.Next() {
		var s string
		var n int
		_ = rows.Scan(&s, &n)
		m[s] = n
	}
	var msgs int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM worktic_messages WHERE tenant_id=?`, tid).Scan(&msgs)
	writeJSON(w, map[string]any{"states": m, "messages": msgs, "isolation": "tenant_id + connection_id", "session_storage": filepath.Join(a.cfg.DataDir, "wa_sessions", "tenant_<id>", "channel_<id>.db")})
}

func (a *App) sendViaConnection(ctx context.Context, tid, connectionID int64, externalChat, text string) (string, error) {
	var typ string
	if a.db.QueryRow(`SELECT type FROM channel_connections WHERE id=? AND tenant_id=? AND status='connected'`, connectionID, tid).Scan(&typ) != nil {
		return "", errors.New("conexión no disponible")
	}
	if typ == "whatsapp_qr" {
		rt := a.channelManager.runtime(connectionID)
		if rt == nil || rt.wa == nil || !rt.wa.IsLoggedIn() {
			return "", errors.New("WhatsApp no conectado")
		}
		jid, err := types.ParseJID(externalChat)
		if err != nil {
			return "", err
		}
		resp, err := rt.wa.SendMessage(ctx, jid, &waProto.Message{Conversation: proto.String(text)})
		if err != nil {
			return "", err
		}
		return resp.ID, nil
	}
	return "", errors.New("envío directo para este canal requiere webhook oficial configurado")
}

func encryptLocal(s, key string) string {
	if s == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(key))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return ""
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return ""
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return ""
	}
	sealed := gcm.Seal(nonce, nonce, []byte(s), nil)
	return base64.RawURLEncoding.EncodeToString(sealed)
}
func qrcodeEncode(code string) (string, error) {
	b, err := qrcode.Encode(code, qrcode.Medium, 320)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

func (m *ChannelManager) shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, rt := range m.runtimes {
		if rt != nil && rt.wa != nil {
			rt.wa.Disconnect()
		}
		delete(m.runtimes, key)
	}
}
