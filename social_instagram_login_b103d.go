package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// B10.3D: direct Instagram Business Login, independent of Facebook/Messenger OAuth.
// Disabled by default; existing B9 and Messenger connections remain untouched.
func instagramB103DEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("WORKTIC_B103D_IG_LOGIN_ENABLED")), "enabled")
}

func instagramB103DCredentials() (string, string, error) {
	id := strings.TrimSpace(os.Getenv("WORKTIC_IG_LOGIN_APP_ID"))
	secret := strings.TrimSpace(os.Getenv("WORKTIC_IG_LOGIN_APP_SECRET"))
	if !isMetaSocialConfigID(id) || len(secret) < 8 {
		return "", "", errors.New("Instagram Login no configurado: se requiere ID y secreto exclusivos de Instagram")
	}
	return id, secret, nil
}

func instagramB103DAuthorizationURL(state, redirect string) (string, error) {
	id, _, err := instagramB103DCredentials()
	if err != nil {
		return "", err
	}
	if len(state) < 32 || !strings.HasPrefix(redirect, "https://") {
		return "", errors.New("OAuth Instagram invalido")
	}
	vals := url.Values{
		"client_id": {id}, "redirect_uri": {redirect}, "response_type": {"code"},
		"scope": {"instagram_business_basic"}, "state": {state},
	}
	return "https://www.instagram.com/oauth/authorize?" + vals.Encode(), nil
}

// Never return Meta error bodies, authorization codes, tokens or request URLs to a user.
func instagramB103DJSONRequest(ctx context.Context, method, endpoint string, values url.Values, bearer string, out any) error {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	var body io.Reader
	if method == http.MethodPost {
		body = strings.NewReader(values.Encode())
	}
	if method == http.MethodGet && len(values) > 0 {
		endpoint += "?" + values.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return errors.New("solicitud Instagram invalida")
	}
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.New("Instagram no respondio a la solicitud")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Instagram rechazo la etapa OAuth (HTTP %d)", resp.StatusCode)
	}
	if err = json.NewDecoder(io.LimitReader(resp.Body, 65536)).Decode(out); err != nil {
		return errors.New("Instagram devolvio una respuesta invalida")
	}
	return nil
}

type instagramB103DShortToken struct {
	AccessToken string          `json:"access_token"`
	UserID      json.RawMessage `json:"user_id"`
	Permissions json.RawMessage `json:"permissions"`
}

func instagramB103DID(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil && isMetaAccountID(s) {
		return s
	}
	var num json.Number
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	if dec.Decode(&num) == nil && isMetaAccountID(num.String()) {
		return num.String()
	}
	return ""
}

func instagramB103DParseShort(raw []byte) (instagramB103DShortToken, string, error) {
	var outer struct {
		Data []json.RawMessage `json:"data"`
	}
	if json.Unmarshal(raw, &outer) != nil {
		return instagramB103DShortToken{}, "", errors.New("OAuth Instagram: respuesta no JSON")
	}
	item := raw
	if outer.Data != nil {
		if len(outer.Data) != 1 {
			return instagramB103DShortToken{}, "", errors.New("OAuth Instagram: respuesta ambigua")
		}
		item = outer.Data[0]
	}
	var tok instagramB103DShortToken
	if json.Unmarshal(item, &tok) != nil || tok.AccessToken == "" || len(tok.AccessToken) > 8192 {
		return tok, "", errors.New("OAuth Instagram: token ausente")
	}
	id := instagramB103DID(tok.UserID)
	if id == "" {
		return tok, "", errors.New("OAuth Instagram: usuario ausente")
	}
	if len(tok.Permissions) > 0 {
		var perms string
		if json.Unmarshal(tok.Permissions, &perms) == nil {
			found := false
			for _, scope := range strings.Split(perms, ",") {
				if strings.TrimSpace(scope) == "instagram_business_basic" {
					found = true
				}
			}
			if !found {
				return tok, "", errors.New("Instagram no concedio instagram_business_basic")
			}
		} else {
			var perms []string
			if json.Unmarshal(tok.Permissions, &perms) == nil {
				found := false
				for _, scope := range perms {
					if scope == "instagram_business_basic" {
						found = true
					}
				}
				if !found {
					return tok, "", errors.New("Instagram no concedio instagram_business_basic")
				}
			} else {
				return tok, "", errors.New("Formato de permisos Instagram invalido")
			}
		}
	}
	return tok, id, nil
}

