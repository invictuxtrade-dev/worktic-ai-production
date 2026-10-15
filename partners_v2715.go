package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const partnerAcademyWebhookPathV2715 = "/webhooks/academy/partner-certification"

func initPartnersV2715Schema(db *DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS partner_cosell_deals_v2715 (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 referrer_partner_user_id INTEGER NOT NULL,
 closer_partner_user_id INTEGER NOT NULL,
 prospect_name TEXT NOT NULL DEFAULT '',
 prospect_email TEXT NOT NULL DEFAULT '',
 prospect_phone TEXT NOT NULL DEFAULT '',
 crm_contact_id INTEGER NOT NULL DEFAULT 0,
 crm_opportunity_id INTEGER NOT NULL DEFAULT 0,
 customer_user_id INTEGER NOT NULL DEFAULT 0,
 payment_id INTEGER NOT NULL DEFAULT 0,
 referrer_split REAL NOT NULL DEFAULT 70,
 closer_split REAL NOT NULL DEFAULT 30,
 referrer_accepted INTEGER NOT NULL DEFAULT 0,
 closer_accepted INTEGER NOT NULL DEFAULT 0,
 status TEXT NOT NULL DEFAULT 'pending',
 note TEXT NOT NULL DEFAULT '',
 created_by INTEGER NOT NULL DEFAULT 0,
 created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL,
 won_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_partner_cosell_v2715_parties ON partner_cosell_deals_v2715(referrer_partner_user_id,closer_partner_user_id,status,created_at);
CREATE INDEX IF NOT EXISTS idx_partner_cosell_v2715_prospect ON partner_cosell_deals_v2715(prospect_email,status);

CREATE TABLE IF NOT EXISTS partner_certifications_v2715 (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 partner_user_id INTEGER NOT NULL,
 academy_email TEXT NOT NULL DEFAULT '',
 exam_code TEXT NOT NULL DEFAULT 'activate-certified',
 score REAL NOT NULL DEFAULT 0,
 passing_score REAL NOT NULL DEFAULT 0,
 certificate_id TEXT NOT NULL DEFAULT '',
 certificate_url TEXT NOT NULL DEFAULT '',
 status TEXT NOT NULL DEFAULT 'passed',
 source TEXT NOT NULL DEFAULT 'academy_webhook',
 issued_at TEXT NOT NULL DEFAULT '',
 created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL,
 UNIQUE(partner_user_id,exam_code)
);
CREATE INDEX IF NOT EXISTS idx_partner_certifications_v2715_status ON partner_certifications_v2715(status,issued_at);

CREATE TABLE IF NOT EXISTS partner_academy_events_v2715 (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 academy_email TEXT NOT NULL DEFAULT '',
 exam_code TEXT NOT NULL DEFAULT '',
 event_status TEXT NOT NULL DEFAULT '',
 score REAL NOT NULL DEFAULT 0,
 certificate_id TEXT NOT NULL DEFAULT '',
 matched_user_id INTEGER NOT NULL DEFAULT 0,
 raw_json TEXT NOT NULL DEFAULT '{}',
 processed INTEGER NOT NULL DEFAULT 0,
 created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_partner_academy_events_v2715_email ON partner_academy_events_v2715(academy_email,created_at DESC);

CREATE TABLE IF NOT EXISTS partner_marketplace_jobs_v2715 (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 tenant_id INTEGER NOT NULL,
 business_user_id INTEGER NOT NULL,
 business_name TEXT NOT NULL DEFAULT '',
 title TEXT NOT NULL,
 product_service TEXT NOT NULL DEFAULT '',
 description TEXT NOT NULL DEFAULT '',
 region TEXT NOT NULL DEFAULT '',
 modality TEXT NOT NULL DEFAULT 'remote',
 commission_percent REAL NOT NULL DEFAULT 0,
 currency TEXT NOT NULL DEFAULT 'USD',
 leads_source TEXT NOT NULL DEFAULT 'business',
 requirements TEXT NOT NULL DEFAULT '',
 slots INTEGER NOT NULL DEFAULT 1,
 status TEXT NOT NULL DEFAULT 'pending_review',
 created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_partner_marketplace_jobs_v2715_status ON partner_marketplace_jobs_v2715(status,created_at DESC);
CREATE INDEX IF NOT EXISTS idx_partner_marketplace_jobs_v2715_tenant ON partner_marketplace_jobs_v2715(tenant_id,status,created_at DESC);

CREATE TABLE IF NOT EXISTS partner_marketplace_applications_v2715 (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 job_id INTEGER NOT NULL,
 specialist_partner_user_id INTEGER NOT NULL,
 message TEXT NOT NULL DEFAULT '',
 status TEXT NOT NULL DEFAULT 'applied',
 created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL,
 accepted_at TEXT NOT NULL DEFAULT '',
 UNIQUE(job_id,specialist_partner_user_id)
);
CREATE INDEX IF NOT EXISTS idx_partner_marketplace_apps_v2715_job ON partner_marketplace_applications_v2715(job_id,status,created_at DESC);
CREATE INDEX IF NOT EXISTS idx_partner_marketplace_apps_v2715_partner ON partner_marketplace_applications_v2715(specialist_partner_user_id,status,created_at DESC);

CREATE TABLE IF NOT EXISTS partner_marketplace_sales_v2715 (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 application_id INTEGER NOT NULL,
 job_id INTEGER NOT NULL,
 tenant_id INTEGER NOT NULL,
 business_user_id INTEGER NOT NULL,
 specialist_partner_user_id INTEGER NOT NULL,
 crm_opportunity_id INTEGER NOT NULL DEFAULT 0,
 customer_name TEXT NOT NULL DEFAULT '',
 external_reference TEXT NOT NULL DEFAULT '',
 gross_sale_amount REAL NOT NULL DEFAULT 0,
 commission_percent REAL NOT NULL DEFAULT 0,
 commission_pool REAL NOT NULL DEFAULT 0,
 specialist_percent REAL NOT NULL DEFAULT 80,
 platform_percent REAL NOT NULL DEFAULT 20,
 specialist_amount REAL NOT NULL DEFAULT 0,
 platform_amount REAL NOT NULL DEFAULT 0,
 currency TEXT NOT NULL DEFAULT 'USD',
 status TEXT NOT NULL DEFAULT 'reported',
 business_note TEXT NOT NULL DEFAULT '',
 admin_note TEXT NOT NULL DEFAULT '',
 funding_reference TEXT NOT NULL DEFAULT '',
 ledger_id INTEGER NOT NULL DEFAULT 0,
 reported_at TEXT NOT NULL,
 business_confirmed_at TEXT NOT NULL DEFAULT '',
 approved_at TEXT NOT NULL DEFAULT '',
 reversed_at TEXT NOT NULL DEFAULT '',
 updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_partner_marketplace_sales_v2715_specialist ON partner_marketplace_sales_v2715(specialist_partner_user_id,status,reported_at DESC);
CREATE INDEX IF NOT EXISTS idx_partner_marketplace_sales_v2715_business ON partner_marketplace_sales_v2715(tenant_id,status,reported_at DESC);

CREATE TABLE IF NOT EXISTS partner_payout_profiles_v2715 (
 partner_user_id INTEGER PRIMARY KEY,
 method TEXT NOT NULL DEFAULT 'USDT_TRC20',
 destination TEXT NOT NULL DEFAULT '',
 holder_name TEXT NOT NULL DEFAULT '',
 country TEXT NOT NULL DEFAULT '',
 updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS partner_autopilot_plans_v2715 (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 partner_user_id INTEGER NOT NULL,
 tenant_id INTEGER NOT NULL DEFAULT 0,
 niche TEXT NOT NULL DEFAULT '',
 goal TEXT NOT NULL DEFAULT '',
 target_clients INTEGER NOT NULL DEFAULT 3,
 offer TEXT NOT NULL DEFAULT '',
 primary_channel TEXT NOT NULL DEFAULT 'mixed',
 plan_json TEXT NOT NULL DEFAULT '{}',
 raw_text TEXT NOT NULL DEFAULT '',
 status TEXT NOT NULL DEFAULT 'draft',
 campaign_id INTEGER NOT NULL DEFAULT 0,
 landing_id INTEGER NOT NULL DEFAULT 0,
 created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_partner_autopilot_v2715_partner ON partner_autopilot_plans_v2715(partner_user_id,created_at DESC);

CREATE TABLE IF NOT EXISTS partner_payment_reversals_v2715 (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 payment_id INTEGER NOT NULL,
 ledger_id INTEGER NOT NULL,
 partner_user_id INTEGER NOT NULL,
 amount REAL NOT NULL DEFAULT 0,
 reason TEXT NOT NULL,
 action TEXT NOT NULL DEFAULT 'reversed_before_payout',
 adjustment_ledger_id INTEGER NOT NULL DEFAULT 0,
 actor_user_id INTEGER NOT NULL DEFAULT 0,
 created_at TEXT NOT NULL,
 UNIQUE(ledger_id,reason)
);
CREATE INDEX IF NOT EXISTS idx_partner_reversals_v2715_payment ON partner_payment_reversals_v2715(payment_id,created_at DESC);
`)
	if err != nil {
		return err
	}
	// Extensiones aditivas sobre tablas V27.14 ya preparadas.
	for _, stmt := range []string{
		`ALTER TABLE partner_payouts_v2714 ADD COLUMN reference TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE partner_payouts_v2714 ADD COLUMN reviewed_by INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE partner_payouts_v2714 ADD COLUMN paid_by INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE partner_payouts_v2714 ADD COLUMN updated_at TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE partner_payouts_v2714 ADD COLUMN cancelled_at TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE billing_payments ADD COLUMN subscription_id INTEGER NOT NULL DEFAULT 0`,
	} {
		_, _ = db.Exec(stmt)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	defaults := map[string]string{
		"rank_protection_days":                  "90",
		"mentorship_percent":                    "2",
		"mentorship_min_rank":                   "apex",
		"human_sales_specialist_percent":        "80",
		"human_sales_platform_percent":          "20",
		"human_sales_hold_days":                 "7",
		"payout_min_amount":                     "20",
		"cosell_referrer_split":                 "70",
		"academy_activate_exam_code":            "activate-certified",
		"partner_license_required":              "1",
		"partner_autopilot_create_growth_draft": "1",
	}
	for k, v := range defaults {
		_, _ = db.Exec(`INSERT OR IGNORE INTO partner_settings_v2714(key,value,updated_at) VALUES(?,?,?)`, k, v, now)
	}
	return nil
}

func (a *App) partnerSettingTextV2715(key, fallback string) string {
	var v string
	if a.db.QueryRow(`SELECT value FROM partner_settings_v2714 WHERE key=?`, key).Scan(&v) == nil && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return fallback
}

func (a *App) partnerPaidEligibilityV2715(userID int64) (bool, string, string) {
	if userID == 0 {
		return false, "", ""
	}
	var role string
	_ = a.db.QueryRow(`SELECT role FROM app_users WHERE id=?`, userID).Scan(&role)
	if role == "superadmin" {
		return true, "enterprise", ""
	}
	if a.partnerSettingIntV2714("partner_license_required", 1) == 0 {
		return true, "", ""
	}
	var plan, ends string
	err := a.db.QueryRow(`SELECT plan_code,ends_at FROM billing_subscriptions WHERE user_id=? AND status='active' AND plan_code<>'free' AND ends_at>? ORDER BY ends_at DESC LIMIT 1`, userID, time.Now().UTC().Format(time.RFC3339)).Scan(&plan, &ends)
	return err == nil, plan, ends
}

func (a *App) partnerEarningEligibleV2715(partnerUserID int64) bool {
	var status string
	if a.db.QueryRow(`SELECT status FROM partner_profiles_v2714 WHERE user_id=?`, partnerUserID).Scan(&status) != nil || status != "active" {
		return false
	}
	ok, _, _ := a.partnerPaidEligibilityV2715(partnerUserID)
	return ok
}

func (a *App) partnerUserByEmailV2715(email string) (*User, error) {
	var u User
	var active int
	err := a.db.QueryRow(`SELECT u.id,u.name,u.email,u.role,u.company,u.active,u.created_at,u.tenant_id,COALESCE(t.account_type,'personal') FROM app_users u LEFT JOIN tenants t ON t.id=u.tenant_id WHERE lower(u.email)=lower(?)`, strings.TrimSpace(email)).Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.Company, &active, &u.CreatedAt, &u.TenantID, &u.AccountType)
	if err != nil {
		return nil, err
	}
	u.Active = active == 1
	return &u, nil
}

func (a *App) partnerRankOrderV2715(code string) int {
	var n int
	_ = a.db.QueryRow(`SELECT sort_order FROM partner_rank_rules_v2714 WHERE code=? AND active=1`, code).Scan(&n)
	return n
}

func (a *App) partnerBalancesV2715(partnerUserID int64) map[string]float64 {
	a.releasePartnerCommissionsV2714(partnerUserID)
	out := map[string]float64{"pending": 0, "available_gross": 0, "reserved": 0, "available": 0, "paid": 0, "reversed": 0, "earned": 0}
	rows, err := a.db.Query(`SELECT status,COALESCE(SUM(amount),0) FROM partner_commission_ledger_v2714 WHERE partner_user_id=? GROUP BY status`, partnerUserID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var s string
			var v float64
			if rows.Scan(&s, &v) == nil {
				switch s {
				case "pending":
					out["pending"] = roundUSDT(v)
				case "available":
					out["available_gross"] = roundUSDT(v)
				case "reversed":
					out["reversed"] = roundUSDT(v)
				}
			}
		}
	}
	var reserved, paid, earned float64
	_ = a.db.QueryRow(`SELECT COALESCE(SUM(amount),0) FROM partner_payouts_v2714 WHERE partner_user_id=? AND status IN ('requested','approved','processing','needs_review')`, partnerUserID).Scan(&reserved)
	_ = a.db.QueryRow(`SELECT COALESCE(SUM(amount),0) FROM partner_payouts_v2714 WHERE partner_user_id=? AND status='paid'`, partnerUserID).Scan(&paid)
	_ = a.db.QueryRow(`SELECT COALESCE(SUM(CASE WHEN amount>0 AND commission_type<>'payout_debit' THEN amount ELSE 0 END),0) FROM partner_commission_ledger_v2714 WHERE partner_user_id=? AND status<>'reversed'`, partnerUserID).Scan(&earned)
	out["reserved"] = reserved
	out["paid"] = paid
	out["earned"] = earned
	out["available"] = roundUSDT(out["available_gross"] - out["reserved"])
	for k, v := range out {
		out[k] = roundUSDT(v)
	}
	return out
}

func (a *App) syncPartnerCertificationStatusV2715(userID int64) {
	if userID == 0 {
		return
	}
	var certStatus string
	err := a.db.QueryRow(`SELECT status FROM partner_certifications_v2715 WHERE partner_user_id=? ORDER BY id DESC LIMIT 1`, userID).Scan(&certStatus)
	if err != nil {
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	switch strings.ToLower(strings.TrimSpace(certStatus)) {
	case "passed":
		_, _ = a.db.Exec(`UPDATE partner_profiles_v2714 SET activate_status='certified',updated_at=? WHERE user_id=? AND status='active'`, now, userID)
	case "revoked":
		_, _ = a.db.Exec(`UPDATE partner_profiles_v2714 SET activate_status='suspended',updated_at=? WHERE user_id=?`, now, userID)
	}
}

func (a *App) partnerMeHandlerV2715(w http.ResponseWriter, r *http.Request) {
	u := a.currentUser(r)
	if u == nil {
		writeError(w, errors.New("sesión requerida"), 401)
		return
	}
	eligible, plan, ends := a.partnerPaidEligibilityV2715(u.ID)
	switch r.Method {
	case http.MethodGet:
		var count int
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM partner_profiles_v2714 WHERE user_id=?`, u.ID).Scan(&count)
		if count == 0 {
			writeJSON(w, map[string]any{"active": false, "can_activate": eligible, "eligible": eligible, "plan_code": plan, "plan_ends_at": ends, "reason": map[bool]string{true: "", false: "paid_license_required"}[eligible]})
			return
		}
		p, err := a.ensurePartnerProfileV2714(u)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		a.reconcilePartnerAcademyForUserV2715(u)
		a.syncPartnerCertificationStatusV2715(u.ID)
		p, _ = a.ensurePartnerProfileV2714(u)
		p["active"] = p["status"] == "active"
		p["eligible"] = eligible
		p["plan_code"] = plan
		p["plan_ends_at"] = ends
		writeJSON(w, p)
	case http.MethodPost:
		if !eligible {
			writeError(w, errors.New("para activar Worktic Partners necesitas una licencia WorkticAI de pago vigente"), 403)
			return
		}
		p, err := a.ensurePartnerProfileV2714(u)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		a.reconcilePartnerAcademyForUserV2715(u)
		a.syncPartnerCertificationStatusV2715(u.ID)
		p, _ = a.ensurePartnerProfileV2714(u)
		p["active"] = p["status"] == "active"
		p["eligible"] = true
		p["plan_code"] = plan
		p["plan_ends_at"] = ends
		p["can_activate"] = false
		writeJSON(w, p)
	default:
		http.Error(w, "Método no permitido", 405)
	}
}

func (a *App) partnerOverviewHandlerV2715(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", 405)
		return
	}
	u := a.currentUser(r)
	if u == nil {
		writeError(w, errors.New("sesión requerida"), 401)
		return
	}
	eligible, plan, ends := a.partnerPaidEligibilityV2715(u.ID)
	var exists int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM partner_profiles_v2714 WHERE user_id=?`, u.ID).Scan(&exists)
	if exists == 0 {
		writeJSON(w, map[string]any{"active": false, "can_activate": eligible, "eligible": eligible, "plan_code": plan, "plan_ends_at": ends})
		return
	}
	profile, err := a.ensurePartnerProfileV2714(u)
	if err != nil {
		writeError(w, err, 500)
		return
	}
	a.reconcilePartnerAcademyForUserV2715(u)
	a.syncPartnerCertificationStatusV2715(u.ID)
	profile, _ = a.ensurePartnerProfileV2714(u)
	if fmt.Sprint(profile["status"]) != "active" {
		writeJSON(w, map[string]any{"active": false, "can_activate": false, "suspended": true, "profile": profile, "eligible": eligible})
		return
	}
	direct, adopted, level2, level3 := a.partnerMetricsV2714(u.ID)
	rank := a.partnerRankRuleByCodeV2714(fmt.Sprint(profile["rank_code"]))
	var clicks, signups, cosellOpen, jobsOpen, acceptedJobs int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM partner_clicks_v2714 WHERE partner_user_id=?`, u.ID).Scan(&clicks)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM partner_attributions_v2714 WHERE partner_user_id=?`, u.ID).Scan(&signups)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM partner_cosell_deals_v2715 WHERE (referrer_partner_user_id=? OR closer_partner_user_id=?) AND status IN ('pending','active')`, u.ID, u.ID).Scan(&cosellOpen)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM partner_marketplace_jobs_v2715 WHERE status='open'`).Scan(&jobsOpen)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM partner_marketplace_applications_v2715 WHERE specialist_partner_user_id=? AND status='accepted'`, u.ID).Scan(&acceptedJobs)
	balances := a.partnerBalancesV2715(u.ID)
	code := fmt.Sprint(profile["partner_code"])
	base := strings.TrimRight(a.cfg.BaseURL, "/")
	if base == "" {
		base = "https://workticai.com"
	}
	var cert map[string]any
	cert = map[string]any{"status": fmt.Sprint(profile["activate_status"])}
	var examCode, certificateID, certificateURL, issuedAt string
	var score, passing float64
	if a.db.QueryRow(`SELECT exam_code,score,passing_score,certificate_id,certificate_url,issued_at FROM partner_certifications_v2715 WHERE partner_user_id=? AND status='passed' ORDER BY id DESC LIMIT 1`, u.ID).Scan(&examCode, &score, &passing, &certificateID, &certificateURL, &issuedAt) == nil {
		cert = map[string]any{"status": "certified", "exam_code": examCode, "score": score, "passing_score": passing, "certificate_id": certificateID, "certificate_url": certificateURL, "issued_at": issuedAt}
	}
	writeJSON(w, map[string]any{
		"active": true, "eligible": eligible, "plan_code": plan, "plan_ends_at": ends,
		"profile": profile, "referral_url": base + "/r/" + code, "rank": rank,
		"stats":    map[string]any{"clicks": clicks, "signups": signups, "direct_active": direct, "adopted_active": adopted, "level2_partners": level2, "level3_partners": level3, "cosell_open": cosellOpen, "marketplace_open": jobsOpen, "marketplace_accepted": acceptedJobs},
		"balances": balances, "certification": cert,
	})
}

