package main

import "testing"

func TestAgentSalesHubSignalV273519(t *testing.T) {
	samples := []struct {
		message  string
		expected bool
	}{
		{"Hola, precio", false},
		{"Tienen productos?", false},
		{"Qué planes ofrecen?", false},
		{"Me interesa el Plan Negocio", true},
		{"Quiero comprar el Plan Empresa", true},
		{"Quiero una cotización para mi negocio", true},
		{"Cómo pago?", true},
	}
	for _, sample := range samples {
		ok, _, _, _ := agentSalesHubSignalV273519(sample.message)
		if ok != sample.expected {
			t.Errorf("señal %q = %v, esperado %v", sample.message, ok, sample.expected)
		}
	}
}

func TestAgentSalesHubV273519IsolationAndIdempotency(t *testing.T) {
	db, err := openAppDB("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, query := range []string{
		`CREATE TABLE ai_agents(id INTEGER,tenant_id INTEGER,status TEXT,tools TEXT)`,
		`CREATE TABLE ai_agent_profiles_v22(agent_id INTEGER,tenant_id INTEGER,crm_enabled INTEGER,tasks_enabled INTEGER,pipeline_enabled INTEGER)`,
		`INSERT INTO ai_agents VALUES(1,10,'active','opportunity')`,
		`INSERT INTO ai_agent_profiles_v22 VALUES(1,10,1,1,1)`,
		`CREATE TABLE crm_contacts(id INTEGER,tenant_id INTEGER,name TEXT,stage TEXT,deleted_at TEXT,external_id TEXT,phone TEXT NOT NULL DEFAULT '',email TEXT NOT NULL DEFAULT '')`,
		`INSERT INTO crm_contacts(id,tenant_id,name,stage,deleted_at,external_id) VALUES(2,10,'María','Nuevo','','wa:mar')`,
		`CREATE TABLE crm_opportunities(id INTEGER,tenant_id INTEGER,contact_id INTEGER,stage TEXT,deleted_at TEXT,interest TEXT,score INTEGER,manual_lock INTEGER,title TEXT,last_activity_at TEXT,source TEXT NOT NULL DEFAULT '',source_ref TEXT NOT NULL DEFAULT '',channel TEXT NOT NULL DEFAULT '',probability INTEGER NOT NULL DEFAULT 10,updated_at TEXT NOT NULL DEFAULT '',closed_at TEXT NOT NULL DEFAULT '',automation_state TEXT NOT NULL DEFAULT 'manual')`,
		`INSERT INTO crm_opportunities(id,tenant_id,contact_id,stage,deleted_at,interest,score,manual_lock,title,last_activity_at) VALUES(3,10,2,'Nuevo','','',40,1,'Oportunidad María','')`,
		`CREATE TABLE app_users(id INTEGER,tenant_id INTEGER,active INTEGER)`,
	} {
		if _, err = db.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
	if err = initOpportunitySalesHubV273518(db); err != nil {
		t.Fatal(err)
	}
	if err = initAgentSalesHubV273519(db); err != nil {
		t.Fatal(err)
	}
	a := &App{db: db}
	msg := "Me interesa el Plan Negocio"
	// Always disabled unless an administrator explicitly opts in.
	if err = a.agentSalesHubFromMessageV273519(10, 1, "wa:mar", msg, "whatsapp"); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = db.QueryRow(`SELECT COUNT(*) FROM opportunity_hub_tasks_v273518`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("opt-out: tasks=%d err=%v", count, err)
	}
	if _, err = db.Exec(`INSERT INTO agent_sales_hub_settings_v273519(tenant_id,agent_id,enabled,record_activity,create_followups,advance_stage,followup_hours) VALUES(10,1,1,1,1,1,24)`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if err = a.agentSalesHubFromMessageV273519(10, 1, "wa:mar", msg, "whatsapp"); err != nil {
			t.Fatal(err)
		}
	}
	if err = db.QueryRow(`SELECT COUNT(*) FROM opportunity_hub_tasks_v273518`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("duplicated task count=%d err=%v", count, err)
	}
	var stage string
	if err = db.QueryRow(`SELECT stage FROM crm_opportunities WHERE id=3 AND tenant_id=10`).Scan(&stage); err != nil || stage != "Nuevo" {
		t.Fatalf("manual_lock changed stage=%s err=%v", stage, err)
	}
	if err = a.agentSalesHubFromMessageV273519(11, 1, "wa:mar", msg, "whatsapp"); err != nil {
		t.Fatal(err)
	}
	if err = a.agentSalesHubFromMessageV273519(10, 1, "wa:unknown", msg, "whatsapp"); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(`SELECT COUNT(*) FROM opportunity_hub_tasks_v273518`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("cross-tenant/unknown caused change count=%d err=%v", count, err)
	}
	// A distinct message during an open follow-up must not create another task.
	if err = a.agentSalesHubFromMessageV273519(10, 1, "wa:mar", "Quiero comprar el Plan Negocio", "whatsapp"); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(`SELECT COUNT(*) FROM opportunity_hub_tasks_v273518`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("followup duplicated count=%d err=%v", count, err)
	}
	// A new follow-up may be added after the earlier one is completed.
	if _, err = db.Exec(`UPDATE opportunity_hub_tasks_v273518 SET status='done' WHERE tenant_id=10`); err != nil {
		t.Fatal(err)
	}
	if err = a.agentSalesHubFromMessageV273519(10, 1, "wa:mar", "Quiero una cotización", "whatsapp"); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(`SELECT COUNT(*) FROM opportunity_hub_tasks_v273518`).Scan(&count); err != nil || count != 2 {
		t.Fatalf("completed task didn't allow another count=%d err=%v", count, err)
	}
	// An inactive agent cannot touch opportunities even if its setting is ON.
	if _, err = db.Exec(`UPDATE ai_agents SET status='draft' WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	if err = a.agentSalesHubFromMessageV273519(10, 1, "wa:mar", "Quiero comprar ahora", "whatsapp"); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(`SELECT COUNT(*) FROM opportunity_hub_tasks_v273518`).Scan(&count); err != nil || count != 2 {
		t.Fatalf("inactive agent created task count=%d err=%v", count, err)
	}
}
