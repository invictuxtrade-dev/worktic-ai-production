package main

import (
	"strings"
	"testing"
	"time"
)

func TestEmailP1D3FailClosedFlags(t *testing.T) {
	t.Setenv("WORKTIC_EMAIL_P1D3_SELF_SERVICE_ENABLED", "false")
	if emailP1D3Open() {
		t.Fatal("self service must fail closed")
	}
	t.Setenv("WORKTIC_EMAIL_P1D3_SELF_SERVICE_ENABLED", "true")
	if !emailP1D3Open() {
		t.Fatal("flag ignored")
	}
}
func TestEmailP1D3TenantAndAccountIsolation(t *testing.T) {
	a := setupInboxAuditTest(t)
	t.Setenv("WORKTIC_EMAIL_P1D3_SELF_SERVICE_ENABLED", "true")
	// Cuenta nueva nunca pertenece a la lista global del piloto de envío.
	t.Setenv("WORKTIC_EMAIL_P1B_ALLOWED_EMAILS", "pilot@example.com")
	t.Setenv("WORKTIC_EMAIL_P1B_ALLOWED_ACCOUNT_IDS", "999")
	if err := emailP1Schema(a.db); err != nil {
		t.Fatal(err)
	}
	if err := emailP1D3Schema(a.db); err != nil {
		t.Fatal(err)
	}
	now := emailP1Now()
	_, err := a.db.Exec(`INSERT INTO email_p1_accounts(tenant_id,provider,email,encrypted_token,created_at,updated_at) VALUES(11,'google','other@gmail.com','not-real',?,?)`, now, now)
	if err != nil {
		t.Fatal(err)
	}
	var id int64
	if err = a.db.QueryRow(`SELECT id FROM email_p1_accounts WHERE tenant_id=11 AND email='other@gmail.com'`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	_, err = a.db.Exec(`INSERT INTO email_p1d3_accounts(tenant_id,account_id,sync_enabled,updated_at) VALUES(11,?,1,?)`, id, now)
	if err != nil {
		t.Fatal(err)
	}
	if !a.emailP1D3AccountEnabled(11, id) {
		t.Fatal("allowed tenant denied")
	}
	if a.emailP1D3AccountEnabled(12, id) || a.emailP1D3AccountEnabled(11, id+100) {
		t.Fatal("tenant or account isolation bypass")
	}
	if a.emailP1D3SyncAllowed(11, id, "microsoft", "other@gmail.com") {
		t.Fatal("microsoft accidentally enabled")
	}
	t.Setenv("WORKTIC_EMAIL_P1B_FAST_SYNC_ENABLED", "true")
	t.Setenv("WORKTIC_EMAIL_P1C1_ENABLED", "true")
	t.Setenv("WORKTIC_EMAIL_P1C2_ENABLED", "true")
	t.Setenv("WORKTIC_EMAIL_P1D1_ENABLED", "true")
	if !a.emailP1C1AccountEnabled(11, id, "other@gmail.com") || !a.emailP1C2AccountEnabled(11, id, "other@gmail.com") || !a.emailP1D1AccountEnabled(11, id, "other@gmail.com") {
		t.Fatal("enabled tenant not configured")
	}
	if a.emailP1C1AccountEnabled(12, id, "other@gmail.com") {
		t.Fatal("tenant mismatch")
	}
	if emailP1D2Enabled(id, "other@gmail.com") {
		t.Fatal("new account escaped automatic-send pilot")
	}
	_, err = a.db.Exec(`UPDATE email_p1d3_accounts SET sync_enabled=0 WHERE tenant_id=11 AND account_id=?`, id)
	if err != nil || a.emailP1D3AccountEnabled(11, id) {
		t.Fatal("pause not effective", err)
	}
	t.Setenv("WORKTIC_EMAIL_P1D3_SELF_SERVICE_ENABLED", "false")
	if a.emailP1D3AccountEnabled(11, id) {
		t.Fatal("global kill switch ignored")
	}
}
func TestEmailP1D3SubjectRFC2047(t *testing.T) {
	c := emailP1D2Candidate{Tenant: 11, Account: 7, MessageID: 123, AccountEmail: "pilot@example.com", Sender: "buyer@example.com", Subject: "Prueba automática – información", InternetID: "<id@example.com>", ThreadID: "thread1"}
	raw, err := emailP1D2Mime(c, "Hola, muchas gracias por tu mensaje de interés.")
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, "Subject: =?utf-8?q?") && !strings.Contains(s, "Subject: =?utf-8?b?") {
		t.Fatalf("missing RFC 2047 encoded header: %q", s)
	}
	if strings.Contains(s, "Subject: Re: Prueba automática") {
		t.Fatal("raw UTF8 in Subject")
	}
	if !strings.Contains(s, "charset=UTF-8") || !strings.Contains(s, "\r\n\r\nHola") {
		t.Fatal("body encoding changed")
	}
}

func TestEmailP1D3NoHistoricalImport(t *testing.T) {
	at := time.Date(2026, 11, 11, 18, 0, 0, 0, time.UTC)
	if emailP1D3NewMessage("2026-11-11T17:59:59Z", at) {
		t.Fatal("old message accepted")
	}
	if !emailP1D3NewMessage("2026-11-11T18:00:01Z", at) {
		t.Fatal("new message rejected")
	}
	if emailP1D3NewMessage("invalid", at) {
		t.Fatal("bad time accepted")
	}
}
