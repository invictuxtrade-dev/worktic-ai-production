package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestAgendaAuditTenantIsolationV273515(t *testing.T) {
	db, err := openAppDB("sqlite", filepath.Join(t.TempDir(), "agenda_test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE crm_appointments(id INTEGER PRIMARY KEY,tenant_id INTEGER NOT NULL,contact_name TEXT NOT NULL,service TEXT NOT NULL,starts_at TEXT NOT NULL,status TEXT NOT NULL,notes TEXT NOT NULL,duration_minutes INTEGER NOT NULL);
CREATE TABLE agenda_appointment_audit_v273515(id INTEGER PRIMARY KEY AUTOINCREMENT,tenant_id INTEGER NOT NULL,appointment_id INTEGER NOT NULL,actor_id INTEGER NOT NULL,actor_name TEXT NOT NULL,action TEXT NOT NULL,before_json TEXT NOT NULL,after_json TEXT NOT NULL,created_at TEXT NOT NULL);`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO crm_appointments(id,tenant_id,contact_name,service,starts_at,status,notes,duration_minutes) VALUES(7,1,'Cita privada','Demo','2026-10-29T10:00:00','Programada','',30)`); err != nil {
		t.Fatal(err)
	}
	app := &App{db: db}
	if got := app.agendaSnapshotV273515(2, 7); got != "" {
		t.Fatalf("tenant 2 can see tenant 1 snapshot: %q", got)
	}
	got := app.agendaSnapshotV273515(1, 7)
	if !strings.Contains(got, "Cita privada") {
		t.Fatalf("expected original appointment: %q", got)
	}
	app.agendaAuditV273515(1, &User{ID: 9, Name: "Operador"}, 7, "update", got, map[string]any{"contact_name": "Cita privada"})
	var tenant, actor, appointment int64
	var who, action string
	if err = db.QueryRow(`SELECT tenant_id,actor_id,appointment_id,actor_name,action FROM agenda_appointment_audit_v273515 WHERE tenant_id=?`, 1).Scan(&tenant, &actor, &appointment, &who, &action); err != nil {
		t.Fatal(err)
	}
	if tenant != 1 || actor != 9 || appointment != 7 || who != "Operador" || action != "update" {
		t.Fatal("audit contents incorrect")
	}
	var count int
	if err = db.QueryRow(`SELECT COUNT(*) FROM agenda_appointment_audit_v273515 WHERE tenant_id=?`, 2).Scan(&count); err != nil || count != 0 {
		t.Fatalf("cross-tenant audit visibility: %v count=%d", err, count)
	}
}
