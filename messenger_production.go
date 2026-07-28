package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// initMessengerProductionSchema adds the durable outbound queue and the
// provider-message idempotency constraint used by both the webhook and the
// Conversations API fallback.
func initMessengerProductionSchema(db *sql.DB) error {
	if _, err := db.Exec(`
CREATE TABLE IF NOT EXISTS messenger_outbox (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 tenant_id INTEGER NOT NULL,
 connection_id INTEGER NOT NULL,
 local_message_id TEXT NOT NULL UNIQUE,
 chat_jid TEXT NOT NULL,
 psid TEXT NOT NULL,
 text TEXT NOT NULL,
 source TEXT NOT NULL DEFAULT 'manual',
 status TEXT NOT NULL DEFAULT 'pending',
 attempts INTEGER NOT NULL DEFAULT 0,
 next_attempt_at TEXT NOT NULL,
 meta_message_id TEXT NOT NULL DEFAULT '',
 last_error TEXT NOT NULL DEFAULT '',
 created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL,
 sent_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_messenger_outbox_due ON messenger_outbox(status,next_attempt_at,id);
CREATE INDEX IF NOT EXISTS idx_messenger_outbox_connection ON messenger_outbox(tenant_id,connection_id,status,id DESC);
`); err != nil {
		return err
	}

	// Older builds did not enforce uniqueness for provider message IDs. Remove
	// exact duplicates before creating the partial unique index. Blank provider
	// IDs remain allowed for legacy records.
	_, _ = db.Exec(`DELETE FROM worktic_messages
WHERE wa_id<>'' AND id NOT IN (
 SELECT MIN(id) FROM worktic_messages
 WHERE wa_id<>''
 GROUP BY tenant_id,channel_connection_id,channel,wa_id
)`)
	_, err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_worktic_messages_provider_unique
ON worktic_messages(tenant_id,channel_connection_id,channel,wa_id) WHERE wa_id<>''`)
	return err
}

type messengerLongTokenExchange struct {
	PageToken          string
	PageName           string
	LongUserToken      string
	LongUserExpiresAt  int64
	PageTokenExpiresAt int64
	DataAccessExpires  int64
	Scopes             []string
}

func (a *App) exchangeMessengerUserToken(ctx context.Context, shortUserToken, appID, appSecret, pageID string) (messengerLongTokenExchange, error) {
	shortUserToken = strings.TrimSpace(shortUserToken)
	appID = strings.TrimSpace(firstNonEmpty(appID, a.cfg.MetaAppID))
	appSecret = strings.TrimSpace(firstNonEmpty(appSecret, a.cfg.MetaAppSecret))
	pageID = strings.TrimSpace(pageID)
	if shortUserToken == "" {
		return messengerLongTokenExchange{}, errors.New("pega el User Access Token temporal del Graph API Explorer")
	}
	if appID == "" || appSecret == "" {
		return messengerLongTokenExchange{}, errors.New("Meta App ID y App Secret son obligatorios para generar el token estable")
	}
	if pageID == "" {
		return messengerLongTokenExchange{}, errors.New("Page ID obligatorio")
	}

	values := url.Values{
		"grant_type":        {"fb_exchange_token"},
		"client_id":         {appID},
		"client_secret":     {appSecret},
		"fb_exchange_token": {shortUserToken},
	}
	endpoint := "https://graph.facebook.com/" + a.messengerGraphVersion() + "/oauth/access_token?" + values.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return messengerLongTokenExchange{}, err
	}
	resp, err := (&http.Client{Timeout: 25 * time.Second}).Do(req)
	if err != nil {
		return messengerLongTokenExchange{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return messengerLongTokenExchange{}, fmt.Errorf("Meta no pudo convertir el token: %s", messengerMetaErrorText(resp.StatusCode, body))
	}
	var exchanged struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &exchanged); err != nil || strings.TrimSpace(exchanged.AccessToken) == "" {
		return messengerLongTokenExchange{}, errors.New("Meta no devolvió el User Access Token de larga duración")
	}

	var page struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		AccessToken string `json:"access_token"`
	}
	if err := a.messengerGraph(http.MethodGet, "/"+url.PathEscape(pageID), exchanged.AccessToken, url.Values{"fields": {"id,name,access_token"}}, &page); err != nil {
		return messengerLongTokenExchange{}, fmt.Errorf("no fue posible obtener el Page Access Token estable: %w", err)
	}
	if strings.TrimSpace(page.ID) != "" && strings.TrimSpace(page.ID) != pageID {
		return messengerLongTokenExchange{}, fmt.Errorf("Meta devolvió la página %s y se esperaba %s", page.ID, pageID)
	}
	if strings.TrimSpace(page.AccessToken) == "" {
		return messengerLongTokenExchange{}, errors.New("Meta no devolvió access_token para la página; revisa el control total y los permisos de la página")
	}

	validation, err := a.validateMessengerPageToken(page.AccessToken, pageID, appID, appSecret)
	if err != nil {
		return messengerLongTokenExchange{}, fmt.Errorf("el Page Access Token obtenido no pasó la validación: %w", err)
	}
	longExpires := int64(0)
	if exchanged.ExpiresIn > 0 {
		longExpires = time.Now().UTC().Add(time.Duration(exchanged.ExpiresIn) * time.Second).Unix()
	}
	return messengerLongTokenExchange{
		PageToken:          page.AccessToken,
		PageName:           firstNonEmpty(page.Name, "Página "+pageID),
		LongUserToken:      exchanged.AccessToken,
		LongUserExpiresAt:  longExpires,
		PageTokenExpiresAt: validation.ExpiresAt,
		DataAccessExpires:  validation.DataAccessExpiresAt,
		Scopes:             validation.Scopes,
	}, nil
}

func messengerMetaErrorText(status int, body []byte) string {
	var payload struct {
		Error struct {
			Message string `json:"message"`
			Code    int    `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(body, &payload)
	if strings.TrimSpace(payload.Error.Message) != "" {
		if payload.Error.Code != 0 {
			return fmt.Sprintf("código %d · %s", payload.Error.Code, payload.Error.Message)
		}
		return payload.Error.Message
	}
	text := strings.TrimSpace(string(body))
	if text == "" {
		text = fmt.Sprintf("HTTP %d", status)
	}
	return text
}

