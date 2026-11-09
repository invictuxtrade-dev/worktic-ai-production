package main

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
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
	InstagramAuthMethod string `json:"instagram_auth_method,omitempty"`
	PageID              string `json:"page_id,omitempty"`
	InstagramID         string `json:"instagram_id,omitempty"`
	AuthorURN           string `json:"author_urn,omitempty"`
	ChatID              string `json:"chat_id,omitempty"`
	ChannelID           string `json:"channel_id,omitempty"`
	OpenID              string `json:"open_id,omitempty"`
}

func ensureSocialProviderSchema(db *DB) {
	alters := []string{
		`ALTER TABLE social_connections ADD COLUMN encrypted_credentials TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE social_connections ADD COLUMN config_json TEXT NOT NULL DEFAULT '{}'`,
		`ALTER TABLE social_connections ADD COLUMN token_expires_at TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE social_connections ADD COLUMN refresh_status TEXT NOT NULL DEFAULT ''`,
		`CREATE TABLE IF NOT EXISTS social_oauth_states(id INTEGER PRIMARY KEY AUTOINCREMENT,state TEXT UNIQUE NOT NULL,tenant_id INTEGER NOT NULL,platform TEXT NOT NULL,created_at TEXT NOT NULL,expires_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS social_oauth_pending(state TEXT PRIMARY KEY,tenant_id INTEGER NOT NULL,platform TEXT NOT NULL,encrypted_user_token TEXT NOT NULL,created_at TEXT NOT NULL,expires_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS social_publish_attempts(id INTEGER PRIMARY KEY AUTOINCREMENT,tenant_id INTEGER NOT NULL,post_id INTEGER NOT NULL,attempt_no INTEGER NOT NULL DEFAULT 1,status TEXT NOT NULL,provider_response TEXT NOT NULL DEFAULT '',error_message TEXT NOT NULL DEFAULT '',created_at TEXT NOT NULL)`,
	}
	for _, q := range alters {
		_, _ = db.Exec(q)
	}
	// Depurar periodicamente autorizaciones temporales abandonadas.
	_, _ = db.Exec(`DELETE FROM social_oauth_pending WHERE expires_at<?`, time.Now().UTC().Format(time.RFC3339))
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
		http.Error(w, "plataforma OAuth no vÃ¡lida", 400)
		return
	}
	// Validate provider configuration before creating a pending authorization.
	// In B9, Instagram Unified Login is gated until Meta's SUAT/account
	// discovery is verified; the old Page-based callback is NOT compatible.
	if p == "instagram" && !socialB9Enabled() && !instagramB103DEnabled() {
		http.Error(w, "B9 Instagram Unified Login requiere habilitacion explicita; el OAuth antiguo esta bloqueado", http.StatusConflict)
		return
	}
	state := randomState()
	redirect := a.cfg.BaseURL + "/api/social/oauth/callback/" + p
	var auth string
	if p == "instagram" && instagramB103DEnabled() {
		auth, err = instagramB103DAuthorizationURL(state, redirect)
		if err != nil {
			http.Error(w, "Instagram Login no esta configurado", http.StatusConflict)
			return
		}
	} else {
		switch p {
		case "facebook", "instagram":
			if a.cfg.MetaAppID == "" || (p == "instagram" && socialB9Enabled() && a.cfg.MetaAppSecret == "") {
				http.Error(w, "Credenciales Meta incompletas", 409)
				return
			}
			// B2: Meta define los permisos mediante una configuracion independiente por flujo.
			// Fallar cerrado impide volver a pedir scopes no disponibles en esta aplicacion.
			key := "WORKTIC_META_SOCIAL_FACEBOOK_CONFIG_ID"
			if p == "instagram" {
				key = "WORKTIC_META_SOCIAL_INSTAGRAM_CONFIG_ID"
			}
			configID := strings.TrimSpace(os.Getenv(key))
			if configID == "" || !isMetaSocialConfigID(configID) {
				http.Error(w, key+" no configurado o invalido", http.StatusConflict)
				return
			}
			if strings.TrimSpace(a.cfg.ChannelEncryptionKey) == "" {
				http.Error(w, "CHANNEL_ENCRYPTION_KEY no configurado", http.StatusConflict)
				return
			}
			q := url.Values{
				"client_id": {a.cfg.MetaAppID}, "redirect_uri": {redirect},
				"state": {state}, "response_type": {"code"},
				"config_id": {configID}, "override_default_response_type": {"true"},
			}
			if p == "instagram" && socialB9Enabled() {
				q.Set("prompt", "login")
			}
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
			q := url.Values{"client_id": {a.cfg.GoogleClientID}, "redirect_uri": {redirect}, "response_type": {"code"}, "scope": {"https://www.googleapis.com/auth/youtube.upload https://www.googleapis.com/auth/youtube.readonly https://www.googleapis.com/auth/youtube.force-ssl"}, "access_type": {"offline"}, "prompt": {"consent"}, "state": {state}}
			auth = "https://accounts.google.com/o/oauth2/v2/auth?" + q.Encode()
		}
	}
	if auth == "" {
		http.Error(w, "Proveedor OAuth no configurado", http.StatusConflict)
		return
	}
	now := time.Now().UTC()
	exp := now.Add(15 * time.Minute)
	_, err = a.db.Exec(`INSERT INTO social_oauth_states(state,tenant_id,platform,created_at,expires_at) VALUES(?,?,?,?,?)`, state, tid, p, now.Format(time.RFC3339), exp.Format(time.RFC3339))
	if err != nil {
		http.Error(w, "No fue posible iniciar la autorizacion", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, map[string]any{"authorization_url": auth})
}

