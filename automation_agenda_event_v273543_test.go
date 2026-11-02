package main

import "testing"

func TestAgendaEventsV273543(t *testing.T) {
	before := `{"contact_name":"Ana","status":"Programada","starts_at":"2026-11-05T14:00","notes":"sensitive"}`
	cases := []struct{ action, after, kind string }{
		{"create", `{"contact_name":"Ana","status":"Programada","contact_phone":"3001002000"}`, "appointment.created"},
		{"update", `{"contact_name":"Ana","status":"Confirmada"}`, "appointment.updated"},
		{"update", `{"contact_name":"Ana","status":"Cancelada"}`, "appointment.cancelled"},
		{"delete", "", "appointment.deleted"},
	}
	for _, c := range cases {
		kind, payload, ok := automationAgendaEventV273543(77, c.action, before, c.after)
		if !ok || kind != c.kind || payload["appointment_id"] != int64(77) {
			t.Fatalf("bad event %#v %#v", kind, payload)
		}
		if _, exists := payload["notes"]; exists {
			t.Fatal("private notes must not be emitted")
		}
		if _, exists := payload["tenant_id"]; exists {
			t.Fatal("tenant sourced from DB, not payload")
		}
	}
	if _, _, ok := automationAgendaEventV273543(0, "create", "", "{}"); ok {
		t.Fatal("invalid ID should not emit")
	}
	if _, _, ok := automationAgendaEventV273543(1, "bogus", "", "{}"); ok {
		t.Fatal("unknown action should not emit")
	}
}
