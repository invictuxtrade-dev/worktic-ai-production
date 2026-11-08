package main

import (
	"net/http"
	"os"
	"strings"
)

// B9 read-only preflight: never returns Meta client credentials or access tokens.
func (a *App) socialB9ReadinessHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "metodo no permitido", http.StatusMethodNotAllowed)
		return
	}
	_, user, err := a.tenantFor(r)
	if err != nil {
		http.Error(w, "sesion requerida", http.StatusUnauthorized)
		return
	}
	if !socialPermissionsFor(user).ManageConnections {
		http.Error(w, "administrador requerido", http.StatusForbidden)
		return
	}
	configured := isMetaSocialConfigID(strings.TrimSpace(os.Getenv("WORKTIC_META_SOCIAL_INSTAGRAM_CONFIG_ID")))
	mode := strings.TrimSpace(os.Getenv("WORKTIC_B9_INSTAGRAM_UNIFIED_MODE"))
	// No inference that a configured config_id implies a verified Unified/SUAT flow.
	writeJSON(w, map[string]any{
		"phase":                       "B9-unified-beta",
		"provider":                    "instagram",
		"meta_app_configured":         a.cfg.MetaAppID != "" && a.cfg.MetaAppSecret != "",
		"instagram_config_id_present": configured,
		"encryption_ready":            strings.TrimSpace(a.cfg.ChannelEncryptionKey) != "",
		"callback_path":               "/api/social/oauth/callback/instagram",
		"unified_mode_enabled":        strings.EqualFold(mode, "enabled"),
		"unified_login_beta_enabled":  strings.EqualFold(mode, "enabled"),
		"note":                        "Flujo de autorizacion B9 en beta; probar con una cuenta de rol Meta antes de clientes externos.",
	})
}
