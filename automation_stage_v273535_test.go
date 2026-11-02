package main

import (
	"encoding/json"
	"testing"
)

func TestStagePayloadChangedV273535(t *testing.T) {
	p, ok := automationOpportunityStagePayloadV273535(11, 25, 8, 3, "Nuevo", "Contactado", "whatsapp")
	if !ok || p["opportunity_id"] != int64(25) || p["previous_stage"] != "Nuevo" || p["stage"] != "Contactado" {
		t.Fatal(p, ok)
	}
	if _, ok := automationOpportunityStagePayloadV273535(11, 25, 8, 3, "Nuevo", "Nuevo", ""); ok {
		t.Fatal("duplicate stage must not emit")
	}
	if _, ok := automationOpportunityStagePayloadV273535(0, 25, 8, 3, "Nuevo", "Contactado", ""); ok {
		t.Fatal("invalid tenant must not emit")
	}
}
func TestTriggerFiltersV273535(t *testing.T) {
	p, _ := automationOpportunityStagePayloadV273535(11, 25, 8, 3, "Nuevo", "Contactado", "")
	cases := []struct {
		json  string
		event string
		want  bool
	}{
		{`{}`, "opportunity.stage_changed", true},
		{`{"stage":"Contactado"}`, "opportunity.stage_changed", true},
		{`{"stage":"Ganado"}`, "opportunity.stage_changed", false},
		{`{"from_stage":"Nuevo","stage":"Contactado"}`, "opportunity.stage_changed", true},
		{`{"from_stage":"Perdido"}`, "opportunity.stage_changed", false},
		{`{"stage":"Contactado"}`, "lead.created", false},
		{`{"unknown":"ignored"}`, "opportunity.stage_changed", false},
		{`{"stage":3}`, "opportunity.stage_changed", false},
		{`{invalid`, "opportunity.stage_changed", false},
	}
	for _, tt := range cases {
		if got := automationTriggerMatchesV273535([]byte(tt.json), tt.event, p); got != tt.want {
			t.Errorf("%q %s: %v want %v", tt.json, tt.event, got, tt.want)
		}
	}
}
func TestFilterValidationV273535(t *testing.T) {
	good := []string{`{}`, `{"stage":""}`, `{"stage":"Ganado","from_stage":"Nuevo"}`}
	bad := []string{`{"stage":"hacker"}`, `{"other":"value"}`, `{"stage":123}`, `{"stage":"Contactado"`}
	for _, s := range good {
		if e := automationValidTriggerConfigV273535(json.RawMessage(s), "opportunity.stage_changed"); e != nil {
			t.Errorf("unexpected %v", e)
		}
	}
	for _, s := range bad {
		if e := automationValidTriggerConfigV273535(json.RawMessage(s), "opportunity.stage_changed"); e == nil {
			t.Errorf("accepted %s", s)
		}
	}
	if e := automationValidTriggerConfigV273535(json.RawMessage(`{"stage":"Contactado"}`), "lead.created"); e == nil {
		t.Fatal("wrong trigger accepted filter")
	}
}
