package main

import "testing"

func TestTriggerChannelFiltersV273536(t *testing.T) {
	cases := []struct {
		raw, event string
		payload    map[string]any
		matches    bool
	}{
		{`{}`, "message.received", map[string]any{"channel": "messenger"}, true},
		{`{"channel":"whatsapp_cloud"}`, "message.received", map[string]any{"channel": "whatsapp_cloud"}, true},
		{`{"channel":"messenger"}`, "message.received", map[string]any{"channel": "whatsapp_cloud"}, false},
		{`{"channel":"messenger"}`, "message.received", map[string]any{"channel": "messenger"}, true},
		{`{"channel":"messenger"}`, "message.received", map[string]any{}, false},
		{`{"channel":"messenger"}`, "lead.created", map[string]any{"channel": "messenger"}, false},
		{`{"channel":"webchat"}`, "message.received", map[string]any{"channel": "webchat"}, false},
		{`{"channel":"web"}`, "message.received", map[string]any{"channel": "web"}, true},
		{`{"channel":"web"}`, "message.received", map[string]any{"channel": "messenger"}, false},
		{`{"channel":12}`, "message.received", map[string]any{"channel": 12}, false},
		{`{"unknown":"foo"}`, "message.received", map[string]any{"unknown": "foo"}, false},
		{`{"platform":"instagram"}`, "social.comment", map[string]any{"platform": "instagram"}, true},
		{`{"platform":"facebook"}`, "social.comment", map[string]any{"platform": "youtube"}, false},
		{`{"platform":"youtube"}`, "social.comment", map[string]any{}, false},
		{`{"platform":"youtube"}`, "message.received", map[string]any{"platform": "youtube"}, false},
		{`{"stage":"Contactado"}`, "opportunity.stage_changed", map[string]any{"stage": "Contactado"}, true},
		{`{"from_stage":"Nuevo","stage":"Contactado"}`, "opportunity.stage_changed", map[string]any{"previous_stage": "Nuevo", "stage": "Contactado"}, true},
		{`{"from_stage":"Nuevo","stage":"Ganado"}`, "opportunity.stage_changed", map[string]any{"previous_stage": "Nuevo", "stage": "Contactado"}, false},
		{`{"form_id":"42"}`, "form.submitted", map[string]any{"form_id": int64(42)}, true},
		{`{"form_id":"42"}`, "form.submitted", map[string]any{"form_id": int64(12)}, false},
		{`{"form_id":"42"}`, "form.submitted", map[string]any{}, false},
		{`{"form_id":"42"}`, "lead.created", map[string]any{"form_id": int64(42)}, false},
		{`{invalid`, "message.received", nil, false},
	}
	for i, c := range cases {
		actual := automationTriggerMatchesV273536([]byte(c.raw), c.event, c.payload)
		if actual != c.matches {
			t.Errorf("case %d got %t want %t (%s)", i, actual, c.matches, c.raw)
		}
	}
}
func TestTriggerFilterValidationV273536(t *testing.T) {
	tests := []struct {
		raw, event string
		valid      bool
	}{
		{`{}`, "message.received", true},
		{`{"channel":"messenger"}`, "message.received", true},
		{`{"channel":"whatsapp_cloud"}`, "message.received", true},
		{`{"channel":"webchat"}`, "message.received", false},
		{`{"channel":"web"}`, "message.received", true},
		{`{"platform":"facebook"}`, "social.comment", true},
		{`{"platform":"youtube"}`, "social.comment", true},
		{`{"platform":"facebook"}`, "message.received", false},
		{`{"channel":"messenger"}`, "social.comment", false},
		{`{"channel":"messenger","bogus":"ok"}`, "message.received", false},
		{`{"stage":"Nuevo"}`, "opportunity.stage_changed", true},
		{`{"stage":"NoExiste"}`, "opportunity.stage_changed", false},
		{`{"channel":false}`, "message.received", false},
		{`{"form_id":"12"}`, "form.submitted", true},
		{`{"form_id":"0"}`, "form.submitted", false},
		{`{"form_id":"no"}`, "form.submitted", false},
		{`{"form_id":"12"}`, "lead.created", false},
		{`null`, "lead.created", true},
	}
	for i, x := range tests {
		err := automationValidTriggerConfigV273536([]byte(x.raw), x.event)
		if (err == nil) != x.valid {
			t.Errorf("case %d validation %v expected valid %t", i, err, x.valid)
		}
	}
}
