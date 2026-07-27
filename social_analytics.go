package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type SocialPermissions struct {
	View              bool `json:"view"`
	CreateDraft       bool `json:"create_draft"`
	EditContent       bool `json:"edit_content"`
	Schedule          bool `json:"schedule"`
	Publish           bool `json:"publish"`
	ManageConnections bool `json:"manage_connections"`
	DeleteContent     bool `json:"delete_content"`
	ViewAnalytics     bool `json:"view_analytics"`
}

func normalizeSocialRole(role string) string {
	r := strings.ToLower(strings.TrimSpace(role))
	switch r {
	case "owner", "propietario", "root", "superadmin":
		return "owner"
	case "admin", "administrator", "administrador":
		return "admin"
	case "supervisor", "manager":
		return "supervisor"
	case "advisor", "agent", "asesor", "editor":
		return "advisor"
	default:
		return r
	}
}

func socialPermissionsFor(u *User) SocialPermissions {
	if u == nil {
		return SocialPermissions{}
	}
	switch normalizeSocialRole(u.Role) {
	case "owner", "admin":
		return SocialPermissions{true, true, true, true, true, true, true, true}
	case "supervisor":
		return SocialPermissions{true, true, true, true, true, false, true, true}
	case "advisor":
		return SocialPermissions{true, true, true, false, false, false, false, true}
	default:
		return SocialPermissions{View: true, ViewAnalytics: true}
	}
}

func (a *App) requireSocialPermission(r *http.Request, check func(SocialPermissions) bool) (int64, *User, bool) {
	tid, u, err := a.tenantFor(r)
	if err != nil {
		return 0, nil, false
	}
	return tid, u, check(socialPermissionsFor(u))
}

func parseSocialRange(r *http.Request) (string, string) {
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

func (a *App) socialAnalyticsHandler(w http.ResponseWriter, r *http.Request) {
	tid, u, ok := a.requireSocialPermission(r, func(p SocialPermissions) bool { return p.ViewAnalytics })
	if !ok {
		http.Error(w, "No tienes permiso para ver la analítica social", 403)
		return
	}
	from, to := parseSocialRange(r)
	platform := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("platform")))
	metricWhere := "tenant_id=? AND metric_date>=? AND metric_date<=?"
	args := []any{tid, from, to}
	postWhere := "p.tenant_id=? AND substr(CASE WHEN p.published_at='' THEN p.created_at ELSE p.published_at END,1,10)>=? AND substr(CASE WHEN p.published_at='' THEN p.created_at ELSE p.published_at END,1,10)<=?"
	postArgs := []any{tid, from, to}
	if validSocialPlatform(platform) {
		metricWhere += " AND post_id IN (SELECT id FROM social_posts WHERE tenant_id=? AND platform=?)"
		args = append(args, tid, platform)
		postWhere += " AND p.platform=?"
		postArgs = append(postArgs, platform)
	}
	out := map[string]any{"from": from, "to": to, "platform": platform, "permissions": socialPermissionsFor(u)}
	kpi := map[string]any{}
	var impressions, reach, likes, comments, shares, clicks, views, leads, conversions int64
	_ = a.db.QueryRow(`SELECT COALESCE(SUM(impressions),0),COALESCE(SUM(reach),0),COALESCE(SUM(likes),0),COALESCE(SUM(comments),0),COALESCE(SUM(shares),0),COALESCE(SUM(clicks),0),COALESCE(SUM(video_views),0),COALESCE(SUM(leads),0),COALESCE(SUM(conversions),0) FROM social_metrics WHERE `+metricWhere, args...).Scan(&impressions, &reach, &likes, &comments, &shares, &clicks, &views, &leads, &conversions)
	var posts, published, failed int64
	_ = a.db.QueryRow(`SELECT COUNT(*),COALESCE(SUM(CASE WHEN p.status='published' THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN p.status='failed' THEN 1 ELSE 0 END),0) FROM social_posts p WHERE `+postWhere, postArgs...).Scan(&posts, &published, &failed)
	interactions := likes + comments + shares
	engagement := 0.0
	ctr := 0.0
	conversionRate := 0.0
	if reach > 0 {
		engagement = float64(interactions) / float64(reach) * 100
	}
	if impressions > 0 {
		ctr = float64(clicks) / float64(impressions) * 100
	}
	if clicks > 0 {
		conversionRate = float64(conversions) / float64(clicks) * 100
	}
	kpi["posts"] = posts
	kpi["published"] = published
	kpi["failed"] = failed
	kpi["impressions"] = impressions
	kpi["reach"] = reach
	kpi["interactions"] = interactions
	kpi["likes"] = likes
	kpi["comments"] = comments
	kpi["shares"] = shares
	kpi["clicks"] = clicks
	kpi["video_views"] = views
	kpi["leads"] = leads
	kpi["conversions"] = conversions
	kpi["engagement_rate"] = engagement
	kpi["ctr"] = ctr
	kpi["conversion_rate"] = conversionRate
	out["kpis"] = kpi

	series := []map[string]any{}
	rows, _ := a.db.Query(`SELECT metric_date,COALESCE(SUM(impressions),0),COALESCE(SUM(reach),0),COALESCE(SUM(likes+comments+shares),0),COALESCE(SUM(clicks),0),COALESCE(SUM(leads),0),COALESCE(SUM(conversions),0) FROM social_metrics WHERE `+metricWhere+` GROUP BY metric_date ORDER BY metric_date`, args...)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var d string
			var a1, a2, a3, a4, a5, a6 int64
			_ = rows.Scan(&d, &a1, &a2, &a3, &a4, &a5, &a6)
			series = append(series, map[string]any{"date": d, "impressions": a1, "reach": a2, "interactions": a3, "clicks": a4, "leads": a5, "conversions": a6})
		}
	}
	out["series"] = series

	byPlatform := []map[string]any{}
	bp, _ := a.db.Query(`SELECT p.platform,COUNT(DISTINCT p.id),COALESCE(SUM(m.impressions),0),COALESCE(SUM(m.reach),0),COALESCE(SUM(m.likes+m.comments+m.shares),0),COALESCE(SUM(m.clicks),0),COALESCE(SUM(m.leads),0),COALESCE(SUM(m.conversions),0) FROM social_posts p LEFT JOIN social_metrics m ON m.post_id=p.id AND m.tenant_id=p.tenant_id AND m.metric_date>=? AND m.metric_date<=? WHERE p.tenant_id=? GROUP BY p.platform ORDER BY COALESCE(SUM(m.reach),0) DESC`, from, to, tid)
	if bp != nil {
		defer bp.Close()
		for bp.Next() {
			var p string
			var n, im, re, it, cl, le, co int64
			_ = bp.Scan(&p, &n, &im, &re, &it, &cl, &le, &co)
			er := 0.0
			if re > 0 {
				er = float64(it) / float64(re) * 100
			}
			byPlatform = append(byPlatform, map[string]any{"platform": p, "posts": n, "impressions": im, "reach": re, "interactions": it, "clicks": cl, "leads": le, "conversions": co, "engagement_rate": er})
		}
	}
	out["by_platform"] = byPlatform

	top := []map[string]any{}
	tp, _ := a.db.Query(`SELECT p.id,p.platform,p.title,p.caption,p.published_url,COALESCE(SUM(m.reach),0),COALESCE(SUM(m.likes+m.comments+m.shares),0),COALESCE(SUM(m.clicks),0),COALESCE(SUM(m.leads),0),COALESCE(SUM(m.conversions),0) FROM social_posts p LEFT JOIN social_metrics m ON m.post_id=p.id AND m.tenant_id=p.tenant_id AND m.metric_date>=? AND m.metric_date<=? WHERE p.tenant_id=? AND p.status='published' GROUP BY p.id ORDER BY COALESCE(SUM(m.reach),0) DESC,COALESCE(SUM(m.likes+m.comments+m.shares),0) DESC LIMIT 10`, from, to, tid)
	if tp != nil {
		defer tp.Close()
		for tp.Next() {
			var id, re, it, cl, le, co int64
			var p, t, c, url string
			_ = tp.Scan(&id, &p, &t, &c, &url, &re, &it, &cl, &le, &co)
			top = append(top, map[string]any{"id": id, "platform": p, "title": t, "caption": c, "published_url": url, "reach": re, "interactions": it, "clicks": cl, "leads": le, "conversions": co})
		}
	}
	out["top_posts"] = top
	out["funnel"] = map[string]any{"impressions": impressions, "reach": reach, "interactions": interactions, "clicks": clicks, "leads": leads, "conversions": conversions}
	writeJSON(w, out)
}

