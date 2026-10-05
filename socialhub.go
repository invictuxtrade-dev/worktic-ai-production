package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type SocialConnection struct {
	ID                int64  `json:"id"`
	TenantID          int64  `json:"tenant_id"`
	Platform          string `json:"platform"`
	AccountName       string `json:"account_name"`
	ExternalAccountID string `json:"external_account_id"`
	Status            string `json:"status"`
	ProviderMode      string `json:"provider_mode"`
	Scopes            string `json:"scopes"`
	LastSyncAt        string `json:"last_sync_at"`
	LastError         string `json:"last_error"`
	CreatedAt         string `json:"created_at"`
	UpdatedAt         string `json:"updated_at"`
	Token             string `json:"token,omitempty"`
	ChatID            string `json:"chat_id,omitempty"`
}

type SocialPost struct {
	ID             int64  `json:"id"`
	TenantID       int64  `json:"tenant_id"`
	GroupID        int64  `json:"group_id"`
	CampaignID     int64  `json:"campaign_id"`
	ConnectionID   int64  `json:"connection_id"`
	Platform       string `json:"platform"`
	Format         string `json:"format"`
	Title          string `json:"title"`
	Caption        string `json:"caption"`
	CTA            string `json:"cta"`
	LinkURL        string `json:"link_url"`
	MediaJSON      string `json:"media_json"`
	ScheduledAt    string `json:"scheduled_at"`
	PublishedAt    string `json:"published_at"`
	Status         string `json:"status"`
	ExternalPostID string `json:"external_post_id"`
	PublishedURL   string `json:"published_url"`
	ErrorMessage   string `json:"error_message"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
	GroupName      string `json:"group_name,omitempty"`
	MasterContent  string `json:"master_content,omitempty"`
	Objective      string `json:"objective,omitempty"`
}

func initSocialHubSchema(db *DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS social_connections(
 id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL, platform TEXT NOT NULL,
 account_name TEXT NOT NULL DEFAULT '', external_account_id TEXT NOT NULL DEFAULT '',
 status TEXT NOT NULL DEFAULT 'disconnected', provider_mode TEXT NOT NULL DEFAULT 'sandbox',
 scopes TEXT NOT NULL DEFAULT '', last_sync_at TEXT NOT NULL DEFAULT '', last_error TEXT NOT NULL DEFAULT '',
 created_at TEXT NOT NULL, updated_at TEXT NOT NULL, UNIQUE(tenant_id,platform,external_account_id));
CREATE INDEX IF NOT EXISTS idx_social_connections_tenant ON social_connections(tenant_id);
CREATE TABLE IF NOT EXISTS social_post_groups(
 id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL, name TEXT NOT NULL DEFAULT '',
 master_content TEXT NOT NULL DEFAULT '', objective TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'draft',
 created_at TEXT NOT NULL, updated_at TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS idx_social_groups_tenant ON social_post_groups(tenant_id);
CREATE TABLE IF NOT EXISTS social_posts(
 id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL, group_id INTEGER NOT NULL DEFAULT 0,
 campaign_id INTEGER NOT NULL DEFAULT 0, connection_id INTEGER NOT NULL DEFAULT 0, platform TEXT NOT NULL,
 format TEXT NOT NULL DEFAULT 'post', title TEXT NOT NULL DEFAULT '', caption TEXT NOT NULL DEFAULT '',
 cta TEXT NOT NULL DEFAULT '', link_url TEXT NOT NULL DEFAULT '', media_json TEXT NOT NULL DEFAULT '[]',
 scheduled_at TEXT NOT NULL DEFAULT '', published_at TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'draft',
 external_post_id TEXT NOT NULL DEFAULT '', published_url TEXT NOT NULL DEFAULT '', error_message TEXT NOT NULL DEFAULT '',
 created_at TEXT NOT NULL, updated_at TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS idx_social_posts_tenant_status ON social_posts(tenant_id,status,scheduled_at);
CREATE TABLE IF NOT EXISTS social_metrics(
 id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL, post_id INTEGER NOT NULL, metric_date TEXT NOT NULL,
 impressions INTEGER NOT NULL DEFAULT 0, reach INTEGER NOT NULL DEFAULT 0, likes INTEGER NOT NULL DEFAULT 0,
 comments INTEGER NOT NULL DEFAULT 0, shares INTEGER NOT NULL DEFAULT 0, clicks INTEGER NOT NULL DEFAULT 0,
 video_views INTEGER NOT NULL DEFAULT 0, leads INTEGER NOT NULL DEFAULT 0, conversions INTEGER NOT NULL DEFAULT 0,
 UNIQUE(tenant_id,post_id,metric_date));
`)
	return err
}

