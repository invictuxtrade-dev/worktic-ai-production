package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

type analyticsTouchV24 struct {
	Source     string `json:"source"`
	Channel    string `json:"channel"`
	PostID     int64  `json:"post_id"`
	CampaignID int64  `json:"campaign_id"`
	LandingID  int64  `json:"landing_id"`
	At         string `json:"at"`
}

type analyticsDimV24 struct {
	Key          string  `json:"key"`
	Label        string  `json:"label"`
	Leads        int64   `json:"leads"`
	Appointments int64   `json:"appointments"`
	Sales        int64   `json:"sales"`
	Revenue      float64 `json:"revenue"`
	Spend        float64 `json:"spend"`
	Clicks       int64   `json:"clicks"`
	Reach        int64   `json:"reach"`
}

func initAnalyticsV24Schema(db *DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS analytics_attribution_events_v24(
		 id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL, event_key TEXT NOT NULL DEFAULT '',
		 event_type TEXT NOT NULL, contact_id INTEGER NOT NULL DEFAULT 0, lead_id INTEGER NOT NULL DEFAULT 0,
		 opportunity_id INTEGER NOT NULL DEFAULT 0, appointment_id INTEGER NOT NULL DEFAULT 0,
		 campaign_id INTEGER NOT NULL DEFAULT 0, social_post_id INTEGER NOT NULL DEFAULT 0, agent_id INTEGER NOT NULL DEFAULT 0,
		 owner TEXT NOT NULL DEFAULT '', channel TEXT NOT NULL DEFAULT '', source TEXT NOT NULL DEFAULT '', source_ref TEXT NOT NULL DEFAULT '',
		 value REAL NOT NULL DEFAULT 0, metadata_json TEXT NOT NULL DEFAULT '{}', occurred_at TEXT NOT NULL, created_at TEXT NOT NULL)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_analytics_event_key_v24 ON analytics_attribution_events_v24(tenant_id,event_key) WHERE event_key<>''`,
		`CREATE INDEX IF NOT EXISTS idx_analytics_events_tenant_v24 ON analytics_attribution_events_v24(tenant_id,event_type,occurred_at)`,
		`CREATE TABLE IF NOT EXISTS analytics_settings_v24(
		 tenant_id INTEGER PRIMARY KEY, attribution_model TEXT NOT NULL DEFAULT 'last_touch', currency TEXT NOT NULL DEFAULT 'USD',
		 lookback_days INTEGER NOT NULL DEFAULT 90, updated_at TEXT NOT NULL)`,
		`ALTER TABLE social_posts ADD COLUMN tracking_url TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE marketing_leads ADD COLUMN landing_id INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE marketing_leads ADD COLUMN social_post_id INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE marketing_leads ADD COLUMN attribution_channel TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE marketing_leads ADD COLUMN attribution_source_ref TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE marketing_leads ADD COLUMN first_touch_source TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE marketing_leads ADD COLUMN first_touch_channel TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE marketing_leads ADD COLUMN first_touch_post_id INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE marketing_leads ADD COLUMN first_touch_campaign_id INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE marketing_leads ADD COLUMN last_touch_source TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE marketing_leads ADD COLUMN last_touch_channel TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE marketing_leads ADD COLUMN last_touch_post_id INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE marketing_leads ADD COLUMN last_touch_campaign_id INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE crm_opportunities ADD COLUMN lead_id INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE crm_opportunities ADD COLUMN campaign_id INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE crm_opportunities ADD COLUMN social_post_id INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE crm_opportunities ADD COLUMN agent_id INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE crm_appointments ADD COLUMN contact_id INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE crm_appointments ADD COLUMN opportunity_id INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE crm_appointments ADD COLUMN campaign_id INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE crm_appointments ADD COLUMN source TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE crm_appointments ADD COLUMN source_ref TEXT NOT NULL DEFAULT ''`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
			return err
		}
	}
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_marketing_leads_attr_v24 ON marketing_leads(tenant_id,social_post_id,campaign_id,created_at)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_crm_opps_attr_v24 ON crm_opportunities(tenant_id,lead_id,campaign_id,social_post_id,stage)`)
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_crm_appts_attr_v24 ON crm_appointments(tenant_id,contact_id,opportunity_id,created_at)`)
	return nil
}

func encodeAnalyticsTouchV24(t analyticsTouchV24) string {
	b, _ := json.Marshal(t)
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeAnalyticsTouchV24(v string) analyticsTouchV24 {
	var t analyticsTouchV24
	b, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(v))
	if err == nil {
		_ = json.Unmarshal(b, &t)
	}
	return t
}

func (a *App) recordAnalyticsEventV24(tid int64, eventKey, eventType string, fields map[string]any) {
	if tid <= 0 || strings.TrimSpace(eventType) == "" {
		return
	}
	ival := func(k string) int64 {
		switch v := fields[k].(type) {
		case int64:
			return v
		case int:
			return int64(v)
		case float64:
			return int64(v)
		case string:
			n, _ := strconv.ParseInt(v, 10, 64)
			return n
		}
		return 0
	}
	sval := func(k string) string {
		v, ok := fields[k]
		if !ok || v == nil {
			return ""
		}
		return strings.TrimSpace(fmt.Sprint(v))
	}
	fval := func(k string) float64 {
		switch v := fields[k].(type) {
		case float64:
			return v
		case int64:
			return float64(v)
		case int:
			return float64(v)
		case string:
			n, _ := strconv.ParseFloat(v, 64)
			return n
		}
		return 0
	}
	occurred := sval("occurred_at")
	if occurred == "" || occurred == "<nil>" {
		occurred = time.Now().UTC().Format(time.RFC3339)
	}
	meta, _ := json.Marshal(fields)
	now := time.Now().UTC().Format(time.RFC3339)
	q := `INSERT INTO analytics_attribution_events_v24(tenant_id,event_key,event_type,contact_id,lead_id,opportunity_id,appointment_id,campaign_id,social_post_id,agent_id,owner,channel,source,source_ref,value,metadata_json,occurred_at,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`
	if eventKey != "" {
		q = `INSERT OR IGNORE INTO analytics_attribution_events_v24(tenant_id,event_key,event_type,contact_id,lead_id,opportunity_id,appointment_id,campaign_id,social_post_id,agent_id,owner,channel,source,source_ref,value,metadata_json,occurred_at,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`
	}
	_, _ = a.db.Exec(q, tid, eventKey, eventType, ival("contact_id"), ival("lead_id"), ival("opportunity_id"), ival("appointment_id"), ival("campaign_id"), ival("social_post_id"), ival("agent_id"), sval("owner"), sval("channel"), sval("source"), sval("source_ref"), fval("value"), string(meta), occurred, now)
}

func (a *App) ensureSocialTrackingURLV24(tid, postID int64) string {
	if tid <= 0 || postID <= 0 {
		return ""
	}
	var link, tracked string
	if a.db.QueryRow(`SELECT link_url,COALESCE(tracking_url,'') FROM social_posts WHERE tenant_id=? AND id=?`, tid, postID).Scan(&link, &tracked) != nil {
		return ""
	}
	if strings.TrimSpace(link) == "" {
		return ""
	}
	if tracked == "" {
		tracked = fmt.Sprintf("%s/t/%d/%d", strings.TrimRight(a.cfg.BaseURL, "/"), tid, postID)
		_, _ = a.db.Exec(`UPDATE social_posts SET tracking_url=? WHERE tenant_id=? AND id=?`, tracked, tid, postID)
	}
	return tracked
}

func appendAnalyticsQueryV24(destination string, touch analyticsTouchV24) string {
	u, err := url.Parse(destination)
	if err != nil || u.Scheme == "" {
		return destination
	}
	q := u.Query()
	if q.Get("utm_source") == "" && touch.Channel != "" {
		q.Set("utm_source", touch.Channel)
	}
	if q.Get("utm_medium") == "" {
		q.Set("utm_medium", "social")
	}
	if q.Get("utm_campaign") == "" && touch.CampaignID > 0 {
		q.Set("utm_campaign", fmt.Sprintf("worktic_%d", touch.CampaignID))
	}
	if touch.PostID > 0 {
		q.Set("wt_post", strconv.FormatInt(touch.PostID, 10))
	}
	if touch.CampaignID > 0 {
		q.Set("wt_campaign", strconv.FormatInt(touch.CampaignID, 10))
	}
	if touch.Channel != "" {
		q.Set("wt_channel", touch.Channel)
	}
	q.Set("wt_source", "social")
	u.RawQuery = q.Encode()
	return u.String()
}

func (a *App) analyticsTrackingRedirectHandler(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 3 || parts[0] != "t" {
		http.NotFound(w, r)
		return
	}
	tid, _ := strconv.ParseInt(parts[1], 10, 64)
	postID, _ := strconv.ParseInt(parts[2], 10, 64)
	var destination, platform string
	var campaignID int64
	if tid <= 0 || postID <= 0 || a.db.QueryRow(`SELECT link_url,platform,campaign_id FROM social_posts WHERE tenant_id=? AND id=?`, tid, postID).Scan(&destination, &platform, &campaignID) != nil || strings.TrimSpace(destination) == "" {
		http.NotFound(w, r)
		return
	}
	touch := analyticsTouchV24{Source: "social", Channel: platform, PostID: postID, CampaignID: campaignID, At: time.Now().UTC().Format(time.RFC3339)}
	lastVal := encodeAnalyticsTouchV24(touch)
	secure := strings.HasPrefix(strings.ToLower(a.cfg.BaseURL), "https://")
	if _, err := r.Cookie("wt_first_touch"); err != nil {
		http.SetCookie(w, &http.Cookie{Name: "wt_first_touch", Value: lastVal, Path: "/", MaxAge: 86400 * 90, HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode})
	}
	http.SetCookie(w, &http.Cookie{Name: "wt_last_touch", Value: lastVal, Path: "/", MaxAge: 86400 * 90, HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode})
	a.recordAnalyticsEventV24(tid, "", "content_click", map[string]any{"campaign_id": campaignID, "social_post_id": postID, "channel": platform, "source": "social", "source_ref": fmt.Sprintf("post:%d", postID), "occurred_at": touch.At})
	http.Redirect(w, r, appendAnalyticsQueryV24(destination, touch), http.StatusFound)
}

func analyticsHiddenInputsV24(r *http.Request, landingID, campaignID int64) string {
	if r == nil {
		return ""
	}
	q := r.URL.Query()
	get := func(k string) string { return strings.TrimSpace(q.Get(k)) }
	vals := map[string]string{
		"utm_source":   get("utm_source"),
		"utm_campaign": get("utm_campaign"),
		"wt_post":      get("wt_post"),
		"wt_campaign":  get("wt_campaign"),
		"wt_channel":   get("wt_channel"),
		"wt_source":    get("wt_source"),
	}
	if campaignID > 0 && vals["wt_campaign"] == "" {
		vals["wt_campaign"] = strconv.FormatInt(campaignID, 10)
	}
	if landingID > 0 {
		vals["wt_landing"] = strconv.FormatInt(landingID, 10)
	}
	var b strings.Builder
	keys := []string{"utm_source", "utm_campaign", "wt_post", "wt_campaign", "wt_channel", "wt_source", "wt_landing"}
	for _, k := range keys {
		if v := vals[k]; v != "" {
			b.WriteString(`<input type="hidden" name="` + k + `" value="` + templateEscapeV24(v) + `">`)
		}
	}
	return b.String()
}

func templateEscapeV24(v string) string {
	r := strings.NewReplacer("&", "&amp;", "\"", "&quot;", "<", "&lt;", ">", "&gt;")
	return r.Replace(v)
}

func touchFromRequestV24(r *http.Request, which string) analyticsTouchV24 {
	if r == nil {
		return analyticsTouchV24{}
	}
	if c, err := r.Cookie(which); err == nil {
		if t := decodeAnalyticsTouchV24(c.Value); t.Channel != "" || t.PostID > 0 || t.CampaignID > 0 {
			return t
		}
	}
	_ = r.ParseForm()
	postID, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("wt_post")), 10, 64)
	campaignID, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("wt_campaign")), 10, 64)
	landingID, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("wt_landing")), 10, 64)
	return analyticsTouchV24{Source: firstNonEmpty(strings.TrimSpace(r.FormValue("wt_source")), "landing"), Channel: firstNonEmpty(strings.TrimSpace(r.FormValue("wt_channel")), strings.TrimSpace(r.FormValue("utm_source")), "landing"), PostID: postID, CampaignID: campaignID, LandingID: landingID, At: time.Now().UTC().Format(time.RFC3339)}
}

func (a *App) applyLeadAttributionFromRequestV24(tid, leadID int64, r *http.Request) {
	if tid <= 0 || leadID <= 0 || r == nil {
		return
	}
	first := touchFromRequestV24(r, "wt_first_touch")
	last := touchFromRequestV24(r, "wt_last_touch")
	_ = r.ParseForm()
	if last.PostID == 0 {
		last.PostID, _ = strconv.ParseInt(strings.TrimSpace(r.FormValue("wt_post")), 10, 64)
	}
	if last.CampaignID == 0 {
		last.CampaignID, _ = strconv.ParseInt(strings.TrimSpace(r.FormValue("wt_campaign")), 10, 64)
	}
	if last.LandingID == 0 {
		last.LandingID, _ = strconv.ParseInt(strings.TrimSpace(r.FormValue("wt_landing")), 10, 64)
	}
	if first.Channel == "" && first.PostID == 0 && first.CampaignID == 0 {
		first = last
	}
	campaign := last.CampaignID
	if campaign == 0 {
		campaign = first.CampaignID
	}
	_, _ = a.db.Exec(`UPDATE marketing_leads SET
		campaign_id=CASE WHEN campaign_id=0 AND ?>0 THEN ? ELSE campaign_id END,
		landing_id=CASE WHEN ?>0 THEN ? ELSE landing_id END,
		social_post_id=CASE WHEN ?>0 THEN ? ELSE social_post_id END,
		attribution_channel=?,attribution_source_ref=?,
		first_touch_source=?,first_touch_channel=?,first_touch_post_id=?,first_touch_campaign_id=?,
		last_touch_source=?,last_touch_channel=?,last_touch_post_id=?,last_touch_campaign_id=?
		WHERE tenant_id=? AND id=?`, campaign, campaign, last.LandingID, last.LandingID, last.PostID, last.PostID,
		firstNonEmpty(last.Channel, first.Channel, strings.TrimSpace(r.FormValue("utm_source")), "landing"),
		func() string {
			if last.PostID > 0 {
				return fmt.Sprintf("post:%d", last.PostID)
			}
			return ""
		}(),
		first.Source, first.Channel, first.PostID, first.CampaignID,
		last.Source, last.Channel, last.PostID, last.CampaignID, tid, leadID)
}

func parseLeadRefV24(sourceRef string) int64 {
	if strings.HasPrefix(sourceRef, "lead:") {
		n, _ := strconv.ParseInt(strings.TrimPrefix(sourceRef, "lead:"), 10, 64)
		return n
	}
	return 0
}

func (a *App) contactIDForAttributionV24(tid int64, phone, email string) int64 {
	phone = normalizeCRMPhone(phone)
	email = normalizeCRMEmail(email)
	var id int64
	if phone != "" {
		_ = a.db.QueryRow(`SELECT id FROM crm_contacts WHERE tenant_id=? AND phone=? AND COALESCE(deleted_at,'')='' ORDER BY id DESC LIMIT 1`, tid, phone).Scan(&id)
	}
	if id == 0 && email != "" {
		_ = a.db.QueryRow(`SELECT id FROM crm_contacts WHERE tenant_id=? AND lower(email)=? AND COALESCE(deleted_at,'')='' ORDER BY id DESC LIMIT 1`, tid, email).Scan(&id)
	}
	return id
}

func (a *App) syncAnalyticsV24Tenant(tid int64) {
	if tid <= 0 {
		return
	}
	// Leads: source of truth for acquisition attribution.
	rows, err := a.db.Query(`SELECT id,campaign_id,name,phone,email,source,utm_source,created_at,social_post_id,attribution_channel,first_touch_source,first_touch_channel,first_touch_post_id,first_touch_campaign_id,last_touch_source,last_touch_channel,last_touch_post_id,last_touch_campaign_id,page_id FROM marketing_leads WHERE tenant_id=?`, tid)
	if err == nil {
		for rows.Next() {
			var id, campaignID, postID, ftp, ftc, ltp, ltc int64
			var name, phone, email, source, utm, created, attrChannel, fts, ftch, lts, ltch, pageID string
			_ = rows.Scan(&id, &campaignID, &name, &phone, &email, &source, &utm, &created, &postID, &attrChannel, &fts, &ftch, &ftp, &ftc, &lts, &ltch, &ltp, &ltc, &pageID)
			contactID := a.contactIDForAttributionV24(tid, phone, email)
			channel := firstNonEmpty(attrChannel, ltch, ftch, utm)
			if channel == "" {
				if source == "meta_lead_ads" {
					channel = "facebook"
				} else {
					channel = "landing"
				}
			}
			if postID == 0 {
				postID = ltp
				if postID == 0 {
					postID = ftp
				}
			}
			if campaignID == 0 {
				campaignID = ltc
				if campaignID == 0 {
					campaignID = ftc
				}
			}
			agentID := int64(0)
			if pageID != "" {
				_ = a.db.QueryRow(`SELECT default_agent_id FROM meta_lead_profiles WHERE tenant_id=? AND page_id=? ORDER BY id DESC LIMIT 1`, tid, pageID).Scan(&agentID)
			}
			a.recordAnalyticsEventV24(tid, fmt.Sprintf("lead:%d", id), "lead", map[string]any{"lead_id": id, "contact_id": contactID, "campaign_id": campaignID, "social_post_id": postID, "agent_id": agentID, "channel": channel, "source": source, "source_ref": fmt.Sprintf("lead:%d", id), "occurred_at": created})
		}
		rows.Close()
	}
	// Opportunities and sales; enrich from lead attribution where possible.
	oppRows, err := a.db.Query(`SELECT id,contact_id,stage,value,owner,source,source_ref,channel,created_at,updated_at,closed_at,lead_id,campaign_id,social_post_id,agent_id FROM crm_opportunities WHERE tenant_id=? AND COALESCE(deleted_at,'')=''`, tid)
	if err == nil {
		for oppRows.Next() {
			var id, contactID, leadID, campaignID, postID, agentID int64
			var stage, owner, source, sourceRef, channel, created, updated, closed string
			var value float64
			_ = oppRows.Scan(&id, &contactID, &stage, &value, &owner, &source, &sourceRef, &channel, &created, &updated, &closed, &leadID, &campaignID, &postID, &agentID)
			if leadID == 0 {
				leadID = parseLeadRefV24(sourceRef)
			}
			if leadID > 0 {
				var lc, lp, la int64
				var lch string
				_ = a.db.QueryRow(`SELECT campaign_id,social_post_id,attribution_channel FROM marketing_leads WHERE tenant_id=? AND id=?`, tid, leadID).Scan(&lc, &lp, &lch)
				if campaignID == 0 {
					campaignID = lc
				}
				if postID == 0 {
					postID = lp
				}
				if channel == "" {
					channel = lch
				}
				_ = a.db.QueryRow(`SELECT COALESCE(default_agent_id,0) FROM meta_lead_profiles WHERE tenant_id=? AND page_id=(SELECT page_id FROM marketing_leads WHERE tenant_id=? AND id=?) ORDER BY id DESC LIMIT 1`, tid, tid, leadID).Scan(&la)
				if agentID == 0 {
					agentID = la
				}
				_, _ = a.db.Exec(`UPDATE crm_opportunities SET lead_id=?,campaign_id=?,social_post_id=?,agent_id=? WHERE tenant_id=? AND id=?`, leadID, campaignID, postID, agentID, tid, id)
			}
			when := firstNonEmpty(created, updated)
			a.recordAnalyticsEventV24(tid, fmt.Sprintf("opportunity:%d", id), "opportunity", map[string]any{"contact_id": contactID, "lead_id": leadID, "opportunity_id": id, "campaign_id": campaignID, "social_post_id": postID, "agent_id": agentID, "owner": owner, "channel": channel, "source": source, "source_ref": sourceRef, "value": value, "occurred_at": when})
			if stage == "Ganado" {
				a.recordAnalyticsEventV24(tid, fmt.Sprintf("sale:%d", id), "sale", map[string]any{"contact_id": contactID, "lead_id": leadID, "opportunity_id": id, "campaign_id": campaignID, "social_post_id": postID, "agent_id": agentID, "owner": owner, "channel": channel, "source": source, "source_ref": sourceRef, "value": value, "occurred_at": firstNonEmpty(closed, updated)})
			}
		}
		oppRows.Close()
	}
	// Appointments: attach to CRM contact and most relevant opportunity.
	apRows, err := a.db.Query(`SELECT id,contact_phone,created_at,contact_id,opportunity_id,campaign_id,source,source_ref FROM crm_appointments WHERE tenant_id=?`, tid)
	if err == nil {
		for apRows.Next() {
			var id, contactID, oppID, campaignID int64
			var phone, created, source, sourceRef string
			_ = apRows.Scan(&id, &phone, &created, &contactID, &oppID, &campaignID, &source, &sourceRef)
			if contactID == 0 {
				contactID = a.contactIDForAttributionV24(tid, phone, "")
			}
			var leadID, postID, agentID int64
			var channel, owner string
			if contactID > 0 && oppID == 0 {
				_ = a.db.QueryRow(`SELECT id,lead_id,campaign_id,social_post_id,agent_id,channel,owner FROM crm_opportunities WHERE tenant_id=? AND contact_id=? AND COALESCE(deleted_at,'')='' ORDER BY last_activity_at DESC,id DESC LIMIT 1`, tid, contactID).Scan(&oppID, &leadID, &campaignID, &postID, &agentID, &channel, &owner)
			} else if oppID > 0 {
				_ = a.db.QueryRow(`SELECT lead_id,campaign_id,social_post_id,agent_id,channel,owner FROM crm_opportunities WHERE tenant_id=? AND id=?`, tid, oppID).Scan(&leadID, &campaignID, &postID, &agentID, &channel, &owner)
			}
			if source == "" {
				source = "agenda"
			}
			if sourceRef == "" && oppID > 0 {
				sourceRef = fmt.Sprintf("opportunity:%d", oppID)
			}
			_, _ = a.db.Exec(`UPDATE crm_appointments SET contact_id=?,opportunity_id=?,campaign_id=?,source=?,source_ref=? WHERE tenant_id=? AND id=?`, contactID, oppID, campaignID, source, sourceRef, tid, id)
			a.recordAnalyticsEventV24(tid, fmt.Sprintf("appointment:%d", id), "appointment", map[string]any{"contact_id": contactID, "lead_id": leadID, "opportunity_id": oppID, "appointment_id": id, "campaign_id": campaignID, "social_post_id": postID, "agent_id": agentID, "owner": owner, "channel": channel, "source": source, "source_ref": sourceRef, "occurred_at": created})
		}
		apRows.Close()
	}
	// Conversation first-touch events are channel-level when no acquisition identity exists.
	convRows, err := a.db.Query(`SELECT channel,chat_jid,MIN(timestamp) FROM worktic_messages WHERE tenant_id=? AND direction='in' GROUP BY channel,chat_jid`, tid)
	if err == nil {
		for convRows.Next() {
			var channel, chat, occurred string
			_ = convRows.Scan(&channel, &chat, &occurred)
			a.recordAnalyticsEventV24(tid, "conversation:"+channel+":"+chat, "conversation", map[string]any{"channel": channel, "source": "inbox", "source_ref": chat, "occurred_at": occurred})
		}
		convRows.Close()
	}
}

func (a *App) runAnalyticsV24Sync() {
	t := time.NewTicker(60 * time.Second)
	defer t.Stop()
	for {
		rows, err := a.db.Query(`SELECT id FROM tenants`)
		if err == nil {
			var ids []int64
			for rows.Next() {
				var id int64
				if rows.Scan(&id) == nil {
					ids = append(ids, id)
				}
			}
			rows.Close()
			for _, id := range ids {
				a.syncAnalyticsV24Tenant(id)
			}
		}
		<-t.C
	}
}

func (a *App) analyticsV24SettingsHandler(w http.ResponseWriter, r *http.Request) {
	tid, u, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, _ = a.db.Exec(`INSERT OR IGNORE INTO analytics_settings_v24(tenant_id,attribution_model,currency,lookback_days,updated_at) VALUES(?,'last_touch','USD',90,?)`, tid, now)
	if r.Method == http.MethodGet {
		var model, currency string
		var lookback int
		_ = a.db.QueryRow(`SELECT attribution_model,currency,lookback_days FROM analytics_settings_v24 WHERE tenant_id=?`, tid).Scan(&model, &currency, &lookback)
		role := normalizeSocialRole(u.Role)
		writeJSON(w, map[string]any{"attribution_model": model, "currency": currency, "lookback_days": lookback, "can_manage": role == "owner" || role == "admin"})
		return
	}
	if r.Method != http.MethodPut {
		http.Error(w, "Método no permitido", 405)
		return
	}
	role := normalizeSocialRole(u.Role)
	if role != "owner" && role != "admin" {
		http.Error(w, "Solo owner/admin puede cambiar Analytics", 403)
		return
	}
	var q struct {
		AttributionModel string `json:"attribution_model"`
		Currency         string `json:"currency"`
		LookbackDays     int    `json:"lookback_days"`
	}
	if json.NewDecoder(r.Body).Decode(&q) != nil {
		http.Error(w, "Datos inválidos", 400)
		return
	}
	if q.AttributionModel != "first_touch" {
		q.AttributionModel = "last_touch"
	}
	q.Currency = strings.ToUpper(strings.TrimSpace(q.Currency))
	if q.Currency != "USD" && q.Currency != "COP" && q.Currency != "EUR" && q.Currency != "MXN" {
		q.Currency = "USD"
	}
	if q.LookbackDays < 1 || q.LookbackDays > 730 {
		q.LookbackDays = 90
	}
	_, err = a.db.Exec(`INSERT INTO analytics_settings_v24(tenant_id,attribution_model,currency,lookback_days,updated_at) VALUES(?,?,?,?,?) ON CONFLICT(tenant_id) DO UPDATE SET attribution_model=excluded.attribution_model,currency=excluded.currency,lookback_days=excluded.lookback_days,updated_at=excluded.updated_at`, tid, q.AttributionModel, q.Currency, q.LookbackDays, now)
	if err != nil {
		writeError(w, err, 500)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "attribution_model": q.AttributionModel, "currency": q.Currency, "lookback_days": q.LookbackDays})
}

func parseAnalyticsRangeV24(r *http.Request) (string, string) {
	now := time.Now().UTC()
	from := strings.TrimSpace(r.URL.Query().Get("from"))
	to := strings.TrimSpace(r.URL.Query().Get("to"))
	if from == "" {
		from = now.AddDate(0, 0, -29).Format("2006-01-02")
	}
	if to == "" {
		to = now.Format("2006-01-02")
	}
	return from, to
}

func leadAttributionExprV24(model string) (channel, campaign, post string) {
	if model == "first_touch" {
		return `COALESCE(NULLIF(l.first_touch_channel,''),NULLIF(l.attribution_channel,''),NULLIF(l.utm_source,''),NULLIF(l.source,''),'direct')`, `CASE WHEN l.first_touch_campaign_id>0 THEN l.first_touch_campaign_id ELSE l.campaign_id END`, `CASE WHEN l.first_touch_post_id>0 THEN l.first_touch_post_id ELSE l.social_post_id END`
	}
	return `COALESCE(NULLIF(l.last_touch_channel,''),NULLIF(l.attribution_channel,''),NULLIF(l.utm_source,''),NULLIF(l.source,''),'direct')`, `CASE WHEN l.last_touch_campaign_id>0 THEN l.last_touch_campaign_id ELSE l.campaign_id END`, `CASE WHEN l.last_touch_post_id>0 THEN l.last_touch_post_id ELSE l.social_post_id END`
}

func ratioV24(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}

func (a *App) analyticsV24OverviewHandler(w http.ResponseWriter, r *http.Request) {
	tid, _, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", 405)
		return
	}
	a.syncAnalyticsV24Tenant(tid)
	from, to := parseAnalyticsRangeV24(r)
	var defaultModel, currency string
	var lookback int
	_ = a.db.QueryRow(`SELECT attribution_model,currency,lookback_days FROM analytics_settings_v24 WHERE tenant_id=?`, tid).Scan(&defaultModel, &currency, &lookback)
	if defaultModel == "" {
		defaultModel = "last_touch"
	}
	if currency == "" {
		currency = "USD"
	}
	model := strings.TrimSpace(r.URL.Query().Get("model"))
	if model == "" {
		model = defaultModel
	}
	if model != "first_touch" {
		model = "last_touch"
	}
	channelExpr, campaignExpr, postExpr := leadAttributionExprV24(model)
	end := to + "T23:59:59"
	start := from + "T00:00:00"
	var leads, conversations, appointments, opportunities, sales, clicks int64
	var revenue, spend float64
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM marketing_leads WHERE tenant_id=? AND created_at>=? AND created_at<=?`, tid, start, end).Scan(&leads)
	_ = a.db.QueryRow(`SELECT COUNT(DISTINCT chat_jid) FROM worktic_messages WHERE tenant_id=? AND direction='in' AND timestamp>=? AND timestamp<=?`, tid, start, end).Scan(&conversations)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM crm_appointments WHERE tenant_id=? AND created_at>=? AND created_at<=?`, tid, start, end).Scan(&appointments)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM crm_opportunities WHERE tenant_id=? AND COALESCE(deleted_at,'')='' AND created_at>=? AND created_at<=?`, tid, start, end).Scan(&opportunities)
	_ = a.db.QueryRow(`SELECT COUNT(*),COALESCE(SUM(value),0) FROM crm_opportunities WHERE tenant_id=? AND COALESCE(deleted_at,'')='' AND stage='Ganado' AND COALESCE(NULLIF(closed_at,''),updated_at)>=? AND COALESCE(NULLIF(closed_at,''),updated_at)<=?`, tid, start, end).Scan(&sales, &revenue)
	// V25: gasto oficial de Ads Center + gasto manual de campañas que no estén mapeadas a una cuenta publicitaria.
	var officialSpend, manualUnmappedSpend float64
	_ = a.db.QueryRow(`SELECT COALESCE(SUM(m.spend),0) FROM ads_daily_metrics_v25 m JOIN ads_campaigns_v25 c ON c.id=m.campaign_row_id AND c.tenant_id=m.tenant_id WHERE m.tenant_id=? AND m.metric_date>=? AND m.metric_date<=? AND (c.currency='' OR UPPER(c.currency)=UPPER(?))`, tid, from, to, currency).Scan(&officialSpend)
	_ = a.db.QueryRow(`SELECT COALESCE(SUM(mm.spend),0) FROM marketing_metrics mm WHERE mm.tenant_id=? AND mm.metric_date>=? AND mm.metric_date<=? AND NOT EXISTS(SELECT 1 FROM ads_campaigns_v25 ac WHERE ac.tenant_id=mm.tenant_id AND ac.internal_campaign_id=mm.campaign_id AND ac.internal_campaign_id>0)`, tid, from, to).Scan(&manualUnmappedSpend)
	spend = officialSpend + manualUnmappedSpend
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM analytics_attribution_events_v24 WHERE tenant_id=? AND event_type='content_click' AND occurred_at>=? AND occurred_at<=?`, tid, start, end).Scan(&clicks)

	byChannel := map[string]*analyticsDimV24{}
	getDim := func(k string) *analyticsDimV24 {
		k = strings.TrimSpace(k)
		if k == "" {
			k = "direct"
		}
		if byChannel[k] == nil {
			byChannel[k] = &analyticsDimV24{Key: k, Label: k}
		}
		return byChannel[k]
	}
	rows, _ := a.db.Query(`SELECT `+channelExpr+`,COUNT(*) FROM marketing_leads l WHERE l.tenant_id=? AND l.created_at>=? AND l.created_at<=? GROUP BY 1`, tid, start, end)
	if rows != nil {
		for rows.Next() {
			var k string
			var n int64
			_ = rows.Scan(&k, &n)
			getDim(k).Leads = n
		}
		rows.Close()
	}
	rows, _ = a.db.Query(`SELECT COALESCE((SELECT `+channelExpr+` FROM marketing_leads l WHERE l.tenant_id=o.tenant_id AND l.id=o.lead_id),NULLIF(o.channel,''),NULLIF(o.source,''),'direct'),COUNT(*),COALESCE(SUM(o.value),0) FROM crm_opportunities o WHERE o.tenant_id=? AND o.stage='Ganado' AND COALESCE(o.deleted_at,'')='' AND COALESCE(NULLIF(o.closed_at,''),o.updated_at)>=? AND COALESCE(NULLIF(o.closed_at,''),o.updated_at)<=? GROUP BY 1`, tid, start, end)
	if rows != nil {
		for rows.Next() {
			var k string
			var n int64
			var rev float64
			_ = rows.Scan(&k, &n, &rev)
			d := getDim(k)
			d.Sales = n
			d.Revenue = rev
		}
		rows.Close()
	}
	rows, _ = a.db.Query(`SELECT COALESCE((SELECT `+channelExpr+` FROM marketing_leads l WHERE l.tenant_id=o.tenant_id AND l.id=o.lead_id),NULLIF(o.channel,''),NULLIF(a.source,''),'direct'),COUNT(*) FROM crm_appointments a LEFT JOIN crm_opportunities o ON o.tenant_id=a.tenant_id AND o.id=a.opportunity_id WHERE a.tenant_id=? AND a.created_at>=? AND a.created_at<=? GROUP BY 1`, tid, start, end)
	if rows != nil {
		for rows.Next() {
			var k string
			var n int64
			_ = rows.Scan(&k, &n)
			getDim(k).Appointments = n
		}
		rows.Close()
	}
	rows, _ = a.db.Query(`SELECT platform,COALESCE(SUM(clicks),0),COALESCE(SUM(reach),0) FROM social_posts p LEFT JOIN social_metrics m ON m.post_id=p.id AND m.tenant_id=p.tenant_id AND m.metric_date>=? AND m.metric_date<=? WHERE p.tenant_id=? GROUP BY platform`, from, to, tid)
	if rows != nil {
		for rows.Next() {
			var k string
			var c, rr int64
			_ = rows.Scan(&k, &c, &rr)
			d := getDim(k)
			d.Clicks = c
			d.Reach = rr
		}
		rows.Close()
	}
	// Ads Center V25: cuando la plataforma no permite atribuir el gasto a una red orgánica concreta,
	// se muestra como dimensión pagada separada (meta_ads, tiktok_ads, google_ads).
	paidRows, _ := a.db.Query(`SELECT m.provider,COALESCE(SUM(m.spend),0) FROM ads_daily_metrics_v25 m JOIN ads_campaigns_v25 c ON c.id=m.campaign_row_id AND c.tenant_id=m.tenant_id WHERE m.tenant_id=? AND m.metric_date>=? AND m.metric_date<=? AND (c.currency='' OR UPPER(c.currency)=UPPER(?)) GROUP BY m.provider`, tid, from, to, currency)
	if paidRows != nil {
		for paidRows.Next() {
			var provider string
			var sp float64
			_ = paidRows.Scan(&provider, &sp)
			getDim(provider + "_ads").Spend = sp
		}
		paidRows.Close()
	}

	channels := make([]analyticsDimV24, 0, len(byChannel))
	for _, d := range byChannel {
		channels = append(channels, *d)
	}
	sort.Slice(channels, func(i, j int) bool {
		return channels[i].Revenue > channels[j].Revenue || (channels[i].Revenue == channels[j].Revenue && channels[i].Leads > channels[j].Leads)
	})

	campaigns := []map[string]any{}
	campRows, _ := a.db.Query(`SELECT c.id,c.name,COALESCE((SELECT SUM(spend) FROM marketing_metrics mm WHERE mm.tenant_id=c.tenant_id AND mm.campaign_id=c.id AND mm.metric_date>=? AND mm.metric_date<=?),0),COALESCE((SELECT COUNT(*) FROM marketing_leads l WHERE l.tenant_id=c.tenant_id AND `+campaignExpr+`=c.id AND l.created_at>=? AND l.created_at<=?),0),COALESCE((SELECT COUNT(*) FROM crm_appointments ap LEFT JOIN crm_opportunities o ON o.tenant_id=ap.tenant_id AND o.id=ap.opportunity_id WHERE ap.tenant_id=c.tenant_id AND ap.created_at>=? AND ap.created_at<=? AND (ap.campaign_id=c.id OR EXISTS(SELECT 1 FROM marketing_leads l WHERE l.tenant_id=ap.tenant_id AND l.id=o.lead_id AND `+campaignExpr+`=c.id))),0),COALESCE((SELECT COUNT(*) FROM crm_opportunities o WHERE o.tenant_id=c.tenant_id AND (EXISTS(SELECT 1 FROM marketing_leads l WHERE l.tenant_id=o.tenant_id AND l.id=o.lead_id AND `+campaignExpr+`=c.id) OR (o.lead_id=0 AND o.campaign_id=c.id)) AND o.stage='Ganado' AND COALESCE(NULLIF(o.closed_at,''),o.updated_at)>=? AND COALESCE(NULLIF(o.closed_at,''),o.updated_at)<=?),0),COALESCE((SELECT SUM(o.value) FROM crm_opportunities o WHERE o.tenant_id=c.tenant_id AND (EXISTS(SELECT 1 FROM marketing_leads l WHERE l.tenant_id=o.tenant_id AND l.id=o.lead_id AND `+campaignExpr+`=c.id) OR (o.lead_id=0 AND o.campaign_id=c.id)) AND o.stage='Ganado' AND COALESCE(NULLIF(o.closed_at,''),o.updated_at)>=? AND COALESCE(NULLIF(o.closed_at,''),o.updated_at)<=?),0) FROM marketing_campaigns c WHERE c.tenant_id=? ORDER BY 7 DESC,4 DESC LIMIT 30`, from, to, start, end, start, end, start, end, start, end, tid)
	if campRows != nil {
		defer campRows.Close()
		for campRows.Next() {
			var id, le, ap, sa int64
			var name string
			var sp, rev float64
			_ = campRows.Scan(&id, &name, &sp, &le, &ap, &sa, &rev)
			campaigns = append(campaigns, map[string]any{"id": id, "name": name, "spend": sp, "leads": le, "appointments": ap, "sales": sa, "revenue": rev, "cpl": ratioV24(sp, float64(le)), "cac": ratioV24(sp, float64(sa)), "roas": ratioV24(rev, sp)})
		}
	}

	content := []map[string]any{}
	contentRows, _ := a.db.Query(`SELECT p.id,p.platform,p.title,p.published_url,COALESCE(SUM(m.reach),0),COALESCE(SUM(m.clicks),0),COALESCE((SELECT COUNT(*) FROM marketing_leads l WHERE l.tenant_id=p.tenant_id AND `+postExpr+`=p.id AND l.created_at>=? AND l.created_at<=?),0),COALESCE((SELECT COUNT(*) FROM crm_appointments ap LEFT JOIN crm_opportunities o ON o.tenant_id=ap.tenant_id AND o.id=ap.opportunity_id WHERE ap.tenant_id=p.tenant_id AND ap.created_at>=? AND ap.created_at<=? AND EXISTS(SELECT 1 FROM marketing_leads l WHERE l.tenant_id=ap.tenant_id AND l.id=o.lead_id AND `+postExpr+`=p.id)),0),COALESCE((SELECT COUNT(*) FROM crm_opportunities o WHERE o.tenant_id=p.tenant_id AND (EXISTS(SELECT 1 FROM marketing_leads l WHERE l.tenant_id=o.tenant_id AND l.id=o.lead_id AND `+postExpr+`=p.id) OR (o.lead_id=0 AND o.social_post_id=p.id)) AND o.stage='Ganado' AND COALESCE(NULLIF(o.closed_at,''),o.updated_at)>=? AND COALESCE(NULLIF(o.closed_at,''),o.updated_at)<=?),0),COALESCE((SELECT SUM(o.value) FROM crm_opportunities o WHERE o.tenant_id=p.tenant_id AND (EXISTS(SELECT 1 FROM marketing_leads l WHERE l.tenant_id=o.tenant_id AND l.id=o.lead_id AND `+postExpr+`=p.id) OR (o.lead_id=0 AND o.social_post_id=p.id)) AND o.stage='Ganado' AND COALESCE(NULLIF(o.closed_at,''),o.updated_at)>=? AND COALESCE(NULLIF(o.closed_at,''),o.updated_at)<=?),0) FROM social_posts p LEFT JOIN social_metrics m ON m.post_id=p.id AND m.tenant_id=p.tenant_id AND m.metric_date>=? AND m.metric_date<=? WHERE p.tenant_id=? AND p.status='published' GROUP BY p.id ORDER BY 10 DESC,7 DESC,5 DESC LIMIT 20`, start, end, start, end, start, end, start, end, from, to, tid)
	if contentRows != nil {
		defer contentRows.Close()
		for contentRows.Next() {
			var id, reach, cl, le, ap, sa int64
			var platform, title, purl string
			var rev float64
			_ = contentRows.Scan(&id, &platform, &title, &purl, &reach, &cl, &le, &ap, &sa, &rev)
			content = append(content, map[string]any{"id": id, "platform": platform, "title": title, "published_url": purl, "reach": reach, "clicks": cl, "leads": le, "appointments": ap, "sales": sa, "revenue": rev})
		}
	}

	owners := []map[string]any{}
	ownerRows, _ := a.db.Query(`SELECT COALESCE(NULLIF(owner,''),'Sin asignar'),COUNT(*),SUM(CASE WHEN stage='Ganado' THEN 1 ELSE 0 END),COALESCE(SUM(CASE WHEN stage='Ganado' THEN value ELSE 0 END),0) FROM crm_opportunities WHERE tenant_id=? AND COALESCE(deleted_at,'')='' AND updated_at>=? AND updated_at<=? GROUP BY 1 ORDER BY 4 DESC`, tid, start, end)
	if ownerRows != nil {
		defer ownerRows.Close()
		for ownerRows.Next() {
			var name string
			var opp, won int64
			var rev float64
			_ = ownerRows.Scan(&name, &opp, &won, &rev)
			owners = append(owners, map[string]any{"owner": name, "opportunities": opp, "sales": won, "revenue": rev, "conversion_rate": ratioV24(float64(won)*100, float64(opp))})
		}
	}

	agents := []map[string]any{}
	agentRows, _ := a.db.Query(`SELECT a.id,a.name,COUNT(o.id),SUM(CASE WHEN o.stage='Ganado' THEN 1 ELSE 0 END),COALESCE(SUM(CASE WHEN o.stage='Ganado' THEN o.value ELSE 0 END),0) FROM ai_agents a LEFT JOIN crm_opportunities o ON o.tenant_id=a.tenant_id AND o.agent_id=a.id AND COALESCE(o.deleted_at,'')='' AND o.updated_at>=? AND o.updated_at<=? WHERE a.tenant_id=? GROUP BY a.id ORDER BY 5 DESC,3 DESC`, start, end, tid)
	if agentRows != nil {
		defer agentRows.Close()
		for agentRows.Next() {
			var id, opp, won int64
			var name string
			var rev float64
			_ = agentRows.Scan(&id, &name, &opp, &won, &rev)
			agents = append(agents, map[string]any{"id": id, "name": name, "opportunities": opp, "sales": won, "revenue": rev})
		}
	}

	trendMap := map[string]map[string]any{}
	for d := from; ; {
		trendMap[d] = map[string]any{"date": d, "leads": int64(0), "appointments": int64(0), "sales": int64(0), "revenue": float64(0)}
		tt, _ := time.Parse("2006-01-02", d)
		if d >= to {
			break
		}
		d = tt.AddDate(0, 0, 1).Format("2006-01-02")
	}
	tr, _ := a.db.Query(`SELECT substr(created_at,1,10),COUNT(*) FROM marketing_leads l WHERE l.tenant_id=? AND l.created_at>=? AND l.created_at<=? GROUP BY 1`, tid, start, end)
	if tr != nil {
		for tr.Next() {
			var d string
			var n int64
			_ = tr.Scan(&d, &n)
			if trendMap[d] != nil {
				trendMap[d]["leads"] = n
			}
		}
		tr.Close()
	}
	tr, _ = a.db.Query(`SELECT substr(created_at,1,10),COUNT(*) FROM crm_appointments WHERE tenant_id=? AND created_at>=? AND created_at<=? GROUP BY 1`, tid, start, end)
	if tr != nil {
		for tr.Next() {
			var d string
			var n int64
			_ = tr.Scan(&d, &n)
			if trendMap[d] != nil {
				trendMap[d]["appointments"] = n
			}
		}
		tr.Close()
	}
	tr, _ = a.db.Query(`SELECT substr(COALESCE(NULLIF(closed_at,''),updated_at),1,10),COUNT(*),COALESCE(SUM(value),0) FROM crm_opportunities WHERE tenant_id=? AND stage='Ganado' AND COALESCE(deleted_at,'')='' AND COALESCE(NULLIF(closed_at,''),updated_at)>=? AND COALESCE(NULLIF(closed_at,''),updated_at)<=? GROUP BY 1`, tid, start, end)
	if tr != nil {
		for tr.Next() {
			var d string
			var n int64
			var rev float64
			_ = tr.Scan(&d, &n, &rev)
			if trendMap[d] != nil {
				trendMap[d]["sales"] = n
				trendMap[d]["revenue"] = rev
			}
		}
		tr.Close()
	}
	trend := []map[string]any{}
	keys := make([]string, 0, len(trendMap))
	for k := range trendMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		trend = append(trend, trendMap[k])
	}

	writeJSON(w, map[string]any{"from": from, "to": to, "model": model, "currency": currency, "lookback_days": lookback, "kpis": map[string]any{"leads": leads, "conversations": conversations, "appointments": appointments, "opportunities": opportunities, "sales": sales, "revenue": revenue, "spend": spend, "tracked_clicks": clicks, "lead_to_sale_rate": ratioV24(float64(sales)*100, float64(leads)), "cpl": ratioV24(spend, float64(leads)), "cac": ratioV24(spend, float64(sales)), "roas": ratioV24(revenue, spend), "avg_deal": ratioV24(revenue, float64(sales))}, "funnel": map[string]any{"clicks": clicks, "leads": leads, "conversations": conversations, "appointments": appointments, "opportunities": opportunities, "sales": sales}, "by_channel": channels, "campaigns": campaigns, "content": content, "owners": owners, "agents": agents, "trend": trend})
}

func (a *App) analyticsV24JourneysHandler(w http.ResponseWriter, r *http.Request) {
	tid, _, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	a.syncAnalyticsV24Tenant(tid)
	rows, err := a.db.Query(`SELECT o.id,o.title,o.value,o.owner,o.channel,o.closed_at,o.lead_id,o.campaign_id,o.social_post_id,o.agent_id,COALESCE(c.name,''),COALESCE(mc.name,''),COALESCE(sp.title,''),COALESCE(sp.platform,''),COALESCE(a.name,'') FROM crm_opportunities o LEFT JOIN crm_contacts c ON c.id=o.contact_id AND c.tenant_id=o.tenant_id LEFT JOIN marketing_campaigns mc ON mc.id=o.campaign_id AND mc.tenant_id=o.tenant_id LEFT JOIN social_posts sp ON sp.id=o.social_post_id AND sp.tenant_id=o.tenant_id LEFT JOIN ai_agents a ON a.id=o.agent_id AND a.tenant_id=o.tenant_id WHERE o.tenant_id=? AND o.stage='Ganado' AND COALESCE(o.deleted_at,'')='' ORDER BY COALESCE(NULLIF(o.closed_at,''),o.updated_at) DESC LIMIT 50`, tid)
	if err != nil {
		writeError(w, err, 500)
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, leadID, campID, postID, agentID int64
		var title, owner, channel, closed, contact, campaign, postTitle, platform, agent string
		var value float64
		_ = rows.Scan(&id, &title, &value, &owner, &channel, &closed, &leadID, &campID, &postID, &agentID, &contact, &campaign, &postTitle, &platform, &agent)
		out = append(out, map[string]any{"opportunity_id": id, "title": title, "value": value, "owner": owner, "channel": channel, "closed_at": closed, "lead_id": leadID, "campaign_id": campID, "social_post_id": postID, "agent_id": agentID, "contact": contact, "campaign": campaign, "content": postTitle, "platform": platform, "agent": agent})
	}
	writeJSON(w, map[string]any{"items": out})
}

func (a *App) analyticsV24ExportHandler(w http.ResponseWriter, r *http.Request) {
	tid, _, err := a.tenantForRequest(r)
	if err != nil {
		http.Error(w, "Sesión requerida", 401)
		return
	}
	from, to := parseAnalyticsRangeV24(r)
	start, end := from+"T00:00:00", to+"T23:59:59"
	rows, err := a.db.Query(`SELECT o.id,COALESCE(c.name,''),o.title,o.value,o.owner,o.channel,COALESCE(mc.name,''),COALESCE(sp.platform,''),COALESCE(sp.title,''),COALESCE(a.name,''),COALESCE(NULLIF(o.closed_at,''),o.updated_at) FROM crm_opportunities o LEFT JOIN crm_contacts c ON c.id=o.contact_id AND c.tenant_id=o.tenant_id LEFT JOIN marketing_campaigns mc ON mc.id=o.campaign_id AND mc.tenant_id=o.tenant_id LEFT JOIN social_posts sp ON sp.id=o.social_post_id AND sp.tenant_id=o.tenant_id LEFT JOIN ai_agents a ON a.id=o.agent_id AND a.tenant_id=o.tenant_id WHERE o.tenant_id=? AND o.stage='Ganado' AND COALESCE(o.deleted_at,'')='' AND COALESCE(NULLIF(o.closed_at,''),o.updated_at)>=? AND COALESCE(NULLIF(o.closed_at,''),o.updated_at)<=? ORDER BY 11 DESC`, tid, start, end)
	if err != nil {
		http.Error(w, "No fue posible exportar", 500)
		return
	}
	defer rows.Close()
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="worktic-attribution-%s-%s.csv"`, from, to))
	fmt.Fprintln(w, "opportunity_id,contact,title,revenue,owner,channel,campaign,platform,content,agent,closed_at")
	csvEsc := func(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }
	for rows.Next() {
		var id int64
		var contact, title, owner, channel, campaign, platform, content, agent, closed string
		var value float64
		_ = rows.Scan(&id, &contact, &title, &value, &owner, &channel, &campaign, &platform, &content, &agent, &closed)
		fmt.Fprintf(w, "%d,%s,%s,%.2f,%s,%s,%s,%s,%s,%s,%s\n", id, csvEsc(contact), csvEsc(title), value, csvEsc(owner), csvEsc(channel), csvEsc(campaign), csvEsc(platform), csvEsc(content), csvEsc(agent), csvEsc(closed))
	}
}
