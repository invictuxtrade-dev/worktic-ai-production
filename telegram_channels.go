package main

import (
	"bytes"
	"context"
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

type telegramChannelCredentials struct {
	Token         string `json:"token"`
	WebhookSecret string `json:"webhook_secret"`
}

type telegramAPIEnvelope struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result"`
	Description string          `json:"description"`
}

type telegramBotInfo struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name"`
	Username  string `json:"username"`
}

type telegramWebhookInfo struct {
	URL                  string `json:"url"`
	HasCustomCertificate bool   `json:"has_custom_certificate"`
	PendingUpdateCount   int    `json:"pending_update_count"`
	LastErrorDate        int64  `json:"last_error_date"`
	LastErrorMessage     string `json:"last_error_message"`
	MaxConnections       int    `json:"max_connections"`
	IPAddress            string `json:"ip_address"`
}

type telegramUpdate struct {
	UpdateID int64 `json:"update_id"`
	Message  *struct {
		MessageID int64  `json:"message_id"`
		Date      int64  `json:"date"`
		Text      string `json:"text"`
		Chat      struct {
			ID        int64  `json:"id"`
			Type      string `json:"type"`
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
			Username  string `json:"username"`
		} `json:"chat"`
		From struct {
			ID        int64  `json:"id"`
			IsBot     bool   `json:"is_bot"`
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
			Username  string `json:"username"`
		} `json:"from"`
	} `json:"message"`
}

func (a *App) telegramChannelCredentials(connectionID int64) (telegramChannelCredentials, error) {
	var enc string
	if err := a.db.QueryRow(`SELECT encrypted_credentials FROM channel_connections WHERE id=? AND type='telegram'`, connectionID).Scan(&enc); err != nil {
		return telegramChannelCredentials{}, errors.New("conexión Telegram no encontrada")
	}
	plain := decryptLocal(enc, a.cfg.ChannelEncryptionKey)
	if plain == "" {
		return telegramChannelCredentials{}, errors.New("no fue posible descifrar las credenciales de Telegram")
	}
	var c telegramChannelCredentials
	if json.Unmarshal([]byte(plain), &c) == nil && c.Token != "" {
		return c, nil
	}
	// Compatibilidad con conexiones de versiones anteriores, donde se cifraba solo el token.
	c.Token = plain
	return c, nil
}

func encryptTelegramCredentials(c telegramChannelCredentials, key string) string {
	b, _ := json.Marshal(c)
	return encryptLocal(string(b), key)
}

