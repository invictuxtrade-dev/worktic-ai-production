package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type messengerCredentials struct {
	PageToken   string `json:"page_token"`
	AppID       string `json:"app_id"`
	AppSecret   string `json:"app_secret"`
	VerifyToken string `json:"verify_token"`
}

type messengerTokenValidation struct {
	Valid               bool
	Method              string
	PageID              string
	AppID               string
	TokenType           string
	Scopes              []string
	ExpiresAt           int64
	DataAccessExpiresAt int64
	Warning             string
}

type metaGraphAPIError struct {
	Status       int
	Code         int
	ErrorSubcode int
	Type         string
	Message      string
	TraceID      string
}

func (e *metaGraphAPIError) Error() string {
	if e == nil {
		return "error desconocido de Meta"
	}
	parts := make([]string, 0, 3)
	if e.Code != 0 {
		parts = append(parts, fmt.Sprintf("código %d", e.Code))
	}
	if strings.TrimSpace(e.Message) != "" {
		parts = append(parts, strings.TrimSpace(e.Message))
	}
	if len(parts) == 0 {
		return fmt.Sprintf("Meta Graph API respondió HTTP %d", e.Status)
	}
	return "Meta: " + strings.Join(parts, " · ")
}

func (a *App) messengerCredentialsFor(c ChannelConnection) (messengerCredentials, error) {
	raw := decryptLocal(c.EncryptedCredentials, a.cfg.ChannelEncryptionKey)
	if raw == "" {
		return messengerCredentials{}, errors.New("credenciales de Messenger no disponibles")
	}
	var cr messengerCredentials
	if json.Unmarshal([]byte(raw), &cr) != nil || cr.PageToken == "" {
		cr.PageToken = raw
	}
	return cr, nil
}

func (a *App) messengerGraphVersion() string {
	version := strings.TrimSpace(a.cfg.MetaGraphVersion)
	if version == "" {
		return "v22.0"
	}
	if !strings.HasPrefix(strings.ToLower(version), "v") {
		version = "v" + version
	}
	return version
}

func (a *App) messengerGraph(method, path, token string, values url.Values, out any) error {
	endpoint := "https://graph.facebook.com/" + a.messengerGraphVersion() + path
	if values == nil {
		values = url.Values{}
	}
	values.Set("access_token", token)
	var req *http.Request
	var err error
	if method == http.MethodGet {
		req, err = http.NewRequest(method, endpoint+"?"+values.Encode(), nil)
	} else {
		req, err = http.NewRequest(method, endpoint, strings.NewReader(values.Encode()))
		if req != nil {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
	}
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 18 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var payload struct {
			Error struct {
				Message      string `json:"message"`
				Type         string `json:"type"`
				Code         int    `json:"code"`
				ErrorSubcode int    `json:"error_subcode"`
				TraceID      string `json:"fbtrace_id"`
			} `json:"error"`
		}
		_ = json.Unmarshal(b, &payload)
		return &metaGraphAPIError{
			Status:       resp.StatusCode,
			Code:         payload.Error.Code,
			ErrorSubcode: payload.Error.ErrorSubcode,
			Type:         payload.Error.Type,
			Message:      firstNonEmpty(payload.Error.Message, strings.TrimSpace(string(b))),
			TraceID:      payload.Error.TraceID,
		}
	}
	if out != nil {
		if len(strings.TrimSpace(string(b))) == 0 {
			return nil
		}
		return json.Unmarshal(b, out)
	}
	return nil
}

func messengerHasScope(scopes []string, wanted string) bool {
	for _, scope := range scopes {
		if strings.EqualFold(strings.TrimSpace(scope), wanted) {
			return true
		}
	}
	return false
}

func messengerUniqueScopes(scopes []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if scope == "" || seen[scope] {
			continue
		}
		seen[scope] = true
		out = append(out, scope)
	}
	return out
}

