package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// An unauthenticated request must be rejected BEFORE body validation or AI
// streaming. This test must use production's real authentication behavior.
func TestAgentBuilderA3RejectsUnauthenticated(t *testing.T) {
	a := &App{}
	r := httptest.NewRequest(http.MethodPost, "/api/agents/builder-a1", bytes.NewBufferString(`{"mode":"stream","messages":[{"role":"user","content":"Hola"}]}`))
	w := httptest.NewRecorder()
	a.agentBuilderA1Handler(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated request: expected 401, got %d: %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), `"type":"delta"`) {
		t.Fatalf("unauthenticated request must not stream AI data: %s", w.Body.String())
	}
}

// The formatter is a PRIVATE helper, not an HTTP route. Test it with a fake
// upstream source instead of disabling application authentication.
func TestAgentBuilderA3StreamingYieldsSeparateChunks(t *testing.T) {
	w := httptest.NewRecorder()
	agentBuilderA3WriteStream(w, context.Background(), func(emit func(string) error) error {
		if err := emit("Hola "); err != nil {
			return err
		}
		return emit("desde Worktic AI")
	})
	if w.Code != http.StatusOK {
		t.Fatalf("response code: %d body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Header().Get("Content-Type"), "application/x-ndjson") {
		t.Fatalf("content type: %s", w.Header().Get("Content-Type"))
	}
	if !w.Flushed {
		t.Fatal("expected response flush")
	}
	lines := strings.Split(strings.TrimSpace(w.Body.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected two deltas + done, got %d: %s", len(lines), w.Body.String())
	}
	expectedTypes := []string{"delta", "delta", "done"}
	expectedValues := []string{"Hola ", "desde Worktic AI", "ok"}
	for i, line := range lines {
		var event struct {
			Type  string `json:"type"`
			Value string `json:"value"`
		}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatal(err)
		}
		if event.Type != expectedTypes[i] || event.Value != expectedValues[i] {
			t.Fatalf("event %d: got %#v", i, event)
		}
	}
}

func TestAgentBuilderA3StreamHidesProviderErrors(t *testing.T) {
	w := httptest.NewRecorder()
	agentBuilderA3WriteStream(w, context.Background(), func(emit func(string) error) error {
		return errors.New("private API key and provider details MUST NOT escape")
	})
	text := w.Body.String()
	if !strings.Contains(text, `"type":"error"`) {
		t.Fatalf("missing safe error event: %s", text)
	}
	if strings.Contains(text, "private API key") {
		t.Fatalf("upstream secret leaked: %s", text)
	}
}

func TestAgentBuilderA3StreamCapsUnicodeOutput(t *testing.T) {
	w := httptest.NewRecorder()
	agentBuilderA3WriteStream(w, context.Background(), func(emit func(string) error) error {
		return emit(strings.Repeat("á", 2500))
	})
	line := strings.Split(strings.TrimSpace(w.Body.String()), "\n")[0]
	var event struct {
		Type  string `json:"type"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		t.Fatal(err)
	}
	if len([]rune(event.Value)) != 2400 {
		t.Fatalf("expected 2400 runes; got %d", len([]rune(event.Value)))
	}
}
