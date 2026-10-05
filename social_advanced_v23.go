package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func initSocialAdvancedV23Schema(db *DB) error {
	qs := []string{
		`CREATE TABLE IF NOT EXISTS social_media_assets(
            id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL,
            name TEXT NOT NULL DEFAULT '', url TEXT NOT NULL, media_type TEXT NOT NULL DEFAULT 'image',
            mime TEXT NOT NULL DEFAULT '', size_bytes INTEGER NOT NULL DEFAULT 0,
            source TEXT NOT NULL DEFAULT 'upload', tags TEXT NOT NULL DEFAULT '', created_by INTEGER NOT NULL DEFAULT 0,
            created_at TEXT NOT NULL, UNIQUE(tenant_id,url));`,
		`CREATE INDEX IF NOT EXISTS idx_social_media_assets_tenant ON social_media_assets(tenant_id,created_at);`,
		`CREATE TABLE IF NOT EXISTS social_approvals(
            id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL, group_id INTEGER NOT NULL,
            requested_by INTEGER NOT NULL DEFAULT 0, desired_action TEXT NOT NULL DEFAULT 'draft',
            desired_scheduled_at TEXT NOT NULL DEFAULT '', request_note TEXT NOT NULL DEFAULT '',
            status TEXT NOT NULL DEFAULT 'pending', reviewed_by INTEGER NOT NULL DEFAULT 0,
            review_note TEXT NOT NULL DEFAULT '', requested_at TEXT NOT NULL, reviewed_at TEXT NOT NULL DEFAULT '');`,
		`CREATE INDEX IF NOT EXISTS idx_social_approvals_tenant_status ON social_approvals(tenant_id,status,requested_at);`,
		`CREATE TABLE IF NOT EXISTS social_comments_v23(
            id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL, post_id INTEGER NOT NULL DEFAULT 0,
            connection_id INTEGER NOT NULL DEFAULT 0, platform TEXT NOT NULL, external_comment_id TEXT NOT NULL,
            parent_external_id TEXT NOT NULL DEFAULT '', author_name TEXT NOT NULL DEFAULT '', author_external_id TEXT NOT NULL DEFAULT '',
            body TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'open', response_text TEXT NOT NULL DEFAULT '',
            external_reply_id TEXT NOT NULL DEFAULT '', provider_created_at TEXT NOT NULL DEFAULT '', synced_at TEXT NOT NULL,
            UNIQUE(tenant_id,platform,external_comment_id));`,
		`CREATE INDEX IF NOT EXISTS idx_social_comments_v23_tenant ON social_comments_v23(tenant_id,status,synced_at);`,
		`CREATE TABLE IF NOT EXISTS social_repurpose_jobs_v23(
            id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL, source_group_id INTEGER NOT NULL DEFAULT 0,
            requested_by INTEGER NOT NULL DEFAULT 0, target_platforms TEXT NOT NULL DEFAULT '[]', target_formats TEXT NOT NULL DEFAULT '[]',
            status TEXT NOT NULL DEFAULT 'completed', result_json TEXT NOT NULL DEFAULT '{}', created_at TEXT NOT NULL);`,
	}
	for _, q := range qs {
		if _, err := db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) syncSocialAssetFolder(tid int64) {
	dir := filepath.Join(a.cfg.DataDir, "social_uploads", strconv.FormatInt(tid, 10))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ext := strings.ToLower(filepath.Ext(name))
		mediaType := "image"
		if ext == ".mp4" || ext == ".mov" || ext == ".webm" {
			mediaType = "video"
		}
		info, _ := e.Info()
		var size int64
		if info != nil {
			size = info.Size()
		}
		u := "/uploads/social/" + strconv.FormatInt(tid, 10) + "/" + name
		_, _ = a.db.Exec(`INSERT OR IGNORE INTO social_media_assets(tenant_id,name,url,media_type,size_bytes,source,created_at) VALUES(?,?,?,?,?,?,?)`, tid, name, u, mediaType, size, "upload", now)
	}
}