func (a *App) persistMessengerLongToken(r *http.Request, c ChannelConnection, shortUserToken, pageID, appID, appSecret string) (map[string]any, error) {
	if previous, previousErr := a.messengerCredentialsFor(c); previousErr == nil {
		appID = firstNonEmpty(strings.TrimSpace(appID), previous.AppID, a.cfg.MetaAppID)
		appSecret = firstNonEmpty(strings.TrimSpace(appSecret), previous.AppSecret, a.cfg.MetaAppSecret)
	}
	exchange, err := a.exchangeMessengerUserToken(r.Context(), shortUserToken, appID, appSecret, pageID)
	if err != nil {
		return nil, err
	}
	result, err := a.configureMessenger(r, c, exchange.PageToken, pageID, appID, appSecret)
	if err != nil {
		return nil, err
	}

	// Reload the configuration generated by configureMessenger, then enrich it
	// with the long-lived user-token metadata. The user token is encrypted and
	// never returned to the browser or written to logs.
	var currentConfig string
	if err := a.db.QueryRow(`SELECT config_json FROM channel_connections WHERE id=? AND tenant_id=?`, c.ID, c.TenantID).Scan(&currentConfig); err != nil {
		return nil, err
	}
	cfg := map[string]any{}
	_ = json.Unmarshal([]byte(currentConfig), &cfg)
	if cfg == nil {
		cfg = map[string]any{}
	}
	verify := messengerAnyString(cfg["verify_token"])
	now := time.Now().UTC()
	cfg["token_source"] = "long_lived_user_exchange"
	cfg["long_user_token_expires_at"] = exchange.LongUserExpiresAt
	cfg["token_expires_at"] = exchange.PageTokenExpiresAt
	cfg["data_access_expires_at"] = exchange.DataAccessExpires
	cfg["token_scopes"] = exchange.Scopes
	cfg["page_name"] = exchange.PageName
	cfg["token_exchanged_at"] = now.Format(time.RFC3339Nano)
	cfgRaw, _ := json.Marshal(cfg)
	credentials := messengerCredentials{
		PageToken:          exchange.PageToken,
		AppID:              strings.TrimSpace(firstNonEmpty(appID, a.cfg.MetaAppID)),
		AppSecret:          strings.TrimSpace(firstNonEmpty(appSecret, a.cfg.MetaAppSecret)),
		VerifyToken:        verify,
		LongUserToken:      exchange.LongUserToken,
		LongUserExpiresAt:  exchange.LongUserExpiresAt,
		PageTokenExpiresAt: exchange.PageTokenExpiresAt,
		DataAccessExpires:  exchange.DataAccessExpires,
	}
	credRaw, _ := json.Marshal(credentials)
	enc := encryptLocal(string(credRaw), a.cfg.ChannelEncryptionKey)
	_, err = a.db.Exec(`UPDATE channel_connections SET encrypted_credentials=?,config_json=?,status='connected',last_error='',updated_at=? WHERE id=? AND tenant_id=?`, enc, string(cfgRaw), now.Format(time.RFC3339Nano), c.ID, c.TenantID)
	if err != nil {
		return nil, err
	}
	a.recordMessengerChannelEvent(c, "messenger_token_exchange", "success", map[string]any{
		"page_id":                    pageID,
		"page_token_expires_at":      exchange.PageTokenExpiresAt,
		"long_user_token_expires_at": exchange.LongUserExpiresAt,
		"data_access_expires_at":     exchange.DataAccessExpires,
		"scopes":                     exchange.Scopes,
	})
	result["token_source"] = "long_lived_user_exchange"
	result["long_user_token_expires_at"] = exchange.LongUserExpiresAt
	result["token_expires_at"] = exchange.PageTokenExpiresAt
	result["data_access_expires_at"] = exchange.DataAccessExpires
	return result, nil
}

