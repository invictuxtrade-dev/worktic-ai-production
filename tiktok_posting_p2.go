package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
)

// P2 is OFF by default: enable only AFTER TikTok's production approval.
func tiktokP2Reject(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func tiktokP2Enabled() bool {
	return strings.TrimSpace(os.Getenv("WORKTIC_TIKTOK_POSTING_P2_ENABLED")) == "1"
}

const tiktokP2Scopes = "user.info.basic,video.upload,video.publish"

type tiktokP2Creator struct {
	CreatorUsername         string   `json:"creator_username"`
	CreatorNickname         string   `json:"creator_nickname"`
	PrivacyLevelOptions     []string `json:"privacy_level_options"`
	CommentDisabled         bool     `json:"comment_disabled"`
	DuetDisabled            bool     `json:"duet_disabled"`
	StitchDisabled          bool     `json:"stitch_disabled"`
	MaxVideoPostDurationSec int      `json:"max_video_post_duration_sec"`
}

type tiktokP2Envelope struct {
	Data  json.RawMessage `json:"data"`
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func tiktokP2Parse(body []byte, target any) error {
	var env tiktokP2Envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return errors.New("TikTok devolvio respuesta invalida")
	}
	if env.Error.Code != "ok" {
		return fmt.Errorf("TikTok rechazo solicitud: %s", tiktokP2SafeCode(env.Error.Code))
	}
	if len(env.Data) == 0 {
		return errors.New("TikTok no devolvio datos")
	}
	if err := json.Unmarshal(env.Data, target); err != nil {
		return errors.New("TikTok devolvio datos no reconocidos")
	}
	return nil
}

func tiktokP2SafeCode(code string) string {
	code = strings.TrimSpace(code)
	if len(code) == 0 || len(code) > 64 {
		return "provider_error"
	}
	for _, r := range code {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
			return "provider_error"
		}
	}
	return code
}

func tiktokP2AllowedPrivacy(options []string, choice string) bool {
	if choice == "" {
		return false
	}
	for _, opt := range options {
		if opt == choice {
			return true
		}
	}
	return false
}

func tiktokP2VerifiedVideoURL(raw, prefix string) error {
	u, e := url.Parse(strings.TrimSpace(raw))
	if e != nil || u == nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" {
		return errors.New("El video requiere una URL HTTPS publica")
	}
	p, e := url.Parse(strings.TrimSpace(prefix))
	if e != nil || p == nil || p.Scheme != "https" || p.Host == "" || p.User != nil || p.RawQuery != "" || p.Fragment != "" || p.Path == "" {
		return errors.New("Configura WORKTIC_TIKTOK_VERIFIED_MEDIA_PREFIX con la ruta HTTPS verificada en TikTok")
	}
	if !strings.EqualFold(u.Host, p.Host) || !strings.HasPrefix(u.EscapedPath(), p.EscapedPath()) {
		return errors.New("El video debe estar alojado bajo el prefijo HTTPS verificado en TikTok")
	}
	return nil
}

func tiktokP2API(ctx context.Context, token, endpoint string, payload any, result any) error {
	// apiJSON does not inspect TikTok's error.code on HTTP 200; P2 does.
	var raw json.RawMessage
	if err := apiJSON(ctx, http.MethodPost, endpoint, token, payload, &raw); err != nil {
		return errors.New("TikTok no acepto la solicitud; revisa el estado de la autorizacion o el recurso")
	}
	return tiktokP2Parse(raw, result)
}

