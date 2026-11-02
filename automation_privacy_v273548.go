package main

import (
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// V27.35.48: minimized audit data. Raw event data is retained only by the
// operational event queue and durable wait state when required by the engine.
// Never write full event bodies or LLM replies to the execution step audit.
var automationAuditEmailV273548 = regexp.MustCompile(`(?i)\b[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}\b`)
var automationAuditPhoneV273548 = regexp.MustCompile(`(?:\+?\d[\d\s().-]{8,}\d)`)

func automationRedactTextV273548(raw string, maxRunes int) string {
	text := automationAuditEmailV273548.ReplaceAllString(raw, "[correo oculto]")
	text = automationAuditPhoneV273548.ReplaceAllString(text, "[teléfono oculto]")
	text = strings.TrimSpace(text)
	if maxRunes > 0 && utf8.RuneCountInString(text) > maxRunes {
		r := []rune(text)
		text = string(r[:maxRunes])
	}
	return text
}

// Only a documented list of non-identity fields is exposed to the analysis
// node. Contact identifiers, addresses, tokens, phone and email are excluded.
// Message text can be commercially necessary and is still potentially personal;
// email/phone patterns are redacted but this is not full PII detection.
func automationAIEventV273548(raw any) (map[string]any, error) {
	in, ok := raw.(map[string]any)
	if !ok {
		return nil, errors.New("evento IA inválido")
	}
	permitted := map[string]bool{
		"text": true, "message": true, "body": true, "description": true,
		"channel": true, "platform": true, "source": true, "score": true,
		"interest": true, "stage": true, "old_stage": true, "new_stage": true,
		"from_stage": true, "status": true, "service": true, "form_name": true,
		"intent": true, "campaign": true, "priority": true,
	}
	out := make(map[string]any)
	for key, val := range in {
		if !permitted[key] {
			continue
		}
		switch x := val.(type) {
		case string:
			limit := 140
			if key == "text" || key == "message" || key == "body" || key == "description" {
				limit = 1500
			}
			out[key] = automationRedactTextV273548(x, limit)
		case bool:
			out[key] = x
		case int, int32, int64, float64:
			out[key] = x
		case json.Number:
			if _, err := strconv.ParseFloat(string(x), 64); err == nil {
				out[key] = x
			}
		}
	}
	data, err := json.Marshal(out)
	if err != nil || len(data) > 6500 {
		return nil, errors.New("datos del evento demasiado extensos para análisis IA")
	}
	return out, nil
}

func automationAuditEventV273548(eventType, source string, payload map[string]any) string {
	result := map[string]any{"redacted": true, "event_type": automationRedactTextV273548(eventType, 60), "source": automationRedactTextV273548(source, 60), "fields_count": len(payload)}
	b, _ := json.Marshal(result)
	return string(b)
}

func automationAuditInputV273548(state map[string]any) string {
	_, hasEvent := state["event"]
	n := 0
	for k := range state {
		if k != "event" && k != "event_type" && k != "source" && !strings.HasPrefix(k, "_") {
			n++
		}
	}
	b, _ := json.Marshal(map[string]any{"redacted": true, "has_event": hasEvent, "previous_results": n})
	return string(b)
}
func automationAuditOutputV273548(out map[string]any) string {
	summary := map[string]any{"redacted": true, "result_fields": len(out)}
	if matched, ok := out["match"].(bool); ok {
		summary["condition_match"] = matched
	}
	if priority, ok := out["priority"].(string); ok && (priority == "alta" || priority == "media" || priority == "baja") {
		summary["ai_priority"] = priority
	}
	b, _ := json.Marshal(summary)
	return string(b)
}

// Keep errors actionable but do not persist uncontrolled provider response
// bodies, SQL error strings, client identifiers or returned secret values.
func automationAuditErrorV273548(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "conexión") || strings.Contains(msg, "conexion"):
		return "Revisa la conexión configurada para esta acción"
	case strings.Contains(msg, "permiso") || strings.Contains(msg, "empresa"):
		return "No se pudo verificar el permiso o la empresa de este recurso"
	case strings.Contains(msg, "agente"):
		return "Revisa el agente IA seleccionado y su configuración"
	case strings.Contains(msg, "consentimiento") || strings.Contains(msg, "ventana") || strings.Contains(msg, "whatsapp"):
		return "Envío bloqueado: revisa consentimiento y ventana de atención de WhatsApp"
	case strings.Contains(msg, "oportunidad") || strings.Contains(msg, "contacto") || strings.Contains(msg, "tarea"):
		return "Revisa los datos requeridos del contacto, oportunidad o tarea"
	case strings.Contains(msg, "espera") || strings.Contains(msg, "nodo") || strings.Contains(msg, "grafo"):
		return "Revisa la configuración y las conexiones de los bloques"
	default:
		return "La acción falló: revisa la configuración o el servicio conectado"
	}
}