func (a *App) validateMessengerPageToken(pageToken, pageID, appID, appSecret string) (messengerTokenValidation, error) {
	pageToken = strings.TrimSpace(pageToken)
	pageID = strings.TrimSpace(pageID)
	appID = strings.TrimSpace(firstNonEmpty(appID, a.cfg.MetaAppID))
	appSecret = strings.TrimSpace(firstNonEmpty(appSecret, a.cfg.MetaAppSecret))
	if pageToken == "" {
		return messengerTokenValidation{}, errors.New("Page Access Token obligatorio")
	}
	if pageID == "" {
		return messengerTokenValidation{}, errors.New("Page ID obligatorio")
	}

	var debugWarning string
	if appID != "" && appSecret != "" {
		var debug struct {
			Data struct {
				AppID               string   `json:"app_id"`
				Type                string   `json:"type"`
				Application         string   `json:"application"`
				DataAccessExpiresAt int64    `json:"data_access_expires_at"`
				ExpiresAt           int64    `json:"expires_at"`
				IsValid             bool     `json:"is_valid"`
				ProfileID           string   `json:"profile_id"`
				UserID              string   `json:"user_id"`
				Scopes              []string `json:"scopes"`
				GranularScopes      []struct {
					Scope     string   `json:"scope"`
					TargetIDs []string `json:"target_ids"`
				} `json:"granular_scopes"`
			} `json:"data"`
		}
		appAccessToken := appID + "|" + appSecret
		if err := a.messengerGraph(http.MethodGet, "/debug_token", appAccessToken, url.Values{"input_token": {pageToken}}, &debug); err == nil {
			scopes := append([]string{}, debug.Data.Scopes...)
			messagingTargets := make([]string, 0)
			for _, granular := range debug.Data.GranularScopes {
				scopes = append(scopes, granular.Scope)
				if strings.EqualFold(granular.Scope, "pages_messaging") {
					messagingTargets = append(messagingTargets, granular.TargetIDs...)
				}
			}
			scopes = messengerUniqueScopes(scopes)
			if !debug.Data.IsValid {
				return messengerTokenValidation{}, errors.New("Meta indicó que el Page Access Token no es válido")
			}
			if debug.Data.AppID != "" && appID != "" && debug.Data.AppID != appID {
				return messengerTokenValidation{}, fmt.Errorf("el token pertenece a la app %s y no a la app %s", debug.Data.AppID, appID)
			}
			if !messengerHasScope(scopes, "pages_messaging") {
				return messengerTokenValidation{}, errors.New("el token no incluye el permiso pages_messaging")
			}
			detectedPageID := strings.TrimSpace(debug.Data.ProfileID)
			if detectedPageID == "" && len(messagingTargets) == 1 {
				detectedPageID = strings.TrimSpace(messagingTargets[0])
			}
			if detectedPageID != "" && detectedPageID != pageID {
				return messengerTokenValidation{}, fmt.Errorf("el token pertenece a la página %s y no a la página %s", detectedPageID, pageID)
			}
			if len(messagingTargets) > 0 {
				allowed := false
				for _, targetID := range messagingTargets {
					if strings.TrimSpace(targetID) == pageID {
						allowed = true
						break
					}
				}
				if !allowed {
					return messengerTokenValidation{}, fmt.Errorf("pages_messaging no está concedido para la página %s", pageID)
				}
			}
			return messengerTokenValidation{
				Valid:               true,
				Method:              "debug_token",
				PageID:              firstNonEmpty(detectedPageID, pageID),
				AppID:               debug.Data.AppID,
				TokenType:           debug.Data.Type,
				Scopes:              scopes,
				ExpiresAt:           debug.Data.ExpiresAt,
				DataAccessExpiresAt: debug.Data.DataAccessExpiresAt,
			}, nil
		} else {
			debugWarning = "No fue posible consultar debug_token; se validó directamente con Messenger Platform."
		}
	}

	// Sin App Secret no usamos messenger_profile, conversations ni metadatos
	// públicos como prueba de validez: esos endpoints cambian entre versiones y
	// pueden exigir permisos que no son necesarios para recibir/responder mensajes.
	// Primero intentamos resolver únicamente el ID del objeto representado por el
	// token. Esta consulta no solicita nombre, contenido ni engagement.
	var identity struct {
		ID string `json:"id"`
	}
	identityErr := a.messengerGraph(http.MethodGet, "/me", pageToken, url.Values{"fields": {"id"}}, &identity)
	if identityErr == nil {
		detectedPageID := strings.TrimSpace(identity.ID)
		if detectedPageID != "" && detectedPageID != pageID {
			return messengerTokenValidation{}, fmt.Errorf("el token pertenece a la página %s y no a la página %s", detectedPageID, pageID)
		}
		warning := strings.TrimSpace(debugWarning)
		if appSecret == "" {
			warning = strings.TrimSpace(strings.Join([]string{warning, "Identidad de página validada. Para verificar permisos y aplicación de forma estricta, configura META_APP_SECRET o el App Secret de esta conexión."}, " "))
		}
		return messengerTokenValidation{
			Valid:     true,
			Method:    "page_identity",
			PageID:    firstNonEmpty(detectedPageID, pageID),
			AppID:     appID,
			TokenType: "PAGE",
			Scopes:    []string{"pages_messaging"},
			Warning:   warning,
		}, nil
	}

	// Los tokens inválidos suelen responder OAuth code 190/102 o HTTP 401. Esos
	// casos sí deben bloquearse. Errores de permisos, campos no disponibles o
	// cambios de versión no invalidan por sí solos un Page Access Token que el
	// usuario ya generó desde la sección oficial de Messenger de Meta.
	var graphErr *metaGraphAPIError
	if errors.As(identityErr, &graphErr) {
		if graphErr.Code == 190 || graphErr.Code == 102 || graphErr.Status == http.StatusUnauthorized {
			return messengerTokenValidation{}, fmt.Errorf("Page Access Token inválido o vencido: %v", identityErr)
		}
	}

	warning := "Meta no permitió una validación adicional sin App Secret. El token fue aceptado para configurar el webhook y será comprobado funcionalmente al recibir o enviar el primer mensaje."
	if strings.TrimSpace(debugWarning) != "" {
		warning = strings.TrimSpace(debugWarning + " " + warning)
	}
	return messengerTokenValidation{
		Valid:     true,
		Method:    "provisional_page_token",
		PageID:    pageID,
		AppID:     appID,
		TokenType: "PAGE",
		Scopes:    []string{"pages_messaging"},
		Warning:   warning,
	}, nil
}

func (a *App) ensureMessengerConnectionSetup(r *http.Request, c ChannelConnection) map[string]any {
	var cfg map[string]any
	if json.Unmarshal([]byte(c.ConfigJSON), &cfg) != nil || cfg == nil {
		cfg = map[string]any{}
	}
	changed := false
	hook, _ := cfg["webhook_url"].(string)
	if strings.TrimSpace(hook) == "" {
		if base, err := a.publicBaseURL(r); err == nil {
			hook = strings.TrimRight(base, "/") + "/webhooks/messenger/" + c.PublicID
			cfg["webhook_url"] = hook
			changed = true
		}
	}
	verify, _ := cfg["verify_token"].(string)
	var cr messengerCredentials
	if previous, err := a.messengerCredentialsFor(c); err == nil {
		cr = previous
		if strings.TrimSpace(verify) == "" {
			verify = strings.TrimSpace(previous.VerifyToken)
		}
	}
	if strings.TrimSpace(verify) == "" {
		verify = "wtm_" + randomToken(24)
		changed = true
	}
	if cfg["verify_token"] != verify {
		cfg["verify_token"] = verify
		changed = true
	}
	if cr.PageToken != "" && cr.VerifyToken != verify {
		cr.VerifyToken = verify
		raw, _ := json.Marshal(cr)
		enc := encryptLocal(string(raw), a.cfg.ChannelEncryptionKey)
		_, _ = a.db.Exec(`UPDATE channel_connections SET encrypted_credentials=?,updated_at=? WHERE id=? AND tenant_id=?`, enc, time.Now().UTC().Format(time.RFC3339), c.ID, c.TenantID)
	}
	if changed {
		raw, _ := json.Marshal(cfg)
		_, _ = a.db.Exec(`UPDATE channel_connections SET config_json=?,updated_at=? WHERE id=? AND tenant_id=?`, string(raw), time.Now().UTC().Format(time.RFC3339), c.ID, c.TenantID)
	}
	return cfg
}

