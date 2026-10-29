package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Agenda activity is scoped to the authenticated tenant. Entries are append-only.
func (a *App) agendaAuditV273515(tenantID int64, actor *User, appointmentID int64, action, before string, after any) {
	if tenantID <= 0 || appointmentID <= 0 {
		return
	}
	afterText := ""
	if after != nil {
		b, err := json.Marshal(after)
		if err == nil {
			afterText = string(b)
		}
	}
	actorID := int64(0)
	actorName := "Sistema"
	if actor != nil {
		actorID = actor.ID
		actorName = strings.TrimSpace(actor.Name)
		if actorName == "" {
			actorName = actor.Email
		}
	}
	_, err := a.db.Exec(`INSERT INTO agenda_appointment_audit_v273515(tenant_id,appointment_id,actor_id,actor_name,action,before_json,after_json,created_at) VALUES(?,?,?,?,?,?,?,?)`,
		tenantID, appointmentID, actorID, actorName, action, before, afterText, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		log.Printf("agenda audit write failed: %v", err)
	}
}

func (a *App) agendaSnapshotV273515(tenantID, appointmentID int64) string {
	if tenantID <= 0 || appointmentID <= 0 {
		return ""
	}
	var name, service, startsAt, status, notes string
	var duration int
	err := a.db.QueryRow(`SELECT contact_name,service,starts_at,status,notes,duration_minutes FROM crm_appointments WHERE tenant_id=? AND id=?`, tenantID, appointmentID).
		Scan(&name, &service, &startsAt, &status, &notes, &duration)
	if err != nil {
		return ""
	}
	data, _ := json.Marshal(map[string]any{"contact_name": name, "service": service, "starts_at": startsAt, "status": status, "notes": notes, "duration_minutes": duration})
	return string(data)
}

func (a *App) agendaActivityHandlerV273515(w http.ResponseWriter, r *http.Request) {
	tenant, _, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	rows, err := a.db.Query(`SELECT appointment_id,actor_id,actor_name,action,before_json,after_json,created_at FROM agenda_appointment_audit_v273515 WHERE tenant_id=? ORDER BY id DESC LIMIT ?`, tenant, limit)
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	result := make([]map[string]any, 0)
	labels := map[string]string{"create": "Cita creada", "update": "Cita actualizada", "delete": "Cita eliminada"}
	for rows.Next() {
		var appointmentID, actorID int64
		var actorName, action, before, after, created string
		if err = rows.Scan(&appointmentID, &actorID, &actorName, &action, &before, &after, &created); err != nil {
			writeError(w, err, 500)
			return
		}
		desc := labels[action]
		if desc == "" {
			desc = fmt.Sprintf("Cambio: %s", action)
		}
		var details map[string]any
		if after != "" {
			_ = json.Unmarshal([]byte(after), &details)
		} else if before != "" {
			_ = json.Unmarshal([]byte(before), &details)
		}
		if n, ok := details["contact_name"].(string); ok && strings.TrimSpace(n) != "" {
			desc += " · " + n
		}
		result = append(result, map[string]any{"appointment_id": appointmentID, "actor_id": actorID, "actor_name": actorName, "action": action, "description": desc, "created_at": created})
	}
	if err = rows.Err(); err != nil && err != sql.ErrNoRows {
		writeError(w, err, 500)
		return
	}
	writeJSON(w, result)
}
