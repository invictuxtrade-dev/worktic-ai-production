package main

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
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

type socialToken struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	TokenType    string `json:"token_type,omitempty"`
	ExpiresIn    int64  `json:"expires_in,omitempty"`
	OpenID       string `json:"open_id,omitempty"`
}

type socialProviderConfig struct {
	PageID      string `json:"page_id,omitempty"`
	InstagramID string `json:"instagram_id,omitempty"`
	AuthorURN   string `json:"author_urn,omitempty"`
	ChatID      string `json:"chat_id,omitempty"`
	ChannelID   string `json:"channel_id,omitempty"`
	OpenID      string `json:"open_id,omitempty"`
}

func ensureSocialProviderSchema(db *sql.DB) {
	alters := []string{
		`ALTER TABLE social_connections ADD COLUMN encrypted_credentials TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE social_connections ADD COLUMN config_json TEXT NOT NULL DEFAULT '{}'`,
		`ALTER TABLE social_connections ADD COLUMN token_expires_at TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE social_connections ADD COLUMN refresh_status TEXT NOT NULL DEFAULT ''`,
		`CREATE TABLE IF NOT EXISTS social_oauth_states(id INTEGER PRIMARY KEY AUTOINCREMENT,state TEXT UNIQUE NOT NULL,tenant_id INTEGER NOT NULL,platform TEXT NOT NULL,created_at TEXT NOT NULL,expires_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS social_publish_attempts(id INTEGER PRIMARY KEY AUTOINCREMENT,tenant_id INTEGER NOT NULL,post_id INTEGER NOT NULL,attempt_no INTEGER NOT NULL DEFAULT 1,status TEXT NOT NULL,provider_response TEXT NOT NULL DEFAULT '',error_message TEXT NOT NULL DEFAULT '',created_at TEXT NOT NULL)`,
	}
	for _, q := range alters {
		_, _ = db.Exec(q)
	}
}

func decryptLocal(s, key string) string {
	if s == "" {
		return ""
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256([]byte(key))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return ""
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil || len(raw) < gcm.NonceSize() {
		return ""
	}
	plain, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	if err != nil {
		return ""
	}
	return string(plain)
}

func randomState() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func (a *App) socialOAuthStartHandler(w http.ResponseWriter, r *http.Request) {
	ensureSocialProviderSchema(a.db)
	tid, u, err := a.tenantFor(r)
	if err != nil {
		http.Error(w, err.Error(), 401)
		return
	}
	if !socialPermissionsFor(u).ManageConnections {
		http.Error(w, "Solo propietarios y administradores pueden conectar redes", 403)
		return
	}
	p := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("platform")))
	if !validSocialPlatform(p) || p == "telegram" {
		http.Error(w, "plataforma OAuth no válida", 400)
		return
	}
	state := randomState()
	now := time.Now().UTC()
	exp := now.Add(15 * time.Minute)
	_, err = a.db.Exec(`INSERT INTO social_oauth_states(state,tenant_id,platform,created_at,expires_at) VALUES(?,?,?,?,?)`, state, tid, p, now.Format(time.RFC3339), exp.Format(time.RFC3339))
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	redirect := a.cfg.BaseURL + "/api/social/oauth/callback/" + p
	var auth string
	switch p {
	case "facebook", "instagram":
		if a.cfg.MetaAppID == "" {
			http.Error(w, "META_APP_ID no configurado", 409)
			return
		}
		q := url.Values{"client_id": {a.cfg.MetaAppID}, "redirect_uri": {redirect}, "state": {state}, "response_type": {"code"}, "scope": {"pages_show_list,pages_read_engagement,pages_manage_posts,instagram_basic,instagram_content_publish,business_management"}}
		auth = "https://www.facebook.com/" + a.cfg.MetaGraphVersion + "/dialog/oauth?" + q.Encode()
	case "linkedin":
		if a.cfg.LinkedInClientID == "" {
			http.Error(w, "LINKEDIN_CLIENT_ID no configurado", 409)
			return
		}
		q := url.Values{"response_type": {"code"}, "client_id": {a.cfg.LinkedInClientID}, "redirect_uri": {redirect}, "state": {state}, "scope": {"openid profile w_member_social"}}
		auth = "https://www.linkedin.com/oauth/v2/authorization?" + q.Encode()
	case "tiktok":
		if a.cfg.TikTokClientKey == "" {
			http.Error(w, "TIKTOK_CLIENT_KEY no configurado", 409)
			return
		}
		q := url.Values{"client_key": {a.cfg.TikTokClientKey}, "response_type": {"code"}, "scope": {"user.info.basic,video.upload,video.publish"}, "redirect_uri": {redirect}, "state": {state}}
		auth = "https://www.tiktok.com/v2/auth/authorize/?" + q.Encode()
	case "youtube":
		if a.cfg.GoogleClientID == "" {
			http.Error(w, "GOOGLE_CLIENT_ID no configurado", 409)
			return
		}
		q := url.Values{"client_id": {a.cfg.GoogleClientID}, "redirect_uri": {redirect}, "response_type": {"code"}, "scope": {"https://www.googleapis.com/auth/youtube.upload https://www.googleapis.com/auth/youtube.readonly"}, "access_type": {"offline"}, "prompt": {"consent"}, "state": {state}}
		auth = "https://accounts.google.com/o/oauth2/v2/auth?" + q.Encode()
	}
	writeJSON(w, map[string]any{"authorization_url": auth, "state": state})
}