func (a *App) partnerNetworkTreeHandlerV2715(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", 405)
		return
	}
	u := a.currentUser(r)
	if u == nil {
		writeError(w, errors.New("sesión requerida"), 401)
		return
	}
	type node struct {
		UserID       int64  `json:"user_id"`
		SponsorID    int64  `json:"sponsor_user_id"`
		Name         string `json:"name"`
		Email        string `json:"email"`
		Code         string `json:"partner_code"`
		Rank         string `json:"rank_code"`
		Activate     string `json:"activate_status"`
		Status       string `json:"status"`
		Level        int    `json:"level"`
		DirectActive int    `json:"direct_active"`
		Adopted      int    `json:"adopted_active"`
		CreatedAt    string `json:"created_at"`
	}
	out := []node{}
	current := []int64{u.ID}
	for level := 1; level <= 3 && len(current) > 0; level++ {
		next := []int64{}
		for _, sponsorID := range current {
			rows, err := a.db.Query(`SELECT p.user_id,p.sponsor_partner_user_id,u.name,u.email,p.partner_code,p.rank_code,p.activate_status,p.status,p.created_at FROM partner_profiles_v2714 p JOIN app_users u ON u.id=p.user_id WHERE p.sponsor_partner_user_id=? ORDER BY p.id ASC`, sponsorID)
			if err != nil {
				continue
			}
			for rows.Next() {
				var n node
				if rows.Scan(&n.UserID, &n.SponsorID, &n.Name, &n.Email, &n.Code, &n.Rank, &n.Activate, &n.Status, &n.CreatedAt) == nil {
					n.Level = level
					n.DirectActive, n.Adopted, _, _ = a.partnerMetricsV2714(n.UserID)
					out = append(out, n)
					next = append(next, n.UserID)
				}
			}
			rows.Close()
		}
		current = next
	}
	counts := map[string]int{"direct": 0, "level2": 0, "level3": 0, "total": len(out)}
	for _, n := range out {
		if n.Level == 1 {
			counts["direct"]++
		} else if n.Level == 2 {
			counts["level2"]++
		} else if n.Level == 3 {
			counts["level3"]++
		}
	}
	writeJSON(w, map[string]any{"nodes": out, "counts": counts, "root_user_id": u.ID})
}

func (a *App) partnerCosellHandlerV2715(w http.ResponseWriter, r *http.Request) {
	u := a.currentUser(r)
	if u == nil {
		writeError(w, errors.New("sesión requerida"), 401)
		return
	}
	if !a.partnerEarningEligibleV2715(u.ID) {
		writeError(w, errors.New("Partner Center requiere perfil activo y licencia de pago vigente"), 403)
		return
	}
	switch r.Method {
	case http.MethodGet:
		rows, err := a.db.Query(`SELECT d.id,d.referrer_partner_user_id,ru.name,d.closer_partner_user_id,cu.name,d.prospect_name,d.prospect_email,d.prospect_phone,d.crm_opportunity_id,d.customer_user_id,d.payment_id,d.referrer_split,d.closer_split,d.referrer_accepted,d.closer_accepted,d.status,d.note,d.created_at,d.updated_at,d.won_at FROM partner_cosell_deals_v2715 d JOIN app_users ru ON ru.id=d.referrer_partner_user_id JOIN app_users cu ON cu.id=d.closer_partner_user_id WHERE d.referrer_partner_user_id=? OR d.closer_partner_user_id=? ORDER BY d.id DESC`, u.ID, u.ID)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var id, refID, closerID, oppID, customerID, paymentID int64
			var refName, closerName, prospectName, email, phone, status, note, created, updated, won string
			var refSplit, closerSplit float64
			var refAccepted, closerAccepted int
			if rows.Scan(&id, &refID, &refName, &closerID, &closerName, &prospectName, &email, &phone, &oppID, &customerID, &paymentID, &refSplit, &closerSplit, &refAccepted, &closerAccepted, &status, &note, &created, &updated, &won) == nil {
				out = append(out, map[string]any{"id": id, "referrer_partner_user_id": refID, "referrer_name": refName, "closer_partner_user_id": closerID, "closer_name": closerName, "prospect_name": prospectName, "prospect_email": email, "prospect_phone": phone, "crm_opportunity_id": oppID, "customer_user_id": customerID, "payment_id": paymentID, "referrer_split": refSplit, "closer_split": closerSplit, "referrer_accepted": refAccepted == 1, "closer_accepted": closerAccepted == 1, "status": status, "note": note, "created_at": created, "updated_at": updated, "won_at": won, "my_role": map[bool]string{true: "referrer", false: "closer"}[refID == u.ID]})
			}
		}
		writeJSON(w, out)
	case http.MethodPost:
		var q struct {
			OtherPartnerCode string  `json:"other_partner_code"`
			MyRole           string  `json:"my_role"`
			ProspectName     string  `json:"prospect_name"`
			ProspectEmail    string  `json:"prospect_email"`
			ProspectPhone    string  `json:"prospect_phone"`
			CRMOpportunityID int64   `json:"crm_opportunity_id"`
			ReferrerSplit    float64 `json:"referrer_split"`
			Note             string  `json:"note"`
		}
		if json.NewDecoder(r.Body).Decode(&q) != nil {
			writeError(w, errors.New("datos inválidos"), 400)
			return
		}
		q.MyRole = strings.ToLower(strings.TrimSpace(q.MyRole))
		if q.MyRole != "referrer" && q.MyRole != "closer" {
			writeError(w, errors.New("elige si actúas como referidor o asesor comercial"), 400)
			return
		}
		if strings.TrimSpace(q.ProspectName) == "" && strings.TrimSpace(q.ProspectEmail) == "" {
			writeError(w, errors.New("indica al menos nombre o correo del prospecto"), 400)
			return
		}
		var otherID int64
		if a.db.QueryRow(`SELECT user_id FROM partner_profiles_v2714 WHERE partner_code=? AND status='active'`, normalizePartnerCodeV2714(q.OtherPartnerCode)).Scan(&otherID) != nil || otherID == 0 || otherID == u.ID {
			writeError(w, errors.New("Partner colaborador inválido"), 400)
			return
		}
		if !a.partnerEarningEligibleV2715(otherID) {
			writeError(w, errors.New("el Partner colaborador debe tener licencia de pago vigente"), 400)
			return
		}
		if q.ReferrerSplit == 0 {
			q.ReferrerSplit = a.partnerSettingFloatV2714("cosell_referrer_split", 70)
		}
		if q.ReferrerSplit < 10 || q.ReferrerSplit > 90 {
			writeError(w, errors.New("el reparto del referidor debe estar entre 10% y 90%"), 400)
			return
		}
		refID, closerID := u.ID, otherID
		if q.MyRole == "closer" {
			refID, closerID = otherID, u.ID
		}
		now := time.Now().UTC().Format(time.RFC3339)
		refAccepted, closerAccepted := 0, 0
		if q.MyRole == "referrer" {
			refAccepted = 1
		} else {
			closerAccepted = 1
		}
		res, err := a.db.Exec(`INSERT INTO partner_cosell_deals_v2715(referrer_partner_user_id,closer_partner_user_id,prospect_name,prospect_email,prospect_phone,crm_opportunity_id,referrer_split,closer_split,referrer_accepted,closer_accepted,status,note,created_by,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,'pending',?,?,?,?)`, refID, closerID, strings.TrimSpace(q.ProspectName), strings.ToLower(strings.TrimSpace(q.ProspectEmail)), strings.TrimSpace(q.ProspectPhone), q.CRMOpportunityID, q.ReferrerSplit, 100-q.ReferrerSplit, refAccepted, closerAccepted, strings.TrimSpace(q.Note), u.ID, now, now)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		id, _ := res.LastInsertId()
		a.partnerAuditV2714(u.ID, u.ID, "cosell.created", map[string]any{"deal_id": id, "referrer": refID, "closer": closerID})
		writeJSON(w, map[string]any{"ok": true, "id": id})
	case http.MethodPut:
		var q struct {
			ID     int64  `json:"id"`
			Action string `json:"action"`
		}
		if json.NewDecoder(r.Body).Decode(&q) != nil || q.ID == 0 {
			writeError(w, errors.New("acuerdo inválido"), 400)
			return
		}
		var refID, closerID int64
		var status string
		var refAccepted, closerAccepted int
		if a.db.QueryRow(`SELECT referrer_partner_user_id,closer_partner_user_id,status,referrer_accepted,closer_accepted FROM partner_cosell_deals_v2715 WHERE id=?`, q.ID).Scan(&refID, &closerID, &status, &refAccepted, &closerAccepted) != nil || (u.ID != refID && u.ID != closerID) {
			writeError(w, errors.New("acuerdo no encontrado"), 404)
			return
		}
		if status == "won" || status == "cancelled" || status == "rejected" {
			writeError(w, errors.New("este Co-Sell ya no admite cambios"), 400)
			return
		}
		now := time.Now().UTC().Format(time.RFC3339)
		switch q.Action {
		case "accept":
			if u.ID == refID {
				refAccepted = 1
			} else {
				closerAccepted = 1
			}
			newStatus := "pending"
			if refAccepted == 1 && closerAccepted == 1 {
				newStatus = "active"
			}
			_, _ = a.db.Exec(`UPDATE partner_cosell_deals_v2715 SET referrer_accepted=?,closer_accepted=?,status=?,updated_at=? WHERE id=?`, refAccepted, closerAccepted, newStatus, now, q.ID)
		case "reject":
			_, _ = a.db.Exec(`UPDATE partner_cosell_deals_v2715 SET status='rejected',updated_at=? WHERE id=?`, now, q.ID)
		case "cancel":
			if u.ID != refID {
				writeError(w, errors.New("solo el referidor puede cancelar un acuerdo activo"), 403)
				return
			}
			_, _ = a.db.Exec(`UPDATE partner_cosell_deals_v2715 SET status='cancelled',updated_at=? WHERE id=?`, now, q.ID)
		default:
			writeError(w, errors.New("acción inválida"), 400)
			return
		}
		a.partnerAuditV2714(u.ID, u.ID, "cosell."+q.Action, map[string]any{"deal_id": q.ID})
		writeJSON(w, map[string]any{"ok": true})
	default:
		http.Error(w, "Método no permitido", 405)
	}
}