func (a *App) configureMessenger(r *http.Request, c ChannelConnection, pageToken, pageID, appID, appSecret string) (map[string]any, error) {
	pageToken = strings.TrimSpace(pageToken)
	pageID = strings.TrimSpace(pageID)
	appID = strings.TrimSpace(appID)
	appSecret = strings.TrimSpace(appSecret)

	var previous messengerCredentials
	if p, err := a.messengerCredentialsFor(c); err == nil {
		previous = p
	}
	if pageToken == "" {
		pageToken = previous.PageToken
	}
	if appID == "" {
		appID = firstNonEmpty(previous.AppID, a.cfg.MetaAppID)
	}
	if appSecret == "" {
		appSecret = firstNonEmpty(previous.AppSecret, a.cfg.MetaAppSecret)
	}
	if pageToken == "" {
		return nil, errors.New("Page Access Token obligatorio")
	}
	if pageID == "" {
		pageID = c.ExternalAccountID
	}
	if pageID == "" {
		return nil, errors.New("Page ID obligatorio")
	}

	cfg := a.ensureMessengerConnectionSetup(r, c)
	verify, _ := cfg["verify_token"].(string)
	hook, _ := cfg["webhook_url"].(string)
	if verify == "" || hook == "" {
		return nil, errors.New("no fue posible preparar el webhook de Messenger")
	}

	validation, err := a.validateMessengerPageToken(pageToken, pageID, appID, appSecret)
	if err != nil {
		return nil, err
	}

	pageName, _ := cfg["page_name"].(string)
	metadataWarning := ""
	if messengerHasScope(validation.Scopes, "pages_read_engagement") {
		var page struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		if metaErr := a.messengerGraph(http.MethodGet, "/"+url.PathEscape(pageID), pageToken, url.Values{"fields": {"id,name"}}, &page); metaErr == nil {
			pageName = page.Name
		} else {
			metadataWarning = "El token es válido para Messenger, pero Meta no permitió consultar el nombre de la página."
		}
	} else {
		metadataWarning = "El token no incluye pages_read_engagement. No se consultan metadatos de la página y esto no bloquea Messenger."
	}
	if strings.TrimSpace(pageName) == "" {
		pageName = "Página " + pageID
	}

	cr := messengerCredentials{PageToken: pageToken, AppID: appID, AppSecret: appSecret, VerifyToken: verify}
	rawCredentials, _ := json.Marshal(cr)
	enc := encryptLocal(string(rawCredentials), a.cfg.ChannelEncryptionKey)

	cfg["page_id"] = pageID
	cfg["page_name"] = pageName
	cfg["webhook_url"] = hook
	cfg["verify_token"] = verify
	cfg["subscribed_fields"] = []string{"messages", "messaging_postbacks", "message_reads", "message_deliveries"}
	cfg["token_valid"] = true
	cfg["token_validation_method"] = validation.Method
	cfg["token_scopes"] = validation.Scopes
	cfg["token_app_id"] = firstNonEmpty(validation.AppID, appID)
	cfg["token_type"] = validation.TokenType
	cfg["token_expires_at"] = validation.ExpiresAt
	cfg["data_access_expires_at"] = validation.DataAccessExpiresAt
	cfg["metadata_warning"] = metadataWarning
	cfg["validation_warning"] = validation.Warning
	cfgRaw, _ := json.Marshal(cfg)

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = a.db.Exec(`UPDATE channel_connections SET encrypted_credentials=?,external_account_id=?,config_json=?,status='connected',last_connected_at=?,last_error='',updated_at=? WHERE id=? AND tenant_id=?`, enc, pageID, string(cfgRaw), now, now, c.ID, c.TenantID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"ok":                      true,
		"page_id":                 pageID,
		"page_name":               pageName,
		"webhook_url":             hook,
		"verify_token":            verify,
		"token_valid":             true,
		"pages_messaging":         true,
		"page_id_match":           true,
		"token_validation_method": validation.Method,
		"token_scopes":            validation.Scopes,
		"metadata_warning":        metadataWarning,
		"validation_warning":      validation.Warning,
	}, nil
}

func (a *App) messengerConnectionDetails(r *http.Request, c ChannelConnection) map[string]any {
	cfg := a.ensureMessengerConnectionSetup(r, c)
	hook, _ := cfg["webhook_url"].(string)
	verify, _ := cfg["verify_token"].(string)
	pageName, _ := cfg["page_name"].(string)
	pageID, _ := cfg["page_id"].(string)
	appID, _ := cfg["token_app_id"].(string)
	metadataWarning, _ := cfg["metadata_warning"].(string)
	validationWarning, _ := cfg["validation_warning"].(string)
	if pageID == "" {
		pageID = c.ExternalAccountID
	}
	hasPageToken := false
	appSecretConfigured := strings.TrimSpace(a.cfg.MetaAppSecret) != ""
	if cr, err := a.messengerCredentialsFor(c); err == nil {
		hasPageToken = strings.TrimSpace(cr.PageToken) != ""
		appSecretConfigured = appSecretConfigured || strings.TrimSpace(cr.AppSecret) != ""
		if appID == "" {
			appID = cr.AppID
		}
	}

	// Durable webhook diagnostic. We prefer channel_events over config_json so
	// another configuration save cannot erase the latest receipt marker.
	lastHTTPAt, lastHTTPStatus, lastHTTPDetail := a.latestMessengerChannelEvent(c, "messenger_webhook_http")
	lastProcessedAt, lastProcessedStatus, lastProcessedDetail := a.latestMessengerChannelEvent(c, "messenger_webhook_processed")
	lastRealAt, lastRealStatus, lastRealDetail := a.latestMessengerChannelEvent(c, "messenger_message_received")
	lastRejectedAt, lastRejectedStatus, lastRejectedDetail := a.latestMessengerChannelEvent(c, "messenger_webhook_rejected")

	lastWebhookAt := lastHTTPAt
	if lastWebhookAt == "" {
		if v, ok := cfg["last_webhook_at"].(string); ok {
			lastWebhookAt = v
		}
	}
	lastMessageAt := strings.TrimSpace(c.LastMessageAt)
	if lastRealAt != "" {
		lastMessageAt = lastRealAt
	}

	lastShape := ""
	lastEventCount := any(0)
	lastWebhookError := ""
	if strings.TrimSpace(lastProcessedDetail) != "" {
		var d map[string]any
		if json.Unmarshal([]byte(lastProcessedDetail), &d) == nil {
			lastShape, _ = d["shape"].(string)
			if n, ok := d["event_count"]; ok {
				lastEventCount = n
			}
			lastWebhookError, _ = d["error"].(string)
		}
	}
	if lastShape == "" {
		lastShape, _ = cfg["last_webhook_shape"].(string)
	}
	if lastWebhookError == "" {
		lastWebhookError, _ = cfg["last_webhook_error"].(string)
	}

	return map[string]any{
		"ok":                       c.Status == "connected" && hook != "" && verify != "",
		"platform":                 "messenger",
		"page_id":                  pageID,
		"page_name":                pageName,
		"app_id":                   appID,
		"has_page_token":           hasPageToken,
		"app_secret_configured":    appSecretConfigured,
		"webhook_url":              hook,
		"verify_token":             verify,
		"assigned_agent_id":        c.AssignedAgentID,
		"status":                   c.Status,
		"last_message_at":          lastMessageAt,
		"last_webhook_at":          lastWebhookAt,
		"last_webhook_shape":       lastShape,
		"last_webhook_event_count": lastEventCount,
		"last_webhook_error":       lastWebhookError,
		"last_webhook_http_status": lastHTTPStatus,
		"last_webhook_http_detail": lastHTTPDetail,
		"last_processed_at":        lastProcessedAt,
		"last_processed_status":    lastProcessedStatus,
		"last_processed_detail":    lastProcessedDetail,
		"last_real_message_status": lastRealStatus,
		"last_real_message_detail": lastRealDetail,
		"last_rejected_at":         lastRejectedAt,
		"last_rejected_status":     lastRejectedStatus,
		"last_rejected_detail":     lastRejectedDetail,
		"last_error":               c.LastError,
		"metadata_warning":         metadataWarning,
		"validation_warning":       validationWarning,
		"configuration_ready":      hook != "" && verify != "",
	}
}