func (a *App) socialOAuthCallbackHandler(w http.ResponseWriter, r *http.Request) {
	ensureSocialProviderSchema(a.db)
	p := strings.TrimPrefix(r.URL.Path, "/api/social/oauth/callback/")
	state, code := r.URL.Query().Get("state"), r.URL.Query().Get("code")
	if e := r.URL.Query().Get("error"); e != "" {
		http.Redirect(w, r, a.cfg.BaseURL+"/app.html?social_oauth=error&reason="+url.QueryEscape(e), 302)
		return
	}
	var tid int64
	var storedP, expires string
	err := a.db.QueryRow(`SELECT tenant_id,platform,expires_at FROM social_oauth_states WHERE state=?`, state).Scan(&tid, &storedP, &expires)
	if err != nil || storedP != p || code == "" {
		http.Error(w, "OAuth inválido o vencido", 400)
		return
	}
	ex, _ := time.Parse(time.RFC3339, expires)
	if time.Now().After(ex) {
		http.Error(w, "OAuth vencido", 400)
		return
	}
	_, _ = a.db.Exec(`DELETE FROM social_oauth_states WHERE state=?`, state)
	redirect := a.cfg.BaseURL + "/api/social/oauth/callback/" + p
	token, err := a.exchangeSocialCode(r.Context(), p, code, redirect)
	if err != nil {
		http.Redirect(w, r, a.cfg.BaseURL+"/app.html?social_oauth=error&reason="+url.QueryEscape(err.Error()), 302)
		return
	}
	if err = a.discoverAndSaveSocialConnection(r.Context(), tid, p, token); err != nil {
		http.Redirect(w, r, a.cfg.BaseURL+"/app.html?social_oauth=error&reason="+url.QueryEscape(err.Error()), 302)
		return
	}
	http.Redirect(w, r, a.cfg.BaseURL+"/app.html?social_oauth=success&platform="+url.QueryEscape(p), 302)
}