func (a *App) findCosellForPaymentV2715(customerUserID, directPartnerID int64) (dealID, referrerID, closerID int64, refSplit, closerSplit float64) {
	var email string
	_ = a.db.QueryRow(`SELECT lower(email) FROM app_users WHERE id=?`, customerUserID).Scan(&email)
	if email == "" {
		return
	}
	_ = a.db.QueryRow(`SELECT id,referrer_partner_user_id,closer_partner_user_id,referrer_split,closer_split FROM partner_cosell_deals_v2715 WHERE status='active' AND referrer_accepted=1 AND closer_accepted=1 AND referrer_partner_user_id=? AND (customer_user_id=? OR (customer_user_id=0 AND lower(prospect_email)=?)) ORDER BY id DESC LIMIT 1`, directPartnerID, customerUserID, email).Scan(&dealID, &referrerID, &closerID, &refSplit, &closerSplit)
	return
}

func (a *App) addPartnerCommissionV2715(partnerUserID, customerUserID, paymentID int64, typ string, level int, base, percent float64, dedupe string, availableAt time.Time, metadata map[string]any) (int64, error) {
	if partnerUserID == 0 || base <= 0 || percent <= 0 || strings.TrimSpace(dedupe) == "" {
		return 0, nil
	}
	if !a.partnerEarningEligibleV2715(partnerUserID) {
		return 0, nil
	}
	amount := roundUSDT(base * percent / 100)
	if amount <= 0 {
		return 0, nil
	}
	now := time.Now().UTC().Format(time.RFC3339)
	raw, _ := json.Marshal(metadata)
	res, err := a.db.Exec(`INSERT OR IGNORE INTO partner_commission_ledger_v2714(partner_user_id,customer_user_id,payment_id,commission_type,level,eligible_base,percent,amount,currency,status,available_at,dedupe_key,metadata_json,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,'pending',?,?,?,?,?)`, partnerUserID, customerUserID, paymentID, typ, level, roundUSDT(base), percent, amount, "USD", availableAt.UTC().Format(time.RFC3339), dedupe, string(raw), now, now)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	if id == 0 {
		_ = a.db.QueryRow(`SELECT id FROM partner_commission_ledger_v2714 WHERE dedupe_key=?`, dedupe).Scan(&id)
	}
	return id, nil
}

func (a *App) addPartnerAmountMovementV2715(partnerUserID int64, typ, dedupe string, amount float64, status string, metadata map[string]any) (int64, error) {
	if partnerUserID == 0 || amount == 0 || dedupe == "" {
		return 0, nil
	}
	now := time.Now().UTC().Format(time.RFC3339)
	raw, _ := json.Marshal(metadata)
	res, err := a.db.Exec(`INSERT OR IGNORE INTO partner_commission_ledger_v2714(partner_user_id,customer_user_id,payment_id,commission_type,level,eligible_base,percent,amount,currency,status,available_at,dedupe_key,metadata_json,created_at,updated_at) VALUES(?,0,0,?,0,0,0,?,'USD',?,?,?,?,?,?)`, partnerUserID, typ, roundUSDT(amount), status, now, dedupe, string(raw), now, now)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	if id == 0 {
		_ = a.db.QueryRow(`SELECT id FROM partner_commission_ledger_v2714 WHERE dedupe_key=?`, dedupe).Scan(&id)
	}
	return id, nil
}

func (a *App) processPartnerPaymentV2715(paymentID int64) error {
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
	cosellID, refID, closerID, refSplit, closerSplit := a.findCosellForPaymentV2715(customerUserID, directPartnerID)
	holdDays := a.partnerSettingIntV2714("commission_hold_days", 14)
	if holdDays < 0 {
		holdDays = 0
	}
	approvedAt := time.Now().UTC()
	for i := 0; i < periods; i++ {
		releaseAt := approvedAt.AddDate(0, i, 0).AddDate(0, 0, holdDays)
		common := map[string]any{"plan_code": planCode, "payment_created_at": createdAt, "billing_cycle": map[bool]string{true: "annual", false: "monthly"}[annual], "installment": i + 1, "installments": periods, "release_at": releaseAt.Format(time.RFC3339)}
		if cosellID > 0 && refID == directPartnerID && closerID > 0 {
			metaRef := clonePartnerMetaV2714(common)
			metaRef["cosell_deal_id"] = cosellID
			metaRef["split_percent"] = refSplit
			metaRef["rank_code"] = directRank.Code
			metaCloser := clonePartnerMetaV2714(common)
			metaCloser["cosell_deal_id"] = cosellID
			metaCloser["split_percent"] = closerSplit
			metaCloser["rank_code"] = directRank.Code
			_, _ = a.addPartnerCommissionV2715(refID, customerUserID, paymentID, "cosell_referrer", 1, periodBase, directRank.DirectPercent*refSplit/100, fmt.Sprintf("payment:%d:cosell_referrer:1:%d", paymentID, i+1), releaseAt, metaRef)
			_, _ = a.addPartnerCommissionV2715(closerID, customerUserID, paymentID, "cosell_closer", 1, periodBase, directRank.DirectPercent*closerSplit/100, fmt.Sprintf("payment:%d:cosell_closer:1:%d", paymentID, i+1), releaseAt, metaCloser)
		} else {
			meta := clonePartnerMetaV2714(common)
			meta["rank_code"] = directRank.Code
			_, _ = a.addPartnerCommissionV2715(directPartnerID, customerUserID, paymentID, "direct", 1, periodBase, directRank.DirectPercent, fmt.Sprintf("payment:%d:direct:1:%d", paymentID, i+1), releaseAt, meta)
		}
		if priorApproved > 0 {
			_, _ = a.addPartnerCommissionV2715(directPartnerID, customerUserID, paymentID, "renewal", 1, periodBase, a.partnerSettingFloatV2714("renewal_percent", 2), fmt.Sprintf("payment:%d:renewal:1:%d", paymentID, i+1), releaseAt, common)
		}
		if adoptionEligible {
			meta := clonePartnerMetaV2714(common)
			meta["impact_score"] = impactScore
			_, _ = a.addPartnerCommissionV2715(specialistID, customerUserID, paymentID, "adoption", 0, periodBase, a.partnerSettingFloatV2714("adoption_percent", 4), fmt.Sprintf("payment:%d:adoption:0:%d", paymentID, i+1), releaseAt, meta)
		}
		if level2ID > 0 {
			r := a.evaluatePartnerRankV2714(level2ID)
			meta := clonePartnerMetaV2714(common)
			meta["origin_partner_user_id"] = directPartnerID
			meta["rank_code"] = r.Code
			_, _ = a.addPartnerCommissionV2715(level2ID, customerUserID, paymentID, "network", 2, periodBase, r.Level2Percent, fmt.Sprintf("payment:%d:network:2:%d", paymentID, i+1), releaseAt, meta)
			minRank := a.partnerSettingTextV2715("mentorship_min_rank", "apex")
			if a.partnerRankOrderV2715(r.Code) >= a.partnerRankOrderV2715(minRank) {
				mentorMeta := clonePartnerMetaV2714(common)
				mentorMeta["mentored_partner_user_id"] = directPartnerID
				mentorMeta["mentor_rank_code"] = r.Code
				_, _ = a.addPartnerCommissionV2715(level2ID, customerUserID, paymentID, "mentorship", 2, periodBase, a.partnerSettingFloatV2714("mentorship_percent", 2), fmt.Sprintf("payment:%d:mentorship:2:%d", paymentID, i+1), releaseAt, mentorMeta)
			}
		}
		if level3ID > 0 {
			r := a.evaluatePartnerRankV2714(level3ID)
			meta := clonePartnerMetaV2714(common)
			meta["origin_partner_user_id"] = directPartnerID
			meta["level2_partner_user_id"] = level2ID
			meta["rank_code"] = r.Code
			_, _ = a.addPartnerCommissionV2715(level3ID, customerUserID, paymentID, "network", 3, periodBase, r.Level3Percent, fmt.Sprintf("payment:%d:network:3:%d", paymentID, i+1), releaseAt, meta)
		}
	}
	if cosellID > 0 {
		now := time.Now().UTC().Format(time.RFC3339)
		_, _ = a.db.Exec(`UPDATE partner_cosell_deals_v2715 SET customer_user_id=?,payment_id=?,status='won',won_at=?,updated_at=? WHERE id=? AND status='active'`, customerUserID, paymentID, now, now, cosellID)
	}
	a.partnerAuditV2714(0, directPartnerID, "payment.commissions_processed.v2715", map[string]any{"payment_id": paymentID, "customer_user_id": customerUserID, "amount": amount, "annual": annual, "installments": periods, "cosell_deal_id": cosellID})
	return nil
}

func (a *App) reconcilePartnerApprovedPaymentsV2715() error {
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
		if err := a.processPartnerPaymentV2715(id); err != nil {
			return fmt.Errorf("payment %d: %w", id, err)
		}
	}
	return nil
}

func (a *App) reconcilePartnerPaymentReversalsV2715() error {
	rows, err := a.db.Query(`SELECT id,status FROM billing_payments WHERE status IN ('refunded','chargeback') ORDER BY id ASC`)
	if err != nil {
		return err
	}
	defer rows.Close()
	type item struct {
		id     int64
		status string
	}
	items := []item{}
	for rows.Next() {
		var x item
		if rows.Scan(&x.id, &x.status) == nil {
			items = append(items, x)
		}
	}
	for _, x := range items {
		reason := "refund"
		if x.status == "chargeback" {
			reason = "chargeback"
		}
		if err := a.reversePartnerPaymentV2715(x.id, reason, 0); err != nil {
			return fmt.Errorf("payment %d reversal: %w", x.id, err)
		}
	}
	return nil
}

