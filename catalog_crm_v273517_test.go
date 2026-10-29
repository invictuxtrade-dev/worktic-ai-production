package main

import (
	"testing"
)

func TestCatalogCRMNameMatchingV273517(t *testing.T) {
	products := []catalogCommercialItemV273516{
		{CatalogProduct: CatalogProduct{ID: 1, Name: "Plan Negocio - Worktic AI"}, Available: true},
		{CatalogProduct: CatalogProduct{ID: 2, Name: "Plan Personal (Solo Software)"}, Available: true},
		{CatalogProduct: CatalogProduct{ID: 3, Name: "Plan Empresa - Worktic AI"}, Available: false},
	}
	cases := []struct {
		msg  string
		want []int64
	}{
		{"Me interesa el Plan Negocio y quiero precio", []int64{1}},
		{"Quiero cotizar Plan Personal y Plan Negocio", []int64{1, 2}},
		{"Qué planes tienen?", nil},
		{"Quiero el Plan Empresa", nil},
	}
	for _, tc := range cases {
		got := catalogMentionedV273517(products, tc.msg)
		if len(got) != len(tc.want) {
			t.Fatalf("%q: %d results, want %d", tc.msg, len(got), len(tc.want))
		}
		for i, p := range got {
			if p.ID != tc.want[i] {
				t.Fatalf("%q: product %d, want %d", tc.msg, p.ID, tc.want[i])
			}
		}
	}
}

