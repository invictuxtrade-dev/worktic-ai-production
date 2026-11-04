package main

import (
	"testing"
	"time"
)

func TestWACloudAIQueueEligibilityV273609(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	at := now.Add(-15 * time.Second).Format(time.RFC3339)
	if !waCloudAIQueueEligibleV273609(59, "text", "Quiero saber los planes", at, now) {
		t.Fatal("mensaje entrante reciente debía ser elegible")
	}
	tests := []struct {
		title          string
		agent          int64
		kind, text, ts string
	}{
		{"sin agente", 0, "text", "hola", at},
		{"imagen", 59, "image", "[image]", at},
		{"vacío", 59, "text", " ", at},
		{"baja", 59, "text", "STOP", at},
		{"atrasado", 59, "text", "hola", now.Add(-15 * time.Minute).Format(time.RFC3339)},
		{"futuro", 59, "text", "hola", now.Add(7 * time.Minute).Format(time.RFC3339)},
		{"timestamp vacío", 59, "text", "hola", ""},
	}
	for _, tc := range tests {
		t.Run(tc.title, func(t *testing.T) {
			if waCloudAIQueueEligibleV273609(tc.agent, tc.kind, tc.text, tc.ts, now) {
				t.Fatal("evento inseguro admitido")
			}
		})
	}
}