func (a *App) testMessengerConnection(r *http.Request, c ChannelConnection) (map[string]any, error) {
	result := a.messengerConnectionDetails(r, c)
	result["token_valid"] = false
	result["pages_messaging"] = false
	result["page_id_match"] = false
	result["token_check_available"] = true
	result["subscription_manual"] = true
	result["recommended_fields"] = []string{"messages", "messaging_postbacks", "message_reads", "message_deliveries"}

	cr, err := a.messengerCredentialsFor(c)
	if err != nil {
		result["token_error"] = err.Error()
		result["ok"] = false
		return result, nil
	}
	pageID, _ := result["page_id"].(string)
	validation, validationErr := a.validateMessengerPageToken(cr.PageToken, pageID, cr.AppID, cr.AppSecret)
	if validationErr != nil {
		result["token_error"] = validationErr.Error()
		result["ok"] = false
		return result, nil
	}
	result["token_valid"] = validation.Valid
	result["pages_messaging"] = messengerHasScope(validation.Scopes, "pages_messaging")
	result["page_id_match"] = validation.PageID == "" || validation.PageID == pageID
	result["token_validation_method"] = validation.Method
	result["token_scopes"] = validation.Scopes
	result["token_app_id"] = validation.AppID
	result["token_type"] = validation.TokenType
	result["token_expires_at"] = validation.ExpiresAt
	result["data_access_expires_at"] = validation.DataAccessExpiresAt
	if validation.Warning != "" {
		result["validation_warning"] = validation.Warning
	}
	ready, _ := result["configuration_ready"].(bool)
	result["ok"] = ready && validation.Valid && result["pages_messaging"].(bool) && result["page_id_match"].(bool)
	return result, nil
}

type messengerInboundEvent struct {
	SenderID    string
	RecipientID string
	MessageID   string
	Text        string
	MessageType string
	IsEcho      bool
	Timestamp   int64
	IsSample    bool
	SourcePath  string
}

type messengerWebhookWalkContext struct {
	SenderID    string
	RecipientID string
	Timestamp   int64
	Field       string
}

func messengerAnyString(v any) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case json.Number:
		return strings.TrimSpace(x.String())
	case float64:
		if x == float64(int64(x)) {
			return fmt.Sprintf("%d", int64(x))
		}
		return strings.TrimSpace(fmt.Sprintf("%v", x))
	case float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return strings.TrimSpace(fmt.Sprint(x))
	default:
		return ""
	}
}

func messengerAnyInt64(v any) int64 {
	s := messengerAnyString(v)
	if s == "" {
		return 0
	}
	var n int64
	_, _ = fmt.Sscan(s, &n)
	return n
}

func messengerMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func messengerSlice(v any) []any {
	x, _ := v.([]any)
	return x
}

func messengerNestedString(m map[string]any, keys ...string) string {
	var current any = m
	for _, key := range keys {
		mm, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current = mm[key]
	}
	return messengerAnyString(current)
}

func messengerPartyID(v any) string {
	if id := messengerAnyString(v); id != "" {
		return id
	}
	m := messengerMap(v)
	if m == nil {
		return ""
	}
	for _, key := range []string{"id", "user_id", "page_id", "psid"} {
		if id := messengerAnyString(m[key]); id != "" {
			return id
		}
	}
	return ""
}

func messengerMessageText(m map[string]any) (string, string) {
	if m == nil {
		return "", ""
	}
	if text := messengerAnyString(m["text"]); text != "" {
		return text, "text"
	}
	if text := messengerNestedString(m, "text", "body"); text != "" {
		return text, "text"
	}
	if payload := messengerNestedString(m, "quick_reply", "payload"); payload != "" {
		return payload, "quick_reply"
	}
	if payload := messengerNestedString(m, "quickReply", "payload"); payload != "" {
		return payload, "quick_reply"
	}
	if title := messengerAnyString(m["title"]); title != "" {
		return title, "postback"
	}
	if payload := messengerAnyString(m["payload"]); payload != "" {
		return payload, "postback"
	}
	if attachments := messengerSlice(m["attachments"]); len(attachments) > 0 {
		first := messengerMap(attachments[0])
		attachmentType := messengerAnyString(first["type"])
		if attachmentType == "" {
			attachmentType = "archivo"
		}
		if title := messengerNestedString(first, "payload", "title"); title != "" {
			return title, "attachment"
		}
		return "[Adjunto: " + attachmentType + "]", "attachment"
	}
	if stickerID := messengerAnyString(m["sticker_id"]); stickerID != "" {
		return "[Sticker]", "sticker"
	}
	return "", ""
}