func (a *App) exchangeSocialCode(ctx context.Context, p, code, redirect string) (socialToken, error) {
	var endpoint string
	vals := url.Values{}
	switch p {
	case "facebook", "instagram":
		endpoint = "https://graph.facebook.com/" + a.cfg.MetaGraphVersion + "/oauth/access_token"
		vals = url.Values{"client_id": {a.cfg.MetaAppID}, "client_secret": {a.cfg.MetaAppSecret}, "redirect_uri": {redirect}, "code": {code}}
	case "linkedin":
		endpoint = "https://www.linkedin.com/oauth/v2/accessToken"
		vals = url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {redirect}, "client_id": {a.cfg.LinkedInClientID}, "client_secret": {a.cfg.LinkedInClientSecret}}
	case "tiktok":
		endpoint = "https://open.tiktokapis.com/v2/oauth/token/"
		vals = url.Values{"client_key": {a.cfg.TikTokClientKey}, "client_secret": {a.cfg.TikTokClientSecret}, "code": {code}, "grant_type": {"authorization_code"}, "redirect_uri": {redirect}}
	case "youtube":
		endpoint = "https://oauth2.googleapis.com/token"
		vals = url.Values{"client_id": {a.cfg.GoogleClientID}, "client_secret": {a.cfg.GoogleClientSecret}, "code": {code}, "grant_type": {"authorization_code"}, "redirect_uri": {redirect}}
	default:
		return socialToken{}, errors.New("proveedor no soportado")
	}
	req, _ := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(vals.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return socialToken{}, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode/100 != 2 {
		return socialToken{}, fmt.Errorf("token %s: %s", resp.Status, string(b))
	}
	var t socialToken
	if err = json.Unmarshal(b, &t); err != nil {
		return t, err
	}
	if t.AccessToken == "" {
		return t, errors.New("proveedor no devolvió access token")
	}
	return t, nil
}

func apiJSON(ctx context.Context, method, endpoint, token string, body any, out any) error {
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequestWithContext(ctx, method, endpoint, rd)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("%s: %s", resp.Status, string(b))
	}
	if out != nil {
		return json.Unmarshal(b, out)
	}
	return nil
}

func (a *App) saveOfficialConnection(tid int64, p, name, external string, tok socialToken, cfg socialProviderConfig, scopes string) error {
	now := time.Now().UTC()
	exp := ""
	if tok.ExpiresIn > 0 {
		exp = now.Add(time.Duration(tok.ExpiresIn) * time.Second).Format(time.RFC3339)
	}
	tb, _ := json.Marshal(tok)
	cb, _ := json.Marshal(cfg)
	_, err := a.db.Exec(`INSERT INTO social_connections(tenant_id,platform,account_name,external_account_id,status,provider_mode,scopes,last_sync_at,last_error,created_at,updated_at,encrypted_credentials,config_json,token_expires_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(tenant_id,platform,external_account_id) DO UPDATE SET account_name=excluded.account_name,status='connected',provider_mode='official',scopes=excluded.scopes,last_error='',updated_at=excluded.updated_at,encrypted_credentials=excluded.encrypted_credentials,config_json=excluded.config_json,token_expires_at=excluded.token_expires_at`, tid, p, name, external, "connected", "official", scopes, now.Format(time.RFC3339), "", now.Format(time.RFC3339), now.Format(time.RFC3339), encryptLocal(string(tb), a.cfg.ChannelEncryptionKey), string(cb), exp)
	return err
}

