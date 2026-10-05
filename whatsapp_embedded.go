package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type whatsappEmbeddedCompleteRequest struct {
	ConnectionID  int64  `json:"connection_id"`
	Code          string `json:"code"`
	WABAID        string `json:"waba_id"`
	PhoneNumberID string `json:"phone_number_id"`
	PIN           string `json:"pin"`
	AgentID       int64  `json:"agent_id"`
}

func (a *App) whatsappEmbeddedConfigHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, errors.New("método no permitido"), 405)
		return
	}
	_, u, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	if u.Role != "owner" && u.Role != "admin" && u.Role != "superadmin" {
		writeError(w, errors.New("sin permiso para conectar WhatsApp Business"), 403)
		return
	}
	if strings.TrimSpace(a.cfg.MetaAppID) == "" || strings.TrimSpace(a.cfg.MetaWhatsAppConfigID) == "" {
		writeError(w, errors.New("Embedded Signup no está configurado. Define META_APP_ID y META_WHATSAPP_CONFIG_ID"), 409)
		return
	}
	writeJSON(w, map[string]any{
		"ok":              true,
		"app_id":          a.cfg.MetaAppID,
		"config_id":       a.cfg.MetaWhatsAppConfigID,
		"graph_version":   a.cfg.MetaGraphVersion,
		"embedded_signup": true,
		"manual_fallback": true,
		"webhook_url":     a.cfg.BaseURL + "/webhooks/meta/whatsapp",
	})
}

func (a *App) exchangeWhatsAppEmbeddedCode(ctx context.Context, code string) (string, error) {
	if strings.TrimSpace(code) == "" {
		return "", errors.New("Meta no devolvió el código de autorización")
	}
	if a.cfg.MetaAppID == "" || a.cfg.MetaAppSecret == "" {
		return "", errors.New("META_APP_ID y META_APP_SECRET son obligatorios")
	}
	q := url.Values{
		"client_id":     {a.cfg.MetaAppID},
		"client_secret": {a.cfg.MetaAppSecret},
		"code":          {strings.TrimSpace(code)},
	}
	ep := "https://graph.facebook.com/" + a.cfg.MetaGraphVersion + "/oauth/access_token?" + q.Encode()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ep, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode/100 != 2 {
		var ge struct {
			Error struct {
				Message string `json:"message"`
				Code    int    `json:"code"`
			} `json:"error"`
		}
		_ = json.Unmarshal(b, &ge)
		if ge.Error.Message != "" {
			return "", fmt.Errorf("Meta OAuth: %s (code %d)", ge.Error.Message, ge.Error.Code)
		}
		return "", fmt.Errorf("Meta OAuth HTTP %d", resp.StatusCode)
	}
	var out struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(b, &out); err != nil || strings.TrimSpace(out.AccessToken) == "" {
		return "", errors.New("Meta no devolvió un Access Token válido")
	}
	return out.AccessToken, nil
}

func (a *App) appAccessToken() string {
	if a.cfg.MetaAppID == "" || a.cfg.MetaAppSecret == "" {
		return ""
	}
	return a.cfg.MetaAppID + "|" + a.cfg.MetaAppSecret
}

func (a *App) discoverWABAFromEmbeddedToken(ctx context.Context, userToken string) (string, error) {
	debugToken := a.appAccessToken()
	if strings.TrimSpace(a.cfg.MetaSystemUserAccessToken) != "" {
		debugToken = strings.TrimSpace(a.cfg.MetaSystemUserAccessToken)
	}
	if debugToken == "" {
		return "", errors.New("no hay token de aplicación/sistema para consultar el WABA compartido")
	}
	ep := "https://graph.facebook.com/" + a.cfg.MetaGraphVersion + "/debug_token?input_token=" + url.QueryEscape(userToken)
	var out struct {
		Data struct {
			IsValid        bool `json:"is_valid"`
			GranularScopes []struct {
				Scope     string   `json:"scope"`
				TargetIDs []string `json:"target_ids"`
			} `json:"granular_scopes"`
		} `json:"data"`
	}
	if err := a.whatsappCloudGraph(ctx, http.MethodGet, ep, debugToken, nil, &out); err != nil {
		return "", err
	}
	if !out.Data.IsValid {
		return "", errors.New("el token de Embedded Signup no es válido")
	}
	for _, gs := range out.Data.GranularScopes {
		if gs.Scope == "whatsapp_business_management" && len(gs.TargetIDs) > 0 {
			return gs.TargetIDs[0], nil
		}
	}
	return "", errors.New("Meta no devolvió un WABA compartido para whatsapp_business_management")
}