func messengerAppendInboundEvent(events *[]messengerInboundEvent, seen map[string]bool, ctx messengerWebhookWalkContext, message map[string]any, postback map[string]any, sourcePath string) {
	messageText, messageType := messengerMessageText(message)
	if messageText == "" {
		messageText, messageType = messengerMessageText(postback)
	}
	mid := ""
	isEcho := false
	if message != nil {
		mid = firstNonEmpty(messengerAnyString(message["mid"]), messengerAnyString(message["id"]), messengerAnyString(message["message_id"]))
		if v, ok := message["is_echo"].(bool); ok {
			isEcho = v
		}
		if v, ok := message["isEcho"].(bool); ok {
			isEcho = isEcho || v
		}
	}
	if mid == "" && postback != nil {
		mid = firstNonEmpty(messengerAnyString(postback["mid"]), messengerAnyString(postback["id"]), messengerAnyString(postback["message_id"]))
	}
	if messageType == "" && postback != nil {
		messageType = "postback"
	}
	if messageType == "" {
		messageType = "text"
	}
	isSample := strings.EqualFold(mid, "test_message_id") || strings.EqualFold(messageText, "test_message") || strings.HasPrefix(strings.ToLower(mid), "test_")
	key := strings.Join([]string{ctx.SenderID, ctx.RecipientID, mid, messageText, fmt.Sprint(ctx.Timestamp), messageType}, "|")
	if seen[key] {
		return
	}
	seen[key] = true
	*events = append(*events, messengerInboundEvent{
		SenderID:    strings.TrimSpace(ctx.SenderID),
		RecipientID: strings.TrimSpace(ctx.RecipientID),
		MessageID:   strings.TrimSpace(mid),
		Text:        strings.TrimSpace(messageText),
		MessageType: messageType,
		IsEcho:      isEcho,
		Timestamp:   ctx.Timestamp,
		IsSample:    isSample,
		SourcePath:  sourcePath,
	})
}

func messengerWalkWebhookNode(node any, path string, ctx messengerWebhookWalkContext, events *[]messengerInboundEvent, seen map[string]bool, paths map[string]bool) {
	switch value := node.(type) {
	case []any:
		for i, item := range value {
			messengerWalkWebhookNode(item, fmt.Sprintf("%s[%d]", path, i), ctx, events, seen, paths)
		}
	case map[string]any:
		local := ctx
		if field := messengerAnyString(value["field"]); field != "" {
			local.Field = field
		}
		for _, key := range []string{"sender", "from"} {
			if id := messengerPartyID(value[key]); id != "" {
				local.SenderID = id
				break
			}
		}
		for _, key := range []string{"recipient", "to"} {
			if id := messengerPartyID(value[key]); id != "" {
				local.RecipientID = id
				break
			}
		}
		if id := firstNonEmpty(messengerAnyString(value["sender_id"]), messengerAnyString(value["from_id"])); id != "" {
			local.SenderID = id
		}
		if id := firstNonEmpty(messengerAnyString(value["recipient_id"]), messengerAnyString(value["to_id"])); id != "" {
			local.RecipientID = id
		}
		if ts := firstNonEmpty(messengerAnyString(value["timestamp"]), messengerAnyString(value["time"])); ts != "" {
			local.Timestamp = messengerAnyInt64(ts)
		}

		message := messengerMap(value["message"])
		postback := messengerMap(value["postback"])
		if message != nil || postback != nil {
			messengerAppendInboundEvent(events, seen, local, message, postback, path)
			paths[path] = true
		}

		// Some current Meta webhook variants put one or more message objects
		// inside value.messages[] instead of value.message. Preserve sender and
		// recipient inherited from the parent while parsing each item.
		if messages := messengerSlice(value["messages"]); len(messages) > 0 {
			for i, rawMessage := range messages {
				messageMap := messengerMap(rawMessage)
				if messageMap == nil {
					continue
				}
				messageCtx := local
				if id := firstNonEmpty(messengerPartyID(messageMap["sender"]), messengerPartyID(messageMap["from"]), messengerAnyString(messageMap["from"])); id != "" {
					messageCtx.SenderID = id
				}
				if id := firstNonEmpty(messengerPartyID(messageMap["recipient"]), messengerPartyID(messageMap["to"]), messengerAnyString(messageMap["to"])); id != "" {
					messageCtx.RecipientID = id
				}
				if ts := firstNonEmpty(messengerAnyString(messageMap["timestamp"]), messengerAnyString(messageMap["time"])); ts != "" {
					messageCtx.Timestamp = messengerAnyInt64(ts)
				}
				payloadMessage := messageMap
				if nested := messengerMap(messageMap["message"]); nested != nil {
					payloadMessage = nested
				}
				messengerAppendInboundEvent(events, seen, messageCtx, payloadMessage, messengerMap(messageMap["postback"]), fmt.Sprintf("%s.messages[%d]", path, i))
				paths[fmt.Sprintf("%s.messages[]", path)] = true
			}
		}

		for key, child := range value {
			// message and postback were already consumed with the correct parent
			// sender/recipient context. Walking them again would create duplicates.
			if key == "message" || key == "postback" || key == "messages" {
				continue
			}
			childPath := key
			if path != "" {
				childPath = path + "." + key
			}
			messengerWalkWebhookNode(child, childPath, local, events, seen, paths)
		}
	}
}

func decodeMessengerWebhookPayload(body []byte) (string, []messengerInboundEvent, string, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var payload any
	if err := decoder.Decode(&payload); err != nil {
		return "", nil, "invalid_json", err
	}
	objectType := ""
	if root := messengerMap(payload); root != nil {
		objectType = messengerAnyString(root["object"])
	}
	events := make([]messengerInboundEvent, 0)
	seen := map[string]bool{}
	paths := map[string]bool{}
	messengerWalkWebhookNode(payload, "root", messengerWebhookWalkContext{}, &events, seen, paths)
	pathNames := make([]string, 0, len(paths))
	for path := range paths {
		pathNames = append(pathNames, path)
	}
	if len(pathNames) == 0 {
		return objectType, events, "unrecognized", nil
	}
	// Stable enough for diagnostics without importing sort only for display.
	shape := strings.Join(pathNames, ",")
	return objectType, events, shape, nil
}

func messengerPayloadPreview(body []byte) string {
	const limit = 16000
	body = bytes.TrimSpace(body)
	if len(body) > limit {
		return string(body[:limit]) + "…"
	}
	return string(body)
}