func (a *App) discoverAndSaveSocialConnection(ctx context.Context, tid int64, p string, t socialToken) error {
	switch p {
	case "facebook", "instagram":
		var pages struct {
			Data []struct {
				ID, Name, AccessToken string `json:"id"`
				Instagram             *struct {
					ID string `json:"id"`
				} `json:"instagram_business_account"`
			} `json:"data"`
		}
		ep := "https://graph.facebook.com/" + a.cfg.MetaGraphVersion + "/me/accounts?fields=id,name,access_token,instagram_business_account&access_token=" + url.QueryEscape(t.AccessToken)
		if err := apiJSON(ctx, "GET", ep, "", nil, &pages); err != nil {
			return err
		}
		if len(pages.Data) == 0 {
			return errors.New("no se encontraron páginas administradas")
		}
		for _, pg := range pages.Data {
			pt := t
			pt.AccessToken = pg.AccessToken
			_ = a.saveOfficialConnection(tid, "facebook", pg.Name, pg.ID, pt, socialProviderConfig{PageID: pg.ID}, "pages_manage_posts,pages_read_engagement")
			if pg.Instagram != nil {
				_ = a.saveOfficialConnection(tid, "instagram", pg.Name+" · Instagram", pg.Instagram.ID, pt, socialProviderConfig{PageID: pg.ID, InstagramID: pg.Instagram.ID}, "instagram_basic,instagram_content_publish")
			}
		}
		return nil
	case "linkedin":
		var me struct {
			Sub, Name string `json:"sub"`
		}
		if err := apiJSON(ctx, "GET", "https://api.linkedin.com/v2/userinfo", t.AccessToken, nil, &me); err != nil {
			return err
		}
		return a.saveOfficialConnection(tid, p, me.Name, me.Sub, t, socialProviderConfig{AuthorURN: "urn:li:person:" + me.Sub}, "openid,profile,w_member_social")
	case "tiktok":
		var me struct {
			Data struct {
				User struct {
					OpenID      string `json:"open_id"`
					DisplayName string `json:"display_name"`
				} `json:"user"`
			} `json:"data"`
		}
		_ = apiJSON(ctx, "GET", "https://open.tiktokapis.com/v2/user/info/?fields=open_id,display_name", t.AccessToken, nil, &me)
		if me.Data.User.OpenID == "" {
			me.Data.User.OpenID = t.OpenID
		}
		return a.saveOfficialConnection(tid, p, me.Data.User.DisplayName, me.Data.User.OpenID, t, socialProviderConfig{OpenID: me.Data.User.OpenID}, "user.info.basic,video.upload,video.publish")
	case "youtube":
		var ch struct {
			Items []struct {
				ID      string `json:"id"`
				Snippet struct {
					Title string `json:"title"`
				} `json:"snippet"`
			} `json:"items"`
		}
		if err := apiJSON(ctx, "GET", "https://www.googleapis.com/youtube/v3/channels?part=snippet&mine=true", t.AccessToken, nil, &ch); err != nil {
			return err
		}
		if len(ch.Items) == 0 {
			return errors.New("no se encontró canal de YouTube")
		}
		return a.saveOfficialConnection(tid, p, ch.Items[0].Snippet.Title, ch.Items[0].ID, t, socialProviderConfig{ChannelID: ch.Items[0].ID}, "youtube.upload,youtube.readonly")
	}
	return errors.New("proveedor no soportado")
}