func (a *App) reversePartnerPaymentV2715(paymentID int64, reason string, actorUserID int64) error {
	reason = strings.ToLower(strings.TrimSpace(reason))
	if reason != "refund" && reason != "chargeback" {
		return errors.New("motivo de reverso inválido")
	}
	rows, err := a.db.Query(`SELECT id,partner_user_id,amount,status,commission_type FROM partner_commission_ledger_v2714 WHERE payment_id=? AND amount>0 ORDER BY id`, paymentID)
	if err != nil {
		return err
	}
	type row struct {
		id, partner int64
		amount      float64
		status, typ string
	}
	items := []row{}
	for rows.Next() {
		var x row
		if rows.Scan(&x.id, &x.partner, &x.amount, &x.status, &x.typ) == nil {
			items = append(items, x)
		}
	}
	rows.Close()
	now := time.Now().UTC().Format(time.RFC3339)
	for _, x := range items {
		var existing int
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM partner_payment_reversals_v2715 WHERE ledger_id=? AND reason=?`, x.id, reason).Scan(&existing)
		if existing > 0 || x.status == "reversed" {
			continue
		}
		action := "reversed_before_payout"
		adjustID := int64(0)
		if x.status == "paid" {
			action = "negative_adjustment"
			meta := map[string]any{"payment_id": paymentID, "original_ledger_id": x.id, "original_type": x.typ, "reason": reason}
			adjustID, _ = a.addPartnerAmountMovementV2715(x.partner, reason+"_reversal", fmt.Sprintf("reversal:%d:%s", x.id, reason), -roundUSDT(x.amount), "available", meta)
		} else {
			_, _ = a.db.Exec(`UPDATE partner_commission_ledger_v2714 SET status='reversed',updated_at=? WHERE id=? AND status<>'reversed'`, now, x.id)
		}
		_, _ = a.db.Exec(`INSERT OR IGNORE INTO partner_payment_reversals_v2715(payment_id,ledger_id,partner_user_id,amount,reason,action,adjustment_ledger_id,actor_user_id,created_at) VALUES(?,?,?,?,?,?,?,?,?)`, paymentID, x.id, x.partner, x.amount, reason, action, adjustID, actorUserID, now)
		balances := a.partnerBalancesV2715(x.partner)
		if balances["available"] < 0 {
			_, _ = a.db.Exec(`UPDATE partner_payouts_v2714 SET status='needs_review',admin_note=CASE WHEN admin_note='' THEN ? ELSE admin_note||' | '||? END,updated_at=? WHERE partner_user_id=? AND status IN ('requested','approved','processing')`, "Revisión automática por "+reason, "Revisión automática por "+reason, now, x.partner)
		}
	}
	a.partnerAuditV2714(actorUserID, 0, "payment."+reason+".partner_reversal", map[string]any{"payment_id": paymentID})
	return nil
}

func (a *App) partnerPayoutsHandlerV2715(w http.ResponseWriter, r *http.Request) {
	u := a.currentUser(r)
	if u == nil {
		writeError(w, errors.New("sesión requerida"), 401)
		return
	}
	if !a.partnerEarningEligibleV2715(u.ID) {
		writeError(w, errors.New("Partner Center requiere licencia de pago vigente"), 403)
		return
	}
	switch r.Method {
	case http.MethodGet:
		balances := a.partnerBalancesV2715(u.ID)
		profile := map[string]any{"method": "USDT_TRC20", "destination": "", "holder_name": "", "country": ""}
		var method, destination, holder, country, updated string
		if a.db.QueryRow(`SELECT method,destination,holder_name,country,updated_at FROM partner_payout_profiles_v2715 WHERE partner_user_id=?`, u.ID).Scan(&method, &destination, &holder, &country, &updated) == nil {
			profile = map[string]any{"method": method, "destination": destination, "holder_name": holder, "country": country, "updated_at": updated}
		}
		rows, err := a.db.Query(`SELECT id,amount,currency,status,method,destination,admin_note,reference,requested_at,reviewed_at,paid_at,updated_at FROM partner_payouts_v2714 WHERE partner_user_id=? ORDER BY id DESC LIMIT 100`, u.ID)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		items := []map[string]any{}
		for rows.Next() {
			var id int64
			var amount float64
			var currency, status, m, dest, note, ref, req, review, paid, upd string
			if rows.Scan(&id, &amount, &currency, &status, &m, &dest, &note, &ref, &req, &review, &paid, &upd) == nil {
				items = append(items, map[string]any{"id": id, "amount": amount, "currency": currency, "status": status, "method": m, "destination": dest, "admin_note": note, "reference": ref, "requested_at": req, "reviewed_at": review, "paid_at": paid, "updated_at": upd})
			}
		}
		rows.Close()
		writeJSON(w, map[string]any{"balances": balances, "profile": profile, "items": items, "minimum": a.partnerSettingFloatV2714("payout_min_amount", 20)})
	case http.MethodPost:
		var q struct {
			Action      string  `json:"action"`
			Amount      float64 `json:"amount"`
			Method      string  `json:"method"`
			Destination string  `json:"destination"`
			HolderName  string  `json:"holder_name"`
			Country     string  `json:"country"`
		}
		if json.NewDecoder(r.Body).Decode(&q) != nil {
			writeError(w, errors.New("datos inválidos"), 400)
			return
		}
		now := time.Now().UTC().Format(time.RFC3339)
		if q.Action == "profile" {
			q.Method = strings.TrimSpace(q.Method)
			q.Destination = strings.TrimSpace(q.Destination)
			if q.Method == "" || q.Destination == "" {
				writeError(w, errors.New("método y destino son obligatorios"), 400)
				return
			}
			_, _ = a.db.Exec(`INSERT INTO partner_payout_profiles_v2715(partner_user_id,method,destination,holder_name,country,updated_at) VALUES(?,?,?,?,?,?) ON CONFLICT(partner_user_id) DO UPDATE SET method=excluded.method,destination=excluded.destination,holder_name=excluded.holder_name,country=excluded.country,updated_at=excluded.updated_at`, u.ID, q.Method, q.Destination, strings.TrimSpace(q.HolderName), strings.TrimSpace(q.Country), now)
			writeJSON(w, map[string]any{"ok": true})
			return
		}
		if q.Action != "request" {
			writeError(w, errors.New("acción inválida"), 400)
			return
		}
		if q.Method == "" || q.Destination == "" {
			_ = a.db.QueryRow(`SELECT method,destination,holder_name,country FROM partner_payout_profiles_v2715 WHERE partner_user_id=?`, u.ID).Scan(&q.Method, &q.Destination, &q.HolderName, &q.Country)
		}
		if strings.TrimSpace(q.Method) == "" || strings.TrimSpace(q.Destination) == "" {
			writeError(w, errors.New("configura primero tu método de pago"), 400)
			return
		}
		min := a.partnerSettingFloatV2714("payout_min_amount", 20)
		balances := a.partnerBalancesV2715(u.ID)
		q.Amount = roundUSDT(q.Amount)
		if q.Amount < min {
			writeError(w, fmt.Errorf("el retiro mínimo es %.2f USD", min), 400)
			return
		}
		if q.Amount > balances["available"]+0.0001 {
			writeError(w, errors.New("saldo disponible insuficiente"), 400)
			return
		}
		res, err := a.db.Exec(`INSERT INTO partner_payouts_v2714(partner_user_id,amount,currency,status,method,destination,admin_note,requested_at,updated_at) VALUES(?,?,'USD','requested',?,?, '',?,?)`, u.ID, q.Amount, strings.TrimSpace(q.Method), strings.TrimSpace(q.Destination), now, now)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		id, _ := res.LastInsertId()
		a.partnerAuditV2714(u.ID, u.ID, "payout.requested", map[string]any{"payout_id": id, "amount": q.Amount, "method": q.Method})
		writeJSON(w, map[string]any{"ok": true, "id": id})
	case http.MethodPut:
		var q struct {
			ID int64 `json:"id"`
		}
		if json.NewDecoder(r.Body).Decode(&q) != nil || q.ID == 0 {
			writeError(w, errors.New("solicitud inválida"), 400)
			return
		}
		now := time.Now().UTC().Format(time.RFC3339)
		res, _ := a.db.Exec(`UPDATE partner_payouts_v2714 SET status='cancelled',cancelled_at=?,updated_at=? WHERE id=? AND partner_user_id=? AND status='requested'`, now, now, q.ID, u.ID)
		n, _ := res.RowsAffected()
		if n == 0 {
			writeError(w, errors.New("solo puedes cancelar solicitudes pendientes"), 400)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	default:
		http.Error(w, "Método no permitido", 405)
	}
}

func (a *App) adminPartnerPayoutsHandlerV2715(w http.ResponseWriter, r *http.Request) {
	admin, ok := a.requireSuperadmin(w, r)
	if !ok {
		return
	}
	if r.Method == http.MethodGet {
		rows, err := a.db.Query(`SELECT p.id,p.partner_user_id,u.name,u.email,p.amount,p.currency,p.status,p.method,p.destination,p.admin_note,p.reference,p.requested_at,p.reviewed_at,p.paid_at,p.updated_at FROM partner_payouts_v2714 p JOIN app_users u ON u.id=p.partner_user_id ORDER BY CASE p.status WHEN 'needs_review' THEN 0 WHEN 'requested' THEN 1 WHEN 'approved' THEN 2 WHEN 'processing' THEN 3 ELSE 4 END,p.id DESC LIMIT 500`)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var id, partnerID int64
			var amount float64
			var name, email, currency, status, method, destination, note, ref, requested, reviewed, paid, updated string
			if rows.Scan(&id, &partnerID, &name, &email, &amount, &currency, &status, &method, &destination, &note, &ref, &requested, &reviewed, &paid, &updated) == nil {
				out = append(out, map[string]any{"id": id, "partner_user_id": partnerID, "partner_name": name, "partner_email": email, "amount": amount, "currency": currency, "status": status, "method": method, "destination": destination, "admin_note": note, "reference": ref, "requested_at": requested, "reviewed_at": reviewed, "paid_at": paid, "updated_at": updated})
			}
		}
		writeJSON(w, out)
		return
	}
	if r.Method != http.MethodPut {
		http.Error(w, "Método no permitido", 405)
		return
	}
	var q struct {
		ID        int64  `json:"id"`
		Action    string `json:"action"`
		Note      string `json:"note"`
		Reference string `json:"reference"`
	}
	if json.NewDecoder(r.Body).Decode(&q) != nil || q.ID == 0 {
		writeError(w, errors.New("payout inválido"), 400)
		return
	}
	var partnerID int64
	var amount float64
	var status string
	if a.db.QueryRow(`SELECT partner_user_id,amount,status FROM partner_payouts_v2714 WHERE id=?`, q.ID).Scan(&partnerID, &amount, &status) != nil {
		writeError(w, errors.New("payout no encontrado"), 404)
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	switch q.Action {
	case "approve":
		if status != "requested" && status != "needs_review" {
			writeError(w, errors.New("estado no permite aprobación"), 400)
			return
		}
		if a.partnerBalancesV2715(partnerID)["available_gross"]+0.0001 < amount {
			writeError(w, errors.New("el saldo bruto ya no cubre este payout; revisa reversos"), 400)
			return
		}
		_, _ = a.db.Exec(`UPDATE partner_payouts_v2714 SET status='approved',admin_note=?,reviewed_at=?,reviewed_by=?,updated_at=? WHERE id=?`, strings.TrimSpace(q.Note), now, admin.ID, now, q.ID)
	case "processing":
		if status != "approved" {
			writeError(w, errors.New("primero aprueba el payout"), 400)
			return
		}
		_, _ = a.db.Exec(`UPDATE partner_payouts_v2714 SET status='processing',updated_at=? WHERE id=?`, now, q.ID)
	case "reject":
		if status == "paid" {
			writeError(w, errors.New("un payout pagado no puede rechazarse"), 400)
			return
		}
		_, _ = a.db.Exec(`UPDATE partner_payouts_v2714 SET status='rejected',admin_note=?,reviewed_at=?,reviewed_by=?,updated_at=? WHERE id=?`, strings.TrimSpace(q.Note), now, admin.ID, now, q.ID)
	case "pay":
		if status != "approved" && status != "processing" {
			writeError(w, errors.New("el payout debe estar aprobado o en proceso"), 400)
			return
		}
		if strings.TrimSpace(q.Reference) == "" {
			writeError(w, errors.New("registra hash, referencia o comprobante de pago"), 400)
			return
		}
		if a.partnerBalancesV2715(partnerID)["available_gross"]+0.0001 < amount {
			writeError(w, errors.New("saldo insuficiente después de reversos; revisa antes de pagar"), 400)
			return
		}
		_, err := a.addPartnerAmountMovementV2715(partnerID, "payout_debit", fmt.Sprintf("payout:%d", q.ID), -roundUSDT(amount), "available", map[string]any{"payout_id": q.ID, "reference": strings.TrimSpace(q.Reference)})
		if err != nil {
			writeError(w, err, 500)
			return
		}
		_, _ = a.db.Exec(`UPDATE partner_payouts_v2714 SET status='paid',reference=?,admin_note=?,paid_at=?,paid_by=?,updated_at=? WHERE id=?`, strings.TrimSpace(q.Reference), strings.TrimSpace(q.Note), now, admin.ID, now, q.ID)
	default:
		writeError(w, errors.New("acción inválida"), 400)
		return
	}
	a.partnerAuditV2714(admin.ID, partnerID, "admin.payout."+q.Action, map[string]any{"payout_id": q.ID, "amount": amount, "reference": q.Reference})
	writeJSON(w, map[string]any{"ok": true})
}

func (a *App) reconcilePartnerAcademyEventRawV2715(eventID int64, email, raw string) bool {
	u, err := a.partnerUserByEmailV2715(email)
	if err != nil || u == nil || !u.Active {
		return false
	}
	var q struct {
		ExamCode       string  `json:"exam_code"`
		Status         string  `json:"status"`
		Score          float64 `json:"score"`
		PassingScore   float64 `json:"passing_score"`
		CertificateID  string  `json:"certificate_id"`
		CertificateURL string  `json:"certificate_url"`
		IssuedAt       string  `json:"issued_at"`
	}
	if json.Unmarshal([]byte(raw), &q) != nil {
		return false
	}
	if strings.TrimSpace(q.ExamCode) == "" {
		q.ExamCode = a.partnerSettingTextV2715("academy_activate_exam_code", "activate-certified")
	}
	if err := a.applyAcademyCertificationV2715(u, q.ExamCode, q.Status, q.Score, q.PassingScore, q.CertificateID, q.CertificateURL, q.IssuedAt, "academy_reconcile"); err != nil {
		return false
	}
	_, _ = a.db.Exec(`UPDATE partner_academy_events_v2715 SET matched_user_id=?,processed=1 WHERE id=?`, u.ID, eventID)
	return true
}

func (a *App) reconcilePartnerAcademyForUserV2715(u *User) {
	if u == nil || strings.TrimSpace(u.Email) == "" {
		return
	}
	rows, err := a.db.Query(`SELECT id,raw_json FROM partner_academy_events_v2715 WHERE processed=0 AND lower(academy_email)=lower(?) ORDER BY id ASC LIMIT 50`, u.Email)
	if err != nil {
		return
	}
	type item struct {
		id  int64
		raw string
	}
	items := []item{}
	for rows.Next() {
		var x item
		if rows.Scan(&x.id, &x.raw) == nil {
			items = append(items, x)
		}
	}
	rows.Close()
	for _, x := range items {
		_ = a.reconcilePartnerAcademyEventRawV2715(x.id, u.Email, x.raw)
	}
}

func (a *App) reconcilePartnerAcademyEventsV2715() error {
	rows, err := a.db.Query(`SELECT id,academy_email,raw_json FROM partner_academy_events_v2715 WHERE processed=0 ORDER BY id ASC LIMIT 1000`)
	if err != nil {
		return err
	}
	type item struct {
		id         int64
		email, raw string
	}
	items := []item{}
	for rows.Next() {
		var x item
		if rows.Scan(&x.id, &x.email, &x.raw) == nil {
			items = append(items, x)
		}
	}
	rows.Close()
	for _, x := range items {
		_ = a.reconcilePartnerAcademyEventRawV2715(x.id, x.email, x.raw)
	}
	return nil
}

func (a *App) academyPartnerCertificationWebhookV2715(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	secret := strings.TrimSpace(a.cfg.PartnerAcademyWebhookSecret)
	if secret == "" {
		writeError(w, errors.New("integración Academy no configurada"), 503)
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeError(w, errors.New("payload inválido"), 400)
		return
	}
	sig := strings.TrimSpace(r.Header.Get("X-Worktic-Signature"))
	if strings.HasPrefix(sig, "sha256=") {
		sig = strings.TrimPrefix(sig, "sha256=")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(raw)
	expected := hex.EncodeToString(mac.Sum(nil))
	if sig == "" || !hmac.Equal([]byte(strings.ToLower(sig)), []byte(strings.ToLower(expected))) {
		writeError(w, errors.New("firma Academy inválida"), 401)
		return
	}
	var q struct {
		Email          string  `json:"email"`
		ExamCode       string  `json:"exam_code"`
		Status         string  `json:"status"`
		Score          float64 `json:"score"`
		PassingScore   float64 `json:"passing_score"`
		CertificateID  string  `json:"certificate_id"`
		CertificateURL string  `json:"certificate_url"`
		IssuedAt       string  `json:"issued_at"`
	}
	if json.Unmarshal(raw, &q) != nil || strings.TrimSpace(q.Email) == "" {
		writeError(w, errors.New("evento Academy inválido"), 400)
		return
	}
	if q.ExamCode == "" {
		q.ExamCode = a.partnerSettingTextV2715("academy_activate_exam_code", "activate-certified")
	}
	q.Status = strings.ToLower(strings.TrimSpace(q.Status))
	now := time.Now().UTC().Format(time.RFC3339)
	matchedID := int64(0)
	processed := 0
	u, userErr := a.partnerUserByEmailV2715(q.Email)
	if userErr == nil && u.Active {
		matchedID = u.ID
		if err := a.applyAcademyCertificationV2715(u, q.ExamCode, q.Status, q.Score, q.PassingScore, q.CertificateID, q.CertificateURL, q.IssuedAt, "academy_webhook"); err == nil {
			processed = 1
		}
	}
	_, _ = a.db.Exec(`INSERT INTO partner_academy_events_v2715(academy_email,exam_code,event_status,score,certificate_id,matched_user_id,raw_json,processed,created_at) VALUES(?,?,?,?,?,?,?,?,?)`, strings.ToLower(strings.TrimSpace(q.Email)), q.ExamCode, q.Status, q.Score, q.CertificateID, matchedID, string(raw), processed, now)
	writeJSON(w, map[string]any{"ok": true, "matched_user_id": matchedID, "processed": processed == 1})
}

func (a *App) applyAcademyCertificationV2715(u *User, examCode, status string, score, passing float64, certificateID, certificateURL, issuedAt, source string) error {
	if u == nil || u.ID == 0 {
		return errors.New("usuario inválido")
	}
	if issuedAt == "" {
		issuedAt = time.Now().UTC().Format(time.RFC3339)
	}
	status = strings.ToLower(strings.TrimSpace(status))
	passed := status == "passed" || status == "certified" || status == "approved"
	if passed && passing > 0 && score+0.0001 < passing {
		passed = false
		status = "failed"
	}
	now := time.Now().UTC().Format(time.RFC3339)
	certStatus := "failed"
	if passed {
		certStatus = "passed"
	} else if status == "revoked" || status == "suspended" {
		certStatus = "revoked"
	}
	_, err := a.db.Exec(`INSERT INTO partner_certifications_v2715(partner_user_id,academy_email,exam_code,score,passing_score,certificate_id,certificate_url,status,source,issued_at,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(partner_user_id,exam_code) DO UPDATE SET academy_email=excluded.academy_email,score=excluded.score,passing_score=excluded.passing_score,certificate_id=excluded.certificate_id,certificate_url=excluded.certificate_url,status=excluded.status,source=excluded.source,issued_at=excluded.issued_at,updated_at=excluded.updated_at`, u.ID, strings.ToLower(strings.TrimSpace(u.Email)), examCode, score, passing, strings.TrimSpace(certificateID), strings.TrimSpace(certificateURL), certStatus, source, issuedAt, now, now)
	if err != nil {
		return err
	}
	var profileExists int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM partner_profiles_v2714 WHERE user_id=?`, u.ID).Scan(&profileExists)
	eligible, _, _ := a.partnerPaidEligibilityV2715(u.ID)
	if passed && profileExists == 0 && eligible {
		_, _ = a.ensurePartnerProfileV2714(u)
		profileExists = 1
	}
	if profileExists > 0 {
		activateStatus := "exam_pending"
		if passed {
			activateStatus = "certified"
		} else if certStatus == "revoked" {
			activateStatus = "suspended"
		}
		_, _ = a.db.Exec(`UPDATE partner_profiles_v2714 SET activate_status=?,updated_at=? WHERE user_id=?`, activateStatus, now, u.ID)
	}
	a.partnerAuditV2714(0, u.ID, "academy.certification."+certStatus, map[string]any{"exam_code": examCode, "score": score, "passing_score": passing, "certificate_id": certificateID, "source": source})
	return nil
}

func (a *App) partnerCertificationHandlerV2715(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", 405)
		return
	}
	u := a.currentUser(r)
	if u == nil {
		writeError(w, errors.New("sesión requerida"), 401)
		return
	}
	var status string
	_ = a.db.QueryRow(`SELECT COALESCE(activate_status,'not_certified') FROM partner_profiles_v2714 WHERE user_id=?`, u.ID).Scan(&status)
	items := []map[string]any{}
	rows, err := a.db.Query(`SELECT exam_code,score,passing_score,certificate_id,certificate_url,status,source,issued_at,updated_at FROM partner_certifications_v2715 WHERE partner_user_id=? ORDER BY id DESC`, u.ID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var exam, certID, certURL, s, source, issued, updated string
			var score, passing float64
			if rows.Scan(&exam, &score, &passing, &certID, &certURL, &s, &source, &issued, &updated) == nil {
				items = append(items, map[string]any{"exam_code": exam, "score": score, "passing_score": passing, "certificate_id": certID, "certificate_url": certURL, "status": s, "source": source, "issued_at": issued, "updated_at": updated})
			}
		}
	}
	writeJSON(w, map[string]any{"activate_status": status, "academy_url": "https://academy.workticai.com", "items": items})
}

