package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"
)

// T1 habilita solo el acceso minimo a perfil. Publicacion y mensajeria
// dependen de permisos y auditorias adicionales en TikTok for Developers.
const tiktokT1InitialScope = "user.info.basic"

func tiktokT1Enabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("WORKTIC_TIKTOK_T1_ENABLED")), "true")
}

func tiktokT1HasScope(scopes, target string) bool {
	for _, scope := range strings.FieldsFunc(scopes, func(r rune) bool { return r == ',' || r == ' ' || r == '\n' || r == '\t' }) {
		if strings.TrimSpace(scope) == target {
			return true
		}
	}
	return false
}

func (a *App) tiktokT1Ready() (bool, string) {
	if !tiktokT1Enabled() {
		return false, "TikTok T1 no esta habilitado"
	}
	if strings.TrimSpace(a.cfg.TikTokClientKey) == "" || strings.TrimSpace(a.cfg.TikTokClientSecret) == "" {
		return false, "Configura TIKTOK_CLIENT_KEY y TIKTOK_CLIENT_SECRET en Render"
	}
	if len(strings.TrimSpace(os.Getenv("CHANNEL_ENCRYPTION_KEY"))) < 32 {
		return false, "Configura CHANNEL_ENCRYPTION_KEY de al menos 32 caracteres en Render"
	}
	if !strings.HasPrefix(strings.ToLower(a.cfg.BaseURL), "https://") {
		return false, "TikTok OAuth requiere BASE_URL HTTPS"
	}
	return true, "Listo para conectar mediante TikTok Login Kit"
}

func (a *App) tiktokT1ReadinessHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Metodo no permitido", http.StatusMethodNotAllowed)
		return
	}
	_, u, err := a.tenantFor(r)
	if err != nil {
		http.Error(w, "No autenticado", http.StatusUnauthorized)
		return
	}
	if !socialPermissionsFor(u).View {
		http.Error(w, "Sin permiso para ver redes sociales", http.StatusForbidden)
		return
	}
	ready, reason := a.tiktokT1Ready()
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, map[string]any{
		"enabled": tiktokT1Enabled(), "login_ready": ready,
		"message": reason, "requested_scopes": []string{tiktokT1InitialScope},
		"callback_url":           a.cfg.BaseURL + "/api/social/oauth/callback/tiktok",
		"direct_publish_enabled": false, "business_messaging_enabled": false,
	})
}

type tiktokT1Profile struct {
	Data struct {
		User struct {
			OpenID      string `json:"open_id"`
			DisplayName string `json:"display_name"`
		} `json:"user"`
	} `json:"data"`
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func tiktokT1ValidateProfile(p tiktokT1Profile, tokenOpenID string) (string, string, error) {
	if p.Error.Code != "" && p.Error.Code != "ok" {
		return "", "", errors.New("TikTok rechazo la consulta de perfil")
	}
	id := strings.TrimSpace(p.Data.User.OpenID)
	if id == "" || (tokenOpenID != "" && tokenOpenID != id) {
		return "", "", errors.New("TikTok no confirmo un perfil consistente")
	}
	name := strings.TrimSpace(p.Data.User.DisplayName)
	if name == "" {
		name = "Cuenta TikTok"
	}
	return id, name, nil
}
func tiktokT1FetchProfile(ctx context.Context, token, tokenOpenID string) (string, string, error) {
	var profile tiktokT1Profile
	if err := apiJSON(ctx, "GET", "https://open.tiktokapis.com/v2/user/info/?fields=open_id,display_name", token, nil, &profile); err != nil {
		return "", "", err
	}
	return tiktokT1ValidateProfile(profile, tokenOpenID)
}

// TikTok rota el refresh_token. Evita rotarlo cada vez que el usuario pulsa Probar.
func tiktokT1TokenNeedsRefresh(expiry string, now time.Time) bool {
	exp, err := time.Parse(time.RFC3339, expiry)
	return err != nil || !now.Add(5*time.Minute).Before(exp)
}