// isMetaSocialConfigID solo admite los identificadores numericos provistos por Meta.
func isMetaSocialConfigID(id string) bool {
	if len(id) < 8 || len(id) > 32 {
		return false
	}
	for _, r := range id {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func (a *App) socialOAuthCallbackHandler(w http.ResponseWriter, r *http.Request) {
	ensureSocialProviderSchema(a.db)
	p := strings.TrimPrefix(r.URL.Path, "/api/social/oauth/callback/")
	if r.Method == http.MethodPost && p == "instagram" && socialB9Enabled() {
		a.completeInstagramB9Selection(w, r)
		return
	}
	if r.Method == http.MethodPost && (p == "facebook" || p == "instagram") {
		a.completeMetaSocialSelection(w, r, p)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "metodo no permitido", http.StatusMethodNotAllowed)
		return
	}
	state, code := r.URL.Query().Get("state"), r.URL.Query().Get("code")
	if e := r.URL.Query().Get("error"); e != "" {
		http.Redirect(w, r, a.cfg.BaseURL+"/app.html?social_oauth=error&reason="+url.QueryEscape(e), http.StatusFound)
		return
	}
	if len(state) < 32 || len(state) > 128 || code == "" {
		http.Error(w, "OAuth invalido", http.StatusBadRequest)
		return
	}
	// One atomic statement prevents concurrent callbacks from reusing a state.
	// Both SQLite (modernc) and PostgreSQL support DELETE ... RETURNING.
	var tid int64
	err := a.db.QueryRow(`DELETE FROM social_oauth_states
		WHERE state=? AND platform=? AND expires_at>?
		RETURNING tenant_id`, state, p, time.Now().UTC().Format(time.RFC3339)).Scan(&tid)
	if err != nil {
		http.Error(w, "OAuth invalido, vencido o utilizado", http.StatusBadRequest)
		return
	}
	redirect := a.cfg.BaseURL + "/api/social/oauth/callback/" + p
	if p == "instagram" && instagramB103DEnabled() {
		if err := a.completeInstagramB103DOAuth(w, r, tid, code, redirect); err != nil {
			reason := "Instagram B10.3D: error en autorizacion o verificacion [IG_LOGIN_ERROR]"
			if strings.Contains(err.Error(), "ya tiene una conexion") {
				reason = "Instagram B10.3D: cuenta ya vinculada [IG_ALREADY_CONNECTED]"
			}
			http.Redirect(w, r, a.cfg.BaseURL+"/app.html?social_oauth=error&reason="+url.QueryEscape(reason), http.StatusFound)
		}
		return
	}
	token, err := a.exchangeSocialCode(r.Context(), p, code, redirect)
	if err != nil {
		http.Redirect(w, r, a.cfg.BaseURL+"/app.html?social_oauth=error&reason="+url.QueryEscape("No se pudo completar el intercambio OAuth con el proveedor"), http.StatusFound)
		return
	}
	if p == "instagram" && socialB9Enabled() {
		if err = a.beginInstagramB9Selection(w, r, tid, token); err != nil {
			reason := "Instagram B9: error preparando seleccion [SELECTION_SETUP_ERROR]"
			if err.Error() == "Meta no permitio consultar cuentas Instagram asignadas" {
				reason = "Instagram B9: Meta rechazo la consulta de cuentas [DISCOVERY_API_ERROR]"
			} else if strings.HasPrefix(err.Error(), "Meta no devolvio cuentas Instagram profesionales asignadas [B9_SUAT_DIAG;") {
				reason = "Instagram B9: Meta devolvio cero cuentas autorizadas [DISCOVERY_EMPTY] " + strings.TrimPrefix(err.Error(), "Meta no devolvio cuentas Instagram profesionales asignadas ")
			}
			http.Redirect(w, r, a.cfg.BaseURL+"/app.html?social_oauth=error&reason="+url.QueryEscape(reason), http.StatusFound)
		}
		return
	}
	if p == "facebook" || p == "instagram" {
		// Nunca activar todas las paginas sin que el usuario las seleccione.
		if err = a.beginMetaSocialSelection(w, r, tid, p, token); err != nil {
			http.Redirect(w, r, a.cfg.BaseURL+"/app.html?social_oauth=error&reason="+url.QueryEscape(err.Error()), http.StatusFound)
		}
		return
	}
	if err = a.discoverAndSaveSocialConnection(r.Context(), tid, p, token); err != nil {
		http.Redirect(w, r, a.cfg.BaseURL+"/app.html?social_oauth=error&reason="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}
	http.Redirect(w, r, a.cfg.BaseURL+"/app.html?social_oauth=success&platform="+url.QueryEscape(p), http.StatusFound)
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
		return t, errors.New("proveedor no devolviÃ³ access token")
	}
	// Meta entrega inicialmente un token de usuario corto. Se intercambia por uno
	// de larga duraciÃ³n antes de descubrir las pÃ¡ginas y sus tokens.
	if p == "facebook" || (p == "instagram" && !socialB9Enabled()) {
		q := url.Values{
			"grant_type":        {"fb_exchange_token"},
			"client_id":         {a.cfg.MetaAppID},
			"client_secret":     {a.cfg.MetaAppSecret},
			"fb_exchange_token": {t.AccessToken},
		}
		req2, _ := http.NewRequestWithContext(ctx, "GET", "https://graph.facebook.com/"+a.cfg.MetaGraphVersion+"/oauth/access_token?"+q.Encode(), nil)
		if resp2, e2 := http.DefaultClient.Do(req2); e2 == nil {
			defer resp2.Body.Close()
			b2, _ := io.ReadAll(io.LimitReader(resp2.Body, 2<<20))
			if resp2.StatusCode/100 == 2 {
				var lt socialToken
				if json.Unmarshal(b2, &lt) == nil && lt.AccessToken != "" {
					t = lt
				}
			}
		}
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
	if (p == "facebook" || p == "instagram") && (strings.TrimSpace(a.cfg.ChannelEncryptionKey) == "" || strings.TrimSpace(tok.AccessToken) == "") {
		return errors.New("credenciales Meta invalidas: falta token o clave de cifrado")
	}
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
		// Estas plataformas solo se activan con seleccion explicita en el callback.
		return errors.New("Facebook e Instagram requieren elegir la pagina antes de conectar")
	case "linkedin":
		var me struct {
			Sub  string `json:"sub"`
			Name string `json:"name"`
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
			return errors.New("no se encontrÃ³ canal de YouTube")
		}
		return a.saveOfficialConnection(tid, p, ch.Items[0].Snippet.Title, ch.Items[0].ID, t, socialProviderConfig{ChannelID: ch.Items[0].ID}, "youtube.upload,youtube.readonly")
	}
	return errors.New("proveedor no soportado")
}

// metaSocialPage es la respuesta real de /me/accounts (incluye tareas por pagina).
type metaSocialPage struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	AccessToken string   `json:"access_token"`
	Tasks       []string `json:"tasks"`
	Instagram   *struct {
		ID string `json:"id"`
	} `json:"instagram_business_account"`
}

type metaSocialPermission struct {
	Permission string `json:"permission"`
	Status     string `json:"status"`
}

func metaGrantedSocialPermissions(items []metaSocialPermission) map[string]bool {
	granted := make(map[string]bool)
	for _, item := range items {
		if strings.EqualFold(item.Status, "granted") {
			granted[item.Permission] = true
		}
	}
	return granted
}

func metaTask(tasks []string, allowed ...string) bool {
	for _, task := range tasks {
		for _, a := range allowed {
			if strings.EqualFold(task, a) {
				return true
			}
		}
	}
	return false
}

