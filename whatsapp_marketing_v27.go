package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type waMarketingAudienceV27 struct {
	Source     string `json:"source"`
	Status     string `json:"status"`
	MinScore   int    `json:"min_score"`
	CampaignID int64  `json:"campaign_id"`
}

type waMarketingCampaignInputV27 struct {
	ID                 int64                  `json:"id"`
	Name               string                 `json:"name"`
	ConnectionID       int64                  `json:"connection_id"`
	TemplateName       string                 `json:"template_name"`
	TemplateLanguage   string                 `json:"template_language"`
	VariableFields     []string               `json:"variable_fields"`
	Audience           waMarketingAudienceV27 `json:"audience"`
	ScheduledAt        string                 `json:"scheduled_at"`
	RatePerMinute      int                    `json:"rate_per_minute"`
	FollowupCampaignID int64                  `json:"followup_campaign_id"`
}

func initWhatsAppMarketingV27Schema(db *DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS whatsapp_marketing_campaigns_v27(
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 tenant_id INTEGER NOT NULL,
 created_by INTEGER NOT NULL DEFAULT 0,
 name TEXT NOT NULL,
 connection_id INTEGER NOT NULL,
 template_name TEXT NOT NULL,
 template_language TEXT NOT NULL DEFAULT 'es',
 variable_fields_json TEXT NOT NULL DEFAULT '[]',
 audience_json TEXT NOT NULL DEFAULT '{}',
 scheduled_at TEXT NOT NULL DEFAULT '',
 rate_per_minute INTEGER NOT NULL DEFAULT 20,
 followup_campaign_id INTEGER NOT NULL DEFAULT 0,
 status TEXT NOT NULL DEFAULT 'draft',
 total_recipients INTEGER NOT NULL DEFAULT 0,
 sent_count INTEGER NOT NULL DEFAULT 0,
 delivered_count INTEGER NOT NULL DEFAULT 0,
 read_count INTEGER NOT NULL DEFAULT 0,
 replied_count INTEGER NOT NULL DEFAULT 0,
 failed_count INTEGER NOT NULL DEFAULT 0,
 created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL,
 started_at TEXT NOT NULL DEFAULT '',
 completed_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_wa_marketing_campaigns_v27 ON whatsapp_marketing_campaigns_v27(tenant_id,status,scheduled_at);
CREATE TABLE IF NOT EXISTS whatsapp_marketing_recipients_v27(
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 tenant_id INTEGER NOT NULL,
 campaign_id INTEGER NOT NULL,
 lead_id INTEGER NOT NULL DEFAULT 0,
 contact_id INTEGER NOT NULL DEFAULT 0,
 name TEXT NOT NULL DEFAULT '',
 phone TEXT NOT NULL,
 status TEXT NOT NULL DEFAULT 'queued',
 meta_message_id TEXT NOT NULL DEFAULT '',
 error_message TEXT NOT NULL DEFAULT '',
 created_at TEXT NOT NULL,
 sent_at TEXT NOT NULL DEFAULT '',
 delivered_at TEXT NOT NULL DEFAULT '',
 read_at TEXT NOT NULL DEFAULT '',
 replied_at TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_wa_marketing_recipient_unique_v27 ON whatsapp_marketing_recipients_v27(tenant_id,campaign_id,phone);
CREATE INDEX IF NOT EXISTS idx_wa_marketing_recipient_status_v27 ON whatsapp_marketing_recipients_v27(tenant_id,campaign_id,status,id);
CREATE INDEX IF NOT EXISTS idx_wa_marketing_recipient_mid_v27 ON whatsapp_marketing_recipients_v27(meta_message_id);
CREATE TABLE IF NOT EXISTS whatsapp_marketing_optouts_v27(
 tenant_id INTEGER NOT NULL,
 phone TEXT NOT NULL,
 reason TEXT NOT NULL DEFAULT '',
 source TEXT NOT NULL DEFAULT 'inbound',
 created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,phone)
);
`)
	return err
}

func clampV27(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func normalizeMarketingPhoneV27(v string) string {
	var b strings.Builder
	for _, r := range v {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func parseScheduleV27(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04", "2006-01-02 15:04"} {
		if t, err := time.Parse(layout, v); err == nil {
			return t.UTC().Format(time.RFC3339)
		}
	}
	return ""
}

func (a *App) whatsappMarketingConnectionsV27(tid int64) []map[string]any {
	rows, err := a.db.Query(`SELECT id,name,external_account_id,config_json FROM channel_connections WHERE tenant_id=? AND type='whatsapp_cloud' AND status='connected' ORDER BY id`, tid)
	if err != nil {
		return []map[string]any{}
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id int64
		var name, ext, cfgRaw string
		if rows.Scan(&id, &name, &ext, &cfgRaw) != nil {
			continue
		}
		var cfg whatsappCloudConfig
		_ = json.Unmarshal([]byte(cfgRaw), &cfg)
		out = append(out, map[string]any{"id": id, "name": name, "phone_number_id": ext, "display_phone": cfg.DisplayPhone, "verified_name": cfg.VerifiedName, "quality_rating": cfg.QualityRating})
	}
	return out
}

func (a *App) whatsappMarketingOverviewV27Handler(w http.ResponseWriter, r *http.Request) {
	tid, _, err := a.tenantFor(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	var campaigns, running, recipients, sent, delivered, read, replied, failed, optouts int
	_ = a.db.QueryRow(`SELECT COUNT(*),COALESCE(SUM(CASE WHEN status IN ('running','scheduled') THEN 1 ELSE 0 END),0) FROM whatsapp_marketing_campaigns_v27 WHERE tenant_id=?`, tid).Scan(&campaigns, &running)
	_ = a.db.QueryRow(`SELECT COUNT(*),COALESCE(SUM(CASE WHEN status IN ('sent','delivered','read','replied') THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN status IN ('delivered','read','replied') THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN status IN ('read','replied') THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN status='replied' THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN status='failed' THEN 1 ELSE 0 END),0) FROM whatsapp_marketing_recipients_v27 WHERE tenant_id=?`, tid).Scan(&recipients, &sent, &delivered, &read, &replied, &failed)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM whatsapp_marketing_optouts_v27 WHERE tenant_id=?`, tid).Scan(&optouts)
	pct := func(n, d int) float64 {
		if d <= 0 {
			return 0
		}
		return float64(n) * 100 / float64(d)
	}
	writeJSON(w, map[string]any{
		"campaigns": campaigns, "active_campaigns": running, "recipients": recipients,
		"sent": sent, "delivered": delivered, "read": read, "replied": replied, "failed": failed, "optouts": optouts,
		"delivery_rate": pct(delivered, sent), "read_rate": pct(read, delivered), "reply_rate": pct(replied, delivered),
		"connections": a.whatsappMarketingConnectionsV27(tid),
	})
}

