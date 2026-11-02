package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// V27.35.36: filter incoming events by real producer metadata. This extends
// B1 stage filters and keeps existing workflows with {} fully compatible.
// Missing keys must NEVER match a configured filter.
func automationTriggerMatchesV273536(raw []byte, eventType string, payload map[string]any) bool {
	if err := automationValidTriggerConfigV273536(raw, eventType); err != nil {
		return false
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "{}" || trimmed == "null" {
		return true
	}
	if eventType == "opportunity.stage_changed" {
		return automationTriggerMatchesV273535(raw, eventType, payload)
	}
	var cfg map[string]string
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return false
	}
	for key, expected := range cfg {
		expected = strings.TrimSpace(expected)
		if expected == "" {
			continue
		}
		got := strings.TrimSpace(automationEventFieldV273535(payload, key))
		if got == "" || !strings.EqualFold(got, expected) {
			return false
		}
	}
	return true
}

func automationValidTriggerConfigV273536(raw []byte, eventType string) error {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "{}" || trimmed == "null" {
		return nil
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil || cfg == nil {
		return fmt.Errorf("configuración del disparador inválida")
	}
	for key, val := range cfg {
		value, isString := val.(string)
		if !isString {
			return fmt.Errorf("el filtro %s debe ser texto", key)
		}
		value = strings.TrimSpace(value)
		switch {
		case eventType == "opportunity.stage_changed" && (key == "stage" || key == "from_stage"):
			if value != "" && !automationValidOpportunityStageV273535(value) {
				return fmt.Errorf("etapa inválida: %s", key)
			}
		case eventType == "message.received" && key == "channel":
			// These are the exact values published in WhatsApp Cloud and Messenger events.
			if value != "" && value != "whatsapp_cloud" && value != "messenger" && value != "web" {
				return fmt.Errorf("canal no disponible para automatizaciones: %s", value)
			}
		case eventType == "form.submitted" && key == "form_id":
			if value != "" {
				formID, err := strconv.ParseInt(value, 10, 64)
				if err != nil || formID <= 0 {
					return fmt.Errorf("identificador de formulario inválido")
				}
			}
		case eventType == "social.comment" && key == "platform":
			// Exact platform values in social_posts / social.comment event.
			if value != "" && value != "facebook" && value != "instagram" && value != "youtube" {
				return fmt.Errorf("red social no disponible: %s", value)
			}
		default:
			return fmt.Errorf("filtro %s no admitido para %s", key, eventType)
		}
	}
	return nil
}