// No anunciar permisos que no fueron concedidos; las tareas restringen la
// capacidad operativa incluso si la persona otorgo un permiso general.
func metaEffectiveSocialScopes(p string, granted map[string]bool, tasks []string) string {
	var permissions []string
	if p == "facebook" {
		for _, name := range []string{"pages_show_list", "pages_read_engagement", "business_management"} {
			if granted[name] {
				permissions = append(permissions, name)
			}
		}
		if granted["pages_manage_metadata"] && metaTask(tasks, "MANAGE") {
			permissions = append(permissions, "pages_manage_metadata")
		}
		if granted["pages_manage_posts"] && metaTask(tasks, "CREATE_CONTENT", "MANAGE") {
			permissions = append(permissions, "pages_manage_posts")
		}
		if granted["pages_manage_engagement"] && metaTask(tasks, "MODERATE", "MANAGE") {
			permissions = append(permissions, "pages_manage_engagement")
		}
		if granted["pages_read_user_content"] && metaTask(tasks, "MODERATE", "MANAGE") {
			permissions = append(permissions, "pages_read_user_content")
		}
		if granted["leads_retrieval"] && metaTask(tasks, "MANAGE_LEADS", "MANAGE") {
			permissions = append(permissions, "leads_retrieval")
		}
	} else if p == "instagram" {
		for _, name := range []string{"instagram_basic", "instagram_manage_insights"} {
			if granted[name] {
				permissions = append(permissions, name)
			}
		}
		if granted["instagram_content_publish"] && metaTask(tasks, "CREATE_CONTENT", "MANAGE") {
			permissions = append(permissions, "instagram_content_publish")
		}
		if granted["instagram_manage_comments"] && metaTask(tasks, "MODERATE", "MANAGE") {
			permissions = append(permissions, "instagram_manage_comments")
		}
	}
	sort.Strings(permissions)
	return strings.Join(permissions, ",")
}

func (a *App) metaSocialPermissions(ctx context.Context, token string) (map[string]bool, error) {
	var out struct {
		Data []metaSocialPermission `json:"data"`
	}
	url := "https://graph.facebook.com/" + a.cfg.MetaGraphVersion + "/me/permissions?fields=permission,status&limit=100"
	if err := apiJSON(ctx, http.MethodGet, url, token, nil, &out); err != nil {
		return nil, errors.New("Meta no permitio comprobar los permisos otorgados")
	}
	return metaGrantedSocialPermissions(out.Data), nil
}

func (a *App) metaSocialPages(ctx context.Context, p, token string) ([]metaSocialPage, error) {
	var pages []metaSocialPage
	fields := "id,name,access_token,tasks"
	if p == "instagram" {
		fields += ",instagram_business_account"
	}
	base := "https://graph.facebook.com/" + a.cfg.MetaGraphVersion + "/me/accounts?fields=" + fields + "&limit=100"
	var after string
	for batch := 0; batch < 5; batch++ {
		endpoint := base
		if after != "" {
			endpoint += "&after=" + url.QueryEscape(after)
		}
		var out struct {
			Data   []metaSocialPage `json:"data"`
			Paging struct {
				Next    string `json:"next"`
				Cursors struct {
					After string `json:"after"`
				} `json:"cursors"`
			} `json:"paging"`
		}
		if err := apiJSON(ctx, http.MethodGet, endpoint, token, nil, &out); err != nil {
			return nil, errors.New("Meta no permitio consultar las paginas autorizadas")
		}
		pages = append(pages, out.Data...)
		if out.Paging.Next == "" {
			return pages, nil
		}
		if out.Paging.Cursors.After == "" || out.Paging.Cursors.After == after {
			return nil, errors.New("Meta devolvio una paginacion invalida de paginas")
		}
		after = out.Paging.Cursors.After
	}
	return nil, errors.New("Demasiadas paginas para la seleccion; vuelve a intentar con menos activos")
}

// La pantalla intermedia selecciona una pagina (se puede repetir para agregar
// otras) sin entregar tokens de usuario/pagina al navegador.
var metaSocialSelectTemplate = template.Must(template.New("social-meta-select").Parse(`<!doctype html>
<html lang="es"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Conectar red social | Worktic AI</title><style>
body{margin:0;background:#f5f6fc;font:16px system-ui,sans-serif;color:#17172a;display:flex;min-height:100vh;align-items:center;justify-content:center}
main{max-width:540px;background:#fff;padding:32px;border:1px solid #e4def7;border-radius:20px;box-shadow:0 12px 35px #20204b12;margin:20px}
h1{font-size:24px;margin:0 0 10px}p{color:#59617d;line-height:1.5}label{display:block;margin:20px 0 8px;font-weight:600}
select{width:100%;padding:15px;border:1px solid #b9a8ef;border-radius:12px;font:inherit}
button{margin-top:24px;width:100%;border:0;border-radius:12px;padding:16px;color:#fff;font:600 16px system-ui;background:linear-gradient(95deg,#5060ff,#cf30ed);cursor:pointer}
small{display:block;color:#667085;margin-top:14px}a{color:#514cbb}</style></head><body><main>
<h1>Conectar {{if eq .Platform "facebook"}}Facebook{{else}}Instagram{{end}}</h1>
<p>Tu autorizacion de Meta fue recibida. Elige la pagina que deseas conectar a este espacio de Worktic AI. Puedes repetir el proceso para agregar otra.</p>
<form method="post" action="{{.Action}}">
<input type="hidden" name="selection_state" value="{{.State}}">
<label for="page">Pagina autorizada</label>
<select id="page" name="page_id" required>
<option value="">Selecciona una pagina</option>
{{range .Pages}}<option value="{{.ID}}">{{.Name}}</option>{{end}}
</select><button type="submit">Conectar pagina seleccionada</button>
</form><small>No se instalaran conexiones para las demas paginas. Los permisos que no otorgo Meta permaneceran deshabilitados.</small>
</main></body></html>`))

func (a *App) beginMetaSocialSelection(w http.ResponseWriter, r *http.Request, tid int64, p string, tok socialToken) error {
	if strings.TrimSpace(a.cfg.ChannelEncryptionKey) == "" {
		return errors.New("No esta configurada la clave de cifrado")
	}
	granted, err := a.metaSocialPermissions(r.Context(), tok.AccessToken)
	if err != nil {
		return err
	}
	if !granted["pages_show_list"] {
		return errors.New("La autorizacion no incluye pages_show_list")
	}
	if p == "facebook" && !granted["pages_read_engagement"] {
		return errors.New("La autorizacion no incluye pages_read_engagement")
	}
	if p == "instagram" && !granted["instagram_basic"] {
		return errors.New("La autorizacion no incluye instagram_basic")
	}
	pages, err := a.metaSocialPages(r.Context(), p, tok.AccessToken)
	if err != nil {
		return err
	}
	var choices []metaSocialPage
	for _, page := range pages {
		if page.ID == "" || page.AccessToken == "" {
			continue
		}
		if p == "instagram" && (page.Instagram == nil || page.Instagram.ID == "") {
			continue
		}
		choices = append(choices, page)
	}
	if len(choices) == 0 {
		return errors.New("No se encontraron paginas con token valido para esta integracion")
	}
	b, err := json.Marshal(tok)
	if err != nil {
		return errors.New("No fue posible preparar el token de Meta")
	}
	selectionState := randomState()
	now := time.Now().UTC()
	_, err = a.db.Exec(`INSERT INTO social_oauth_pending(state,tenant_id,platform,encrypted_user_token,created_at,expires_at) VALUES(?,?,?,?,?,?)`,
		selectionState, tid, p, encryptLocal(string(b), a.cfg.ChannelEncryptionKey), now.Format(time.RFC3339), now.Add(15*time.Minute).Format(time.RFC3339))
	if err != nil {
		return errors.New("No fue posible preparar la seleccion de paginas")
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "origin")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'")
	return metaSocialSelectTemplate.Execute(w, struct {
		Platform string
		State    string
		Action   string
		Pages    []metaSocialPage
	}{p, selectionState, a.cfg.BaseURL + "/api/social/oauth/callback/" + p, choices})
}

