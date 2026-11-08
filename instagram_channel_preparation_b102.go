package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// B10.2 is preparation only. It does not enable Instagram messaging or AI replies.
func (a *App) instagramChannelPreparationHandler(w http.ResponseWriter, r *http.Request) {
	tid, user, err := a.tenantFor(r)
	if err != nil {
		http.Error(w, "Sesión requerida", http.StatusUnauthorized)
		return
	}
	perms := socialPermissionsFor(user)
	if !perms.View {
		http.Error(w, "Acceso denegado", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Método no permitido", 405)
		return
	}
	if r.Method == http.MethodPost {
		if !perms.ManageConnections {
			http.Error(w, "Permiso de administración requerido", 403)
			return
		}
		if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
			http.Error(w, "Contenido inválido", 415)
			return
		}
		if r.Header.Get("Origin") != "" && r.Header.Get("Origin") != "https://"+r.Host {
			http.Error(w, "Origen inválido", 403)
			return
		}
	}
	// The table contains no credentials and is keyed by both tenant and social account ID.
	_, err = a.db.Exec(`CREATE TABLE IF NOT EXISTS instagram_agent_preparation_b10 (tenant_id BIGINT NOT NULL, social_connection_id BIGINT NOT NULL, agent_id BIGINT NOT NULL DEFAULT 0, updated_at TEXT NOT NULL, PRIMARY KEY (tenant_id,social_connection_id))`)
	if err != nil {
		http.Error(w, "No se pudo inicializar la asignación", 500)
		return
	}
	if r.Method == http.MethodPost {
		var q struct {
			SocialConnectionID int64 `json:"social_connection_id"`
			AgentID            int64 `json:"agent_id"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&q) != nil || q.SocialConnectionID <= 0 || q.AgentID < 0 {
			http.Error(w, "Datos inválidos", 400)
			return
		}
		var exists int
		if a.db.QueryRow(`SELECT 1 FROM social_connections WHERE id=? AND tenant_id=? AND platform='instagram' AND status='connected' AND provider_mode='official'`, q.SocialConnectionID, tid).Scan(&exists) != nil {
			http.Error(w, "Cuenta oficial no encontrada en esta empresa", 404)
			return
		}
		if q.AgentID > 0 {
			if a.db.QueryRow(`SELECT 1 FROM ai_agents WHERE id=? AND tenant_id=? AND status='active'`, q.AgentID, tid).Scan(&exists) != nil {
				http.Error(w, "El agente no está activo en esta empresa", 400)
				return
			}
		}
		now := time.Now().UTC().Format(time.RFC3339)
		_, err = a.db.Exec(`INSERT INTO instagram_agent_preparation_b10(tenant_id,social_connection_id,agent_id,updated_at) VALUES(?,?,?,?) ON CONFLICT(tenant_id,social_connection_id) DO UPDATE SET agent_id=excluded.agent_id,updated_at=excluded.updated_at`, tid, q.SocialConnectionID, q.AgentID, now)
		if err != nil {
			http.Error(w, "No se pudo guardar la asignación", 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "social_connection_id": q.SocialConnectionID, "agent_id": q.AgentID, "messaging_enabled": false, "ai_enabled": false})
		return
	}
	rows, err := a.db.Query(`SELECT sc.id,sc.account_name,sc.external_account_id,sc.status,COALESCE(p.agent_id,0) FROM social_connections sc LEFT JOIN instagram_agent_preparation_b10 p ON p.tenant_id=sc.tenant_id AND p.social_connection_id=sc.id WHERE sc.tenant_id=? AND sc.platform='instagram' AND sc.provider_mode='official' AND sc.status='connected' ORDER BY sc.id`, tid)
	if err != nil {
		http.Error(w, "No fue posible consultar Instagram", 500)
		return
	}
	defer rows.Close()
	type item struct {
		ID                int64  `json:"id"`
		AccountName       string `json:"account_name"`
		ExternalAccountID string `json:"external_account_id"`
		Status            string `json:"status"`
		AgentID           int64  `json:"agent_id"`
	}
	result := []item{}
	for rows.Next() {
		var x item
		if rows.Scan(&x.ID, &x.AccountName, &x.ExternalAccountID, &x.Status, &x.AgentID) == nil {
			result = append(result, x)
		}
	}
	if rows.Err() != nil {
		http.Error(w, "No fue posible leer las cuentas", 500)
		return
	}
	writeJSON(w, map[string]any{"connections": result, "messaging_enabled": false, "ai_enabled": false, "phase": "B10.2-preparation", "tenant": strconv.FormatInt(tid, 10)})
}
