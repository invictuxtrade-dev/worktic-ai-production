package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestEmailP1C2ChatJIDIsolation(t *testing.T) {
	a := emailP1C2ChatJID(1, 1, "thread1", "p1", "lead@example.com")
	if !strings.HasPrefix(a, "email:1:1:") {
		t.Fatalf("prefijo incorrecto %q", a)
	}
	if a != emailP1C2ChatJID(1, 1, "thread1", "p1", "LEAD@example.com") {
		t.Fatal("remitente casefold no estable")
	}
	if a == emailP1C2ChatJID(2, 1, "thread1", "p1", "lead@example.com") {
		t.Fatal("mezcla tenants")
	}
	if a == emailP1C2ChatJID(1, 2, "thread1", "p1", "lead@example.com") {
		t.Fatal("mezcla buzones")
	}
	if a == emailP1C2ChatJID(1, 1, "thread2", "p1", "lead@example.com") {
		t.Fatal("mezcla hilos")
	}
	if a == emailP1C2ChatJID(1, 1, "thread1", "p1", "other@example.com") {
		t.Fatal("mezcla remitentes")
	}
	if b := emailP1C2ChatJID(1, 1, "", "message01", "lead@example.com"); b == a {
		t.Fatal("fallback invalido")
	}
}

func TestEmailP1C2FlagFailClosed(t *testing.T) {
	t.Setenv("WORKTIC_EMAIL_P1C2_ENABLED", "true")
	t.Setenv("WORKTIC_EMAIL_P1C1_ENABLED", "true")
	t.Setenv("WORKTIC_EMAIL_P1B_FAST_SYNC_ENABLED", "false")
	if emailP1C2Enabled(1, "pilot@example.com") {
		t.Fatal("se habilitó sin fast sync")
	}
	t.Setenv("WORKTIC_EMAIL_P1B_FAST_SYNC_ENABLED", "true")
	t.Setenv("WORKTIC_EMAIL_P1B_ALLOWED_ACCOUNT_IDS", "1")
	t.Setenv("WORKTIC_EMAIL_P1B_ALLOWED_EMAILS", "pilot@example.com")
	if !emailP1C2Enabled(1, "pilot@example.com") {
		t.Fatal("piloto válido rechazado")
	}
	if emailP1C2Enabled(2, "pilot@example.com") || emailP1C2Enabled(1, "other@example.com") {
		t.Fatal("piloto ajeno autorizado")
	}
	t.Setenv("WORKTIC_EMAIL_P1C2_ENABLED", "false")
	if emailP1C2Enabled(1, "pilot@example.com") {
		t.Fatal("flag apagado no se respeta")
	}
}

// Regression R1: SQLite cannot resolve c.channel from an outer query in an
// ORDER BY clause inside a correlated SELECT. The audit suite caught it.
func TestEmailP1C2ConversationListSQLiteRegression(t *testing.T) {
	a := setupInboxAuditTest(t)
	owner := &User{ID: 1, Name: "Administrador A", Role: "owner"}
	// The inbox audit fixture contains a Messenger conversation. It must
	// remain visible as before the email integration.
	r := requestInboxAudit(t, a.conversationsHandler, http.MethodGet, "/api/conversations", nil, owner)
	if r.Code != http.StatusOK {
		t.Fatalf("list conversations failed on sqlite: %d %s", r.Code, r.Body.String())
	}
	var convs []Conversation
	if err := json.Unmarshal(r.Body.Bytes(), &convs); err != nil {
		t.Fatalf("decode conversations: %v: %s", err, r.Body.String())
	}
	foundMessenger := false
	for _, c := range convs {
		if c.ChatJID == "messenger:test" {
			foundMessenger = true
			if c.LastText != "Hola" {
				t.Fatalf("messenger last text changed: %q", c.LastText)
			}
		}
	}
	if !foundMessenger {
		t.Fatal("messenger contact missing from inbox")
	}

	// Insert email messages out of reception order to ensure last activity
	// is determined by the Gmail timestamp, not by internal insertion ID.
	chat := "email:11:1:regression"
	if _, e := a.db.Exec(`INSERT INTO worktic_contacts(tenant_id,chat_jid,channel,phone,name,updated_at) VALUES(11,?,'email','','Correo piloto','2026-11-11T15:00:00Z')`, chat); e != nil {
		t.Fatal(e)
	}
	for _, m := range []struct{ txt, when string }{{"Correo reciente", "2026-11-11T15:00:00Z"}, {"Correo antiguo", "2026-11-10T12:00:00Z"}} {
		if _, e := a.db.Exec(`INSERT INTO worktic_messages(tenant_id,chat_jid,channel,wa_id,sender_jid,direction,message_type,text,status,timestamp) VALUES(11,?,'email','wa-regression','lead@example.org','in','text',?,'received',?)`, chat, m.txt, m.when); e != nil {
			t.Fatal(e)
		}
	}
	r = requestInboxAudit(t, a.conversationsHandler, http.MethodGet, "/api/conversations", nil, owner)
	if r.Code != http.StatusOK {
		t.Fatalf("email inbox listing failed: %d %s", r.Code, r.Body.String())
	}
	if err := json.Unmarshal(r.Body.Bytes(), &convs); err != nil {
		t.Fatal(err)
	}
	for _, c := range convs {
		if c.ChatJID == chat {
			if c.LastText != "Correo reciente" || c.LastTimestamp != "2026-11-11T15:00:00Z" {
				t.Fatalf("email chronology failed: %q %q", c.LastText, c.LastTimestamp)
			}
			return
		}
	}
	t.Fatal("email chat missing from inbox")
}