func (a *App) completeMetaSocialSelection(w http.ResponseWriter, r *http.Request, p string) {
	const maxFormBytes = 16384
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Solicitud de seleccion invalida", http.StatusBadRequest)
		return
	}
	selectionState := strings.TrimSpace(r.PostForm.Get("selection_state"))
	pageID := strings.TrimSpace(r.PostForm.Get("page_id"))
	if len(selectionState) < 32 || len(pageID) == 0 || len(pageID) > 128 {
		http.Error(w, "Seleccion incompleta", http.StatusBadRequest)
		return
	}
	var tid int64
	var storedP, encrypted, expiry string
	err := a.db.QueryRow(`SELECT tenant_id,platform,encrypted_user_token,expires_at FROM social_oauth_pending WHERE state=?`, selectionState).Scan(&tid, &storedP, &encrypted, &expiry)
	if err != nil || storedP != p {
		http.Error(w, "Seleccion vencida o no encontrada", http.StatusBadRequest)
		return
	}
	ex, err := time.Parse(time.RFC3339, expiry)
	if err != nil || time.Now().After(ex) {
		_, _ = a.db.Exec(`DELETE FROM social_oauth_pending WHERE state=?`, selectionState)
		http.Error(w, "Seleccion vencida; vuelve a conectar desde Worktic AI", http.StatusBadRequest)
		return
	}
	var userToken socialToken
	if json.Unmarshal([]byte(decryptLocal(encrypted, a.cfg.ChannelEncryptionKey)), &userToken) != nil || userToken.AccessToken == "" {
		http.Error(w, "Credenciales vencidas o invalidas", http.StatusConflict)
		return
	}
	granted, err := a.metaSocialPermissions(r.Context(), userToken.AccessToken)
	if err != nil || !granted["pages_show_list"] {
		http.Error(w, "Meta no confirmo acceso a las paginas", http.StatusConflict)
		return
	}
	pages, err := a.metaSocialPages(r.Context(), p, userToken.AccessToken)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	var chosen *metaSocialPage
	for i := range pages {
		if pages[i].ID == pageID && pages[i].AccessToken != "" {
			chosen = &pages[i]
			break
		}
	}
	if chosen == nil {
		http.Error(w, "Pagina no autorizada o sin token de pagina", http.StatusForbidden)
		return
	}
	pageScopes := metaEffectiveSocialScopes(p, granted, chosen.Tasks)
	var externalID, accountName string
	cfg := socialProviderConfig{PageID: chosen.ID}
	if p == "facebook" {
		if !granted["pages_read_engagement"] {
			http.Error(w, "Falta pages_read_engagement", http.StatusForbidden)
			return
		}
		// Prueba real de lectura de publicaciones; una pagina sin publicaciones
		// tambien puede responder correctamente con una lista vacia.
		var posts struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		endpoint := "https://graph.facebook.com/" + a.cfg.MetaGraphVersion + "/" + url.PathEscape(chosen.ID) + "/posts?fields=id&limit=1"
		if apiJSON(r.Context(), http.MethodGet, endpoint, chosen.AccessToken, nil, &posts) != nil {
			http.Error(w, "Meta no permitio leer las publicaciones de esta pagina con el token autorizado", http.StatusConflict)
			return
		}
		externalID, accountName = chosen.ID, chosen.Name
	} else {
		if !granted["instagram_basic"] || chosen.Instagram == nil || chosen.Instagram.ID == "" {
			http.Error(w, "La pagina no tiene Instagram autorizada", http.StatusForbidden)
			return
		}
		cfg.InstagramID = chosen.Instagram.ID
		var ig struct {
			ID       string `json:"id"`
			Username string `json:"username"`
		}
		endpoint := "https://graph.facebook.com/" + a.cfg.MetaGraphVersion + "/" + url.PathEscape(cfg.InstagramID) + "?fields=id,username"
		if apiJSON(r.Context(), http.MethodGet, endpoint, chosen.AccessToken, nil, &ig) != nil || ig.ID != cfg.InstagramID {
			http.Error(w, "Meta no permitio leer la cuenta de Instagram", http.StatusConflict)
			return
		}
		externalID, accountName = cfg.InstagramID, ig.Username
	}
	pageToken := userToken
	pageToken.AccessToken = chosen.AccessToken
	// No atribuir a los page access tokens la caducidad del user token.
	// Meta determina por separado vigencia y revocacion de cada credencial.
	pageToken.ExpiresIn = 0
	pageToken.RefreshToken = ""
	if err = a.saveOfficialConnection(tid, p, accountName, externalID, pageToken, cfg, pageScopes); err != nil {
		http.Error(w, "No se pudo guardar la conexion verificada", http.StatusInternalServerError)
		return
	}
	_, _ = a.db.Exec(`DELETE FROM social_oauth_pending WHERE state=?`, selectionState)
	http.Redirect(w, r, a.cfg.BaseURL+"/app.html?social_oauth=success&platform="+url.QueryEscape(p), http.StatusSeeOther)
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
	if json.NewDecoder(r.Body).Decode(&q) != nil || q.ID == 0 {
		http.Error(w, "id requerido", 400)
		return
	}
	ensureSocialProviderSchema(a.db)
	var p, mode, enc, cfgj string
	err = a.db.QueryRow(`SELECT platform,provider_mode,encrypted_credentials,config_json FROM social_connections WHERE id=? AND tenant_id=?`, q.ID, tid).Scan(&p, &mode, &enc, &cfgj)
	if err != nil {
		http.Error(w, "conexiÃ³n no encontrada", 404)
		return
	}
	if mode == "sandbox" {
		writeJSON(w, map[string]any{"ok": true, "platform": p, "status": "connected", "message": "ConexiÃ³n sandbox activa"})
		return
	}
	var tok socialToken
	if json.Unmarshal([]byte(decryptLocal(enc, a.cfg.ChannelEncryptionKey)), &tok) != nil || tok.AccessToken == "" {
		a.markSocialConnectionTest(q.ID, tid, false, "No fue posible descifrar las credenciales. Verifica CHANNEL_ENCRYPTION_KEY.")
		http.Error(w, "credenciales invÃ¡lidas", 409)
		return
	}
	var cfg socialProviderConfig
	_ = json.Unmarshal([]byte(cfgj), &cfg)
	if p == "youtube" || p == "tiktok" {
		tok, err = a.refreshSocialTokenIfNeeded(r.Context(), q.ID, tid, p, tok)
		if err != nil {
			a.markSocialConnectionTest(q.ID, tid, false, err.Error())
			http.Error(w, err.Error(), 409)
			return
		}
	}
	var detail string
	switch p {
	case "facebook":
		var out map[string]any
		err = apiJSON(r.Context(), "GET", "https://graph.facebook.com/"+a.cfg.MetaGraphVersion+"/"+url.PathEscape(cfg.PageID)+"?fields=id,name&access_token="+url.QueryEscape(tok.AccessToken), "", nil, &out)
		detail = fmt.Sprint(out["name"])
	case "instagram":
		if cfg.InstagramAuthMethod == "instagram_login" {
			detail, err = instagramB103DProfile(r.Context(), tok.AccessToken, cfg.InstagramID)
		} else {
			var out map[string]any
			err = apiJSON(r.Context(), "GET", "https://graph.facebook.com/"+a.cfg.MetaGraphVersion+"/"+url.PathEscape(cfg.InstagramID)+"?fields=id,username&access_token="+url.QueryEscape(tok.AccessToken), "", nil, &out)
			detail = fmt.Sprint(out["username"])
		}
	case "linkedin":
		var out map[string]any
		err = apiJSON(r.Context(), "GET", "https://api.linkedin.com/v2/userinfo", tok.AccessToken, nil, &out)
		detail = fmt.Sprint(out["name"])
	case "telegram":
		var out map[string]any
		err = apiJSON(r.Context(), "GET", "https://api.telegram.org/bot"+tok.AccessToken+"/getChat?chat_id="+url.QueryEscape(cfg.ChatID), "", nil, &out)
		detail = cfg.ChatID
	case "tiktok":
		var out map[string]any
		err = apiJSON(r.Context(), "GET", "https://open.tiktokapis.com/v2/user/info/?fields=open_id,display_name", tok.AccessToken, nil, &out)
		detail = cfg.OpenID
	case "youtube":
		var out map[string]any
		err = apiJSON(r.Context(), "GET", "https://www.googleapis.com/youtube/v3/channels?part=snippet&mine=true", tok.AccessToken, nil, &out)
		detail = cfg.ChannelID
	default:
		err = errors.New("plataforma no soportada")
	}
	if err != nil {
		a.markSocialConnectionTest(q.ID, tid, false, err.Error())
		http.Error(w, err.Error(), 502)
		return
	}
	a.markSocialConnectionTest(q.ID, tid, true, "")
	writeJSON(w, map[string]any{"ok": true, "platform": p, "status": "connected", "detail": detail, "message": "ConexiÃ³n oficial verificada correctamente"})
}

