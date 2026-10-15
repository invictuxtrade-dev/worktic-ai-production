package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const partnerReferralCookieV2714 = "worktic_partner_ref"

func initPartnersV2714Schema(db *DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS partner_settings_v2714 (
 key TEXT PRIMARY KEY,
 value TEXT NOT NULL DEFAULT '',
 updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS partner_profiles_v2714 (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 user_id INTEGER NOT NULL UNIQUE,
 tenant_id INTEGER NOT NULL DEFAULT 0,
 partner_code TEXT NOT NULL UNIQUE,
 display_name TEXT NOT NULL DEFAULT '',
 status TEXT NOT NULL DEFAULT 'active',
 rank_code TEXT NOT NULL DEFAULT 'spark',
 rank_grace_started_at TEXT NOT NULL DEFAULT '',
 rank_protected_until TEXT NOT NULL DEFAULT '',
 sponsor_partner_user_id INTEGER NOT NULL DEFAULT 0,
 share_enabled INTEGER NOT NULL DEFAULT 1,
 sell_enabled INTEGER NOT NULL DEFAULT 1,
 activate_status TEXT NOT NULL DEFAULT 'not_certified',
 lead_enabled INTEGER NOT NULL DEFAULT 1,
 created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_partner_profiles_v2714_status ON partner_profiles_v2714(status,rank_code,created_at);
CREATE INDEX IF NOT EXISTS idx_partner_profiles_v2714_sponsor ON partner_profiles_v2714(sponsor_partner_user_id,status);

CREATE TABLE IF NOT EXISTS partner_clicks_v2714 (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 partner_user_id INTEGER NOT NULL,
 partner_code TEXT NOT NULL,
 visitor_token TEXT NOT NULL DEFAULT '',
 landing_path TEXT NOT NULL DEFAULT '',
 referrer TEXT NOT NULL DEFAULT '',
 ip_hash TEXT NOT NULL DEFAULT '',
 user_agent TEXT NOT NULL DEFAULT '',
 created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_partner_clicks_v2714_partner ON partner_clicks_v2714(partner_user_id,created_at DESC);

CREATE TABLE IF NOT EXISTS partner_attributions_v2714 (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 customer_user_id INTEGER NOT NULL UNIQUE,
 partner_user_id INTEGER NOT NULL,
 partner_code TEXT NOT NULL,
 source TEXT NOT NULL DEFAULT 'referral_link',
 visitor_token TEXT NOT NULL DEFAULT '',
 created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_partner_attributions_v2714_partner ON partner_attributions_v2714(partner_user_id,created_at DESC);

CREATE TABLE IF NOT EXISTS partner_rank_rules_v2714 (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 code TEXT NOT NULL UNIQUE,
 name TEXT NOT NULL,
 min_direct_active INTEGER NOT NULL DEFAULT 0,
 min_adopted INTEGER NOT NULL DEFAULT 0,
 direct_percent REAL NOT NULL DEFAULT 0,
 level2_percent REAL NOT NULL DEFAULT 0,
 level3_percent REAL NOT NULL DEFAULT 0,
 sort_order INTEGER NOT NULL DEFAULT 0,
 active INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE IF NOT EXISTS partner_rank_snapshots_v2714 (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 partner_user_id INTEGER NOT NULL,
 rank_code TEXT NOT NULL,
 direct_active INTEGER NOT NULL DEFAULT 0,
 adopted_active INTEGER NOT NULL DEFAULT 0,
 level2_partners INTEGER NOT NULL DEFAULT 0,
 level3_partners INTEGER NOT NULL DEFAULT 0,
 evaluated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_partner_rank_snapshots_v2714_partner ON partner_rank_snapshots_v2714(partner_user_id,evaluated_at DESC);

CREATE TABLE IF NOT EXISTS partner_adoption_v2714 (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 customer_user_id INTEGER NOT NULL UNIQUE,
 specialist_partner_user_id INTEGER NOT NULL DEFAULT 0,
 impact_score INTEGER NOT NULL DEFAULT 0,
 status TEXT NOT NULL DEFAULT 'pending',
 notes TEXT NOT NULL DEFAULT '',
 approved_by INTEGER NOT NULL DEFAULT 0,
 approved_at TEXT NOT NULL DEFAULT '',
 created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_partner_adoption_v2714_specialist ON partner_adoption_v2714(specialist_partner_user_id,status,impact_score);

CREATE TABLE IF NOT EXISTS partner_commission_ledger_v2714 (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 partner_user_id INTEGER NOT NULL,
 customer_user_id INTEGER NOT NULL DEFAULT 0,
 payment_id INTEGER NOT NULL DEFAULT 0,
 commission_type TEXT NOT NULL,
 level INTEGER NOT NULL DEFAULT 0,
 eligible_base REAL NOT NULL DEFAULT 0,
 percent REAL NOT NULL DEFAULT 0,
 amount REAL NOT NULL DEFAULT 0,
 currency TEXT NOT NULL DEFAULT 'USD',
 status TEXT NOT NULL DEFAULT 'pending',
 available_at TEXT NOT NULL DEFAULT '',
 dedupe_key TEXT NOT NULL UNIQUE,
 metadata_json TEXT NOT NULL DEFAULT '{}',
 created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_partner_ledger_v2714_partner ON partner_commission_ledger_v2714(partner_user_id,status,created_at DESC);
CREATE INDEX IF NOT EXISTS idx_partner_ledger_v2714_payment ON partner_commission_ledger_v2714(payment_id,commission_type);

CREATE TABLE IF NOT EXISTS partner_payouts_v2714 (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 partner_user_id INTEGER NOT NULL,
 amount REAL NOT NULL DEFAULT 0,
 currency TEXT NOT NULL DEFAULT 'USD',
 status TEXT NOT NULL DEFAULT 'requested',
 method TEXT NOT NULL DEFAULT '',
 destination TEXT NOT NULL DEFAULT '',
 admin_note TEXT NOT NULL DEFAULT '',
 requested_at TEXT NOT NULL,
 reviewed_at TEXT NOT NULL DEFAULT '',
 paid_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_partner_payouts_v2714_partner ON partner_payouts_v2714(partner_user_id,status,requested_at DESC);

CREATE TABLE IF NOT EXISTS partner_audit_v2714 (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 actor_user_id INTEGER NOT NULL DEFAULT 0,
 partner_user_id INTEGER NOT NULL DEFAULT 0,
 event TEXT NOT NULL,
 metadata_json TEXT NOT NULL DEFAULT '{}',
 created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_partner_audit_v2714_partner ON partner_audit_v2714(partner_user_id,created_at DESC);
`)
	if err != nil {
		return err
	}
	_, _ = db.Exec(`ALTER TABLE partner_profiles_v2714 ADD COLUMN rank_grace_started_at TEXT NOT NULL DEFAULT ''`)
	_, _ = db.Exec(`ALTER TABLE partner_profiles_v2714 ADD COLUMN rank_protected_until TEXT NOT NULL DEFAULT ''`)
	now := time.Now().UTC().Format(time.RFC3339)
	for k, v := range map[string]string{
		"commission_hold_days":  "14",
		"adoption_score_min":    "70",
		"adoption_percent":      "4",
		"renewal_percent":       "2",
		"mentorship_percent":    "2",
		"cosell_referrer_split": "70",
		"currency":              "USD",
	} {
		_, _ = db.Exec(`INSERT OR IGNORE INTO partner_settings_v2714(key,value,updated_at) VALUES(?,?,?)`, k, v, now)
	}
	ranks := []struct {
		code, name                      string
		direct, adopted, order          int
		directPct, level2Pct, level3Pct float64
	}{
		{"spark", "Spark", 0, 0, 1, 25, 0, 0},
		{"pulse", "Pulse", 3, 2, 2, 28, 0, 0},
		{"orbit", "Orbit", 8, 6, 3, 30, 3, 0},
		{"apex", "Apex", 18, 14, 4, 32, 4, 2},
		{"infinity", "Infinity", 35, 28, 5, 34, 5, 3},
	}
	for _, x := range ranks {
		_, _ = db.Exec(`INSERT OR IGNORE INTO partner_rank_rules_v2714(code,name,min_direct_active,min_adopted,direct_percent,level2_percent,level3_percent,sort_order,active) VALUES(?,?,?,?,?,?,?,?,1)`, x.code, x.name, x.direct, x.adopted, x.directPct, x.level2Pct, x.level3Pct, x.order)
	}
	return nil
}

func normalizePartnerCodeV2714(v string) string {
	v = strings.ToUpper(strings.TrimSpace(v))
	var b strings.Builder
	for _, r := range v {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (a *App) uniquePartnerCodeV2714(userID int64) string {
	for i := 0; i < 8; i++ {
		code := fmt.Sprintf("WT%06d%s", userID%1000000, strings.ToUpper(randomToken(2)))
		var count int
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM partner_profiles_v2714 WHERE partner_code=?`, code).Scan(&count)
		if count == 0 {
			return code
		}
	}
	return fmt.Sprintf("WT%d%s", userID, strings.ToUpper(randomToken(4)))
}

func (a *App) partnerSettingIntV2714(key string, fallback int) int {
	var raw string
	if a.db.QueryRow(`SELECT value FROM partner_settings_v2714 WHERE key=?`, key).Scan(&raw) == nil {
		if n, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil {
			return n
		}
	}
	return fallback
}

func (a *App) partnerSettingFloatV2714(key string, fallback float64) float64 {
	var raw string
	if a.db.QueryRow(`SELECT value FROM partner_settings_v2714 WHERE key=?`, key).Scan(&raw) == nil {
		if n, err := strconv.ParseFloat(strings.TrimSpace(raw), 64); err == nil {
			return n
		}
	}
	return fallback
}

func (a *App) partnerAuditV2714(actorID, partnerID int64, event string, metadata any) {
	raw, _ := json.Marshal(metadata)
	_, _ = a.db.Exec(`INSERT INTO partner_audit_v2714(actor_user_id,partner_user_id,event,metadata_json,created_at) VALUES(?,?,?,?,?)`, actorID, partnerID, strings.TrimSpace(event), string(raw), time.Now().UTC().Format(time.RFC3339))
}

func (a *App) capturePartnerAttributionTxV2714(tx *Tx, customerUserID int64, code string, now time.Time) error {
	code = normalizePartnerCodeV2714(code)
	if code == "" || customerUserID == 0 {
		return nil
	}
	var partnerUserID int64
	var status string
	if err := tx.QueryRow(`SELECT user_id,status FROM partner_profiles_v2714 WHERE partner_code=?`, code).Scan(&partnerUserID, &status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	if status != "active" || partnerUserID == customerUserID {
		return nil
	}
	_, err := tx.Exec(`INSERT OR IGNORE INTO partner_attributions_v2714(customer_user_id,partner_user_id,partner_code,source,created_at) VALUES(?,?,?,'referral_link',?)`, customerUserID, partnerUserID, code, now.UTC().Format(time.RFC3339))
	return err
}

func partnerRefFromRequestV2714(r *http.Request, explicit string) string {
	if v := normalizePartnerCodeV2714(explicit); v != "" {
		return v
	}
	if c, err := r.Cookie(partnerReferralCookieV2714); err == nil {
		return normalizePartnerCodeV2714(c.Value)
	}
	return ""
}

func (a *App) partnerReferralRedirectHandlerV2714(w http.ResponseWriter, r *http.Request) {
	code := normalizePartnerCodeV2714(strings.TrimPrefix(r.URL.Path, "/r/"))
	if code == "" {
		http.Redirect(w, r, "/partners.html", http.StatusFound)
		return
	}
	var partnerUserID int64
	var status string
	if err := a.db.QueryRow(`SELECT user_id,status FROM partner_profiles_v2714 WHERE partner_code=?`, code).Scan(&partnerUserID, &status); err != nil || status != "active" {
		http.Redirect(w, r, "/partners.html", http.StatusFound)
		return
	}
	visitor := randomToken(12)
	http.SetCookie(w, &http.Cookie{Name: partnerReferralCookieV2714, Value: code, Path: "/", MaxAge: 30 * 24 * 60 * 60, HttpOnly: true, Secure: a.cfg.AppEnv == "production", SameSite: http.SameSiteLaxMode})
	ipHash := fmt.Sprintf("%x", sha256BytesV2714(clientIPV26(r, a.cfg.TrustProxy)+"|"+r.UserAgent()))
	now := time.Now().UTC()
	var lastClick string
	_ = a.db.QueryRow(`SELECT created_at FROM partner_clicks_v2714 WHERE partner_user_id=? AND ip_hash=? ORDER BY id DESC LIMIT 1`, partnerUserID, ipHash).Scan(&lastClick)
	shouldLog := true
	if t, err := time.Parse(time.RFC3339, lastClick); err == nil && now.Sub(t) < 5*time.Minute {
		shouldLog = false
	}
	if shouldLog {
		_, _ = a.db.Exec(`INSERT INTO partner_clicks_v2714(partner_user_id,partner_code,visitor_token,landing_path,referrer,ip_hash,user_agent,created_at) VALUES(?,?,?,?,?,?,?,?)`, partnerUserID, code, visitor, r.URL.Path, r.Referer(), ipHash, truncatePartnerV2714(r.UserAgent(), 500), now.Format(time.RFC3339))
	}
	dst := "/login.html?tab=register&ref=" + url.QueryEscape(code)
	http.Redirect(w, r, dst, http.StatusFound)
}

func sha256BytesV2714(v string) []byte {
	h := sha256SumV2714(v)
	return h[:]
}

func sha256SumV2714(v string) [32]byte {
	return sha256.Sum256([]byte(v))
}

func truncatePartnerV2714(v string, n int) string {
	v = strings.TrimSpace(v)
	if len(v) <= n {
		return v
	}
	return v[:n]
}

type partnerRankRuleV2714 struct {
	Code            string  `json:"code"`
	Name            string  `json:"name"`
	MinDirectActive int     `json:"min_direct_active"`
	MinAdopted      int     `json:"min_adopted"`
	DirectPercent   float64 `json:"direct_percent"`
	Level2Percent   float64 `json:"level2_percent"`
	Level3Percent   float64 `json:"level3_percent"`
	SortOrder       int     `json:"sort_order"`
}

func (a *App) partnerMetricsV2714(partnerUserID int64) (direct, adopted, level2, level3 int) {
	now := time.Now().UTC().Format(time.RFC3339)
	_ = a.db.QueryRow(`SELECT COUNT(DISTINCT pa.customer_user_id) FROM partner_attributions_v2714 pa WHERE pa.partner_user_id=? AND EXISTS (SELECT 1 FROM billing_subscriptions s WHERE s.user_id=pa.customer_user_id AND s.status='active' AND s.plan_code<>'free' AND s.ends_at>?)`, partnerUserID, now).Scan(&direct)
	_ = a.db.QueryRow(`SELECT COUNT(DISTINCT pa.customer_user_id) FROM partner_attributions_v2714 pa JOIN partner_adoption_v2714 ad ON ad.customer_user_id=pa.customer_user_id WHERE pa.partner_user_id=? AND ad.status='approved' AND ad.impact_score>=? AND EXISTS (SELECT 1 FROM billing_subscriptions s WHERE s.user_id=pa.customer_user_id AND s.status='active' AND s.plan_code<>'free' AND s.ends_at>?)`, partnerUserID, a.partnerSettingIntV2714("adoption_score_min", 70), now).Scan(&adopted)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM partner_profiles_v2714 WHERE sponsor_partner_user_id=? AND status='active'`, partnerUserID).Scan(&level2)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM partner_profiles_v2714 p3 WHERE p3.status='active' AND p3.sponsor_partner_user_id IN (SELECT user_id FROM partner_profiles_v2714 WHERE sponsor_partner_user_id=? AND status='active')`, partnerUserID).Scan(&level3)
	return
}

func (a *App) partnerRankRuleByCodeV2714(code string) partnerRankRuleV2714 {
	var x partnerRankRuleV2714
	_ = a.db.QueryRow(`SELECT code,name,min_direct_active,min_adopted,direct_percent,level2_percent,level3_percent,sort_order FROM partner_rank_rules_v2714 WHERE code=? AND active=1`, code).Scan(&x.Code, &x.Name, &x.MinDirectActive, &x.MinAdopted, &x.DirectPercent, &x.Level2Percent, &x.Level3Percent, &x.SortOrder)
	if x.Code == "" {
		x = partnerRankRuleV2714{Code: "spark", Name: "Spark", DirectPercent: 25, SortOrder: 1}
	}
	return x
}

func (a *App) evaluatePartnerRankV2714(partnerUserID int64) partnerRankRuleV2714 {
	direct, adopted, level2, level3 := a.partnerMetricsV2714(partnerUserID)
	rows, err := a.db.Query(`SELECT code,name,min_direct_active,min_adopted,direct_percent,level2_percent,level3_percent,sort_order FROM partner_rank_rules_v2714 WHERE active=1 ORDER BY sort_order DESC`)
	candidate := partnerRankRuleV2714{Code: "spark", Name: "Spark", DirectPercent: 25, SortOrder: 1}
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var x partnerRankRuleV2714
			if rows.Scan(&x.Code, &x.Name, &x.MinDirectActive, &x.MinAdopted, &x.DirectPercent, &x.Level2Percent, &x.Level3Percent, &x.SortOrder) == nil && direct >= x.MinDirectActive && adopted >= x.MinAdopted {
				candidate = x
				break
			}
		}
	}
	var currentCode, graceStarted, protectedUntil string
	_ = a.db.QueryRow(`SELECT rank_code,COALESCE(rank_grace_started_at,''),COALESCE(rank_protected_until,'') FROM partner_profiles_v2714 WHERE user_id=?`, partnerUserID).Scan(&currentCode, &graceStarted, &protectedUntil)
	current := a.partnerRankRuleByCodeV2714(currentCode)
	selected := candidate
	now := time.Now().UTC()
	if current.Code != "" && candidate.SortOrder < current.SortOrder {
		if strings.TrimSpace(protectedUntil) == "" {
			graceStarted = now.Format(time.RFC3339)
			protectedUntil = now.AddDate(0, 0, 90).Format(time.RFC3339)
			selected = current
		} else if t, e := time.Parse(time.RFC3339, protectedUntil); e == nil && now.Before(t) {
			selected = current
		} else {
			graceStarted, protectedUntil = "", ""
			selected = candidate
		}
	} else {
		// Si mantiene o mejora el rango, no necesita periodo de protección activo.
		graceStarted, protectedUntil = "", ""
		selected = candidate
	}
	nowText := now.Format(time.RFC3339)
	_, _ = a.db.Exec(`UPDATE partner_profiles_v2714 SET rank_code=?,rank_grace_started_at=?,rank_protected_until=?,updated_at=? WHERE user_id=?`, selected.Code, graceStarted, protectedUntil, nowText, partnerUserID)
	var lastRank string
	var lastDirect, lastAdopted, lastL2, lastL3 int
	_ = a.db.QueryRow(`SELECT rank_code,direct_active,adopted_active,level2_partners,level3_partners FROM partner_rank_snapshots_v2714 WHERE partner_user_id=? ORDER BY id DESC LIMIT 1`, partnerUserID).Scan(&lastRank, &lastDirect, &lastAdopted, &lastL2, &lastL3)
	if lastRank != selected.Code || lastDirect != direct || lastAdopted != adopted || lastL2 != level2 || lastL3 != level3 {
		_, _ = a.db.Exec(`INSERT INTO partner_rank_snapshots_v2714(partner_user_id,rank_code,direct_active,adopted_active,level2_partners,level3_partners,evaluated_at) VALUES(?,?,?,?,?,?,?)`, partnerUserID, selected.Code, direct, adopted, level2, level3, nowText)
	}
	return selected
}

func (a *App) ensurePartnerProfileV2714(u *User) (map[string]any, error) {
	if u == nil {
		return nil, errors.New("sesión requerida")
	}
	var id, sponsor int64
	var code, displayName, status, rank, activateStatus, graceStarted, protectedUntil, createdAt, updatedAt string
	var share, sell, lead int
	err := a.db.QueryRow(`SELECT id,partner_code,display_name,status,rank_code,COALESCE(rank_grace_started_at,''),COALESCE(rank_protected_until,''),sponsor_partner_user_id,share_enabled,sell_enabled,activate_status,lead_enabled,created_at,updated_at FROM partner_profiles_v2714 WHERE user_id=?`, u.ID).Scan(&id, &code, &displayName, &status, &rank, &graceStarted, &protectedUntil, &sponsor, &share, &sell, &activateStatus, &lead, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		now := time.Now().UTC().Format(time.RFC3339)
		var sponsorID int64
		_ = a.db.QueryRow(`SELECT partner_user_id FROM partner_attributions_v2714 WHERE customer_user_id=?`, u.ID).Scan(&sponsorID)
		code = a.uniquePartnerCodeV2714(u.ID)
		res, e := a.db.Exec(`INSERT INTO partner_profiles_v2714(user_id,tenant_id,partner_code,display_name,status,rank_code,sponsor_partner_user_id,share_enabled,sell_enabled,activate_status,lead_enabled,created_at,updated_at) VALUES(?,?,?,?,'active','spark',?,1,1,'not_certified',1,?,?)`, u.ID, u.TenantID, code, u.Name, sponsorID, now, now)
		if e != nil {
			return nil, e
		}
		id, _ = res.LastInsertId()
		status, rank, sponsor, share, sell, lead, activateStatus, displayName, createdAt, updatedAt = "active", "spark", sponsorID, 1, 1, 1, "not_certified", u.Name, now, now
		a.partnerAuditV2714(u.ID, u.ID, "partner.activated", map[string]any{"partner_code": code, "sponsor_partner_user_id": sponsorID})
	} else if err != nil {
		return nil, err
	}
	rule := a.evaluatePartnerRankV2714(u.ID)
	_ = a.db.QueryRow(`SELECT status,COALESCE(rank_grace_started_at,''),COALESCE(rank_protected_until,'') FROM partner_profiles_v2714 WHERE user_id=?`, u.ID).Scan(&status, &graceStarted, &protectedUntil)
	return map[string]any{
		"id": id, "user_id": u.ID, "tenant_id": u.TenantID, "partner_code": code, "display_name": displayName,
		"status": status, "rank_code": rule.Code, "rank_name": rule.Name, "rank_grace_started_at": graceStarted, "rank_protected_until": protectedUntil, "sponsor_partner_user_id": sponsor,
		"share_enabled": share == 1, "sell_enabled": sell == 1, "activate_status": activateStatus, "lead_enabled": lead == 1,
		"created_at": createdAt, "updated_at": updatedAt,
	}, nil
}

func (a *App) partnerMeHandlerV2714(w http.ResponseWriter, r *http.Request) {
	u := a.currentUser(r)
	if u == nil {
		writeError(w, errors.New("sesión requerida"), 401)
		return
	}
	switch r.Method {
	case http.MethodGet:
		var count int
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM partner_profiles_v2714 WHERE user_id=?`, u.ID).Scan(&count)
		if count == 0 {
			writeJSON(w, map[string]any{"active": false, "can_activate": true})
			return
		}
		p, err := a.ensurePartnerProfileV2714(u)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		p["active"] = p["status"] == "active"
		writeJSON(w, p)
	case http.MethodPost:
		p, err := a.ensurePartnerProfileV2714(u)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		p["active"] = p["status"] == "active"
		p["can_activate"] = false
		writeJSON(w, p)
	default:
		http.Error(w, "Método no permitido", 405)
	}
}

func (a *App) releasePartnerCommissionsV2714(partnerUserID int64) {
	now := time.Now().UTC().Format(time.RFC3339)
	if partnerUserID > 0 {
		_, _ = a.db.Exec(`UPDATE partner_commission_ledger_v2714 SET status='available',updated_at=? WHERE partner_user_id=? AND status='pending' AND available_at<>'' AND available_at<=?`, now, partnerUserID, now)
	} else {
		_, _ = a.db.Exec(`UPDATE partner_commission_ledger_v2714 SET status='available',updated_at=? WHERE status='pending' AND available_at<>'' AND available_at<=?`, now, now)
	}
}

func (a *App) partnerOverviewHandlerV2714(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", 405)
		return
	}
	u := a.currentUser(r)
	if u == nil {
		writeError(w, errors.New("sesión requerida"), 401)
		return
	}
	var exists int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM partner_profiles_v2714 WHERE user_id=?`, u.ID).Scan(&exists)
	if exists == 0 {
		writeJSON(w, map[string]any{"active": false, "can_activate": true})
		return
	}
	profile, err := a.ensurePartnerProfileV2714(u)
	if err != nil {
		writeError(w, err, 500)
		return
	}
	if fmt.Sprint(profile["status"]) != "active" {
		writeJSON(w, map[string]any{"active": false, "can_activate": false, "suspended": true, "profile": profile})
		return
	}
	direct, adopted, level2, level3 := a.partnerMetricsV2714(u.ID)
	rank := a.partnerRankRuleByCodeV2714(fmt.Sprint(profile["rank_code"]))
	var clicks, signups int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM partner_clicks_v2714 WHERE partner_user_id=?`, u.ID).Scan(&clicks)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM partner_attributions_v2714 WHERE partner_user_id=?`, u.ID).Scan(&signups)
	a.releasePartnerCommissionsV2714(u.ID)
	balances := map[string]float64{"pending": 0, "available": 0, "paid": 0, "reversed": 0}
	rows, e := a.db.Query(`SELECT status,COALESCE(SUM(amount),0) FROM partner_commission_ledger_v2714 WHERE partner_user_id=? GROUP BY status`, u.ID)
	if e == nil {
		defer rows.Close()
		for rows.Next() {
			var s string
			var v float64
			if rows.Scan(&s, &v) == nil {
				balances[s] = roundUSDT(v)
			}
		}
	}
	code := fmt.Sprint(profile["partner_code"])
	base := strings.TrimRight(a.cfg.BaseURL, "/")
	if base == "" {
		base = "https://workticai.com"
	}
	writeJSON(w, map[string]any{
		"active":       true,
		"profile":      profile,
		"referral_url": base + "/r/" + code,
		"rank":         rank,
		"stats":        map[string]any{"clicks": clicks, "signups": signups, "direct_active": direct, "adopted_active": adopted, "level2_partners": level2, "level3_partners": level3},
		"balances":     balances,
	})
}

func (a *App) partnerRanksHandlerV2714(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", 405)
		return
	}
	rows, err := a.db.Query(`SELECT code,name,min_direct_active,min_adopted,direct_percent,level2_percent,level3_percent,sort_order FROM partner_rank_rules_v2714 WHERE active=1 ORDER BY sort_order ASC`)
	if err != nil {
		writeError(w, err, 500)
		return
	}
	defer rows.Close()
	out := []partnerRankRuleV2714{}
	for rows.Next() {
		var x partnerRankRuleV2714
		if rows.Scan(&x.Code, &x.Name, &x.MinDirectActive, &x.MinAdopted, &x.DirectPercent, &x.Level2Percent, &x.Level3Percent, &x.SortOrder) == nil {
			out = append(out, x)
		}
	}
	writeJSON(w, out)
}

func (a *App) partnerCustomersHandlerV2714(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", 405)
		return
	}
	u := a.currentUser(r)
	if u == nil {
		writeError(w, errors.New("sesión requerida"), 401)
		return
	}
	rows, err := a.db.Query(`SELECT pa.customer_user_id,cu.name,cu.email,pa.created_at,COALESCE(ad.impact_score,0),COALESCE(ad.status,'pending'),COALESCE((SELECT s.plan_code FROM billing_subscriptions s WHERE s.user_id=pa.customer_user_id AND s.status='active' ORDER BY s.ends_at DESC LIMIT 1),'free'),COALESCE((SELECT s.ends_at FROM billing_subscriptions s WHERE s.user_id=pa.customer_user_id AND s.status='active' ORDER BY s.ends_at DESC LIMIT 1),'') FROM partner_attributions_v2714 pa JOIN app_users cu ON cu.id=pa.customer_user_id LEFT JOIN partner_adoption_v2714 ad ON ad.customer_user_id=pa.customer_user_id WHERE pa.partner_user_id=? ORDER BY pa.id DESC`, u.ID)
	if err != nil {
		writeError(w, err, 500)
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id int64
		var name, email, created, adoptionStatus, plan, ends string
		var score int
		if rows.Scan(&id, &name, &email, &created, &score, &adoptionStatus, &plan, &ends) == nil {
			out = append(out, map[string]any{"user_id": id, "name": name, "email": email, "created_at": created, "impact_score": score, "adoption_status": adoptionStatus, "plan_code": plan, "plan_ends_at": ends})
		}
	}
	writeJSON(w, out)
}

func (a *App) partnerNetworkHandlerV2714(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", 405)
		return
	}
	u := a.currentUser(r)
	if u == nil {
		writeError(w, errors.New("sesión requerida"), 401)
		return
	}
	query := `SELECT p.user_id,u.name,u.email,p.partner_code,p.rank_code,p.activate_status,p.created_at FROM partner_profiles_v2714 p JOIN app_users u ON u.id=p.user_id WHERE p.sponsor_partner_user_id=? AND p.status='active' ORDER BY p.id DESC`
	level2 := []map[string]any{}
	level2IDs := []int64{}
	rows, err := a.db.Query(query, u.ID)
	if err == nil {
		for rows.Next() {
			var id int64
			var name, email, code, rank, activate, created string
			if rows.Scan(&id, &name, &email, &code, &rank, &activate, &created) == nil {
				level2 = append(level2, map[string]any{"user_id": id, "name": name, "email": email, "partner_code": code, "rank_code": rank, "activate_status": activate, "created_at": created})
				level2IDs = append(level2IDs, id)
			}
		}
		rows.Close()
	}
	level3 := []map[string]any{}
	for _, parentID := range level2IDs {
		rows, err := a.db.Query(query, parentID)
		if err != nil {
			continue
		}
		for rows.Next() {
			var id int64
			var name, email, code, rank, activate, created string
			if rows.Scan(&id, &name, &email, &code, &rank, &activate, &created) == nil {
				level3 = append(level3, map[string]any{"user_id": id, "sponsor_user_id": parentID, "name": name, "email": email, "partner_code": code, "rank_code": rank, "activate_status": activate, "created_at": created})
			}
		}
		rows.Close()
	}
	writeJSON(w, map[string]any{"level2": level2, "level3": level3})
}

func (a *App) partnerLedgerHandlerV2714(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", 405)
		return
	}
	u := a.currentUser(r)
	if u == nil {
		writeError(w, errors.New("sesión requerida"), 401)
		return
	}
	a.releasePartnerCommissionsV2714(u.ID)
	rows, err := a.db.Query(`SELECT l.id,l.customer_user_id,COALESCE(cu.name,''),l.payment_id,l.commission_type,l.level,l.eligible_base,l.percent,l.amount,l.currency,l.status,l.available_at,l.created_at FROM partner_commission_ledger_v2714 l LEFT JOIN app_users cu ON cu.id=l.customer_user_id WHERE l.partner_user_id=? ORDER BY l.id DESC LIMIT 250`, u.ID)
	if err != nil {
		writeError(w, err, 500)
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, customerID, paymentID int64
		var customerName, typ, currency, status, availableAt, createdAt string
		var level int
		var base, pct, amount float64
		if rows.Scan(&id, &customerID, &customerName, &paymentID, &typ, &level, &base, &pct, &amount, &currency, &status, &availableAt, &createdAt) == nil {
			out = append(out, map[string]any{"id": id, "customer_user_id": customerID, "customer_name": customerName, "payment_id": paymentID, "commission_type": typ, "level": level, "eligible_base": base, "percent": pct, "amount": amount, "currency": currency, "status": status, "available_at": availableAt, "created_at": createdAt})
		}
	}
	writeJSON(w, out)
}

func (a *App) addPartnerCommissionV2714(partnerUserID, customerUserID, paymentID int64, typ string, level int, base, percent float64, metadata map[string]any) error {
	if partnerUserID == 0 || paymentID == 0 || base <= 0 || percent <= 0 {
		return nil
	}
	var active int
	if a.db.QueryRow(`SELECT COUNT(*) FROM partner_profiles_v2714 WHERE user_id=? AND status='active'`, partnerUserID).Scan(&active) != nil || active == 0 {
		return nil
	}
	amount := roundUSDT(base * percent / 100)
	if amount <= 0 {
		return nil
	}
	now := time.Now().UTC()
	holdDays := a.partnerSettingIntV2714("commission_hold_days", 14)
	if holdDays < 0 {
		holdDays = 0
	}
	installment := 1
	releaseAt := now.AddDate(0, 0, holdDays)
	if metadata != nil {
		if rawInstallment, ok := metadata["installment"]; ok {
			switch v := rawInstallment.(type) {
			case int:
				if v > 0 {
					installment = v
				}
			case float64:
				if v > 0 {
					installment = int(v)
				}
			}
		}
		if rawRelease, ok := metadata["release_at"].(string); ok && strings.TrimSpace(rawRelease) != "" {
			if t, e := time.Parse(time.RFC3339, rawRelease); e == nil {
				releaseAt = t
			}
		}
	}
	availableAt := releaseAt.Format(time.RFC3339)
	dedupe := fmt.Sprintf("payment:%d:%s:%d:%d", paymentID, typ, level, installment)
	raw, _ := json.Marshal(metadata)
	_, err := a.db.Exec(`INSERT OR IGNORE INTO partner_commission_ledger_v2714(partner_user_id,customer_user_id,payment_id,commission_type,level,eligible_base,percent,amount,currency,status,available_at,dedupe_key,metadata_json,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,'pending',?,?,?,?,?)`, partnerUserID, customerUserID, paymentID, typ, level, roundUSDT(base), percent, amount, "USD", availableAt, dedupe, string(raw), now.Format(time.RFC3339), now.Format(time.RFC3339))
	return err
}

func (a *App) processPartnerPaymentV2714(paymentID int64) error {
	if paymentID == 0 {
		return nil
	}
	var customerUserID int64
	var amount, originalAmount, monthlyPrice float64
	var planCode, status, createdAt string
	if err := a.db.QueryRow(`SELECT bp.user_id,bp.amount_usdt,COALESCE(bp.original_amount_usdt,0),bp.plan_code,bp.status,bp.created_at,COALESCE(p.price_usdt,0) FROM billing_payments bp LEFT JOIN billing_plans p ON p.code=bp.plan_code WHERE bp.id=?`, paymentID).Scan(&customerUserID, &amount, &originalAmount, &planCode, &status, &createdAt, &monthlyPrice); err != nil {
		return err
	}
	if status != "approved" || amount <= 0 || planCode == "free" {
		return nil
	}
	var directPartnerID int64
	if err := a.db.QueryRow(`SELECT partner_user_id FROM partner_attributions_v2714 WHERE customer_user_id=?`, customerUserID).Scan(&directPartnerID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	directRank := a.evaluatePartnerRankV2714(directPartnerID)
	annual := monthlyPrice > 0 && (originalAmount > monthlyPrice*2 || amount > monthlyPrice*2)
	periods := 1
	if annual {
		periods = 12
	}
	periodBase := roundUSDT(amount / float64(periods))
	var priorApproved int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM billing_payments WHERE user_id=? AND plan_code=? AND status='approved' AND id<>? AND id<?`, customerUserID, planCode, paymentID, paymentID).Scan(&priorApproved)
	var specialistID int64
	var impactScore int
	var adoptionStatus string
	adoptionEligible := a.db.QueryRow(`SELECT ad.specialist_partner_user_id,ad.impact_score,ad.status FROM partner_adoption_v2714 ad JOIN partner_profiles_v2714 p ON p.user_id=ad.specialist_partner_user_id WHERE ad.customer_user_id=? AND p.status='active' AND p.activate_status='certified'`, customerUserID).Scan(&specialistID, &impactScore, &adoptionStatus) == nil && adoptionStatus == "approved" && impactScore >= a.partnerSettingIntV2714("adoption_score_min", 70) && specialistID > 0
	var level2ID, level3ID int64
	_ = a.db.QueryRow(`SELECT sponsor_partner_user_id FROM partner_profiles_v2714 WHERE user_id=? AND status='active'`, directPartnerID).Scan(&level2ID)
	if level2ID > 0 {
		_ = a.db.QueryRow(`SELECT sponsor_partner_user_id FROM partner_profiles_v2714 WHERE user_id=? AND status='active'`, level2ID).Scan(&level3ID)
	}
	holdDays := a.partnerSettingIntV2714("commission_hold_days", 14)
	if holdDays < 0 {
		holdDays = 0
	}
	approvedAt := time.Now().UTC()
	for i := 0; i < periods; i++ {
		releaseAt := approvedAt.AddDate(0, i, 0).AddDate(0, 0, holdDays)
		common := map[string]any{"plan_code": planCode, "payment_created_at": createdAt, "billing_cycle": map[bool]string{true: "annual", false: "monthly"}[annual], "installment": i + 1, "installments": periods, "release_at": releaseAt.Format(time.RFC3339)}
		directMeta := clonePartnerMetaV2714(common)
		directMeta["rank_code"] = directRank.Code
		if err := a.addPartnerCommissionV2714(directPartnerID, customerUserID, paymentID, "direct", 1, periodBase, directRank.DirectPercent, directMeta); err != nil {
			return err
		}
		if priorApproved > 0 {
			_ = a.addPartnerCommissionV2714(directPartnerID, customerUserID, paymentID, "renewal", 1, periodBase, a.partnerSettingFloatV2714("renewal_percent", 2), common)
		}
		if adoptionEligible {
			adoptMeta := clonePartnerMetaV2714(common)
			adoptMeta["impact_score"] = impactScore
			_ = a.addPartnerCommissionV2714(specialistID, customerUserID, paymentID, "adoption", 0, periodBase, a.partnerSettingFloatV2714("adoption_percent", 4), adoptMeta)
		}
		if level2ID > 0 {
			level2Rank := a.evaluatePartnerRankV2714(level2ID)
			l2Meta := clonePartnerMetaV2714(common)
			l2Meta["origin_partner_user_id"] = directPartnerID
			l2Meta["rank_code"] = level2Rank.Code
			_ = a.addPartnerCommissionV2714(level2ID, customerUserID, paymentID, "network", 2, periodBase, level2Rank.Level2Percent, l2Meta)
		}
		if level3ID > 0 {
			level3Rank := a.evaluatePartnerRankV2714(level3ID)
			l3Meta := clonePartnerMetaV2714(common)
			l3Meta["origin_partner_user_id"] = directPartnerID
			l3Meta["level2_partner_user_id"] = level2ID
			l3Meta["rank_code"] = level3Rank.Code
			_ = a.addPartnerCommissionV2714(level3ID, customerUserID, paymentID, "network", 3, periodBase, level3Rank.Level3Percent, l3Meta)
		}
	}
	a.partnerAuditV2714(0, directPartnerID, "payment.commissions_processed", map[string]any{"payment_id": paymentID, "customer_user_id": customerUserID, "amount": amount, "annual": annual, "installments": periods})
	return nil
}

func clonePartnerMetaV2714(src map[string]any) map[string]any {
	out := make(map[string]any, len(src)+2)
	for k, v := range src {
		out[k] = v
	}
	return out
}

func (a *App) adminPartnersHandlerV2714(w http.ResponseWriter, r *http.Request) {
	admin, ok := a.requireSuperadmin(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		a.releasePartnerCommissionsV2714(0)
		rows, err := a.db.Query(`SELECT p.user_id,u.name,u.email,p.partner_code,p.status,p.rank_code,p.activate_status,p.sponsor_partner_user_id,p.created_at FROM partner_profiles_v2714 p JOIN app_users u ON u.id=p.user_id ORDER BY p.id DESC LIMIT 500`)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		defer rows.Close()
		items := []map[string]any{}
		for rows.Next() {
			var uid, sponsor int64
			var name, email, code, status, rank, activate, created string
			if rows.Scan(&uid, &name, &email, &code, &status, &rank, &activate, &sponsor, &created) != nil {
				continue
			}
			direct, adopted, l2, l3 := a.partnerMetricsV2714(uid)
			var pending, available, paid float64
			_ = a.db.QueryRow(`SELECT COALESCE(SUM(CASE WHEN status='pending' THEN amount ELSE 0 END),0),COALESCE(SUM(CASE WHEN status='available' THEN amount ELSE 0 END),0),COALESCE(SUM(CASE WHEN status='paid' THEN amount ELSE 0 END),0) FROM partner_commission_ledger_v2714 WHERE partner_user_id=?`, uid).Scan(&pending, &available, &paid)
			items = append(items, map[string]any{"user_id": uid, "name": name, "email": email, "partner_code": code, "status": status, "rank_code": rank, "activate_status": activate, "sponsor_partner_user_id": sponsor, "created_at": created, "direct_active": direct, "adopted_active": adopted, "level2_partners": l2, "level3_partners": l3, "pending": roundUSDT(pending), "available": roundUSDT(available), "paid": roundUSDT(paid)})
		}
		var total, active, certified int
		var pending, available, paid float64
		_ = a.db.QueryRow(`SELECT COUNT(*),COALESCE(SUM(CASE WHEN status='active' THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN activate_status='certified' THEN 1 ELSE 0 END),0) FROM partner_profiles_v2714`).Scan(&total, &active, &certified)
		_ = a.db.QueryRow(`SELECT COALESCE(SUM(CASE WHEN status='pending' THEN amount ELSE 0 END),0),COALESCE(SUM(CASE WHEN status='available' THEN amount ELSE 0 END),0),COALESCE(SUM(CASE WHEN status='paid' THEN amount ELSE 0 END),0) FROM partner_commission_ledger_v2714`).Scan(&pending, &available, &paid)
		writeJSON(w, map[string]any{"items": items, "summary": map[string]any{"partners": total, "active": active, "certified": certified, "pending": roundUSDT(pending), "available": roundUSDT(available), "paid": roundUSDT(paid)}})
	case http.MethodPut:
		var q struct {
			UserID         int64  `json:"user_id"`
			Action         string `json:"action"`
			Status         string `json:"status"`
			ActivateStatus string `json:"activate_status"`
			RankCode       string `json:"rank_code"`
		}
		if json.NewDecoder(r.Body).Decode(&q) != nil || q.UserID == 0 {
			writeError(w, errors.New("partner inválido"), 400)
			return
		}
		now := time.Now().UTC().Format(time.RFC3339)
		switch q.Action {
		case "status":
			if q.Status != "active" && q.Status != "suspended" {
				writeError(w, errors.New("estado inválido"), 400)
				return
			}
			_, _ = a.db.Exec(`UPDATE partner_profiles_v2714 SET status=?,updated_at=? WHERE user_id=?`, q.Status, now, q.UserID)
		case "certification":
			allowed := map[string]bool{"not_certified": true, "in_training": true, "exam_pending": true, "certified": true, "suspended": true}
			if !allowed[q.ActivateStatus] {
				writeError(w, errors.New("estado de certificación inválido"), 400)
				return
			}
			_, _ = a.db.Exec(`UPDATE partner_profiles_v2714 SET activate_status=?,updated_at=? WHERE user_id=?`, q.ActivateStatus, now, q.UserID)
		case "reevaluate_rank":
			a.evaluatePartnerRankV2714(q.UserID)
		default:
			writeError(w, errors.New("acción inválida"), 400)
			return
		}
		a.partnerAuditV2714(admin.ID, q.UserID, "admin.partner."+q.Action, q)
		writeJSON(w, map[string]any{"ok": true})
	default:
		http.Error(w, "Método no permitido", 405)
	}
}

func (a *App) adminPartnerAdoptionHandlerV2714(w http.ResponseWriter, r *http.Request) {
	admin, ok := a.requireSuperadmin(w, r)
	if !ok {
		return
	}
	if r.Method == http.MethodGet {
		rows, err := a.db.Query(`SELECT pa.customer_user_id,cu.name,cu.email,pa.partner_user_id,COALESCE(op.name,''),COALESCE(ad.specialist_partner_user_id,0),COALESCE(sp.name,''),COALESCE(ad.impact_score,0),COALESCE(ad.status,'pending'),COALESCE(ad.notes,''),COALESCE(ad.updated_at,pa.created_at) FROM partner_attributions_v2714 pa JOIN app_users cu ON cu.id=pa.customer_user_id LEFT JOIN app_users op ON op.id=pa.partner_user_id LEFT JOIN partner_adoption_v2714 ad ON ad.customer_user_id=pa.customer_user_id LEFT JOIN app_users sp ON sp.id=ad.specialist_partner_user_id ORDER BY pa.id DESC LIMIT 500`)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var customerID, originID, specialistID int64
			var customerName, customerEmail, originName, specialistName, status, notes, updatedAt string
			var score int
			if rows.Scan(&customerID, &customerName, &customerEmail, &originID, &originName, &specialistID, &specialistName, &score, &status, &notes, &updatedAt) == nil {
				out = append(out, map[string]any{"customer_user_id": customerID, "customer_name": customerName, "customer_email": customerEmail, "origin_partner_user_id": originID, "origin_partner_name": originName, "specialist_partner_user_id": specialistID, "specialist_name": specialistName, "impact_score": score, "status": status, "notes": notes, "updated_at": updatedAt})
			}
		}
		writeJSON(w, out)
		return
	}
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		http.Error(w, "Método no permitido", 405)
		return
	}
	var q struct {
		CustomerUserID          int64  `json:"customer_user_id"`
		SpecialistPartnerUserID int64  `json:"specialist_partner_user_id"`
		ImpactScore             int    `json:"impact_score"`
		Status                  string `json:"status"`
		Notes                   string `json:"notes"`
	}
	if json.NewDecoder(r.Body).Decode(&q) != nil || q.CustomerUserID == 0 {
		writeError(w, errors.New("cliente obligatorio"), 400)
		return
	}
	if q.ImpactScore < 0 || q.ImpactScore > 100 {
		writeError(w, errors.New("Impact Score debe estar entre 0 y 100"), 400)
		return
	}
	if q.Status == "" {
		q.Status = "pending"
	}
	if q.Status != "pending" && q.Status != "approved" && q.Status != "rejected" {
		writeError(w, errors.New("estado inválido"), 400)
		return
	}
	if q.SpecialistPartnerUserID > 0 {
		var active int
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM partner_profiles_v2714 WHERE user_id=? AND status='active' AND activate_status='certified'`, q.SpecialistPartnerUserID).Scan(&active)
		if active == 0 {
			writeError(w, errors.New("el especialista debe ser un Partner activo con Activate Certified"), 400)
			return
		}
	}
	now := time.Now().UTC().Format(time.RFC3339)
	approvedAt := ""
	approvedBy := int64(0)
	if q.Status == "approved" {
		approvedAt = now
		approvedBy = admin.ID
	}
	var count int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM partner_adoption_v2714 WHERE customer_user_id=?`, q.CustomerUserID).Scan(&count)
	if count == 0 {
		_, _ = a.db.Exec(`INSERT INTO partner_adoption_v2714(customer_user_id,specialist_partner_user_id,impact_score,status,notes,approved_by,approved_at,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?)`, q.CustomerUserID, q.SpecialistPartnerUserID, q.ImpactScore, q.Status, strings.TrimSpace(q.Notes), approvedBy, approvedAt, now, now)
	} else {
		_, _ = a.db.Exec(`UPDATE partner_adoption_v2714 SET specialist_partner_user_id=?,impact_score=?,status=?,notes=?,approved_by=?,approved_at=?,updated_at=? WHERE customer_user_id=?`, q.SpecialistPartnerUserID, q.ImpactScore, q.Status, strings.TrimSpace(q.Notes), approvedBy, approvedAt, now, q.CustomerUserID)
	}
	var originPartnerID int64
	_ = a.db.QueryRow(`SELECT partner_user_id FROM partner_attributions_v2714 WHERE customer_user_id=?`, q.CustomerUserID).Scan(&originPartnerID)
	if originPartnerID > 0 {
		a.evaluatePartnerRankV2714(originPartnerID)
	}
	a.partnerAuditV2714(admin.ID, q.SpecialistPartnerUserID, "admin.adoption.updated", q)
	writeJSON(w, map[string]any{"ok": true})
}

func (a *App) adminPartnerLedgerHandlerV2714(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireSuperadmin(w, r); !ok {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", 405)
		return
	}
	a.releasePartnerCommissionsV2714(0)
	rows, err := a.db.Query(`SELECT l.id,l.partner_user_id,pu.name,pu.email,l.customer_user_id,COALESCE(cu.name,''),l.payment_id,l.commission_type,l.level,l.eligible_base,l.percent,l.amount,l.currency,l.status,l.available_at,l.created_at FROM partner_commission_ledger_v2714 l JOIN app_users pu ON pu.id=l.partner_user_id LEFT JOIN app_users cu ON cu.id=l.customer_user_id ORDER BY l.id DESC LIMIT 1000`)
	if err != nil {
		writeError(w, err, 500)
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, partnerID, customerID, paymentID int64
		var partnerName, partnerEmail, customerName, typ, currency, status, availableAt, createdAt string
		var level int
		var base, pct, amount float64
		if rows.Scan(&id, &partnerID, &partnerName, &partnerEmail, &customerID, &customerName, &paymentID, &typ, &level, &base, &pct, &amount, &currency, &status, &availableAt, &createdAt) == nil {
			out = append(out, map[string]any{"id": id, "partner_user_id": partnerID, "partner_name": partnerName, "partner_email": partnerEmail, "customer_user_id": customerID, "customer_name": customerName, "payment_id": paymentID, "commission_type": typ, "level": level, "eligible_base": base, "percent": pct, "amount": amount, "currency": currency, "status": status, "available_at": availableAt, "created_at": createdAt})
		}
	}
	writeJSON(w, out)
}

func (a *App) reconcilePartnerApprovedPaymentsV2714() error {
	rows, err := a.db.Query(`SELECT id FROM billing_payments WHERE status='approved' ORDER BY id ASC`)
	if err != nil {
		return err
	}
	defer rows.Close()
	ids := []int64{}
	for rows.Next() {
		var id int64
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	for _, id := range ids {
		if err := a.processPartnerPaymentV2714(id); err != nil {
			return fmt.Errorf("payment %d: %w", id, err)
		}
	}
	return nil
}