func (a *App) adminPartnerCertificationsHandlerV2715(w http.ResponseWriter, r *http.Request) {
	admin, ok := a.requireSuperadmin(w, r)
	if !ok {
		return
	}
	if r.Method == http.MethodGet {
		rows, err := a.db.Query(`SELECT c.partner_user_id,u.name,u.email,c.exam_code,c.score,c.passing_score,c.certificate_id,c.certificate_url,c.status,c.source,c.issued_at,c.updated_at FROM partner_certifications_v2715 c JOIN app_users u ON u.id=c.partner_user_id ORDER BY c.id DESC LIMIT 500`)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var uid int64
			var name, email, exam, certID, certURL, status, source, issued, updated string
			var score, passing float64
			if rows.Scan(&uid, &name, &email, &exam, &score, &passing, &certID, &certURL, &status, &source, &issued, &updated) == nil {
				out = append(out, map[string]any{"partner_user_id": uid, "name": name, "email": email, "exam_code": exam, "score": score, "passing_score": passing, "certificate_id": certID, "certificate_url": certURL, "status": status, "source": source, "issued_at": issued, "updated_at": updated})
			}
		}
		writeJSON(w, out)
		return
	}
	if r.Method != http.MethodPut {
		http.Error(w, "Método no permitido", 405)
		return
	}
	var q struct {
		UserID         int64   `json:"user_id"`
		Action         string  `json:"action"`
		Score          float64 `json:"score"`
		PassingScore   float64 `json:"passing_score"`
		CertificateID  string  `json:"certificate_id"`
		CertificateURL string  `json:"certificate_url"`
	}
	if json.NewDecoder(r.Body).Decode(&q) != nil || q.UserID == 0 {
		writeError(w, errors.New("usuario inválido"), 400)
		return
	}
	var email string
	if a.db.QueryRow(`SELECT email FROM app_users WHERE id=?`, q.UserID).Scan(&email) != nil {
		writeError(w, errors.New("usuario no encontrado"), 404)
		return
	}
	u, _ := a.partnerUserByEmailV2715(email)
	status := "passed"
	if q.Action == "revoke" {
		status = "revoked"
	} else if q.Action != "certify" {
		writeError(w, errors.New("acción inválida"), 400)
		return
	}
	if q.PassingScore == 0 {
		q.PassingScore = 80
	}
	if q.Action == "certify" && q.Score == 0 {
		q.Score = 100
	}
	if err := a.applyAcademyCertificationV2715(u, a.partnerSettingTextV2715("academy_activate_exam_code", "activate-certified"), status, q.Score, q.PassingScore, q.CertificateID, q.CertificateURL, time.Now().UTC().Format(time.RFC3339), "admin"); err != nil {
		writeError(w, err, 500)
		return
	}
	a.partnerAuditV2714(admin.ID, q.UserID, "admin.certification."+q.Action, q)
	writeJSON(w, map[string]any{"ok": true})
}

func (a *App) partnerMarketplaceJobsHandlerV2715(w http.ResponseWriter, r *http.Request) {
	u := a.currentUser(r)
	if u == nil {
		writeError(w, errors.New("sesión requerida"), 401)
		return
	}
	switch r.Method {
	case http.MethodGet:
		rows, err := a.db.Query(`SELECT j.id,j.tenant_id,j.business_user_id,j.business_name,j.title,j.product_service,j.description,j.region,j.modality,j.commission_percent,j.currency,j.leads_source,j.requirements,j.slots,j.status,j.created_at,j.updated_at,(SELECT COUNT(*) FROM partner_marketplace_applications_v2715 a WHERE a.job_id=j.id AND a.status='accepted') FROM partner_marketplace_jobs_v2715 j WHERE j.status='open' OR j.tenant_id=? ORDER BY CASE WHEN j.status='open' THEN 0 ELSE 1 END,j.id DESC LIMIT 250`, u.TenantID)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var id, tenantID, businessUserID int64
			var slots, accepted int
			var business, title, product, desc, region, modality, currency, leads, reqs, status, created, updated string
			var pct float64
			if rows.Scan(&id, &tenantID, &businessUserID, &business, &title, &product, &desc, &region, &modality, &pct, &currency, &leads, &reqs, &slots, &status, &created, &updated, &accepted) == nil {
				out = append(out, map[string]any{"id": id, "tenant_id": tenantID, "business_user_id": businessUserID, "business_name": business, "title": title, "product_service": product, "description": desc, "region": region, "modality": modality, "commission_percent": pct, "currency": currency, "leads_source": leads, "requirements": reqs, "slots": slots, "accepted": accepted, "status": status, "created_at": created, "updated_at": updated, "owned_by_me": tenantID == u.TenantID && (u.Role == "owner" || u.Role == "admin" || u.Role == "superadmin")})
			}
		}
		writeJSON(w, out)
	case http.MethodPost:
		if u.Role != "superadmin" && (u.Role != "owner" && u.Role != "admin" || u.AccountType != "business") {
			writeError(w, errors.New("solo empresas WorkticAI pueden publicar oportunidades comerciales"), 403)
			return
		}
		var q struct {
			Title             string  `json:"title"`
			ProductService    string  `json:"product_service"`
			Description       string  `json:"description"`
			Region            string  `json:"region"`
			Modality          string  `json:"modality"`
			CommissionPercent float64 `json:"commission_percent"`
			Currency          string  `json:"currency"`
			LeadsSource       string  `json:"leads_source"`
			Requirements      string  `json:"requirements"`
			Slots             int     `json:"slots"`
		}
		if json.NewDecoder(r.Body).Decode(&q) != nil || strings.TrimSpace(q.Title) == "" || q.CommissionPercent <= 0 || q.CommissionPercent > 50 {
			writeError(w, errors.New("título y comisión válida (0-50%) son obligatorios"), 400)
			return
		}
		if q.Slots < 1 {
			q.Slots = 1
		}
		if q.Slots > 100 {
			q.Slots = 100
		}
		q.Currency = "USD" // Partner Center liquida comisiones y payouts en USD.
		if q.Modality == "" {
			q.Modality = "remote"
		}
		if q.LeadsSource == "" {
			q.LeadsSource = "business"
		}
		now := time.Now().UTC().Format(time.RFC3339)
		business := firstNonEmpty(u.Company, u.Name)
		res, err := a.db.Exec(`INSERT INTO partner_marketplace_jobs_v2715(tenant_id,business_user_id,business_name,title,product_service,description,region,modality,commission_percent,currency,leads_source,requirements,slots,status,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,'pending_review',?,?)`, u.TenantID, u.ID, business, strings.TrimSpace(q.Title), strings.TrimSpace(q.ProductService), strings.TrimSpace(q.Description), strings.TrimSpace(q.Region), strings.TrimSpace(q.Modality), q.CommissionPercent, strings.ToUpper(strings.TrimSpace(q.Currency)), strings.TrimSpace(q.LeadsSource), strings.TrimSpace(q.Requirements), q.Slots, now, now)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		id, _ := res.LastInsertId()
		a.partnerAuditV2714(u.ID, 0, "marketplace.job.created", map[string]any{"job_id": id, "tenant_id": u.TenantID})
		writeJSON(w, map[string]any{"ok": true, "id": id, "status": "pending_review"})
	case http.MethodPut:
		var q struct {
			ID     int64  `json:"id"`
			Action string `json:"action"`
		}
		if json.NewDecoder(r.Body).Decode(&q) != nil || q.ID == 0 {
			writeError(w, errors.New("oportunidad inválida"), 400)
			return
		}
		var tenantID int64
		var status string
		if a.db.QueryRow(`SELECT tenant_id,status FROM partner_marketplace_jobs_v2715 WHERE id=?`, q.ID).Scan(&tenantID, &status) != nil || (u.Role != "superadmin" && tenantID != u.TenantID) {
			writeError(w, errors.New("oportunidad no encontrada"), 404)
			return
		}
		now := time.Now().UTC().Format(time.RFC3339)
		switch q.Action {
		case "pause":
			_, _ = a.db.Exec(`UPDATE partner_marketplace_jobs_v2715 SET status='paused',updated_at=? WHERE id=? AND status='open'`, now, q.ID)
		case "reopen":
			_, _ = a.db.Exec(`UPDATE partner_marketplace_jobs_v2715 SET status='pending_review',updated_at=? WHERE id=? AND status IN ('paused','closed')`, now, q.ID)
		case "close":
			_, _ = a.db.Exec(`UPDATE partner_marketplace_jobs_v2715 SET status='closed',updated_at=? WHERE id=?`, now, q.ID)
		default:
			writeError(w, errors.New("acción inválida"), 400)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	default:
		http.Error(w, "Método no permitido", 405)
	}
}

