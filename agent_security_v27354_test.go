package main

import "testing"

func TestAgentTenantIsolationV27354(t *testing.T) {
	db, err := openAppDB("sqlite", "file:agent-security-test?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := initMultiAgentSchema(db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS tenants(id INTEGER PRIMARY KEY, name TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO tenants(id,name) VALUES(101,'A'),(102,'B')`); err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct {
		id, tenant int64
		status     string
	}{{1, 101, "active"}, {2, 102, "active"}, {3, 101, "draft"}} {
		_, err := db.Exec(`INSERT INTO ai_agents(id,tenant_id,name,status,created_at,updated_at) VALUES(?,?,?,?,?,?)`, row.id, row.tenant, "Test", row.status, "2026-10-28", "2026-10-28")
		if err != nil {
			t.Fatal(err)
		}
	}
	a := &App{db: db}
	if err := a.securityTenantAgentActive(101, 1); err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{2, 3, 999} {
		if err := a.securityTenantAgentActive(101, id); err == nil {
			t.Fatalf("tenant 101 accepted unauthorized or draft agent %d", id)
		}
	}
	if _, err := a.securityTenantResolveAgent(101, 2, "whatsapp", "hola"); err == nil {
		t.Fatal("foreign assigned agent accepted")
	}
	if _, err := a.securityTenantResolveAgent(101, 3, "whatsapp", "hola"); err == nil {
		t.Fatal("draft assigned agent accepted")
	}
	if _, err := a.securityTenantResolveAgent(101, 1, "whatsapp", "hola"); err != nil {
		t.Fatal(err)
	}
	// A route from tenant A to B's agent must never be followed.
	_, err = db.Exec(`INSERT INTO ai_agent_routes(tenant_id,agent_id,name,priority,enabled) VALUES(101,2,'bad',1,1)`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.securityTenantResolveAgent(101, 0, "whatsapp", "hola"); err == nil {
		t.Fatal("foreign routed agent accepted")
	}
	if err := a.securityLegacyAgentSingleTenant(); err == nil {
		t.Fatal("shared global legacy agent allowed with multiple tenants")
	}
}