func (a *App) socialMediaLibraryV23Handler(w http.ResponseWriter, r *http.Request) {
	tid, u, err := a.tenantFor(r)
	if err != nil {
		http.Error(w, err.Error(), 401)
		return
	}
	p := socialPermissionsFor(u)
	if !p.View {
		http.Error(w, "No tienes acceso al Social Hub", 403)
		return
	}
	switch r.Method {
	case http.MethodGet:
		a.syncSocialAssetFolder(tid)
		typ := strings.TrimSpace(r.URL.Query().Get("type"))
		search := strings.TrimSpace(r.URL.Query().Get("q"))
		q := `SELECT id,name,url,media_type,mime,size_bytes,source,tags,created_at FROM social_media_assets WHERE tenant_id=?`
		args := []any{tid}
		if typ != "" {
			q += " AND media_type=?"
			args = append(args, typ)
		}
		if search != "" {
			q += " AND (name LIKE ? OR tags LIKE ?)"
			s := "%" + search + "%"
			args = append(args, s, s)
		}
		q += " ORDER BY id DESC LIMIT 500"
		rows, e := a.db.Query(q, args...)
		if e != nil {
			http.Error(w, e.Error(), 500)
			return
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var id, size int64
			var name, uurl, mt, mime, src, tags, created string
			_ = rows.Scan(&id, &name, &uurl, &mt, &mime, &size, &src, &tags, &created)
			out = append(out, map[string]any{"id": id, "name": name, "url": uurl, "media_type": mt, "mime": mime, "size_bytes": size, "source": src, "tags": tags, "created_at": created})
		}
		writeJSON(w, out)
	case http.MethodPost:
		if !p.CreateDraft {
			http.Error(w, "No tienes permiso para agregar recursos", 403)
			return
		}
		var q struct{ Name, URL, MediaType, Tags string }
		if json.NewDecoder(r.Body).Decode(&q) != nil {
			http.Error(w, "datos inválidos", 400)
			return
		}
		q.URL = strings.TrimSpace(q.URL)
		if q.URL == "" {
			http.Error(w, "URL requerida", 400)
			return
		}
		if q.MediaType != "video" {
			q.MediaType = "image"
		}
		if q.Name == "" {
			q.Name = filepath.Base(strings.Split(q.URL, "?")[0])
		}
		now := time.Now().UTC().Format(time.RFC3339)
		res, e := a.db.Exec(`INSERT OR IGNORE INTO social_media_assets(tenant_id,name,url,media_type,source,tags,created_by,created_at) VALUES(?,?,?,?,?,?,?,?)`, tid, q.Name, q.URL, q.MediaType, "external", q.Tags, u.ID, now)
		if e != nil {
			http.Error(w, e.Error(), 500)
			return
		}
		id, _ := res.LastInsertId()
		writeJSON(w, map[string]any{"ok": true, "id": id})
	case http.MethodDelete:
		if !p.DeleteContent {
			http.Error(w, "No tienes permiso para eliminar recursos", 403)
			return
		}
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		_, e := a.db.Exec(`DELETE FROM social_media_assets WHERE id=? AND tenant_id=?`, id, tid)
		if e != nil {
			http.Error(w, e.Error(), 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	default:
		http.Error(w, "Método no permitido", 405)
	}
}

func (a *App) socialApprovalsV23Handler(w http.ResponseWriter, r *http.Request) {
	tid, u, err := a.tenantFor(r)
	if err != nil {
		http.Error(w, err.Error(), 401)
		return
	}
	p := socialPermissionsFor(u)
	switch r.Method {
	case http.MethodGet:
		if !p.View {
			http.Error(w, "Sin acceso", 403)
			return
		}
		rows, e := a.db.Query(`SELECT a.id,a.group_id,a.desired_action,a.desired_scheduled_at,a.request_note,a.status,a.review_note,a.requested_at,a.reviewed_at,COALESCE(g.name,''),COALESCE(g.master_content,''),COALESCE(ur.name,''),COALESCE(uv.name,'') FROM social_approvals a LEFT JOIN social_post_groups g ON g.id=a.group_id AND g.tenant_id=a.tenant_id LEFT JOIN users ur ON ur.id=a.requested_by LEFT JOIN users uv ON uv.id=a.reviewed_by WHERE a.tenant_id=? ORDER BY a.id DESC LIMIT 200`, tid)
		if e != nil {
			http.Error(w, e.Error(), 500)
			return
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var id, gid int64
			var action, sch, rn, st, rev, reqAt, revAt, name, content, requester, reviewer string
			_ = rows.Scan(&id, &gid, &action, &sch, &rn, &st, &rev, &reqAt, &revAt, &name, &content, &requester, &reviewer)
			out = append(out, map[string]any{"id": id, "group_id": gid, "desired_action": action, "desired_scheduled_at": sch, "request_note": rn, "status": st, "review_note": rev, "requested_at": reqAt, "reviewed_at": revAt, "name": name, "content": content, "requester": requester, "reviewer": reviewer})
		}
		writeJSON(w, out)
	case http.MethodPost:
		var q struct {
			Action                           string `json:"action"`
			GroupID, ApprovalID              int64
			DesiredAction, ScheduledAt, Note string
		}
		if json.NewDecoder(r.Body).Decode(&q) != nil {
			http.Error(w, "datos inválidos", 400)
			return
		}
		now := time.Now().UTC().Format(time.RFC3339)
		if q.Action == "request" {
			if !p.CreateDraft {
				http.Error(w, "No tienes permiso para solicitar aprobación", 403)
				return
			}
			var n int
			_ = a.db.QueryRow(`SELECT COUNT(*) FROM social_post_groups WHERE id=? AND tenant_id=?`, q.GroupID, tid).Scan(&n)
			if n == 0 {
				http.Error(w, "contenido no encontrado", 404)
				return
			}
			if q.DesiredAction != "publish" && q.DesiredAction != "schedule" {
				q.DesiredAction = "draft"
			}
			if q.DesiredAction == "schedule" && strings.TrimSpace(q.ScheduledAt) == "" {
				http.Error(w, "fecha requerida para programar", 400)
				return
			}
			tx, e := a.db.Begin()
			if e != nil {
				http.Error(w, e.Error(), 500)
				return
			}
			defer tx.Rollback()
			res, e := tx.Exec(`INSERT INTO social_approvals(tenant_id,group_id,requested_by,desired_action,desired_scheduled_at,request_note,status,requested_at) VALUES(?,?,?,?,?,?,?,?)`, tid, q.GroupID, u.ID, q.DesiredAction, q.ScheduledAt, q.Note, "pending", now)
			if e == nil {
				_, e = tx.Exec(`UPDATE social_post_groups SET status='pending_approval',updated_at=? WHERE id=? AND tenant_id=?`, now, q.GroupID, tid)
			}
			if e == nil {
				_, e = tx.Exec(`UPDATE social_posts SET status='pending_approval',updated_at=? WHERE group_id=? AND tenant_id=? AND status<>'published'`, now, q.GroupID, tid)
			}
			if e != nil {
				http.Error(w, e.Error(), 500)
				return
			}
			_ = tx.Commit()
			id, _ := res.LastInsertId()
			writeJSON(w, map[string]any{"ok": true, "id": id, "status": "pending"})
			return
		}
		if q.Action == "approve" || q.Action == "reject" {
			if !p.Publish {
				http.Error(w, "No tienes permiso para aprobar publicaciones", 403)
				return
			}
			var gid int64
			var st, desired, scheduled string
			e := a.db.QueryRow(`SELECT group_id,status,desired_action,desired_scheduled_at FROM social_approvals WHERE id=? AND tenant_id=?`, q.ApprovalID, tid).Scan(&gid, &st, &desired, &scheduled)
			if e != nil {
				http.Error(w, "solicitud no encontrada", 404)
				return
			}
			if st != "pending" {
				http.Error(w, "solicitud ya revisada", 409)
				return
			}
			next := "draft"
			reviewStatus := "rejected"
			if q.Action == "approve" {
				reviewStatus = "approved"
				if desired == "publish" {
					next = "queued"
				} else if desired == "schedule" {
					next = "scheduled"
				} else {
					next = "draft"
				}
			}
			tx, e := a.db.Begin()
			if e != nil {
				http.Error(w, e.Error(), 500)
				return
			}
			defer tx.Rollback()
			_, e = tx.Exec(`UPDATE social_approvals SET status=?,reviewed_by=?,review_note=?,reviewed_at=? WHERE id=? AND tenant_id=?`, reviewStatus, u.ID, q.Note, now, q.ApprovalID, tid)
			if e == nil {
				_, e = tx.Exec(`UPDATE social_post_groups SET status=?,updated_at=? WHERE id=? AND tenant_id=?`, next, now, gid, tid)
			}
			if e == nil {
				_, e = tx.Exec(`UPDATE social_posts SET status=?,scheduled_at=CASE WHEN ?='scheduled' THEN ? ELSE scheduled_at END,updated_at=? WHERE group_id=? AND tenant_id=? AND status<>'published'`, next, next, scheduled, now, gid, tid)
			}
			if e != nil {
				http.Error(w, e.Error(), 500)
				return
			}
			_ = tx.Commit()
			writeJSON(w, map[string]any{"ok": true, "status": reviewStatus, "content_status": next})
			return
		}
		http.Error(w, "acción inválida", 400)
	default:
		http.Error(w, "Método no permitido", 405)
	}
}

func (a *App) socialRepurposeV23Handler(w http.ResponseWriter, r *http.Request) {
	tid, u, err := a.tenantFor(r)
	if err != nil {
		http.Error(w, err.Error(), 401)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	if !socialPermissionsFor(u).CreateDraft {
		http.Error(w, "Sin permiso", 403)
		return
	}
	if a.openAIKey() == "" {
		http.Error(w, "OpenAI no está configurado", 503)
		return
	}
	var q struct {
		GroupID            int64    `json:"group_id"`
		Content            string   `json:"content"`
		Platforms, Formats []string `json:"platforms"`
		Objective, Tone    string
	}
	if json.NewDecoder(r.Body).Decode(&q) != nil {
		http.Error(w, "datos inválidos", 400)
		return
	}
	if q.GroupID > 0 && strings.TrimSpace(q.Content) == "" {
		_ = a.db.QueryRow(`SELECT master_content FROM social_post_groups WHERE id=? AND tenant_id=?`, q.GroupID, tid).Scan(&q.Content)
	}
	q.Content = strings.TrimSpace(q.Content)
	if q.Content == "" {
		http.Error(w, "contenido fuente requerido", 400)
		return
	}
	ps := []string{}
	for _, p := range q.Platforms {
		p = strings.ToLower(strings.TrimSpace(p))
		if validSocialPlatform(p) {
			ps = append(ps, p)
		}
	}
	if len(ps) == 0 {
		ps = []string{"facebook", "instagram", "tiktok", "youtube", "linkedin"}
	}
	if len(q.Formats) == 0 {
		q.Formats = []string{"post", "reel", "short"}
	}
	sys := `Eres un estratega senior de social media. Reutiliza el contenido fuente sin inventar métricas, testimonios, resultados ni hechos. Devuelve SOLO JSON válido con la forma {"variants":[{"platform":"instagram","format":"reel","title":"...","content":"...","cta":"...","hashtags":["#..."]}]}. Crea una variante diferenciada por red/formato, breve y lista para revisión humana.`
	user := fmt.Sprintf("Contenido fuente:\n%s\n\nPlataformas: %s\nFormatos: %s\nObjetivo: %s\nTono: %s", q.Content, strings.Join(ps, ","), strings.Join(q.Formats, ","), q.Objective, q.Tone)
	raw, e := a.callOpenAI(sys, user)
	if e != nil {
		http.Error(w, e.Error(), 502)
		return
	}
	raw = strings.TrimSpace(raw)
	if i := strings.Index(raw, "{"); i >= 0 {
		raw = raw[i:]
	}
	if i := strings.LastIndex(raw, "}"); i >= 0 {
		raw = raw[:i+1]
	}
	var parsed map[string]any
	if json.Unmarshal([]byte(raw), &parsed) != nil {
		http.Error(w, "La IA no devolvió JSON válido", 502)
		return
	}
	pb, _ := json.Marshal(ps)
	fb, _ := json.Marshal(q.Formats)
	now := time.Now().UTC().Format(time.RFC3339)
	res, _ := a.db.Exec(`INSERT INTO social_repurpose_jobs_v23(tenant_id,source_group_id,requested_by,target_platforms,target_formats,status,result_json,created_at) VALUES(?,?,?,?,?,?,?,?)`, tid, q.GroupID, u.ID, string(pb), string(fb), "completed", raw, now)
	id, _ := res.LastInsertId()
	parsed["job_id"] = id
	writeJSON(w, parsed)
}

func socialCommentLoadCreds(a *App, tid, postID int64) (SocialPost, socialToken, socialProviderConfig, error) {
	var p SocialPost
	var enc, cfgj, cstatus string
	err := a.db.QueryRow(`SELECT p.id,p.tenant_id,p.connection_id,p.platform,p.external_post_id,COALESCE(c.encrypted_credentials,''),COALESCE(c.config_json,'{}'),COALESCE(c.status,'') FROM social_posts p LEFT JOIN social_connections c ON c.id=p.connection_id AND c.tenant_id=p.tenant_id WHERE p.id=? AND p.tenant_id=?`, postID, tid).Scan(&p.ID, &p.TenantID, &p.ConnectionID, &p.Platform, &p.ExternalPostID, &enc, &cfgj, &cstatus)
	if err != nil {
		return p, socialToken{}, socialProviderConfig{}, err
	}
	if cstatus != "connected" {
		return p, socialToken{}, socialProviderConfig{}, errors.New("cuenta no conectada")
	}
	var tok socialToken
	if json.Unmarshal([]byte(decryptLocal(enc, a.cfg.ChannelEncryptionKey)), &tok) != nil || tok.AccessToken == "" {
		return p, tok, socialProviderConfig{}, errors.New("credenciales inválidas")
	}
	var cfg socialProviderConfig
	_ = json.Unmarshal([]byte(cfgj), &cfg)
	return p, tok, cfg, nil
}

func (a *App) syncSocialCommentsV23(ctx context.Context, tid, postID int64) (int, error) {
	p, tok, _, err := socialCommentLoadCreds(a, tid, postID)
	if err != nil {
		return 0, err
	}
	if p.ExternalPostID == "" {
		return 0, errors.New("la publicación no tiene ID externo")
	}
	if p.Platform == "youtube" {
		tok, err = a.refreshSocialTokenIfNeeded(ctx, p.ConnectionID, tid, p.Platform, tok)
		if err != nil {
			return 0, err
		}
	}
	now := time.Now().UTC().Format(time.RFC3339)
	count := 0
	save := func(ext, parent, author, authorID, body, created string) {
		if strings.TrimSpace(ext) == "" {
			return
		}
		res, _ := a.db.Exec(`INSERT OR IGNORE INTO social_comments_v23(tenant_id,post_id,connection_id,platform,external_comment_id,parent_external_id,author_name,author_external_id,body,provider_created_at,synced_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, tid, p.ID, p.ConnectionID, p.Platform, ext, parent, author, authorID, body, created, now)
		inserted, _ := res.RowsAffected()
		_, _ = a.db.Exec(`UPDATE social_comments_v23 SET body=?,author_name=?,author_external_id=?,provider_created_at=?,synced_at=? WHERE tenant_id=? AND platform=? AND external_comment_id=?`, body, author, authorID, created, now, tid, p.Platform, ext)
		if inserted > 0 {
			_ = a.enqueueAutomationEvent(tid, "social.comment", "social_v23", map[string]any{"post_id": p.ID, "platform": p.Platform, "external_comment_id": ext, "author": author, "body": body})
		}
		count++
	}
	switch p.Platform {
	case "facebook":
		endpoint := "https://graph.facebook.com/" + a.cfg.MetaGraphVersion + "/" + url.PathEscape(p.ExternalPostID) + "/comments?fields=id,from,message,created_time,parent&limit=100&access_token=" + url.QueryEscape(tok.AccessToken)
		resp, e := http.Get(endpoint)
		if e != nil {
			return count, e
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		if resp.StatusCode/100 != 2 {
			return count, fmt.Errorf("Meta comments: %s", b)
		}
		var out struct {
			Data []struct {
				ID      string `json:"id"`
				Message string `json:"message"`
				Created string `json:"created_time"`
				From    struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"from"`
				Parent *struct {
					ID string `json:"id"`
				} `json:"parent"`
			} `json:"data"`
		}
		_ = json.Unmarshal(b, &out)
		for _, c := range out.Data {
			parent := ""
			if c.Parent != nil {
				parent = c.Parent.ID
			}
			save(c.ID, parent, c.From.Name, c.From.ID, c.Message, c.Created)
		}
	case "instagram":
		endpoint := "https://graph.facebook.com/" + a.cfg.MetaGraphVersion + "/" + url.PathEscape(p.ExternalPostID) + "/comments?fields=id,text,username,timestamp&limit=100&access_token=" + url.QueryEscape(tok.AccessToken)
		resp, e := http.Get(endpoint)
		if e != nil {
			return count, e
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		if resp.StatusCode/100 != 2 {
			return count, fmt.Errorf("Instagram comments: %s", b)
		}
		var out struct {
			Data []struct {
				ID        string `json:"id"`
				Text      string `json:"text"`
				Username  string `json:"username"`
				Timestamp string `json:"timestamp"`
			} `json:"data"`
		}
		_ = json.Unmarshal(b, &out)
		for _, c := range out.Data {
			save(c.ID, "", c.Username, "", c.Text, c.Timestamp)
		}
	case "youtube":
		req, _ := http.NewRequestWithContext(ctx, "GET", "https://www.googleapis.com/youtube/v3/commentThreads?part=snippet&maxResults=100&textFormat=plainText&videoId="+url.QueryEscape(p.ExternalPostID), nil)
		req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
		resp, e := http.DefaultClient.Do(req)
		if e != nil {
			return count, e
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		if resp.StatusCode/100 != 2 {
			return count, fmt.Errorf("YouTube comments: %s", b)
		}
		var out struct {
			Items []struct {
				Snippet struct {
					TopLevelComment struct {
						ID      string `json:"id"`
						Snippet struct {
							AuthorDisplayName string `json:"authorDisplayName"`
							AuthorChannelID   string `json:"authorChannelId"`
							TextDisplay       string `json:"textDisplay"`
							PublishedAt       string `json:"publishedAt"`
						} `json:"snippet"`
					} `json:"topLevelComment"`
				} `json:"snippet"`
			} `json:"items"`
		}
		_ = json.Unmarshal(b, &out)
		for _, it := range out.Items {
			c := it.Snippet.TopLevelComment
			save(c.ID, "", c.Snippet.AuthorDisplayName, c.Snippet.AuthorChannelID, c.Snippet.TextDisplay, c.Snippet.PublishedAt)
		}
	default:
		return count, errors.New("sincronización de comentarios disponible en esta versión para Facebook, Instagram y YouTube")
	}
	return count, nil
}

func (a *App) socialCommunityV23Handler(w http.ResponseWriter, r *http.Request) {
	tid, u, err := a.tenantFor(r)
	if err != nil {
		http.Error(w, err.Error(), 401)
		return
	}
	p := socialPermissionsFor(u)
	if !p.View {
		http.Error(w, "Sin acceso", 403)
		return
	}
	switch r.Method {
	case http.MethodGet:
		platform := strings.TrimSpace(r.URL.Query().Get("platform"))
		status := strings.TrimSpace(r.URL.Query().Get("status"))
		q := `SELECT c.id,c.post_id,c.platform,c.external_comment_id,c.author_name,c.body,c.status,c.response_text,c.provider_created_at,c.synced_at,COALESCE(p.title,''),COALESCE(p.published_url,'') FROM social_comments_v23 c LEFT JOIN social_posts p ON p.id=c.post_id AND p.tenant_id=c.tenant_id WHERE c.tenant_id=?`
		args := []any{tid}
		if platform != "" {
			q += " AND c.platform=?"
			args = append(args, platform)
		}
		if status != "" {
			q += " AND c.status=?"
			args = append(args, status)
		}
		q += " ORDER BY c.id DESC LIMIT 300"
		rows, e := a.db.Query(q, args...)
		if e != nil {
			http.Error(w, e.Error(), 500)
			return
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var id, pid int64
			var pl, ext, author, body, st, response, pc, sy, title, pu string
			_ = rows.Scan(&id, &pid, &pl, &ext, &author, &body, &st, &response, &pc, &sy, &title, &pu)
			out = append(out, map[string]any{"id": id, "post_id": pid, "platform": pl, "external_comment_id": ext, "author_name": author, "body": body, "status": st, "response_text": response, "provider_created_at": pc, "synced_at": sy, "post_title": title, "published_url": pu})
		}
		writeJSON(w, out)
	case http.MethodPost:
		var q struct {
			Action     string `json:"action"`
			ID, PostID int64
			Text, Tone string
		}
		if json.NewDecoder(r.Body).Decode(&q) != nil {
			http.Error(w, "datos inválidos", 400)
			return
		}
		if q.Action == "sync" {
			if !p.ViewAnalytics {
				http.Error(w, "Sin permiso", 403)
				return
			}
			if q.PostID > 0 {
				n, e := a.syncSocialCommentsV23(r.Context(), tid, q.PostID)
				if e != nil {
					http.Error(w, e.Error(), 502)
					return
				}
				writeJSON(w, map[string]any{"ok": true, "synced": n})
				return
			}
			rows, _ := a.db.Query(`SELECT id FROM social_posts WHERE tenant_id=? AND status='published' AND platform IN ('facebook','instagram','youtube') ORDER BY id DESC LIMIT 30`, tid)
			total := 0
			if rows != nil {
				var ids []int64
				for rows.Next() {
					var id int64
					_ = rows.Scan(&id)
					ids = append(ids, id)
				}
				rows.Close()
				for _, id := range ids {
					n, _ := a.syncSocialCommentsV23(r.Context(), tid, id)
					total += n
				}
			}
			writeJSON(w, map[string]any{"ok": true, "synced": total})
			return
		}
		if q.Action == "resolve" {
			_, e := a.db.Exec(`UPDATE social_comments_v23 SET status='resolved' WHERE id=? AND tenant_id=?`, q.ID, tid)
			if e != nil {
				http.Error(w, e.Error(), 500)
				return
			}
			writeJSON(w, map[string]any{"ok": true})
			return
		}
		if q.Action == "ai_reply" {
			if a.openAIKey() == "" {
				http.Error(w, "OpenAI no configurado", 503)
				return
			}
			var author, body, platform string
			e := a.db.QueryRow(`SELECT author_name,body,platform FROM social_comments_v23 WHERE id=? AND tenant_id=?`, q.ID, tid).Scan(&author, &body, &platform)
			if e != nil {
				http.Error(w, "comentario no encontrado", 404)
				return
			}
			sys := `Eres community manager profesional. Redacta una respuesta breve, útil y humana al comentario. No inventes hechos, precios ni promesas. Si hay una queja, reconoce el problema sin admitir responsabilidad legal y propone continuar por canal privado cuando corresponda. Devuelve solo el texto de respuesta.`
			user := fmt.Sprintf("Red: %s\nAutor: %s\nComentario: %s\nTono: %s", platform, author, body, q.Tone)
			ans, e := a.callOpenAI(sys, user)
			if e != nil {
				http.Error(w, e.Error(), 502)
				return
			}
			writeJSON(w, map[string]any{"reply": strings.TrimSpace(ans)})
			return
		}
		if q.Action == "reply" {
			if !p.Publish {
				http.Error(w, "No tienes permiso para responder", 403)
				return
			}
			if strings.TrimSpace(q.Text) == "" {
				http.Error(w, "respuesta requerida", 400)
				return
			}
			var postID int64
			var platform, ext string
			e := a.db.QueryRow(`SELECT post_id,platform,external_comment_id FROM social_comments_v23 WHERE id=? AND tenant_id=?`, q.ID, tid).Scan(&postID, &platform, &ext)
			if e != nil {
				http.Error(w, "comentario no encontrado", 404)
				return
			}
			post, tok, _, e := socialCommentLoadCreds(a, tid, postID)
			if e != nil {
				http.Error(w, e.Error(), 409)
				return
			}
			replyID := ""
			if platform == "facebook" || platform == "instagram" {
				vals := url.Values{"message": {q.Text}, "access_token": {tok.AccessToken}}
				req, _ := http.NewRequestWithContext(r.Context(), "POST", "https://graph.facebook.com/"+a.cfg.MetaGraphVersion+"/"+url.PathEscape(ext)+"/comments", strings.NewReader(vals.Encode()))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				resp, e := http.DefaultClient.Do(req)
				if e != nil {
					http.Error(w, e.Error(), 502)
					return
				}
				b, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				if resp.StatusCode/100 != 2 {
					http.Error(w, string(b), 502)
					return
				}
				var out map[string]any
				_ = json.Unmarshal(b, &out)
				replyID = fmt.Sprint(out["id"])
			} else if platform == "youtube" {
				tok, e = a.refreshSocialTokenIfNeeded(r.Context(), post.ConnectionID, tid, platform, tok)
				if e != nil {
					http.Error(w, e.Error(), 502)
					return
				}
				payload := map[string]any{"snippet": map[string]any{"parentId": ext, "textOriginal": q.Text}}
				b, _ := json.Marshal(payload)
				req, _ := http.NewRequestWithContext(r.Context(), "POST", "https://www.googleapis.com/youtube/v3/comments?part=snippet", bytes.NewReader(b))
				req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
				req.Header.Set("Content-Type", "application/json")
				resp, e := http.DefaultClient.Do(req)
				if e != nil {
					http.Error(w, e.Error(), 502)
					return
				}
				rb, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				if resp.StatusCode/100 != 2 {
					http.Error(w, string(rb), 502)
					return
				}
				var out map[string]any
				_ = json.Unmarshal(rb, &out)
				replyID = fmt.Sprint(out["id"])
			} else {
				http.Error(w, "respuesta oficial no disponible para esta red en V23", 409)
				return
			}
			_, _ = a.db.Exec(`UPDATE social_comments_v23 SET status='replied',response_text=?,external_reply_id=? WHERE id=? AND tenant_id=?`, q.Text, replyID, q.ID, tid)
			_ = a.enqueueAutomationEvent(tid, "social.comment.replied", "social_v23", map[string]any{"comment_id": q.ID, "post_id": postID, "platform": platform, "reply_id": replyID})
			writeJSON(w, map[string]any{"ok": true, "reply_id": replyID})
			return
		}
		http.Error(w, "acción inválida", 400)
	default:
		http.Error(w, "Método no permitido", 405)
	}
}
