package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// B9 internal bootstrap is deliberately NOT the self-service path for SaaS customers.
// Disabled unless a single target tenant/account and an independent confirmation key are set.
type b9InternalSettings struct {
	Tenant                              int64
	InstagramID, Token, Confirm, Origin string
}

func b9InternalSettingsFromEnv() (b9InternalSettings, bool) {
	tenant, err := strconv.ParseInt(strings.TrimSpace(os.Getenv("WORKTIC_B9_INTERNAL_TENANT_ID")), 10, 64)
	c := b9InternalSettings{Tenant: tenant, InstagramID: strings.TrimSpace(os.Getenv("WORKTIC_B9_INTERNAL_IG_ID")), Token: strings.TrimSpace(os.Getenv("WORKTIC_B9_INTERNAL_IG_TOKEN")), Confirm: strings.TrimSpace(os.Getenv("WORKTIC_B9_INTERNAL_CONFIRM_KEY")), Origin: strings.TrimRight(strings.TrimSpace(os.Getenv("WORKTIC_B9_INTERNAL_ORIGIN")), "/")}
	return c, err == nil && tenant > 0 && isMetaAccountID(c.InstagramID) && len(c.Token) > 40 && len(c.Confirm) >= 32 && (c.Origin == "https://www.workticai.com" || c.Origin == "https://workticai.com") && strings.EqualFold(os.Getenv("WORKTIC_B9_INTERNAL_MODE"), "enabled")
}

func (a *App) b9InternalVerifiedAccount(ctx context.Context, token string, expected string) (instagramB9Account, []string, error) {
	if a.cfg.MetaAppID == "" || a.cfg.MetaAppSecret == "" || a.cfg.ChannelEncryptionKey == "" {
		return instagramB9Account{}, nil, errors.New("configuracion de seguridad incompleta")
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	// Verify token belongs to OUR application, is valid, and grants basic Instagram access.
	debugURL := "https://graph.facebook.com/" + a.cfg.MetaGraphVersion + "/debug_token?input_token=" + url.QueryEscape(token)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, debugURL, nil)
	if err != nil {
		return instagramB9Account{}, nil, errors.New("consulta de identidad fallida")
	}
	req.Header.Set("Authorization", "Bearer "+a.cfg.MetaAppID+"|"+a.cfg.MetaAppSecret)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return instagramB9Account{}, nil, errors.New("Meta no respondio a la validacion")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return instagramB9Account{}, nil, errors.New("Meta rechazo validacion de token")
	}
	var d struct {
		Data struct {
			AppID   string   `json:"app_id"`
			IsValid bool     `json:"is_valid"`
			Type    string   `json:"type"`
			Scopes  []string `json:"scopes"`
		} `json:"data"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&d) != nil {
		return instagramB9Account{}, nil, errors.New("Meta devolvio identidad invalida")
	}
	if !d.Data.IsValid || d.Data.AppID != a.cfg.MetaAppID || d.Data.Type != "SYSTEM_USER" {
		return instagramB9Account{}, nil, errors.New("token no corresponde a usuario del sistema de esta app")
	}
	hasBasic := false
	scopes := []string{}
	allowed := map[string]bool{"instagram_basic": true, "instagram_content_publish": true, "instagram_manage_comments": true, "instagram_manage_messages": true}
	for _, s := range d.Data.Scopes {
		if allowed[s] {
			scopes = append(scopes, s)
		}
		if s == "instagram_basic" {
			hasBasic = true
		}
	}
	if !hasBasic {
		return instagramB9Account{}, nil, errors.New("falta instagram_basic")
	}
	accounts, err := a.instagramB9Accounts(ctx, token)
	if err != nil {
		return instagramB9Account{}, nil, errors.New("Meta no permitio verificar activos")
	}
	var account instagramB9Account
	for _, v := range accounts {
		if v.ID == expected {
			account = v
			break
		}
	}
	if account.ID == "" {
		return instagramB9Account{}, nil, errors.New("la cuenta esperada no figura entre activos autorizados")
	}
	var profile instagramB9Account
	path := "https://graph.facebook.com/" + a.cfg.MetaGraphVersion + "/" + url.PathEscape(expected) + "?fields=id,username"
	if apiJSON(ctx, http.MethodGet, path, token, nil, &profile) != nil || profile.ID != expected || strings.TrimSpace(profile.Username) == "" {
		return instagramB9Account{}, nil, errors.New("identidad de Instagram no confirmada")
	}
	return profile, scopes, nil
}

// POST /api/social/b9/internal-bootstrap with header X-Worktic-B9-Key
// JSON {"action":"check"} performs no writes; {"action":"connect"} persists only the verified asset.
func (a *App) socialB9InternalBootstrapHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodPost {
		http.Error(w, "metodo no permitido", 405)
		return
	}
	cfg, ok := b9InternalSettingsFromEnv()
	if !ok {
		http.Error(w, "configuracion interna deshabilitada", 404)
		return
	}
	tid, user, err := a.tenantFor(r)
	if err != nil {
		http.Error(w, "sesion requerida", 401)
		return
	}
	if tid != cfg.Tenant || !socialPermissionsFor(user).ManageConnections {
		http.Error(w, "no autorizado", 403)
		return
	}
	// Require an explicit matching origin even if the global API middleware allows missing Origin.
	if r.Header.Get("Origin") != cfg.Origin || !strings.EqualFold(r.Header.Get("Content-Type"), "application/json") {
		http.Error(w, "origen o contenido invalido", 403)
		return
	}
	k := r.Header.Get("X-Worktic-B9-Key")
	if len(k) != len(cfg.Confirm) || subtle.ConstantTimeCompare([]byte(k), []byte(cfg.Confirm)) != 1 {
		http.Error(w, "confirmacion requerida", 403)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var q struct {
		Action string `json:"action"`
	}
	if json.NewDecoder(r.Body).Decode(&q) != nil || (q.Action != "check" && q.Action != "connect") {
		http.Error(w, "accion invalida", 400)
		return
	}
	account, scopes, err := a.b9InternalVerifiedAccount(r.Context(), cfg.Token, cfg.InstagramID)
	if err != nil {
		http.Error(w, err.Error(), 409)
		return
	}
	if q.Action == "check" {
		writeJSON(w, map[string]any{"ok": true, "verified": true, "username": account.Username, "scopes": scopes, "saved": false})
		return
	}
	// Do not silently take an IG asset already connected to another company.
	var count int
	if err = a.db.QueryRow(`SELECT COUNT(*) FROM social_connections WHERE platform='instagram' AND external_account_id=? AND tenant_id<>?`, account.ID, tid).Scan(&count); err != nil || count > 0 {
		http.Error(w, "cuenta ya asignada o conflicto de base de datos", 409)
		return
	}
	ensureSocialProviderSchema(a.db)
	if err = a.saveOfficialConnection(tid, "instagram", account.Username, account.ID, socialToken{AccessToken: cfg.Token, TokenType: "SYSTEM_USER"}, socialProviderConfig{InstagramID: account.ID}, strings.Join(scopes, ",")); err != nil {
		http.Error(w, "no se pudo guardar la conexion", 500)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "saved": true, "username": account.Username, "platform": "instagram"})
}