func telegramAPICall(ctx context.Context, token, method string, values url.Values, out any) error {
	if strings.TrimSpace(token) == "" {
		return errors.New("token de Telegram vacío")
	}
	var body io.Reader
	if values != nil {
		body = strings.NewReader(values.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.telegram.org/bot"+token+"/"+method, body)
	if err != nil {
		return err
	}
	if values != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Telegram no respondió: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return err
	}
	var env telegramAPIEnvelope
	if json.Unmarshal(raw, &env) != nil {
		return fmt.Errorf("respuesta inválida de Telegram (HTTP %d)", resp.StatusCode)
	}
	if !env.OK {
		if env.Description == "" {
			env.Description = strings.TrimSpace(string(raw))
		}
		return fmt.Errorf("Telegram: %s", env.Description)
	}
	if out != nil && len(env.Result) > 0 {
		if err := json.Unmarshal(env.Result, out); err != nil {
			return fmt.Errorf("no fue posible interpretar la respuesta de Telegram: %w", err)
		}
	}
	return nil
}

// resolveTelegramWebhookURL follows only HTTP redirects and returns the final
// public URL that Telegram must call directly. Telegram rejects webhook
// endpoints that answer with 301/302/307/308, so Worktic canonicalizes the
// address before registering it.
func resolveTelegramWebhookURL(ctx context.Context, candidate string) (string, []string, error) {
	current := strings.TrimSpace(candidate)
	if current == "" {
		return "", nil, errors.New("URL de webhook vacía")
	}
	history := []string{current}
	client := &http.Client{
		Timeout: 12 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	for i := 0; i < 6; i++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, current, bytes.NewReader([]byte(`{}`)))
		if err != nil {
			return "", history, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "Worktic-Telegram-Webhook-Probe/1.0")
		resp, err := client.Do(req)
		if err != nil {
			return "", history, fmt.Errorf("no fue posible comprobar la URL pública del webhook: %w", err)
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
		resp.Body.Close()
		if resp.StatusCode < 300 || resp.StatusCode > 399 {
			return current, history, nil
		}
		location := strings.TrimSpace(resp.Header.Get("Location"))
		if location == "" {
			return "", history, fmt.Errorf("el webhook responde HTTP %d sin indicar destino", resp.StatusCode)
		}
		base, err := url.Parse(current)
		if err != nil {
			return "", history, err
		}
		next, err := base.Parse(location)
		if err != nil {
			return "", history, err
		}
		if next.Scheme != "https" {
			return "", history, fmt.Errorf("el webhook redirige a una URL no segura: %s", next.String())
		}
		current = next.String()
		history = append(history, current)
	}
	return "", history, errors.New("el webhook tiene demasiadas redirecciones")
}

func setTelegramWebhook(ctx context.Context, token, webhookURL, secret string) error {
	values := url.Values{
		"url":                  {webhookURL},
		"secret_token":         {secret},
		"drop_pending_updates": {"false"},
		"allowed_updates":      {`["message"]`},
	}
	return telegramAPICall(ctx, token, "setWebhook", values, nil)
}

func (a *App) publicBaseURL(r *http.Request) (string, error) {
	configured := strings.TrimRight(strings.TrimSpace(a.cfg.BaseURL), "/")
	if configured != "" && !strings.Contains(configured, "localhost") && !strings.Contains(configured, "127.0.0.1") {
		if !strings.HasPrefix(configured, "https://") {
			return "", errors.New("BASE_URL debe comenzar por https:// para registrar el webhook de Telegram")
		}
		return configured, nil
	}
	host := strings.TrimSpace(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = strings.TrimSpace(r.Host)
	}
	if host == "" {
		return "", errors.New("no fue posible determinar el dominio público; configura BASE_URL")
	}
	proto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))
	if proto == "" {
		if r.TLS != nil {
			proto = "https"
		} else {
			proto = "http"
		}
	}
	if proto != "https" {
		return "", errors.New("Telegram exige una URL HTTPS pública; configura BASE_URL=https://tu-dominio")
	}
	return "https://" + host, nil
}

func (a *App) configureTelegramWebhook(r *http.Request, c ChannelConnection, token string) (telegramBotInfo, telegramWebhookInfo, string, error) {
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	var bot telegramBotInfo
	if err := telegramAPICall(ctx, token, "getMe", nil, &bot); err != nil {
		return bot, telegramWebhookInfo{}, "", err
	}
	if !bot.IsBot || bot.ID == 0 {
		return bot, telegramWebhookInfo{}, "", errors.New("el token no pertenece a un bot válido")
	}
	base, err := a.publicBaseURL(r)
	if err != nil {
		return bot, telegramWebhookInfo{}, "", err
	}
	candidateURL := base + "/webhooks/telegram/" + url.PathEscape(c.PublicID)
	webhookURL, redirectHistory, err := resolveTelegramWebhookURL(ctx, candidateURL)
	if err != nil {
		return bot, telegramWebhookInfo{}, "", err
	}
	if len(redirectHistory) > 1 {
		log.Printf("[TG-WEBHOOK] URL canonicalizada: %s -> %s", candidateURL, webhookURL)
	}
	secret := randomToken(16)
	if err := setTelegramWebhook(ctx, token, webhookURL, secret); err != nil {
		return bot, telegramWebhookInfo{}, "", err
	}
	var info telegramWebhookInfo
	if err := telegramAPICall(ctx, token, "getWebhookInfo", nil, &info); err != nil {
		return bot, info, "", err
	}
	if strings.TrimRight(info.URL, "/") != strings.TrimRight(webhookURL, "/") {
		return bot, info, "", fmt.Errorf("Telegram no confirmó el webhook esperado; recibió %q", info.URL)
	}
	return bot, info, secret, nil
}