func TestCatalogCRMAgentOptInTenantV273517(t *testing.T) {
	db, err := openAppDB("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, stmt := range []string{
		`CREATE TABLE ai_agents(id INTEGER,tenant_id INTEGER,status TEXT,tools TEXT)`,
		`CREATE TABLE ai_agent_profiles_v22(agent_id INTEGER,tenant_id INTEGER,catalog_enabled INTEGER,crm_enabled INTEGER)`,
		`INSERT INTO ai_agents VALUES(1,10,'active','catalog,opportunity')`,
		`INSERT INTO ai_agent_profiles_v22 VALUES(1,10,1,1)`,
	} {
		if _, err = db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	if err = initCatalogCRMV273517(db); err != nil {
		t.Fatal(err)
	}
	if catalogAgentCRMAllowedV273517(db, 10, 1) {
		t.Fatal("should default off")
	}
	if _, err = db.Exec(`INSERT INTO catalog_agent_crm_settings_v273517(tenant_id,agent_id,enabled) VALUES(10,1,1)`); err != nil {
		t.Fatal(err)
	}
	if !catalogAgentCRMAllowedV273517(db, 10, 1) {
		t.Fatal("authorized agent denied")
	}
	if catalogAgentCRMAllowedV273517(db, 11, 1) {
		t.Fatal("tenant boundary bypass")
	}
	if _, err = db.Exec(`UPDATE ai_agent_profiles_v22 SET crm_enabled=0 WHERE tenant_id=10`); err != nil {
		t.Fatal(err)
	}
	if catalogAgentCRMAllowedV273517(db, 10, 1) {
		t.Fatal("CRM disabled yet auto-link allowed")
	}
	if _, err = db.Exec(`UPDATE ai_agent_profiles_v22 SET crm_enabled=1,catalog_enabled=0 WHERE tenant_id=10`); err != nil {
		t.Fatal(err)
	}
	if catalogAgentCRMAllowedV273517(db, 10, 1) {
		t.Fatal("catalog disabled yet auto-link allowed")
	}
}

func TestCatalogAutoLinkIdempotentV273517(t *testing.T) {
	db, err := openAppDB("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	statements := []string{
		`CREATE TABLE ai_agents(id INTEGER,tenant_id INTEGER,status TEXT,tools TEXT)`,
		`CREATE TABLE ai_agent_profiles_v22(agent_id INTEGER,tenant_id INTEGER,catalog_enabled INTEGER,crm_enabled INTEGER)`,
		`INSERT INTO ai_agents VALUES(1,10,'active','catalog,opportunity')`,
		`INSERT INTO ai_agent_profiles_v22 VALUES(1,10,1,1)`,
		`CREATE TABLE crm_products(id INTEGER,tenant_id INTEGER,type TEXT,name TEXT,sku TEXT,category TEXT,description TEXT,price REAL,promotional_price REAL,promotion_starts TEXT,promotion_ends TEXT,currency TEXT,stock INTEGER,unlimited_stock INTEGER,image_url TEXT,gallery_json TEXT,payment_url TEXT,delivery_info TEXT,faq TEXT,variants TEXT,locations TEXT,featured INTEGER,active INTEGER,updated_at TEXT)`,
		`INSERT INTO crm_products VALUES(100,10,'product','Plan Negocio - Worktic AI','','','','75',0,'','','USDT',10,0,'','[]','','','','','',0,1,'')`,
		`CREATE TABLE crm_contacts(id INTEGER,tenant_id INTEGER,name TEXT,stage TEXT,deleted_at TEXT,external_id TEXT,phone TEXT NOT NULL DEFAULT '',email TEXT NOT NULL DEFAULT '',updated_at TEXT NOT NULL DEFAULT '')`,
		`INSERT INTO crm_contacts(id,tenant_id,name,stage,deleted_at,external_id) VALUES(2,10,'María','Interesado','','wa:test')`,
		`CREATE TABLE crm_opportunities(id INTEGER,tenant_id INTEGER,contact_id INTEGER,stage TEXT,deleted_at TEXT,interest TEXT,score INTEGER,manual_lock INTEGER,title TEXT,last_activity_at TEXT,source TEXT NOT NULL DEFAULT '',source_ref TEXT NOT NULL DEFAULT '',channel TEXT NOT NULL DEFAULT '',probability INTEGER NOT NULL DEFAULT 10,updated_at TEXT NOT NULL DEFAULT '',closed_at TEXT NOT NULL DEFAULT '',automation_state TEXT NOT NULL DEFAULT 'manual')`,
		`INSERT INTO crm_opportunities(id,tenant_id,contact_id,stage,deleted_at,interest,score,manual_lock,title,last_activity_at) VALUES(3,10,2,'Interesado','','Interés',65,1,'Oportunidad María','')`,
	}
	for _, stmt := range statements {
		if _, err = db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	if err = initCatalogCommerceV273516(db); err != nil {
		t.Fatal(err)
	}
	if err = initCatalogCRMV273517(db); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO catalog_agent_crm_settings_v273517(tenant_id,agent_id,enabled) VALUES(10,1,1)`); err != nil {
		t.Fatal(err)
	}
	app := &App{db: db}
	for i := 0; i < 2; i++ {
		if err = app.catalogAutoLinkFromMessageV273517(10, 1, "wa:test", "Me interesa el Plan Negocio, cuanto cuesta?", "whatsapp"); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err = db.QueryRow(`SELECT COUNT(*) FROM catalog_opportunity_items_v273516 WHERE tenant_id=10 AND opportunity_id=3 AND product_id=100`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("duplicated links: %d", count)
	}
	if err = app.catalogAutoLinkFromMessageV273517(10, 1, "unknown", "Me interesa el Plan Negocio", "whatsapp"); err != nil {
		t.Fatal(err)
	}
	if err = app.catalogAutoLinkFromMessageV273517(11, 1, "wa:test", "Me interesa el Plan Negocio", "whatsapp"); err != nil {
		t.Fatal(err)
	}
	var audit int
	if err = db.QueryRow(`SELECT COUNT(*) FROM catalog_audit_v273516 WHERE tenant_id=10 AND action='auto_link_agent'`).Scan(&audit); err != nil {
		t.Fatal(err)
	}
	if audit != 1 {
		t.Fatalf("expected one audit entry, got %d", audit)
	}
}
