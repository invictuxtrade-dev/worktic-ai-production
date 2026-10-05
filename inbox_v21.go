package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

func initInboxV21Schema(db *DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS inbox_conversations(
 id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL, chat_jid TEXT NOT NULL,
 status TEXT NOT NULL DEFAULT 'open', mode TEXT NOT NULL DEFAULT 'ai', priority TEXT NOT NULL DEFAULT 'normal',
 assigned_user_id INTEGER NOT NULL DEFAULT 0, department TEXT NOT NULL DEFAULT '', tags TEXT NOT NULL DEFAULT '',
 sla_due_at TEXT NOT NULL DEFAULT '', closed_at TEXT NOT NULL DEFAULT '', updated_at TEXT NOT NULL,
 UNIQUE(tenant_id,chat_jid)
);
CREATE INDEX IF NOT EXISTS idx_inbox_conversations_tenant ON inbox_conversations(tenant_id,status,updated_at DESC);
CREATE TABLE IF NOT EXISTS inbox_notes(
 id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL, chat_jid TEXT NOT NULL,
 user_id INTEGER NOT NULL DEFAULT 0, user_name TEXT NOT NULL DEFAULT '', note TEXT NOT NULL,
 created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_inbox_notes_chat ON inbox_notes(tenant_id,chat_jid,id DESC);
CREATE TABLE IF NOT EXISTS inbox_departments(
 id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL, name TEXT NOT NULL, sla_minutes INTEGER NOT NULL DEFAULT 60,
 active INTEGER NOT NULL DEFAULT 1, created_at TEXT NOT NULL, UNIQUE(tenant_id,name)
);
`)
	if err != nil {
		return err
	}
	return nil
}

func (a *App) ensureInboxConversation(tenantID int64, chat string) {
	if tenantID <= 0 || strings.TrimSpace(chat) == "" {
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, _ = a.db.Exec(`INSERT OR IGNORE INTO inbox_conversations(tenant_id,chat_jid,updated_at) VALUES(?,?,?)`, tenantID, chat, now)
}

func (a *App) inboxAIAllowed(tenantID int64, chat string) bool {
	a.ensureInboxConversation(tenantID, chat)
	var mode, status string
	err := a.db.QueryRow(`SELECT mode,status FROM inbox_conversations WHERE tenant_id=? AND chat_jid=?`, tenantID, chat).Scan(&mode, &status)
	if err != nil {
		return true
	}
	return mode != "human" && status != "closed"
}

func (a *App) emitInboxEvent(tenantID int64, eventType string, payload map[string]any) {
	b, _ := json.Marshal(payload)
	_, _ = a.db.Exec(`INSERT INTO automation_events(tenant_id,event_type,source,payload_json,status,created_at) VALUES(?,?, 'inbox',?,'pending',?)`, tenantID, eventType, string(b), time.Now().UTC().Format(time.RFC3339))
}

func (a *App) inboxOverviewHandler(w http.ResponseWriter, r *http.Request) {
	tid, _, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	var open, pending, closed, human, unread int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM inbox_conversations WHERE tenant_id=? AND status='open'`, tid).Scan(&open)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM inbox_conversations WHERE tenant_id=? AND status='pending'`, tid).Scan(&pending)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM inbox_conversations WHERE tenant_id=? AND status='closed'`, tid).Scan(&closed)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM inbox_conversations WHERE tenant_id=? AND mode='human' AND status<>'closed'`, tid).Scan(&human)
	_ = a.db.QueryRow(`SELECT COALESCE(SUM(unread),0) FROM worktic_contacts WHERE tenant_id=?`, tid).Scan(&unread)
	writeJSON(w, map[string]any{"open": open, "pending": pending, "closed": closed, "human": human, "unread": unread})
}

func (a *App) inboxConversationHandler(w http.ResponseWriter, r *http.Request) {
	tid, u, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	chat := strings.TrimSpace(r.URL.Query().Get("chat"))
	if chat == "" {
		writeError(w, errors.New("chat obligatorio"), 400)
		return
	}
	var exists int
	if a.db.QueryRow(`SELECT COUNT(*) FROM worktic_contacts WHERE tenant_id=? AND chat_jid=?`, tid, chat).Scan(&exists) != nil || exists == 0 {
		writeError(w, errors.New("conversación no encontrada"), 404)
		return
	}
	a.ensureInboxConversation(tid, chat)
	if r.Method == http.MethodGet {
		var status, mode, priority, department, tags, sla, closed, updated string
		var assigned int64
		_ = a.db.QueryRow(`SELECT status,mode,priority,assigned_user_id,department,tags,sla_due_at,closed_at,updated_at FROM inbox_conversations WHERE tenant_id=? AND chat_jid=?`, tid, chat).Scan(&status, &mode, &priority, &assigned, &department, &tags, &sla, &closed, &updated)
		var assignee string
		if assigned > 0 {
			_ = a.db.QueryRow(`SELECT name FROM app_users WHERE id=? AND tenant_id=?`, assigned, tid).Scan(&assignee)
		}
		writeJSON(w, map[string]any{"chat": chat, "status": status, "mode": mode, "priority": priority, "assigned_user_id": assigned, "assigned_user_name": assignee, "department": department, "tags": tags, "sla_due_at": sla, "closed_at": closed, "updated_at": updated})
		return
	}
	if r.Method != http.MethodPut {
		http.Error(w, "Método no permitido", 405)
		return
	}
	var req struct {
		Status, Mode, Priority, Department, Tags string
		AssignedUserID                           int64           `json:"assigned_user_id"`
		SLA                                      MinutesOrString `json:"-"`
	}
	var raw map[string]any
	if json.NewDecoder(r.Body).Decode(&raw) != nil {
		writeError(w, errors.New("payload inválido"), 400)
		return
	}
	getS := func(k string) string {
		if v, ok := raw[k].(string); ok {
			return strings.TrimSpace(v)
		}
		return ""
	}
	req.Status = getS("status")
	req.Mode = getS("mode")
	req.Priority = getS("priority")
	req.Department = getS("department")
	req.Tags = getS("tags")
	if v, ok := raw["assigned_user_id"].(float64); ok {
		req.AssignedUserID = int64(v)
	}
	var curStatus, curMode, curPriority, curDept, curTags, curSLA string
	var curAssigned int64
	_ = a.db.QueryRow(`SELECT status,mode,priority,assigned_user_id,department,tags,sla_due_at FROM inbox_conversations WHERE tenant_id=? AND chat_jid=?`, tid, chat).Scan(&curStatus, &curMode, &curPriority, &curAssigned, &curDept, &curTags, &curSLA)
	if req.Status == "" {
		req.Status = curStatus
	}
	if req.Mode == "" {
		req.Mode = curMode
	}
	if req.Priority == "" {
		req.Priority = curPriority
	}
	if req.Department == "" {
		req.Department = curDept
	}
	if req.Tags == "" {
		req.Tags = curTags
	}
	if _, ok := raw["assigned_user_id"]; !ok {
		req.AssignedUserID = curAssigned
	}
	if req.Status != "open" && req.Status != "pending" && req.Status != "closed" {
		writeError(w, errors.New("estado inválido"), 400)
		return
	}
	if req.Mode != "ai" && req.Mode != "human" {
		writeError(w, errors.New("modo inválido"), 400)
		return
	}
	if req.AssignedUserID > 0 {
		var n int
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM app_users WHERE id=? AND tenant_id=? AND active=1`, req.AssignedUserID, tid).Scan(&n)
		if n == 0 {
			writeError(w, errors.New("asesor inválido"), 400)
			return
		}
	}
	sla := curSLA
	if mins, ok := raw["sla_minutes"].(float64); ok && mins > 0 {
		sla = time.Now().UTC().Add(time.Duration(mins) * time.Minute).Format(time.RFC3339)
	}
	closed := ""
	if req.Status == "closed" {
		closed = time.Now().UTC().Format(time.RFC3339)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = a.db.Exec(`UPDATE inbox_conversations SET status=?,mode=?,priority=?,assigned_user_id=?,department=?,tags=?,sla_due_at=?,closed_at=?,updated_at=? WHERE tenant_id=? AND chat_jid=?`, req.Status, req.Mode, req.Priority, req.AssignedUserID, req.Department, req.Tags, sla, closed, now, tid, chat)
	if err != nil {
		writeError(w, err, 500)
		return
	}
	if req.Mode != curMode {
		a.emitInboxEvent(tid, "conversation.mode_changed", map[string]any{"chat": chat, "from": curMode, "to": req.Mode, "user_id": u.ID})
	}
	if req.AssignedUserID != curAssigned {
		a.emitInboxEvent(tid, "conversation.assigned", map[string]any{"chat": chat, "assigned_user_id": req.AssignedUserID, "user_id": u.ID})
	}
	if req.Status != curStatus {
		a.emitInboxEvent(tid, "conversation."+req.Status, map[string]any{"chat": chat, "from": curStatus, "to": req.Status, "user_id": u.ID})
	}
	writeJSON(w, map[string]any{"ok": true})
}

