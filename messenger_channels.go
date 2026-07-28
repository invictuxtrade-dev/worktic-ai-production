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
	"net/http"
	"net/url"
	"strings"
	"time"
)

type messengerCredentials struct {
	PageToken   string `json:"page_token"`
	AppSecret   string `json:"app_secret"`
	VerifyToken string `json:"verify_token"`
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

func (a *App) messengerGraph(method, path, token string, values url.Values, out any) error {
	endpoint := "https://graph.facebook.com/v22.0" + path
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
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Meta Graph API (%d): %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	if out != nil {
		return json.Unmarshal(b, out)
	}
	return nil
}

func (a *App) configureMessenger(r *http.Request, c ChannelConnection, pageToken, pageID, appSecret string) (map[string]any, error) {
	var page struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := a.messengerGraph(http.MethodGet, "/me", pageToken, url.Values{"fields": {"id,name"}}, &page); err != nil {
		return nil, err
	}
	if page.ID == "" {
		return nil, errors.New("Meta no devolvió el ID de la página")
	}
	if pageID != "" && pageID != page.ID {
		return nil, fmt.Errorf("el Page ID no coincide; Meta detectó %s", page.ID)
	}
	verify := ""
	var previousCfg map[string]any
	_ = json.Unmarshal([]byte(c.ConfigJSON), &previousCfg)
	if previousCfg != nil {
		verify, _ = previousCfg["verify_token"].(string)
	}
	if verify == "" {
		if previous, previousErr := a.messengerCredentialsFor(c); previousErr == nil {
			verify = previous.VerifyToken
			if appSecret == "" {
				appSecret = previous.AppSecret
			}
		}
	}
	if verify == "" {
		verify = "wtm_" + randomToken(18)
	}
	base, err := a.publicBaseURL(r)
	if err != nil {
		return nil, err
	}
	hook := strings.TrimRight(base, "/") + "/webhooks/messenger/" + c.PublicID
	cr := messengerCredentials{PageToken: pageToken, AppSecret: appSecret, VerifyToken: verify}
	raw, _ := json.Marshal(cr)
	enc := encryptLocal(string(raw), a.cfg.ChannelEncryptionKey)
	cfg, _ := json.Marshal(map[string]any{"page_id": page.ID, "page_name": page.Name, "webhook_url": hook, "verify_token": verify, "subscribed_fields": []string{"messages", "messaging_postbacks", "message_reads", "message_deliveries"}})
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = a.db.Exec(`UPDATE channel_connections SET encrypted_credentials=?,external_account_id=?,config_json=?,status='connected',last_connected_at=?,last_error='',updated_at=? WHERE id=? AND tenant_id=?`, enc, page.ID, string(cfg), now, now, c.ID, c.TenantID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "page_id": page.ID, "page_name": page.Name, "webhook_url": hook, "verify_token": verify}, nil
}

func (a *App) messengerConnectionDetails(r *http.Request, c ChannelConnection) map[string]any {
	var cfg map[string]any
	if json.Unmarshal([]byte(c.ConfigJSON), &cfg) != nil || cfg == nil {
		cfg = map[string]any{}
	}
	hook, _ := cfg["webhook_url"].(string)
	verify, _ := cfg["verify_token"].(string)
	pageName, _ := cfg["page_name"].(string)
	pageID, _ := cfg["page_id"].(string)
	if pageID == "" {
		pageID = c.ExternalAccountID
	}
	if hook == "" {
		if base, err := a.publicBaseURL(r); err == nil {
			hook = strings.TrimRight(base, "/") + "/webhooks/messenger/" + c.PublicID
		}
	}
	if verify == "" {
		if cr, err := a.messengerCredentialsFor(c); err == nil {
			verify = cr.VerifyToken
		}
	}
	return map[string]any{
		"ok":                  c.Status == "connected" && hook != "" && verify != "",
		"platform":            "messenger",
		"page_id":             pageID,
		"page_name":           pageName,
		"webhook_url":         hook,
		"verify_token":        verify,
		"assigned_agent_id":   c.AssignedAgentID,
		"status":              c.Status,
		"last_message_at":     c.LastMessageAt,
		"last_error":          c.LastError,
		"configuration_ready": hook != "" && verify != "",
	}
}

func (a *App) testMessengerConnection(r *http.Request, c ChannelConnection) (map[string]any, error) {
	result := a.messengerConnectionDetails(r, c)
	result["token_valid"] = false
	result["token_check_available"] = true
	result["subscription_manual"] = true
	result["recommended_fields"] = []string{"messages", "messaging_postbacks", "message_reads", "message_deliveries"}

	cr, err := a.messengerCredentialsFor(c)
	if err != nil {
		result["token_error"] = err.Error()
		result["ok"] = false
		return result, nil
	}
	var page struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if graphErr := a.messengerGraph(http.MethodGet, "/me", cr.PageToken, url.Values{"fields": {"id,name"}}, &page); graphErr != nil {
		result["token_error"] = graphErr.Error()
		result["token_check_available"] = false
		return result, nil
	}
	result["token_valid"] = true
	result["page_id"] = page.ID
	result["page_name"] = page.Name
	ready, _ := result["configuration_ready"].(bool)
	result["ok"] = ready && page.ID != ""
	return result, nil
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
	cr, err := a.messengerCredentialsFor(c)
	if err != nil {
		http.Error(w, "not configured", 503)
		return
	}
	if r.Method == http.MethodGet {
		if r.URL.Query().Get("hub.mode") == "subscribe" && hmac.Equal([]byte(r.URL.Query().Get("hub.verify_token")), []byte(cr.VerifyToken)) {
			w.WriteHeader(200)
			_, _ = w.Write([]byte(r.URL.Query().Get("hub.challenge")))
			return
		}
		http.Error(w, "verification failed", 403)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	body, _ := io.ReadAll(io.LimitReader(r.Body, 2<<20))
	if cr.AppSecret != "" {
		sig := strings.TrimPrefix(r.Header.Get("X-Hub-Signature-256"), "sha256=")
		mac := hmac.New(sha256.New, []byte(cr.AppSecret))
		mac.Write(body)
		expected := hex.EncodeToString(mac.Sum(nil))
		if sig == "" || !hmac.Equal([]byte(sig), []byte(expected)) {
			http.Error(w, "invalid signature", 403)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("EVENT_RECEIVED"))
	var payload struct {
		Object string `json:"object"`
		Entry  []struct {
			Messaging []struct {
				Sender struct {
					ID string `json:"id"`
				} `json:"sender"`
				Message struct {
					MID    string `json:"mid"`
					Text   string `json:"text"`
					IsEcho bool   `json:"is_echo"`
				} `json:"message"`
				Timestamp int64 `json:"timestamp"`
			} `json:"messaging"`
		} `json:"entry"`
	}
	if json.Unmarshal(body, &payload) != nil {
		return
	}
	for _, e := range payload.Entry {
		for _, m := range e.Messaging {
			if m.Message.IsEcho || strings.TrimSpace(m.Message.Text) == "" || m.Sender.ID == "" {
				continue
			}
			chat := "messenger:" + m.Sender.ID
			now := time.Now().UTC().Format(time.RFC3339)
			_, _ = a.db.Exec(`INSERT OR IGNORE INTO worktic_messages(tenant_id,channel_connection_id,channel,wa_id,chat_jid,sender_jid,direction,message_type,text,status,timestamp) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, c.TenantID, c.ID, "messenger", m.Message.MID, chat, m.Sender.ID, "in", "text", m.Message.Text, "received", now)
			_, _ = a.db.Exec(`INSERT INTO worktic_contacts(tenant_id,channel_connection_id,chat_jid,channel,phone,name,unread,updated_at) VALUES(?,?,?,?,?,?,1,?) ON CONFLICT(chat_jid) DO UPDATE SET unread=worktic_contacts.unread+1,updated_at=excluded.updated_at`, c.TenantID, c.ID, chat, "messenger", m.Sender.ID, "Usuario Messenger", now)
			_ = a.syncCRMContactAt(c.TenantID, "Usuario Messenger", "", "", "messenger", "conversation", chat, now)
			_ = a.syncOpportunityFromConversation(c.TenantID, chat, "messenger", m.Message.Text, now)
			_, _ = a.db.Exec(`UPDATE channel_connections SET last_message_at=?,last_error='',updated_at=? WHERE id=?`, now, now, c.ID)
			go a.maybeMessengerAIReply(c, m.Sender.ID, chat, m.Message.Text)
		}
	}
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
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://graph.facebook.com/v22.0/me/messages?access_token="+url.QueryEscape(cr.PageToken), bytes.NewReader(payload))
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