func (a *App) testTelegramConnection(r *http.Request, c ChannelConnection) (map[string]any, error) {
	creds, err := a.telegramChannelCredentials(c.ID)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	var bot telegramBotInfo
	if err := telegramAPICall(ctx, creds.Token, "getMe", nil, &bot); err != nil {
		return nil, err
	}
	base, err := a.publicBaseURL(r)
	if err != nil {
		return nil, err
	}
	candidate := base + "/webhooks/telegram/" + url.PathEscape(c.PublicID)
	resolved, redirectHistory, err := resolveTelegramWebhookURL(ctx, candidate)
	if err != nil {
		return nil, err
	}
	var info telegramWebhookInfo
	if err := telegramAPICall(ctx, creds.Token, "getWebhookInfo", nil, &info); err != nil {
		return nil, err
	}
	repaired := false
	currentMatches := strings.TrimRight(info.URL, "/") == strings.TrimRight(resolved, "/")
	redirectError := strings.Contains(strings.ToLower(info.LastErrorMessage), "redirect") || strings.Contains(info.LastErrorMessage, "307") || strings.Contains(info.LastErrorMessage, "301") || strings.Contains(info.LastErrorMessage, "302") || strings.Contains(info.LastErrorMessage, "308")
	if !currentMatches || redirectError {
		if strings.TrimSpace(creds.WebhookSecret) == "" {
			creds.WebhookSecret = randomToken(16)
		}
		if err := setTelegramWebhook(ctx, creds.Token, resolved, creds.WebhookSecret); err != nil {
			return nil, err
		}
		if err := telegramAPICall(ctx, creds.Token, "getWebhookInfo", nil, &info); err != nil {
			return nil, err
		}
		credsEncrypted := encryptTelegramCredentials(creds, a.cfg.ChannelEncryptionKey)
		_, _ = a.db.Exec(`UPDATE channel_connections SET encrypted_credentials=?,last_error='',updated_at=? WHERE id=?`, credsEncrypted, time.Now().UTC().Format(time.RFC3339), c.ID)
		repaired = true
	}
	matches := strings.TrimRight(info.URL, "/") == strings.TrimRight(resolved, "/")
	operational := matches && (strings.TrimSpace(info.LastErrorMessage) == "" || repaired)
	result := map[string]any{
		"ok":                   operational,
		"platform":             "telegram",
		"token_valid":          true,
		"bot_id":               bot.ID,
		"bot_username":         bot.Username,
		"bot_name":             bot.FirstName,
		"webhook_configured":   info.URL != "",
		"webhook_matches":      matches,
		"webhook_url":          info.URL,
		"expected_webhook_url": resolved,
		"original_webhook_url": candidate,
		"redirect_detected":    len(redirectHistory) > 1,
		"redirect_history":     redirectHistory,
		"auto_repaired":        repaired,
		"pending_updates":      info.PendingUpdateCount,
		"last_error":           info.LastErrorMessage,
		"status":               map[bool]string{true: "operational", false: "attention_required"}[operational],
	}
	return result, nil
}