func (a *App) refreshMessengerPageTokenFromStoredUser(c ChannelConnection, reason string) (bool, error) {
	cr, err := a.messengerCredentialsFor(c)
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(cr.LongUserToken) == "" {
		return false, errors.New("no existe User Access Token de larga duración guardado")
	}
	if cr.LongUserExpiresAt > 0 && time.Now().UTC().Unix() >= cr.LongUserExpiresAt {
		return false, errors.New("el User Access Token de larga duración venció; vuelve a autorizarlo")
	}
	pageID := strings.TrimSpace(c.ExternalAccountID)
	if pageID == "" {
		return false, errors.New("Page ID no disponible")
	}
	var page struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		AccessToken string `json:"access_token"`
	}
	if err := a.messengerGraph(http.MethodGet, "/"+url.PathEscape(pageID), cr.LongUserToken, url.Values{"fields": {"id,name,access_token"}}, &page); err != nil {
		return false, err
	}
	if strings.TrimSpace(page.AccessToken) == "" {
		return false, errors.New("Meta no devolvió un Page Access Token renovado")
	}
	validation, err := a.validateMessengerPageToken(page.AccessToken, pageID, cr.AppID, cr.AppSecret)
	if err != nil {
		return false, err
	}
	cr.PageToken = page.AccessToken
	cr.PageTokenExpiresAt = validation.ExpiresAt
	cr.DataAccessExpires = validation.DataAccessExpiresAt
	raw, _ := json.Marshal(cr)
	enc := encryptLocal(string(raw), a.cfg.ChannelEncryptionKey)
	var cfg map[string]any
	_ = json.Unmarshal([]byte(c.ConfigJSON), &cfg)
	if cfg == nil {
		cfg = map[string]any{}
	}
	cfg["token_expires_at"] = validation.ExpiresAt
	cfg["data_access_expires_at"] = validation.DataAccessExpiresAt
	cfg["token_scopes"] = validation.Scopes
	cfg["token_refreshed_at"] = time.Now().UTC().Format(time.RFC3339Nano)
	cfg["token_refresh_reason"] = reason
	if strings.TrimSpace(page.Name) != "" {
		cfg["page_name"] = page.Name
	}
	cfgRaw, _ := json.Marshal(cfg)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = a.db.Exec(`UPDATE channel_connections SET encrypted_credentials=?,config_json=?,last_error='',updated_at=? WHERE id=? AND tenant_id=?`, enc, string(cfgRaw), now, c.ID, c.TenantID)
	if err == nil {
		a.recordMessengerChannelEvent(c, "messenger_token_refresh", "success", map[string]any{"reason": reason, "token_expires_at": validation.ExpiresAt, "data_access_expires_at": validation.DataAccessExpiresAt})
	}
	return err == nil, err
}

