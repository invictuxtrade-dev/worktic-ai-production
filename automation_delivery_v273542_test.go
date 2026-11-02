package main

import (
	"errors"
	"testing"
)

func TestAutomationDeliveryOutcomeV273542(t *testing.T) {
	samples := []struct {
		name   string
		result map[string]any
		err    error
		status string
		id     int64
	}{
		{"done", map[string]any{"status": "success", "execution_id": int64(8)}, nil, "success", 8},
		{"waiting", map[string]any{"status": "waiting", "execution_id": int64(9)}, nil, "accepted", 9},
		{"failed", map[string]any{"status": "failed"}, errors.New("no autorizado"), "failed", 0},
		{"manual review", map[string]any{"status": "needs_review"}, errors.New("inconcluso"), "needs_review", 0},
		{"failed response without error", map[string]any{"status": "failed"}, nil, "failed", 0},
	}
	for _, test := range samples {
		t.Run(test.name, func(t *testing.T) {
			got, id, _ := automationDeliveryOutcomeV273542(test.result, test.err)
			if got != test.status || id != test.id {
				t.Fatalf("got %q %d, want %q %d", got, id, test.status, test.id)
			}
		})
	}
}