func (a *App) telegramWebhookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}
	publicID := strings.TrimPrefix(r.URL.Path, "/webhooks/telegram/")
	publicID, _ = url.PathUnescape(strings.TrimSpace(publicID))
	if publicID == "" || strings.Contains(publicID, "/") {
		http.NotFound(w, r)
		return
	}
	var c ChannelConnection
	var enc string
	err := a.db.QueryRow(`SELECT id,tenant_id,public_id,type,name,status,external_account_id,assigned_agent_id,config_json,last_connected_at,last_disconnected_at,last_message_at,last_error,created_at,updated_at,encrypted_credentials FROM channel_connections WHERE public_id=? AND type='telegram'`, publicID).Scan(
		&c.ID, &c.TenantID, &c.PublicID, &c.Type, &c.Name, &c.Status, &c.ExternalAccountID, &c.AssignedAgentID, &c.ConfigJSON, &c.LastConnectedAt, &c.LastDisconnectedAt, &c.LastMessageAt, &c.LastError, &c.CreatedAt, &c.UpdatedAt, &enc,
	)
	if err != nil || c.Status != "connected" {
		http.NotFound(w, r)
		return
	}
	plain := decryptLocal(enc, a.cfg.ChannelEncryptionKey)
	var creds telegramChannelCredentials
	if json.Unmarshal([]byte(plain), &creds) != nil {
		creds.Token = plain
	}
	if creds.WebhookSecret == "" || r.Header.Get("X-Telegram-Bot-Api-Secret-Token") != creds.WebhookSecret {
		http.Error(w, "Webhook no autorizado", http.StatusUnauthorized)
		return
	}
	var update telegramUpdate
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	if err := dec.Decode(&update); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}
	// Telegram espera una respuesta rápida. Procesamos la IA fuera del request.
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"ok":true}`))
	if update.Message == nil || update.Message.From.IsBot {
		return
	}
	text := strings.TrimSpace(update.Message.Text)
	if text == "" {
		text = "[Mensaje no textual]"
	}
	chatID := strconv.FormatInt(update.Message.Chat.ID, 10)
	storedChat := fmt.Sprintf("t%d:c%d:telegram:%s", c.TenantID, c.ID, chatID)
	name := strings.TrimSpace(strings.Join([]string{update.Message.From.FirstName, update.Message.From.LastName}, " "))
	if name == "" {
		name = update.Message.From.Username
	}
	at := time.Now().UTC()
	if update.Message.Date > 0 {
		at = time.Unix(update.Message.Date, 0).UTC()
	}
	now := at.Format(time.RFC3339)
	messageID := "tg-" + strconv.FormatInt(update.Message.MessageID, 10)
	_, _ = a.db.Exec(`INSERT OR IGNORE INTO worktic_messages(tenant_id,channel_connection_id,channel,wa_id,chat_jid,sender_jid,direction,message_type,text,status,timestamp) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, c.TenantID, c.ID, "telegram", messageID, storedChat, chatID, "in", "text", text, "received", now)
	_, _ = a.db.Exec(`INSERT INTO worktic_contacts(tenant_id,channel_connection_id,chat_jid,channel,phone,name,unread,updated_at) VALUES(?,?,?,?,?,?,1,?) ON CONFLICT(chat_jid) DO UPDATE SET unread=worktic_contacts.unread+1,name=CASE WHEN excluded.name<>'' THEN excluded.name ELSE worktic_contacts.name END,updated_at=excluded.updated_at`, c.TenantID, c.ID, storedChat, "telegram", chatID, name, now)
	_, _ = a.db.Exec(`UPDATE channel_connections SET last_message_at=?,last_error='',updated_at=? WHERE id=?`, now, now, c.ID)
	log.Printf("[TG-WEBHOOK] recibido tenant=%d conexion=%d chat=%s update=%d", c.TenantID, c.ID, chatID, update.UpdateID)
	go a.maybeTenantTelegramAIReply(c, creds.Token, chatID, storedChat, text)
}