func (a *App) tiktokP2GetAuthorized(ctx context.Context, tid, connectionID int64, required string) (socialToken, error) {
	var provider, status, scopes, encrypted string
	e := a.db.QueryRow(`SELECT provider_mode,status,scopes,encrypted_credentials FROM social_connections WHERE tenant_id=? AND id=? AND platform='tiktok'`, tid, connectionID).Scan(&provider, &status, &scopes, &encrypted)
	if e != nil || provider != "official" || status != "connected" {
		return socialToken{}, errors.New("Selecciona una cuenta TikTok oficial conectada a esta empresa")
	}
	var token socialToken
	if json.Unmarshal([]byte(decryptLocal(encrypted, a.cfg.ChannelEncryptionKey)), &token) != nil || token.AccessToken == "" {
		return socialToken{}, errors.New("Reconecta tu cuenta TikTok")
	}
	if !tiktokT1HasScope(scopes, required) || !tiktokT1HasScope(token.Scope, required) {
		return socialToken{}, fmt.Errorf("TikTok no concedio %s a esta cuenta. Debes reautorizarla", required)
	}
	token, e = a.refreshSocialTokenIfNeeded(ctx, connectionID, tid, "tiktok", token)
	if e != nil {
		return socialToken{}, errors.New("No se pudo renovar la autorizacion de TikTok")
	}
	if !tiktokT1HasScope(token.Scope, required) {
		return socialToken{}, fmt.Errorf("TikTok ya no concede %s. Reconecta la cuenta", required)
	}
	return token, nil
}

func tiktokP2FetchCreator(ctx context.Context, token string) (tiktokP2Creator, error) {
	var c tiktokP2Creator
	err := tiktokP2API(ctx, token, "https://open.tiktokapis.com/v2/post/publish/creator_info/query/", map[string]any{}, &c)
	if err != nil {
		return c, err
	}
	if c.CreatorUsername == "" || len(c.PrivacyLevelOptions) == 0 {
		return c, errors.New("TikTok no devolvio opciones validas de privacidad para el creador")
	}
	return c, nil
}

type tiktokP2Submission struct {
	ConnectionID int64  `json:"connection_id"`
	Mode         string `json:"mode"`
	VideoURL     string `json:"video_url"`
	Title        string `json:"title"`
	PrivacyLevel string `json:"privacy_level"`
	AllowComment bool   `json:"allow_comment"`
	AllowDuet    bool   `json:"allow_duet"`
	AllowStitch  bool   `json:"allow_stitch"`
	BrandOrganic bool   `json:"brand_organic"`
	BrandContent bool   `json:"brand_content"`
	AIGenerated  bool   `json:"ai_generated"`
	Confirmed    bool   `json:"confirmed"`
}

func tiktokP2ValidateDirect(req tiktokP2Submission, c tiktokP2Creator) error {
	if !tiktokP2AllowedPrivacy(c.PrivacyLevelOptions, req.PrivacyLevel) {
		return errors.New("Selecciona manualmente la privacidad vigente de esta cuenta TikTok")
	}
	if req.BrandContent && req.PrivacyLevel == "SELF_ONLY" {
		return errors.New("TikTok no admite colaboracion pagada con visibilidad Solo yo")
	}
	if req.AllowComment && c.CommentDisabled {
		return errors.New("TikTok no permite comentarios para esta cuenta")
	}
	if req.AllowDuet && c.DuetDisabled {
		return errors.New("TikTok no permite Duos para esta cuenta")
	}
	if req.AllowStitch && c.StitchDisabled {
		return errors.New("TikTok no permite Pegar para esta cuenta")
	}
	return nil
}