func (a *App) recordMessengerChannelEvent(c ChannelConnection, eventType, status string, detail map[string]any) {
	if strings.TrimSpace(eventType) == "" {
		return
	}
	if strings.TrimSpace(status) == "" {
		status = "received"
	}
	if detail == nil {
		detail = map[string]any{}
	}
	raw, _ := json.Marshal(detail)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		_, err = a.db.Exec(`INSERT INTO channel_events(tenant_id,connection_id,event_type,status,detail,created_at) VALUES(?,?,?,?,?,?)`, c.TenantID, c.ID, eventType, status, string(raw), now)
		if err == nil {
			return
		}
		time.Sleep(time.Duration(attempt+1) * 40 * time.Millisecond)
	}
	log.Printf("[messenger webhook] audit insert failed connection=%d event=%s error=%v", c.ID, eventType, err)
}

func (a *App) latestMessengerChannelEvent(c ChannelConnection, eventType string) (createdAt, status, detail string) {
	_ = a.db.QueryRow(`SELECT created_at,status,detail FROM channel_events WHERE tenant_id=? AND connection_id=? AND event_type=? ORDER BY id DESC LIMIT 1`, c.TenantID, c.ID, eventType).Scan(&createdAt, &status, &detail)
	return strings.TrimSpace(createdAt), strings.TrimSpace(status), strings.TrimSpace(detail)
}