func validSocialPlatform(p string) bool {
	switch p {
	case "facebook", "instagram", "linkedin", "telegram", "tiktok", "youtube":
		return true
	}
	return false
}

func (a *App) socialConnectionsHandler(w http.ResponseWriter, r *http.Request) {
	t, u, err := a.tenantFor(r)
	if err != nil {
		http.Error(w, err.Error(), 401)
		return
	}
	perms := socialPermissionsFor(u)
	if !perms.View {
		http.Error(w, "No tienes acceso al Social Hub", 403)
		return
	}
	switch r.Method {
	case "GET":
		rows, err := a.db.Query(`SELECT id,tenant_id,platform,account_name,external_account_id,status,provider_mode,scopes,last_sync_at,last_error,created_at,updated_at FROM social_connections WHERE tenant_id=? ORDER BY platform,account_name`, t)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		defer rows.Close()
		out := []SocialConnection{}
		for rows.Next() {
			var x SocialConnection
			_ = rows.Scan(&x.ID, &x.TenantID, &x.Platform, &x.AccountName, &x.ExternalAccountID, &x.Status, &x.ProviderMode, &x.Scopes, &x.LastSyncAt, &x.LastError, &x.CreatedAt, &x.UpdatedAt)
			out = append(out, x)
		}
		writeJSON(w, out)
	case "POST":
		if !perms.ManageConnections {
			http.Error(w, "Solo propietarios y administradores pueden conectar redes", 403)
			return
		}
		var x SocialConnection
		if json.NewDecoder(r.Body).Decode(&x) != nil {
			http.Error(w, "datos inválidos", 400)
			return
		}
		x.Platform = strings.ToLower(strings.TrimSpace(x.Platform))
		x.AccountName = strings.TrimSpace(x.AccountName)
		x.ExternalAccountID = strings.TrimSpace(x.ExternalAccountID)
		if !validSocialPlatform(x.Platform) || x.AccountName == "" {
			http.Error(w, "red y nombre de cuenta son obligatorios", 400)
			return
		}
		if x.ExternalAccountID == "" {
			x.ExternalAccountID = fmt.Sprintf("sandbox-%d", time.Now().UnixNano())
		}
		if x.ProviderMode == "" {
			x.ProviderMode = "sandbox"
		}
		status := "connected"
		enc, cfgJSON := "", "{}"
		if x.ProviderMode == "official" {
			status = "pending_oauth"
			if x.Platform == "telegram" {
				if strings.TrimSpace(x.Token) == "" || strings.TrimSpace(x.ChatID) == "" {
					http.Error(w, "Telegram requiere token del bot y ID/@canal", 400)
					return
				}
				tok, _ := json.Marshal(socialToken{AccessToken: strings.TrimSpace(x.Token)})
				cfg, _ := json.Marshal(socialProviderConfig{ChatID: strings.TrimSpace(x.ChatID)})
				enc, cfgJSON, status = encryptLocal(string(tok), a.cfg.ChannelEncryptionKey), string(cfg), "connected"
				x.ExternalAccountID = strings.TrimSpace(x.ChatID)
				x.Scopes = "sendMessage,sendPhoto,sendVideo,sendMediaGroup"
			}
		}
		now := time.Now().UTC().Format(time.RFC3339)
		ensureSocialProviderSchema(a.db)
		res, err := a.db.Exec(`INSERT INTO social_connections(tenant_id,platform,account_name,external_account_id,status,provider_mode,scopes,last_sync_at,last_error,created_at,updated_at,encrypted_credentials,config_json) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, t, x.Platform, x.AccountName, x.ExternalAccountID, status, x.ProviderMode, x.Scopes, "", "", now, now, enc, cfgJSON)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		id, _ := res.LastInsertId()
		writeJSON(w, map[string]any{"id": id, "status": status})
	case "DELETE":
		if !perms.ManageConnections {
			http.Error(w, "Solo propietarios y administradores pueden eliminar conexiones", 403)
			return
		}
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		res, err := a.db.Exec(`DELETE FROM social_connections WHERE id=? AND tenant_id=?`, id, t)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		n, _ := res.RowsAffected()
		writeJSON(w, map[string]any{"ok": n > 0})
	default:
		http.Error(w, "Método no permitido", 405)
	}
}

func (a *App) socialPostsHandler(w http.ResponseWriter, r *http.Request) {
	t, u, err := a.tenantFor(r)
	if err != nil {
		http.Error(w, err.Error(), 401)
		return
	}
	perms := socialPermissionsFor(u)
	if !perms.View {
		http.Error(w, "No tienes acceso al Social Hub", 403)
		return
	}
	switch r.Method {
	case "GET":
		status := strings.TrimSpace(r.URL.Query().Get("status"))
		q := `SELECT p.id,p.tenant_id,p.group_id,p.campaign_id,p.connection_id,p.platform,p.format,p.title,p.caption,p.cta,p.link_url,p.media_json,p.scheduled_at,p.published_at,p.status,p.external_post_id,p.published_url,p.error_message,p.created_at,p.updated_at,COALESCE(g.name,''),COALESCE(g.master_content,''),COALESCE(g.objective,'') FROM social_posts p LEFT JOIN social_post_groups g ON g.id=p.group_id AND g.tenant_id=p.tenant_id WHERE p.tenant_id=?`
		args := []any{t}
		if status != "" {
			q += " AND p.status=?"
			args = append(args, status)
		}
		q += " ORDER BY CASE WHEN p.scheduled_at='' THEN p.created_at ELSE p.scheduled_at END DESC LIMIT 500"
		rows, err := a.db.Query(q, args...)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		defer rows.Close()
		out := []SocialPost{}
		for rows.Next() {
			var x SocialPost
			_ = rows.Scan(&x.ID, &x.TenantID, &x.GroupID, &x.CampaignID, &x.ConnectionID, &x.Platform, &x.Format, &x.Title, &x.Caption, &x.CTA, &x.LinkURL, &x.MediaJSON, &x.ScheduledAt, &x.PublishedAt, &x.Status, &x.ExternalPostID, &x.PublishedURL, &x.ErrorMessage, &x.CreatedAt, &x.UpdatedAt, &x.GroupName, &x.MasterContent, &x.Objective)
			out = append(out, x)
		}
		writeJSON(w, out)
	case "POST":
		if !perms.CreateDraft {
			http.Error(w, "No tienes permiso para crear contenido", 403)
			return
		}
		var req struct {
			Name, Objective, MasterContent, Format, CTA, LinkURL, MediaURL, ScheduledAt, Action string
			CampaignID                                                                          int64
			Platforms                                                                           []string
			MediaURLs                                                                           []string `json:"mediaURLs"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil || strings.TrimSpace(req.MasterContent) == "" || len(req.Platforms) == 0 {
			http.Error(w, "contenido y redes son obligatorios", 400)
			return
		}
		req.Action = strings.ToLower(strings.TrimSpace(req.Action))
		req.ScheduledAt = strings.TrimSpace(req.ScheduledAt)
		if req.ScheduledAt != "" && req.Action != "publish" {
			req.Action = "schedule"
		}
		if req.Action == "schedule" {
			if req.ScheduledAt == "" {
				http.Error(w, "fecha de programación requerida", 400)
				return
			}
			if _, err := time.Parse(time.RFC3339, req.ScheduledAt); err != nil {
				http.Error(w, "fecha de programación inválida", 400)
				return
			}
		}
		decision := a.socialGovernanceDecisionV231(t, req.CampaignID, req.Platforms, req.MasterContent, req.Action)
		if decision.Blocked && (req.Action == "publish" || req.Action == "schedule") {
			a.auditSocialGovernanceV231(t, 0, req.CampaignID, u.ID, req.Action, req.Platforms, decision)
			http.Error(w, decision.Reason, http.StatusConflict)
			return
		}
		status := "draft"
		if req.Action == "schedule" {
			if decision.RequireApproval {
				status = "pending_approval"
			} else {
				if !perms.Schedule {
					http.Error(w, "Tu rol solo puede guardar borradores", 403)
					return
				}
				status = "scheduled"
			}
		}
		if req.Action == "publish" {
			if decision.RequireApproval {
				status = "pending_approval"
			} else {
				if !perms.Publish {
					http.Error(w, "No tienes permiso para publicar", 403)
					return
				}
				status = "queued"
			}
		}
		now := time.Now().UTC().Format(time.RFC3339)
		tx, err := a.db.Begin()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		defer tx.Rollback()
		res, err := tx.Exec(`INSERT INTO social_post_groups(tenant_id,name,master_content,objective,status,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`, t, req.Name, req.MasterContent, req.Objective, status, now, now)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		gid, _ := res.LastInsertId()
		ids := []int64{}
		for _, p := range req.Platforms {
			p = strings.ToLower(strings.TrimSpace(p))
			if !validSocialPlatform(p) {
				continue
			}
			var cid int64
			_ = tx.QueryRow(`SELECT id FROM social_connections WHERE tenant_id=? AND platform=? AND status='connected' ORDER BY id LIMIT 1`, t, p).Scan(&cid)
			caption := adaptSocialCaption(p, req.MasterContent, req.CTA)
			rr, e := tx.Exec(`INSERT INTO social_posts(tenant_id,group_id,campaign_id,connection_id,platform,format,title,caption,cta,link_url,media_json,scheduled_at,status,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, t, gid, req.CampaignID, cid, p, req.Format, req.Name, caption, req.CTA, req.LinkURL, toMediaJSONMany(req.MediaURLs, req.MediaURL), req.ScheduledAt, status, now, now)
			if e != nil {
				http.Error(w, e.Error(), 500)
				return
			}
			id, _ := rr.LastInsertId()
			ids = append(ids, id)
		}
		if len(ids) == 0 {
			http.Error(w, "no se encontró una red válida", 400)
			return
		}
		if decision.RequireApproval && (req.Action == "publish" || req.Action == "schedule") {
			_, err = tx.Exec(`INSERT INTO social_approvals(tenant_id,group_id,requested_by,desired_action,desired_scheduled_at,request_note,status,requested_at) VALUES(?,?,?,?,?,?,?,?)`, t, gid, u.ID, req.Action, req.ScheduledAt, "Generado automáticamente por la política de gobernanza V23.1", "pending", now)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
		}
		if err = tx.Commit(); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		a.auditSocialGovernanceV231(t, gid, req.CampaignID, u.ID, req.Action, req.Platforms, decision)
		writeJSON(w, map[string]any{"group_id": gid, "post_ids": ids, "status": status, "governance": decision})
	case "PUT":
		if !perms.EditContent {
			http.Error(w, "No tienes permiso para editar contenido", 403)
			return
		}
		var req struct {
			GroupID                                                                             int64 `json:"group_id"`
			Name, Objective, MasterContent, Format, CTA, LinkURL, MediaURL, ScheduledAt, Action string
			CampaignID                                                                          int64
			Platforms                                                                           []string
			MediaURLs                                                                           []string `json:"mediaURLs"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil || req.GroupID == 0 || strings.TrimSpace(req.MasterContent) == "" || len(req.Platforms) == 0 {
			http.Error(w, "grupo, contenido y redes son obligatorios", 400)
			return
		}
		req.Action = strings.ToLower(strings.TrimSpace(req.Action))
		req.ScheduledAt = strings.TrimSpace(req.ScheduledAt)
		if req.ScheduledAt != "" && req.Action != "publish" {
			req.Action = "schedule"
		}
		if req.Action == "schedule" {
			if req.ScheduledAt == "" {
				http.Error(w, "fecha de programación requerida", 400)
				return
			}
			if _, err := time.Parse(time.RFC3339, req.ScheduledAt); err != nil {
				http.Error(w, "fecha de programación inválida", 400)
				return
			}
		}
		decision := a.socialGovernanceDecisionV231(t, req.CampaignID, req.Platforms, req.MasterContent, req.Action)
		if decision.Blocked && (req.Action == "publish" || req.Action == "schedule") {
			a.auditSocialGovernanceV231(t, req.GroupID, req.CampaignID, u.ID, req.Action, req.Platforms, decision)
			http.Error(w, decision.Reason, http.StatusConflict)
			return
		}
		status := "draft"
		if req.Action == "schedule" {
			if decision.RequireApproval {
				status = "pending_approval"
			} else {
				if !perms.Schedule {
					http.Error(w, "Tu rol solo puede guardar borradores", 403)
					return
				}
				status = "scheduled"
			}
		}
		if req.Action == "publish" {
			if decision.RequireApproval {
				status = "pending_approval"
			} else {
				if !perms.Publish {
					http.Error(w, "No tienes permiso para publicar", 403)
					return
				}
				status = "queued"
			}
		}
		now := time.Now().UTC().Format(time.RFC3339)
		tx, err := a.db.Begin()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		defer tx.Rollback()
		res, err := tx.Exec(`UPDATE social_post_groups SET name=?,master_content=?,objective=?,status=?,updated_at=? WHERE id=? AND tenant_id=?`, req.Name, req.MasterContent, req.Objective, status, now, req.GroupID, t)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			http.Error(w, "publicación no encontrada", 404)
			return
		}
		rows, err := tx.Query(`SELECT id,platform,status FROM social_posts WHERE tenant_id=? AND group_id=?`, t, req.GroupID)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		existing := map[string]int64{}
		for rows.Next() {
			var id int64
			var p, st string
			_ = rows.Scan(&id, &p, &st)
			if st != "published" {
				existing[p] = id
			}
		}
		rows.Close()
		wanted := map[string]bool{}
		for _, p := range req.Platforms {
			p = strings.ToLower(strings.TrimSpace(p))
			if !validSocialPlatform(p) {
				continue
			}
			wanted[p] = true
			var cid int64
			_ = tx.QueryRow(`SELECT id FROM social_connections WHERE tenant_id=? AND platform=? AND status='connected' ORDER BY id LIMIT 1`, t, p).Scan(&cid)
			caption := adaptSocialCaption(p, req.MasterContent, req.CTA)
			if id, ok := existing[p]; ok {
				_, err = tx.Exec(`UPDATE social_posts SET campaign_id=?,connection_id=?,format=?,title=?,caption=?,cta=?,link_url=?,media_json=?,scheduled_at=?,status=?,error_message='',updated_at=? WHERE id=? AND tenant_id=?`, req.CampaignID, cid, req.Format, req.Name, caption, req.CTA, req.LinkURL, toMediaJSONMany(req.MediaURLs, req.MediaURL), req.ScheduledAt, status, now, id, t)
			} else {
				_, err = tx.Exec(`INSERT INTO social_posts(tenant_id,group_id,campaign_id,connection_id,platform,format,title,caption,cta,link_url,media_json,scheduled_at,status,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, t, req.GroupID, req.CampaignID, cid, p, req.Format, req.Name, caption, req.CTA, req.LinkURL, toMediaJSONMany(req.MediaURLs, req.MediaURL), req.ScheduledAt, status, now, now)
			}
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
		}
		for p, id := range existing {
			if !wanted[p] {
				_, err = tx.Exec(`DELETE FROM social_posts WHERE id=? AND tenant_id=? AND status<>'published'`, id, t)
				if err != nil {
					http.Error(w, err.Error(), 500)
					return
				}
			}
		}
		if decision.RequireApproval && (req.Action == "publish" || req.Action == "schedule") {
			_, _ = tx.Exec(`UPDATE social_approvals SET status='superseded',reviewed_at=? WHERE tenant_id=? AND group_id=? AND status='pending'`, now, t, req.GroupID)
			_, err = tx.Exec(`INSERT INTO social_approvals(tenant_id,group_id,requested_by,desired_action,desired_scheduled_at,request_note,status,requested_at) VALUES(?,?,?,?,?,?,?,?)`, t, req.GroupID, u.ID, req.Action, req.ScheduledAt, "Generado automáticamente por la política de gobernanza V23.1", "pending", now)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
		}
		if err = tx.Commit(); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		a.auditSocialGovernanceV231(t, req.GroupID, req.CampaignID, u.ID, req.Action, req.Platforms, decision)
		writeJSON(w, map[string]any{"ok": true, "group_id": req.GroupID, "status": status, "governance": decision})
	case "DELETE":
		if !perms.DeleteContent {
			http.Error(w, "No tienes permiso para eliminar publicaciones", 403)
			return
		}
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		_, err := a.db.Exec(`DELETE FROM social_posts WHERE id=? AND tenant_id=?`, id, t)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	default:
		http.Error(w, "Método no permitido", 405)
	}
}

func toMediaJSON(url string) string { return toMediaJSONMany(nil, url) }
func toMediaJSONMany(urls []string, fallback string) string {
	seen := map[string]bool{}
	out := []map[string]string{}
	for _, raw := range append(urls, fallback) {
		u := strings.TrimSpace(raw)
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		typ := "image"
		l := strings.ToLower(strings.Split(u, "?")[0])
		if strings.HasSuffix(l, ".mp4") || strings.HasSuffix(l, ".mov") || strings.HasSuffix(l, ".webm") {
			typ = "video"
		}
		out = append(out, map[string]string{"url": u, "type": typ})
		if len(out) >= 10 {
			break
		}
	}
	b, _ := json.Marshal(out)
	return string(b)
}
func adaptSocialCaption(platform, body, cta string) string {
	body = strings.TrimSpace(body)
	cta = strings.TrimSpace(cta)
	switch platform {
	case "linkedin":
		body = "Una mirada empresarial:\n\n" + body
	case "instagram":
		body = body + "\n\n#WorkticAI #Ventas #Automatización #InteligenciaArtificial"
	case "telegram":
		body = "📣 " + body
	case "tiktok":
		body = "🎬 " + body + "\n\n#ParaTi #Negocios"
	case "youtube":
		body = "▶️ " + body
	}
	if cta != "" {
		body += "\n\n" + cta
	}
	return body
}

func (a *App) socialPublishHandler(w http.ResponseWriter, r *http.Request) {
	t, u, err := a.tenantFor(r)
	if err != nil {
		http.Error(w, err.Error(), 401)
		return
	}
	if !socialPermissionsFor(u).Publish {
		http.Error(w, "No tienes permiso para publicar contenido", 403)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "Método no permitido", 405)
		return
	}
	var req struct {
		ID int64 `json:"id"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.ID == 0 {
		http.Error(w, "id requerido", 400)
		return
	}
	var groupID, campaignID int64
	var status, master string
	if err := a.db.QueryRow(`SELECT p.group_id,p.campaign_id,p.status,COALESCE(g.master_content,p.caption) FROM social_posts p LEFT JOIN social_post_groups g ON g.id=p.group_id AND g.tenant_id=p.tenant_id WHERE p.id=? AND p.tenant_id=?`, req.ID, t).Scan(&groupID, &campaignID, &status, &master); err != nil {
		http.Error(w, "publicación no encontrada", 404)
		return
	}
	platforms := []string{}
	rows, _ := a.db.Query(`SELECT DISTINCT platform FROM social_posts WHERE tenant_id=? AND group_id=?`, t, groupID)
	if rows != nil {
		for rows.Next() {
			var p string
			_ = rows.Scan(&p)
			if validSocialPlatform(p) {
				platforms = append(platforms, p)
			}
		}
		rows.Close()
	}
	decision := a.socialGovernanceDecisionV231(t, campaignID, platforms, master, "publish")
	if decision.Blocked {
		a.auditSocialGovernanceV231(t, groupID, campaignID, u.ID, "publish", platforms, decision)
		http.Error(w, decision.Reason, http.StatusConflict)
		return
	}
	if status == "pending_approval" || decision.RequireApproval {
		now := time.Now().UTC().Format(time.RFC3339)
		tx, e := a.db.Begin()
		if e != nil {
			http.Error(w, e.Error(), 500)
			return
		}
		defer tx.Rollback()
		_, e = tx.Exec(`UPDATE social_post_groups SET status='pending_approval',updated_at=? WHERE id=? AND tenant_id=?`, now, groupID, t)
		if e == nil {
			_, e = tx.Exec(`UPDATE social_posts SET status='pending_approval',updated_at=? WHERE group_id=? AND tenant_id=? AND status<>'published'`, now, groupID, t)
		}
		var n int
		if e == nil {
			_ = tx.QueryRow(`SELECT COUNT(*) FROM social_approvals WHERE tenant_id=? AND group_id=? AND status='pending'`, t, groupID).Scan(&n)
			if n == 0 {
				_, e = tx.Exec(`INSERT INTO social_approvals(tenant_id,group_id,requested_by,desired_action,desired_scheduled_at,request_note,status,requested_at) VALUES(?,?,?,?,?,?,?,?)`, t, groupID, u.ID, "publish", "", "Generado por la política de gobernanza V23.1", "pending", now)
			}
		}
		if e != nil {
			http.Error(w, e.Error(), 500)
			return
		}
		_ = tx.Commit()
		a.auditSocialGovernanceV231(t, groupID, campaignID, u.ID, "publish", platforms, decision)
		writeJSON(w, map[string]any{"ok": true, "status": "pending_approval", "governance": decision})
		return
	}
	result, err := a.publishSocialPost(r.Context(), t, req.ID)
	if err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "status": "published", "result": result, "governance": decision})
}

func (a *App) socialOverviewHandler(w http.ResponseWriter, r *http.Request) {
	t, u, err := a.tenantFor(r)
	if err != nil {
		http.Error(w, err.Error(), 401)
		return
	}
	out := map[string]any{"permissions": socialPermissionsFor(u), "tenant_id": t}
	for _, q := range []struct{ k, q string }{{"connections", `SELECT COUNT(*) FROM social_connections WHERE tenant_id=? AND status='connected'`}, {"scheduled", `SELECT COUNT(*) FROM social_posts WHERE tenant_id=? AND status IN ('scheduled','queued')`}, {"published", `SELECT COUNT(*) FROM social_posts WHERE tenant_id=? AND status='published'`}, {"failed", `SELECT COUNT(*) FROM social_posts WHERE tenant_id=? AND status='failed'`}} {
		var n int
		_ = a.db.QueryRow(q.q, t).Scan(&n)
		out[q.k] = n
	}
	rows, _ := a.db.Query(`SELECT platform,COUNT(*) FROM social_posts WHERE tenant_id=? GROUP BY platform`, t)
	by := map[string]int{}
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var p string
			var n int
			_ = rows.Scan(&p, &n)
			by[p] = n
		}
	}
	out["by_platform"] = by
	writeJSON(w, out)
}