func (a *App) whatsappMarketingCampaignsV27Handler(w http.ResponseWriter, r *http.Request) {
	tid, u, err := a.tenantFor(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	switch r.Method {
	case http.MethodGet:
		rows, err := a.db.Query(`SELECT id,name,connection_id,template_name,template_language,variable_fields_json,audience_json,scheduled_at,rate_per_minute,followup_campaign_id,status,total_recipients,sent_count,delivered_count,read_count,replied_count,failed_count,created_at,updated_at,started_at,completed_at FROM whatsapp_marketing_campaigns_v27 WHERE tenant_id=? ORDER BY id DESC LIMIT 200`, tid)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var id, conn, rpm, follow int64
			var total, sent, delivered, read, replied, failed int
			var name, tpl, lang, vars, audience, scheduled, status, created, updated, started, completed string
			if rows.Scan(&id, &name, &conn, &tpl, &lang, &vars, &audience, &scheduled, &rpm, &follow, &status, &total, &sent, &delivered, &read, &replied, &failed, &created, &updated, &started, &completed) != nil {
				continue
			}
			var variableFields []string
			var audienceObj map[string]any
			_ = json.Unmarshal([]byte(vars), &variableFields)
			_ = json.Unmarshal([]byte(audience), &audienceObj)
			out = append(out, map[string]any{"id": id, "name": name, "connection_id": conn, "template_name": tpl, "template_language": lang, "variable_fields": variableFields, "audience": audienceObj, "scheduled_at": scheduled, "rate_per_minute": rpm, "followup_campaign_id": follow, "status": status, "total_recipients": total, "sent_count": sent, "delivered_count": delivered, "read_count": read, "replied_count": replied, "failed_count": failed, "created_at": created, "updated_at": updated, "started_at": started, "completed_at": completed})
		}
		writeJSON(w, out)
	case http.MethodPost, http.MethodPut:
		if u.Role != "superadmin" && u.Role != "owner" && u.Role != "admin" {
			writeError(w, errors.New("solo propietarios y administradores pueden gestionar campañas"), 403)
			return
		}
		var q waMarketingCampaignInputV27
		if json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&q) != nil {
			writeError(w, errors.New("datos inválidos"), 400)
			return
		}
		q.Name = strings.TrimSpace(q.Name)
		q.TemplateName = strings.TrimSpace(q.TemplateName)
		q.TemplateLanguage = firstNonEmpty(strings.TrimSpace(q.TemplateLanguage), "es")
		q.RatePerMinute = clampV27(q.RatePerMinute, 1, 120)
		q.ScheduledAt = parseScheduleV27(q.ScheduledAt)
		if q.Name == "" || q.ConnectionID <= 0 || q.TemplateName == "" {
			writeError(w, errors.New("nombre, conexión WhatsApp Cloud y plantilla son obligatorios"), 400)
			return
		}
		var exists int
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM channel_connections WHERE id=? AND tenant_id=? AND type='whatsapp_cloud' AND status='connected'`, q.ConnectionID, tid).Scan(&exists)
		if exists == 0 {
			writeError(w, errors.New("la conexión WhatsApp Cloud no está disponible"), 409)
			return
		}
		varsRaw, _ := json.Marshal(q.VariableFields)
		audRaw, _ := json.Marshal(q.Audience)
		now := time.Now().UTC().Format(time.RFC3339)
		if r.Method == http.MethodPost {
			res, err := a.db.Exec(`INSERT INTO whatsapp_marketing_campaigns_v27(tenant_id,created_by,name,connection_id,template_name,template_language,variable_fields_json,audience_json,scheduled_at,rate_per_minute,followup_campaign_id,status,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,'draft',?,?)`, tid, u.ID, q.Name, q.ConnectionID, q.TemplateName, q.TemplateLanguage, string(varsRaw), string(audRaw), q.ScheduledAt, q.RatePerMinute, q.FollowupCampaignID, now, now)
			if err != nil {
				writeError(w, err, 500)
				return
			}
			id, _ := res.LastInsertId()
			writeJSON(w, map[string]any{"ok": true, "id": id})
			return
		}
		if q.ID <= 0 {
			writeError(w, errors.New("id obligatorio"), 400)
			return
		}
		var status string
		if a.db.QueryRow(`SELECT status FROM whatsapp_marketing_campaigns_v27 WHERE id=? AND tenant_id=?`, q.ID, tid).Scan(&status) != nil {
			writeError(w, errors.New("campaña no encontrada"), 404)
			return
		}
		if status != "draft" && status != "paused" {
			writeError(w, errors.New("solo se pueden editar campañas en borrador o pausadas"), 409)
			return
		}
		_, err = a.db.Exec(`UPDATE whatsapp_marketing_campaigns_v27 SET name=?,connection_id=?,template_name=?,template_language=?,variable_fields_json=?,audience_json=?,scheduled_at=?,rate_per_minute=?,followup_campaign_id=?,updated_at=? WHERE id=? AND tenant_id=?`, q.Name, q.ConnectionID, q.TemplateName, q.TemplateLanguage, string(varsRaw), string(audRaw), q.ScheduledAt, q.RatePerMinute, q.FollowupCampaignID, now, q.ID, tid)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	case http.MethodDelete:
		if u.Role != "superadmin" && u.Role != "owner" && u.Role != "admin" {
			writeError(w, errors.New("sin permisos"), 403)
			return
		}
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		if id <= 0 {
			writeError(w, errors.New("id inválido"), 400)
			return
		}
		_, _ = a.db.Exec(`DELETE FROM whatsapp_marketing_recipients_v27 WHERE tenant_id=? AND campaign_id=? AND status='queued'`, tid, id)
		_, err := a.db.Exec(`DELETE FROM whatsapp_marketing_campaigns_v27 WHERE tenant_id=? AND id=? AND status='draft'`, tid, id)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	default:
		http.Error(w, "Método no permitido", 405)
	}
}

func (a *App) eligibleWhatsAppMarketingLeadsV27(tid int64, audience waMarketingAudienceV27, limit int) ([]map[string]any, error) {
	clauses := []string{"tenant_id=?", "consent=1", "phone<>''"}
	args := []any{tid}
	if s := strings.TrimSpace(audience.Source); s != "" && s != "all" {
		clauses = append(clauses, "source=?")
		args = append(args, s)
	}
	if s := strings.TrimSpace(audience.Status); s != "" && s != "all" {
		clauses = append(clauses, "status=?")
		args = append(args, s)
	}
	if audience.MinScore > 0 {
		clauses = append(clauses, "score>=?")
		args = append(args, audience.MinScore)
	}
	if audience.CampaignID > 0 {
		clauses = append(clauses, "campaign_id=?")
		args = append(args, audience.CampaignID)
	}
	q := `SELECT id,name,phone,email,source,status,score,created_at FROM marketing_leads WHERE ` + strings.Join(clauses, " AND ") + ` ORDER BY id DESC LIMIT ?`
	args = append(args, clampV27(limit, 1, 5000))
	rows, err := a.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	seen := map[string]bool{}
	for rows.Next() {
		var id int64
		var score int
		var name, phone, email, source, status, created string
		if rows.Scan(&id, &name, &phone, &email, &source, &status, &score, &created) != nil {
			continue
		}
		phone = normalizeMarketingPhoneV27(phone)
		if phone == "" || seen[phone] {
			continue
		}
		var opted int
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM whatsapp_marketing_optouts_v27 WHERE tenant_id=? AND phone=?`, tid, phone).Scan(&opted)
		if opted > 0 {
			continue
		}
		seen[phone] = true
		out = append(out, map[string]any{"lead_id": id, "name": name, "phone": phone, "email": email, "source": source, "status": status, "score": score, "created_at": created})
	}
	return out, nil
}