func (a *App) monitorMessengerTokens() {
	rows, err := a.db.Query(`SELECT id,tenant_id,public_id,type,name,status,external_account_id,assigned_agent_id,config_json,encrypted_credentials,last_connected_at,last_disconnected_at,last_message_at,last_error,created_at,updated_at FROM channel_connections WHERE type='messenger' AND status='connected' ORDER BY id`)
	if err != nil {
		log.Printf("[messenger token] list error=%v", err)
		return
	}
	defer rows.Close()
	var connections []ChannelConnection
	for rows.Next() {
		var c ChannelConnection
		if rows.Scan(&c.ID, &c.TenantID, &c.PublicID, &c.Type, &c.Name, &c.Status, &c.ExternalAccountID, &c.AssignedAgentID, &c.ConfigJSON, &c.EncryptedCredentials, &c.LastConnectedAt, &c.LastDisconnectedAt, &c.LastMessageAt, &c.LastError, &c.CreatedAt, &c.UpdatedAt) == nil {
			connections = append(connections, c)
		}
	}
	for _, c := range connections {
		cr, credErr := a.messengerCredentialsFor(c)
		if credErr != nil {
			continue
		}
		validation, validationErr := a.validateMessengerPageToken(cr.PageToken, c.ExternalAccountID, cr.AppID, cr.AppSecret)
		if validationErr != nil {
			refreshed, refreshErr := a.refreshMessengerPageTokenFromStoredUser(c, "validation_failed")
			if !refreshed {
				detail := map[string]any{"error": validationErr.Error()}
				if refreshErr != nil {
					detail["refresh_error"] = refreshErr.Error()
				}
				a.recordMessengerChannelEvent(c, "messenger_token_health", "error", detail)
				_, _ = a.db.Exec(`UPDATE channel_connections SET last_error=?,updated_at=? WHERE id=? AND tenant_id=?`, "Messenger token: "+validationErr.Error(), time.Now().UTC().Format(time.RFC3339Nano), c.ID, c.TenantID)
			}
			continue
		}
		cr.PageTokenExpiresAt = validation.ExpiresAt
		cr.DataAccessExpires = validation.DataAccessExpiresAt
		nowUnix := time.Now().UTC().Unix()
		nearExpiry := validation.ExpiresAt > 0 && validation.ExpiresAt-nowUnix <= 24*3600
		if nearExpiry && strings.TrimSpace(cr.LongUserToken) != "" {
			if refreshed, refreshErr := a.refreshMessengerPageTokenFromStoredUser(c, "page_token_near_expiry"); refreshed {
				continue
			} else if refreshErr != nil {
				a.recordMessengerChannelEvent(c, "messenger_token_health", "warning", map[string]any{"error": refreshErr.Error(), "token_expires_at": validation.ExpiresAt})
			}
		}
		status := "healthy"
		detail := map[string]any{
			"method":                     validation.Method,
			"token_expires_at":           validation.ExpiresAt,
			"data_access_expires_at":     validation.DataAccessExpiresAt,
			"long_user_token_expires_at": cr.LongUserExpiresAt,
			"scopes":                     validation.Scopes,
		}
		if cr.LongUserExpiresAt > 0 && cr.LongUserExpiresAt-nowUnix <= 7*24*3600 {
			status = "reauthorization_due"
			detail["warning"] = "El User Access Token estable vence en menos de 7 días; genera uno nuevo antes del vencimiento."
		}
		a.recordMessengerChannelEvent(c, "messenger_token_health", status, detail)
	}
}

func (a *App) runMessengerTokenMonitor() {
	hours := envInt("MESSENGER_TOKEN_CHECK_HOURS", 6)
	if hours < 1 {
		hours = 1
	}
	if hours > 24 {
		hours = 24
	}
	time.Sleep(20 * time.Second)
	a.monitorMessengerTokens()
	ticker := time.NewTicker(time.Duration(hours) * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		a.monitorMessengerTokens()
	}
}