func (a *App) tiktokPostingP2Handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	tid, u, err := a.tenantFor(r)
	if err != nil {
		tiktokP2Reject(w, "No autenticado", http.StatusUnauthorized)
		return
	}
	perms := socialPermissionsFor(u)
	if !perms.View {
		tiktokP2Reject(w, "Sin acceso al Social Hub", http.StatusForbidden)
		return
	}
	if r.Method == http.MethodGet && r.URL.Query().Get("action") == "readiness" {
		writeJSON(w, map[string]any{"enabled": tiktokP2Enabled(), "message": "Requiere Content Posting API aprobada, permisos concedidos, URL verificada y confirmacion por publicacion"})
		return
	}
	if !tiktokP2Enabled() {
		tiktokP2Reject(w, "TikTok Content Posting P2 aun no esta autorizado para produccion", http.StatusConflict)
		return
	}
	if !perms.Publish {
		tiktokP2Reject(w, "Solo usuarios con permiso de publicacion pueden exportar a TikTok", http.StatusForbidden)
		return
	}
	switch {
	case r.Method == http.MethodGet && r.URL.Query().Get("action") == "creator":
		id, _ := strconv.ParseInt(r.URL.Query().Get("connection_id"), 10, 64)
		if id <= 0 {
			tiktokP2Reject(w, "Cuenta invalida", 400)
			return
		}
		token, e := a.tiktokP2GetAuthorized(r.Context(), tid, id, "video.publish")
		if e != nil {
			tiktokP2Reject(w, e.Error(), http.StatusConflict)
			return
		}
		c, e := tiktokP2FetchCreator(r.Context(), token.AccessToken)
		if e != nil {
			tiktokP2Reject(w, e.Error(), 502)
			return
		}
		writeJSON(w, c)
	case r.Method == http.MethodPost:
		// A non-simple same-origin request is required for a high-impact POST.
		// Reject cross-site forms and scripts that do not carry this header.
		if r.Header.Get("X-Worktic-TikTok-Consent") != "p2" {
			tiktokP2Reject(w, "Falta confirmacion del formulario Worktic", http.StatusForbidden)
			return
		}
		if origin := strings.TrimSpace(r.Header.Get("Origin")); origin != "" {
			parsed, parseErr := url.Parse(origin)
			if parseErr != nil || parsed.Host != r.Host || parsed.Scheme != "https" {
				tiktokP2Reject(w, "Origen de solicitud no permitido", http.StatusForbidden)
				return
			}
		}
		if r.URL.Query().Get("action") != "submit" {
			tiktokP2Reject(w, "Accion invalida", 400)
			return
		}
		var req tiktokP2Submission
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req) != nil {
			tiktokP2Reject(w, "Solicitud invalida", 400)
			return
		}
		req.Title = strings.TrimSpace(req.Title)
		if req.ConnectionID <= 0 || req.VideoURL == "" || len(req.Title) == 0 || len(utf16.Encode([]rune(req.Title))) > 2200 || !req.Confirmed {
			tiktokP2Reject(w, "Debes seleccionar cuenta, video, titulo y confirmar explicitamente", 400)
			return
		}
		if err := tiktokP2VerifiedVideoURL(req.VideoURL, os.Getenv("WORKTIC_TIKTOK_VERIFIED_MEDIA_PREFIX")); err != nil {
			tiktokP2Reject(w, err.Error(), 400)
			return
		}
		if req.Mode != "direct" && req.Mode != "inbox" {
			tiktokP2Reject(w, "Modalidad invalida", 400)
			return
		}
		scope := "video.publish"
		if req.Mode == "inbox" {
			scope = "video.upload"
		}
		token, e := a.tiktokP2GetAuthorized(r.Context(), tid, req.ConnectionID, scope)
		if e != nil {
			tiktokP2Reject(w, e.Error(), 409)
			return
		}
		var endpoint string
		var body any
		if req.Mode == "direct" {
			// Re-query at submit; never trust stale UI privacy values.
			creator, e := tiktokP2FetchCreator(r.Context(), token.AccessToken)
			if e != nil {
				tiktokP2Reject(w, e.Error(), 502)
				return
			}
			if err := tiktokP2ValidateDirect(req, creator); err != nil {
				tiktokP2Reject(w, err.Error(), 400)
				return
			}
			endpoint = "https://open.tiktokapis.com/v2/post/publish/video/init/"
			body = map[string]any{"post_info": map[string]any{"title": req.Title, "privacy_level": req.PrivacyLevel, "disable_comment": !req.AllowComment, "disable_duet": !req.AllowDuet, "disable_stitch": !req.AllowStitch, "brand_organic_toggle": req.BrandOrganic, "brand_content_toggle": req.BrandContent}, "source_info": map[string]any{"source": "PULL_FROM_URL", "video_url": req.VideoURL}, "is_aigc": req.AIGenerated}
		} else {
			endpoint = "https://open.tiktokapis.com/v2/post/publish/inbox/video/init/"
			body = map[string]any{"source_info": map[string]any{"source": "PULL_FROM_URL", "video_url": req.VideoURL}}
		}
		var accepted struct {
			PublishID string `json:"publish_id"`
		}
		if err := tiktokP2API(r.Context(), token.AccessToken, endpoint, body, &accepted); err != nil {
			tiktokP2Reject(w, err.Error(), 502)
			return
		}
		if accepted.PublishID == "" {
			tiktokP2Reject(w, "TikTok no confirmo un identificador de envio; comprueba en la cuenta antes de reintentar", 502)
			return
		}
		now := time.Now().UTC().Format(time.RFC3339)
		// Persist accepted submissions without labelling them as already published.
		_, e = a.db.Exec(`CREATE TABLE IF NOT EXISTS social_tiktok_submissions_p2(publish_id TEXT PRIMARY KEY,tenant_id BIGINT NOT NULL,connection_id BIGINT NOT NULL,mode TEXT NOT NULL,privacy_level TEXT NOT NULL DEFAULT '',status TEXT NOT NULL,created_at TEXT NOT NULL,checked_at TEXT NOT NULL DEFAULT '')`)
		if e == nil {
			_, e = a.db.Exec(`INSERT INTO social_tiktok_submissions_p2(publish_id,tenant_id,connection_id,mode,privacy_level,status,created_at) VALUES(?,?,?,?,?,?,?)`, accepted.PublishID, tid, req.ConnectionID, req.Mode, req.PrivacyLevel, "PROCESSING", now)
		}
		writeJSON(w, map[string]any{"ok": true, "status": "PROCESSING", "publish_id": accepted.PublishID, "tracking_saved": e == nil, "message": "TikTok acepto el envio; aun no confirmo publicacion. No reenvies hasta consultar el estado."})
	case r.Method == http.MethodGet && r.URL.Query().Get("action") == "status":
		publishID := strings.TrimSpace(r.URL.Query().Get("publish_id"))
		if len(publishID) == 0 || len(publishID) > 160 {
			tiktokP2Reject(w, "Identificador invalido", 400)
			return
		}
		var connectionID int64
		if a.db.QueryRow(`SELECT connection_id FROM social_tiktok_submissions_p2 WHERE tenant_id=? AND publish_id=?`, tid, publishID).Scan(&connectionID) != nil {
			tiktokP2Reject(w, "Envio no encontrado en esta empresa", 404)
			return
		}
		// Either granted upload or direct-post scope may poll the publish ID.
		token, e := a.tiktokP2GetAuthorized(r.Context(), tid, connectionID, "video.upload")
		if e != nil {
			token, e = a.tiktokP2GetAuthorized(r.Context(), tid, connectionID, "video.publish")
		}
		if e != nil {
			tiktokP2Reject(w, "Reconecta TikTok para consultar el estado", 409)
			return
		}
		var status struct {
			Status     string `json:"status"`
			FailReason string `json:"fail_reason"`
		}
		if err := tiktokP2API(r.Context(), token.AccessToken, "https://open.tiktokapis.com/v2/post/publish/status/fetch/", map[string]any{"publish_id": publishID}, &status); err != nil {
			tiktokP2Reject(w, err.Error(), 502)
			return
		}
		safeStatus := tiktokP2SafeCode(status.Status)
		safeReason := tiktokP2SafeCode(status.FailReason)
		_, _ = a.db.Exec(`UPDATE social_tiktok_submissions_p2 SET status=?,checked_at=? WHERE tenant_id=? AND publish_id=?`, safeStatus, time.Now().UTC().Format(time.RFC3339), tid, publishID)
		writeJSON(w, map[string]any{"status": safeStatus, "fail_reason": safeReason, "publish_id": publishID})
	default:
		tiktokP2Reject(w, "Metodo no permitido", http.StatusMethodNotAllowed)
	}
}
