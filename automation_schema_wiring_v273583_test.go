package main

import "testing"

// Schema regression: a brand-new database must have the idempotent sales-hub
// receipts ledger after normal automation schema initialization.
// This test touches ONLY a local in-memory SQLite database.
func TestAutomationSalesHubSchemaWiringV273583(t *testing.T) {
	db, err := openAppDB("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for i := 0; i < 2; i++ {
		if err := initAutomationV20Schema(db); err != nil {
			t.Fatalf("schema initialization attempt %d: %v", i+1, err)
		}
	}
	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='automation_sales_hub_effects_v273545'`).Scan(&count)
	if err != nil || count != 1 {
		t.Fatalf("missing sales hub receipt table: count=%d err=%v", count, err)
	}
	result, err := db.Exec(`INSERT INTO automation_sales_hub_effects_v273545(tenant_id,execution_id,node_id,effect_type,record_id,created_at) VALUES(1,2,'assign','owner.assign',3,'2026-11-02T00:00:00Z')`)
	if err != nil {
		t.Fatalf("insert receipt: %v", err)
	}
	if changed, err := result.RowsAffected(); err != nil || changed != 1 {
		t.Fatalf("insert receipt affected=%d err=%v", changed, err)
	}
	_, err = db.Exec(`INSERT INTO automation_sales_hub_effects_v273545(tenant_id,execution_id,node_id,effect_type,record_id,created_at) VALUES(1,2,'assign','owner.assign',3,'2026-11-02T00:00:00Z')`)
	if err == nil {
		t.Fatal("duplicate side-effect receipt should be rejected")
	}
}
