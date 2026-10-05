package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
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

type MetaLeadProfile struct {
	ID                       int64  `json:"id"`
	TenantID                 int64  `json:"tenant_id"`
	SocialConnectionID       int64  `json:"social_connection_id"`
	PageID                   string `json:"page_id"`
	PageName                 string `json:"page_name"`
	Enabled                  bool   `json:"enabled"`
	DefaultCampaignID        int64  `json:"default_campaign_id"`
	DefaultAgentID           int64  `json:"default_agent_id"`
	WhatsAppConnectionID     int64  `json:"whatsapp_connection_id"`
	WhatsAppTemplateName     string `json:"whatsapp_template_name"`
	WhatsAppTemplateLanguage string `json:"whatsapp_template_language"`
	WhatsAppOptInField       string `json:"whatsapp_opt_in_field"`
	AutoWhatsAppFollowup     bool   `json:"auto_whatsapp_followup"`
	BaseScore                int    `json:"base_score"`
	LastLeadAt               string `json:"last_lead_at"`
	LastError                string `json:"last_error"`
	UpdatedAt                string `json:"updated_at"`
}

type metaLeadField struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
}

type metaLeadDetail struct {
	ID                        string          `json:"id"`
	CreatedTime               string          `json:"created_time"`
	FormID                    string          `json:"form_id"`
	AdID                      string          `json:"ad_id"`
	AdsetID                   string          `json:"adset_id"`
	CampaignID                string          `json:"campaign_id"`
	IsOrganic                 bool            `json:"is_organic"`
	Platform                  string          `json:"platform"`
	FieldData                 []metaLeadField `json:"field_data"`
	CustomDisclaimerResponses []struct {
		CheckboxKey string `json:"checkbox_key"`
		IsChecked   string `json:"is_checked"`
	} `json:"custom_disclaimer_responses"`
}

