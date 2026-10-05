package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type SocialGovernanceV231 struct {
	Mode                string            `json:"mode"`
	EmergencyPause      bool              `json:"emergency_pause"`
	ChannelRules        map[string]string `json:"channel_rules"`
	ApprovalCampaignIDs []int64           `json:"approval_campaign_ids"`
	SensitiveKeywords   []string          `json:"sensitive_keywords"`
	BlockedKeywords     []string          `json:"blocked_keywords"`
	UpdatedAt           string            `json:"updated_at"`
}

type SocialGovernanceDecisionV231 struct {
	Mode            string `json:"mode"`
	Decision        string `json:"decision"`
	RequireApproval bool   `json:"require_approval"`
	Blocked         bool   `json:"blocked"`
	Reason          string `json:"reason"`
}

func initSocialGovernanceV231Schema(db *DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS social_governance_v231(
 tenant_id INTEGER PRIMARY KEY,
 mode TEXT NOT NULL DEFAULT 'autopilot',
 emergency_pause INTEGER NOT NULL DEFAULT 0,
 channel_rules_json TEXT NOT NULL DEFAULT '{}',
 approval_campaign_ids_json TEXT NOT NULL DEFAULT '[]',
 sensitive_keywords_json TEXT NOT NULL DEFAULT '[]',
 blocked_keywords_json TEXT NOT NULL DEFAULT '[]',
 updated_by INTEGER NOT NULL DEFAULT 0,
 updated_at TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS social_governance_audit_v231(
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 tenant_id INTEGER NOT NULL,
 group_id INTEGER NOT NULL DEFAULT 0,
 campaign_id INTEGER NOT NULL DEFAULT 0,
 action TEXT NOT NULL DEFAULT '',
 decision TEXT NOT NULL DEFAULT '',
 reason TEXT NOT NULL DEFAULT '',
 platforms_json TEXT NOT NULL DEFAULT '[]',
 created_by INTEGER NOT NULL DEFAULT 0,
 created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_social_governance_audit_tenant ON social_governance_audit_v231(tenant_id,created_at);
`)
	return err
}

func normalizeGovernanceModeV231(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "approval":
		return "approval"
	case "hybrid":
		return "hybrid"
	default:
		return "autopilot"
	}
}

func normalizeChannelRuleV231(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "approval":
		return "approval"
	case "autopilot":
		return "autopilot"
	default:
		return "inherit"
	}
}

func cleanKeywordListV231(xs []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, raw := range xs {
		v := strings.ToLower(strings.TrimSpace(raw))
		if v == "" || seen[v] {
			continue
		}
		if len(v) > 80 {
			v = v[:80]
		}
		seen[v] = true
		out = append(out, v)
		if len(out) >= 100 {
			break
		}
	}
	return out
}

func (a *App) loadSocialGovernanceV231(tid int64) SocialGovernanceV231 {
	g := SocialGovernanceV231{Mode: "autopilot", ChannelRules: map[string]string{}, ApprovalCampaignIDs: []int64{}, SensitiveKeywords: []string{}, BlockedKeywords: []string{}}
	var paused int
	var rulesJSON, campaignsJSON, sensitiveJSON, blockedJSON string
	err := a.db.QueryRow(`SELECT mode,emergency_pause,channel_rules_json,approval_campaign_ids_json,sensitive_keywords_json,blocked_keywords_json,updated_at FROM social_governance_v231 WHERE tenant_id=?`, tid).Scan(&g.Mode, &paused, &rulesJSON, &campaignsJSON, &sensitiveJSON, &blockedJSON, &g.UpdatedAt)
	if err != nil {
		return g
	}
	g.Mode = normalizeGovernanceModeV231(g.Mode)
	g.EmergencyPause = paused != 0
	_ = json.Unmarshal([]byte(rulesJSON), &g.ChannelRules)
	_ = json.Unmarshal([]byte(campaignsJSON), &g.ApprovalCampaignIDs)
	_ = json.Unmarshal([]byte(sensitiveJSON), &g.SensitiveKeywords)
	_ = json.Unmarshal([]byte(blockedJSON), &g.BlockedKeywords)
	if g.ChannelRules == nil {
		g.ChannelRules = map[string]string{}
	}
	return g
}

func containsAnyKeywordV231(content string, xs []string) string {
	hay := strings.ToLower(content)
	for _, x := range xs {
		x = strings.ToLower(strings.TrimSpace(x))
		if x != "" && strings.Contains(hay, x) {
			return x
		}
	}
	return ""
}

func int64InV231(v int64, xs []int64) bool {
	if v == 0 {
		return false
	}
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func (a *App) socialGovernanceDecisionV231(tid, campaignID int64, platforms []string, content, action string) SocialGovernanceDecisionV231 {
	g := a.loadSocialGovernanceV231(tid)
	d := SocialGovernanceDecisionV231{Mode: g.Mode, Decision: "autopilot", Reason: "Política permite ejecución automática"}
	action = strings.ToLower(strings.TrimSpace(action))
	if action != "publish" && action != "schedule" {
		d.Decision = "draft"
		d.Reason = "Los borradores no requieren gobernanza de publicación"
		return d
	}
	if g.EmergencyPause {
		d.Decision, d.Blocked, d.Reason = "paused", true, "Autopilot está pausado globalmente para esta empresa"
		return d
	}
	if kw := containsAnyKeywordV231(content, g.BlockedKeywords); kw != "" {
		d.Decision, d.Blocked, d.Reason = "blocked", true, "Contenido bloqueado por la regla: "+kw
		return d
	}
	allExplicitAutopilot := len(platforms) > 0
	for _, p := range platforms {
		p = strings.ToLower(strings.TrimSpace(p))
		rule := normalizeChannelRuleV231(g.ChannelRules[p])
		if rule == "approval" {
			d.Decision, d.RequireApproval, d.Reason = "approval", true, "La red "+p+" requiere aprobación"
			return d
		}
		if rule != "autopilot" {
			allExplicitAutopilot = false
		}
	}
	if int64InV231(campaignID, g.ApprovalCampaignIDs) {
		d.Decision, d.RequireApproval, d.Reason = "approval", true, "La campaña está configurada para aprobación"
		return d
	}
	if g.Mode == "hybrid" {
		if kw := containsAnyKeywordV231(content, g.SensitiveKeywords); kw != "" {
			d.Decision, d.RequireApproval, d.Reason = "approval", true, "Regla sensible detectada: "+kw
			return d
		}
	}
	if allExplicitAutopilot {
		d.Decision, d.RequireApproval, d.Reason = "autopilot", false, "Todas las redes seleccionadas tienen excepción Autopilot"
		return d
	}
	if g.Mode == "approval" {
		d.Decision, d.RequireApproval, d.Reason = "approval", true, "La empresa opera en modo Aprobación"
		return d
	}
	d.Decision = "autopilot"
	if g.Mode == "hybrid" {
		d.Reason = "Modo Híbrido: ninguna regla de revisión fue activada"
	}
	return d
}

func (a *App) auditSocialGovernanceV231(tid, groupID, campaignID, userID int64, action string, platforms []string, d SocialGovernanceDecisionV231) {
	b, _ := json.Marshal(platforms)
	_, _ = a.db.Exec(`INSERT INTO social_governance_audit_v231(tenant_id,group_id,campaign_id,action,decision,reason,platforms_json,created_by,created_at) VALUES(?,?,?,?,?,?,?,?,?)`, tid, groupID, campaignID, action, d.Decision, d.Reason, string(b), userID, time.Now().UTC().Format(time.RFC3339))
}

func (a *App) socialGovernanceV231Handler(w http.ResponseWriter, r *http.Request) {
	tid, u, err := a.tenantFor(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	perms := socialPermissionsFor(u)
	if !perms.View {
		http.Error(w, "No tienes acceso al Social Hub", http.StatusForbidden)
		return
	}
	switch r.Method {
	case http.MethodGet:
		g := a.loadSocialGovernanceV231(tid)
		var pending, held int
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM social_approvals WHERE tenant_id=? AND status='pending'`, tid).Scan(&pending)
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM social_posts WHERE tenant_id=? AND status='governance_hold'`, tid).Scan(&held)
		writeJSON(w, map[string]any{"settings": g, "can_manage": perms.ManageConnections, "pending_approvals": pending, "held_posts": held})
	case http.MethodPost:
		if !perms.ManageConnections {
			http.Error(w, "Solo propietarios y administradores pueden cambiar la gobernanza", http.StatusForbidden)
			return
		}
		var q struct {
			Action              string            `json:"action"`
			Mode                string            `json:"mode"`
			EmergencyPause      bool              `json:"emergency_pause"`
			ChannelRules        map[string]string `json:"channel_rules"`
			ApprovalCampaignIDs []int64           `json:"approval_campaign_ids"`
			SensitiveKeywords   []string          `json:"sensitive_keywords"`
			BlockedKeywords     []string          `json:"blocked_keywords"`
		}
		if json.NewDecoder(r.Body).Decode(&q) != nil {
			http.Error(w, "datos inválidos", http.StatusBadRequest)
			return
		}
		if q.Action == "toggle_pause" {
			now := time.Now().UTC().Format(time.RFC3339)
			_, err := a.db.Exec(`INSERT INTO social_governance_v231(tenant_id,mode,emergency_pause,updated_by,updated_at) VALUES(?,?,?,?,?) ON CONFLICT(tenant_id) DO UPDATE SET emergency_pause=excluded.emergency_pause,updated_by=excluded.updated_by,updated_at=excluded.updated_at`, tid, "autopilot", boolToIntV231(q.EmergencyPause), u.ID, now)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			if !q.EmergencyPause {
				_, _ = a.db.Exec(`UPDATE social_posts SET status='queued',error_message='',updated_at=? WHERE tenant_id=? AND status='governance_hold' AND error_message='GOVERNANCE_PAUSE'`, now, tid)
			}
			writeJSON(w, map[string]any{"ok": true, "emergency_pause": q.EmergencyPause})
			return
		}
		q.Mode = normalizeGovernanceModeV231(q.Mode)
		cleanRules := map[string]string{}
		for _, p := range []string{"facebook", "instagram", "tiktok", "youtube", "linkedin", "telegram"} {
			cleanRules[p] = normalizeChannelRuleV231(q.ChannelRules[p])
		}
		q.SensitiveKeywords = cleanKeywordListV231(q.SensitiveKeywords)
		q.BlockedKeywords = cleanKeywordListV231(q.BlockedKeywords)
		// IDs positivos, únicos y con límite razonable.
		campaigns := []int64{}
		seenCampaigns := map[int64]bool{}
		for _, id := range q.ApprovalCampaignIDs {
			if id > 0 && !seenCampaigns[id] {
				campaigns = append(campaigns, id)
				seenCampaigns[id] = true
				if len(campaigns) >= 200 {
					break
				}
			}
		}
		rulesJSON, _ := json.Marshal(cleanRules)
		campaignsJSON, _ := json.Marshal(campaigns)
		sensitiveJSON, _ := json.Marshal(q.SensitiveKeywords)
		blockedJSON, _ := json.Marshal(q.BlockedKeywords)
		now := time.Now().UTC().Format(time.RFC3339)
		_, err = a.db.Exec(`INSERT INTO social_governance_v231(tenant_id,mode,emergency_pause,channel_rules_json,approval_campaign_ids_json,sensitive_keywords_json,blocked_keywords_json,updated_by,updated_at) VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(tenant_id) DO UPDATE SET mode=excluded.mode,emergency_pause=excluded.emergency_pause,channel_rules_json=excluded.channel_rules_json,approval_campaign_ids_json=excluded.approval_campaign_ids_json,sensitive_keywords_json=excluded.sensitive_keywords_json,blocked_keywords_json=excluded.blocked_keywords_json,updated_by=excluded.updated_by,updated_at=excluded.updated_at`, tid, q.Mode, boolToIntV231(q.EmergencyPause), string(rulesJSON), string(campaignsJSON), string(sensitiveJSON), string(blockedJSON), u.ID, now)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		_, _ = a.db.Exec(`UPDATE social_posts SET status='queued',error_message='',updated_at=? WHERE tenant_id=? AND status='governance_hold' AND error_message LIKE 'GOVERNANCE_BLOCK:%'`, now, tid)
		writeJSON(w, map[string]any{"ok": true, "settings": a.loadSocialGovernanceV231(tid)})
	default:
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
	}
}

func boolToIntV231(v bool) int {
	if v {
		return 1
	}
	return 0
}

func parseCSVInt64V231(raw string) []int64 {
	out := []int64{}
	for _, part := range strings.Split(raw, ",") {
		n, _ := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if n > 0 {
			out = append(out, n)
		}
	}
	return out
}