// Endpoint seguro para almacenar métricas obtenidas por los conectores oficiales.
// Se usa internamente por los jobs de sincronización y también facilita pruebas controladas.
func (a *App) socialMetricsIngestHandler(w http.ResponseWriter, r *http.Request) {
	tid, _, ok := a.requireSocialPermission(r, func(p SocialPermissions) bool { return p.ManageConnections })
	if !ok {
		http.Error(w, "No tienes permiso para sincronizar métricas", 403)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	var q struct {
		PostID                                                                              int64  `json:"post_id"`
		MetricDate                                                                          string `json:"metric_date"`
		Impressions, Reach, Likes, Comments, Shares, Clicks, VideoViews, Leads, Conversions int64
	}
	if json.NewDecoder(r.Body).Decode(&q) != nil || q.PostID == 0 {
		http.Error(w, "datos inválidos", 400)
		return
	}
	if q.MetricDate == "" {
		q.MetricDate = time.Now().UTC().Format("2006-01-02")
	}
	var exists int
	if a.db.QueryRow(`SELECT COUNT(*) FROM social_posts WHERE id=? AND tenant_id=?`, q.PostID, tid).Scan(&exists) != nil || exists == 0 {
		http.Error(w, "publicación no encontrada", 404)
		return
	}
	_, err := a.db.Exec(`INSERT INTO social_metrics(tenant_id,post_id,metric_date,impressions,reach,likes,comments,shares,clicks,video_views,leads,conversions) VALUES(?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(tenant_id,post_id,metric_date) DO UPDATE SET impressions=excluded.impressions,reach=excluded.reach,likes=excluded.likes,comments=excluded.comments,shares=excluded.shares,clicks=excluded.clicks,video_views=excluded.video_views,leads=excluded.leads,conversions=excluded.conversions`, tid, q.PostID, q.MetricDate, q.Impressions, q.Reach, q.Likes, q.Comments, q.Shares, q.Clicks, q.VideoViews, q.Leads, q.Conversions)
	if err != nil {
		http.Error(w, fmt.Sprintf("no se pudieron guardar métricas: %v", err), 500)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}
