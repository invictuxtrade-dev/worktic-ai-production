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
	"strconv"
	"strings"
	"time"
)

type adsCredentialV25 struct {
	AccessToken     string `json:"access_token"`
	RefreshToken    string `json:"refresh_token,omitempty"`
	ExpiresAt       string `json:"expires_at,omitempty"`
	LoginCustomerID string `json:"login_customer_id,omitempty"`
}

type adsConnectionV25 struct {
	ID          int64  `json:"id"`
	Provider    string `json:"provider"`
	AccountID   string `json:"account_id"`
	AccountName string `json:"account_name"`
	Currency    string `json:"currency"`
	Status      string `json:"status"`
	LastSyncAt  string `json:"last_sync_at"`
	LastError   string `json:"last_error"`
	CanManage   bool   `json:"can_manage"`
}

func initAdsV25Schema(db *DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS ads_connections_v25(
          id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL, provider TEXT NOT NULL,
          account_id TEXT NOT NULL, account_name TEXT NOT NULL DEFAULT '', currency TEXT NOT NULL DEFAULT '',
          status TEXT NOT NULL DEFAULT 'connected', encrypted_credentials TEXT NOT NULL DEFAULT '', config_json TEXT NOT NULL DEFAULT '{}',
          last_sync_at TEXT NOT NULL DEFAULT '', last_error TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
          UNIQUE(tenant_id,provider,account_id))`,
		`CREATE INDEX IF NOT EXISTS idx_ads_conn_v25_tenant ON ads_connections_v25(tenant_id,provider,status)`,
		`CREATE TABLE IF NOT EXISTS ads_campaigns_v25(
          id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL, connection_id INTEGER NOT NULL, provider TEXT NOT NULL,
          account_id TEXT NOT NULL, external_campaign_id TEXT NOT NULL, name TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT '',
          objective TEXT NOT NULL DEFAULT '', channel_type TEXT NOT NULL DEFAULT '', budget_daily REAL NOT NULL DEFAULT 0,
          budget_total REAL NOT NULL DEFAULT 0, currency TEXT NOT NULL DEFAULT '', internal_campaign_id INTEGER NOT NULL DEFAULT 0,
          raw_json TEXT NOT NULL DEFAULT '{}', created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
          UNIQUE(tenant_id,provider,account_id,external_campaign_id))`,
		`CREATE INDEX IF NOT EXISTS idx_ads_campaign_v25_tenant ON ads_campaigns_v25(tenant_id,provider,status,internal_campaign_id)`,
		`CREATE TABLE IF NOT EXISTS ads_entities_v25(
          id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL, connection_id INTEGER NOT NULL, provider TEXT NOT NULL,
          account_id TEXT NOT NULL, entity_type TEXT NOT NULL, external_id TEXT NOT NULL, external_campaign_id TEXT NOT NULL DEFAULT '',
          external_group_id TEXT NOT NULL DEFAULT '', name TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT '', budget REAL NOT NULL DEFAULT 0,
          currency TEXT NOT NULL DEFAULT '', raw_json TEXT NOT NULL DEFAULT '{}', updated_at TEXT NOT NULL,
          UNIQUE(tenant_id,provider,account_id,entity_type,external_id))`,
		`CREATE INDEX IF NOT EXISTS idx_ads_entities_v25_tenant ON ads_entities_v25(tenant_id,provider,entity_type,external_campaign_id)`,
		`CREATE TABLE IF NOT EXISTS ads_daily_metrics_v25(
          id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL, campaign_row_id INTEGER NOT NULL DEFAULT 0,
          provider TEXT NOT NULL, account_id TEXT NOT NULL, external_campaign_id TEXT NOT NULL, metric_date TEXT NOT NULL,
          spend REAL NOT NULL DEFAULT 0, impressions INTEGER NOT NULL DEFAULT 0, clicks INTEGER NOT NULL DEFAULT 0,
          reach INTEGER NOT NULL DEFAULT 0, conversions REAL NOT NULL DEFAULT 0, conversion_value REAL NOT NULL DEFAULT 0,
          leads REAL NOT NULL DEFAULT 0, raw_json TEXT NOT NULL DEFAULT '{}', updated_at TEXT NOT NULL,
          UNIQUE(tenant_id,provider,account_id,external_campaign_id,metric_date))`,
		`CREATE INDEX IF NOT EXISTS idx_ads_metrics_v25_tenant ON ads_daily_metrics_v25(tenant_id,metric_date,provider)`,
		`CREATE TABLE IF NOT EXISTS ads_oauth_states_v25(
          id INTEGER PRIMARY KEY AUTOINCREMENT, state TEXT UNIQUE NOT NULL, tenant_id INTEGER NOT NULL, provider TEXT NOT NULL,
          created_at TEXT NOT NULL, expires_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS ads_sync_log_v25(
          id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL, connection_id INTEGER NOT NULL, provider TEXT NOT NULL,
          status TEXT NOT NULL, rows_synced INTEGER NOT NULL DEFAULT 0, error_message TEXT NOT NULL DEFAULT '',
          started_at TEXT NOT NULL, finished_at TEXT NOT NULL DEFAULT '')`,
	}
	for _, q := range stmts {
		if _, err := db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

func canManageAdsV25(role string) bool {
	return role == "owner" || role == "admin" || role == "superadmin"
}

func (a *App) adsV25ConfigHandler(w http.ResponseWriter, r *http.Request) {
	tid, u, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	_ = tid
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", 405)
		return
	}
	writeJSON(w, map[string]any{
		"can_manage": canManageAdsV25(u.Role),
		"providers": map[string]any{
			"meta":   map[string]any{"ready": a.cfg.MetaAppID != "" && a.cfg.MetaAppSecret != "", "label": "Meta Ads"},
			"tiktok": map[string]any{"ready": a.cfg.TikTokBusinessAppID != "" && a.cfg.TikTokBusinessSecret != "" && a.cfg.TikTokBusinessAuthURL != "", "label": "TikTok Ads"},
			"google": map[string]any{"ready": a.cfg.GoogleClientID != "" && a.cfg.GoogleClientSecret != "" && a.cfg.GoogleAdsDeveloperToken != "", "label": "Google / YouTube Ads"},
		},
	})
}

func (a *App) adsV25ConnectionsHandler(w http.ResponseWriter, r *http.Request) {
	tid, u, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	switch r.Method {
	case http.MethodGet:
		rows, err := a.db.Query(`SELECT id,provider,account_id,account_name,currency,status,last_sync_at,last_error FROM ads_connections_v25 WHERE tenant_id=? ORDER BY provider,account_name,account_id`, tid)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		defer rows.Close()
		out := []adsConnectionV25{}
		for rows.Next() {
			var x adsConnectionV25
			_ = rows.Scan(&x.ID, &x.Provider, &x.AccountID, &x.AccountName, &x.Currency, &x.Status, &x.LastSyncAt, &x.LastError)
			x.CanManage = canManageAdsV25(u.Role)
			out = append(out, x)
		}
		writeJSON(w, map[string]any{"connections": out, "can_manage": canManageAdsV25(u.Role)})
	case http.MethodPost:
		if !canManageAdsV25(u.Role) {
			writeError(w, errors.New("sin permisos"), 403)
			return
		}
		var q struct{ Provider, AccountID, AccountName, Currency, AccessToken, RefreshToken, LoginCustomerID string }
		if json.NewDecoder(r.Body).Decode(&q) != nil {
			writeError(w, errors.New("JSON inválido"), 400)
			return
		}
		q.Provider = strings.ToLower(strings.TrimSpace(q.Provider))
		q.AccountID = strings.TrimSpace(q.AccountID)
		if q.Provider == "" || q.AccountID == "" {
			writeError(w, errors.New("provider y account_id son obligatorios"), 400)
			return
		}
		if q.Provider != "google" && strings.TrimSpace(q.AccessToken) == "" {
			writeError(w, errors.New("access_token obligatorio"), 400)
			return
		}
		if q.Provider == "google" && strings.TrimSpace(q.AccessToken) == "" && strings.TrimSpace(q.RefreshToken) == "" {
			writeError(w, errors.New("token de Google obligatorio"), 400)
			return
		}
		cred := adsCredentialV25{AccessToken: q.AccessToken, RefreshToken: q.RefreshToken, LoginCustomerID: q.LoginCustomerID}
		cb, _ := json.Marshal(cred)
		now := time.Now().UTC().Format(time.RFC3339)
		_, err := a.db.Exec(`INSERT INTO ads_connections_v25(tenant_id,provider,account_id,account_name,currency,status,encrypted_credentials,config_json,created_at,updated_at) VALUES(?,?,?,?,?,'connected',?,'{}',?,?) ON CONFLICT(tenant_id,provider,account_id) DO UPDATE SET account_name=excluded.account_name,currency=excluded.currency,status='connected',encrypted_credentials=excluded.encrypted_credentials,last_error='',updated_at=excluded.updated_at`, tid, q.Provider, q.AccountID, q.AccountName, q.Currency, encryptLocal(string(cb), a.cfg.ChannelEncryptionKey), now, now)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	case http.MethodDelete:
		if !canManageAdsV25(u.Role) {
			writeError(w, errors.New("sin permisos"), 403)
			return
		}
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		if id <= 0 {
			writeError(w, errors.New("id inválido"), 400)
			return
		}
		_, _ = a.db.Exec(`DELETE FROM ads_daily_metrics_v25 WHERE tenant_id=? AND campaign_row_id IN (SELECT id FROM ads_campaigns_v25 WHERE tenant_id=? AND connection_id=?)`, tid, tid, id)
		_, _ = a.db.Exec(`DELETE FROM ads_entities_v25 WHERE tenant_id=? AND connection_id=?`, tid, id)
		_, _ = a.db.Exec(`DELETE FROM ads_campaigns_v25 WHERE tenant_id=? AND connection_id=?`, tid, id)
		_, _ = a.db.Exec(`DELETE FROM ads_connections_v25 WHERE tenant_id=? AND id=?`, tid, id)
		writeJSON(w, map[string]any{"ok": true})
	default:
		http.Error(w, "Método no permitido", 405)
	}
}

func (a *App) adsV25OAuthStartHandler(w http.ResponseWriter, r *http.Request) {
	tid, u, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	if !canManageAdsV25(u.Role) {
		writeError(w, errors.New("sin permisos"), 403)
		return
	}
	p := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("provider")))
	state := randomState()
	now := time.Now().UTC()
	exp := now.Add(15 * time.Minute)
	if _, err = a.db.Exec(`INSERT INTO ads_oauth_states_v25(state,tenant_id,provider,created_at,expires_at) VALUES(?,?,?,?,?)`, state, tid, p, now.Format(time.RFC3339), exp.Format(time.RFC3339)); err != nil {
		writeError(w, err, 500)
		return
	}
	redirect := a.cfg.BaseURL + "/api/ads/v25/oauth/callback/" + p
	var auth string
	switch p {
	case "meta":
		if a.cfg.MetaAppID == "" {
			writeError(w, errors.New("META_APP_ID no configurado"), 400)
			return
		}
		q := url.Values{"client_id": {a.cfg.MetaAppID}, "redirect_uri": {redirect}, "state": {state}, "response_type": {"code"}, "scope": {"ads_read,ads_management,business_management"}}
		auth = "https://www.facebook.com/" + a.cfg.MetaGraphVersion + "/dialog/oauth?" + q.Encode()
	case "google":
		if a.cfg.GoogleClientID == "" || a.cfg.GoogleAdsDeveloperToken == "" {
			writeError(w, errors.New("Google Ads no configurado"), 400)
			return
		}
		q := url.Values{"client_id": {a.cfg.GoogleClientID}, "redirect_uri": {redirect}, "state": {state}, "response_type": {"code"}, "scope": {"https://www.googleapis.com/auth/adwords"}, "access_type": {"offline"}, "prompt": {"consent"}}
		auth = "https://accounts.google.com/o/oauth2/v2/auth?" + q.Encode()
	case "tiktok":
		if a.cfg.TikTokBusinessAuthURL == "" {
			writeError(w, errors.New("TIKTOK_BUSINESS_AUTH_URL no configurado"), 400)
			return
		}
		u0, e := url.Parse(a.cfg.TikTokBusinessAuthURL)
		if e != nil {
			writeError(w, e, 400)
			return
		}
		q := u0.Query()
		q.Set("state", state)
		u0.RawQuery = q.Encode()
		auth = u0.String()
	default:
		writeError(w, errors.New("proveedor no soportado"), 400)
		return
	}
	writeJSON(w, map[string]any{"authorization_url": auth})
}

func (a *App) adsV25OAuthCallbackHandler(w http.ResponseWriter, r *http.Request) {
	p := strings.TrimPrefix(r.URL.Path, "/api/ads/v25/oauth/callback/")
	p = strings.Trim(strings.ToLower(p), "/")
	state := r.URL.Query().Get("state")
	var tid int64
	var stored, expires string
	if a.db.QueryRow(`SELECT tenant_id,provider,expires_at FROM ads_oauth_states_v25 WHERE state=?`, state).Scan(&tid, &stored, &expires) != nil || stored != p {
		http.Redirect(w, r, a.cfg.BaseURL+"/app.html?ads_oauth=error", 302)
		return
	}
	ex, _ := time.Parse(time.RFC3339, expires)
	if time.Now().UTC().After(ex) {
		http.Redirect(w, r, a.cfg.BaseURL+"/app.html?ads_oauth=expired", 302)
		return
	}
	_, _ = a.db.Exec(`DELETE FROM ads_oauth_states_v25 WHERE state=?`, state)
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	var err error
	switch p {
	case "meta":
		err = a.completeMetaAdsOAuthV25(ctx, tid, r.URL.Query().Get("code"), a.cfg.BaseURL+"/api/ads/v25/oauth/callback/meta")
	case "google":
		err = a.completeGoogleAdsOAuthV25(ctx, tid, r.URL.Query().Get("code"), a.cfg.BaseURL+"/api/ads/v25/oauth/callback/google")
	case "tiktok":
		code := firstNonEmpty(r.URL.Query().Get("auth_code"), r.URL.Query().Get("code"))
		err = a.completeTikTokAdsOAuthV25(ctx, tid, code)
	default:
		err = errors.New("proveedor no soportado")
	}
	if err != nil {
		http.Redirect(w, r, a.cfg.BaseURL+"/app.html?ads_oauth=error&msg="+url.QueryEscape(err.Error()), 302)
		return
	}
	http.Redirect(w, r, a.cfg.BaseURL+"/app.html?ads_oauth=success&provider="+url.QueryEscape(p), 302)
}

func (a *App) completeMetaAdsOAuthV25(ctx context.Context, tid int64, code, redirect string) error {
	if code == "" {
		return errors.New("Meta no devolvió code")
	}
	t, err := a.exchangeSocialCode(ctx, "facebook", code, redirect)
	if err != nil {
		return err
	}
	endpoint := "https://graph.facebook.com/" + a.cfg.MetaGraphVersion + "/me/adaccounts?fields=id,account_id,name,currency,account_status&limit=100&access_token=" + url.QueryEscape(t.AccessToken)
	var resp struct {
		Data []struct {
			ID            string `json:"id"`
			AccountID     string `json:"account_id"`
			Name          string `json:"name"`
			Currency      string `json:"currency"`
			AccountStatus int    `json:"account_status"`
		} `json:"data"`
	}
	if err = apiJSON(ctx, "GET", endpoint, "", nil, &resp); err != nil {
		return err
	}
	if len(resp.Data) == 0 {
		return errors.New("Meta no devolvió cuentas publicitarias")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	cred := adsCredentialV25{AccessToken: t.AccessToken}
	cb, _ := json.Marshal(cred)
	for _, x := range resp.Data {
		account := firstNonEmpty(x.ID, "act_"+x.AccountID)
		_, _ = a.db.Exec(`INSERT INTO ads_connections_v25(tenant_id,provider,account_id,account_name,currency,status,encrypted_credentials,config_json,created_at,updated_at) VALUES(?,?,?,?,?,'connected',?,'{}',?,?) ON CONFLICT(tenant_id,provider,account_id) DO UPDATE SET account_name=excluded.account_name,currency=excluded.currency,status='connected',encrypted_credentials=excluded.encrypted_credentials,last_error='',updated_at=excluded.updated_at`, tid, "meta", account, x.Name, x.Currency, encryptLocal(string(cb), a.cfg.ChannelEncryptionKey), now, now)
	}
	return nil
}

func (a *App) completeTikTokAdsOAuthV25(ctx context.Context, tid int64, code string) error {
	if code == "" {
		return errors.New("TikTok no devolvió auth_code")
	}
	body, _ := json.Marshal(map[string]any{"app_id": a.cfg.TikTokBusinessAppID, "secret": a.cfg.TikTokBusinessSecret, "auth_code": code})
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://business-api.tiktok.com/open_api/v1.3/oauth2/access_token/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("TikTok token: %s", string(b))
	}
	var tr struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			AccessToken   string   `json:"access_token"`
			RefreshToken  string   `json:"refresh_token"`
			AdvertiserIDs []string `json:"advertiser_ids"`
		} `json:"data"`
	}
	if json.Unmarshal(b, &tr) != nil || tr.Data.AccessToken == "" {
		return fmt.Errorf("TikTok token inválido: %s", tr.Message)
	}
	advs := tr.Data.AdvertiserIDs
	if len(advs) == 0 {
		ep := "https://business-api.tiktok.com/open_api/v1.3/oauth2/advertiser/get/?app_id=" + url.QueryEscape(a.cfg.TikTokBusinessAppID) + "&secret=" + url.QueryEscape(a.cfg.TikTokBusinessSecret)
		req2, _ := http.NewRequestWithContext(ctx, "GET", ep, nil)
		req2.Header.Set("Access-Token", tr.Data.AccessToken)
		rr, e := http.DefaultClient.Do(req2)
		if e == nil {
			defer rr.Body.Close()
			bb, _ := io.ReadAll(io.LimitReader(rr.Body, 2<<20))
			var ar struct {
				Data struct {
					List []struct {
						AdvertiserID   string `json:"advertiser_id"`
						AdvertiserName string `json:"advertiser_name"`
					} `json:"list"`
				} `json:"data"`
			}
			if json.Unmarshal(bb, &ar) == nil {
				for _, x := range ar.Data.List {
					advs = append(advs, x.AdvertiserID)
				}
			}
		}
	}
	if len(advs) == 0 {
		return errors.New("TikTok no devolvió advertiser IDs")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	cred := adsCredentialV25{AccessToken: tr.Data.AccessToken, RefreshToken: tr.Data.RefreshToken}
	cb, _ := json.Marshal(cred)
	for _, aid := range advs {
		name, curr := a.tiktokAdvertiserInfoV25(ctx, tr.Data.AccessToken, aid)
		_, _ = a.db.Exec(`INSERT INTO ads_connections_v25(tenant_id,provider,account_id,account_name,currency,status,encrypted_credentials,config_json,created_at,updated_at) VALUES(?,?,?,?,?,'connected',?,'{}',?,?) ON CONFLICT(tenant_id,provider,account_id) DO UPDATE SET account_name=excluded.account_name,currency=excluded.currency,status='connected',encrypted_credentials=excluded.encrypted_credentials,last_error='',updated_at=excluded.updated_at`, tid, "tiktok", aid, name, curr, encryptLocal(string(cb), a.cfg.ChannelEncryptionKey), now, now)
	}
	return nil
}

func (a *App) tiktokAdvertiserInfoV25(ctx context.Context, token, aid string) (string, string) {
	ids, _ := json.Marshal([]string{aid})
	ep := "https://business-api.tiktok.com/open_api/v1.3/advertiser/info/?advertiser_ids=" + url.QueryEscape(string(ids))
	req, _ := http.NewRequestWithContext(ctx, "GET", ep, nil)
	req.Header.Set("Access-Token", token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return aid, ""
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	var x struct {
		Data struct {
			List []struct {
				AdvertiserName string `json:"advertiser_name"`
				Currency       string `json:"currency"`
			} `json:"list"`
		} `json:"data"`
	}
	if json.Unmarshal(b, &x) == nil && len(x.Data.List) > 0 {
		return x.Data.List[0].AdvertiserName, x.Data.List[0].Currency
	}
	return aid, ""
}

func (a *App) completeGoogleAdsOAuthV25(ctx context.Context, tid int64, code, redirect string) error {
	if code == "" {
		return errors.New("Google no devolvió code")
	}
	vals := url.Values{"client_id": {a.cfg.GoogleClientID}, "client_secret": {a.cfg.GoogleClientSecret}, "code": {code}, "grant_type": {"authorization_code"}, "redirect_uri": {redirect}}
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://oauth2.googleapis.com/token", strings.NewReader(vals.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("Google token: %s", string(b))
	}
	var tok struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if json.Unmarshal(b, &tok) != nil || tok.AccessToken == "" {
		return errors.New("token Google inválido")
	}
	baseCred := adsCredentialV25{AccessToken: tok.AccessToken, RefreshToken: tok.RefreshToken, ExpiresAt: time.Now().UTC().Add(time.Duration(tok.ExpiresIn) * time.Second).Format(time.RFC3339)}
	roots, err := a.googleAccessibleCustomersV25(ctx, baseCred)
	if err != nil {
		return err
	}
	if len(roots) == 0 {
		return errors.New("Google Ads no devolvió clientes accesibles")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	created := map[string]bool{}
	upsert := func(id, name, curr, loginID string, manager bool) {
		id = strings.ReplaceAll(strings.TrimSpace(id), "-", "")
		if id == "" || created[id] {
			return
		}
		created[id] = true
		c := baseCred
		c.LoginCustomerID = strings.ReplaceAll(loginID, "-", "")
		cb, _ := json.Marshal(c)
		cfg, _ := json.Marshal(map[string]any{"manager": manager, "login_customer_id": c.LoginCustomerID})
		_, _ = a.db.Exec(`INSERT INTO ads_connections_v25(tenant_id,provider,account_id,account_name,currency,status,encrypted_credentials,config_json,created_at,updated_at) VALUES(?,?,?,?,?,'connected',?,?,?,?) ON CONFLICT(tenant_id,provider,account_id) DO UPDATE SET account_name=excluded.account_name,currency=excluded.currency,status='connected',encrypted_credentials=excluded.encrypted_credentials,config_json=excluded.config_json,last_error='',updated_at=excluded.updated_at`, tid, "google", id, firstNonEmpty(name, "Google Ads "+id), curr, encryptLocal(string(cb), a.cfg.ChannelEncryptionKey), string(cfg), now, now)
	}
	for _, root := range roots {
		root = strings.ReplaceAll(root, "-", "")
		name, curr, _ := a.googleCustomerInfoV25(ctx, &baseCred, root)
		upsert(root, name, curr, "", false)

		// Si la cuenta accesible es MCC, descubre clientes de primer nivel y guarda el MCC como login-customer-id.
		managerCred := baseCred
		managerCred.LoginCustomerID = root
		q := `SELECT customer_client.client_customer,customer_client.descriptive_name,customer_client.currency_code,customer_client.manager,customer_client.level,customer_client.status FROM customer_client WHERE customer_client.level <= 1`
		children, e := a.googleAdsQueryV25(ctx, managerCred, root, q)
		if e != nil {
			continue
		}
		for _, row := range children {
			cc, _ := row["customerClient"].(map[string]any)
			if cc == nil {
				continue
			}
			resource := stringAnyV25(cc["clientCustomer"])
			id := strings.TrimPrefix(resource, "customers/")
			level := int(numberAnyV25(cc["level"]))
			if id == "" || level == 0 {
				continue
			}
			upsert(id, stringAnyV25(cc["descriptiveName"]), stringAnyV25(cc["currencyCode"]), root, strings.EqualFold(stringAnyV25(cc["manager"]), "true"))
		}
	}
	if len(created) == 0 {
		return errors.New("no se pudieron registrar cuentas de Google Ads")
	}
	return nil
}

func (a *App) googleAccessibleCustomersV25(ctx context.Context, cred adsCredentialV25) ([]string, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://googleads.googleapis.com/v25/customers:listAccessibleCustomers", nil)
	req.Header.Set("Authorization", "Bearer "+cred.AccessToken)
	req.Header.Set("developer-token", a.cfg.GoogleAdsDeveloperToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 3<<20))
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("Google Ads clientes: %s", string(b))
	}
	var x struct {
		ResourceNames []string `json:"resourceNames"`
	}
	_ = json.Unmarshal(b, &x)
	out := []string{}
	for _, rn := range x.ResourceNames {
		id := strings.TrimPrefix(rn, "customers/")
		if id != "" {
			out = append(out, id)
		}
	}
	return out, nil
}

func (a *App) googleCustomerInfoV25(ctx context.Context, cred *adsCredentialV25, customerID string) (string, string, error) {
	if err := a.refreshGoogleAdsTokenV25(ctx, cred); err != nil {
		return "", "", err
	}
	q := `SELECT customer.descriptive_name, customer.currency_code FROM customer LIMIT 1`
	rows, err := a.googleAdsQueryV25(ctx, *cred, customerID, q)
	if err != nil {
		return "", "", err
	}
	if len(rows) == 0 {
		return "", "", nil
	}
	cust, _ := rows[0]["customer"].(map[string]any)
	return fmt.Sprint(cust["descriptiveName"]), fmt.Sprint(cust["currencyCode"]), nil
}

func (a *App) refreshGoogleAdsTokenV25(ctx context.Context, cred *adsCredentialV25) error {
	if cred.AccessToken != "" {
		if t, err := time.Parse(time.RFC3339, cred.ExpiresAt); err != nil || time.Until(t) > 2*time.Minute {
			return nil
		}
	}
	if cred.RefreshToken == "" {
		return errors.New("Google Ads refresh token no disponible")
	}
	vals := url.Values{"client_id": {a.cfg.GoogleClientID}, "client_secret": {a.cfg.GoogleClientSecret}, "refresh_token": {cred.RefreshToken}, "grant_type": {"refresh_token"}}
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://oauth2.googleapis.com/token", strings.NewReader(vals.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("Google refresh: %s", string(b))
	}
	var x struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if json.Unmarshal(b, &x) != nil || x.AccessToken == "" {
		return errors.New("Google refresh inválido")
	}
	cred.AccessToken = x.AccessToken
	cred.ExpiresAt = time.Now().UTC().Add(time.Duration(x.ExpiresIn) * time.Second).Format(time.RFC3339)
	return nil
}

func (a *App) adsV25SyncHandler(w http.ResponseWriter, r *http.Request) {
	tid, u, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	if !canManageAdsV25(u.Role) {
		writeError(w, errors.New("sin permisos"), 403)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	var q struct {
		ConnectionID int64 `json:"connection_id"`
		Days         int   `json:"days"`
	}
	_ = json.NewDecoder(r.Body).Decode(&q)
	if q.Days <= 0 || q.Days > 90 {
		q.Days = 30
	}
	n, err := a.syncAdsV25Tenant(r.Context(), tid, q.ConnectionID, q.Days)
	if err != nil {
		writeError(w, err, 502)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "rows_synced": n})
}

func (a *App) syncAdsV25Tenant(ctx context.Context, tid, connectionID int64, days int) (int, error) {
	q := `SELECT id,provider,account_id,encrypted_credentials FROM ads_connections_v25 WHERE tenant_id=? AND status='connected'`
	args := []any{tid}
	if connectionID > 0 {
		q += ` AND id=?`
		args = append(args, connectionID)
	}
	rows, err := a.db.Query(q, args...)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	total := 0
	var errs []string
	for rows.Next() {
		var id int64
		var p, account, enc string
		_ = rows.Scan(&id, &p, &account, &enc)
		start := time.Now().UTC()
		logRes, _ := a.db.Exec(`INSERT INTO ads_sync_log_v25(tenant_id,connection_id,provider,status,started_at) VALUES(?,?,?,'running',?)`, tid, id, p, start.Format(time.RFC3339))
		logID, _ := logRes.LastInsertId()
		n, e := a.syncAdsConnectionV25(ctx, tid, id, p, account, enc, days)
		status := "ok"
		em := ""
		if e != nil {
			status = "error"
			em = e.Error()
			errs = append(errs, p+": "+em)
			_, _ = a.db.Exec(`UPDATE ads_connections_v25 SET last_error=?,updated_at=? WHERE id=? AND tenant_id=?`, em, time.Now().UTC().Format(time.RFC3339), id, tid)
		} else {
			total += n
			now := time.Now().UTC().Format(time.RFC3339)
			_, _ = a.db.Exec(`UPDATE ads_connections_v25 SET last_sync_at=?,last_error='',updated_at=? WHERE id=? AND tenant_id=?`, now, now, id, tid)
		}
		_, _ = a.db.Exec(`UPDATE ads_sync_log_v25 SET status=?,rows_synced=?,error_message=?,finished_at=? WHERE id=?`, status, n, em, time.Now().UTC().Format(time.RFC3339), logID)
	}
	a.rebuildMarketingMetricsFromAdsV25(tid)
	if len(errs) > 0 && total == 0 {
		return total, errors.New(strings.Join(errs, " | "))
	}
	return total, nil
}

func (a *App) readAdsCredentialV25(enc string) (adsCredentialV25, error) {
	var c adsCredentialV25
	if json.Unmarshal([]byte(decryptLocal(enc, a.cfg.ChannelEncryptionKey)), &c) != nil {
		return c, errors.New("credenciales inválidas")
	}
	return c, nil
}

func (a *App) syncAdsConnectionV25(ctx context.Context, tid, connID int64, p, account, enc string, days int) (int, error) {
	// Conserva histórico/mapeos de campañas, pero deja explícito si una campaña ya no aparece en el proveedor.
	_, _ = a.db.Exec(`UPDATE ads_campaigns_v25 SET status='ARCHIVED_OR_REMOVED',updated_at=? WHERE tenant_id=? AND connection_id=?`, time.Now().UTC().Format(time.RFC3339), tid, connID)
	// Grupos/anuncios son inventario actual; se reconstruyen en cada sincronización.
	_, _ = a.db.Exec(`DELETE FROM ads_entities_v25 WHERE tenant_id=? AND connection_id=?`, tid, connID)
	cred, err := a.readAdsCredentialV25(enc)
	if err != nil {
		return 0, err
	}
	from := time.Now().UTC().AddDate(0, 0, -days+1).Format("2006-01-02")
	to := time.Now().UTC().Format("2006-01-02")
	switch p {
	case "meta":
		return a.syncMetaAdsV25(ctx, tid, connID, account, cred, from, to)
	case "tiktok":
		return a.syncTikTokAdsV25(ctx, tid, connID, account, cred, from, to)
	case "google":
		n, e := a.syncGoogleAdsV25(ctx, tid, connID, account, &cred, from, to)
		if e == nil {
			cb, _ := json.Marshal(cred)
			_, _ = a.db.Exec(`UPDATE ads_connections_v25 SET encrypted_credentials=? WHERE id=? AND tenant_id=?`, encryptLocal(string(cb), a.cfg.ChannelEncryptionKey), connID, tid)
		}
		return n, e
	}
	return 0, errors.New("proveedor no soportado")
}

func upsertAdsEntityV25(db *DB, tid, connID int64, provider, account, entityType, externalID, campaignID, groupID, name, status, currency string, budget float64, raw any) error {
	if strings.TrimSpace(externalID) == "" {
		return nil
	}
	b, _ := json.Marshal(raw)
	_, err := db.Exec(`INSERT INTO ads_entities_v25(tenant_id,connection_id,provider,account_id,entity_type,external_id,external_campaign_id,external_group_id,name,status,budget,currency,raw_json,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(tenant_id,provider,account_id,entity_type,external_id) DO UPDATE SET connection_id=excluded.connection_id,external_campaign_id=excluded.external_campaign_id,external_group_id=excluded.external_group_id,name=excluded.name,status=excluded.status,budget=excluded.budget,currency=excluded.currency,raw_json=excluded.raw_json,updated_at=excluded.updated_at`, tid, connID, provider, account, entityType, externalID, campaignID, groupID, name, status, budget, currency, string(b), time.Now().UTC().Format(time.RFC3339))
	return err
}

func upsertAdsCampaignV25(db *DB, tid, connID int64, p, account, extID, name, status, objective, channel, currency string, daily, total float64, raw any) (int64, error) {
	b, _ := json.Marshal(raw)
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(`INSERT INTO ads_campaigns_v25(tenant_id,connection_id,provider,account_id,external_campaign_id,name,status,objective,channel_type,budget_daily,budget_total,currency,raw_json,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(tenant_id,provider,account_id,external_campaign_id) DO UPDATE SET connection_id=excluded.connection_id,name=excluded.name,status=excluded.status,objective=excluded.objective,channel_type=excluded.channel_type,budget_daily=excluded.budget_daily,budget_total=excluded.budget_total,currency=excluded.currency,raw_json=excluded.raw_json,updated_at=excluded.updated_at`, tid, connID, p, account, extID, name, status, objective, channel, daily, total, currency, string(b), now, now)
	if err != nil {
		return 0, err
	}
	var id int64
	err = db.QueryRow(`SELECT id FROM ads_campaigns_v25 WHERE tenant_id=? AND provider=? AND account_id=? AND external_campaign_id=?`, tid, p, account, extID).Scan(&id)
	return id, err
}
func upsertAdsMetricV25(db *DB, tid, campaignRow int64, p, account, extID, date string, spend float64, impr, clicks, reach int64, conv, val, leads float64, raw any) error {
	b, _ := json.Marshal(raw)
	_, err := db.Exec(`INSERT INTO ads_daily_metrics_v25(tenant_id,campaign_row_id,provider,account_id,external_campaign_id,metric_date,spend,impressions,clicks,reach,conversions,conversion_value,leads,raw_json,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(tenant_id,provider,account_id,external_campaign_id,metric_date) DO UPDATE SET campaign_row_id=excluded.campaign_row_id,spend=excluded.spend,impressions=excluded.impressions,clicks=excluded.clicks,reach=excluded.reach,conversions=excluded.conversions,conversion_value=excluded.conversion_value,leads=excluded.leads,raw_json=excluded.raw_json,updated_at=excluded.updated_at`, tid, campaignRow, p, account, extID, date, spend, impr, clicks, reach, conv, val, leads, string(b), time.Now().UTC().Format(time.RFC3339))
	return err
}

func numberAnyV25(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case json.Number:
		n, _ := x.Float64()
		return n
	case string:
		n, _ := strconv.ParseFloat(strings.ReplaceAll(x, ",", ""), 64)
		return n
	}
	return 0
}
func stringAnyV25(v any) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

func metaPagedMapsV25(ctx context.Context, endpoint string, maxPages int) ([]map[string]any, error) {
	if maxPages <= 0 {
		maxPages = 20
	}
	out := []map[string]any{}
	next := endpoint
	for page := 0; page < maxPages && next != ""; page++ {
		var x struct {
			Data   []map[string]any `json:"data"`
			Paging struct {
				Next string `json:"next"`
			} `json:"paging"`
		}
		if err := apiJSON(ctx, "GET", next, "", nil, &x); err != nil {
			return out, err
		}
		out = append(out, x.Data...)
		next = strings.TrimSpace(x.Paging.Next)
	}
	return out, nil
}

func (a *App) syncMetaAdsV25(ctx context.Context, tid, connID int64, account string, cred adsCredentialV25, from, to string) (int, error) {
	fields := "id,name,status,effective_status,objective,daily_budget,lifetime_budget"
	ep := fmt.Sprintf("https://graph.facebook.com/%s/%s/campaigns?fields=%s&limit=200&access_token=%s", a.cfg.MetaGraphVersion, url.PathEscape(account), url.QueryEscape(fields), url.QueryEscape(cred.AccessToken))
	camps, err := metaPagedMapsV25(ctx, ep, 30)
	if err != nil {
		return 0, err
	}
	ids := map[string]int64{}
	for _, c := range camps {
		id := stringAnyV25(c["id"])
		row, err := upsertAdsCampaignV25(a.db, tid, connID, "meta", account, id, stringAnyV25(c["name"]), firstNonEmpty(stringAnyV25(c["effective_status"]), stringAnyV25(c["status"])), stringAnyV25(c["objective"]), "META", a.connectionCurrencyV25(tid, connID), numberAnyV25(c["daily_budget"])/100, numberAnyV25(c["lifetime_budget"])/100, c)
		if err == nil {
			ids[id] = row
		}
	}
	tr := url.QueryEscape(fmt.Sprintf(`{"since":"%s","until":"%s"}`, from, to))
	f := url.QueryEscape("campaign_id,campaign_name,impressions,reach,clicks,spend,actions,action_values,date_start,date_stop")
	iep := fmt.Sprintf("https://graph.facebook.com/%s/%s/insights?level=campaign&time_increment=1&time_range=%s&fields=%s&limit=1000&access_token=%s", a.cfg.MetaGraphVersion, url.PathEscape(account), tr, f, url.QueryEscape(cred.AccessToken))
	ins, err := metaPagedMapsV25(ctx, iep, 50)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, m := range ins {
		cid := stringAnyV25(m["campaign_id"])
		row := ids[cid]
		if row == 0 {
			row, _ = upsertAdsCampaignV25(a.db, tid, connID, "meta", account, cid, stringAnyV25(m["campaign_name"]), "", "", "META", a.connectionCurrencyV25(tid, connID), 0, 0, m)
		}
		leads, conv, val := metaActionsV25(m)
		_ = upsertAdsMetricV25(a.db, tid, row, "meta", account, cid, stringAnyV25(m["date_start"]), numberAnyV25(m["spend"]), int64(numberAnyV25(m["impressions"])), int64(numberAnyV25(m["clicks"])), int64(numberAnyV25(m["reach"])), conv, val, leads, m)
		n++
	}
	if err := a.syncMetaAdsEntitiesV25(ctx, tid, connID, account, cred); err != nil {
		return n, err
	}
	return n, nil
}

func (a *App) syncMetaAdsEntitiesV25(ctx context.Context, tid, connID int64, account string, cred adsCredentialV25) error {
	currency := a.connectionCurrencyV25(tid, connID)
	adsetsURL := fmt.Sprintf("https://graph.facebook.com/%s/%s/adsets?fields=id,name,campaign_id,status,effective_status,daily_budget,lifetime_budget&limit=200&access_token=%s", a.cfg.MetaGraphVersion, url.PathEscape(account), url.QueryEscape(cred.AccessToken))
	sets, err := metaPagedMapsV25(ctx, adsetsURL, 50)
	if err != nil {
		return err
	}
	for _, x := range sets {
		budget := numberAnyV25(x["daily_budget"]) / 100
		if budget == 0 {
			budget = numberAnyV25(x["lifetime_budget"]) / 100
		}
		_ = upsertAdsEntityV25(a.db, tid, connID, "meta", account, "adgroup", stringAnyV25(x["id"]), stringAnyV25(x["campaign_id"]), "", stringAnyV25(x["name"]), firstNonEmpty(stringAnyV25(x["effective_status"]), stringAnyV25(x["status"])), currency, budget, x)
	}
	adsURL := fmt.Sprintf("https://graph.facebook.com/%s/%s/ads?fields=id,name,campaign_id,adset_id,status,effective_status&limit=200&access_token=%s", a.cfg.MetaGraphVersion, url.PathEscape(account), url.QueryEscape(cred.AccessToken))
	ads, err := metaPagedMapsV25(ctx, adsURL, 50)
	if err != nil {
		return err
	}
	for _, x := range ads {
		_ = upsertAdsEntityV25(a.db, tid, connID, "meta", account, "ad", stringAnyV25(x["id"]), stringAnyV25(x["campaign_id"]), stringAnyV25(x["adset_id"]), stringAnyV25(x["name"]), firstNonEmpty(stringAnyV25(x["effective_status"]), stringAnyV25(x["status"])), currency, 0, x)
	}
	return nil
}

func metaActionsV25(m map[string]any) (leads, conv, val float64) {
	if xs, ok := m["actions"].([]any); ok {
		for _, z := range xs {
			q, _ := z.(map[string]any)
			typ := strings.ToLower(stringAnyV25(q["action_type"]))
			v := numberAnyV25(q["value"])
			if strings.Contains(typ, "lead") {
				leads += v
			}
			if strings.Contains(typ, "purchase") || strings.Contains(typ, "lead") {
				conv += v
			}
		}
	}
	if xs, ok := m["action_values"].([]any); ok {
		for _, z := range xs {
			q, _ := z.(map[string]any)
			if strings.Contains(strings.ToLower(stringAnyV25(q["action_type"])), "purchase") {
				val += numberAnyV25(q["value"])
			}
		}
	}
	return
}

func (a *App) syncTikTokAdsV25(ctx context.Context, tid, connID int64, account string, cred adsCredentialV25, from, to string) (int, error) {
	ep := "https://business-api.tiktok.com/open_api/v1.3/campaign/get/?advertiser_id=" + url.QueryEscape(account) + "&page_size=1000"
	req, _ := http.NewRequestWithContext(ctx, "GET", ep, nil)
	req.Header.Set("Access-Token", cred.AccessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
	resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return 0, fmt.Errorf("TikTok campañas: %s", string(b))
	}
	var cr struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			List []map[string]any `json:"list"`
		} `json:"data"`
	}
	_ = json.Unmarshal(b, &cr)
	if cr.Code != 0 {
		return 0, errors.New(cr.Message)
	}
	ids := map[string]int64{}
	curr := a.connectionCurrencyV25(tid, connID)
	for _, c := range cr.Data.List {
		cid := stringAnyV25(c["campaign_id"])
		row, _ := upsertAdsCampaignV25(a.db, tid, connID, "tiktok", account, cid, stringAnyV25(c["campaign_name"]), firstNonEmpty(stringAnyV25(c["operation_status"]), stringAnyV25(c["secondary_status"])), stringAnyV25(c["objective_type"]), "TIKTOK", curr, numberAnyV25(c["budget"]), 0, c)
		ids[cid] = row
	}
	dims, _ := json.Marshal([]string{"campaign_id", "stat_time_day"})
	metrics, _ := json.Marshal([]string{"campaign_name", "spend", "impressions", "clicks", "conversion"})
	q := url.Values{"advertiser_id": {account}, "report_type": {"BASIC"}, "service_type": {"AUCTION"}, "data_level": {"AUCTION_CAMPAIGN"}, "dimensions": {string(dims)}, "metrics": {string(metrics)}, "start_date": {from}, "end_date": {to}, "page": {"1"}, "page_size": {"1000"}}
	rep := "https://business-api.tiktok.com/open_api/v1.3/report/integrated/get/?" + q.Encode()
	req, _ = http.NewRequestWithContext(ctx, "GET", rep, nil)
	req.Header.Set("Access-Token", cred.AccessToken)
	rr, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	bb, _ := io.ReadAll(io.LimitReader(rr.Body, 8<<20))
	rr.Body.Close()
	if rr.StatusCode/100 != 2 {
		return 0, fmt.Errorf("TikTok report: %s", string(bb))
	}
	var rp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			List []struct {
				Dimensions map[string]any `json:"dimensions"`
				Metrics    map[string]any `json:"metrics"`
			} `json:"list"`
		} `json:"data"`
	}
	_ = json.Unmarshal(bb, &rp)
	if rp.Code != 0 {
		return 0, errors.New(rp.Message)
	}
	n := 0
	for _, x := range rp.Data.List {
		cid := stringAnyV25(x.Dimensions["campaign_id"])
		date := stringAnyV25(x.Dimensions["stat_time_day"])
		row := ids[cid]
		if row == 0 {
			row, _ = upsertAdsCampaignV25(a.db, tid, connID, "tiktok", account, cid, stringAnyV25(x.Metrics["campaign_name"]), "", "", "TIKTOK", curr, 0, 0, x)
		}
		conv := numberAnyV25(x.Metrics["conversion"])
		raw := map[string]any{"dimensions": x.Dimensions, "metrics": x.Metrics}
		_ = upsertAdsMetricV25(a.db, tid, row, "tiktok", account, cid, date, numberAnyV25(x.Metrics["spend"]), int64(numberAnyV25(x.Metrics["impressions"])), int64(numberAnyV25(x.Metrics["clicks"])), 0, conv, 0, conv, raw)
		n++
	}
	if err := a.syncTikTokAdsEntitiesV25(ctx, tid, connID, account, cred); err != nil {
		return n, err
	}
	return n, nil
}

func (a *App) syncTikTokAdsEntitiesV25(ctx context.Context, tid, connID int64, account string, cred adsCredentialV25) error {
	currency := a.connectionCurrencyV25(tid, connID)
	for _, typ := range []string{"adgroup", "ad"} {
		ep := fmt.Sprintf("https://business-api.tiktok.com/open_api/v1.3/%s/get/?advertiser_id=%s&page_size=1000", typ, url.QueryEscape(account))
		req, _ := http.NewRequestWithContext(ctx, "GET", ep, nil)
		req.Header.Set("Access-Token", cred.AccessToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
		resp.Body.Close()
		if resp.StatusCode/100 != 2 {
			return fmt.Errorf("TikTok %s: %s", typ, string(b))
		}
		var x struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Data    struct {
				List []map[string]any `json:"list"`
			} `json:"data"`
		}
		_ = json.Unmarshal(b, &x)
		if x.Code != 0 {
			return errors.New(x.Message)
		}
		for _, e := range x.Data.List {
			if typ == "adgroup" {
				_ = upsertAdsEntityV25(a.db, tid, connID, "tiktok", account, "adgroup", stringAnyV25(e["adgroup_id"]), stringAnyV25(e["campaign_id"]), "", stringAnyV25(e["adgroup_name"]), firstNonEmpty(stringAnyV25(e["operation_status"]), stringAnyV25(e["secondary_status"])), currency, numberAnyV25(e["budget"]), e)
			} else {
				_ = upsertAdsEntityV25(a.db, tid, connID, "tiktok", account, "ad", stringAnyV25(e["ad_id"]), stringAnyV25(e["campaign_id"]), stringAnyV25(e["adgroup_id"]), stringAnyV25(e["ad_name"]), firstNonEmpty(stringAnyV25(e["operation_status"]), stringAnyV25(e["secondary_status"])), currency, 0, e)
			}
		}
	}
	return nil
}

func (a *App) googleAdsQueryV25(ctx context.Context, cred adsCredentialV25, customerID, query string) ([]map[string]any, error) {
	b, _ := json.Marshal(map[string]any{"query": query})
	ep := "https://googleads.googleapis.com/v25/customers/" + strings.ReplaceAll(customerID, "-", "") + "/googleAds:searchStream"
	req, _ := http.NewRequestWithContext(ctx, "POST", ep, bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+cred.AccessToken)
	req.Header.Set("developer-token", a.cfg.GoogleAdsDeveloperToken)
	if cred.LoginCustomerID != "" {
		req.Header.Set("login-customer-id", strings.ReplaceAll(cred.LoginCustomerID, "-", ""))
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(io.LimitReader(resp.Body, 12<<20))
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("Google Ads: %s", string(rb))
	}
	var chunks []struct {
		Results []map[string]any `json:"results"`
	}
	if json.Unmarshal(rb, &chunks) != nil {
		return nil, errors.New("respuesta Google Ads inválida")
	}
	out := []map[string]any{}
	for _, c := range chunks {
		out = append(out, c.Results...)
	}
	return out, nil
}

func (a *App) syncGoogleAdsV25(ctx context.Context, tid, connID int64, account string, cred *adsCredentialV25, from, to string) (int, error) {
	if err := a.refreshGoogleAdsTokenV25(ctx, cred); err != nil {
		return 0, err
	}
	currency := a.connectionCurrencyV25(tid, connID)
	q := fmt.Sprintf(`SELECT campaign.id,campaign.name,campaign.status,campaign.advertising_channel_type,campaign.advertising_channel_sub_type,campaign_budget.amount_micros,segments.date,metrics.impressions,metrics.clicks,metrics.cost_micros,metrics.conversions,metrics.conversions_value FROM campaign WHERE campaign.status != 'REMOVED' AND segments.date BETWEEN '%s' AND '%s'`, from, to)
	rows, err := a.googleAdsQueryV25(ctx, *cred, account, q)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, r := range rows {
		camp, _ := r["campaign"].(map[string]any)
		budget, _ := r["campaignBudget"].(map[string]any)
		seg, _ := r["segments"].(map[string]any)
		met, _ := r["metrics"].(map[string]any)
		cid := stringAnyV25(camp["id"])
		channel := stringAnyV25(camp["advertisingChannelType"])
		row, _ := upsertAdsCampaignV25(a.db, tid, connID, "google", account, cid, stringAnyV25(camp["name"]), stringAnyV25(camp["status"]), stringAnyV25(camp["advertisingChannelSubType"]), channel, currency, numberAnyV25(budget["amountMicros"])/1e6, 0, r)
		conv := numberAnyV25(met["conversions"])
		_ = upsertAdsMetricV25(a.db, tid, row, "google", account, cid, stringAnyV25(seg["date"]), numberAnyV25(met["costMicros"])/1e6, int64(numberAnyV25(met["impressions"])), int64(numberAnyV25(met["clicks"])), 0, conv, numberAnyV25(met["conversionsValue"]), 0, r)
		n++
	}
	if err := a.syncGoogleAdsEntitiesV25(ctx, tid, connID, account, cred); err != nil {
		return n, err
	}
	return n, nil
}

func (a *App) syncGoogleAdsEntitiesV25(ctx context.Context, tid, connID int64, account string, cred *adsCredentialV25) error {
	if err := a.refreshGoogleAdsTokenV25(ctx, cred); err != nil {
		return err
	}
	currency := a.connectionCurrencyV25(tid, connID)
	groups, err := a.googleAdsQueryV25(ctx, *cred, account, `SELECT campaign.id,ad_group.id,ad_group.name,ad_group.status,ad_group.cpc_bid_micros FROM ad_group WHERE ad_group.status != 'REMOVED'`)
	if err != nil {
		return err
	}
	for _, r := range groups {
		camp, _ := r["campaign"].(map[string]any)
		g, _ := r["adGroup"].(map[string]any)
		_ = upsertAdsEntityV25(a.db, tid, connID, "google", account, "adgroup", stringAnyV25(g["id"]), stringAnyV25(camp["id"]), "", stringAnyV25(g["name"]), stringAnyV25(g["status"]), currency, numberAnyV25(g["cpcBidMicros"])/1e6, r)
	}
	ads, err := a.googleAdsQueryV25(ctx, *cred, account, `SELECT campaign.id,ad_group.id,ad_group_ad.ad.id,ad_group_ad.ad.type,ad_group_ad.status FROM ad_group_ad WHERE ad_group_ad.status != 'REMOVED'`)
	if err != nil {
		return err
	}
	for _, r := range ads {
		camp, _ := r["campaign"].(map[string]any)
		g, _ := r["adGroup"].(map[string]any)
		aga, _ := r["adGroupAd"].(map[string]any)
		ad, _ := aga["ad"].(map[string]any)
		id := stringAnyV25(ad["id"])
		name := stringAnyV25(ad["type"])
		if name != "" {
			name += " · " + id
		} else {
			name = id
		}
		_ = upsertAdsEntityV25(a.db, tid, connID, "google", account, "ad", id, stringAnyV25(camp["id"]), stringAnyV25(g["id"]), name, stringAnyV25(aga["status"]), currency, 0, r)
	}
	return nil
}

func (a *App) connectionCurrencyV25(tid, connID int64) string {
	var c string
	_ = a.db.QueryRow(`SELECT currency FROM ads_connections_v25 WHERE tenant_id=? AND id=?`, tid, connID).Scan(&c)
	return c
}

func (a *App) rebuildMarketingMetricsFromAdsV25(tid int64) {
	var reportCurrency string
	_ = a.db.QueryRow(`SELECT currency FROM analytics_settings_v24 WHERE tenant_id=?`, tid).Scan(&reportCurrency)
	if reportCurrency == "" {
		reportCurrency = "USD"
	}
	rows, err := a.db.Query(`SELECT c.internal_campaign_id,m.metric_date,SUM(m.spend),SUM(m.impressions),SUM(m.clicks),SUM(m.leads),SUM(m.conversions),SUM(m.conversion_value) FROM ads_daily_metrics_v25 m JOIN ads_campaigns_v25 c ON c.id=m.campaign_row_id AND c.tenant_id=m.tenant_id WHERE m.tenant_id=? AND c.internal_campaign_id>0 AND (c.currency='' OR UPPER(c.currency)=UPPER(?)) GROUP BY c.internal_campaign_id,m.metric_date`, tid, reportCurrency)
	if err != nil {
		return
	}
	defer rows.Close()
	nowRows := []struct {
		cid           int64
		date          string
		sp            float64
		imp, cl       int64
		le, conv, val float64
	}{}
	for rows.Next() {
		var x struct {
			cid           int64
			date          string
			sp            float64
			imp, cl       int64
			le, conv, val float64
		}
		_ = rows.Scan(&x.cid, &x.date, &x.sp, &x.imp, &x.cl, &x.le, &x.conv, &x.val)
		nowRows = append(nowRows, x)
	}
	for _, x := range nowRows {
		// El gasto/clics/leads provienen del proveedor. Ventas e ingresos se mantienen en CRM/Analytics V24.
		_, _ = a.db.Exec(`INSERT INTO marketing_metrics(tenant_id,campaign_id,metric_date,spend,impressions,reach,clicks,leads,conversations,appointments,sales,revenue) VALUES(?,?,?,?,?,0,?,?,0,0,0,0) ON CONFLICT(tenant_id,campaign_id,metric_date) DO UPDATE SET spend=excluded.spend,impressions=excluded.impressions,clicks=excluded.clicks,leads=excluded.leads`, tid, x.cid, x.date, x.sp, x.imp, x.cl, int64(x.le))
	}
}

type adsWorkticMetricsV25 struct {
	Leads        int64   `json:"leads"`
	Appointments int64   `json:"appointments"`
	Sales        int64   `json:"sales"`
	Revenue      float64 `json:"revenue"`
}

func (a *App) adsWorkticMetricsV25(tid, campaignID int64, from, to, model string) adsWorkticMetricsV25 {
	if campaignID <= 0 {
		return adsWorkticMetricsV25{}
	}
	_, campaignExpr, _ := leadAttributionExprV24(model)
	start := from + "T00:00:00"
	end := to + "T23:59:59"
	var m adsWorkticMetricsV25
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM marketing_leads l WHERE l.tenant_id=? AND `+campaignExpr+`=? AND l.created_at>=? AND l.created_at<=?`, tid, campaignID, start, end).Scan(&m.Leads)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM crm_appointments ap LEFT JOIN crm_opportunities o ON o.tenant_id=ap.tenant_id AND o.id=ap.opportunity_id WHERE ap.tenant_id=? AND ap.created_at>=? AND ap.created_at<=? AND (ap.campaign_id=? OR EXISTS(SELECT 1 FROM marketing_leads l WHERE l.tenant_id=ap.tenant_id AND l.id=o.lead_id AND `+campaignExpr+`=?))`, tid, start, end, campaignID, campaignID).Scan(&m.Appointments)
	_ = a.db.QueryRow(`SELECT COUNT(*),COALESCE(SUM(o.value),0) FROM crm_opportunities o WHERE o.tenant_id=? AND COALESCE(o.deleted_at,'')='' AND (EXISTS(SELECT 1 FROM marketing_leads l WHERE l.tenant_id=o.tenant_id AND l.id=o.lead_id AND `+campaignExpr+`=?) OR (o.lead_id=0 AND o.campaign_id=?)) AND o.stage='Ganado' AND COALESCE(NULLIF(o.closed_at,''),o.updated_at)>=? AND COALESCE(NULLIF(o.closed_at,''),o.updated_at)<=?`, tid, campaignID, campaignID, start, end).Scan(&m.Sales, &m.Revenue)
	return m
}

func normalizeAttributionModelV25(v string) string {
	if strings.EqualFold(strings.TrimSpace(v), "first_touch") {
		return "first_touch"
	}
	return "last_touch"
}

func (a *App) adsV25CampaignsHandler(w http.ResponseWriter, r *http.Request) {
	tid, u, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", 405)
		return
	}
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	if from == "" {
		from = time.Now().UTC().AddDate(0, 0, -29).Format("2006-01-02")
	}
	if to == "" {
		to = time.Now().UTC().Format("2006-01-02")
	}
	provider := strings.TrimSpace(r.URL.Query().Get("provider"))
	model := normalizeAttributionModelV25(r.URL.Query().Get("model"))
	var reportCurrency string
	_ = a.db.QueryRow(`SELECT currency FROM analytics_settings_v24 WHERE tenant_id=?`, tid).Scan(&reportCurrency)
	if reportCurrency == "" {
		reportCurrency = "USD"
	}
	q := `SELECT c.id,c.provider,c.account_id,c.external_campaign_id,c.name,c.status,c.objective,c.channel_type,c.currency,c.internal_campaign_id,COALESCE(SUM(m.spend),0),COALESCE(SUM(m.impressions),0),COALESCE(SUM(m.clicks),0),COALESCE(SUM(m.conversions),0),COALESCE(SUM(m.conversion_value),0),COALESCE(SUM(m.leads),0) FROM ads_campaigns_v25 c LEFT JOIN ads_daily_metrics_v25 m ON m.campaign_row_id=c.id AND m.tenant_id=c.tenant_id AND m.metric_date>=? AND m.metric_date<=? WHERE c.tenant_id=?`
	args := []any{from, to, tid}
	if provider != "" && provider != "all" {
		q += ` AND c.provider=?`
		args = append(args, provider)
	}
	q += ` GROUP BY c.id ORDER BY 11 DESC,c.name`
	rows, err := a.db.Query(q, args...)
	if err != nil {
		writeError(w, err, 500)
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, internal, imp, clicks int64
		var p, account, ext, name, status, obj, ch, curr string
		var spend, conv, val, leads float64
		_ = rows.Scan(&id, &p, &account, &ext, &name, &status, &obj, &ch, &curr, &internal, &spend, &imp, &clicks, &conv, &val, &leads)
		var internalName string
		if internal > 0 {
			_ = a.db.QueryRow(`SELECT name FROM marketing_campaigns WHERE tenant_id=? AND id=?`, tid, internal).Scan(&internalName)
		}
		wm := a.adsWorkticMetricsV25(tid, internal, from, to, model)
		currencyMatch := curr == "" || strings.EqualFold(curr, reportCurrency)
		wtCPL, wtCAC, wtROAS := 0.0, 0.0, 0.0
		if currencyMatch {
			wtCPL = ratioV24(spend, float64(wm.Leads))
			wtCAC = ratioV24(spend, float64(wm.Sales))
			wtROAS = ratioV24(wm.Revenue, spend)
		}
		items = append(items, map[string]any{"id": id, "provider": p, "account_id": account, "external_campaign_id": ext, "name": name, "status": status, "objective": obj, "channel_type": ch, "currency": curr, "internal_campaign_id": internal, "internal_campaign_name": internalName, "spend": spend, "impressions": imp, "clicks": clicks, "conversions": conv, "conversion_value": val, "platform_leads": leads, "platform_roas": ratioV24(val, spend), "ctr": ratioV24(float64(clicks)*100, float64(imp)), "worktic_leads": wm.Leads, "worktic_appointments": wm.Appointments, "worktic_sales": wm.Sales, "worktic_revenue": wm.Revenue, "worktic_cpl": wtCPL, "worktic_cac": wtCAC, "worktic_roas": wtROAS, "currency_match": currencyMatch, "report_currency": reportCurrency, "can_manage": canManageAdsV25(u.Role) && !(p == "google" && strings.EqualFold(ch, "VIDEO"))})
	}
	writeJSON(w, map[string]any{"items": items, "from": from, "to": to, "model": model})
}

func (a *App) adsV25OverviewHandler(w http.ResponseWriter, r *http.Request) {
	tid, _, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	if from == "" {
		from = time.Now().UTC().AddDate(0, 0, -29).Format("2006-01-02")
	}
	if to == "" {
		to = time.Now().UTC().Format("2006-01-02")
	}
	model := normalizeAttributionModelV25(r.URL.Query().Get("model"))
	var reportCurrency string
	_ = a.db.QueryRow(`SELECT currency FROM analytics_settings_v24 WHERE tenant_id=?`, tid).Scan(&reportCurrency)
	if reportCurrency == "" {
		reportCurrency = "USD"
	}
	var accounts, campaigns int64
	var spend, conv, val, leads float64
	var impr, clicks int64
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM ads_connections_v25 WHERE tenant_id=? AND status='connected'`, tid).Scan(&accounts)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM ads_campaigns_v25 WHERE tenant_id=?`, tid).Scan(&campaigns)
	_ = a.db.QueryRow(`SELECT COALESCE(SUM(m.spend),0),COALESCE(SUM(m.impressions),0),COALESCE(SUM(m.clicks),0),COALESCE(SUM(m.conversions),0),COALESCE(SUM(m.conversion_value),0),COALESCE(SUM(m.leads),0) FROM ads_daily_metrics_v25 m JOIN ads_campaigns_v25 c ON c.id=m.campaign_row_id AND c.tenant_id=m.tenant_id WHERE m.tenant_id=? AND m.metric_date>=? AND m.metric_date<=? AND (c.currency='' OR UPPER(c.currency)=UPPER(?))`, tid, from, to, reportCurrency).Scan(&spend, &impr, &clicks, &conv, &val, &leads)
	providers := []map[string]any{}
	rows, _ := a.db.Query(`SELECT m.provider,COUNT(DISTINCT m.account_id),COALESCE(SUM(m.spend),0),COALESCE(SUM(m.impressions),0),COALESCE(SUM(m.clicks),0),COALESCE(SUM(m.conversions),0),COALESCE(SUM(m.conversion_value),0) FROM ads_daily_metrics_v25 m JOIN ads_campaigns_v25 c ON c.id=m.campaign_row_id AND c.tenant_id=m.tenant_id WHERE m.tenant_id=? AND m.metric_date>=? AND m.metric_date<=? AND (c.currency='' OR UPPER(c.currency)=UPPER(?)) GROUP BY m.provider ORDER BY 3 DESC`, tid, from, to, reportCurrency)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var p string
			var ac, im, cl int64
			var sp, co, va float64
			_ = rows.Scan(&p, &ac, &sp, &im, &cl, &co, &va)
			providers = append(providers, map[string]any{"provider": p, "accounts": ac, "spend": sp, "impressions": im, "clicks": cl, "conversions": co, "conversion_value": va, "cpc": ratioV24(sp, float64(cl)), "cpa": ratioV24(sp, co), "platform_roas": ratioV24(va, sp)})
		}
	}
	var otherCurrencyCampaigns int64
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM ads_campaigns_v25 WHERE tenant_id=? AND currency<>'' AND UPPER(currency)<>UPPER(?)`, tid, reportCurrency).Scan(&otherCurrencyCampaigns)
	// Resultados comerciales reales de Worktic, agregados una sola vez por campaña interna mapeada.
	var wtLeads, wtAppointments, wtSales int64
	var wtRevenue float64
	mapRows, _ := a.db.Query(`SELECT DISTINCT internal_campaign_id FROM ads_campaigns_v25 WHERE tenant_id=? AND internal_campaign_id>0`, tid)
	if mapRows != nil {
		for mapRows.Next() {
			var cid int64
			_ = mapRows.Scan(&cid)
			m := a.adsWorkticMetricsV25(tid, cid, from, to, model)
			wtLeads += m.Leads
			wtAppointments += m.Appointments
			wtSales += m.Sales
			wtRevenue += m.Revenue
		}
		mapRows.Close()
	}
	writeJSON(w, map[string]any{"from": from, "to": to, "model": model, "currency": reportCurrency, "other_currency_campaigns": otherCurrencyCampaigns, "kpis": map[string]any{"accounts": accounts, "campaigns": campaigns, "spend": spend, "impressions": impr, "clicks": clicks, "conversions": conv, "conversion_value": val, "platform_leads": leads, "ctr": ratioV24(float64(clicks)*100, float64(impr)), "cpc": ratioV24(spend, float64(clicks)), "cpa": ratioV24(spend, conv), "platform_roas": ratioV24(val, spend), "worktic_leads": wtLeads, "worktic_appointments": wtAppointments, "worktic_sales": wtSales, "worktic_revenue": wtRevenue, "worktic_cpl": ratioV24(spend, float64(wtLeads)), "worktic_cac": ratioV24(spend, float64(wtSales)), "worktic_roas": ratioV24(wtRevenue, spend)}, "providers": providers})
}

func (a *App) adsV25MappingHandler(w http.ResponseWriter, r *http.Request) {
	tid, u, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	if !canManageAdsV25(u.Role) {
		writeError(w, errors.New("sin permisos"), 403)
		return
	}
	if r.Method != http.MethodPut {
		http.Error(w, "Método no permitido", 405)
		return
	}
	var q struct {
		CampaignID         int64 `json:"campaign_id"`
		InternalCampaignID int64 `json:"internal_campaign_id"`
	}
	if json.NewDecoder(r.Body).Decode(&q) != nil {
		writeError(w, errors.New("JSON inválido"), 400)
		return
	}
	_, err = a.db.Exec(`UPDATE ads_campaigns_v25 SET internal_campaign_id=?,updated_at=? WHERE tenant_id=? AND id=?`, q.InternalCampaignID, time.Now().UTC().Format(time.RFC3339), tid, q.CampaignID)
	if err != nil {
		writeError(w, err, 500)
		return
	}
	a.rebuildMarketingMetricsFromAdsV25(tid)
	writeJSON(w, map[string]any{"ok": true})
}

func (a *App) adsV25CampaignStatusHandler(w http.ResponseWriter, r *http.Request) {
	tid, u, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	if !canManageAdsV25(u.Role) {
		writeError(w, errors.New("sin permisos"), 403)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	var q struct {
		ID     int64  `json:"id"`
		Status string `json:"status"`
	}
	if json.NewDecoder(r.Body).Decode(&q) != nil {
		writeError(w, errors.New("JSON inválido"), 400)
		return
	}
	var p, account, ext, ch, enc string
	var connID int64
	if a.db.QueryRow(`SELECT c.provider,c.account_id,c.external_campaign_id,c.channel_type,c.connection_id,x.encrypted_credentials FROM ads_campaigns_v25 c JOIN ads_connections_v25 x ON x.id=c.connection_id AND x.tenant_id=c.tenant_id WHERE c.tenant_id=? AND c.id=?`, tid, q.ID).Scan(&p, &account, &ext, &ch, &connID, &enc) != nil {
		writeError(w, errors.New("campaña no encontrada"), 404)
		return
	}
	cred, e := a.readAdsCredentialV25(enc)
	if e != nil {
		writeError(w, e, 500)
		return
	}
	desired := strings.ToUpper(strings.TrimSpace(q.Status))
	if desired != "ACTIVE" && desired != "PAUSED" {
		writeError(w, errors.New("estado inválido"), 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	switch p {
	case "meta":
		vals := url.Values{"status": {desired}, "access_token": {cred.AccessToken}}
		req, _ := http.NewRequestWithContext(ctx, "POST", "https://graph.facebook.com/"+a.cfg.MetaGraphVersion+"/"+url.PathEscape(ext), strings.NewReader(vals.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr, e := http.DefaultClient.Do(req)
		if e != nil {
			err = e
		} else {
			bb, _ := io.ReadAll(io.LimitReader(rr.Body, 1<<20))
			rr.Body.Close()
			if rr.StatusCode/100 != 2 {
				err = fmt.Errorf("Meta: %s", string(bb))
			}
		}
	case "tiktok":
		op := "ENABLE"
		if desired == "PAUSED" {
			op = "DISABLE"
		}
		b, _ := json.Marshal(map[string]any{"advertiser_id": account, "campaign_ids": []string{ext}, "operation_status": op})
		req, _ := http.NewRequestWithContext(ctx, "POST", "https://business-api.tiktok.com/open_api/v1.3/campaign/status/update/", bytes.NewReader(b))
		req.Header.Set("Access-Token", cred.AccessToken)
		req.Header.Set("Content-Type", "application/json")
		rr, e := http.DefaultClient.Do(req)
		if e != nil {
			err = e
		} else {
			bb, _ := io.ReadAll(io.LimitReader(rr.Body, 1<<20))
			rr.Body.Close()
			var x struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			}
			_ = json.Unmarshal(bb, &x)
			if rr.StatusCode/100 != 2 || x.Code != 0 {
				err = fmt.Errorf("TikTok: %s", firstNonEmpty(x.Message, string(bb)))
			}
		}
	case "google":
		if strings.EqualFold(ch, "VIDEO") {
			err = errors.New("Google Ads API permite reportar campañas VIDEO heredadas, pero no mutarlas desde esta integración")
		} else {
			if e := a.refreshGoogleAdsTokenV25(ctx, &cred); e != nil {
				err = e
			} else {
				st := "ENABLED"
				if desired == "PAUSED" {
					st = "PAUSED"
				}
				body, _ := json.Marshal(map[string]any{"operations": []any{map[string]any{"update": map[string]any{"resourceName": "customers/" + strings.ReplaceAll(account, "-", "") + "/campaigns/" + ext, "status": st}, "updateMask": "status"}}})
				ep := "https://googleads.googleapis.com/v25/customers/" + strings.ReplaceAll(account, "-", "") + "/campaigns:mutate"
				req, _ := http.NewRequestWithContext(ctx, "POST", ep, bytes.NewReader(body))
				req.Header.Set("Authorization", "Bearer "+cred.AccessToken)
				req.Header.Set("developer-token", a.cfg.GoogleAdsDeveloperToken)
				if cred.LoginCustomerID != "" {
					req.Header.Set("login-customer-id", strings.ReplaceAll(cred.LoginCustomerID, "-", ""))
				}
				req.Header.Set("Content-Type", "application/json")
				rr, e := http.DefaultClient.Do(req)
				if e != nil {
					err = e
				} else {
					bb, _ := io.ReadAll(io.LimitReader(rr.Body, 2<<20))
					rr.Body.Close()
					if rr.StatusCode/100 != 2 {
						err = fmt.Errorf("Google Ads: %s", string(bb))
					}
				}
			}
		}
	default:
		err = errors.New("proveedor no soportado")
	}
	if err != nil {
		writeError(w, err, 502)
		return
	}
	_, _ = a.db.Exec(`UPDATE ads_campaigns_v25 SET status=?,updated_at=? WHERE tenant_id=? AND id=?`, desired, time.Now().UTC().Format(time.RFC3339), tid, q.ID)
	writeJSON(w, map[string]any{"ok": true, "status": desired})
}

func (a *App) adsV25EntitiesHandler(w http.ResponseWriter, r *http.Request) {
	tid, _, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", 405)
		return
	}
	provider := strings.TrimSpace(r.URL.Query().Get("provider"))
	campaign := strings.TrimSpace(r.URL.Query().Get("campaign_id"))
	q := `SELECT id,provider,account_id,entity_type,external_id,external_campaign_id,external_group_id,name,status,budget,currency FROM ads_entities_v25 WHERE tenant_id=?`
	args := []any{tid}
	if provider != "" && provider != "all" {
		q += ` AND provider=?`
		args = append(args, provider)
	}
	if campaign != "" {
		q += ` AND external_campaign_id=?`
		args = append(args, campaign)
	}
	q += ` ORDER BY provider,external_campaign_id,entity_type,name LIMIT 5000`
	rows, err := a.db.Query(q, args...)
	if err != nil {
		writeError(w, err, 500)
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id int64
		var p, account, typ, eid, cid, gid, name, status, curr string
		var budget float64
		_ = rows.Scan(&id, &p, &account, &typ, &eid, &cid, &gid, &name, &status, &budget, &curr)
		items = append(items, map[string]any{"id": id, "provider": p, "account_id": account, "entity_type": typ, "external_id": eid, "campaign_id": cid, "group_id": gid, "name": name, "status": status, "budget": budget, "currency": curr})
	}
	writeJSON(w, map[string]any{"items": items})
}

func (a *App) adsV25InternalCampaignsHandler(w http.ResponseWriter, r *http.Request) {
	tid, _, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	rows, err := a.db.Query(`SELECT id,name,status FROM marketing_campaigns WHERE tenant_id=? ORDER BY updated_at DESC,name`, tid)
	if err != nil {
		writeError(w, err, 500)
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id int64
		var name, status string
		_ = rows.Scan(&id, &name, &status)
		items = append(items, map[string]any{"id": id, "name": name, "status": status})
	}
	writeJSON(w, map[string]any{"items": items})
}

func (a *App) runAdsV25Sync() {
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()
	time.Sleep(45 * time.Second)
	a.syncAllAdsV25Background()
	for range ticker.C {
		a.syncAllAdsV25Background()
	}
}
func (a *App) syncAllAdsV25Background() {
	rows, err := a.db.Query(`SELECT DISTINCT tenant_id FROM ads_connections_v25 WHERE status='connected'`)
	if err != nil {
		return
	}
	defer rows.Close()
	var tids []int64
	for rows.Next() {
		var x int64
		_ = rows.Scan(&x)
		tids = append(tids, x)
	}
	for _, tid := range tids {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		_, _ = a.syncAdsV25Tenant(ctx, tid, 0, 30)
		cancel()
		time.Sleep(2 * time.Second)
	}
}
