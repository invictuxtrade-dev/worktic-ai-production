package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

// An opportunity event is emitted only when a human edits an existing stage.
// Automation actions deliberately do not call this helper, avoiding recursion.
func automationOpportunityStagePayloadV273535(tenant, opportunity, contact, actor int64, before, after, channel string) (map[string]any, bool) {
	before, after = strings.TrimSpace(before), strings.TrimSpace(after)
	if tenant <= 0 || opportunity <= 0 || before == after {
		return nil, false
	}
	return map[string]any{
		"opportunity_id": opportunity,
		"contact_id":     contact,
		"actor_id":       actor,
		"previous_stage": before,
		"stage":          after,
		"from_stage":     before,
		"to_stage":       after,
		"channel":        strings.TrimSpace(channel),
		"origin":         "manual_crm",
	}, true
}

// Explicit trigger filters are evaluated on the worker BEFORE executing a workflow.
// Empty filters retain backward compatibility. Invalid configs fail closed.
func automationTriggerMatchesV273535(configJSON []byte, eventType string, payload map[string]any) bool {
	raw := strings.TrimSpace(string(configJSON))
	if raw == "" || raw == "null" || raw == "{}" {
		return true
	}
	var conf map[string]any
	if json.Unmarshal(configJSON, &conf) != nil || conf == nil {
		return false
	}
	for key, expected := range conf {
		expect, ok := expected.(string)
		if !ok {
			return false
		}
		expect = strings.TrimSpace(expect)
		if expect == "" {
			continue
		}
		var actual string
		switch key {
		case "stage":
			if eventType != "opportunity.stage_changed" {
				return false
			}
			actual = automationEventFieldV273535(payload, "stage")
		case "from_stage":
			if eventType != "opportunity.stage_changed" {
				return false
			}
			actual = automationEventFieldV273535(payload, "previous_stage")
		default:
			return false
		}
		if !strings.EqualFold(strings.TrimSpace(actual), expect) {
			return false
		}
	}
	return true
}

func automationEventFieldV273535(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	v, exists := payload[key]
	if !exists || v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

func automationValidOpportunityStageV273535(stage string) bool {
	switch stage {
	case "Nuevo", "Contactado", "Interesado", "Calificado", "Cotización", "Negociación", "Ganado", "Perdido":
		return true
	}
	return false
}

func automationValidTriggerConfigV273535(configJSON []byte, eventType string) error {
	raw := strings.TrimSpace(string(configJSON))
	if raw == "" || raw == "null" || raw == "{}" {
		return nil
	}
	var conf map[string]any
	if err := json.Unmarshal(configJSON, &conf); err != nil || conf == nil {
		return fmt.Errorf("configuración del disparador inválida")
	}
	for key, raw := range conf {
		val, ok := raw.(string)
		if !ok {
			return fmt.Errorf("valor de filtro inválido: %s", key)
		}
		val = strings.TrimSpace(val)
		switch key {
		case "stage", "from_stage":
			if eventType != "opportunity.stage_changed" {
				return fmt.Errorf("filtro %s no corresponde al disparador", key)
			}
			if val != "" && !automationValidOpportunityStageV273535(val) {
				return fmt.Errorf("etapa no admitida para filtro %s", key)
			}
		default:
			return fmt.Errorf("filtro desconocido: %s", key)
		}
	}
	return nil
}
