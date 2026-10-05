package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func initWhatsAppBusinessCenterSchema(db *DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS whatsapp_business_events(
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  tenant_id INTEGER NOT NULL,
  connection_id INTEGER NOT NULL,
  event_type TEXT NOT NULL,
  external_id TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT '',
  payload_json TEXT NOT NULL DEFAULT '{}',
  created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_wa_business_events_tenant_conn ON whatsapp_business_events(tenant_id,connection_id,created_at);
CREATE TABLE IF NOT EXISTS whatsapp_template_drafts(
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  tenant_id INTEGER NOT NULL,
  connection_id INTEGER NOT NULL,
  name TEXT NOT NULL,
  language TEXT NOT NULL DEFAULT 'es',
  category TEXT NOT NULL DEFAULT 'MARKETING',
  components_json TEXT NOT NULL DEFAULT '[]',
  status TEXT NOT NULL DEFAULT 'draft',
  meta_template_id TEXT NOT NULL DEFAULT '',
  last_error TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  UNIQUE(tenant_id,connection_id,name,language)
);
CREATE INDEX IF NOT EXISTS idx_wa_template_drafts_tenant_conn ON whatsapp_template_drafts(tenant_id,connection_id,updated_at);
`)
	return err
}

func (a *App) whatsappCloudConnectionForTenant(tid int64, id int64) (ChannelConnection, error) {
	var c ChannelConnection
	err := a.db.QueryRow(`SELECT id,tenant_id,public_id,type,name,status,external_account_id,assigned_agent_id,config_json,encrypted_credentials,last_connected_at,last_disconnected_at,last_message_at,last_error,created_at,updated_at FROM channel_connections WHERE id=? AND tenant_id=? AND type='whatsapp_cloud'`, id, tid).Scan(&c.ID, &c.TenantID, &c.PublicID, &c.Type, &c.Name, &c.Status, &c.ExternalAccountID, &c.AssignedAgentID, &c.ConfigJSON, &c.EncryptedCredentials, &c.LastConnectedAt, &c.LastDisconnectedAt, &c.LastMessageAt, &c.LastError, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return c, errors.New("conexión WhatsApp Cloud no encontrada")
	}
	return c, nil
}

func parseConnectionID(r *http.Request) (int64, error) {
	raw := strings.TrimSpace(r.URL.Query().Get("connection_id"))
	if raw == "" {
		var body struct {
			ConnectionID int64 `json:"connection_id"`
		}
		if r.Body != nil && (r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodDelete) {
			// Body parsing is handled by the action-specific handlers; do not consume it here.
		}
		_ = body
		return 0, errors.New("connection_id obligatorio")
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("connection_id inválido")
	}
	return id, nil
}

func (a *App) whatsappBusinessOverviewHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", 405)
		return
	}
	tid, _, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, 409)
		return
	}
	id, err := parseConnectionID(r)
	if err != nil {
		writeError(w, err, 400)
		return
	}
	c, err := a.whatsappCloudConnectionForTenant(tid, id)
	if err != nil {
		writeError(w, err, 404)
		return
	}
	cfg := a.whatsappCloudConfigFor(c)

	since24 := time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)
	since7 := time.Now().UTC().Add(-7 * 24 * time.Hour).Format(time.RFC3339)
	since30 := time.Now().UTC().Add(-30 * 24 * time.Hour).Format(time.RFC3339)
	count := func(q string, args ...any) int { var n int; _ = a.db.QueryRow(q, args...).Scan(&n); return n }
	statusCounts := map[string]int{}
	rows, _ := a.db.Query(`SELECT status,COUNT(*) FROM worktic_messages WHERE tenant_id=? AND channel_connection_id=? AND direction='out' AND timestamp>=? GROUP BY status`, tid, id, since30)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var s string
			var n int
			_ = rows.Scan(&s, &n)
			statusCounts[s] = n
		}
	}

	tplSummary := map[string]int{}
	if cr, e := a.whatsappCloudCredentialsFor(c); e == nil && cfg.WABAID != "" {
		var out struct {
			Data []struct {
				Status string `json:"status"`
			} `json:"data"`
		}
		ep := "https://graph.facebook.com/" + a.cfg.MetaGraphVersion + "/" + url.PathEscape(cfg.WABAID) + "/message_templates?limit=250&fields=id,status"
		if a.whatsappCloudGraph(r.Context(), http.MethodGet, ep, cr.AccessToken, nil, &out) == nil {
			for _, t := range out.Data {
				tplSummary[strings.ToUpper(t.Status)]++
			}
		}
	}

	writeJSON(w, map[string]any{
		"connection": map[string]any{"id": c.ID, "name": c.Name, "status": c.Status, "waba_id": cfg.WABAID, "phone_number_id": cfg.PhoneNumberID, "display_phone_number": cfg.DisplayPhone, "verified_name": cfg.VerifiedName, "quality_rating": cfg.QualityRating, "last_connected_at": c.LastConnectedAt, "last_message_at": c.LastMessageAt, "last_error": c.LastError},
		"messages": map[string]any{
			"in_24h":       count(`SELECT COUNT(*) FROM worktic_messages WHERE tenant_id=? AND channel_connection_id=? AND direction='in' AND timestamp>=?`, tid, id, since24),
			"out_24h":      count(`SELECT COUNT(*) FROM worktic_messages WHERE tenant_id=? AND channel_connection_id=? AND direction='out' AND timestamp>=?`, tid, id, since24),
			"total_7d":     count(`SELECT COUNT(*) FROM worktic_messages WHERE tenant_id=? AND channel_connection_id=? AND timestamp>=?`, tid, id, since7),
			"total_30d":    count(`SELECT COUNT(*) FROM worktic_messages WHERE tenant_id=? AND channel_connection_id=? AND timestamp>=?`, tid, id, since30),
			"statuses_30d": statusCounts,
		},
		"contacts": map[string]any{
			"total":  count(`SELECT COUNT(*) FROM worktic_contacts WHERE tenant_id=? AND channel_connection_id=?`, tid, id),
			"unread": count(`SELECT COALESCE(SUM(unread),0) FROM worktic_contacts WHERE tenant_id=? AND channel_connection_id=?`, tid, id),
		},
		"templates":    tplSummary,
		"webhook_url":  a.cfg.BaseURL + "/webhooks/meta/whatsapp",
		"verify_token": a.cfg.WhatsAppVerifyToken,
	})
}

func (a *App) whatsappBusinessContactsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", 405)
		return
	}
	tid, _, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, 409)
		return
	}
	id, err := parseConnectionID(r)
	if err != nil {
		writeError(w, err, 400)
		return
	}
	if _, err = a.whatsappCloudConnectionForTenant(tid, id); err != nil {
		writeError(w, err, 404)
		return
	}
	rows, err := a.db.Query(`SELECT chat_jid,phone,name,unread,updated_at FROM worktic_contacts WHERE tenant_id=? AND channel_connection_id=? ORDER BY updated_at DESC LIMIT 500`, tid, id)
	if err != nil {
		writeError(w, err, 500)
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var chat, phone, name, updated string
		var unread int
		_ = rows.Scan(&chat, &phone, &name, &unread, &updated)
		out = append(out, map[string]any{"chat_jid": chat, "phone": phone, "name": name, "unread": unread, "updated_at": updated})
	}
	writeJSON(w, out)
}

func (a *App) whatsappBusinessAnalyticsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", 405)
		return
	}
	tid, _, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, 409)
		return
	}
	id, err := parseConnectionID(r)
	if err != nil {
		writeError(w, err, 400)
		return
	}
	if _, err = a.whatsappCloudConnectionForTenant(tid, id); err != nil {
		writeError(w, err, 404)
		return
	}
	days := 30
	if n, e := strconv.Atoi(r.URL.Query().Get("days")); e == nil && n >= 1 && n <= 365 {
		days = n
	}
	since := time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour).Format(time.RFC3339)
	rows, err := a.db.Query(`SELECT substr(timestamp,1,10) day,direction,status,COUNT(*) FROM worktic_messages WHERE tenant_id=? AND channel_connection_id=? AND timestamp>=? GROUP BY day,direction,status ORDER BY day`, tid, id, since)
	if err != nil {
		writeError(w, err, 500)
		return
	}
	defer rows.Close()
	byDay := map[string]map[string]int{}
	for rows.Next() {
		var day, dir, status string
		var n int
		_ = rows.Scan(&day, &dir, &status, &n)
		if byDay[day] == nil {
			byDay[day] = map[string]int{}
		}
		byDay[day][dir] += n
		if dir == "out" {
			byDay[day]["status_"+status] += n
		}
	}
	writeJSON(w, map[string]any{"days": days, "from": since, "series": byDay})
}

func (a *App) whatsappTemplateSendTestHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	tid, _, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, 409)
		return
	}
	var q struct {
		ConnectionID int64  `json:"connection_id"`
		To           string `json:"to"`
		Name         string `json:"name"`
		Language     string `json:"language"`
		Components   []any  `json:"components"`
	}
	if json.NewDecoder(r.Body).Decode(&q) != nil || q.ConnectionID <= 0 || strings.TrimSpace(q.To) == "" || strings.TrimSpace(q.Name) == "" {
		writeError(w, errors.New("connection_id, to y name son obligatorios"), 400)
		return
	}
	c, err := a.whatsappCloudConnectionForTenant(tid, q.ConnectionID)
	if err != nil {
		writeError(w, err, 404)
		return
	}
	cr, err := a.whatsappCloudCredentialsFor(c)
	if err != nil {
		writeError(w, err, 400)
		return
	}
	cfg := a.whatsappCloudConfigFor(c)
	if q.Language == "" {
		q.Language = "es"
	}
	body := map[string]any{"messaging_product": "whatsapp", "to": strings.TrimPrefix(strings.TrimSpace(q.To), "+"), "type": "template", "template": map[string]any{"name": q.Name, "language": map[string]any{"code": q.Language}}}
	if len(q.Components) > 0 {
		body["template"].(map[string]any)["components"] = q.Components
	}
	var out struct {
		Messages []struct {
			ID string `json:"id"`
		} `json:"messages"`
	}
	ep := "https://graph.facebook.com/" + a.cfg.MetaGraphVersion + "/" + url.PathEscape(cfg.PhoneNumberID) + "/messages"
	if err = a.whatsappCloudGraph(r.Context(), http.MethodPost, ep, cr.AccessToken, body, &out); err != nil {
		writeError(w, err, 502)
		return
	}
	id := ""
	if len(out.Messages) > 0 {
		id = out.Messages[0].ID
	}
	_, _ = a.db.Exec(`INSERT INTO whatsapp_business_events(tenant_id,connection_id,event_type,external_id,status,payload_json,created_at) VALUES(?,?,?,?,?,?,?)`, tid, q.ConnectionID, "template_test", id, "sent", mustJSON(body), time.Now().UTC().Format(time.RFC3339))
	writeJSON(w, map[string]any{"ok": true, "message_id": id})
}

func mustJSON(v any) string { b, _ := json.Marshal(v); return string(b) }

func (a *App) whatsappTemplateDraftsHandler(w http.ResponseWriter, r *http.Request) {
	tid, _, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, 409)
		return
	}
	switch r.Method {
	case http.MethodGet:
		id, err := parseConnectionID(r)
		if err != nil {
			writeError(w, err, 400)
			return
		}
		rows, err := a.db.Query(`SELECT id,name,language,category,components_json,status,meta_template_id,last_error,created_at,updated_at FROM whatsapp_template_drafts WHERE tenant_id=? AND connection_id=? ORDER BY updated_at DESC`, tid, id)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var did int64
			var name, lang, cat, comp, status, metaID, lastErr, created, updated string
			_ = rows.Scan(&did, &name, &lang, &cat, &comp, &status, &metaID, &lastErr, &created, &updated)
			var components any = []any{}
			_ = json.Unmarshal([]byte(comp), &components)
			out = append(out, map[string]any{"id": did, "name": name, "language": lang, "category": cat, "components": components, "status": status, "meta_template_id": metaID, "last_error": lastErr, "created_at": created, "updated_at": updated})
		}
		writeJSON(w, out)
	case http.MethodPost, http.MethodPut:
		var q struct {
			ID           int64  `json:"id"`
			ConnectionID int64  `json:"connection_id"`
			Name         string `json:"name"`
			Language     string `json:"language"`
			Category     string `json:"category"`
			Components   []any  `json:"components"`
		}
		if json.NewDecoder(r.Body).Decode(&q) != nil || q.ConnectionID <= 0 || strings.TrimSpace(q.Name) == "" {
			writeError(w, errors.New("connection_id y name son obligatorios"), 400)
			return
		}
		if _, err = a.whatsappCloudConnectionForTenant(tid, q.ConnectionID); err != nil {
			writeError(w, err, 404)
			return
		}
		if q.Language == "" {
			q.Language = "es"
		}
		if q.Category == "" {
			q.Category = "MARKETING"
		}
		now := time.Now().UTC().Format(time.RFC3339)
		comp := mustJSON(q.Components)
		if q.ID > 0 {
			_, err = a.db.Exec(`UPDATE whatsapp_template_drafts SET name=?,language=?,category=?,components_json=?,updated_at=? WHERE id=? AND tenant_id=? AND connection_id=?`, q.Name, q.Language, q.Category, comp, now, q.ID, tid, q.ConnectionID)
		} else {
			_, err = a.db.Exec(`INSERT INTO whatsapp_template_drafts(tenant_id,connection_id,name,language,category,components_json,status,created_at,updated_at) VALUES(?,?,?,?,?,?,'draft',?,?) ON CONFLICT(tenant_id,connection_id,name,language) DO UPDATE SET category=excluded.category,components_json=excluded.components_json,updated_at=excluded.updated_at`, tid, q.ConnectionID, q.Name, q.Language, q.Category, comp, now, now)
		}
		if err != nil {
			writeError(w, err, 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	case http.MethodDelete:
		did, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		if did <= 0 {
			writeError(w, errors.New("id obligatorio"), 400)
			return
		}
		_, err = a.db.Exec(`DELETE FROM whatsapp_template_drafts WHERE id=? AND tenant_id=?`, did, tid)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	default:
		http.Error(w, "Método no permitido", 405)
	}
}

func (a *App) whatsappTemplateSubmitHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	tid, _, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, 409)
		return
	}
	var q struct {
		DraftID      int64 `json:"draft_id"`
		ConnectionID int64 `json:"connection_id"`
	}
	if json.NewDecoder(r.Body).Decode(&q) != nil || q.DraftID <= 0 || q.ConnectionID <= 0 {
		writeError(w, errors.New("draft_id y connection_id son obligatorios"), 400)
		return
	}
	c, err := a.whatsappCloudConnectionForTenant(tid, q.ConnectionID)
	if err != nil {
		writeError(w, err, 404)
		return
	}
	cr, err := a.whatsappCloudCredentialsFor(c)
	if err != nil {
		writeError(w, err, 400)
		return
	}
	cfg := a.whatsappCloudConfigFor(c)
	var name, lang, cat, comp string
	if a.db.QueryRow(`SELECT name,language,category,components_json FROM whatsapp_template_drafts WHERE id=? AND tenant_id=? AND connection_id=?`, q.DraftID, tid, q.ConnectionID).Scan(&name, &lang, &cat, &comp) != nil {
		writeError(w, errors.New("borrador no encontrado"), 404)
		return
	}
	var components []any
	if json.Unmarshal([]byte(comp), &components) != nil || len(components) == 0 {
		writeError(w, errors.New("la plantilla necesita al menos un componente"), 400)
		return
	}
	payload := map[string]any{"name": name, "language": lang, "category": cat, "components": components}
	var out struct {
		ID       string `json:"id"`
		Status   string `json:"status"`
		Category string `json:"category"`
	}
	ep := "https://graph.facebook.com/" + a.cfg.MetaGraphVersion + "/" + url.PathEscape(cfg.WABAID) + "/message_templates"
	if err = a.whatsappCloudGraph(r.Context(), http.MethodPost, ep, cr.AccessToken, payload, &out); err != nil {
		_, _ = a.db.Exec(`UPDATE whatsapp_template_drafts SET status='error',last_error=?,updated_at=? WHERE id=?`, err.Error(), time.Now().UTC().Format(time.RFC3339), q.DraftID)
		writeError(w, err, 502)
		return
	}
	status := out.Status
	if status == "" {
		status = "PENDING"
	}
	_, _ = a.db.Exec(`UPDATE whatsapp_template_drafts SET status=?,meta_template_id=?,last_error='',updated_at=? WHERE id=?`, strings.ToLower(status), out.ID, time.Now().UTC().Format(time.RFC3339), q.DraftID)
	writeJSON(w, map[string]any{"ok": true, "id": out.ID, "status": status, "category": out.Category})
}

func (a *App) whatsappTemplateDeleteMetaHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Método no permitido", 405)
		return
	}
	tid, _, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, 409)
		return
	}
	cid, err := parseConnectionID(r)
	if err != nil {
		writeError(w, err, 400)
		return
	}
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		writeError(w, errors.New("name obligatorio"), 400)
		return
	}
	c, err := a.whatsappCloudConnectionForTenant(tid, cid)
	if err != nil {
		writeError(w, err, 404)
		return
	}
	cr, err := a.whatsappCloudCredentialsFor(c)
	if err != nil {
		writeError(w, err, 400)
		return
	}
	cfg := a.whatsappCloudConfigFor(c)
	ep := "https://graph.facebook.com/" + a.cfg.MetaGraphVersion + "/" + url.PathEscape(cfg.WABAID) + "/message_templates?name=" + url.QueryEscape(name)
	var out any
	if err = a.whatsappCloudGraph(r.Context(), http.MethodDelete, ep, cr.AccessToken, nil, &out); err != nil {
		writeError(w, err, 502)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "result": out})
}

func (a *App) whatsappTemplatePreviewHandler(w http.ResponseWriter, r *http.Request) {
	// Kept as a lightweight endpoint so the UI can normalize component payloads before submission.
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	var q struct {
		Name       string `json:"name"`
		Language   string `json:"language"`
		Category   string `json:"category"`
		Components []any  `json:"components"`
	}
	if json.NewDecoder(r.Body).Decode(&q) != nil {
		writeError(w, errors.New("payload inválido"), 400)
		return
	}
	if q.Language == "" {
		q.Language = "es"
	}
	if q.Category == "" {
		q.Category = "MARKETING"
	}
	writeJSON(w, map[string]any{"name": q.Name, "language": q.Language, "category": q.Category, "components": q.Components, "summary": fmt.Sprintf("%s · %s · %d componente(s)", q.Category, q.Language, len(q.Components))})
}
