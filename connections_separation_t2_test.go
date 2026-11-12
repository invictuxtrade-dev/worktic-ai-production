package main

import (
	"path/filepath"
	"testing"
)

func TestT2SocialDisconnectKeepsHistoryAndSeparatesTenants(t *testing.T) {
	db, err := openAppDB("sqlite", filepath.Join(t.TempDir(), "socialt2.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = initSocialHubSchema(db); err != nil {
		t.Fatal(err)
	}
	ensureSocialProviderSchema(db)
	_, err = db.Exec(`INSERT INTO social_connections(tenant_id,platform,account_name,external_account_id,status,provider_mode,scopes,last_sync_at,last_error,created_at,updated_at,encrypted_credentials,config_json)
VALUES (7,'tiktok','Cuenta A','profilea','connected','official','user.info.basic','','','now','now','encrypted-value','{}'),
       (8,'tiktok','Cuenta B','profileb','connected','official','user.info.basic','','','now','now','encrypted-value-b','{}');
INSERT INTO social_posts (tenant_id,connection_id,platform,status,created_at,updated_at)
VALUES (7,1,'tiktok','scheduled','now','now'),(7,1,'tiktok','published','now','now'),
       (8,2,'tiktok','scheduled','now','now');`)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := t2SoftRemoveSocialConnection(db, 8, 1); ok || err != nil {
		t.Fatalf("other tenant could unlink connection: ok=%v err=%v", ok, err)
	}
	ok, err := t2SoftRemoveSocialConnection(db, 7, 1)
	if err != nil || !ok {
		t.Fatalf("unlink failed ok=%v err=%v", ok, err)
	}
	var status, enc, scopes, expires, refresh string
	if err := db.QueryRow(`SELECT status,encrypted_credentials,scopes,token_expires_at,refresh_status FROM social_connections WHERE id=1`).Scan(&status, &enc, &scopes, &expires, &refresh); err != nil {
		t.Fatal(err)
	}
	if status != "removed" || enc != "" || scopes != "" || expires != "" || refresh != "" {
		t.Fatalf("credentials not revoked: %q %q %q %q %q", status, enc, scopes, expires, refresh)
	}
	var paused, published, other string
	if err := db.QueryRow(`SELECT status FROM social_posts WHERE tenant_id=7 AND status='paused_connection'`).Scan(&paused); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT status FROM social_posts WHERE tenant_id=7 AND status='published'`).Scan(&published); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT status FROM social_posts WHERE tenant_id=8`).Scan(&other); err != nil {
		t.Fatal(err)
	}
	if paused != "paused_connection" || published != "published" || other != "scheduled" {
		t.Fatalf("history corrupted: %q %q %q", paused, published, other)
	}
	if ok, err := t2SoftRemoveSocialConnection(db, 7, 1); ok || err != nil {
		t.Fatalf("second removal not idempotent: %v %v", ok, err)
	}
}
