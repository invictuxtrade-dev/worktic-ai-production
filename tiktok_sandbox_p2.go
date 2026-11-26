package main

// TikTok P2 Sandbox: isolated and explicitly opt-in. This file never writes
// to social_connections or uses the Production TikTok client credentials.
import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	tiktokSBCallbackURL = "https://www.workticai.com/api/social/oauth/callback/tiktok"
	tiktokSBCookieName  = "worktic_tiktok_p2_sandbox_state"
	tiktokSBCookiePath  = "/api/social/oauth/callback/tiktok"
	tiktokSBScopes      = "user.info.basic,video.upload,video.publish"
)

var tiktokSBIdempotencyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{12,72}$`)

func tiktokSBEnabled() bool { return strings.TrimSpace(os.Getenv("WORKTIC_TIKTOK_SB_ENABLED")) == "1" }
func tiktokSBSubmitEnabled() bool {
	return tiktokSBEnabled() && strings.TrimSpace(os.Getenv("WORKTIC_TIKTOK_SB_SUBMIT_ENABLED")) == "1"
}
func tiktokSBCredentials() (string, string, error) {
	key := strings.TrimSpace(os.Getenv("WORKTIC_TIKTOK_SB_CLIENT_KEY"))
	secret := strings.TrimSpace(os.Getenv("WORKTIC_TIKTOK_SB_CLIENT_SECRET"))
	if key == "" || secret == "" {
		return "", "", errors.New("Faltan credenciales del Sandbox de TikTok")
	}
	return key, secret, nil
}
func tiktokSBAllowedTenant() int64 {
	n, err := strconv.ParseInt(strings.TrimSpace(os.Getenv("WORKTIC_TIKTOK_SB_TENANT_ID")), 10, 64)
	if err != nil || n <= 0 {
		return 0
	}
	return n
}
func tiktokSBReady() error {
	if !tiktokSBEnabled() {
		return errors.New("El laboratorio de TikTok esta desactivado")
	}
	if tiktokSBAllowedTenant() == 0 {
		return errors.New("WORKTIC_TIKTOK_SB_TENANT_ID no esta configurado")
	}
	if _, _, err := tiktokSBCredentials(); err != nil {
		return err
	}
	if len(strings.TrimSpace(os.Getenv("CHANNEL_ENCRYPTION_KEY"))) < 32 {
		return errors.New("CHANNEL_ENCRYPTION_KEY no esta configurada correctamente")
	}
	return nil
}
func tiktokSBCheckOrigin(r *http.Request, action string) bool {
	if r.Method != http.MethodPost || r.Header.Get("X-Worktic-TikTok-Sandbox") != action {
		return false
	}
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	u, err := url.Parse(origin)
	// No CORS, HTTPS only, no alternative Origin. Cannot be sent by a normal HTML form.
	return err == nil && u.Scheme == "https" && u.Host == "www.workticai.com" && u.User == nil && u.Path == "" && u.RawQuery == "" && u.Fragment == ""
}
func tiktokSBCookie(state string, ttl int) *http.Cookie {
	return &http.Cookie{Name: tiktokSBCookieName, Value: state, Path: tiktokSBCookiePath, MaxAge: ttl, Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode}
}
func tiktokSBSafeState(state string) bool {
	return strings.HasPrefix(state, "tsb_") && len(state) >= 40 && len(state) <= 120
}
func tiktokSBStateMatches(r *http.Request, state string) bool {
	if !tiktokSBSafeState(state) {
		return false
	}
	cookie, err := r.Cookie(tiktokSBCookieName)
	return err == nil && subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(state)) == 1
}
func (a *App) tiktokSBSchema() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS worktic_tiktok_sb_oauth (state TEXT PRIMARY KEY, tenant_id BIGINT NOT NULL, user_id BIGINT NOT NULL, expires_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS worktic_tiktok_sb_tokens (tenant_id BIGINT PRIMARY KEY, open_id TEXT NOT NULL, display_name TEXT NOT NULL, scopes TEXT NOT NULL, token_cipher TEXT NOT NULL, expires_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS worktic_tiktok_sb_submissions (tenant_id BIGINT NOT NULL, request_id TEXT NOT NULL, publish_id TEXT NOT NULL DEFAULT '', video_url TEXT NOT NULL, mode TEXT NOT NULL, status TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL, PRIMARY KEY(tenant_id, request_id))`,
	}
	for _, stmt := range stmts {
		if _, err := a.db.Exec(stmt); err != nil {
			return err
		}
	}
	_, _ = a.db.Exec(`DELETE FROM worktic_tiktok_sb_oauth WHERE expires_at<?`, time.Now().UTC().Format(time.RFC3339))
	return nil
}
func tiktokSBReplyError(w http.ResponseWriter, status int, reason string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": reason})
}
func (a *App) tiktokSBAuthorizeStart(w http.ResponseWriter, r *http.Request, tenantID, userID int64) {
	key, _, err := tiktokSBCredentials()
	if err != nil {
		tiktokSBReplyError(w, 409, err.Error())
		return
	}
	if err := a.tiktokSBSchema(); err != nil {
		tiktokSBReplyError(w, 500, "No se pudo inicializar la base de pruebas")
		return
	}
	state := "tsb_" + randomState()
	expires := time.Now().UTC().Add(10 * time.Minute).Format(time.RFC3339)
	if _, err := a.db.Exec(`INSERT INTO worktic_tiktok_sb_oauth(state,tenant_id,user_id,expires_at) VALUES(?,?,?,?)`, state, tenantID, userID, expires); err != nil {
		tiktokSBReplyError(w, 500, "No se pudo crear la autorizacion de pruebas")
		return
	}
	q := url.Values{"client_key": {key}, "response_type": {"code"}, "scope": {tiktokSBScopes}, "redirect_uri": {tiktokSBCallbackURL}, "state": {state}, "disable_auto_auth": {"1"}}
	http.SetCookie(w, tiktokSBCookie(state, 600))
	writeJSON(w, map[string]any{"authorization_url": "https://www.tiktok.com/v2/auth/authorize/?" + q.Encode()})
}

