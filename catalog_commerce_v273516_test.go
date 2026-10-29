package main

import (
	"testing"
	"time"
)

func TestCatalogPromotionDatesV273516(t *testing.T) {
	p := CatalogProduct{Price: 100, PromotionalPrice: 75, PromotionStarts: "2026-10-01", PromotionEnds: "2026-10-31"}
	for _, tc := range []struct {
		date   string
		want   float64
		active bool
	}{
		{"2026-09-30T20:00:00Z", 100, false},
		{"2026-10-20T12:00:00Z", 75, true},
		{"2026-11-01T12:00:00Z", 100, false},
	} {
		dt, _ := time.Parse(time.RFC3339, tc.date)
		price, active := catalogPriceV273516(p, dt)
		if price != tc.want || active != tc.active {
			t.Errorf("%s: %.2f %v", tc.date, price, active)
		}
	}
}
func TestCatalogAvailabilityAndTenantMediaV273516(t *testing.T) {
	p := CatalogProduct{ID: 7, TenantID: 12, Name: "Camisa", Type: "product", Active: true, Stock: 0, Price: 99, ImageURL: "/uploads/catalog/13/secret.png", Gallery: []string{"/uploads/catalog/12/own.png", "javascript:alert(1)"}}
	x := catalogCommercialV273516(p, time.Now())
	if x.Available {
		t.Fatal("out of stock is not sellable")
	}
	if x.ImageURL != "" || len(x.Gallery) != 1 {
		t.Fatalf("media leak: %+v", x)
	}
	if catalogMediaAllowedV273516("javascript:alert(1)", 12) {
		t.Fatal("script URL allowed")
	}
}
func TestCatalogShareOnlyAvailableV273516(t *testing.T) {
	list := []catalogCommercialItemV273516{
		{CatalogProduct: CatalogProduct{ID: 1, Name: "Agotado", Currency: "COP"}, EffectivePrice: 1, Available: false},
		{CatalogProduct: CatalogProduct{ID: 2, Name: "Disponible", Currency: "COP", ImageURL: "https://example.com/image.jpg"}, EffectivePrice: 20, Available: true},
	}
	txt, imgs := catalogShareTextV273516(list, 0, 5)
	if txt == "" || len(imgs) != 1 {
		t.Fatalf("wrong share: %q %v", txt, imgs)
	}
	txt, _ = catalogShareTextV273516(list, 1, 5)
	if txt != "" {
		t.Fatal("shared unavailable item")
	}
}
func TestCatalogAgentPermissionTenantV273516(t *testing.T) {
	db, err := openAppDB("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, stmt := range []string{
		`CREATE TABLE ai_agents(id INTEGER,tenant_id INTEGER,status TEXT,tools TEXT)`,
		`CREATE TABLE ai_agent_profiles_v22(agent_id INTEGER,tenant_id INTEGER,catalog_enabled INTEGER)`,
		`INSERT INTO ai_agents VALUES(1,10,'active','contact,catalog')`,
		`INSERT INTO ai_agent_profiles_v22 VALUES(1,10,1)`,
	} {
		if _, e := db.Exec(stmt); e != nil {
			t.Fatal(e)
		}
	}
	if !catalogAgentEnabledV273516(db, 10, 1) {
		t.Fatal("enabled agent rejected")
	}
	if catalogAgentEnabledV273516(db, 11, 1) {
		t.Fatal("cross tenant leak")
	}
	if catalogAgentEnabledV273516(db, 10, 2) {
		t.Fatal("missing profile allowed")
	}
}

func TestCatalogAgentNeedsBothPermissionsV273516(t *testing.T) {
	db, err := openAppDB("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, s := range []string{
		`CREATE TABLE ai_agents(id INTEGER,tenant_id INTEGER,status TEXT,tools TEXT)`,
		`CREATE TABLE ai_agent_profiles_v22(agent_id INTEGER,tenant_id INTEGER,catalog_enabled INTEGER)`,
		`INSERT INTO ai_agents VALUES(2,22,'active','contact')`,
		`INSERT INTO ai_agent_profiles_v22 VALUES(2,22,1)`,
	} {
		if _, e := db.Exec(s); e != nil {
			t.Fatal(e)
		}
	}
	if catalogAgentEnabledV273516(db, 22, 2) {
		t.Fatal("granted catalog without agent tool")
	}
	_, _ = db.Exec(`UPDATE ai_agents SET tools='contact,catalog' WHERE id=2`)
	if !catalogAgentEnabledV273516(db, 22, 2) {
		t.Fatal("catalog tool + profile should be granted")
	}
	_, _ = db.Exec(`UPDATE ai_agent_profiles_v22 SET catalog_enabled=0 WHERE agent_id=2`)
	if catalogAgentEnabledV273516(db, 22, 2) {
		t.Fatal("V22 profile denial ignored")
	}
}