func (a *App) whatsappMarketingAudiencePreviewV27Handler(w http.ResponseWriter, r *http.Request) {
	tid, _, err := a.tenantFor(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	var audience waMarketingAudienceV27
	if r.Method == http.MethodPost {
		if json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&audience) != nil {
			writeError(w, errors.New("filtro inválido"), 400)
			return
		}
	}
	rows, err := a.eligibleWhatsAppMarketingLeadsV27(tid, audience, 5000)
	if err != nil {
		writeError(w, err, 500)
		return
	}
	preview := rows
	if len(preview) > 25 {
		preview = preview[:25]
	}
	writeJSON(w, map[string]any{"count": len(rows), "preview": preview, "rule": "solo leads con consent=1 y no dados de baja"})
}

func (a *App) prepareWhatsAppMarketingRecipientsV27(tid, campaignID int64) (int, error) {
	var audRaw string
	if a.db.QueryRow(`SELECT audience_json FROM whatsapp_marketing_campaigns_v27 WHERE id=? AND tenant_id=?`, campaignID, tid).Scan(&audRaw) != nil {
		return 0, errors.New("campaña no encontrada")
	}
	var audience waMarketingAudienceV27
	_ = json.Unmarshal([]byte(audRaw), &audience)
	leads, err := a.eligibleWhatsAppMarketingLeadsV27(tid, audience, 5000)
	if err != nil {
		return 0, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	for _, x := range leads {
		_, _ = a.db.Exec(`INSERT OR IGNORE INTO whatsapp_marketing_recipients_v27(tenant_id,campaign_id,lead_id,name,phone,status,created_at) VALUES(?,?,?,?,?,'queued',?)`, tid, campaignID, x["lead_id"], x["name"], x["phone"], now)
	}
	var total int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM whatsapp_marketing_recipients_v27 WHERE tenant_id=? AND campaign_id=?`, tid, campaignID).Scan(&total)
	_, _ = a.db.Exec(`UPDATE whatsapp_marketing_campaigns_v27 SET total_recipients=?,updated_at=? WHERE id=? AND tenant_id=?`, total, now, campaignID, tid)
	return total, nil
}

func (a *App) whatsappMarketingActionV27Handler(w http.ResponseWriter, r *http.Request) {
	tid, u, err := a.tenantFor(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	if u.Role != "superadmin" && u.Role != "owner" && u.Role != "admin" {
		writeError(w, errors.New("solo propietarios y administradores pueden ejecutar campañas"), 403)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	var q struct {
		ID     int64  `json:"id"`
		Action string `json:"action"`
	}
	if json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&q) != nil || q.ID <= 0 {
		writeError(w, errors.New("campaña inválida"), 400)
		return
	}
	var status, scheduled string
	if a.db.QueryRow(`SELECT status,scheduled_at FROM whatsapp_marketing_campaigns_v27 WHERE id=? AND tenant_id=?`, q.ID, tid).Scan(&status, &scheduled) != nil {
		writeError(w, errors.New("campaña no encontrada"), 404)
		return
	}
	now := time.Now().UTC()
	nowS := now.Format(time.RFC3339)
	switch strings.ToLower(strings.TrimSpace(q.Action)) {
	case "prepare":
		total, err := a.prepareWhatsAppMarketingRecipientsV27(tid, q.ID)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "recipients": total})
		return
	case "start", "resume":
		total, err := a.prepareWhatsAppMarketingRecipientsV27(tid, q.ID)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		if total == 0 {
			writeError(w, errors.New("no hay destinatarios con consentimiento que cumplan el segmento"), 409)
			return
		}
		newStatus := "running"
		if scheduled != "" {
			if t, e := time.Parse(time.RFC3339, scheduled); e == nil && t.After(now) {
				newStatus = "scheduled"
			}
		}
		_, err = a.db.Exec(`UPDATE whatsapp_marketing_campaigns_v27 SET status=?,started_at=CASE WHEN started_at='' THEN ? ELSE started_at END,updated_at=? WHERE id=? AND tenant_id=?`, newStatus, nowS, nowS, q.ID, tid)
		if err != nil {
			writeError(w, err, 500)
			return
		}
	case "pause":
		_, err = a.db.Exec(`UPDATE whatsapp_marketing_campaigns_v27 SET status='paused',updated_at=? WHERE id=? AND tenant_id=? AND status IN ('running','scheduled')`, nowS, q.ID, tid)
	case "cancel":
		_, err = a.db.Exec(`UPDATE whatsapp_marketing_campaigns_v27 SET status='cancelled',updated_at=?,completed_at=? WHERE id=? AND tenant_id=? AND status NOT IN ('completed','cancelled')`, nowS, nowS, q.ID, tid)
		_, _ = a.db.Exec(`UPDATE whatsapp_marketing_recipients_v27 SET status='skipped',error_message='campaña cancelada' WHERE tenant_id=? AND campaign_id=? AND status='queued'`, tid, q.ID)
	default:
		writeError(w, errors.New("acción no permitida"), 400)
		return
	}
	if err != nil {
		writeError(w, err, 500)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (a *App) whatsappMarketingRecipientsV27Handler(w http.ResponseWriter, r *http.Request) {
	tid, _, err := a.tenantFor(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	campaignID, _ := strconv.ParseInt(r.URL.Query().Get("campaign_id"), 10, 64)
	if campaignID <= 0 {
		writeError(w, errors.New("campaign_id obligatorio"), 400)
		return
	}
	rows, err := a.db.Query(`SELECT id,lead_id,contact_id,name,phone,status,meta_message_id,error_message,created_at,sent_at,delivered_at,read_at,replied_at FROM whatsapp_marketing_recipients_v27 WHERE tenant_id=? AND campaign_id=? ORDER BY id DESC LIMIT 500`, tid, campaignID)
	if err != nil {
		writeError(w, err, 500)
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, leadID, contactID int64
		var name, phone, status, mid, errMsg, created, sent, delivered, read, replied string
		if rows.Scan(&id, &leadID, &contactID, &name, &phone, &status, &mid, &errMsg, &created, &sent, &delivered, &read, &replied) == nil {
			out = append(out, map[string]any{"id": id, "lead_id": leadID, "contact_id": contactID, "name": name, "phone": phone, "status": status, "meta_message_id": mid, "error_message": errMsg, "created_at": created, "sent_at": sent, "delivered_at": delivered, "read_at": read, "replied_at": replied})
		}
	}
	writeJSON(w, out)
}

func (a *App) sendWhatsAppMarketingTemplateV27(campaignID, recipientID int64) error {
	var tid, connID int64
	var templateName, lang, varsRaw, name, phone string
	if a.db.QueryRow(`SELECT c.tenant_id,c.connection_id,c.template_name,c.template_language,c.variable_fields_json,r.name,r.phone FROM whatsapp_marketing_campaigns_v27 c JOIN whatsapp_marketing_recipients_v27 r ON r.campaign_id=c.id AND r.tenant_id=c.tenant_id WHERE c.id=? AND r.id=?`, campaignID, recipientID).Scan(&tid, &connID, &templateName, &lang, &varsRaw, &name, &phone) != nil {
		return errors.New("destinatario no encontrado")
	}
	var opted int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM whatsapp_marketing_optouts_v27 WHERE tenant_id=? AND phone=?`, tid, phone).Scan(&opted)
	if opted > 0 {
		_, _ = a.db.Exec(`UPDATE whatsapp_marketing_recipients_v27 SET status='skipped',error_message='baja de marketing registrada' WHERE id=?`, recipientID)
		return nil
	}
	c, err := a.whatsappCloudConnectionForTenant(tid, connID)
	if err != nil {
		return err
	}
	cr, err := a.whatsappCloudCredentialsFor(c)
	if err != nil {
		return err
	}
	cfg := a.whatsappCloudConfigFor(c)
	if cfg.PhoneNumberID == "" {
		return errors.New("Phone Number ID no configurado")
	}
	var fields []string
	_ = json.Unmarshal([]byte(varsRaw), &fields)
	params := []map[string]any{}
	for _, f := range fields {
		value := ""
		switch strings.ToLower(strings.TrimSpace(f)) {
		case "name", "nombre", "first_name":
			value = firstNonEmpty(name, "Cliente")
		case "phone", "telefono", "teléfono":
			value = phone
		default:
			value = strings.TrimSpace(f)
		}
		params = append(params, map[string]any{"type": "text", "text": value})
	}
	tpl := map[string]any{"name": templateName, "language": map[string]any{"code": firstNonEmpty(lang, "es")}}
	if len(params) > 0 {
		tpl["components"] = []map[string]any{{"type": "body", "parameters": params}}
	}
	body := map[string]any{"messaging_product": "whatsapp", "recipient_type": "individual", "to": phone, "type": "template", "template": tpl}
	var out struct {
		Messages []struct {
			ID string `json:"id"`
		} `json:"messages"`
	}
	ep := "https://graph.facebook.com/" + a.cfg.MetaGraphVersion + "/" + url.PathEscape(cfg.PhoneNumberID) + "/messages"
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	if err := a.whatsappCloudGraph(ctx, http.MethodPost, ep, cr.AccessToken, body, &out); err != nil {
		return err
	}
	if len(out.Messages) == 0 || out.Messages[0].ID == "" {
		return errors.New("Meta no devolvió ID del mensaje")
	}
	mid := out.Messages[0].ID
	now := time.Now().UTC().Format(time.RFC3339)
	_, _ = a.db.Exec(`UPDATE whatsapp_marketing_recipients_v27 SET status='sent',meta_message_id=?,sent_at=?,error_message='' WHERE id=?`, mid, now, recipientID)
	_, _ = a.db.Exec(`INSERT OR IGNORE INTO worktic_messages(tenant_id,channel_connection_id,channel,wa_id,chat_jid,sender_jid,direction,message_type,text,status,timestamp) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, tid, connID, "whatsapp_cloud", mid, "wacloud:"+phone, phone, "out", "template", "[Campaña WhatsApp: "+templateName+"]", "sent", now)
	a.recordAnalyticsEventV24(tid, "wa-marketing-sent-"+mid, "whatsapp_marketing.sent", map[string]any{"channel": "whatsapp", "source": "whatsapp_marketing", "source_ref": strconv.FormatInt(campaignID, 10), "occurred_at": now})
	return nil
}

func (a *App) refreshWhatsAppMarketingCountsV27(tid, campaignID int64) {
	var total, sent, delivered, read, replied, failed, queued int
	_ = a.db.QueryRow(`SELECT COUNT(*),COALESCE(SUM(CASE WHEN status IN ('sent','delivered','read','replied') THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN status IN ('delivered','read','replied') THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN status IN ('read','replied') THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN status='replied' THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN status='failed' THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN status='queued' THEN 1 ELSE 0 END),0) FROM whatsapp_marketing_recipients_v27 WHERE tenant_id=? AND campaign_id=?`, tid, campaignID).Scan(&total, &sent, &delivered, &read, &replied, &failed, &queued)
	now := time.Now().UTC().Format(time.RFC3339)
	_, _ = a.db.Exec(`UPDATE whatsapp_marketing_campaigns_v27 SET total_recipients=?,sent_count=?,delivered_count=?,read_count=?,replied_count=?,failed_count=?,updated_at=? WHERE id=? AND tenant_id=?`, total, sent, delivered, read, replied, failed, now, campaignID, tid)
	if queued == 0 {
		_, _ = a.db.Exec(`UPDATE whatsapp_marketing_campaigns_v27 SET status=CASE WHEN status='running' THEN 'completed' ELSE status END,completed_at=CASE WHEN status='running' THEN ? ELSE completed_at END,updated_at=? WHERE id=? AND tenant_id=?`, now, now, campaignID, tid)
	}
}

func (a *App) processWhatsAppMarketingV27() {
	now := time.Now().UTC()
	nowS := now.Format(time.RFC3339)
	rows, err := a.db.Query(`SELECT id,tenant_id,rate_per_minute,status,scheduled_at FROM whatsapp_marketing_campaigns_v27 WHERE status IN ('running','scheduled') ORDER BY id LIMIT 20`)
	if err != nil {
		return
	}
	type cinfo struct {
		id, tid           int64
		rpm               int
		status, scheduled string
	}
	items := []cinfo{}
	for rows.Next() {
		var x cinfo
		if rows.Scan(&x.id, &x.tid, &x.rpm, &x.status, &x.scheduled) == nil {
			items = append(items, x)
		}
	}
	rows.Close()
	for _, c := range items {
		if c.status == "scheduled" {
			if t, e := time.Parse(time.RFC3339, c.scheduled); e == nil && t.After(now) {
				continue
			}
			_, _ = a.db.Exec(`UPDATE whatsapp_marketing_campaigns_v27 SET status='running',started_at=CASE WHEN started_at='' THEN ? ELSE started_at END,updated_at=? WHERE id=? AND tenant_id=?`, nowS, nowS, c.id, c.tid)
		}
		cutoff := now.Add(-time.Minute).Format(time.RFC3339)
		var recent int
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM whatsapp_marketing_recipients_v27 WHERE tenant_id=? AND campaign_id=? AND sent_at>=? AND sent_at<>''`, c.tid, c.id, cutoff).Scan(&recent)
		allow := clampV27(c.rpm, 1, 120) - recent
		if allow <= 0 {
			continue
		}
		if allow > 10 {
			allow = 10
		}
		rr, err := a.db.Query(`SELECT id FROM whatsapp_marketing_recipients_v27 WHERE tenant_id=? AND campaign_id=? AND status='queued' ORDER BY id LIMIT ?`, c.tid, c.id, allow)
		if err != nil {
			continue
		}
		ids := []int64{}
		for rr.Next() {
			var id int64
			if rr.Scan(&id) == nil {
				ids = append(ids, id)
			}
		}
		rr.Close()
		for _, rid := range ids {
			if err := a.sendWhatsAppMarketingTemplateV27(c.id, rid); err != nil {
				_, _ = a.db.Exec(`UPDATE whatsapp_marketing_recipients_v27 SET status='failed',error_message=? WHERE id=?`, err.Error(), rid)
			}
		}
		a.refreshWhatsAppMarketingCountsV27(c.tid, c.id)
	}
}