// B10.3E keeps two Instagram identifiers separate:
// - id is the app-scoped identity used by the OAuth code exchange.
// - user_id is the professional Instagram account ID used for webhook routing.
// The callback only accepts an ID match established by the authenticated /me response.
func instagramB103DResolveProfile(shortID string, raw []byte) (string, string, error) {
	var profile struct {
		ID          json.RawMessage `json:"id"`
		UserID      json.RawMessage `json:"user_id"`
		Username    string          `json:"username"`
		AccountType string          `json:"account_type"`
	}
	if err := json.Unmarshal(raw, &profile); err != nil {
		return "", "", errors.New("IG_PROFILE_FORMAT")
	}
	scoped := instagramB103DID(profile.ID)
	professional := instagramB103DID(profile.UserID)
	if shortID == "" || (scoped != shortID && professional != shortID) {
		return "", "", errors.New("IG_PROFILE_ID_MISMATCH")
	}
	if professional == "" {
		// Do not guess the webhook account ID from the app-scoped OAuth ID.
		return "", "", errors.New("IG_PROFILE_PRO_ID_MISSING")
	}
	username := strings.TrimSpace(profile.Username)
	if username == "" {
		return "", "", errors.New("IG_PROFILE_USERNAME_MISSING")
	}
	kind := strings.ToUpper(strings.TrimSpace(profile.AccountType))
	if kind != "BUSINESS" && kind != "MEDIA_CREATOR" && kind != "CREATOR" {
		return "", "", errors.New("IG_PROFILE_NOT_PROFESSIONAL")
	}
	return username, professional, nil
}

func instagramB103DProfile(ctx context.Context, token string, expectedID string) (string, string, error) {
	var raw json.RawMessage
	err := instagramB103DJSONRequest(ctx, http.MethodGet,
		"https://graph.instagram.com/me",
		url.Values{"fields": {"id,user_id,username,account_type"}}, token, &raw)
	if err != nil {
		return "", "", fmt.Errorf("IG_PROFILE_FETCH: %w", err)
	}
	return instagramB103DResolveProfile(expectedID, raw)
}

func (a *App) completeInstagramB103DOAuth(w http.ResponseWriter, r *http.Request, tid int64, code, redirect string) error {
	id, secret, err := instagramB103DCredentials()
	if err != nil {
		return err
	}
	if strings.TrimSpace(a.cfg.ChannelEncryptionKey) == "" {
		return errors.New("IG_ENCRYPTION_NOT_CONFIGURED")
	}
	if len(code) > 4096 {
		return errors.New("IG_CODE_INVALID")
	}

	vals := url.Values{"client_id": {id}, "client_secret": {secret}, "grant_type": {"authorization_code"}, "redirect_uri": {redirect}, "code": {code}}
	// The code exchange may return flat JSON or a single-element data array.
	var response json.RawMessage
	if err = instagramB103DJSONRequest(r.Context(), http.MethodPost, "https://api.instagram.com/oauth/access_token", vals, "", &response); err != nil {
		return fmt.Errorf("IG_SHORT_EXCHANGE: %w", err)
	}
	short, userID, err := instagramB103DParseShort(response)
	if err != nil {
		return fmt.Errorf("IG_SHORT_FORMAT: %w", err)
	}
	var long struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
		TokenType   string `json:"token_type"`
	}
	params := url.Values{"grant_type": {"ig_exchange_token"}, "client_secret": {secret}, "access_token": {short.AccessToken}}
	if err = instagramB103DJSONRequest(r.Context(), http.MethodGet, "https://graph.instagram.com/access_token", params, "", &long); err != nil {
		return fmt.Errorf("IG_LONG_EXCHANGE: %w", err)
	}
	if long.AccessToken == "" || long.ExpiresIn < 86400 || len(long.AccessToken) > 8192 {
		return errors.New("IG_LONG_INVALID")
	}
	username, professionalID, err := instagramB103DProfile(r.Context(), long.AccessToken, userID)
	if err != nil {
		return err
	}

	// Fail closed: neither reassign other tenants nor overwrite existing IG credentials.
	var existing int
	if err = a.db.QueryRow(`SELECT COUNT(*) FROM social_connections WHERE platform='instagram' AND external_account_id IN (?,?)`, userID, professionalID).Scan(&existing); err != nil {
		return errors.New("IG_DUPLICATE_CHECK")
	}
	if existing != 0 {
		return errors.New("IG_ALREADY_CONNECTED")
	}
	tok := socialToken{AccessToken: long.AccessToken, TokenType: "INSTAGRAM_USER", ExpiresIn: long.ExpiresIn}
	cfg := socialProviderConfig{InstagramID: professionalID, InstagramAuthMethod: "instagram_login"}
	if err = a.saveOfficialConnection(tid, "instagram", username, professionalID, tok, cfg, "instagram_business_basic"); err != nil {
		return errors.New("IG_SAVE_FAILED")
	}
	http.Redirect(w, r, a.cfg.BaseURL+"/app.html?social_oauth=success&platform=instagram", http.StatusSeeOther)
	return nil
}