// recordMessengerWebhookReceipt keeps the legacy config_json markers for
// compatibility, but the diagnostic source of truth is channel_events. The
// dedicated event table avoids losing webhook state when another action updates
// config_json at the same time.
func (a *App) recordMessengerWebhookReceipt(c ChannelConnection, shape string, eventCount int, parseErr error) {
	detail := map[string]any{"shape": shape, "event_count": eventCount}
	status := "processed"
	if parseErr != nil {
		status = "parse_error"
		detail["error"] = parseErr.Error()
	}
	a.recordMessengerChannelEvent(c, "messenger_webhook_processed", status, detail)

	cfg := map[string]any{}
	_ = json.Unmarshal([]byte(c.ConfigJSON), &cfg)
	if cfg == nil {
		cfg = map[string]any{}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	cfg["last_webhook_at"] = now
	cfg["last_webhook_shape"] = shape
	cfg["last_webhook_event_count"] = eventCount
	if parseErr != nil {
		cfg["last_webhook_error"] = parseErr.Error()
	} else {
		delete(cfg, "last_webhook_error")
	}
	raw, _ := json.Marshal(cfg)
	if _, err := a.db.Exec(`UPDATE channel_connections SET config_json=?,updated_at=? WHERE id=? AND tenant_id=?`, string(raw), now, c.ID, c.TenantID); err != nil {
		log.Printf("[messenger webhook] legacy diagnostic update failed connection=%d error=%v", c.ID, err)
	}
}

func (a *App) messengerTenantWebhookHandler(w http.ResponseWriter, r *http.Request) {
	pub := strings.TrimPrefix(r.URL.Path, "/webhooks/messenger/")
	if pub == "" {
		http.NotFound(w, r)
		return
	}
	var c ChannelConnection
	err := a.db.QueryRow(`SELECT id,tenant_id,public_id,type,name,status,external_account_id,assigned_agent_id,config_json,encrypted_credentials,last_connected_at,last_disconnected_at,last_message_at,last_error,created_at,updated_at FROM channel_connections WHERE public_id=? AND type='messenger'`, pub).Scan(&c.ID, &c.TenantID, &c.PublicID, &c.Type, &c.Name, &c.Status, &c.ExternalAccountID, &c.AssignedAgentID, &c.ConfigJSON, &c.EncryptedCredentials, &c.LastConnectedAt, &c.LastDisconnectedAt, &c.LastMessageAt, &c.LastError, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodGet {
		var cfg map[string]any
		_ = json.Unmarshal([]byte(c.ConfigJSON), &cfg)
		verify, _ := cfg["verify_token"].(string)
		if strings.TrimSpace(verify) == "" {
			if cr, credErr := a.messengerCredentialsFor(c); credErr == nil {
				verify = cr.VerifyToken
			}
		}
		if r.URL.Query().Get("hub.mode") == "subscribe" && verify != "" && hmac.Equal([]byte(r.URL.Query().Get("hub.verify_token")), []byte(verify)) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(r.URL.Query().Get("hub.challenge")))
			return
		}
		http.Error(w, "verification failed", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, bodyErr := io.ReadAll(io.LimitReader(r.Body, 2<<20))
	bodyReadError := ""
	if bodyErr != nil {
		bodyReadError = bodyErr.Error()
	}
	a.recordMessengerChannelEvent(c, "messenger_webhook_http", "received", map[string]any{
		"method":            r.Method,
		"path":              r.URL.Path,
		"content_type":      r.Header.Get("Content-Type"),
		"content_length":    len(body),
		"user_agent":        r.Header.Get("User-Agent"),
		"signature_present": strings.TrimSpace(r.Header.Get("X-Hub-Signature-256")) != "",
		"read_error":        bodyReadError,
		"payload_sha256":    fmt.Sprintf("%x", sha256.Sum256(body)),
		"payload_preview":   messengerPayloadPreview(body),
	})
	log.Printf("[messenger webhook] HTTP POST received connection=%d tenant=%d public_id=%s bytes=%d content_type=%q signature=%t", c.ID, c.TenantID, c.PublicID, len(body), r.Header.Get("Content-Type"), strings.TrimSpace(r.Header.Get("X-Hub-Signature-256")) != "")
	if bodyErr != nil {
		a.recordMessengerChannelEvent(c, "messenger_webhook_rejected", "body_read_error", map[string]any{"error": bodyErr.Error()})
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	cr, err := a.messengerCredentialsFor(c)
	if err != nil || strings.TrimSpace(cr.PageToken) == "" {
		detail := "Messenger credentials are not configured"
		if err != nil {
			detail = err.Error()
		}
		a.recordMessengerChannelEvent(c, "messenger_webhook_rejected", "not_configured", map[string]any{"error": detail})
		http.Error(w, "not configured", http.StatusServiceUnavailable)
		return
	}
	if cr.AppSecret != "" {
		sig := strings.TrimPrefix(r.Header.Get("X-Hub-Signature-256"), "sha256=")
		mac := hmac.New(sha256.New, []byte(cr.AppSecret))
		mac.Write(body)
		expected := hex.EncodeToString(mac.Sum(nil))
		if sig == "" || !hmac.Equal([]byte(sig), []byte(expected)) {
			a.recordMessengerChannelEvent(c, "messenger_webhook_rejected", "invalid_signature", map[string]any{"signature_present": sig != ""})
			http.Error(w, "invalid signature", 403)
			return
		}
	}

	objectType, events, shape, parseErr := decodeMessengerWebhookPayload(body)
	a.recordMessengerWebhookReceipt(c, shape, len(events), parseErr)
	if parseErr == nil && len(events) == 0 {
		a.recordMessengerChannelEvent(c, "messenger_webhook_unclassified", "no_message_candidates", map[string]any{
			"object":          objectType,
			"shape":           shape,
			"payload_sha256":  fmt.Sprintf("%x", sha256.Sum256(body)),
			"payload_preview": messengerPayloadPreview(body),
		})
	}
	log.Printf("[messenger webhook] connection=%d tenant=%d public_id=%s object=%s shape=%s events=%d parse_error=%v", c.ID, c.TenantID, c.PublicID, objectType, shape, len(events), parseErr)

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("EVENT_RECEIVED"))
	if parseErr != nil {
		return
	}
	for _, event := range events {
		if event.IsEcho {
			a.recordMessengerChannelEvent(c, "messenger_webhook_ignored", "echo", map[string]any{"message_id": event.MessageID, "source_path": event.SourcePath, "sender_id": event.SenderID, "recipient_id": event.RecipientID})
			continue
		}
		if event.IsSample {
			a.recordMessengerChannelEvent(c, "messenger_webhook_sample", "accepted", map[string]any{"shape": shape, "message_id": event.MessageID, "text": event.Text, "source_path": event.SourcePath})
			log.Printf("[messenger webhook] sample event accepted connection=%d shape=%s", c.ID, shape)
			continue
		}
		if event.SenderID == "" || event.Text == "" {
			a.recordMessengerChannelEvent(c, "messenger_webhook_ignored", "missing_sender_or_text", map[string]any{"shape": shape, "sender_id": event.SenderID, "recipient_id": event.RecipientID, "message_id": event.MessageID, "message_type": event.MessageType, "text": event.Text, "source_path": event.SourcePath})
			continue
		}
		messageID := event.MessageID
		if messageID == "" {
			messageID = "messenger-event-" + randomToken(10)
		}
		chat := "messenger:" + event.SenderID
		now := time.Now().UTC().Format(time.RFC3339Nano)
		a.recordMessengerChannelEvent(c, "messenger_message_received", "received", map[string]any{"shape": shape, "sender_id": event.SenderID, "recipient_id": event.RecipientID, "message_id": messageID, "message_type": event.MessageType, "source_path": event.SourcePath})

		if _, err := a.db.Exec(`INSERT OR IGNORE INTO worktic_messages(tenant_id,channel_connection_id,channel,wa_id,chat_jid,sender_jid,direction,message_type,text,status,timestamp) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, c.TenantID, c.ID, "messenger", messageID, chat, event.SenderID, "in", event.MessageType, event.Text, "received", now); err != nil {
			a.recordMessengerChannelEvent(c, "messenger_processing_error", "message_insert", map[string]any{"error": err.Error(), "message_id": messageID})
			log.Printf("[messenger webhook] message insert failed connection=%d sender=%s error=%v", c.ID, event.SenderID, err)
			continue
		}
		if _, err := a.db.Exec(`INSERT INTO worktic_contacts(tenant_id,channel_connection_id,chat_jid,channel,phone,name,unread,updated_at) VALUES(?,?,?,?,?,?,1,?) ON CONFLICT(chat_jid) DO UPDATE SET tenant_id=excluded.tenant_id,channel_connection_id=excluded.channel_connection_id,channel=excluded.channel,phone=excluded.phone,unread=worktic_contacts.unread+1,updated_at=excluded.updated_at`, c.TenantID, c.ID, chat, "messenger", event.SenderID, "Usuario Messenger", now); err != nil {
			a.recordMessengerChannelEvent(c, "messenger_processing_error", "contact_upsert", map[string]any{"error": err.Error(), "message_id": messageID})
			log.Printf("[messenger webhook] contact upsert failed connection=%d sender=%s error=%v", c.ID, event.SenderID, err)
		}
		if err := a.syncCRMContactAt(c.TenantID, "Usuario Messenger", "", "", "messenger", "conversation", chat, now); err != nil {
			a.recordMessengerChannelEvent(c, "messenger_processing_error", "crm_sync", map[string]any{"error": err.Error(), "message_id": messageID})
		}
		if err := a.syncOpportunityFromConversation(c.TenantID, chat, "messenger", event.Text, now); err != nil {
			a.recordMessengerChannelEvent(c, "messenger_processing_error", "opportunity_sync", map[string]any{"error": err.Error(), "message_id": messageID})
		}
		if _, err := a.db.Exec(`UPDATE channel_connections SET last_message_at=?,last_error='',updated_at=? WHERE id=? AND tenant_id=?`, now, now, c.ID, c.TenantID); err != nil {
			a.recordMessengerChannelEvent(c, "messenger_processing_error", "connection_update", map[string]any{"error": err.Error(), "message_id": messageID})
		}
		log.Printf("[messenger webhook] inbound message stored connection=%d sender=%s type=%s", c.ID, event.SenderID, event.MessageType)
		go a.maybeMessengerAIReply(c, event.SenderID, chat, event.Text)
	}

}

func (a *App) sendTenantMessengerText(ctx context.Context, tenantID int64, chat, text string) (string, error) {
	chat = strings.TrimSpace(chat)
	text = strings.TrimSpace(text)
	if tenantID <= 0 || !strings.HasPrefix(chat, "messenger:") || text == "" {
		return "", errors.New("conversación o mensaje inválido")
	}
	psid := strings.TrimPrefix(chat, "messenger:")
	var connectionID int64
	_ = a.db.QueryRow(`SELECT channel_connection_id FROM worktic_contacts WHERE tenant_id=? AND chat_jid=? AND channel='messenger' ORDER BY updated_at DESC LIMIT 1`, tenantID, chat).Scan(&connectionID)
	if connectionID <= 0 {
		_ = a.db.QueryRow(`SELECT channel_connection_id FROM worktic_messages WHERE tenant_id=? AND chat_jid=? AND channel='messenger' AND channel_connection_id>0 ORDER BY id DESC LIMIT 1`, tenantID, chat).Scan(&connectionID)
	}
	if connectionID <= 0 {
		return "", errors.New("no se encontró la conexión de Messenger asociada a esta conversación")
	}
	var c ChannelConnection
	err := a.db.QueryRow(`SELECT id,tenant_id,public_id,type,name,status,external_account_id,assigned_agent_id,config_json,encrypted_credentials,last_connected_at,last_disconnected_at,last_message_at,last_error,created_at,updated_at FROM channel_connections WHERE id=? AND tenant_id=? AND type='messenger'`, connectionID, tenantID).Scan(&c.ID, &c.TenantID, &c.PublicID, &c.Type, &c.Name, &c.Status, &c.ExternalAccountID, &c.AssignedAgentID, &c.ConfigJSON, &c.EncryptedCredentials, &c.LastConnectedAt, &c.LastDisconnectedAt, &c.LastMessageAt, &c.LastError, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return "", errors.New("la conexión de Messenger ya no está disponible")
	}
	cr, err := a.messengerCredentialsFor(c)
	if err != nil || strings.TrimSpace(cr.PageToken) == "" {
		return "", errors.New("Messenger no tiene un Page Access Token configurado")
	}
	payload, _ := json.Marshal(map[string]any{
		"recipient":      map[string]string{"id": psid},
		"messaging_type": "RESPONSE",
		"message":        map[string]string{"text": text},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://graph.facebook.com/"+a.messengerGraphVersion()+"/me/messages?access_token="+url.QueryEscape(cr.PageToken), bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var metaErr struct {
			Error struct {
				Message string `json:"message"`
				Code    int    `json:"code"`
			} `json:"error"`
		}
		_ = json.Unmarshal(body, &metaErr)
		detail := strings.TrimSpace(metaErr.Error.Message)
		if detail == "" {
			detail = strings.TrimSpace(string(body))
		}
		_, _ = a.db.Exec(`UPDATE channel_connections SET last_error=?,updated_at=? WHERE id=? AND tenant_id=?`, detail, time.Now().UTC().Format(time.RFC3339), c.ID, tenantID)
		return "", fmt.Errorf("Messenger: %s", detail)
	}
	var out struct {
		MessageID string `json:"message_id"`
	}
	_ = json.Unmarshal(body, &out)
	if strings.TrimSpace(out.MessageID) == "" {
		out.MessageID = "messenger-" + randomToken(10)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, _ = a.db.Exec(`INSERT OR IGNORE INTO worktic_messages(tenant_id,channel_connection_id,channel,wa_id,chat_jid,sender_jid,direction,message_type,text,status,timestamp) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, tenantID, c.ID, "messenger", out.MessageID, chat, c.ExternalAccountID, "out", "text", text, "sent", now)
	_, _ = a.db.Exec(`UPDATE worktic_contacts SET updated_at=? WHERE tenant_id=? AND chat_jid=?`, now, tenantID, chat)
	_, _ = a.db.Exec(`UPDATE channel_connections SET last_error='',updated_at=? WHERE id=? AND tenant_id=?`, now, c.ID, tenantID)
	return out.MessageID, nil
}

func (a *App) maybeMessengerAIReply(c ChannelConnection, psid, chat, text string) {
	if a.openAIKey() == "" {
		_, _ = a.db.Exec(`UPDATE channel_connections SET last_error='Falta configurar OPENAI_API_KEY' WHERE id=?`, c.ID)
		return
	}
	history := a.channelManager.tenantRecentHistory(c.TenantID, c.ID, chat, 10)
	agentID := c.AssignedAgentID
	system := ""
	if agentID > 0 {
		var ag AIAgent
		var d int
		err := a.db.QueryRow(`SELECT id,tenant_id,name,type,description,objective,tone,language,instructions,knowledge,greeting,away_message,handoff_rules,tools,channels,status,is_default,monthly_budget,created_at,updated_at FROM ai_agents WHERE id=? AND tenant_id=? AND status='active'`, agentID, c.TenantID).Scan(&ag.ID, &ag.TenantID, &ag.Name, &ag.Type, &ag.Description, &ag.Objective, &ag.Tone, &ag.Language, &ag.Instructions, &ag.Knowledge, &ag.Greeting, &ag.AwayMessage, &ag.HandoffRules, &ag.Tools, &ag.Channels, &ag.Status, &d, &ag.MonthlyBudget, &ag.CreatedAt, &ag.UpdatedAt)
		if err == nil {
			system = fmt.Sprintf("Eres %s. Objetivo: %s. Tono: %s. Instrucciones: %s. Conocimiento: %s. Historial:\n%s", ag.Name, ag.Objective, ag.Tone, ag.Instructions, ag.Knowledge, history)
		}
	}
	if system == "" {
		ag := a.loadAgent()
		if !ag.Enabled {
			return
		}
		system = fmt.Sprintf("Eres %s, asistente principal de %s. Objetivo: %s. Tono: %s. Instrucciones: %s. Conocimiento: %s. Historial:\n%s", ag.Name, ag.Company, ag.Objective, ag.Tone, ag.Instructions, ag.Knowledge, history)
	}
	reply, err := a.callOpenAI(system, text)
	if err != nil || strings.TrimSpace(reply) == "" {
		return
	}
	cr, err := a.messengerCredentialsFor(c)
	if err != nil {
		return
	}
	var out struct {
		MessageID string `json:"message_id"`
	}
	payload, _ := json.Marshal(map[string]any{"recipient": map[string]string{"id": psid}, "messaging_type": "RESPONSE", "message": map[string]string{"text": reply}})
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://graph.facebook.com/"+a.messengerGraphVersion()+"/me/messages?access_token="+url.QueryEscape(cr.PageToken), bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return
	}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	now := time.Now().UTC().Format(time.RFC3339)
	_, _ = a.db.Exec(`INSERT OR IGNORE INTO worktic_messages(tenant_id,channel_connection_id,channel,wa_id,chat_jid,sender_jid,direction,message_type,text,status,timestamp) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, c.TenantID, c.ID, "messenger", out.MessageID, chat, "ai", "out", "text", reply, "ai_sent", now)
	_, _ = a.db.Exec(`UPDATE channel_connections SET last_error='',updated_at=? WHERE id=?`, now, c.ID)
}