func (a *App) partnerMarketplaceApplicationsHandlerV2715(w http.ResponseWriter, r *http.Request) {
	u := a.currentUser(r)
	if u == nil {
		writeError(w, errors.New("sesión requerida"), 401)
		return
	}
	switch r.Method {
	case http.MethodGet:
		rows, err := a.db.Query(`SELECT a.id,a.job_id,j.title,j.business_name,j.tenant_id,a.specialist_partner_user_id,u.name,u.email,a.message,a.status,a.created_at,a.updated_at,a.accepted_at FROM partner_marketplace_applications_v2715 a JOIN partner_marketplace_jobs_v2715 j ON j.id=a.job_id JOIN app_users u ON u.id=a.specialist_partner_user_id WHERE a.specialist_partner_user_id=? OR j.tenant_id=? ORDER BY a.id DESC LIMIT 300`, u.ID, u.TenantID)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var id, jobID, tenantID, specialistID int64
			var title, business, specialistName, specialistEmail, message, status, created, updated, accepted string
			if rows.Scan(&id, &jobID, &title, &business, &tenantID, &specialistID, &specialistName, &specialistEmail, &message, &status, &created, &updated, &accepted) == nil {
				out = append(out, map[string]any{"id": id, "job_id": jobID, "job_title": title, "business_name": business, "tenant_id": tenantID, "specialist_partner_user_id": specialistID, "specialist_name": specialistName, "specialist_email": specialistEmail, "message": message, "status": status, "created_at": created, "updated_at": updated, "accepted_at": accepted, "is_mine": specialistID == u.ID, "is_business": tenantID == u.TenantID && (u.Role == "owner" || u.Role == "admin" || u.Role == "superadmin")})
			}
		}
		writeJSON(w, out)
	case http.MethodPost:
		var q struct {
			JobID   int64  `json:"job_id"`
			Message string `json:"message"`
		}
		if json.NewDecoder(r.Body).Decode(&q) != nil || q.JobID == 0 {
			writeError(w, errors.New("oportunidad inválida"), 400)
			return
		}
		var certified int
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM partner_profiles_v2714 WHERE user_id=? AND status='active' AND activate_status='certified'`, u.ID).Scan(&certified)
		if certified == 0 || !a.partnerEarningEligibleV2715(u.ID) {
			writeError(w, errors.New("para aplicar necesitas Activate Certified y licencia WorkticAI vigente"), 403)
			return
		}
		var status string
		var tenantID int64
		if a.db.QueryRow(`SELECT status,tenant_id FROM partner_marketplace_jobs_v2715 WHERE id=?`, q.JobID).Scan(&status, &tenantID) != nil || status != "open" {
			writeError(w, errors.New("oportunidad no disponible"), 400)
			return
		}
		if tenantID == u.TenantID {
			writeError(w, errors.New("no puedes aplicar a una oportunidad de tu propia empresa"), 400)
			return
		}
		now := time.Now().UTC().Format(time.RFC3339)
		_, err := a.db.Exec(`INSERT OR IGNORE INTO partner_marketplace_applications_v2715(job_id,specialist_partner_user_id,message,status,created_at,updated_at) VALUES(?,? ,?,'applied',?,?)`, q.JobID, u.ID, strings.TrimSpace(q.Message), now, now)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	case http.MethodPut:
		var q struct {
			ID     int64  `json:"id"`
			Action string `json:"action"`
		}
		if json.NewDecoder(r.Body).Decode(&q) != nil || q.ID == 0 {
			writeError(w, errors.New("aplicación inválida"), 400)
			return
		}
		var specialistID, tenantID, jobID int64
		var appStatus string
		if a.db.QueryRow(`SELECT a.specialist_partner_user_id,j.tenant_id,a.job_id,a.status FROM partner_marketplace_applications_v2715 a JOIN partner_marketplace_jobs_v2715 j ON j.id=a.job_id WHERE a.id=?`, q.ID).Scan(&specialistID, &tenantID, &jobID, &appStatus) != nil {
			writeError(w, errors.New("aplicación no encontrada"), 404)
			return
		}
		now := time.Now().UTC().Format(time.RFC3339)
		if q.Action == "withdraw" {
			if specialistID != u.ID || appStatus == "accepted" {
				writeError(w, errors.New("no puedes retirar esta aplicación"), 403)
				return
			}
			_, _ = a.db.Exec(`UPDATE partner_marketplace_applications_v2715 SET status='withdrawn',updated_at=? WHERE id=?`, now, q.ID)
			writeJSON(w, map[string]any{"ok": true})
			return
		}
		if u.Role != "superadmin" && (u.Role != "owner" && u.Role != "admin" || tenantID != u.TenantID) {
			writeError(w, errors.New("solo la empresa puede gestionar candidatos"), 403)
			return
		}
		switch q.Action {
		case "accept":
			var slots, accepted int
			_ = a.db.QueryRow(`SELECT slots FROM partner_marketplace_jobs_v2715 WHERE id=?`, jobID).Scan(&slots)
			_ = a.db.QueryRow(`SELECT COUNT(*) FROM partner_marketplace_applications_v2715 WHERE job_id=? AND status='accepted'`, jobID).Scan(&accepted)
			if accepted >= slots {
				writeError(w, errors.New("ya se cubrieron los cupos de esta oportunidad"), 400)
				return
			}
			_, _ = a.db.Exec(`UPDATE partner_marketplace_applications_v2715 SET status='accepted',accepted_at=?,updated_at=? WHERE id=?`, now, now, q.ID)
		case "reject":
			_, _ = a.db.Exec(`UPDATE partner_marketplace_applications_v2715 SET status='rejected',updated_at=? WHERE id=?`, now, q.ID)
		default:
			writeError(w, errors.New("acción inválida"), 400)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	default:
		http.Error(w, "Método no permitido", 405)
	}
}

func (a *App) partnerMarketplaceSalesHandlerV2715(w http.ResponseWriter, r *http.Request) {
	u := a.currentUser(r)
	if u == nil {
		writeError(w, errors.New("sesión requerida"), 401)
		return
	}
	switch r.Method {
	case http.MethodGet:
		rows, err := a.db.Query(`SELECT s.id,s.application_id,s.job_id,j.title,s.tenant_id,j.business_name,s.business_user_id,s.specialist_partner_user_id,su.name,s.crm_opportunity_id,s.customer_name,s.external_reference,s.gross_sale_amount,s.commission_percent,s.commission_pool,s.specialist_percent,s.platform_percent,s.specialist_amount,s.platform_amount,s.currency,s.status,s.business_note,s.admin_note,s.funding_reference,s.reported_at,s.business_confirmed_at,s.approved_at,s.reversed_at,s.updated_at FROM partner_marketplace_sales_v2715 s JOIN partner_marketplace_jobs_v2715 j ON j.id=s.job_id JOIN app_users su ON su.id=s.specialist_partner_user_id WHERE s.specialist_partner_user_id=? OR s.tenant_id=? ORDER BY s.id DESC LIMIT 300`, u.ID, u.TenantID)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var id, appID, jobID, tenantID, businessUserID, specialistID, oppID int64
			var title, business, specialist, customer, ext, currency, status, businessNote, adminNote, funding, reported, confirmed, approved, reversed, updated string
			var gross, cpct, pool, spct, ppct, samount, pamount float64
			if rows.Scan(&id, &appID, &jobID, &title, &tenantID, &business, &businessUserID, &specialistID, &specialist, &oppID, &customer, &ext, &gross, &cpct, &pool, &spct, &ppct, &samount, &pamount, &currency, &status, &businessNote, &adminNote, &funding, &reported, &confirmed, &approved, &reversed, &updated) == nil {
				out = append(out, map[string]any{"id": id, "application_id": appID, "job_id": jobID, "job_title": title, "tenant_id": tenantID, "business_name": business, "business_user_id": businessUserID, "specialist_partner_user_id": specialistID, "specialist_name": specialist, "crm_opportunity_id": oppID, "customer_name": customer, "external_reference": ext, "gross_sale_amount": gross, "commission_percent": cpct, "commission_pool": pool, "specialist_percent": spct, "platform_percent": ppct, "specialist_amount": samount, "platform_amount": pamount, "currency": currency, "status": status, "business_note": businessNote, "admin_note": adminNote, "funding_reference": funding, "reported_at": reported, "business_confirmed_at": confirmed, "approved_at": approved, "reversed_at": reversed, "updated_at": updated, "is_mine": specialistID == u.ID, "is_business": tenantID == u.TenantID})
			}
		}
		writeJSON(w, out)
	case http.MethodPost:
		var q struct {
			ApplicationID    int64   `json:"application_id"`
			CRMOpportunityID int64   `json:"crm_opportunity_id"`
			CustomerName     string  `json:"customer_name"`
			ExternalRef      string  `json:"external_reference"`
			GrossSaleAmount  float64 `json:"gross_sale_amount"`
		}
		if json.NewDecoder(r.Body).Decode(&q) != nil || q.ApplicationID == 0 || q.GrossSaleAmount <= 0 {
			writeError(w, errors.New("aplicación y monto de venta son obligatorios"), 400)
			return
		}
		var jobID, specialistID, tenantID, businessUserID int64
		var appStatus, currency string
		var commissionPct float64
		if a.db.QueryRow(`SELECT a.job_id,a.specialist_partner_user_id,a.status,j.tenant_id,j.business_user_id,j.commission_percent,j.currency FROM partner_marketplace_applications_v2715 a JOIN partner_marketplace_jobs_v2715 j ON j.id=a.job_id WHERE a.id=?`, q.ApplicationID).Scan(&jobID, &specialistID, &appStatus, &tenantID, &businessUserID, &commissionPct, &currency) != nil || specialistID != u.ID || appStatus != "accepted" {
			writeError(w, errors.New("asignación comercial no válida"), 403)
			return
		}
		spct := a.partnerSettingFloatV2714("human_sales_specialist_percent", 80)
		ppct := a.partnerSettingFloatV2714("human_sales_platform_percent", 20)
		if spct <= 0 || ppct < 0 || spct+ppct <= 0 {
			writeError(w, errors.New("configuración de reparto Human Sales inválida"), 500)
			return
		}
		totalSplit := spct + ppct
		spct = spct * 100 / totalSplit
		ppct = 100 - spct
		pool := roundUSDT(q.GrossSaleAmount * commissionPct / 100)
		specialistAmount := roundUSDT(pool * spct / 100)
		platformAmount := roundUSDT(pool - specialistAmount)
		now := time.Now().UTC().Format(time.RFC3339)
		res, err := a.db.Exec(`INSERT INTO partner_marketplace_sales_v2715(application_id,job_id,tenant_id,business_user_id,specialist_partner_user_id,crm_opportunity_id,customer_name,external_reference,gross_sale_amount,commission_percent,commission_pool,specialist_percent,platform_percent,specialist_amount,platform_amount,currency,status,reported_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?, 'reported',?,?)`, q.ApplicationID, jobID, tenantID, businessUserID, u.ID, q.CRMOpportunityID, strings.TrimSpace(q.CustomerName), strings.TrimSpace(q.ExternalRef), roundUSDT(q.GrossSaleAmount), commissionPct, pool, spct, ppct, specialistAmount, platformAmount, currency, now, now)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		id, _ := res.LastInsertId()
		writeJSON(w, map[string]any{"ok": true, "id": id, "commission_pool": pool, "specialist_amount": specialistAmount, "platform_amount": platformAmount})
	case http.MethodPut:
		var q struct {
			ID     int64  `json:"id"`
			Action string `json:"action"`
			Note   string `json:"note"`
		}
		if json.NewDecoder(r.Body).Decode(&q) != nil || q.ID == 0 {
			writeError(w, errors.New("venta inválida"), 400)
			return
		}
		var tenantID int64
		var status string
		if a.db.QueryRow(`SELECT tenant_id,status FROM partner_marketplace_sales_v2715 WHERE id=?`, q.ID).Scan(&tenantID, &status) != nil {
			writeError(w, errors.New("venta no encontrada"), 404)
			return
		}
		if u.Role != "superadmin" && (u.Role != "owner" && u.Role != "admin" || tenantID != u.TenantID) {
			writeError(w, errors.New("solo la empresa puede confirmar la venta"), 403)
			return
		}
		now := time.Now().UTC().Format(time.RFC3339)
		if q.Action == "confirm" && status == "reported" {
			_, _ = a.db.Exec(`UPDATE partner_marketplace_sales_v2715 SET status='business_confirmed',business_note=?,business_confirmed_at=?,updated_at=? WHERE id=?`, strings.TrimSpace(q.Note), now, now, q.ID)
			writeJSON(w, map[string]any{"ok": true})
			return
		}
		if q.Action == "reject" && status == "reported" {
			_, _ = a.db.Exec(`UPDATE partner_marketplace_sales_v2715 SET status='business_rejected',business_note=?,updated_at=? WHERE id=?`, strings.TrimSpace(q.Note), now, q.ID)
			writeJSON(w, map[string]any{"ok": true})
			return
		}
		writeError(w, errors.New("acción o estado inválido"), 400)
	default:
		http.Error(w, "Método no permitido", 405)
	}
}

func (a *App) adminPartnerMarketplaceHandlerV2715(w http.ResponseWriter, r *http.Request) {
	admin, ok := a.requireSuperadmin(w, r)
	if !ok {
		return
	}
	if r.Method == http.MethodGet {
		jobs := []map[string]any{}
		rows, _ := a.db.Query(`SELECT id,tenant_id,business_name,title,commission_percent,currency,slots,status,created_at FROM partner_marketplace_jobs_v2715 ORDER BY id DESC LIMIT 300`)
		if rows != nil {
			for rows.Next() {
				var id, tid int64
				var slots int
				var business, title, currency, status, created string
				var pct float64
				if rows.Scan(&id, &tid, &business, &title, &pct, &currency, &slots, &status, &created) == nil {
					jobs = append(jobs, map[string]any{"id": id, "tenant_id": tid, "business_name": business, "title": title, "commission_percent": pct, "currency": currency, "slots": slots, "status": status, "created_at": created})
				}
			}
			rows.Close()
		}
		sales := []map[string]any{}
		rows, _ = a.db.Query(`SELECT s.id,s.job_id,j.title,j.business_name,s.specialist_partner_user_id,u.name,s.gross_sale_amount,s.commission_percent,s.commission_pool,s.specialist_amount,s.platform_amount,s.currency,s.status,s.funding_reference,s.reported_at,s.business_confirmed_at,s.approved_at FROM partner_marketplace_sales_v2715 s JOIN partner_marketplace_jobs_v2715 j ON j.id=s.job_id JOIN app_users u ON u.id=s.specialist_partner_user_id ORDER BY s.id DESC LIMIT 500`)
		if rows != nil {
			for rows.Next() {
				var id, jobID, specialistID int64
				var title, business, specialist, currency, status, funding, reported, confirmed, approved string
				var gross, cpct, pool, specialistAmount, platformAmount float64
				if rows.Scan(&id, &jobID, &title, &business, &specialistID, &specialist, &gross, &cpct, &pool, &specialistAmount, &platformAmount, &currency, &status, &funding, &reported, &confirmed, &approved) == nil {
					sales = append(sales, map[string]any{"id": id, "job_id": jobID, "job_title": title, "business_name": business, "specialist_partner_user_id": specialistID, "specialist_name": specialist, "gross_sale_amount": gross, "commission_percent": cpct, "commission_pool": pool, "specialist_amount": specialistAmount, "platform_amount": platformAmount, "currency": currency, "status": status, "funding_reference": funding, "reported_at": reported, "business_confirmed_at": confirmed, "approved_at": approved})
				}
			}
			rows.Close()
		}
		writeJSON(w, map[string]any{"jobs": jobs, "sales": sales})
		return
	}
	if r.Method != http.MethodPut {
		http.Error(w, "Método no permitido", 405)
		return
	}
	var q struct {
		Kind             string `json:"kind"`
		ID               int64  `json:"id"`
		Action           string `json:"action"`
		Note             string `json:"note"`
		FundingReference string `json:"funding_reference"`
	}
	if json.NewDecoder(r.Body).Decode(&q) != nil || q.ID == 0 {
		writeError(w, errors.New("registro inválido"), 400)
		return
	}
	now := time.Now().UTC()
	if q.Kind == "job" {
		switch q.Action {
		case "approve":
			_, _ = a.db.Exec(`UPDATE partner_marketplace_jobs_v2715 SET status='open',updated_at=? WHERE id=? AND status='pending_review'`, now.Format(time.RFC3339), q.ID)
		case "reject":
			_, _ = a.db.Exec(`UPDATE partner_marketplace_jobs_v2715 SET status='rejected',updated_at=? WHERE id=?`, now.Format(time.RFC3339), q.ID)
		default:
			writeError(w, errors.New("acción inválida"), 400)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
		return
	}
	if q.Kind != "sale" {
		writeError(w, errors.New("tipo inválido"), 400)
		return
	}
	var specialistID int64
	var pool, spct, specialistAmount float64
	var status string
	if a.db.QueryRow(`SELECT specialist_partner_user_id,commission_pool,specialist_percent,specialist_amount,status FROM partner_marketplace_sales_v2715 WHERE id=?`, q.ID).Scan(&specialistID, &pool, &spct, &specialistAmount, &status) != nil {
		writeError(w, errors.New("venta no encontrada"), 404)
		return
	}
	switch q.Action {
	case "fund_and_approve":
		if status != "business_confirmed" {
			writeError(w, errors.New("la empresa debe confirmar la venta primero"), 400)
			return
		}
		if strings.TrimSpace(q.FundingReference) == "" {
			writeError(w, errors.New("registra referencia de fondeo/settlement"), 400)
			return
		}
		hold := a.partnerSettingIntV2714("human_sales_hold_days", 7)
		availableAt := now.AddDate(0, 0, hold)
		ledgerID, err := a.addPartnerCommissionV2715(specialistID, 0, 0, "human_sales", 0, pool, spct, fmt.Sprintf("human-sale:%d", q.ID), availableAt, map[string]any{"marketplace_sale_id": q.ID, "funding_reference": q.FundingReference})
		if err != nil {
			writeError(w, err, 500)
			return
		}
		_, _ = a.db.Exec(`UPDATE partner_marketplace_sales_v2715 SET status='approved',funding_reference=?,admin_note=?,ledger_id=?,approved_at=?,updated_at=? WHERE id=?`, strings.TrimSpace(q.FundingReference), strings.TrimSpace(q.Note), ledgerID, now.Format(time.RFC3339), now.Format(time.RFC3339), q.ID)
	case "reverse":
		if status != "approved" {
			writeError(w, errors.New("solo ventas aprobadas pueden reversarse"), 400)
			return
		}
		var ledgerID int64
		_ = a.db.QueryRow(`SELECT ledger_id FROM partner_marketplace_sales_v2715 WHERE id=?`, q.ID).Scan(&ledgerID)
		if ledgerID > 0 {
			var ledgerStatus string
			var amount float64
			_ = a.db.QueryRow(`SELECT status,amount FROM partner_commission_ledger_v2714 WHERE id=?`, ledgerID).Scan(&ledgerStatus, &amount)
			if ledgerStatus == "paid" {
				_, _ = a.addPartnerAmountMovementV2715(specialistID, "human_sales_reversal", fmt.Sprintf("human-sale-reversal:%d", q.ID), -amount, "available", map[string]any{"marketplace_sale_id": q.ID})
			} else {
				_, _ = a.db.Exec(`UPDATE partner_commission_ledger_v2714 SET status='reversed',updated_at=? WHERE id=?`, now.Format(time.RFC3339), ledgerID)
			}
		}
		_, _ = a.db.Exec(`UPDATE partner_marketplace_sales_v2715 SET status='reversed',admin_note=?,reversed_at=?,updated_at=? WHERE id=?`, strings.TrimSpace(q.Note), now.Format(time.RFC3339), now.Format(time.RFC3339), q.ID)
	default:
		writeError(w, errors.New("acción inválida"), 400)
		return
	}
	a.partnerAuditV2714(admin.ID, specialistID, "admin.marketplace.sale."+q.Action, map[string]any{"sale_id": q.ID, "specialist_amount": specialistAmount})
	writeJSON(w, map[string]any{"ok": true})
}

func extractJSONV2715(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "```") {
		raw = strings.TrimPrefix(raw, "```json")
		raw = strings.TrimPrefix(raw, "```")
		raw = strings.TrimSuffix(raw, "```")
		raw = strings.TrimSpace(raw)
	}
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		return raw[start : end+1]
	}
	return ""
}

