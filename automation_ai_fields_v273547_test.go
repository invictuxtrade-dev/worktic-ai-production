package main

import "testing"

func TestAutomationAIFieldsV273547(t *testing.T) {
	state := map[string]any{"event": map[string]any{"name": "Carlos", "priority": "evento"}, "ai": map[string]any{"intent": "compra", "priority": "alta", "reply": "recomendar"}, "t2": map[string]any{"intent": "cita"}}
	for key, want := range map[string]string{"name": "Carlos", "priority": "evento", "ai.intent": "compra", "ai.priority": "alta", "ai.reply": "recomendar", "t2.intent": "cita", "missing": "", "event.name": "", "ai.missing": ""} {
		if got := automationStateValueV273547(state, key); got != want {
			t.Fatalf("%s=%q want %q", key, got, want)
		}
	}
}
func TestAutomationAIClassificationV273547(t *testing.T) {
	ok := []string{`{"intent":"compra","priority":"alta","summary":"Quiere precio"}`, "```json\n{\"intent\":\"cita\",\"priority\":\"media\",\"summary\":\"Consulta\"}\n```"}
	for _, raw := range ok {
		got, err := automationParseAIClassificationV273547(raw)
		if err != nil || got["intent"] == "" {
			t.Fatalf("valid: %v %v", got, err)
		}
	}
	bad := []string{`mal`, `{"intent":"x","priority":"urgente","summary":"x"}`, `{"intent":"x","priority":"baja","summary":"x","token":"abc"}`, `{"intent":"x","priority":"baja","summary":"x"} basura`, `{"intent":"x","priority":"baja"}`}
	for _, raw := range bad {
		if _, err := automationParseAIClassificationV273547(raw); err == nil {
			t.Fatalf("should reject %s", raw)
		}
	}
}