func messengerRetryDelay(attempt int) time.Duration {
	switch attempt {
	case 0, 1:
		return 10 * time.Second
	case 2:
		return 30 * time.Second
	case 3:
		return 2 * time.Minute
	case 4:
		return 10 * time.Minute
	default:
		return 30 * time.Minute
	}
}

func (a *App) sendMessengerTextDirect(ctx context.Context, c ChannelConnection, psid, text string) (string, error) {
	cr, err := a.messengerCredentialsFor(c)
	if err != nil || strings.TrimSpace(cr.PageToken) == "" {
		return "", errors.New("Messenger no tiene Page Access Token")
	}
	recipientJSON, _ := json.Marshal(map[string]string{"id": strings.TrimSpace(psid)})
	messageJSON, _ := json.Marshal(map[string]string{"text": strings.TrimSpace(text)})
	values := url.Values{
		"recipient":      {string(recipientJSON)},
		"messaging_type": {"RESPONSE"},
		"message":        {string(messageJSON)},
	}
	var out struct {
		MessageID string `json:"message_id"`
	}
	if err := a.messengerGraph(http.MethodPost, "/me/messages", cr.PageToken, values, &out); err != nil {
		var graphErr *metaGraphAPIError
		if errors.As(err, &graphErr) && (graphErr.Code == 190 || graphErr.Code == 102 || graphErr.Status == http.StatusUnauthorized) {
			if refreshed, _ := a.refreshMessengerPageTokenFromStoredUser(c, "send_oauth_error"); refreshed {
				cr, _ = a.messengerCredentialsFor(c)
				if retryErr := a.messengerGraph(http.MethodPost, "/me/messages", cr.PageToken, values, &out); retryErr == nil {
					if strings.TrimSpace(out.MessageID) == "" {
						out.MessageID = "messenger-" + randomToken(10)
					}
					return out.MessageID, nil
				}
			}
		}
		return "", err
	}
	if strings.TrimSpace(out.MessageID) == "" {
		out.MessageID = "messenger-" + randomToken(10)
	}
	return out.MessageID, nil
}