func (a *App) discoverPhoneFromWABA(ctx context.Context, wabaID, token string) (string, error) {
	var out struct {
		Data []struct {
			ID                 string `json:"id"`
			DisplayPhoneNumber string `json:"display_phone_number"`
		} `json:"data"`
	}
	ep := "https://graph.facebook.com/" + a.cfg.MetaGraphVersion + "/" + url.PathEscape(wabaID) + "/phone_numbers?fields=id,display_phone_number"
	if err := a.whatsappCloudGraph(ctx, http.MethodGet, ep, token, nil, &out); err != nil {
		return "", err
	}
	if len(out.Data) == 0 || out.Data[0].ID == "" {
		return "", errors.New("el WABA no contiene números disponibles")
	}
	return out.Data[0].ID, nil
}

func (a *App) registerEmbeddedPhone(ctx context.Context, phoneNumberID, token, pin string) error {
	pin = strings.TrimSpace(pin)
	if pin == "" {
		return nil
	}
	if ok, _ := regexp.MatchString(`^\d{6}$`, pin); !ok {
		return errors.New("el PIN debe tener exactamente 6 dígitos")
	}
	ep := "https://graph.facebook.com/" + a.cfg.MetaGraphVersion + "/" + url.PathEscape(phoneNumberID) + "/register"
	body := map[string]any{"messaging_product": "whatsapp", "pin": pin}
	return a.whatsappCloudGraph(ctx, http.MethodPost, ep, token, body, nil)
}

func (a *App) whatsappEmbeddedCompleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, errors.New("método no permitido"), 405)
		return
	}
	tid, u, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	if u.Role != "owner" && u.Role != "admin" && u.Role != "superadmin" {
		writeError(w, errors.New("sin permiso para conectar WhatsApp Business"), 403)
		return
	}
	var q whatsappEmbeddedCompleteRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&q); err != nil {
		writeError(w, errors.New("solicitud inválida"), 400)
		return
	}
	if q.ConnectionID <= 0 {
		writeError(w, errors.New("connection_id obligatorio"), 400)
		return
	}
	var c ChannelConnection
	err = a.db.QueryRow(`SELECT id,tenant_id,public_id,type,name,status,external_account_id,assigned_agent_id,config_json,encrypted_credentials,last_connected_at,last_disconnected_at,last_message_at,last_error,created_at,updated_at FROM channel_connections WHERE id=? AND tenant_id=?`, q.ConnectionID, tid).Scan(&c.ID, &c.TenantID, &c.PublicID, &c.Type, &c.Name, &c.Status, &c.ExternalAccountID, &c.AssignedAgentID, &c.ConfigJSON, &c.EncryptedCredentials, &c.LastConnectedAt, &c.LastDisconnectedAt, &c.LastMessageAt, &c.LastError, &c.CreatedAt, &c.UpdatedAt)
	if err != nil || c.Type != "whatsapp_cloud" {
		writeError(w, errors.New("conexión WhatsApp Cloud no encontrada"), 404)
		return
	}

	token, err := a.exchangeWhatsAppEmbeddedCode(r.Context(), q.Code)
	if err != nil {
		writeError(w, err, 502)
		return
	}
	wabaID := strings.TrimSpace(q.WABAID)
	if wabaID == "" {
		wabaID, err = a.discoverWABAFromEmbeddedToken(r.Context(), token)
		if err != nil {
			writeError(w, err, 502)
			return
		}
	}
	phoneID := strings.TrimSpace(q.PhoneNumberID)
	if phoneID == "" {
		phoneID, err = a.discoverPhoneFromWABA(r.Context(), wabaID, token)
		if err != nil {
			writeError(w, err, 502)
			return
		}
	}
	if err := a.registerEmbeddedPhone(r.Context(), phoneID, token, q.PIN); err != nil {
		writeError(w, fmt.Errorf("Meta autorizó el negocio, pero no se pudo registrar el número: %w", err), 502)
		return
	}
	result, err := a.configureWhatsAppCloud(r.Context(), c, wabaID, phoneID, token, firstPositive(q.AgentID, c.AssignedAgentID))
	if err != nil {
		writeError(w, err, 502)
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	cfg := a.whatsappCloudConfigFor(c)
	_ = cfg
	_, _ = a.db.Exec(`INSERT INTO channel_audit(tenant_id,connection_id,user_id,action,detail,created_at) VALUES(?,?,?,?,?,?)`, tid, c.ID, u.ID, "embedded_signup", "whatsapp_cloud:"+wabaID+":"+phoneID, now)
	result["onboarding"] = "embedded_signup"
	result["registered_phone"] = strings.TrimSpace(q.PIN) != ""
	result["manual_ids_required"] = false
	writeJSON(w, result)
}

func firstPositive(v ...int64) int64 {
	for _, x := range v {
		if x > 0 {
			return x
		}
	}
	return 0
}
