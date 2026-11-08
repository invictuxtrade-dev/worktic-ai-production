package main

import (
	"context"
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// B9 is opt-in while Meta Unified Login is being tested with app-role accounts.
func socialB9Enabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("WORKTIC_B9_INSTAGRAM_UNIFIED_MODE")), "enabled")
}

type instagramB9Account struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

// Only relative paths and cursor values from Meta are used. Never follow a URL
// supplied by paging.next, preventing the token from being sent to another host.
func (a *App) instagramB9Accounts(ctx context.Context, token string) ([]instagramB9Account, error) {
	if token == "" {
		return nil, errors.New("token ausente")
	}
	base := "https://graph.facebook.com/" + a.cfg.MetaGraphVersion + "/me/assigned_instagram_accounts?fields=id,username&limit=100"
	after := ""
	seen := map[string]bool{}
	accounts := map[string]instagramB9Account{}
	order := []string{}
	for i := 0; i < 30; i++ {
		endpoint := base
		if after != "" {
			endpoint += "&after=" + url.QueryEscape(after)
		}
		var result struct {
			Data   []instagramB9Account `json:"data"`
			Paging struct {
				Cursors struct {
					After string `json:"after"`
				} `json:"cursors"`
			} `json:"paging"`
		}
		if err := apiJSON(ctx, http.MethodGet, endpoint, token, nil, &result); err != nil {
			return nil, errors.New("Meta no permitio consultar cuentas Instagram asignadas")
		}
		for _, item := range result.Data {
			if item.ID == "" || len(item.ID) > 128 || !isMetaAccountID(item.ID) {
				return nil, errors.New("Meta devolvio una cuenta Instagram invalida")
			}
			if _, ok := accounts[item.ID]; !ok {
				order = append(order, item.ID)
			}
			accounts[item.ID] = item
		}
		next := result.Paging.Cursors.After
		if len(result.Data) == 0 || next == "" {
			out := make([]instagramB9Account, 0, len(order))
			for _, id := range order {
				out = append(out, accounts[id])
			}
			return out, nil
		}
		if next == after || seen[next] {
			return nil, errors.New("Paginacion Instagram repetida")
		}
		seen[next] = true
		after = next
	}
	return nil, errors.New("Demasiadas cuentas Instagram; seleccion incompleta")
}
func isMetaAccountID(v string) bool {
	if v == "" || len(v) > 128 {
		return false
	}
	for _, c := range v {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

var instagramB9SelectTemplate = template.Must(template.New("instagram-b9").Parse(`<!doctype html><html lang="es"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Instagram | Worktic AI</title><style>body{background:#f6f7fc;color:#161b2b;font:16px system-ui;min-height:100vh;display:grid;place-items:center}main{background:white;width:min(500px,90vw);padding:28px;border-radius:18px;box-shadow:0 12px 40px #191c351a}label,select,button{display:block;width:100%;box-sizing:border-box;margin:14px 0}select,button{padding:14px;border-radius:9px;font:inherit}button{color:white;background:#564bdf;border:0}small{color:#606a81}</style></head><body><main><h2>Conectar Instagram</h2><p>Selecciona únicamente la cuenta profesional que deseas vincular a esta empresa.</p><form method="post" action="{{.Action}}"><input type="hidden" name="selection_state" value="{{.State}}"><label for="instagram_id">Cuenta autorizada</label><select id="instagram_id" name="instagram_id" required><option value="">Selecciona una cuenta</option>{{range .Accounts}}<option value="{{.ID}}">@{{.Username}} ({{.ID}})</option>{{end}}</select><button type="submit">Conectar cuenta seleccionada</button></form><small>Solo la cuenta elegida quedará vinculada. No se enviarán credenciales al navegador.</small></main></body></html>`))

func (a *App) beginInstagramB9Selection(w http.ResponseWriter, r *http.Request, tid int64, tok socialToken) error {
	if a.cfg.ChannelEncryptionKey == "" || tok.AccessToken == "" {
		return errors.New("credenciales incompletas")
	}
	accounts, err := a.instagramB9Accounts(r.Context(), tok.AccessToken)
	if err != nil {
		return err
	}
	if len(accounts) == 0 {
		return errors.New("Meta no devolvio cuentas Instagram profesionales asignadas " + a.instagramB9SUATDiagnostic(r.Context(), tok.AccessToken))
	}
	raw, err := json.Marshal(tok)
	if err != nil {
		return err
	}
	state := randomState()
	now := time.Now().UTC()
	_, err = a.db.Exec(`INSERT INTO social_oauth_pending(state,tenant_id,platform,encrypted_user_token,created_at,expires_at) VALUES(?,?,?,?,?,?)`, state, tid, "instagram", encryptLocal(string(raw), a.cfg.ChannelEncryptionKey), now.Format(time.RFC3339), now.Add(15*time.Minute).Format(time.RFC3339))
	if err != nil {
		return errors.New("No se pudo registrar la seleccion temporal")
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'")
	return instagramB9SelectTemplate.Execute(w, struct {
		Action, State string
		Accounts      []instagramB9Account
	}{a.cfg.BaseURL + "/api/social/oauth/callback/instagram", state, accounts})
}

func (a *App) completeInstagramB9Selection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Metodo no permitido", 405)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16384)
	if r.ParseForm() != nil {
		http.Error(w, "Formulario invalido", 400)
		return
	}
	state := r.PostForm.Get("selection_state")
	chosen := r.PostForm.Get("instagram_id")
	if len(state) < 32 || len(state) > 128 || !isMetaAccountID(chosen) {
		http.Error(w, "Seleccion invalida", 400)
		return
	}
	var tid int64
	var enc string
	// Single-use, atomic and expiry-checked; an attacker cannot change tenant_id.
	err := a.db.QueryRow(`DELETE FROM social_oauth_pending WHERE state=? AND platform='instagram' AND expires_at>? RETURNING tenant_id,encrypted_user_token`, state, time.Now().UTC().Format(time.RFC3339)).Scan(&tid, &enc)
	if err != nil {
		http.Error(w, "Seleccion vencida o ya utilizada", 400)
		return
	}
	var tok socialToken
	if json.Unmarshal([]byte(decryptLocal(enc, a.cfg.ChannelEncryptionKey)), &tok) != nil || tok.AccessToken == "" {
		http.Error(w, "Autorizacion invalida", 400)
		return
	}
	accounts, err := a.instagramB9Accounts(r.Context(), tok.AccessToken)
	if err != nil {
		http.Error(w, "No se pudieron verificar los activos de Meta", 502)
		return
	}
	found := false
	for _, acct := range accounts {
		if acct.ID == chosen {
			found = true
			break
		}
	}
	if !found {
		http.Error(w, "Cuenta no autorizada", 403)
		return
	}
	var profile struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	}
	endpoint := "https://graph.facebook.com/" + a.cfg.MetaGraphVersion + "/" + url.PathEscape(chosen) + "?fields=id,username"
	if apiJSON(r.Context(), http.MethodGet, endpoint, tok.AccessToken, nil, &profile) != nil || profile.ID != chosen || profile.Username == "" {
		http.Error(w, "Meta no confirmo identidad de Instagram", 409)
		return
	}
	// Fail closed on invalid grants: never label unobserved scopes as granted.
	granted, err := a.metaSocialPermissions(r.Context(), tok.AccessToken)
	if err != nil || !granted["instagram_basic"] {
		http.Error(w, "Meta no confirmo el permiso instagram_basic", 409)
		return
	}
	scopes := []string{}
	for _, p := range []string{"instagram_basic", "instagram_content_publish", "instagram_manage_comments", "instagram_manage_messages"} {
		if granted[p] {
			scopes = append(scopes, p)
		}
	}
	cfg := socialProviderConfig{InstagramID: chosen}
	if a.saveOfficialConnection(tid, "instagram", profile.Username, chosen, tok, cfg, strings.Join(scopes, ",")) != nil {
		http.Error(w, "No se pudo guardar conexion", 500)
		return
	}
	http.Redirect(w, r, a.cfg.BaseURL+"/app.html?social_oauth=success&platform=instagram", http.StatusSeeOther)
}