func (a *App) partnerAutopilotHandlerV2715(w http.ResponseWriter, r *http.Request) {
	u := a.currentUser(r)
	if u == nil {
		writeError(w, errors.New("sesión requerida"), 401)
		return
	}
	if !a.partnerEarningEligibleV2715(u.ID) {
		writeError(w, errors.New("Partner Autopilot requiere Partner activo y licencia de pago vigente"), 403)
		return
	}
	if r.Method == http.MethodGet {
		rows, err := a.db.Query(`SELECT id,niche,goal,target_clients,offer,primary_channel,plan_json,status,campaign_id,landing_id,created_at,updated_at FROM partner_autopilot_plans_v2715 WHERE partner_user_id=? ORDER BY id DESC LIMIT 50`, u.ID)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var id, target, campaignID, landingID int64
			var niche, goal, offer, channel, planJSON, status, created, updated string
			if rows.Scan(&id, &niche, &goal, &target, &offer, &channel, &planJSON, &status, &campaignID, &landingID, &created, &updated) == nil {
				var plan any
				_ = json.Unmarshal([]byte(planJSON), &plan)
				out = append(out, map[string]any{"id": id, "niche": niche, "goal": goal, "target_clients": target, "offer": offer, "primary_channel": channel, "plan": plan, "status": status, "campaign_id": campaignID, "landing_id": landingID, "created_at": created, "updated_at": updated})
			}
		}
		writeJSON(w, out)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	var q struct {
		Niche          string `json:"niche"`
		Goal           string `json:"goal"`
		TargetClients  int    `json:"target_clients"`
		Offer          string `json:"offer"`
		PrimaryChannel string `json:"primary_channel"`
	}
	if json.NewDecoder(r.Body).Decode(&q) != nil || strings.TrimSpace(q.Niche) == "" {
		writeError(w, errors.New("indica el nicho que quieres trabajar"), 400)
		return
	}
	if q.TargetClients < 1 {
		q.TargetClients = 3
	}
	if q.TargetClients > 100 {
		q.TargetClients = 100
	}
	if q.Goal == "" {
		q.Goal = fmt.Sprintf("Conseguir %d nuevos clientes de WorkticAI", q.TargetClients)
	}
	if q.PrimaryChannel == "" {
		q.PrimaryChannel = "mixed"
	}
	if strings.TrimSpace(a.openAIKey()) == "" {
		writeError(w, errors.New("OpenAI no está configurado para Partner Autopilot"), 503)
		return
	}
	profile, _ := a.ensurePartnerProfileV2714(u)
	refURL := strings.TrimRight(a.cfg.BaseURL, "/") + "/r/" + fmt.Sprint(profile["partner_code"])
	system := `Eres Worktic Partner Autopilot. Diseñas planes comerciales prácticos para que un Partner venda WorkticAI de forma ética, sin spam y usando las herramientas reales de WorkticAI: CRM, Landing Pages, Growth & Campañas, Social Hub, Inbox, WhatsApp Business/Marketing con consentimiento, Copilot, Agenda, Analytics y automatizaciones. Devuelve SOLO JSON válido. No inventes integraciones activas ni resultados garantizados.`
	userPrompt := fmt.Sprintf(`Crea un plan para vender WorkticAI. Nicho: %s. Objetivo: %s. Meta de clientes: %d. Oferta/ángulo: %s. Canal principal: %s. Enlace atribuido: %s.
JSON requerido con estas claves exactas: positioning (string), ideal_customer (string), offer (string), landing_headline (string), landing_subheadline (string), cta (string), outreach_messages (array de 3 strings), followups (array de 3 strings), content_ideas (array de 5 strings), demo_script (array de 5 strings), objections (array de objetos con objection y answer), pipeline (array de strings), weekly_actions (array de 7 strings), compliance_note (string).`, strings.TrimSpace(q.Niche), strings.TrimSpace(q.Goal), q.TargetClients, strings.TrimSpace(q.Offer), strings.TrimSpace(q.PrimaryChannel), refURL)
	raw, err := a.callOpenAI(system, userPrompt)
	if err != nil {
		writeError(w, err, 502)
		return
	}
	jsonText := extractJSONV2715(raw)
	var parsed any
	if jsonText == "" || json.Unmarshal([]byte(jsonText), &parsed) != nil {
		parsed = map[string]any{"positioning": raw, "ideal_customer": q.Niche, "offer": q.Offer, "landing_headline": "Conoce WorkticAI", "landing_subheadline": "Automatiza marketing, atención y ventas.", "cta": "Conocer WorkticAI", "outreach_messages": []string{}, "followups": []string{}, "content_ideas": []string{}, "demo_script": []string{}, "objections": []any{}, "pipeline": []string{"Prospecto", "Contacto", "Demo", "Seguimiento", "Cierre"}, "weekly_actions": []string{}, "compliance_note": "Usa consentimiento y respeta las políticas de cada canal."}
		b, _ := json.Marshal(parsed)
		jsonText = string(b)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := a.db.Exec(`INSERT INTO partner_autopilot_plans_v2715(partner_user_id,tenant_id,niche,goal,target_clients,offer,primary_channel,plan_json,raw_text,status,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,'draft',?,?)`, u.ID, u.TenantID, strings.TrimSpace(q.Niche), strings.TrimSpace(q.Goal), q.TargetClients, strings.TrimSpace(q.Offer), strings.TrimSpace(q.PrimaryChannel), jsonText, raw, now, now)
	if err != nil {
		writeError(w, err, 500)
		return
	}
	id, _ := res.LastInsertId()
	writeJSON(w, map[string]any{"ok": true, "id": id, "plan": parsed})
}

func (a *App) partnerAutopilotActivateHandlerV2715(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	u := a.currentUser(r)
	if u == nil {
		writeError(w, errors.New("sesión requerida"), 401)
		return
	}
	var q struct {
		PlanID int64 `json:"plan_id"`
	}
	if json.NewDecoder(r.Body).Decode(&q) != nil || q.PlanID == 0 {
		writeError(w, errors.New("plan inválido"), 400)
		return
	}
	var niche, goal, offer, planJSON, status string
	var existingCampaign, existingLanding int64
	if a.db.QueryRow(`SELECT niche,goal,offer,plan_json,status,campaign_id,landing_id FROM partner_autopilot_plans_v2715 WHERE id=? AND partner_user_id=?`, q.PlanID, u.ID).Scan(&niche, &goal, &offer, &planJSON, &status, &existingCampaign, &existingLanding) != nil {
		writeError(w, errors.New("plan no encontrado"), 404)
		return
	}
	if existingCampaign > 0 || existingLanding > 0 {
		writeJSON(w, map[string]any{"ok": true, "campaign_id": existingCampaign, "landing_id": existingLanding, "already_activated": true})
		return
	}
	var plan map[string]any
	_ = json.Unmarshal([]byte(planJSON), &plan)
	get := func(k, fallback string) string {
		if v, ok := plan[k].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
		return fallback
	}
	now := time.Now().UTC().Format(time.RFC3339)
	campaignRes, err := a.db.Exec(`INSERT INTO marketing_campaigns(tenant_id,name,mode,objective,product,platforms,destination,audience,budget_daily,budget_total,starts_at,ends_at,status,ai_plan,created_at,updated_at) VALUES(?,?, 'organic',?,'WorkticAI','facebook,instagram,linkedin','whatsapp',?,0,0,'','','draft',?,?,?)`, u.TenantID, "Partner Autopilot · "+niche, goal, get("ideal_customer", niche), planJSON, now, now)
	if err != nil {
		writeError(w, err, 500)
		return
	}
	campaignID, _ := campaignRes.LastInsertId()
	if arr, ok := plan["content_ideas"].([]any); ok {
		for i, v := range arr {
			if i >= 5 {
				break
			}
			text := strings.TrimSpace(fmt.Sprint(v))
			if text == "" {
				continue
			}
			_, _ = a.db.Exec(`INSERT INTO marketing_content(tenant_id,campaign_id,platform,format,title,body,cta,media_url,scheduled_at,status,published_url,created_at) VALUES(?,?, 'instagram','post',?,?,?,'','','draft','',?)`, u.TenantID, campaignID, fmt.Sprintf("Partner Autopilot %d", i+1), text, get("cta", "Conocer WorkticAI"), now)
		}
	}
	profile, _ := a.ensurePartnerProfileV2714(u)
	refURL := strings.TrimRight(a.cfg.BaseURL, "/") + "/r/" + fmt.Sprint(profile["partner_code"])
	slug := fmt.Sprintf("partner-%s-%s", strings.ToLower(fmt.Sprint(profile["partner_code"])), strings.ToLower(randomToken(3)))
	benefits, _ := json.Marshal([]string{"CRM y seguimiento comercial", "Automatización con IA", "Canales y WhatsApp", "Growth, campañas y analítica"})
	landingRes, err := a.db.Exec(`INSERT INTO marketing_landings(tenant_id,name,slug,template,headline,subheadline,badge,primary_cta,primary_url,secondary_cta,secondary_url,hero_image,benefits_json,features_json,testimonials_json,faq_json,premium_json,form_id,campaign_id,accent,published,created_at,updated_at) VALUES(?,?,?,'aurora',?,?,?,?,?,'','', '',?,'[]','[]','[]','{}',0,?,'#7c3aed',0,?,?)`, u.TenantID, "Partner · "+niche, slug, get("landing_headline", "Descubre WorkticAI"), get("landing_subheadline", "Automatiza marketing, atención y ventas desde un solo lugar."), "WORKTICAI", get("cta", "Conocer WorkticAI"), refURL, string(benefits), campaignID, now, now)
	if err != nil {
		writeError(w, err, 500)
		return
	}
	landingID, _ := landingRes.LastInsertId()
	_, _ = a.db.Exec(`UPDATE partner_autopilot_plans_v2715 SET status='activated',campaign_id=?,landing_id=?,updated_at=? WHERE id=?`, campaignID, landingID, now, q.PlanID)
	a.partnerAuditV2714(u.ID, u.ID, "autopilot.activated", map[string]any{"plan_id": q.PlanID, "campaign_id": campaignID, "landing_id": landingID})
	writeJSON(w, map[string]any{"ok": true, "campaign_id": campaignID, "landing_id": landingID, "marketing_view": "marketing", "landing_view": "landings"})
}

func (a *App) adminPartnerSettingsHandlerV2715(w http.ResponseWriter, r *http.Request) {
	admin, ok := a.requireSuperadmin(w, r)
	if !ok {
		return
	}
	keys := []string{"commission_hold_days", "adoption_score_min", "adoption_percent", "renewal_percent", "mentorship_percent", "mentorship_min_rank", "cosell_referrer_split", "human_sales_specialist_percent", "human_sales_platform_percent", "human_sales_hold_days", "payout_min_amount", "rank_protection_days", "partner_license_required", "academy_activate_exam_code"}
	if r.Method == http.MethodGet {
		out := map[string]string{}
		for _, k := range keys {
			out[k] = a.partnerSettingTextV2715(k, "")
		}
		writeJSON(w, out)
		return
	}
	if r.Method != http.MethodPut {
		http.Error(w, "Método no permitido", 405)
		return
	}
	var q map[string]any
	if json.NewDecoder(r.Body).Decode(&q) != nil {
		writeError(w, errors.New("configuración inválida"), 400)
		return
	}
	allowed := map[string]bool{}
	for _, k := range keys {
		allowed[k] = true
	}
	now := time.Now().UTC().Format(time.RFC3339)
	for k, v := range q {
		if !allowed[k] {
			continue
		}
		text := strings.TrimSpace(fmt.Sprint(v))
		_, _ = a.db.Exec(`INSERT INTO partner_settings_v2714(key,value,updated_at) VALUES(?,?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value,updated_at=excluded.updated_at`, k, text, now)
	}
	a.partnerAuditV2714(admin.ID, 0, "admin.settings.updated", q)
	writeJSON(w, map[string]any{"ok": true})
}

func (a *App) adminPartnerRanksHandlerV2715(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireSuperadmin(w, r); !ok {
		return
	}
	if r.Method == http.MethodGet {
		a.partnerRanksHandlerV2714(w, r)
		return
	}
	if r.Method != http.MethodPut {
		http.Error(w, "Método no permitido", 405)
		return
	}
	var q struct {
		Code            string  `json:"code"`
		MinDirectActive int     `json:"min_direct_active"`
		MinAdopted      int     `json:"min_adopted"`
		DirectPercent   float64 `json:"direct_percent"`
		Level2Percent   float64 `json:"level2_percent"`
		Level3Percent   float64 `json:"level3_percent"`
	}
	if json.NewDecoder(r.Body).Decode(&q) != nil || q.Code == "" {
		writeError(w, errors.New("rango inválido"), 400)
		return
	}
	if q.DirectPercent < 0 || q.DirectPercent > 50 || q.Level2Percent < 0 || q.Level2Percent > 20 || q.Level3Percent < 0 || q.Level3Percent > 20 || q.MinDirectActive < 0 || q.MinAdopted < 0 {
		writeError(w, errors.New("valores de rango fuera de límites"), 400)
		return
	}
	res, _ := a.db.Exec(`UPDATE partner_rank_rules_v2714 SET min_direct_active=?,min_adopted=?,direct_percent=?,level2_percent=?,level3_percent=? WHERE code=?`, q.MinDirectActive, q.MinAdopted, q.DirectPercent, q.Level2Percent, q.Level3Percent, q.Code)
	n, _ := res.RowsAffected()
	if n == 0 {
		writeError(w, errors.New("rango no encontrado"), 404)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (a *App) adminPartnerCosellHandlerV2715(w http.ResponseWriter, r *http.Request) {
	admin, ok := a.requireSuperadmin(w, r)
	if !ok {
		return
	}
	if r.Method == http.MethodGet {
		rows, err := a.db.Query(`SELECT d.id,ru.name,cu.name,d.prospect_name,d.prospect_email,d.referrer_split,d.closer_split,d.referrer_accepted,d.closer_accepted,d.status,d.payment_id,d.created_at,d.updated_at FROM partner_cosell_deals_v2715 d JOIN app_users ru ON ru.id=d.referrer_partner_user_id JOIN app_users cu ON cu.id=d.closer_partner_user_id ORDER BY d.id DESC LIMIT 500`)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var id, paymentID int64
			var refName, closerName, prospect, email, status, created, updated string
			var refSplit, closerSplit float64
			var ra, ca int
			if rows.Scan(&id, &refName, &closerName, &prospect, &email, &refSplit, &closerSplit, &ra, &ca, &status, &paymentID, &created, &updated) == nil {
				out = append(out, map[string]any{"id": id, "referrer_name": refName, "closer_name": closerName, "prospect_name": prospect, "prospect_email": email, "referrer_split": refSplit, "closer_split": closerSplit, "referrer_accepted": ra == 1, "closer_accepted": ca == 1, "status": status, "payment_id": paymentID, "created_at": created, "updated_at": updated})
			}
		}
		writeJSON(w, out)
		return
	}
	if r.Method != http.MethodPut {
		http.Error(w, "Método no permitido", 405)
		return
	}
	var q struct {
		ID int64 `json:"id"`
	}
	if json.NewDecoder(r.Body).Decode(&q) != nil || q.ID == 0 {
		writeError(w, errors.New("acuerdo inválido"), 400)
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, _ = a.db.Exec(`UPDATE partner_cosell_deals_v2715 SET status='cancelled',updated_at=? WHERE id=? AND status NOT IN ('won','cancelled')`, now, q.ID)
	a.partnerAuditV2714(admin.ID, 0, "admin.cosell.cancelled", map[string]any{"deal_id": q.ID})
	writeJSON(w, map[string]any{"ok": true})
}

func (a *App) adminPartnerDashboardV2715(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireSuperadmin(w, r); !ok {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", 405)
		return
	}
	var partners, active, certified, jobsOpen, cosellOpen, payoutsPending int
	var commissions, payoutPaid, platformRevenue float64
	_ = a.db.QueryRow(`SELECT COUNT(*),COALESCE(SUM(CASE WHEN status='active' THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN activate_status='certified' THEN 1 ELSE 0 END),0) FROM partner_profiles_v2714`).Scan(&partners, &active, &certified)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM partner_marketplace_jobs_v2715 WHERE status='open'`).Scan(&jobsOpen)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM partner_cosell_deals_v2715 WHERE status IN ('pending','active')`).Scan(&cosellOpen)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM partner_payouts_v2714 WHERE status IN ('requested','needs_review','approved','processing')`).Scan(&payoutsPending)
	_ = a.db.QueryRow(`SELECT COALESCE(SUM(CASE WHEN amount>0 AND status<>'reversed' THEN amount ELSE 0 END),0) FROM partner_commission_ledger_v2714`).Scan(&commissions)
	_ = a.db.QueryRow(`SELECT COALESCE(SUM(amount),0) FROM partner_payouts_v2714 WHERE status='paid'`).Scan(&payoutPaid)
	_ = a.db.QueryRow(`SELECT COALESCE(SUM(platform_amount),0) FROM partner_marketplace_sales_v2715 WHERE status='approved'`).Scan(&platformRevenue)
	writeJSON(w, map[string]any{"partners": partners, "active": active, "certified": certified, "jobs_open": jobsOpen, "cosell_open": cosellOpen, "payouts_pending": payoutsPending, "commissions_generated": roundUSDT(commissions), "payouts_paid": roundUSDT(payoutPaid), "human_sales_platform_revenue": roundUSDT(platformRevenue)})
}

func (a *App) partnerLedgerHandlerV2715(w http.ResponseWriter, r *http.Request) {
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
	rows, err := a.db.Query(`SELECT l.id,l.customer_user_id,COALESCE(cu.name,''),l.payment_id,l.commission_type,l.level,l.eligible_base,l.percent,l.amount,l.currency,l.status,l.available_at,l.metadata_json,l.created_at FROM partner_commission_ledger_v2714 l LEFT JOIN app_users cu ON cu.id=l.customer_user_id WHERE l.partner_user_id=? ORDER BY l.id DESC LIMIT 500`, u.ID)
	if err != nil {
		writeError(w, err, 500)
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, customerID, paymentID int64
		var customerName, typ, currency, status, availableAt, metadata, createdAt string
		var level int
		var base, pct, amount float64
		if rows.Scan(&id, &customerID, &customerName, &paymentID, &typ, &level, &base, &pct, &amount, &currency, &status, &availableAt, &metadata, &createdAt) == nil {
			var meta any
			_ = json.Unmarshal([]byte(metadata), &meta)
			out = append(out, map[string]any{"id": id, "customer_user_id": customerID, "customer_name": customerName, "payment_id": paymentID, "commission_type": typ, "level": level, "eligible_base": base, "percent": pct, "amount": amount, "currency": currency, "status": status, "available_at": availableAt, "metadata": meta, "created_at": createdAt})
		}
	}
	writeJSON(w, map[string]any{"items": out, "balances": a.partnerBalancesV2715(u.ID)})
}

func parseIntQueryV2715(r *http.Request, key string, fallback int64) int64 {
	v, err := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get(key)), 10, 64)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}