func (a *App) runWhatsAppMarketingV27() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		a.processWhatsAppMarketingV27()
	}
}

func isMarketingOptOutV27(text string) bool {
	t := strings.ToLower(strings.TrimSpace(text))
	for _, x := range []string{"stop", "salir", "cancelar", "cancel", "baja", "no más", "no mas", "desuscribir", "unsubscribe"} {
		if t == x || strings.HasPrefix(t, x+" ") {
			return true
		}
	}
	return false
}

func (a *App) trackWhatsAppMarketingInboundV27(tid, connectionID int64, phone, text, at string) {
	phone = normalizeMarketingPhoneV27(phone)
	if phone == "" {
		return
	}
	if at == "" {
		at = time.Now().UTC().Format(time.RFC3339)
	}
	if isMarketingOptOutV27(text) {
		_, _ = a.db.Exec(`INSERT INTO whatsapp_marketing_optouts_v27(tenant_id,phone,reason,source,created_at) VALUES(?,?,?,'inbound',?) ON CONFLICT(tenant_id,phone) DO UPDATE SET reason=excluded.reason,source=excluded.source,created_at=excluded.created_at`, tid, phone, strings.TrimSpace(text), at)
	}
	var rid, campaignID int64
	err := a.db.QueryRow(`SELECT r.id,r.campaign_id FROM whatsapp_marketing_recipients_v27 r JOIN whatsapp_marketing_campaigns_v27 c ON c.id=r.campaign_id AND c.tenant_id=r.tenant_id WHERE r.tenant_id=? AND c.connection_id=? AND r.phone=? AND r.status IN ('sent','delivered','read') ORDER BY r.id DESC LIMIT 1`, tid, connectionID, phone).Scan(&rid, &campaignID)
	if err == nil && rid > 0 {
		_, _ = a.db.Exec(`UPDATE whatsapp_marketing_recipients_v27 SET status='replied',replied_at=? WHERE id=?`, at, rid)
		a.refreshWhatsAppMarketingCountsV27(tid, campaignID)
		a.recordAnalyticsEventV24(tid, fmt.Sprintf("wa-marketing-reply-%d-%s", rid, at), "whatsapp_marketing.replied", map[string]any{"channel": "whatsapp", "source": "whatsapp_marketing", "source_ref": strconv.FormatInt(campaignID, 10), "occurred_at": at})
	}
}

