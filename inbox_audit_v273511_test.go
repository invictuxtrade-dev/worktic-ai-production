package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func setupInboxAuditTest(t *testing.T) *App {
	t.Helper()
	db, err := openAppDB("sqlite", filepath.Join(t.TempDir(), "inbox.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	_, err = db.Exec(`CREATE TABLE app_users (id INTEGER PRIMARY KEY,name TEXT,role TEXT,tenant_id INTEGER,active INTEGER DEFAULT 1);
CREATE TABLE worktic_contacts (tenant_id INTEGER NOT NULL,chat_jid TEXT NOT NULL,name TEXT NOT NULL DEFAULT '',phone TEXT NOT NULL DEFAULT '',channel TEXT NOT NULL DEFAULT 'messenger',unread INTEGER NOT NULL DEFAULT 0,updated_at TEXT NOT NULL DEFAULT '',PRIMARY KEY(tenant_id,chat_jid));
CREATE TABLE worktic_messages (id INTEGER PRIMARY KEY AUTOINCREMENT,tenant_id INTEGER NOT NULL,chat_jid TEXT NOT NULL,channel TEXT NOT NULL,wa_id TEXT NOT NULL,sender_jid TEXT NOT NULL,direction TEXT NOT NULL,message_type TEXT NOT NULL,text TEXT NOT NULL,status TEXT NOT NULL,timestamp TEXT NOT NULL);
INSERT INTO app_users(id,name,role,tenant_id) VALUES(1,'Administrador A','owner',11),(2,'Administrador B','owner',22),(3,'Agente A','agent',11);
INSERT INTO worktic_contacts(tenant_id,chat_jid,name,phone,updated_at) VALUES (11,'messenger:test','Cliente A','123','2026-10-28T12:00:00Z'),(22,'messenger:test','Cliente B','345','2026-10-28T12:00:00Z');
INSERT INTO worktic_messages(tenant_id,chat_jid,channel,wa_id,sender_jid,direction,message_type,text,status,timestamp) VALUES(11,'messenger:test','messenger','1','123','in','text','Hola','received','2026-10-28T12:00:00Z');`)
	if err != nil {
		t.Fatal(err)
	}
	if err = initInboxV21Schema(db); err != nil {
		t.Fatal(err)
	}
	if err = initInboxAuditV273511Schema(db); err != nil {
		t.Fatal(err)
	}
	return &App{db: db}
}

func requestInboxAudit(t *testing.T, handler http.HandlerFunc, method, url string, body any, u *User) *httptest.ResponseRecorder {
	t.Helper()
	data := []byte{}
	if body != nil {
		var err error
		data, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	r := httptest.NewRequest(method, url, bytes.NewReader(data))
	if u != nil {
		r = r.WithContext(context.WithValue(r.Context(), userContextKey, u))
	}
	w := httptest.NewRecorder()
	handler(w, r)
	return w
}

func TestInboxDeleteKeepsEvidenceAndIsolatesTenants(t *testing.T) {
	a := setupInboxAuditTest(t)
	ownerA := &User{ID: 1, Name: "Administrador A", Role: "owner"}
	ownerB := &User{ID: 2, Name: "Administrador B", Role: "owner"}
	agentA := &User{ID: 3, Name: "Agente A", Role: "agent"}
	path := "/api/inbox/remove"
	q := map[string]any{"chat": "messenger:test", "reason": "Prueba de eliminación con auditoría"}
	if r := requestInboxAudit(t, a.inboxRemoveHandler, http.MethodPost, path, q, agentA); r.Code != http.StatusForbidden {
		t.Fatalf("agent removed a chat: %d %s", r.Code, r.Body.String())
	}
	if r := requestInboxAudit(t, a.inboxRemoveHandler, http.MethodPost, path, q, ownerA); r.Code != http.StatusOK {
		t.Fatalf("remove status=%d body=%s", r.Code, r.Body.String())
	}
	if r := requestInboxAudit(t, a.inboxRemoveHandler, http.MethodPost, path, q, ownerA); r.Code != http.StatusConflict {
		t.Fatalf("double removal status=%d", r.Code)
	}
	// B must still see its own conversation even with the exact same chat JID.
	if r := requestInboxAudit(t, a.conversationsHandler, http.MethodGet, "/api/conversations", nil, ownerA); r.Code != http.StatusOK || r.Body.String() != "[]\n" {
		t.Fatalf("removed chat visible in A: %d %s", r.Code, r.Body.String())
	}
	if r := requestInboxAudit(t, a.conversationsHandler, http.MethodGet, "/api/conversations", nil, ownerB); r.Code != http.StatusOK || !bytes.Contains(r.Body.Bytes(), []byte("Cliente B")) {
		t.Fatalf("B lost contact: %d %s", r.Code, r.Body.String())
	}
	if r := requestInboxAudit(t, a.messagesHandler, http.MethodGet, "/api/messages?chat=messenger:test", nil, ownerA); r.Code != http.StatusOK || r.Body.String() != "[]\n" {
		t.Fatalf("deleted messages exposed: %d %s", r.Code, r.Body.String())
	}
	var msgCount int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM worktic_messages WHERE tenant_id=? AND chat_jid=?`, 11, "messenger:test").Scan(&msgCount); err != nil || msgCount != 1 {
		t.Fatalf("message evidence lost: n=%d err=%v", msgCount, err)
	}
	if a.inboxAIAllowed(11, "messenger:test") {
		t.Fatal("AI allowed for removed conversation")
	}
	if !a.inboxAIAllowed(22, "messenger:test") {
		t.Fatal("AI blocked on different tenant")
	}
	if r := requestInboxAudit(t, a.inboxAuditHandler, http.MethodGet, "/api/inbox/audit?chat=messenger:test", nil, ownerA); r.Code != http.StatusOK || !bytes.Contains(r.Body.Bytes(), []byte("conversation.removed")) {
		t.Fatalf("no audit evidence: %d %s", r.Code, r.Body.String())
	}
	if r := requestInboxAudit(t, a.inboxAuditHandler, http.MethodGet, "/api/inbox/audit?chat=messenger:test", nil, agentA); r.Code != http.StatusForbidden {
		t.Fatalf("agent allowed audit: %d", r.Code)
	}
	if r := requestInboxAudit(t, a.inboxRestoreHandler, http.MethodPost, "/api/inbox/restore", map[string]any{"chat": "messenger:test", "reason": "Restaurar tras revisión"}, ownerA); r.Code != http.StatusOK {
		t.Fatalf("restore %d %s", r.Code, r.Body.String())
	}
	if r := requestInboxAudit(t, a.conversationsHandler, http.MethodGet, "/api/conversations", nil, ownerA); r.Code != http.StatusOK || !bytes.Contains(r.Body.Bytes(), []byte("Cliente A")) {
		t.Fatalf("restored A chat missing: %d %s", r.Code, r.Body.String())
	}
	var n int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM inbox_audit_v273511 WHERE tenant_id=11 AND chat_jid='messenger:test'`).Scan(&n); err != nil || n != 2 {
		t.Fatalf("unexpected audit count %d %v", n, err)
	}
}

func TestInboxMetadataUpdateIsAudited(t *testing.T) {
	a := setupInboxAuditTest(t)
	u := &User{ID: 1, Name: "Administrador A", Role: "owner"}
	q := map[string]any{"status": "pending", "mode": "human", "priority": "high", "department": "Ventas"}
	r := requestInboxAudit(t, a.inboxConversationHandler, http.MethodPut, "/api/inbox/conversation?chat=messenger:test", q, u)
	if r.Code != http.StatusOK {
		t.Fatalf("meta update=%d %s", r.Code, r.Body.String())
	}
	var action, before, after string
	if err := a.db.QueryRow(`SELECT action,before_json,after_json FROM inbox_audit_v273511 WHERE tenant_id=11`).Scan(&action, &before, &after); err != nil {
		t.Fatal(err)
	}
	if action != "conversation.updated" || !bytes.Contains([]byte(before), []byte(`"status":"open"`)) || !bytes.Contains([]byte(after), []byte(`"status":"pending"`)) {
		t.Fatalf("incorrect audit %q %s %s", action, before, after)
	}
}
