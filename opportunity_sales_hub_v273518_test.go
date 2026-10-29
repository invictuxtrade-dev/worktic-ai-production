package main

import (
	"testing"
	"time"
)

func TestSalesHubSchemaAndTenantIsolationV273518(t *testing.T) {
	db, err := openAppDB("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, q := range []string{
		`CREATE TABLE app_users(id INTEGER PRIMARY KEY,tenant_id INTEGER,active INTEGER,name TEXT,role TEXT)`,
		`CREATE TABLE crm_opportunities(id INTEGER PRIMARY KEY,tenant_id INTEGER,deleted_at TEXT)`,
		`INSERT INTO app_users VALUES(1,11,1,'Uno','agent'),(2,22,1,'Dos','agent')`,
		`INSERT INTO crm_opportunities VALUES(100,11,''),(200,22,'')`,
	} {
		if _, e := db.Exec(q); e != nil {
			t.Fatal(e)
		}
	}
	if err = initOpportunitySalesHubV273518(db); err != nil {
		t.Fatal(err)
	}
	if err = initOpportunitySalesHubV273518(db); err != nil {
		t.Fatal("migration not idempotent", err)
	}
	if !salesHubOpportunityExistsV273518(db, 11, 100) || salesHubOpportunityExistsV273518(db, 22, 100) {
		t.Fatal("cross tenant opportunity read")
	}
	if salesHubUserExistsV273518(db, 11, 2) || !salesHubUserExistsV273518(db, 11, 1) {
		t.Fatal("cross tenant user")
	}
	if salesHubCanEditV273518(db, 11, 100, 1, "agent") {
		t.Fatal("unassigned nonadmin must not modify")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if _, e := db.Exec(`INSERT INTO opportunity_hub_meta_v273518(tenant_id,opportunity_id,owner_user_id,currency,loss_reason,updated_at) VALUES(?,?,?,?,?,?)`, 11, 100, 1, "COP", "", now); e != nil {
		t.Fatal(e)
	}
	if !salesHubCanEditV273518(db, 11, 100, 1, "agent") || salesHubCanEditV273518(db, 11, 100, 2, "agent") {
		t.Fatal("owner authorization failure")
	}
	if e := salesHubAuditV273518(db, 11, 100, 1, "note.added", "Prueba"); e != nil {
		t.Fatal(e)
	}
	var count int
	if e := db.QueryRow(`SELECT COUNT(*) FROM opportunity_hub_events_v273518 WHERE tenant_id=?`, 22).Scan(&count); e != nil || count != 0 {
		t.Fatal("audit leak", count, e)
	}
}
func TestSalesHubDueDateV273518(t *testing.T) {
	if _, err := salesHubDateV273518("not-a-date"); err == nil {
		t.Fatal("invalid date accepted")
	}
	if _, err := salesHubDateV273518(time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
}