func (a *App) maybeTenantTelegramAIReply(c ChannelConnection, token, telegramChatID, storedChat, text string) {
	if strings.TrimSpace(text) == "" || strings.TrimSpace(token) == "" {
		return
	}
	if a.openAIKey() == "" {
		_, _ = a.db.Exec(`UPDATE channel_connections SET last_error=?,updated_at=? WHERE id=?`, "Falta configurar OPENAI_API_KEY", time.Now().UTC().Format(time.RFC3339), c.ID)
		return
	}
	key := fmt.Sprintf("tenant-ai:%d:%d:%s", c.TenantID, c.ID, storedChat)
	a.mu.Lock()
	if t, ok := a.autoLast[key]; ok && time.Since(t) < time.Duration(a.cfg.AutoReplyCooldownSeconds)*time.Second {
		a.mu.Unlock()
		return
	}
	a.autoLast[key] = time.Now()
	a.mu.Unlock()

	agentID := c.AssignedAgentID
	if agentID == 0 {
		if resolved, err := a.resolveAgent(c.TenantID, "telegram", "", text, 0, 0, 0); err == nil {
			agentID = resolved
		}
	}
	history := a.channelManager.tenantRecentHistory(c.TenantID, c.ID, storedChat, 10)
	system := ""
	if agentID > 0 {
		var ag AIAgent
		var isDefault int
		err := a.db.QueryRow(`SELECT id,tenant_id,name,type,description,objective,tone,language,instructions,knowledge,greeting,away_message,handoff_rules,tools,channels,status,is_default,monthly_budget,created_at,updated_at FROM ai_agents WHERE id=? AND tenant_id=? AND status='active'`, agentID, c.TenantID).Scan(
			&ag.ID, &ag.TenantID, &ag.Name, &ag.Type, &ag.Description, &ag.Objective, &ag.Tone, &ag.Language, &ag.Instructions, &ag.Knowledge, &ag.Greeting, &ag.AwayMessage, &ag.HandoffRules, &ag.Tools, &ag.Channels, &ag.Status, &isDefault, &ag.MonthlyBudget, &ag.CreatedAt, &ag.UpdatedAt,
		)
		if err == nil {
			system = fmt.Sprintf("Eres %s, agente especializado de tipo %s. Objetivo: %s. Tono: %s. Idioma: %s. Instrucciones: %s. Conocimiento verificado: %s. Herramientas permitidas: %s. No inventes datos y responde de forma humana, clara y breve. Historial reciente:\n%s", ag.Name, ag.Type, ag.Objective, ag.Tone, ag.Language, ag.Instructions, ag.Knowledge, ag.Tools, history)
		} else {
			agentID = 0
			_, _ = a.db.Exec(`UPDATE channel_connections SET assigned_agent_id=0,last_error=?,updated_at=? WHERE id=?`, "Agente asignado inválido; usando Asistente Principal", time.Now().UTC().Format(time.RFC3339), c.ID)
		}
	}
	if system == "" {
		legacy := a.loadAgent()
		if !legacy.Enabled {
			_, _ = a.db.Exec(`UPDATE channel_connections SET last_error=?,updated_at=? WHERE id=?`, "Activa el Asistente Principal o asigna un Agente Especializado activo", time.Now().UTC().Format(time.RFC3339), c.ID)
			return
		}
		system = fmt.Sprintf("Eres %s, asistente principal de %s. Objetivo: %s. Tono: %s. Instrucciones: %s. Conocimiento verificado: %s. No inventes datos y responde de forma humana, clara y breve. Historial reciente:\n%s", legacy.Name, legacy.Company, legacy.Objective, legacy.Tone, legacy.Instructions, legacy.Knowledge, history)
	}
	log.Printf("[TG-AI] generando tenant=%d conexion=%d agente=%d", c.TenantID, c.ID, agentID)
	reply, err := a.callOpenAI(system, text)
	period := time.Now().UTC().Format("2006-01")
	if err != nil {
		if agentID > 0 {
			_, _ = a.db.Exec(`INSERT INTO ai_agent_usage(tenant_id,agent_id,period,channel,conversations,responses,errors) VALUES(?,?,?,'telegram',1,0,1) ON CONFLICT(tenant_id,agent_id,period,channel) DO UPDATE SET conversations=conversations+1,errors=errors+1`, c.TenantID, agentID, period)
		}
		_, _ = a.db.Exec(`UPDATE channel_connections SET last_error=?,updated_at=? WHERE id=?`, err.Error(), time.Now().UTC().Format(time.RFC3339), c.ID)
		return
	}
	if strings.TrimSpace(reply) == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	vals := url.Values{"chat_id": {telegramChatID}, "text": {reply}}
	var sent struct {
		MessageID int64 `json:"message_id"`
	}
	if err := telegramAPICall(ctx, token, "sendMessage", vals, &sent); err != nil {
		_, _ = a.db.Exec(`UPDATE channel_connections SET last_error=?,updated_at=? WHERE id=?`, err.Error(), time.Now().UTC().Format(time.RFC3339), c.ID)
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	outID := "tg-" + strconv.FormatInt(sent.MessageID, 10)
	_, _ = a.db.Exec(`INSERT OR IGNORE INTO worktic_messages(tenant_id,channel_connection_id,channel,wa_id,chat_jid,sender_jid,direction,message_type,text,status,timestamp) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, c.TenantID, c.ID, "telegram", outID, storedChat, "ai", "out", "text", reply, "ai_sent", now)
	if agentID > 0 {
		_, _ = a.db.Exec(`INSERT INTO ai_agent_usage(tenant_id,agent_id,period,channel,conversations,responses) VALUES(?,?,?,'telegram',1,1) ON CONFLICT(tenant_id,agent_id,period,channel) DO UPDATE SET conversations=conversations+1,responses=responses+1`, c.TenantID, agentID, period)
	}
	_, _ = a.db.Exec(`UPDATE channel_connections SET last_error='',last_message_at=?,updated_at=? WHERE id=?`, now, now, c.ID)
	log.Printf("[TG-AI] respuesta enviada tenant=%d conexion=%d mensaje=%d", c.TenantID, c.ID, sent.MessageID)
}

func (a *App) deleteTelegramWebhook(connectionID int64) {
	creds, err := a.telegramChannelCredentials(connectionID)
	if err != nil || creds.Token == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	_ = telegramAPICall(ctx, creds.Token, "deleteWebhook", url.Values{"drop_pending_updates": {"false"}}, nil)
}

// Evita que el compilador marque bytes como no usado si futuras versiones cambian el transporte.
var _ = bytes.MinRead
