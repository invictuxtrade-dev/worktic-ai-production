package main

import (
	"encoding/json"
	"strings"
)

// automationAgendaEventV273543 creates a bounded event payload without notes or
// freeform patient/customer descriptions. tenant_id belongs to the DB row, not
// to the externally provided appointment data.
func automationAgendaEventV273543(id int64, action, beforeJSON, afterJSON string) (string, map[string]any, bool) {
	if id <= 0 {
		return "", nil, false
	}
	action = strings.ToLower(strings.TrimSpace(action))
	var before, after map[string]any
	_ = json.Unmarshal([]byte(beforeJSON), &before)
	_ = json.Unmarshal([]byte(afterJSON), &after)
	read := func(m map[string]any, k string) string {
		if m == nil {
			return ""
		}
		v, ok := m[k].(string)
		if !ok {
			return ""
		}
		return strings.TrimSpace(v)
	}
	current := after
	if action == "delete" {
		current = before
	}
	kind := ""
	switch action {
	case "create":
		kind = "appointment.created"
	case "update":
		kind = "appointment.updated"
		prev := read(before, "status")
		next := read(after, "status")
		if !strings.EqualFold(prev, next) && strings.EqualFold(next, "Cancelada") {
			kind = "appointment.cancelled"
		}
	case "delete":
		kind = "appointment.deleted"
	default:
		return "", nil, false
	}
	payload := map[string]any{
		"appointment_id": id, "action": action,
		"contact_name":    read(current, "contact_name"),
		"phone":           read(current, "contact_phone"),
		"service":         read(current, "service"),
		"starts_at":       read(current, "starts_at"),
		"status":          read(current, "status"),
		"previous_status": read(before, "status"),
	}
	return kind, payload, true
}
