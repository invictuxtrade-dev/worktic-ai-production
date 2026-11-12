package main

import (
	"path/filepath"
	"testing"
)

func TestT3RemoveEmailAccountByTenantWithoutDeletingHistory(t *testing.T) {
	db, err := openAppDB("sqlite", filepath.Join(t.TempDir(), "email-t3.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = emailP1Schema(db); err != nil {
		t.Fatal(err)
	}
	if err = emailP1D3Schema(db); err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO email_p1_accounts(id,tenant_id,provider,email,encrypted_token,token_expiry,status,created_at,updated_at)
      VALUES (101,7,'google','uno@example.com','secret1','','connected','now','now'),
             (102,7,'google','dos@example.com','secret2','','disconnected','now','now'),
             (103,8,'google','otro@example.com','secret3','','connected','now','now');
      INSERT INTO email_p1d3_accounts(tenant_id,account_id,sync_enabled,enabled_at,updated_at)
      VALUES (7,101,1,'now','now'), (8,103,1,'now','now');
      INSERT INTO email_p1_messages(tenant_id,account_id,provider_id,subject,created_at)
      VALUES (7,101,'msg-1','Histórico','now');`)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := t3SoftRemoveEmailAccount(db, 8, 101); ok || err != nil {
		t.Fatalf("unauthorized tenant: %v %v", ok, err)
	}
	if ok, err := t3SoftRemoveEmailAccount(db, 7, 101); !ok || err != nil {
		t.Fatalf("remove active: %v %v", ok, err)
	}
	if ok, err := t3SoftRemoveEmailAccount(db, 7, 102); !ok || err != nil {
		t.Fatalf("remove disconnected: %v %v", ok, err)
	}
	var status, secret string
	if err := db.QueryRow(`SELECT status,encrypted_token FROM email_p1_accounts WHERE id=101`).Scan(&status, &secret); err != nil {
		t.Fatal(err)
	}
	if status != "removed" || secret != "" {
		t.Fatalf("not removed safely: %s / %s", status, secret)
	}
	var enabled, historical, visible, other int
	if err := db.QueryRow(`SELECT sync_enabled FROM email_p1d3_accounts WHERE tenant_id=7 AND account_id=101`).Scan(&enabled); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM email_p1_messages WHERE tenant_id=7 AND account_id=101`).Scan(&historical); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM email_p1_accounts WHERE tenant_id=7 AND status<>'removed'`).Scan(&visible); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM email_p1_accounts WHERE tenant_id=8 AND status='connected' AND encrypted_token='secret3'`).Scan(&other); err != nil {
		t.Fatal(err)
	}
	if enabled != 0 || historical != 1 || visible != 0 || other != 1 {
		t.Fatalf("state enabled=%d history=%d visible=%d other=%d", enabled, historical, visible, other)
	}
	if ok, err := t3SoftRemoveEmailAccount(db, 7, 101); ok || err != nil {
		t.Fatalf("double remove: %v %v", ok, err)
	}
	// OAuth uses ON CONFLICT(...email) to restore the same ID, without restoring
	// the old synchronization permission.
	_, err = db.Exec(`INSERT INTO email_p1_accounts(tenant_id,provider,email,encrypted_token,status,created_at,updated_at)
      VALUES (7,'google','uno@example.com','new-token','connected','now','now')
      ON CONFLICT(tenant_id,provider,email) DO UPDATE SET encrypted_token=excluded.encrypted_token,status='connected'`)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT sync_enabled FROM email_p1d3_accounts WHERE tenant_id=7 AND account_id=101`).Scan(&enabled); err != nil {
		t.Fatal(err)
	}
	if enabled != 0 {
		t.Fatalf("reconnect must not auto-enable sync: %d", enabled)
	}
}