func (a *App) enqueueMessengerOutbound(c ChannelConnection, chat, psid, text, source string) (string, error) {
	chat = strings.TrimSpace(chat)
	psid = strings.TrimSpace(psid)
	text = strings.TrimSpace(text)
	if chat == "" || psid == "" || text == "" {
		return "", errors.New("mensaje de Messenger incompleto")
	}
	if source == "" {
		source = "manual"
	}
	localID := "messenger-queued-" + randomToken(12)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := a.db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`INSERT INTO worktic_messages(tenant_id,channel_connection_id,channel,wa_id,chat_jid,sender_jid,direction,message_type,text,status,timestamp) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, c.TenantID, c.ID, "messenger", localID, chat, c.ExternalAccountID, "out", "text", text, "queued", now); err != nil {
		return "", err
	}
	res, err := tx.Exec(`INSERT INTO messenger_outbox(tenant_id,connection_id,local_message_id,chat_jid,psid,text,source,status,attempts,next_attempt_at,created_at,updated_at) VALUES(?,?,?,?,?,?,?,'pending',0,?,?,?)`, c.TenantID, c.ID, localID, chat, psid, text, source, now, now, now)
	if err != nil {
		return "", err
	}
	if err = tx.Commit(); err != nil {
		return "", err
	}
	jobID, _ := res.LastInsertId()
	// Try immediately for a responsive UI. A failure remains safely queued and
	// the worker retries it with backoff.
	if metaID, sendErr := a.processMessengerOutboxJob(jobID); sendErr == nil && metaID != "" {
		return metaID, nil
	}
	return localID, nil
}

func (a *App) processMessengerOutboxJob(jobID int64) (string, error) {
	claimTime := time.Now().UTC().Format(time.RFC3339Nano)
	claim, claimErr := a.db.Exec(`UPDATE messenger_outbox SET status='processing',updated_at=? WHERE id=? AND status IN ('pending','retrying')`, claimTime, jobID)
	if claimErr != nil {
		return "", claimErr
	}
	if affected, _ := claim.RowsAffected(); affected == 0 {
		var status, metaID string
		if err := a.db.QueryRow(`SELECT status,meta_message_id FROM messenger_outbox WHERE id=?`, jobID).Scan(&status, &metaID); err != nil {
			return "", err
		}
		if status == "sent" {
			return metaID, nil
		}
		return "", errors.New("el mensaje ya está siendo procesado")
	}
	var job struct {
		ID, TenantID, ConnectionID                int64
		LocalID, Chat, PSID, Text, Source, Status string
		Attempts                                  int
	}
	err := a.db.QueryRow(`SELECT id,tenant_id,connection_id,local_message_id,chat_jid,psid,text,source,status,attempts FROM messenger_outbox WHERE id=?`, jobID).Scan(&job.ID, &job.TenantID, &job.ConnectionID, &job.LocalID, &job.Chat, &job.PSID, &job.Text, &job.Source, &job.Status, &job.Attempts)
	if err != nil {
		return "", err
	}
	var c ChannelConnection
	err = a.db.QueryRow(`SELECT id,tenant_id,public_id,type,name,status,external_account_id,assigned_agent_id,config_json,encrypted_credentials,last_connected_at,last_disconnected_at,last_message_at,last_error,created_at,updated_at FROM channel_connections WHERE id=? AND tenant_id=? AND type='messenger'`, job.ConnectionID, job.TenantID).Scan(&c.ID, &c.TenantID, &c.PublicID, &c.Type, &c.Name, &c.Status, &c.ExternalAccountID, &c.AssignedAgentID, &c.ConfigJSON, &c.EncryptedCredentials, &c.LastConnectedAt, &c.LastDisconnectedAt, &c.LastMessageAt, &c.LastError, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	metaID, sendErr := a.sendMessengerTextDirect(ctx, c, job.PSID, job.Text)
	now := time.Now().UTC()
	if sendErr == nil {
		messageStatus := "sent"
		if job.Source == "ai" {
			messageStatus = "ai_sent"
		}
		tx, txErr := a.db.Begin()
		if txErr != nil {
			return "", txErr
		}
		defer tx.Rollback()
		if _, txErr = tx.Exec(`UPDATE messenger_outbox SET status='sent',meta_message_id=?,last_error='',sent_at=?,updated_at=? WHERE id=?`, metaID, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano), job.ID); txErr != nil {
			return "", txErr
		}
		if _, txErr = tx.Exec(`UPDATE worktic_messages SET wa_id=?,status=?,timestamp=? WHERE tenant_id=? AND channel_connection_id=? AND wa_id=?`, metaID, messageStatus, now.Format(time.RFC3339Nano), job.TenantID, job.ConnectionID, job.LocalID); txErr != nil {
			return "", txErr
		}
		if txErr = tx.Commit(); txErr != nil {
			return "", txErr
		}
		_, _ = a.db.Exec(`UPDATE worktic_contacts SET updated_at=? WHERE tenant_id=? AND chat_jid=?`, now.Format(time.RFC3339Nano), job.TenantID, job.Chat)
		_, _ = a.db.Exec(`UPDATE channel_connections SET last_error='',updated_at=? WHERE id=? AND tenant_id=?`, now.Format(time.RFC3339Nano), c.ID, c.TenantID)
		a.recordMessengerChannelEvent(c, "messenger_outbound", "sent", map[string]any{"source": job.Source, "message_id": metaID, "attempts": job.Attempts + 1, "psid": job.PSID})
		return metaID, nil
	}

	attempts := job.Attempts + 1
	maxAttempts := envInt("MESSENGER_OUTBOX_MAX_ATTEMPTS", 5)
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	if maxAttempts > 10 {
		maxAttempts = 10
	}
	status := "retrying"
	messageStatus := "pending_retry"
	next := now.Add(messengerRetryDelay(attempts))
	if attempts >= maxAttempts {
		status = "failed"
		messageStatus = "failed"
	}
	_, _ = a.db.Exec(`UPDATE messenger_outbox SET status=?,attempts=?,next_attempt_at=?,last_error=?,updated_at=? WHERE id=?`, status, attempts, next.Format(time.RFC3339Nano), sendErr.Error(), now.Format(time.RFC3339Nano), job.ID)
	_, _ = a.db.Exec(`UPDATE worktic_messages SET status=? WHERE tenant_id=? AND channel_connection_id=? AND wa_id=?`, messageStatus, job.TenantID, job.ConnectionID, job.LocalID)
	_, _ = a.db.Exec(`UPDATE channel_connections SET last_error=?,updated_at=? WHERE id=? AND tenant_id=?`, "Envío Messenger: "+sendErr.Error(), now.Format(time.RFC3339Nano), c.ID, c.TenantID)
	a.recordMessengerChannelEvent(c, "messenger_outbound", status, map[string]any{"source": job.Source, "error": sendErr.Error(), "attempts": attempts, "next_attempt_at": next.Format(time.RFC3339Nano), "psid": job.PSID})
	return "", sendErr
}

func (a *App) runMessengerOutboxWorker() {
	time.Sleep(5 * time.Second)
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		nowTime := time.Now().UTC()
		now := nowTime.Format(time.RFC3339Nano)
		stale := nowTime.Add(-2 * time.Minute).Format(time.RFC3339Nano)
		_, _ = a.db.Exec(`UPDATE messenger_outbox SET status='retrying',next_attempt_at=?,last_error=CASE WHEN last_error='' THEN 'Reintento tras proceso interrumpido' ELSE last_error END,updated_at=? WHERE status='processing' AND updated_at<?`, now, now, stale)
		rows, err := a.db.Query(`SELECT id FROM messenger_outbox WHERE status IN ('pending','retrying') AND next_attempt_at<=? ORDER BY id LIMIT 20`, now)
		if err == nil {
			var ids []int64
			for rows.Next() {
				var id int64
				if rows.Scan(&id) == nil {
					ids = append(ids, id)
				}
			}
			rows.Close()
			for _, id := range ids {
				_, _ = a.processMessengerOutboxJob(id)
			}
		}
		<-ticker.C
	}
}

func (a *App) messengerOutboxStats(c ChannelConnection) map[string]any {
	stats := map[string]any{"pending": 0, "retrying": 0, "processing": 0, "failed": 0, "last_error": "", "last_sent_at": ""}
	rows, err := a.db.Query(`SELECT status,COUNT(*) FROM messenger_outbox WHERE tenant_id=? AND connection_id=? GROUP BY status`, c.TenantID, c.ID)
	if err == nil {
		for rows.Next() {
			var status string
			var count int
			if rows.Scan(&status, &count) == nil {
				stats[status] = count
			}
		}
		rows.Close()
	}
	var lastSentAt, lastError string
	_ = a.db.QueryRow(`SELECT sent_at FROM messenger_outbox WHERE tenant_id=? AND connection_id=? AND status='sent' ORDER BY id DESC LIMIT 1`, c.TenantID, c.ID).Scan(&lastSentAt)
	_ = a.db.QueryRow(`SELECT last_error FROM messenger_outbox WHERE tenant_id=? AND connection_id=? AND last_error<>'' ORDER BY id DESC LIMIT 1`, c.TenantID, c.ID).Scan(&lastError)
	stats["last_sent_at"] = lastSentAt
	stats["last_error"] = lastError
	return stats
}

func (a *App) retryMessengerOutbox(c ChannelConnection) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := a.db.Exec(`UPDATE messenger_outbox SET status='pending',next_attempt_at=?,updated_at=? WHERE tenant_id=? AND connection_id=? AND status IN ('failed','retrying')`, now, now, c.TenantID, c.ID)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func messengerUnixTimeString(v int64) string {
	if v <= 0 {
		return ""
	}
	return time.Unix(v, 0).UTC().Format(time.RFC3339Nano)
}

func messengerSecondsUntil(v int64) int64 {
	if v <= 0 {
		return 0
	}
	return v - time.Now().UTC().Unix()
}

func messengerAnyInt(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	case json.Number:
		i, _ := strconv.Atoi(x.String())
		return i
	default:
		return 0
	}
}