func (a *App) trackWhatsAppMarketingStatusV27(tid, connectionID int64, metaMessageID, status, at string) {
	if metaMessageID == "" {
		return
	}
	if at == "" {
		at = time.Now().UTC().Format(time.RFC3339)
	}
	var rid, campaignID int64
	if a.db.QueryRow(`SELECT id,campaign_id FROM whatsapp_marketing_recipients_v27 WHERE tenant_id=? AND meta_message_id=? ORDER BY id DESC LIMIT 1`, tid, metaMessageID).Scan(&rid, &campaignID) != nil {
		return
	}
	switch status {
	case "delivered":
		_, _ = a.db.Exec(`UPDATE whatsapp_marketing_recipients_v27 SET status=CASE WHEN status='sent' THEN 'delivered' ELSE status END,delivered_at=CASE WHEN delivered_at='' THEN ? ELSE delivered_at END WHERE id=?`, at, rid)
	case "read":
		_, _ = a.db.Exec(`UPDATE whatsapp_marketing_recipients_v27 SET status=CASE WHEN status<>'replied' THEN 'read' ELSE status END,read_at=CASE WHEN read_at='' THEN ? ELSE read_at END WHERE id=?`, at, rid)
	case "failed":
		_, _ = a.db.Exec(`UPDATE whatsapp_marketing_recipients_v27 SET status='failed',error_message='Meta reportó fallo de entrega' WHERE id=?`, rid)
	case "sent":
		_, _ = a.db.Exec(`UPDATE whatsapp_marketing_recipients_v27 SET status=CASE WHEN status='queued' THEN 'sent' ELSE status END WHERE id=?`, rid)
	}
	a.refreshWhatsAppMarketingCountsV27(tid, campaignID)
}
