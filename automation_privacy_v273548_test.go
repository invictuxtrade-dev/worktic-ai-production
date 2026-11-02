package main

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestAutomationAIEventV273548(t *testing.T) {
	event := map[string]any{"name": "Cliente Secreto", "email": "alguien@example.com", "phone": "573001234567", "access_token": "TOK-VERYPRIVATE", "text": "Cotización: escribir a alguien@example.com o al +57 300 123 4567", "score": 80, "channel": "web", "nested": map[string]any{"secret": "unknown"}}
	got, err := automationAIEventV273548(event)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(got)
	s := string(raw)
	for _, bad := range []string{"Cliente Secreto", "alguien@example.com", "573001234567", "300 123 4567", "TOK-VERYPRIVATE", "nested", "access_token", "email", "phone"} {
		if strings.Contains(s, bad) {
			t.Errorf("dato sensible expuesto: %s", bad)
		}
	}
	for _, key := range []string{"text", "score", "channel"} {
		if _, ok := got[key]; !ok {
			t.Errorf("falta campo permitido: %s", key)
		}
	}
}
func TestAutomationAIEventLimitsV273548(t *testing.T) {
	got, err := automationAIEventV273548(map[string]any{"text": strings.Repeat("X", 9000)})
	if err != nil {
		t.Fatal(err)
	}
	if len([]rune(got["text"].(string))) != 1500 {
		t.Fatalf("texto no limitado: %d", len([]rune(got["text"].(string))))
	}
	if _, err := automationAIEventV273548("invalid"); err == nil {
		t.Fatal("untyped event accepted")
	}
}
func TestAutomationAuditRedactionV273548(t *testing.T) {
	state := map[string]any{"event": map[string]any{"email": "secret@example.com", "phone": "+573001234567"}, "ai": map[string]any{"reply": "private answer"}}
	input := automationAuditInputV273548(state)
	output := automationAuditOutputV273548(map[string]any{"reply": "private answer", "priority": "alta", "match": true})
	event := automationAuditEventV273548("message.received", "chat_web", map[string]any{"text": "private chat"})
	for _, txt := range []string{input, output, event} {
		if strings.Contains(txt, "secret@example.com") || strings.Contains(txt, "private") || strings.Contains(txt, "573001234567") {
			t.Fatal("data leaked", txt)
		}
	}
	if !strings.Contains(output, `"condition_match":true`) {
		t.Fatal("condition summary missing", output)
	}
	if !strings.Contains(output, `"ai_priority":"alta"`) {
		t.Fatal("classification summary missing", output)
	}
}
func TestAutomationAuditErrorV273548(t *testing.T) {
	value := automationAuditErrorV273548(errors.New("provider fail token SUPERSECRET email private@example.com"))
	if strings.Contains(value, "SUPERSECRET") || strings.Contains(value, "private@example.com") {
		t.Fatal("private error leaked")
	}
}