// TikTok OAuth uses the same registered callback URL for Live and Sandbox.
// This exact-path mux router dispatches only our distinct tsb_ states here;
// ordinary Production callbacks follow the original unmodified handler.
func (a *App) tiktokSBOAuthRouter(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	if tiktokSBSafeState(state) || strings.HasPrefix(state, "tsb_") {
		a.tiktokSBOAuthCallback(w, r)
		return
	}
	a.socialOAuthCallbackHandler(w, r)
}
func (a *App) tiktokSBOAuthCallback(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet {
		http.Error(w, "Metodo no permitido", 405)
		return
	}
	if !tiktokSBEnabled() || tiktokSBAllowedTenant() == 0 {
		http.Error(w, "Laboratorio desactivado", 403)
		return
	}
	state := r.URL.Query().Get("state")
	if !tiktokSBStateMatches(r, state) {
		http.Error(w, "La autorizacion no coincide con el navegador que la inicio", 403)
		return
	}
	if err := a.tiktokSBSchema(); err != nil {
		http.Error(w, "Error en almacenamiento de pruebas", 500)
		return
	}
	var tenantID, userID int64
	err := a.db.QueryRow(`DELETE FROM worktic_tiktok_sb_oauth WHERE state=? AND expires_at>? RETURNING tenant_id,user_id`, state, time.Now().UTC().Format(time.RFC3339)).Scan(&tenantID, &userID)
	http.SetCookie(w, tiktokSBCookie("", -1))
	if err != nil || tenantID != tiktokSBAllowedTenant() || userID <= 0 {
		http.Error(w, "Autorizacion vencida o utilizada", 400)
		return
	}
	if r.URL.Query().Get("error") != "" {
		http.Redirect(w, r, "/tiktok-sandbox-p2-test.html?oauth=denied", http.StatusSeeOther)
		return
	}
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if code == "" || len(code) > 4096 {
		http.Error(w, "No se recibio autorizacion TikTok", 400)
		return
	}
	tok, err := tiktokSBTokenExchange(r.Context(), "authorization_code", code)
	if err != nil || !tiktokSBScopesGranted(tok.Scope) {
		http.Redirect(w, r, "/tiktok-sandbox-p2-test.html?oauth=failed", http.StatusSeeOther)
		return
	}
	openID, name, err := tiktokT1FetchProfile(r.Context(), tok.AccessToken, tok.OpenID)
	if err != nil {
		http.Redirect(w, r, "/tiktok-sandbox-p2-test.html?oauth=profile_failed", http.StatusSeeOther)
		return
	}
	tok.OpenID = openID
	raw, err := json.Marshal(tok)
	if err != nil {
		http.Error(w, "Error de serializacion", 500)
		return
	}
	cipher := encryptLocal(string(raw), a.cfg.ChannelEncryptionKey)
	if cipher == "" {
		http.Error(w, "No se pudieron proteger las credenciales", 500)
		return
	}
	now := time.Now().UTC()
	expires := now.Add(time.Duration(tok.ExpiresIn) * time.Second).Format(time.RFC3339)
	_, err = a.db.Exec(`INSERT INTO worktic_tiktok_sb_tokens(tenant_id,open_id,display_name,scopes,token_cipher,expires_at,updated_at) VALUES(?,?,?,?,?,?,?) ON CONFLICT(tenant_id) DO UPDATE SET open_id=excluded.open_id,display_name=excluded.display_name,scopes=excluded.scopes,token_cipher=excluded.token_cipher,expires_at=excluded.expires_at,updated_at=excluded.updated_at`, tenantID, openID, name, tok.Scope, cipher, expires, now.Format(time.RFC3339))
	if err != nil {
		http.Error(w, "No se pudo guardar autorizacion de pruebas", 500)
		return
	}
	http.Redirect(w, r, "/tiktok-sandbox-p2-test.html?oauth=success", http.StatusSeeOther)
}
func tiktokSBScopesGranted(scopes string) bool {
	return tiktokT1HasScope(scopes, "user.info.basic") && tiktokT1HasScope(scopes, "video.upload") && tiktokT1HasScope(scopes, "video.publish")
}
func tiktokSBTokenExchange(ctx context.Context, grant, value string) (socialToken, error) {
	key, secret, err := tiktokSBCredentials()
	if err != nil {
		return socialToken{}, err
	}
	fields := url.Values{"client_key": {key}, "client_secret": {secret}, "grant_type": {grant}}
	if grant == "authorization_code" {
		fields.Set("code", value)
		fields.Set("redirect_uri", tiktokSBCallbackURL)
	} else if grant == "refresh_token" {
		fields.Set("refresh_token", value)
	} else {
		return socialToken{}, errors.New("Tipo de autorizacion invalido")
	}
	req, err := http.NewRequestWithContext(ctx, "POST", "https://open.tiktokapis.com/v2/oauth/token/", strings.NewReader(fields.Encode()))
	if err != nil {
		return socialToken{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return socialToken{}, errors.New("No se pudo contactar TikTok")
	}
	defer resp.Body.Close()
	bytes, err := io.ReadAll(io.LimitReader(resp.Body, 128<<10))
	if err != nil {
		return socialToken{}, errors.New("No se pudo leer respuesta TikTok")
	}
	if resp.StatusCode/100 != 2 {
		return socialToken{}, fmt.Errorf("TikTok OAuth devolvio HTTP %d", resp.StatusCode)
	}
	var token socialToken
	if json.Unmarshal(bytes, &token) != nil || token.AccessToken == "" || token.ExpiresIn <= 0 {
		return socialToken{}, errors.New("TikTok no entrego un token valido")
	}
	return token, nil
}
func (a *App) tiktokSBGetToken(ctx context.Context, tenantID int64, required string) (socialToken, string, error) {
	var cipher, expiry, display, scopes string
	err := a.db.QueryRow(`SELECT token_cipher,expires_at,display_name,scopes FROM worktic_tiktok_sb_tokens WHERE tenant_id=?`, tenantID).Scan(&cipher, &expiry, &display, &scopes)
	if err != nil {
		return socialToken{}, "", errors.New("Autoriza primero la cuenta @workticai en el Sandbox")
	}
	var tok socialToken
	if json.Unmarshal([]byte(decryptLocal(cipher, a.cfg.ChannelEncryptionKey)), &tok) != nil || tok.AccessToken == "" {
		return socialToken{}, "", errors.New("Token de pruebas no disponible")
	}
	if !tiktokT1HasScope(scopes, required) || !tiktokT1HasScope(tok.Scope, required) {
		return socialToken{}, "", errors.New("La cuenta de pruebas no concedio los permisos solicitados")
	}
	expiresAt, parseErr := time.Parse(time.RFC3339, expiry)
	if parseErr == nil && expiresAt.After(time.Now().UTC().Add(5*time.Minute)) {
		return tok, display, nil
	}
	if tok.RefreshToken == "" {
		return socialToken{}, "", errors.New("El Sandbox necesita una nueva autorizacion")
	}
	fresh, err := tiktokSBTokenExchange(ctx, "refresh_token", tok.RefreshToken)
	if err != nil {
		return socialToken{}, "", errors.New("No se pudo renovar el token Sandbox")
	}
	if fresh.RefreshToken == "" {
		fresh.RefreshToken = tok.RefreshToken
	}
	if fresh.Scope == "" {
		fresh.Scope = tok.Scope
	}
	if !tiktokT1HasScope(fresh.Scope, required) {
		return socialToken{}, "", errors.New("El Sandbox ya no tiene el permiso requerido")
	}
	// TikTok may rotate refresh tokens. Never use social_connections or Production credentials.
	raw, _ := json.Marshal(fresh)
	enc := encryptLocal(string(raw), a.cfg.ChannelEncryptionKey)
	if enc == "" {
		return socialToken{}, "", errors.New("No se pudo cifrar el token renovado")
	}
	now := time.Now().UTC()
	_, err = a.db.Exec(`UPDATE worktic_tiktok_sb_tokens SET token_cipher=?,scopes=?,expires_at=?,updated_at=? WHERE tenant_id=?`, enc, fresh.Scope, now.Add(time.Duration(fresh.ExpiresIn)*time.Second).Format(time.RFC3339), now.Format(time.RFC3339), tenantID)
	if err != nil {
		return socialToken{}, "", errors.New("No se pudo actualizar el token renovado")
	}
	return fresh, display, nil
}

type tiktokSBSubmitRequest struct {
	VideoURL     string `json:"video_url"`
	Title        string `json:"title"`
	PrivacyLevel string `json:"privacy_level"`
	Mode         string `json:"mode"`
	RequestID    string `json:"request_id"`
	AIGenerated  bool   `json:"ai_generated"`
	BrandOrganic bool   `json:"brand_organic"`
	Confirmed    bool   `json:"confirmed"`
}

func tiktokSBValidateSubmit(req tiktokSBSubmitRequest) error {
	if !req.Confirmed || !tiktokSBIdempotencyPattern.MatchString(req.RequestID) {
		return errors.New("Confirma explicitamente el envio y usa un identificador unico")
	}
	if req.Mode != "direct" && req.Mode != "inbox" {
		return errors.New("Modalidad no permitida")
	}
	if req.Mode == "direct" && req.PrivacyLevel != "SELF_ONLY" {
		return errors.New("Sandbox solamente permite publicacion privada: Solo yo")
	}
	if len([]rune(strings.TrimSpace(req.Title))) == 0 || len([]rune(req.Title)) > 2200 {
		return errors.New("El titulo debe contener entre 1 y 2200 caracteres")
	}
	if !strings.HasPrefix(req.VideoURL, "https://www.workticai.com/uploads/social/") {
		return errors.New("Selecciona un MP4 autorizado en www.workticai.com")
	}
	return nil
}
func (a *App) tiktokSBAPIHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	tenantID, user, err := a.tenantFor(r)
	if err != nil {
		tiktokSBReplyError(w, 401, "Inicia sesion en Worktic AI")
		return
	}
	perms := socialPermissionsFor(user)
	if !perms.ManageConnections || !perms.Publish || user.ID <= 0 {
		tiktokSBReplyError(w, 403, "Solo un administrador autorizado puede usar el laboratorio")
		return
	}
	if !tiktokSBEnabled() || tenantID != tiktokSBAllowedTenant() {
		tiktokSBReplyError(w, 403, "Laboratorio no habilitado para esta empresa")
		return
	}
	if err := tiktokSBReady(); err != nil {
		tiktokSBReplyError(w, 409, err.Error())
		return
	}
	if err := a.tiktokSBSchema(); err != nil {
		tiktokSBReplyError(w, 500, "No se pudo preparar el laboratorio")
		return
	}
	action := r.URL.Query().Get("action")
	switch {
	case r.Method == "GET" && action == "readiness":
		var name, scopes string
		e := a.db.QueryRow(`SELECT display_name,scopes FROM worktic_tiktok_sb_tokens WHERE tenant_id=?`, tenantID).Scan(&name, &scopes)
		writeJSON(w, map[string]any{"ok": true, "environment": "sandbox", "connected": e == nil, "account_name": name, "granted_scopes": scopes, "can_submit": tiktokSBSubmitEnabled(), "privacy": "SELF_ONLY", "callback": tiktokSBCallbackURL})
	case r.Method == "POST" && action == "authorize":
		if !tiktokSBCheckOrigin(r, "authorize") {
			tiktokSBReplyError(w, 403, "Origen o confirmacion no permitidos")
			return
		}
		a.tiktokSBAuthorizeStart(w, r, tenantID, user.ID)
	case r.Method == "GET" && action == "creator":
		tok, _, e := a.tiktokSBGetToken(r.Context(), tenantID, "video.publish")
		if e != nil {
			tiktokSBReplyError(w, 409, e.Error())
			return
		}
		creator, e := tiktokP2FetchCreator(r.Context(), tok.AccessToken)
		if e != nil {
			tiktokSBReplyError(w, 502, "No se pudieron consultar las opciones del creador Sandbox")
			return
		}
		writeJSON(w, map[string]any{"ok": true, "creator": creator, "private_allowed": tiktokP2AllowedPrivacy(creator.PrivacyLevelOptions, "SELF_ONLY")})
	case r.Method == "POST" && action == "submit":
		if !tiktokSBSubmitEnabled() {
			tiktokSBReplyError(w, 409, "El envio de pruebas permanece desactivado")
			return
		}
		if !tiktokSBCheckOrigin(r, "submit") {
			tiktokSBReplyError(w, 403, "Origen o confirmacion no permitidos")
			return
		}
		var sub tiktokSBSubmitRequest
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
		dec.DisallowUnknownFields()
		if dec.Decode(&sub) != nil {
			tiktokSBReplyError(w, 400, "Solicitud de prueba invalida")
			return
		}
		if e := tiktokSBValidateSubmit(sub); e != nil {
			tiktokSBReplyError(w, 400, e.Error())
			return
		}
		if e := tiktokP2VerifiedVideoURL(sub.VideoURL, "https://www.workticai.com/uploads/social/"); e != nil {
			tiktokSBReplyError(w, 400, "URL de video no autorizada")
			return
		}
		if _, e := a.tiktokP2OwnedVideoCheck(tenantID, sub.VideoURL); e != nil {
			tiktokSBReplyError(w, 400, "El MP4 no pertenece a esta empresa")
			return
		}
		required := "video.publish"
		if sub.Mode == "inbox" {
			required = "video.upload"
		}
		tok, _, e := a.tiktokSBGetToken(r.Context(), tenantID, required)
		if e != nil {
			tiktokSBReplyError(w, 409, e.Error())
			return
		}
		if sub.Mode == "direct" {
			creator, ce := tiktokP2FetchCreator(r.Context(), tok.AccessToken)
			if ce != nil || !tiktokP2AllowedPrivacy(creator.PrivacyLevelOptions, "SELF_ONLY") {
				tiktokSBReplyError(w, 409, "TikTok no permite Solo yo para esta cuenta de pruebas")
				return
			}
		}
		now := time.Now().UTC().Format(time.RFC3339)
		// Reserve request id BEFORE any TikTok call: repeated clicks cannot send duplicates.
		_, e = a.db.Exec(`INSERT INTO worktic_tiktok_sb_submissions(tenant_id,request_id,video_url,mode,status,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`, tenantID, sub.RequestID, sub.VideoURL, sub.Mode, "SUBMITTING", now, now)
		if e != nil {
			tiktokSBReplyError(w, 409, "Esta solicitud ya fue procesada o esta en curso. Consulta estado antes de reintentar")
			return
		}
		endpoint := "https://open.tiktokapis.com/v2/post/publish/video/init/"
		body := map[string]any{"source_info": map[string]any{"source": "PULL_FROM_URL", "video_url": sub.VideoURL}, "post_info": map[string]any{"title": sub.Title, "privacy_level": "SELF_ONLY", "disable_comment": true, "disable_duet": true, "disable_stitch": true, "brand_organic_toggle": sub.BrandOrganic, "brand_content_toggle": false}, "is_aigc": sub.AIGenerated}
		if sub.Mode == "inbox" {
			endpoint = "https://open.tiktokapis.com/v2/post/publish/inbox/video/init/"
			body = map[string]any{"source_info": map[string]any{"source": "PULL_FROM_URL", "video_url": sub.VideoURL}}
		}
		var accepted struct {
			PublishID string `json:"publish_id"`
		}
		e = tiktokP2API(r.Context(), tok.AccessToken, endpoint, body, &accepted)
		status := "PROCESSING"
		if e != nil || accepted.PublishID == "" {
			status = "UNKNOWN_OR_FAILED"
		}
		_, _ = a.db.Exec(`UPDATE worktic_tiktok_sb_submissions SET publish_id=?,status=?,updated_at=? WHERE tenant_id=? AND request_id=?`, accepted.PublishID, status, time.Now().UTC().Format(time.RFC3339), tenantID, sub.RequestID)
		if status != "PROCESSING" {
			tiktokSBReplyError(w, 502, "No se confirmo el envio. No reintentes con otro ID sin comprobar el historial TikTok")
			return
		}
		writeJSON(w, map[string]any{"ok": true, "environment": "sandbox", "status": status, "publish_id": accepted.PublishID, "request_id": sub.RequestID, "message": "TikTok acepto el envio; NO es confirmacion de publicacion"})
	case r.Method == "GET" && action == "status":
		requestID := strings.TrimSpace(r.URL.Query().Get("request_id"))
		if !tiktokSBIdempotencyPattern.MatchString(requestID) {
			tiktokSBReplyError(w, 400, "Identificador invalido")
			return
		}
		var publishID, storedStatus, mode string
		e := a.db.QueryRow(`SELECT publish_id,status,mode FROM worktic_tiktok_sb_submissions WHERE tenant_id=? AND request_id=?`, tenantID, requestID).Scan(&publishID, &storedStatus, &mode)
		if e != nil {
			tiktokSBReplyError(w, 404, "Solicitud no encontrada")
			return
		}
		if publishID == "" {
			writeJSON(w, map[string]any{"status": storedStatus, "message": "No hay confirmacion del proveedor; no reintentar automaticamente"})
			return
		}
		required := "video.publish"
		if mode == "inbox" {
			required = "video.upload"
		}
		tok, _, e := a.tiktokSBGetToken(r.Context(), tenantID, required)
		if e != nil {
			tiktokSBReplyError(w, 409, e.Error())
			return
		}
		var remote struct {
			Status     string `json:"status"`
			FailReason string `json:"fail_reason"`
		}
		e = tiktokP2API(r.Context(), tok.AccessToken, "https://open.tiktokapis.com/v2/post/publish/status/fetch/", map[string]any{"publish_id": publishID}, &remote)
		if e != nil {
			tiktokSBReplyError(w, 502, "No fue posible consultar el estado del envio")
			return
		}
		safeStatus := tiktokP2SafeCode(remote.Status)
		_, _ = a.db.Exec(`UPDATE worktic_tiktok_sb_submissions SET status=?,updated_at=? WHERE tenant_id=? AND request_id=?`, safeStatus, time.Now().UTC().Format(time.RFC3339), tenantID, requestID)
		writeJSON(w, map[string]any{"status": safeStatus, "reason": tiktokP2SafeCode(remote.FailReason), "request_id": requestID, "publish_id": publishID})
	default:
		tiktokSBReplyError(w, 405, "Metodo o accion no permitidos")
	}
}