func (a *App) socialConnectionTestHandler(w http.ResponseWriter, r *http.Request) {
	tid, u, err := a.tenantFor(r)
	if err != nil {
		http.Error(w, err.Error(), 401)
		return
	}
	if !socialPermissionsFor(u).ManageConnections {
		http.Error(w, "Solo propietarios y administradores pueden probar conexiones", 403)
		return
	}
	var q struct {
		ID int64 `json:"id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&q)
	var p, status string
	err = a.db.QueryRow(`SELECT platform,status FROM social_connections WHERE id=? AND tenant_id=?`, q.ID, tid).Scan(&p, &status)
	if err != nil {
		http.Error(w, "conexión no encontrada", 404)
		return
	}
	writeJSON(w, map[string]any{"ok": status == "connected", "platform": p, "status": status})
}

func (a *App) runSocialPublisher() {
	ensureSocialProviderSchema(a.db)
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		rows, err := a.db.Query(`SELECT id,tenant_id FROM social_posts WHERE status IN ('queued','scheduled','retrying') AND (scheduled_at='' OR scheduled_at<=?) ORDER BY id LIMIT 20`, time.Now().UTC().Format(time.RFC3339))
		if err != nil {
			continue
		}
		var jobs [][2]int64
		for rows.Next() {
			var id, tid int64
			_ = rows.Scan(&id, &tid)
			jobs = append(jobs, [2]int64{id, tid})
		}
		rows.Close()
		for _, j := range jobs {
			_, _ = a.publishSocialPost(context.Background(), j[1], j[0])
		}
	}
}

func (a *App) publishSocialPost(ctx context.Context, tid, id int64) (map[string]any, error) {
	ensureSocialProviderSchema(a.db)
	var p SocialPost
	var mode, cstatus, enc, cfgj string
	err := a.db.QueryRow(`SELECT p.id,p.tenant_id,p.connection_id,p.platform,p.format,p.title,p.caption,p.link_url,p.media_json,p.status,COALESCE(c.provider_mode,''),COALESCE(c.status,''),COALESCE(c.encrypted_credentials,''),COALESCE(c.config_json,'{}') FROM social_posts p LEFT JOIN social_connections c ON c.id=p.connection_id AND c.tenant_id=p.tenant_id WHERE p.id=? AND p.tenant_id=?`, id, tid).Scan(&p.ID, &p.TenantID, &p.ConnectionID, &p.Platform, &p.Format, &p.Title, &p.Caption, &p.LinkURL, &p.MediaJSON, &p.Status, &mode, &cstatus, &enc, &cfgj)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if p.ConnectionID == 0 || cstatus != "connected" {
		return nil, errors.New("conecta una cuenta activa para esta red")
	}
	_, _ = a.db.Exec(`UPDATE social_posts SET status='publishing',updated_at=? WHERE id=? AND tenant_id=?`, now, id, tid)
	if mode == "sandbox" {
		external := fmt.Sprintf("sandbox-%s-%d", p.Platform, id)
		u := fmt.Sprintf("https://social.worktic.local/%s/%s", p.Platform, external)
		_, _ = a.db.Exec(`UPDATE social_posts SET status='published',published_at=?,external_post_id=?,published_url=?,error_message='',updated_at=? WHERE id=? AND tenant_id=?`, now, external, u, now, id, tid)
		return map[string]any{"id": external, "url": u, "sandbox": true}, nil
	}
	var tok socialToken
	if json.Unmarshal([]byte(decryptLocal(enc, a.cfg.ChannelEncryptionKey)), &tok) != nil || tok.AccessToken == "" {
		return nil, errors.New("credenciales inválidas")
	}
	var cfg socialProviderConfig
	_ = json.Unmarshal([]byte(cfgj), &cfg)
	media := ""
	var arr []map[string]string
	_ = json.Unmarshal([]byte(p.MediaJSON), &arr)
	if len(arr) > 0 {
		media = arr[0]["url"]
	}
	var result map[string]any
	switch p.Platform {
	case "facebook":
		result, err = a.publishFacebook(ctx, tok.AccessToken, cfg.PageID, p.Caption, p.LinkURL, media)
	case "instagram":
		result, err = a.publishInstagram(ctx, tok.AccessToken, cfg.InstagramID, p.Caption, media, p.Format)
	case "linkedin":
		result, err = a.publishLinkedIn(ctx, tok.AccessToken, cfg.AuthorURN, p.Caption, p.LinkURL)
	case "telegram":
		result, err = a.publishTelegram(ctx, tok.AccessToken, cfg.ChatID, p.Caption, media)
	case "tiktok":
		result, err = a.publishTikTok(ctx, tok.AccessToken, p.Caption, media, p.Format)
	case "youtube":
		result, err = a.publishYouTube(ctx, tok.AccessToken, p.Title, p.Caption, media)
	default:
		err = errors.New("plataforma no soportada")
	}
	attempt := 1
	_ = a.db.QueryRow(`SELECT COUNT(*)+1 FROM social_publish_attempts WHERE tenant_id=? AND post_id=?`, tid, id).Scan(&attempt)
	rb, _ := json.Marshal(result)
	if err != nil {
		_, _ = a.db.Exec(`INSERT INTO social_publish_attempts(tenant_id,post_id,attempt_no,status,error_message,created_at) VALUES(?,?,?,?,?,?)`, tid, id, attempt, "failed", err.Error(), now)
		nextStatus := "failed"
		if attempt < 4 {
			nextStatus = "retrying"
		}
		_, _ = a.db.Exec(`UPDATE social_posts SET status=?,error_message=?,updated_at=?,scheduled_at=? WHERE id=? AND tenant_id=?`, nextStatus, err.Error(), now, time.Now().UTC().Add(time.Duration(attempt*5)*time.Minute).Format(time.RFC3339), id, tid)
		return nil, err
	}
	external, _ := result["id"].(string)
	pu, _ := result["url"].(string)
	_, _ = a.db.Exec(`INSERT INTO social_publish_attempts(tenant_id,post_id,attempt_no,status,provider_response,created_at) VALUES(?,?,?,?,?,?)`, tid, id, attempt, "published", string(rb), now)
	_, _ = a.db.Exec(`UPDATE social_posts SET status='published',published_at=?,external_post_id=?,published_url=?,error_message='',updated_at=? WHERE id=? AND tenant_id=?`, now, external, pu, now, id, tid)
	return result, nil
}

func (a *App) publishFacebook(ctx context.Context, token, pageID, caption, link, media string) (map[string]any, error) {
	ep := "https://graph.facebook.com/" + a.cfg.MetaGraphVersion + "/" + pageID + "/feed"
	vals := url.Values{"message": {caption}, "access_token": {token}}
	if link != "" {
		vals.Set("link", link)
	}
	if media != "" {
		ep = "https://graph.facebook.com/" + a.cfg.MetaGraphVersion + "/" + pageID + "/photos"
		vals.Set("url", media)
		vals.Set("caption", caption)
		vals.Del("message")
	}
	var out map[string]any
	req, _ := http.NewRequestWithContext(ctx, "POST", ep, strings.NewReader(vals.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("Meta: %s", string(b))
	}
	_ = json.Unmarshal(b, &out)
	id := fmt.Sprint(out["id"])
	return map[string]any{"id": id, "url": "https://www.facebook.com/" + id}, nil
}
func (a *App) publishInstagram(ctx context.Context, token, igID, caption, media, format string) (map[string]any, error) {
	if media == "" {
		return nil, errors.New("Instagram requiere URL pública de imagen o video")
	}
	vals := url.Values{"caption": {caption}, "access_token": {token}}
	if format == "reel" || format == "video" {
		vals.Set("media_type", "REELS")
		vals.Set("video_url", media)
	} else {
		vals.Set("image_url", media)
	}
	ep := "https://graph.facebook.com/" + a.cfg.MetaGraphVersion + "/" + igID + "/media"
	req, _ := http.NewRequestWithContext(ctx, "POST", ep, strings.NewReader(vals.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("Instagram container: %s", b)
	}
	var c map[string]any
	_ = json.Unmarshal(b, &c)
	cid := fmt.Sprint(c["id"])
	time.Sleep(2 * time.Second)
	vals = url.Values{"creation_id": {cid}, "access_token": {token}}
	req, _ = http.NewRequestWithContext(ctx, "POST", "https://graph.facebook.com/"+a.cfg.MetaGraphVersion+"/"+igID+"/media_publish", strings.NewReader(vals.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	b, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("Instagram publish: %s", b)
	}
	var out map[string]any
	_ = json.Unmarshal(b, &out)
	id := fmt.Sprint(out["id"])
	return map[string]any{"id": id, "url": "https://www.instagram.com/"}, nil
}
func (a *App) publishLinkedIn(ctx context.Context, token, author, caption, link string) (map[string]any, error) {
	content := map[string]any{"author": author, "commentary": caption, "visibility": "PUBLIC", "distribution": map[string]any{"feedDistribution": "MAIN_FEED", "targetEntities": []any{}, "thirdPartyDistributionChannels": []any{}}, "lifecycleState": "PUBLISHED", "isReshareDisabledByAuthor": false}
	if link != "" {
		content["content"] = map[string]any{"article": map[string]any{"source": link, "title": "Ver más"}}
	}
	b, _ := json.Marshal(content)
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://api.linkedin.com/rest/posts", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("LinkedIn-Version", "202606")
	req.Header.Set("X-Restli-Protocol-Version", "2.0.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	rb, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("LinkedIn: %s", rb)
	}
	id := resp.Header.Get("x-restli-id")
	return map[string]any{"id": id, "url": "https://www.linkedin.com/feed/update/" + id}, nil
}
func (a *App) publishTelegram(ctx context.Context, token, chatID, caption, media string) (map[string]any, error) {
	if token == "" || chatID == "" {
		return nil, errors.New("Telegram requiere token de bot y chat/canal")
	}
	method := "sendMessage"
	vals := url.Values{"chat_id": {chatID}, "text": {caption}, "parse_mode": {"HTML"}}
	if media != "" {
		method = "sendPhoto"
		vals = url.Values{"chat_id": {chatID}, "photo": {media}, "caption": {caption}}
	}
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://api.telegram.org/bot"+token+"/"+method, strings.NewReader(vals.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("Telegram: %s", b)
	}
	var out struct {
		Result struct {
			MessageID int `json:"message_id"`
		} `json:"result"`
	}
	_ = json.Unmarshal(b, &out)
	id := strconv.Itoa(out.Result.MessageID)
	return map[string]any{"id": id, "url": ""}, nil
}
func (a *App) publishTikTok(ctx context.Context, token, caption, media, format string) (map[string]any, error) {
	if media == "" {
		return nil, errors.New("TikTok requiere URL de video o foto en dominio verificado")
	}
	body := map[string]any{"post_info": map[string]any{"title": caption, "privacy_level": "PUBLIC_TO_EVERYONE", "disable_duet": false, "disable_comment": false, "disable_stitch": false}, "source_info": map[string]any{"source": "PULL_FROM_URL", "video_url": media}}
	endpoint := "https://open.tiktokapis.com/v2/post/publish/video/init/"
	if format == "carousel" || format == "post" {
		endpoint = "https://open.tiktokapis.com/v2/post/publish/content/init/"
		body = map[string]any{"post_info": map[string]any{"title": caption, "description": caption, "privacy_level": "PUBLIC_TO_EVERYONE"}, "source_info": map[string]any{"source": "PULL_FROM_URL", "photo_images": []string{media}, "photo_cover_index": 0}, "post_mode": "DIRECT_POST", "media_type": "PHOTO"}
	}
	var out map[string]any
	if err := apiJSON(ctx, "POST", endpoint, token, body, &out); err != nil {
		return nil, err
	}
	id := ""
	if d, ok := out["data"].(map[string]any); ok {
		id = fmt.Sprint(d["publish_id"])
	}
	return map[string]any{"id": id, "url": ""}, nil
}
func (a *App) publishYouTube(ctx context.Context, token, title, description, media string) (map[string]any, error) {
	if media == "" {
		return nil, errors.New("YouTube requiere URL pública de video")
	}
	download, err := http.NewRequestWithContext(ctx, "GET", media, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(download)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, errors.New("no se pudo descargar el video")
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 512<<20))
	if err != nil {
		return nil, err
	}
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "video/mp4"
	}
	meta := map[string]any{"snippet": map[string]any{"title": title, "description": description, "categoryId": "22"}, "status": map[string]any{"privacyStatus": "public"}}
	mb, _ := json.Marshal(meta)
	initReq, _ := http.NewRequestWithContext(ctx, "POST", "https://www.googleapis.com/upload/youtube/v3/videos?uploadType=resumable&part=snippet,status", bytes.NewReader(mb))
	initReq.Header.Set("Authorization", "Bearer "+token)
	initReq.Header.Set("Content-Type", "application/json; charset=UTF-8")
	initReq.Header.Set("X-Upload-Content-Type", contentType)
	initReq.Header.Set("X-Upload-Content-Length", strconv.Itoa(len(data)))
	initResp, err := http.DefaultClient.Do(initReq)
	if err != nil {
		return nil, err
	}
	ib, _ := io.ReadAll(initResp.Body)
	initResp.Body.Close()
	if initResp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("YouTube inicio: %s", ib)
	}
	location := initResp.Header.Get("Location")
	if location == "" {
		return nil, errors.New("YouTube no devolvió URL de carga")
	}
	putReq, _ := http.NewRequestWithContext(ctx, "PUT", location, bytes.NewReader(data))
	putReq.Header.Set("Authorization", "Bearer "+token)
	putReq.Header.Set("Content-Type", contentType)
	putReq.Header.Set("Content-Length", strconv.Itoa(len(data)))
	upResp, err := http.DefaultClient.Do(putReq)
	if err != nil {
		return nil, err
	}
	b, _ := io.ReadAll(upResp.Body)
	upResp.Body.Close()
	if upResp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("YouTube carga: %s", b)
	}
	var out map[string]any
	_ = json.Unmarshal(b, &out)
	id := fmt.Sprint(out["id"])
	return map[string]any{"id": id, "url": "https://youtu.be/" + id}, nil
}
