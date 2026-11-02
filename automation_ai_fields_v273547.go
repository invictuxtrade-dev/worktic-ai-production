package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// automationStateValueV273547 only exposes a previous AI output when explicitly
// requested as ai.<field> or <nodeID>.<field>. Unqualified fields stay sourced
// from the trigger payload, preserving all existing CRM/WhatsApp behavior.
func automationStateValueV273547(state map[string]any, field string) string {
	field = strings.TrimSpace(field)
	if field == "" || len(field) > 120 {
		return ""
	}
	if strings.Contains(field, ".") {
		parts := strings.Split(field, ".")
		if len(parts) != 2 || parts[0] == "event" || parts[0] == "" || parts[1] == "" {
			return ""
		}
		var obj map[string]any
		if parts[0] == "ai" {
			obj, _ = state["ai"].(map[string]any)
		} else {
			obj, _ = state[parts[0]].(map[string]any)
		}
		if obj == nil {
			return ""
		}
		if x, ok := obj[parts[1]]; ok && x != nil {
			return strings.TrimSpace(fmt.Sprint(x))
		}
		return ""
	}
	ev, _ := state["event"].(map[string]any)
	if x, ok := ev[field]; ok && x != nil {
		return strings.TrimSpace(fmt.Sprint(x))
	}
	return ""
}

// LLM output is untrusted. A classification must conform to a fixed schema;
// free text never silently becomes a success with fabricated structured fields.
func automationParseAIClassificationV273547(raw string) (map[string]any, error) {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "```") {
		lines := strings.Split(trimmed, "\n")
		if len(lines) >= 3 && strings.HasPrefix(lines[len(lines)-1], "```") {
			trimmed = strings.TrimSpace(strings.Join(lines[1:len(lines)-1], "\n"))
		}
	}
	if len(trimmed) > 8192 {
		return nil, errors.New("respuesta de clasificación demasiado extensa")
	}
	var x struct {
		Intent   string `json:"intent"`
		Priority string `json:"priority"`
		Summary  string `json:"summary"`
	}
	dec := json.NewDecoder(strings.NewReader(trimmed))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&x); err != nil {
		return nil, errors.New("la IA no devolvió la clasificación JSON requerida")
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, errors.New("respuesta IA contiene datos adicionales")
	}
	x.Intent = strings.TrimSpace(x.Intent)
	x.Priority = strings.ToLower(strings.TrimSpace(x.Priority))
	x.Summary = strings.TrimSpace(x.Summary)
	if x.Intent == "" || len([]rune(x.Intent)) > 80 || len([]rune(x.Summary)) > 600 || x.Summary == "" {
		return nil, errors.New("clasificación IA incompleta")
	}
	if x.Priority != "alta" && x.Priority != "media" && x.Priority != "baja" {
		return nil, errors.New("prioridad IA inválida (alta, media o baja)")
	}
	return map[string]any{"intent": x.Intent, "priority": x.Priority, "summary": x.Summary}, nil
}