func initMetaLeadAdsSchema(db *DB) error {
	if _, err := db.Exec(`
CREATE TABLE IF NOT EXISTS meta_lead_profiles(
 id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL, social_connection_id INTEGER NOT NULL,
 page_id TEXT NOT NULL, page_name TEXT NOT NULL DEFAULT '', enabled INTEGER NOT NULL DEFAULT 1,
 default_campaign_id INTEGER NOT NULL DEFAULT 0, default_agent_id INTEGER NOT NULL DEFAULT 0,
 whatsapp_connection_id INTEGER NOT NULL DEFAULT 0, whatsapp_template_name TEXT NOT NULL DEFAULT '',
 whatsapp_template_language TEXT NOT NULL DEFAULT 'es', whatsapp_opt_in_field TEXT NOT NULL DEFAULT '',
 auto_whatsapp_followup INTEGER NOT NULL DEFAULT 0, base_score INTEGER NOT NULL DEFAULT 60,
 last_lead_at TEXT NOT NULL DEFAULT '', last_error TEXT NOT NULL DEFAULT '', updated_at TEXT NOT NULL,
 UNIQUE(tenant_id,page_id)
);
CREATE INDEX IF NOT EXISTS idx_meta_lead_profiles_tenant ON meta_lead_profiles(tenant_id,enabled);
CREATE TABLE IF NOT EXISTS meta_lead_events(
 id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL, page_id TEXT NOT NULL, leadgen_id TEXT NOT NULL,
 form_id TEXT NOT NULL DEFAULT '', ad_id TEXT NOT NULL DEFAULT '', adset_id TEXT NOT NULL DEFAULT '',
 campaign_external_id TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'received',
 error_message TEXT NOT NULL DEFAULT '', payload_json TEXT NOT NULL DEFAULT '{}', created_at TEXT NOT NULL, processed_at TEXT NOT NULL DEFAULT '',
 UNIQUE(page_id,leadgen_id)
);
CREATE INDEX IF NOT EXISTS idx_meta_lead_events_tenant ON meta_lead_events(tenant_id,created_at DESC);
CREATE TABLE IF NOT EXISTS meta_lead_forms(
 id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL, page_id TEXT NOT NULL, form_id TEXT NOT NULL,
 name TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT '', created_time TEXT NOT NULL DEFAULT '',
 last_sync_at TEXT NOT NULL DEFAULT '', raw_json TEXT NOT NULL DEFAULT '{}', UNIQUE(tenant_id,page_id,form_id)
);
CREATE INDEX IF NOT EXISTS idx_meta_lead_forms_tenant ON meta_lead_forms(tenant_id,page_id);
CREATE TABLE IF NOT EXISTS lead_automation_outbox(
 id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL, lead_id INTEGER NOT NULL,
 event_type TEXT NOT NULL DEFAULT 'lead.created', source TEXT NOT NULL DEFAULT 'meta_lead_ads',
 status TEXT NOT NULL DEFAULT 'pending', payload_json TEXT NOT NULL DEFAULT '{}', attempts INTEGER NOT NULL DEFAULT 0,
 last_error TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL, processed_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_lead_automation_outbox_status ON lead_automation_outbox(status,created_at);
`); err != nil {
		return err
	}
	alters := []string{
		`ALTER TABLE marketing_leads ADD COLUMN external_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE marketing_leads ADD COLUMN page_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE marketing_leads ADD COLUMN meta_form_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE marketing_leads ADD COLUMN ad_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE marketing_leads ADD COLUMN adset_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE marketing_leads ADD COLUMN campaign_external_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE marketing_leads ADD COLUMN raw_json TEXT NOT NULL DEFAULT '{}'`,
		`ALTER TABLE marketing_leads ADD COLUMN followup_status TEXT NOT NULL DEFAULT ''`,
	}
	for _, q := range alters {
		if _, err := db.Exec(q); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
			return err
		}
	}
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_marketing_leads_external ON marketing_leads(tenant_id,external_id)`)
	return nil
}

func (a *App) metaLeadPageToken(connectionID, tenantID int64) (string, SocialConnection, error) {
	ensureSocialProviderSchema(a.db)
	var c SocialConnection
	var enc string
	err := a.db.QueryRow(`SELECT id,tenant_id,platform,account_name,external_account_id,status,provider_mode,scopes,last_sync_at,last_error,created_at,updated_at,encrypted_credentials
        FROM social_connections WHERE id=? AND tenant_id=? AND platform='facebook' AND status='connected'`, connectionID, tenantID).
		Scan(&c.ID, &c.TenantID, &c.Platform, &c.AccountName, &c.ExternalAccountID, &c.Status, &c.ProviderMode, &c.Scopes, &c.LastSyncAt, &c.LastError, &c.CreatedAt, &c.UpdatedAt, &enc)
	if err != nil {
		return "", c, errors.New("página de Facebook no conectada")
	}
	var tok socialToken
	raw := decryptLocal(enc, a.cfg.ChannelEncryptionKey)
	if raw == "" || json.Unmarshal([]byte(raw), &tok) != nil || strings.TrimSpace(tok.AccessToken) == "" {
		return "", c, errors.New("token de página no disponible; reconecta Facebook")
	}
	return tok.AccessToken, c, nil
}

func (a *App) metaLeadPagesHandler(w http.ResponseWriter, r *http.Request) {
	tid, u, err := a.tenantFor(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	if r.Method == http.MethodGet {
		rows, err := a.db.Query(`SELECT s.id,s.account_name,s.external_account_id,s.status,s.scopes,
            COALESCE(p.id,0),COALESCE(p.enabled,0),COALESCE(p.default_campaign_id,0),COALESCE(p.default_agent_id,0),
            COALESCE(p.whatsapp_connection_id,0),COALESCE(p.whatsapp_template_name,''),COALESCE(p.whatsapp_template_language,'es'),
            COALESCE(p.whatsapp_opt_in_field,''),COALESCE(p.auto_whatsapp_followup,0),COALESCE(p.base_score,60),COALESCE(p.last_lead_at,''),COALESCE(p.last_error,''),COALESCE(p.updated_at,'')
            FROM social_connections s LEFT JOIN meta_lead_profiles p ON p.tenant_id=s.tenant_id AND p.page_id=s.external_account_id
            WHERE s.tenant_id=? AND s.platform='facebook' ORDER BY s.account_name`, tid)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var sid, pid, campaign, agent, wa int64
			var name, page, status, scopes, tpl, lang, opt, lastLead, lastErr, updated string
			var enabled, auto, score int
			if rows.Scan(&sid, &name, &page, &status, &scopes, &pid, &enabled, &campaign, &agent, &wa, &tpl, &lang, &opt, &auto, &score, &lastLead, &lastErr, &updated) == nil {
				out = append(out, map[string]any{"social_connection_id": sid, "page_name": name, "page_id": page, "status": status, "scopes": scopes, "profile_id": pid, "enabled": enabled == 1, "default_campaign_id": campaign, "default_agent_id": agent, "whatsapp_connection_id": wa, "whatsapp_template_name": tpl, "whatsapp_template_language": lang, "whatsapp_opt_in_field": opt, "auto_whatsapp_followup": auto == 1, "base_score": score, "last_lead_at": lastLead, "last_error": lastErr, "updated_at": updated})
			}
		}
		writeJSON(w, out)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	if u == nil || !canManageAgents(u.Role) {
		writeError(w, errors.New("solo propietarios y administradores pueden configurar Lead Ads"), 403)
		return
	}
	var q struct {
		Action                   string `json:"action"`
		SocialConnectionID       int64  `json:"social_connection_id"`
		DefaultCampaignID        int64  `json:"default_campaign_id"`
		DefaultAgentID           int64  `json:"default_agent_id"`
		WhatsAppConnectionID     int64  `json:"whatsapp_connection_id"`
		WhatsAppTemplateName     string `json:"whatsapp_template_name"`
		WhatsAppTemplateLanguage string `json:"whatsapp_template_language"`
		WhatsAppOptInField       string `json:"whatsapp_opt_in_field"`
		AutoWhatsAppFollowup     bool   `json:"auto_whatsapp_followup"`
		BaseScore                int    `json:"base_score"`
		Enabled                  bool   `json:"enabled"`
	}
	if json.NewDecoder(r.Body).Decode(&q) != nil || q.SocialConnectionID <= 0 {
		writeError(w, errors.New("conexión de Facebook obligatoria"), 400)
		return
	}
	token, c, err := a.metaLeadPageToken(q.SocialConnectionID, tid)
	if err != nil {
		writeError(w, err, 409)
		return
	}
	if q.BaseScore <= 0 {
		q.BaseScore = 60
	}
	if q.BaseScore > 100 {
		q.BaseScore = 100
	}
	if q.WhatsAppTemplateLanguage == "" {
		q.WhatsAppTemplateLanguage = "es"
	}
	action := strings.ToLower(strings.TrimSpace(q.Action))
	if action == "" {
		action = "save"
	}
	if action == "subscribe" || action == "save" {
		ep := "https://graph.facebook.com/" + a.cfg.MetaGraphVersion + "/" + url.PathEscape(c.ExternalAccountID) + "/subscribed_apps?subscribed_fields=leadgen"
		var res map[string]any
		if err = apiJSON(r.Context(), http.MethodPost, ep, token, nil, &res); err != nil {
			_, _ = a.db.Exec(`INSERT INTO meta_lead_profiles(tenant_id,social_connection_id,page_id,page_name,enabled,last_error,updated_at) VALUES(?,?,?,?,1,?,?) ON CONFLICT(tenant_id,page_id) DO UPDATE SET last_error=excluded.last_error,updated_at=excluded.updated_at`, tid, c.ID, c.ExternalAccountID, c.AccountName, err.Error(), time.Now().UTC().Format(time.RFC3339))
			writeError(w, fmt.Errorf("Meta no permitió suscribir leadgen: %w", err), 502)
			return
		}
	}
	en := 0
	if q.Enabled || action == "subscribe" {
		en = 1
	}
	aw := 0
	if q.AutoWhatsAppFollowup {
		aw = 1
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = a.db.Exec(`INSERT INTO meta_lead_profiles(tenant_id,social_connection_id,page_id,page_name,enabled,default_campaign_id,default_agent_id,whatsapp_connection_id,whatsapp_template_name,whatsapp_template_language,whatsapp_opt_in_field,auto_whatsapp_followup,base_score,last_error,updated_at)
        VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,'',?) ON CONFLICT(tenant_id,page_id) DO UPDATE SET social_connection_id=excluded.social_connection_id,page_name=excluded.page_name,enabled=excluded.enabled,default_campaign_id=excluded.default_campaign_id,default_agent_id=excluded.default_agent_id,whatsapp_connection_id=excluded.whatsapp_connection_id,whatsapp_template_name=excluded.whatsapp_template_name,whatsapp_template_language=excluded.whatsapp_template_language,whatsapp_opt_in_field=excluded.whatsapp_opt_in_field,auto_whatsapp_followup=excluded.auto_whatsapp_followup,base_score=excluded.base_score,last_error='',updated_at=excluded.updated_at`, tid, c.ID, c.ExternalAccountID, c.AccountName, en, q.DefaultCampaignID, q.DefaultAgentID, q.WhatsAppConnectionID, strings.TrimSpace(q.WhatsAppTemplateName), q.WhatsAppTemplateLanguage, strings.TrimSpace(q.WhatsAppOptInField), aw, q.BaseScore, now)
	if err != nil {
		writeError(w, err, 500)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "page_id": c.ExternalAccountID, "webhook": a.cfg.BaseURL + "/webhooks/meta/leads"})
}

func (a *App) metaLeadFormsHandler(w http.ResponseWriter, r *http.Request) {
	tid, _, err := a.tenantFor(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	cid, _ := strconv.ParseInt(r.URL.Query().Get("connection_id"), 10, 64)
	if cid <= 0 {
		writeError(w, errors.New("connection_id obligatorio"), 400)
		return
	}
	token, c, err := a.metaLeadPageToken(cid, tid)
	if err != nil {
		writeError(w, err, 409)
		return
	}
	var out struct {
		Data []map[string]any `json:"data"`
	}
	ep := "https://graph.facebook.com/" + a.cfg.MetaGraphVersion + "/" + url.PathEscape(c.ExternalAccountID) + "/leadgen_forms?fields=id,name,status,created_time&limit=100"
	if err = apiJSON(r.Context(), http.MethodGet, ep, token, nil, &out); err != nil {
		writeError(w, err, 502)
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	for _, f := range out.Data {
		id := fmt.Sprint(f["id"])
		name := fmt.Sprint(f["name"])
		status := fmt.Sprint(f["status"])
		created := fmt.Sprint(f["created_time"])
		b, _ := json.Marshal(f)
		_, _ = a.db.Exec(`INSERT INTO meta_lead_forms(tenant_id,page_id,form_id,name,status,created_time,last_sync_at,raw_json) VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(tenant_id,page_id,form_id) DO UPDATE SET name=excluded.name,status=excluded.status,created_time=excluded.created_time,last_sync_at=excluded.last_sync_at,raw_json=excluded.raw_json`, tid, c.ExternalAccountID, id, name, status, created, now, string(b))
	}
	writeJSON(w, out.Data)
}

func (a *App) metaLeadEventsHandler(w http.ResponseWriter, r *http.Request) {
	tid, _, err := a.tenantFor(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	rows, err := a.db.Query(`SELECT id,page_id,leadgen_id,form_id,ad_id,adset_id,campaign_external_id,status,error_message,created_at,processed_at FROM meta_lead_events WHERE tenant_id=? ORDER BY id DESC LIMIT 100`, tid)
	if err != nil {
		writeError(w, err, 500)
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id int64
		var page, lead, form, ad, adset, camp, status, msg, created, processed string
		if rows.Scan(&id, &page, &lead, &form, &ad, &adset, &camp, &status, &msg, &created, &processed) == nil {
			out = append(out, map[string]any{"id": id, "page_id": page, "leadgen_id": lead, "form_id": form, "ad_id": ad, "adset_id": adset, "campaign_external_id": camp, "status": status, "error_message": msg, "created_at": created, "processed_at": processed})
		}
	}
	writeJSON(w, out)
}

func verifyMetaSignature(secret string, raw []byte, header string) bool {
	if secret == "" {
		return false
	}
	const prefix = "sha256="
	if !strings.HasPrefix(header, prefix) {
		return false
	}
	want, err := hex.DecodeString(strings.TrimPrefix(header, prefix))
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(raw)
	return hmac.Equal(want, mac.Sum(nil))
}

func (a *App) metaLeadWebhookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		if r.URL.Query().Get("hub.mode") == "subscribe" && r.URL.Query().Get("hub.verify_token") == a.cfg.MetaLeadsVerifyToken && a.cfg.MetaLeadsVerifyToken != "" {
			w.WriteHeader(200)
			_, _ = w.Write([]byte(r.URL.Query().Get("hub.challenge")))
			return
		}
		http.Error(w, "verify token inválido", 403)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
	if err != nil {
		http.Error(w, "payload inválido", 400)
		return
	}
	if !verifyMetaSignature(a.cfg.MetaAppSecret, raw, r.Header.Get("X-Hub-Signature-256")) {
		http.Error(w, "firma inválida", 401)
		return
	}
	var env struct {
		Object string `json:"object"`
		Entry  []struct {
			ID      string `json:"id"`
			Changes []struct {
				Field string `json:"field"`
				Value struct {
					LeadgenID   string `json:"leadgen_id"`
					FormID      string `json:"form_id"`
					PageID      string `json:"page_id"`
					AdID        string `json:"ad_id"`
					AdgroupID   string `json:"adgroup_id"`
					CreatedTime int64  `json:"created_time"`
				} `json:"value"`
			} `json:"changes"`
		} `json:"entry"`
	}
	if json.Unmarshal(raw, &env) != nil {
		http.Error(w, "JSON inválido", 400)
		return
	}
	// Meta expects a quick acknowledgement. Processing is kept bounded and idempotent.
	for _, entry := range env.Entry {
		for _, ch := range entry.Changes {
			if ch.Field != "leadgen" || ch.Value.LeadgenID == "" {
				continue
			}
			page := ch.Value.PageID
			if page == "" {
				page = entry.ID
			}
			_ = a.processMetaLead(r, page, ch.Value.LeadgenID, ch.Value.FormID, ch.Value.AdID, ch.Value.AdgroupID, raw)
		}
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (a *App) processMetaLead(r *http.Request, pageID, leadgenID, formID, adID, adgroupID string, rawWebhook []byte) error {
	var p MetaLeadProfile
	var enabled, auto int
	err := a.db.QueryRow(`SELECT id,tenant_id,social_connection_id,page_id,page_name,enabled,default_campaign_id,default_agent_id,whatsapp_connection_id,whatsapp_template_name,whatsapp_template_language,whatsapp_opt_in_field,auto_whatsapp_followup,base_score,last_lead_at,last_error,updated_at FROM meta_lead_profiles WHERE page_id=? AND enabled=1 ORDER BY id LIMIT 1`, pageID).Scan(&p.ID, &p.TenantID, &p.SocialConnectionID, &p.PageID, &p.PageName, &enabled, &p.DefaultCampaignID, &p.DefaultAgentID, &p.WhatsAppConnectionID, &p.WhatsAppTemplateName, &p.WhatsAppTemplateLanguage, &p.WhatsAppOptInField, &auto, &p.BaseScore, &p.LastLeadAt, &p.LastError, &p.UpdatedAt)
	if err != nil {
		return err
	}
	p.Enabled = enabled == 1
	p.AutoWhatsAppFollowup = auto == 1
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := a.db.Exec(`INSERT OR IGNORE INTO meta_lead_events(tenant_id,page_id,leadgen_id,form_id,ad_id,adset_id,status,payload_json,created_at) VALUES(?,?,?,?,?,?,'received',?,?)`, p.TenantID, pageID, leadgenID, formID, adID, adgroupID, string(rawWebhook), now)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil
	}
	token, _, err := a.metaLeadPageToken(p.SocialConnectionID, p.TenantID)
	if err != nil {
		a.failMetaLeadEvent(pageID, leadgenID, err)
		return err
	}
	fields := "id,created_time,form_id,ad_id,adset_id,campaign_id,is_organic,platform,field_data,custom_disclaimer_responses"
	ep := "https://graph.facebook.com/" + a.cfg.MetaGraphVersion + "/" + url.PathEscape(leadgenID) + "?fields=" + url.QueryEscape(fields)
	var lead metaLeadDetail
	if err = apiJSON(r.Context(), http.MethodGet, ep, token, nil, &lead); err != nil {
		fallback := "https://graph.facebook.com/" + a.cfg.MetaGraphVersion + "/" + url.PathEscape(leadgenID) + "?fields=" + url.QueryEscape("id,created_time,field_data")
		if err = apiJSON(r.Context(), http.MethodGet, fallback, token, nil, &lead); err != nil {
			a.failMetaLeadEvent(pageID, leadgenID, err)
			return err
		}
	}
	if lead.ID == "" {
		lead.ID = leadgenID
	}
	if lead.FormID == "" {
		lead.FormID = formID
	}
	if lead.AdID == "" {
		lead.AdID = adID
	}
	if lead.AdsetID == "" {
		lead.AdsetID = adgroupID
	}
	vals := map[string]string{}
	for _, f := range lead.FieldData {
		if len(f.Values) > 0 {
			vals[strings.ToLower(strings.TrimSpace(f.Name))] = strings.TrimSpace(f.Values[0])
		}
	}
	full := firstNonEmpty(vals["full_name"], strings.TrimSpace(vals["first_name"]+" "+vals["last_name"]), vals["name"])
	phone := firstNonEmpty(vals["phone_number"], vals["phone"], vals["telefono"], vals["teléfono"])
	email := firstNonEmpty(vals["email"], vals["correo"], vals["correo_electronico"])
	city := firstNonEmpty(vals["city"], vals["ciudad"])
	interest := ""
	for k, v := range vals {
		if k != "full_name" && k != "first_name" && k != "last_name" && k != "name" && k != "phone_number" && k != "phone" && k != "email" && k != "city" && k != "ciudad" && v != "" {
			interest = k + ": " + v
			break
		}
	}
	score := p.BaseScore
	if phone != "" {
		score += 5
	}
	if email != "" {
		score += 5
	}
	if score > 100 {
		score = 100
	}
	consent := false
	if p.WhatsAppOptInField != "" {
		consent = isAffirmative(vals[strings.ToLower(strings.TrimSpace(p.WhatsAppOptInField))])
	}
	leadRaw, _ := json.Marshal(lead)
	created := lead.CreatedTime
	if created == "" {
		created = now
	}
	mr, err := a.db.Exec(`INSERT INTO marketing_leads(tenant_id,campaign_id,form_id,source,name,phone,email,city,interest,score,status,utm_source,utm_campaign,consent,created_at,external_id,page_id,meta_form_id,ad_id,adset_id,campaign_external_id,raw_json,followup_status) VALUES(?,?,?,?,?,?,?,?,?,?,'new','facebook','',?,?,?,?,?,?,?,?,?,'')`, p.TenantID, p.DefaultCampaignID, 0, "meta_lead_ads", full, phone, email, city, interest, score, boolInt(consent), created, lead.ID, pageID, lead.FormID, lead.AdID, lead.AdsetID, lead.CampaignID, string(leadRaw))
	if err != nil {
		a.failMetaLeadEvent(pageID, leadgenID, err)
		return err
	}
	leadID, _ := mr.LastInsertId()
	_ = a.syncCRMContactAt(p.TenantID, full, phone, email, "facebook", "meta_lead_ads", lead.ID, created)
	_ = a.syncOpportunityFromLead(p.TenantID, leadID, full, phone, email, "facebook", "meta_lead_ads", interest, score, created)
	payload := map[string]any{"lead_id": leadID, "external_id": lead.ID, "page_id": pageID, "form_id": lead.FormID, "ad_id": lead.AdID, "campaign_id": lead.CampaignID, "score": score, "agent_id": p.DefaultAgentID, "name": full, "phone": phone, "email": email, "city": city, "interest": interest, "whatsapp_opt_in": consent}
	_, _ = a.db.Exec(`INSERT INTO lead_automation_outbox(tenant_id,lead_id,event_type,source,status,payload_json,created_at) VALUES(?,?,'lead.created','meta_lead_ads','pending',?,?)`, p.TenantID, leadID, mustJSON(payload), now)
	follow := "queued_for_automation"
	if p.AutoWhatsAppFollowup && consent && phone != "" && p.WhatsAppConnectionID > 0 && p.WhatsAppTemplateName != "" {
		if e := a.sendMetaLeadWhatsAppTemplate(r, p, phone); e != nil {
			follow = "error: " + e.Error()
		} else {
			follow = "template_sent"
		}
	} else if p.AutoWhatsAppFollowup && !consent {
		follow = "blocked_no_explicit_whatsapp_optin"
	}
	_, _ = a.db.Exec(`UPDATE marketing_leads SET followup_status=? WHERE id=? AND tenant_id=?`, follow, leadID, p.TenantID)
	_, _ = a.db.Exec(`UPDATE meta_lead_events SET form_id=?,ad_id=?,adset_id=?,campaign_external_id=?,status='processed',processed_at=?,error_message='' WHERE page_id=? AND leadgen_id=?`, lead.FormID, lead.AdID, lead.AdsetID, lead.CampaignID, now, pageID, leadgenID)
	_, _ = a.db.Exec(`UPDATE meta_lead_profiles SET last_lead_at=?,last_error='',updated_at=? WHERE id=?`, now, now, p.ID)
	return nil
}

func (a *App) failMetaLeadEvent(page, lead string, err error) {
	now := time.Now().UTC().Format(time.RFC3339)
	_, _ = a.db.Exec(`UPDATE meta_lead_events SET status='error',error_message=?,processed_at=? WHERE page_id=? AND leadgen_id=?`, err.Error(), now, page, lead)
	_, _ = a.db.Exec(`UPDATE meta_lead_profiles SET last_error=?,updated_at=? WHERE page_id=?`, err.Error(), now, page)
}
func isAffirmative(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "y", "si", "sí", "accepted", "acepto", "aceptado", "on":
		return true
	}
	return false
}

func (a *App) sendMetaLeadWhatsAppTemplate(r *http.Request, p MetaLeadProfile, to string) error {
	c, err := a.whatsappCloudConnectionForTenant(p.TenantID, p.WhatsAppConnectionID)
	if err != nil {
		return err
	}
	cr, err := a.whatsappCloudCredentialsFor(c)
	if err != nil {
		return err
	}
	cfg := a.whatsappCloudConfigFor(c)
	body := map[string]any{"messaging_product": "whatsapp", "to": strings.TrimPrefix(strings.TrimSpace(to), "+"), "type": "template", "template": map[string]any{"name": p.WhatsAppTemplateName, "language": map[string]any{"code": firstNonEmpty(p.WhatsAppTemplateLanguage, "es")}}}
	var out struct {
		Messages []struct {
			ID string `json:"id"`
		} `json:"messages"`
	}
	ep := "https://graph.facebook.com/" + a.cfg.MetaGraphVersion + "/" + url.PathEscape(cfg.PhoneNumberID) + "/messages"
	if err = a.whatsappCloudGraph(r.Context(), http.MethodPost, ep, cr.AccessToken, body, &out); err != nil {
		return err
	}
	mid := ""
	if len(out.Messages) > 0 {
		mid = out.Messages[0].ID
	}
	_, _ = a.db.Exec(`INSERT INTO whatsapp_business_events(tenant_id,connection_id,event_type,external_id,status,payload_json,created_at) VALUES(?,?,?,?,?,?,?)`, p.TenantID, p.WhatsAppConnectionID, "lead_followup", mid, "sent", mustJSON(body), time.Now().UTC().Format(time.RFC3339))
	return nil
}