// MinutesOrString is intentionally empty: JSON is parsed dynamically above to keep compatibility with older clients.
type MinutesOrString struct{}

func (a *App) inboxNotesHandler(w http.ResponseWriter, r *http.Request) {
	tid, u, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	chat := strings.TrimSpace(r.URL.Query().Get("chat"))
	if chat == "" {
		writeError(w, errors.New("chat obligatorio"), 400)
		return
	}
	if r.Method == http.MethodGet {
		rows, e := a.db.Query(`SELECT id,user_id,user_name,note,created_at FROM inbox_notes WHERE tenant_id=? AND chat_jid=? ORDER BY id DESC LIMIT 100`, tid, chat)
		if e != nil {
			writeError(w, e, 500)
			return
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var id, uid int64
			var name, note, created string
			if rows.Scan(&id, &uid, &name, &note, &created) == nil {
				out = append(out, map[string]any{"id": id, "user_id": uid, "user_name": name, "note": note, "created_at": created})
			}
		}
		writeJSON(w, out)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	var req struct {
		Note string `json:"note"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || strings.TrimSpace(req.Note) == "" {
		writeError(w, errors.New("nota obligatoria"), 400)
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = a.db.Exec(`INSERT INTO inbox_notes(tenant_id,chat_jid,user_id,user_name,note,created_at) VALUES(?,?,?,?,?,?)`, tid, chat, u.ID, u.Name, strings.TrimSpace(req.Note), now)
	if err != nil {
		writeError(w, err, 500)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (a *App) inboxTeamHandler(w http.ResponseWriter, r *http.Request) {
	tid, _, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	rows, e := a.db.Query(`SELECT id,name,email,role FROM app_users WHERE tenant_id=? AND active=1 ORDER BY name`, tid)
	if e != nil {
		writeError(w, e, 500)
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id int64
		var n, em, role string
		if rows.Scan(&id, &n, &em, &role) == nil {
			out = append(out, map[string]any{"id": id, "name": n, "email": em, "role": role})
		}
	}
	writeJSON(w, out)
}