func (a *App) markSocialConnectionTest(id, tid int64, ok bool, msg string) {
	now := time.Now().UTC().Format(time.RFC3339)
	status := "connected"
	if !ok {
		status = "attention"
	}
	_, _ = a.db.Exec(`UPDATE social_connections SET status=?,last_error=?,last_sync_at=?,updated_at=? WHERE id=? AND tenant_id=?`, status, msg, now, now, id, tid)
}

func (a *App) refreshSocialTokenIfNeeded(ctx context.Context, connectionID, tid int64, p string, tok socialToken) (socialToken, error) {
	if tok.RefreshToken == "" {
		return tok, nil
	}
	var endpoint string
	vals := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {tok.RefreshToken}}
	switch p {
	case "youtube":
		endpoint = "https://oauth2.googleapis.com/token"
		vals.Set("client_id", a.cfg.GoogleClientID)
		vals.Set("client_secret", a.cfg.GoogleClientSecret)
	case "tiktok":
		endpoint = "https://open.tiktokapis.com/v2/oauth/token/"
		vals.Set("client_key", a.cfg.TikTokClientKey)
		vals.Set("client_secret", a.cfg.TikTokClientSecret)
	default:
		return tok, nil
	}
	req, _ := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(vals.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return tok, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode/100 != 2 {
		return tok, fmt.Errorf("no fue posible renovar el token de %s: %s", p, string(b))
	}
	var nt socialToken
	if json.Unmarshal(b, &nt) != nil || nt.AccessToken == "" {
		return tok, errors.New("el proveedor no devolviÃ³ un token renovado")
	}
	if nt.RefreshToken == "" {
		nt.RefreshToken = tok.RefreshToken
	}
	tb, _ := json.Marshal(nt)
	exp := ""
	if nt.ExpiresIn > 0 {
		exp = time.Now().UTC().Add(time.Duration(nt.ExpiresIn) * time.Second).Format(time.RFC3339)
	}
	_, _ = a.db.Exec(`UPDATE social_connections SET encrypted_credentials=?,token_expires_at=?,refresh_status='ok',last_error='',updated_at=? WHERE id=? AND tenant_id=?`, encryptLocal(string(tb), a.cfg.ChannelEncryptionKey), exp, time.Now().UTC().Format(time.RFC3339), connectionID, tid)
	return nt, nil
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

// metaHasScope evita coincidencias parciales entre nombres de permisos.
func metaHasScope(scopes, required string) bool {
	for _, scope := range strings.Split(scopes, ",") {
		if strings.TrimSpace(scope) == required {
			return true
		}
	}
	return false
}

func (a *App) publishSocialPost(ctx context.Context, tid, id int64) (map[string]any, error) {
	ensureSocialProviderSchema(a.db)
	_ = a.ensureSocialTrackingURLV24(tid, id)
	var p SocialPost
	var mode, cstatus, enc, cfgj, connScopes, master, groupUpdated string
	err := a.db.QueryRow(`SELECT p.id,p.tenant_id,p.group_id,p.campaign_id,p.connection_id,p.platform,p.format,p.title,p.caption,CASE WHEN TRIM(COALESCE(p.tracking_url,''))<>'' THEN p.tracking_url ELSE p.link_url END,p.media_json,p.status,COALESCE(g.master_content,p.caption),COALESCE(g.updated_at,''),COALESCE(c.provider_mode,''),COALESCE(c.status,''),COALESCE(c.encrypted_credentials,''),COALESCE(c.config_json,'{}'),COALESCE(c.scopes,'') FROM social_posts p LEFT JOIN social_post_groups g ON g.id=p.group_id AND g.tenant_id=p.tenant_id LEFT JOIN social_connections c ON c.id=p.connection_id AND c.tenant_id=p.tenant_id WHERE p.id=? AND p.tenant_id=?`, id, tid).Scan(&p.ID, &p.TenantID, &p.GroupID, &p.CampaignID, &p.ConnectionID, &p.Platform, &p.Format, &p.Title, &p.Caption, &p.LinkURL, &p.MediaJSON, &p.Status, &master, &groupUpdated, &mode, &cstatus, &enc, &cfgj, &connScopes)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	gov := a.loadSocialGovernanceV231(tid)
	platforms := []string{p.Platform}
	if p.GroupID > 0 {
		platforms = platforms[:0]
		rows, _ := a.db.Query(`SELECT DISTINCT platform FROM social_posts WHERE tenant_id=? AND group_id=?`, tid, p.GroupID)
		if rows != nil {
			for rows.Next() {
				var platform string
				_ = rows.Scan(&platform)
				if validSocialPlatform(platform) {
					platforms = append(platforms, platform)
				}
			}
			rows.Close()
		}
	}
	decision := a.socialGovernanceDecisionV231(tid, p.CampaignID, platforms, master, "publish")
	if decision.RequireApproval && p.GroupID > 0 {
		var approved, pending int
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM social_approvals WHERE tenant_id=? AND group_id=? AND status='approved' AND reviewed_at>=?`, tid, p.GroupID, groupUpdated).Scan(&approved)
		if approved == 0 {
			_ = a.db.QueryRow(`SELECT COUNT(*) FROM social_approvals WHERE tenant_id=? AND group_id=? AND status='pending'`, tid, p.GroupID).Scan(&pending)
			_, _ = a.db.Exec(`UPDATE social_post_groups SET status='pending_approval',updated_at=? WHERE id=? AND tenant_id=?`, now, p.GroupID, tid)
			_, _ = a.db.Exec(`UPDATE social_posts SET status='pending_approval',updated_at=? WHERE group_id=? AND tenant_id=? AND status<>'published'`, now, p.GroupID, tid)
			if pending == 0 {
				_, _ = a.db.Exec(`INSERT INTO social_approvals(tenant_id,group_id,requested_by,desired_action,desired_scheduled_at,request_note,status,requested_at) VALUES(?,?,?,?,?,?,?,?)`, tid, p.GroupID, 0, "publish", "", "Generado por el publisher al aplicar gobernanza V23.1", "pending", now)
			}
			a.auditSocialGovernanceV231(tid, p.GroupID, p.CampaignID, 0, "publish", platforms, decision)
			return nil, errors.New("La polÃ­tica de Social Hub requiere aprobaciÃ³n antes de publicar")
		}
	}
	if gov.EmergencyPause {
		_, _ = a.db.Exec(`UPDATE social_posts SET status='governance_hold',error_message='GOVERNANCE_PAUSE',updated_at=? WHERE id=? AND tenant_id=?`, now, id, tid)
		return nil, errors.New("Autopilot estÃ¡ pausado globalmente para esta empresa")
	}
	if kw := containsAnyKeywordV231(p.Caption, gov.BlockedKeywords); kw != "" {
		_, _ = a.db.Exec(`UPDATE social_posts SET status='governance_hold',error_message=?,updated_at=? WHERE id=? AND tenant_id=?`, "GOVERNANCE_BLOCK:"+kw, now, id, tid)
		return nil, errors.New("PublicaciÃ³n retenida por una regla de contenido bloqueado: " + kw)
	}
	if p.ConnectionID == 0 || cstatus != "connected" {
		return nil, errors.New("conecta una cuenta activa para esta red")
	}
	// No ejecutar ni reintentar tareas Meta sin los permisos de publicacion.
	if mode != "sandbox" && (p.Platform == "facebook" || p.Platform == "instagram") {
		required := "pages_manage_posts"
		if p.Platform == "instagram" {
			required = "instagram_content_publish"
		}
		if !metaHasScope(connScopes, required) {
			msg := "La conexion requiere el permiso " + required + " antes de publicar; autorizalo en Meta"
			_, _ = a.db.Exec(`UPDATE social_posts SET status='failed',error_message=?,updated_at=? WHERE id=? AND tenant_id=? AND status<>'published'`, msg, now, id, tid)
			return nil, errors.New(msg)
		}
	}
	if consumptionV2723Enabled() {
		claim, claimErr := a.db.Exec(`UPDATE social_posts SET status='publishing',updated_at=? WHERE id=? AND tenant_id=? AND status NOT IN ('published','publishing')`, now, id, tid)
		if claimErr != nil {
			return nil, claimErr
		}
		claimed, _ := claim.RowsAffected()
		if claimed != 1 {
			return nil, errors.New("publicaciÃ³n ya iniciada o publicada; revisar antes de reintentar")
		}
	} else {
		_, _ = a.db.Exec(`UPDATE social_posts SET status='publishing',updated_at=? WHERE id=? AND tenant_id=?`, now, id, tid)
	}
	if mode == "sandbox" {
		external := fmt.Sprintf("sandbox-%s-%d", p.Platform, id)
		u := fmt.Sprintf("https://social.worktic.local/%s/%s", p.Platform, external)
		_, _ = a.db.Exec(`UPDATE social_posts SET status='published',published_at=?,external_post_id=?,published_url=?,error_message='',updated_at=? WHERE id=? AND tenant_id=?`, now, external, u, now, id, tid)
		return map[string]any{"id": external, "url": u, "sandbox": true}, nil
	}
	var tok socialToken
	if json.Unmarshal([]byte(decryptLocal(enc, a.cfg.ChannelEncryptionKey)), &tok) != nil || tok.AccessToken == "" {
		return nil, errors.New("credenciales invÃ¡lidas")
	}
	var cfg socialProviderConfig
	_ = json.Unmarshal([]byte(cfgj), &cfg)
	if p.Platform == "youtube" || p.Platform == "tiktok" {
		tok, err = a.refreshSocialTokenIfNeeded(ctx, p.ConnectionID, tid, p.Platform, tok)
		if err != nil {
			return nil, err
		}
	}
	media := ""
	mediaURLs := []string{}
	var arr []map[string]string
	_ = json.Unmarshal([]byte(p.MediaJSON), &arr)
	for _, m := range arr {
		if u := strings.TrimSpace(m["url"]); u != "" {
			mediaURLs = append(mediaURLs, u)
		}
	}
	if len(mediaURLs) > 0 {
		media = mediaURLs[0]
	}
	// B7: Meta recibe una URL HTTPS publica, no la ruta relativa del editor.
	// Normalizar todos los elementos, incluidos los carruseles de Facebook.
	if (p.Platform == "facebook" || p.Platform == "instagram") && len(mediaURLs) > 0 {
		for i, raw := range mediaURLs {
			publicURL, mediaErr := socialB7CanonicalMediaURL(a.cfg.BaseURL, raw)
			if mediaErr != nil {
				permanentErr := fmt.Errorf("Facebook: recurso multimedia invalido: %w", mediaErr)
				var attemptNo int
				_ = a.db.QueryRow(`SELECT COUNT(*)+1 FROM social_publish_attempts WHERE tenant_id=? AND post_id=?`, tid, id).Scan(&attemptNo)
				_, _ = a.db.Exec(`INSERT INTO social_publish_attempts(tenant_id,post_id,attempt_no,status,error_message,created_at) VALUES(?,?,?,?,?,?)`, tid, id, attemptNo, "failed", permanentErr.Error(), now)
				_, _ = a.db.Exec(`UPDATE social_posts SET status='failed',error_message=?,updated_at=? WHERE id=? AND tenant_id=? AND status<>'published'`, permanentErr.Error(), now, id, tid)
				return nil, permanentErr
			}
			mediaURLs[i] = publicURL
		}
		media = mediaURLs[0]
	}
	var result map[string]any
	switch p.Platform {
	case "facebook":
		if p.Format == "carousel" && len(mediaURLs) > 1 {
			result, err = a.publishFacebookCarousel(ctx, tok.AccessToken, cfg.PageID, p.Caption, mediaURLs)
		} else {
			result, err = a.publishFacebook(ctx, tok.AccessToken, cfg.PageID, p.Caption, p.LinkURL, media, p.Format)
		}
	case "instagram":
		if p.Format == "carousel" && len(mediaURLs) > 1 {
			result, err = a.publishInstagramCarousel(ctx, tok.AccessToken, cfg.InstagramID, p.Caption, mediaURLs)
		} else {
			result, err = a.publishInstagram(ctx, tok.AccessToken, cfg.InstagramID, p.Caption, media, p.Format)
		}
	case "linkedin":
		if p.Format == "carousel" && len(mediaURLs) > 1 {
			err = errors.New("LinkedIn carousel requiere un flujo de documentos/media distinto y no se publica automÃ¡ticamente en V23")
		} else {
			result, err = a.publishLinkedIn(ctx, tok.AccessToken, cfg.AuthorURN, p.Caption, p.LinkURL)
		}
	case "telegram":
		if p.Format == "carousel" && len(mediaURLs) > 1 {
			result, err = a.publishTelegramAlbum(ctx, tok.AccessToken, cfg.ChatID, p.Caption, mediaURLs)
		} else {
			result, err = a.publishTelegram(ctx, tok.AccessToken, cfg.ChatID, p.Caption, media, p.Format)
		}
	case "tiktok":
		result, err = a.publishTikTok(ctx, tok.AccessToken, p.Caption, mediaURLs, p.Format)
	case "youtube":
		if len(mediaURLs) > 1 {
			err = errors.New("YouTube admite un video por publicaciÃ³n")
		} else {
			result, err = a.publishYouTube(ctx, tok.AccessToken, p.Title, p.Caption, media)
		}
	default:
		err = errors.New("plataforma no soportada")
	}
	attempt := 1
	_ = a.db.QueryRow(`SELECT COUNT(*)+1 FROM social_publish_attempts WHERE tenant_id=? AND post_id=?`, tid, id).Scan(&attempt)
	rb, _ := json.Marshal(result)
	if err != nil {
		_, _ = a.db.Exec(`INSERT INTO social_publish_attempts(tenant_id,post_id,attempt_no,status,error_message,created_at) VALUES(?,?,?,?,?,?)`, tid, id, attempt, "failed", err.Error(), now)
		nextStatus := "failed"
		if attempt < 4 && !socialB7IsPermanentMediaURLFailure(err) {
			nextStatus = "retrying"
		}
		_, _ = a.db.Exec(`UPDATE social_posts SET status=?,error_message=?,updated_at=?,scheduled_at=? WHERE id=? AND tenant_id=?`, nextStatus, err.Error(), now, time.Now().UTC().Add(time.Duration(attempt*5)*time.Minute).Format(time.RFC3339), id, tid)
		return nil, err
	}
	external, _ := result["id"].(string)
	pu, _ := result["url"].(string)
	_, _ = a.db.Exec(`INSERT INTO social_publish_attempts(tenant_id,post_id,attempt_no,status,provider_response,created_at) VALUES(?,?,?,?,?,?)`, tid, id, attempt, "published", string(rb), now)
	_, _ = a.db.Exec(`UPDATE social_posts SET status='published',published_at=?,external_post_id=?,published_url=?,error_message='',updated_at=? WHERE id=? AND tenant_id=?`, now, external, pu, now, id, tid)
	// Observe a successful external publication, not an advertising spend.
	ref := external
	if ref == "" {
		ref = fmt.Sprintf("worktic-post-%d", id)
	}
	_ = a.recordExternalUsageV2723(tid, "social_"+p.Platform, "publish", ref, "accepted")
	return result, nil
}

func (a *App) publishFacebook(ctx context.Context, token, pageID, caption, link, media, format string) (map[string]any, error) {
	ep := "https://graph.facebook.com/" + a.cfg.MetaGraphVersion + "/" + pageID + "/feed"
	vals := url.Values{"message": {caption}, "access_token": {token}}
	if link != "" {
		vals.Set("link", link)
	}
	if media != "" {
		isVideo := format == "video" || format == "reel" || strings.Contains(strings.ToLower(media), ".mp4") || strings.Contains(strings.ToLower(media), ".mov") || strings.Contains(strings.ToLower(media), ".webm")
		if isVideo {
			ep = "https://graph.facebook.com/" + a.cfg.MetaGraphVersion + "/" + pageID + "/videos"
			vals.Set("file_url", media)
			vals.Set("description", caption)
			vals.Del("message")
		} else {
			ep = "https://graph.facebook.com/" + a.cfg.MetaGraphVersion + "/" + pageID + "/photos"
			vals.Set("url", media)
			vals.Set("caption", caption)
			vals.Del("message")
		}
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
		return nil, errors.New("Instagram requiere URL pÃºblica de imagen o video")
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
func (a *App) publishFacebookCarousel(ctx context.Context, token, pageID, caption string, media []string) (map[string]any, error) {
	if len(media) < 2 {
		return nil, errors.New("Facebook carrusel requiere al menos 2 imÃ¡genes")
	}
	if len(media) > 10 {
		media = media[:10]
	}
	ids := []string{}
	for _, u := range media {
		vals := url.Values{"url": {u}, "published": {"false"}, "access_token": {token}}
		req, _ := http.NewRequestWithContext(ctx, "POST", "https://graph.facebook.com/"+a.cfg.MetaGraphVersion+"/"+pageID+"/photos", strings.NewReader(vals.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode/100 != 2 {
			return nil, fmt.Errorf("Facebook carousel asset: %s", b)
		}
		var out map[string]any
		_ = json.Unmarshal(b, &out)
		id := fmt.Sprint(out["id"])
		if id != "" {
			ids = append(ids, id)
		}
	}
	if len(ids) < 2 {
		return nil, errors.New("Facebook no pudo preparar los recursos del carrusel")
	}
	vals := url.Values{"message": {caption}, "access_token": {token}}
	for i, id := range ids {
		b, _ := json.Marshal(map[string]string{"media_fbid": id})
		vals.Set(fmt.Sprintf("attached_media[%d]", i), string(b))
	}
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://graph.facebook.com/"+a.cfg.MetaGraphVersion+"/"+pageID+"/feed", strings.NewReader(vals.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("Facebook carousel: %s", b)
	}
	var out map[string]any
	_ = json.Unmarshal(b, &out)
	id := fmt.Sprint(out["id"])
	return map[string]any{"id": id, "url": "https://www.facebook.com/" + id}, nil
}

func (a *App) publishInstagramCarousel(ctx context.Context, token, igID, caption string, media []string) (map[string]any, error) {
	if len(media) < 2 {
		return nil, errors.New("Instagram carrusel requiere al menos 2 recursos")
	}
	if len(media) > 10 {
		media = media[:10]
	}
	children := []string{}
	for _, u := range media {
		vals := url.Values{"is_carousel_item": {"true"}, "access_token": {token}}
		l := strings.ToLower(strings.Split(u, "?")[0])
		if strings.HasSuffix(l, ".mp4") || strings.HasSuffix(l, ".mov") || strings.HasSuffix(l, ".webm") {
			vals.Set("media_type", "VIDEO")
			vals.Set("video_url", u)
		} else {
			vals.Set("image_url", u)
		}
		req, _ := http.NewRequestWithContext(ctx, "POST", "https://graph.facebook.com/"+a.cfg.MetaGraphVersion+"/"+igID+"/media", strings.NewReader(vals.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode/100 != 2 {
			return nil, fmt.Errorf("Instagram carousel asset: %s", b)
		}
		var out map[string]any
		_ = json.Unmarshal(b, &out)
		id := fmt.Sprint(out["id"])
		if id != "" {
			children = append(children, id)
		}
	}
	if len(children) < 2 {
		return nil, errors.New("Instagram no pudo preparar el carrusel")
	}
	time.Sleep(2 * time.Second)
	vals := url.Values{"media_type": {"CAROUSEL"}, "children": {strings.Join(children, ",")}, "caption": {caption}, "access_token": {token}}
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://graph.facebook.com/"+a.cfg.MetaGraphVersion+"/"+igID+"/media", strings.NewReader(vals.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("Instagram carousel container: %s", b)
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
		return nil, fmt.Errorf("Instagram carousel publish: %s", b)
	}
	var out map[string]any
	_ = json.Unmarshal(b, &out)
	id := fmt.Sprint(out["id"])
	return map[string]any{"id": id, "url": "https://www.instagram.com/"}, nil
}

func (a *App) publishTelegramAlbum(ctx context.Context, token, chatID, caption string, media []string) (map[string]any, error) {
	if len(media) < 2 {
		return nil, errors.New("Telegram Ã¡lbum requiere al menos 2 recursos")
	}
	if len(media) > 10 {
		media = media[:10]
	}
	items := []map[string]any{}
	for i, u := range media {
		l := strings.ToLower(strings.Split(u, "?")[0])
		typ := "photo"
		if strings.HasSuffix(l, ".mp4") || strings.HasSuffix(l, ".mov") || strings.HasSuffix(l, ".webm") {
			typ = "video"
		}
		m := map[string]any{"type": typ, "media": u}
		if i == 0 {
			m["caption"] = caption
		}
		items = append(items, m)
	}
	payload := map[string]any{"chat_id": chatID, "media": items}
	b, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://api.telegram.org/bot"+token+"/sendMediaGroup", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	rb, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("Telegram album: %s", rb)
	}
	var out struct {
		Result []struct {
			MessageID int `json:"message_id"`
		} `json:"result"`
	}
	_ = json.Unmarshal(rb, &out)
	id := ""
	if len(out.Result) > 0 {
		id = strconv.Itoa(out.Result[0].MessageID)
	}
	return map[string]any{"id": id, "url": ""}, nil
}

func (a *App) publishLinkedIn(ctx context.Context, token, author, caption, link string) (map[string]any, error) {
	content := map[string]any{"author": author, "commentary": caption, "visibility": "PUBLIC", "distribution": map[string]any{"feedDistribution": "MAIN_FEED", "targetEntities": []any{}, "thirdPartyDistributionChannels": []any{}}, "lifecycleState": "PUBLISHED", "isReshareDisabledByAuthor": false}
	if link != "" {
		content["content"] = map[string]any{"article": map[string]any{"source": link, "title": "Ver mÃ¡s"}}
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
func (a *App) publishTelegram(ctx context.Context, token, chatID, caption, media, format string) (map[string]any, error) {
	if token == "" || chatID == "" {
		return nil, errors.New("Telegram requiere token de bot y chat/canal")
	}
	method := "sendMessage"
	vals := url.Values{"chat_id": {chatID}, "text": {caption}, "parse_mode": {"HTML"}}
	if media != "" {
		isVideo := format == "video" || format == "reel" || strings.Contains(strings.ToLower(media), ".mp4") || strings.Contains(strings.ToLower(media), ".mov") || strings.Contains(strings.ToLower(media), ".webm")
		if isVideo {
			method = "sendVideo"
			vals = url.Values{"chat_id": {chatID}, "video": {media}, "caption": {caption}}
		} else {
			method = "sendPhoto"
			vals = url.Values{"chat_id": {chatID}, "photo": {media}, "caption": {caption}}
		}
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
func (a *App) publishTikTok(ctx context.Context, token, caption string, media []string, format string) (map[string]any, error) {
	if len(media) == 0 {
		return nil, errors.New("TikTok requiere URL de video o foto en dominio verificado")
	}
	// TikTok exige consultar creator_info antes del Direct Post para conocer
	// opciones vigentes (privacidad, lÃ­mites y capacidades) del creador.
	var creator map[string]any
	if err := apiJSON(ctx, "POST", "https://open.tiktokapis.com/v2/post/publish/creator_info/query/", token, map[string]any{}, &creator); err != nil {
		return nil, fmt.Errorf("TikTok creator info: %w", err)
	}
	privacy := "SELF_ONLY"
	if d, ok := creator["data"].(map[string]any); ok {
		if opts, ok := d["privacy_level_options"].([]any); ok && len(opts) > 0 {
			privacy = fmt.Sprint(opts[0])
			for _, o := range opts {
				if fmt.Sprint(o) == "PUBLIC_TO_EVERYONE" {
					privacy = "PUBLIC_TO_EVERYONE"
					break
				}
			}
		}
	}
	isPhoto := format == "carousel" || format == "post"
	if isPhoto {
		if len(media) > 10 {
			media = media[:10]
		}
		body := map[string]any{
			"post_info":   map[string]any{"title": caption, "description": caption, "privacy_level": privacy, "disable_comment": false},
			"source_info": map[string]any{"source": "PULL_FROM_URL", "photo_images": media, "photo_cover_index": 0},
			"post_mode":   "DIRECT_POST", "media_type": "PHOTO",
		}
		var out map[string]any
		if err := apiJSON(ctx, "POST", "https://open.tiktokapis.com/v2/post/publish/content/init/", token, body, &out); err != nil {
			return nil, err
		}
		id := ""
		if d, ok := out["data"].(map[string]any); ok {
			id = fmt.Sprint(d["publish_id"])
		}
		return map[string]any{"id": id, "url": ""}, nil
	}
	body := map[string]any{
		"post_info":   map[string]any{"title": caption, "privacy_level": privacy, "disable_duet": false, "disable_comment": false, "disable_stitch": false},
		"source_info": map[string]any{"source": "PULL_FROM_URL", "video_url": media[0]},
	}
	var out map[string]any
	if err := apiJSON(ctx, "POST", "https://open.tiktokapis.com/v2/post/publish/video/init/", token, body, &out); err != nil {
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
		return nil, errors.New("YouTube requiere URL pÃºblica de video")
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
		return nil, errors.New("YouTube no devolviÃ³ URL de carga")
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
