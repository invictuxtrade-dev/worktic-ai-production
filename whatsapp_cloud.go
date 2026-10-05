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
	"strconv"
	"strings"
	"time"
)

type whatsappCloudCredentials struct {
	AccessToken string `json:"access_token"`
}

type whatsappCloudConfig struct {
	WABAID        string `json:"waba_id"`
	PhoneNumberID string `json:"phone_number_id"`
	DisplayPhone  string `json:"display_phone_number,omitempty"`
	VerifiedName  string `json:"verified_name,omitempty"`
	QualityRating string `json:"quality_rating,omitempty"`
	WebhookURL    string `json:"webhook_url,omitempty"`
	ProviderMode  string `json:"provider_mode,omitempty"`
}

func (a *App) whatsappCloudCredentialsFor(c ChannelConnection) (whatsappCloudCredentials, error) {
	var out whatsappCloudCredentials
	raw := decryptLocal(c.EncryptedCredentials, a.cfg.ChannelEncryptionKey)
	if raw == "" || json.Unmarshal([]byte(raw), &out) != nil || strings.TrimSpace(out.AccessToken) == "" {
		return out, errors.New("token de WhatsApp Cloud no configurado")
	}
	return out, nil
}

func (a *App) whatsappCloudConfigFor(c ChannelConnection) whatsappCloudConfig {
	var cfg whatsappCloudConfig
	_ = json.Unmarshal([]byte(c.ConfigJSON), &cfg)
	if cfg.PhoneNumberID == "" {
		cfg.PhoneNumberID = c.ExternalAccountID
	}
	return cfg
}

func (a *App) whatsappCloudGraph(ctx context.Context, method, endpoint, token string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var ge struct {
			Error struct {
				Message string `json:"message"`
				Type    string `json:"type"`
				Code    int    `json:"code"`
			} `json:"error"`
		}
		_ = json.Unmarshal(data, &ge)
		if ge.Error.Message != "" {
			return fmt.Errorf("Meta API: %s (code %d)", ge.Error.Message, ge.Error.Code)
		}
		return fmt.Errorf("Meta API HTTP %d", resp.StatusCode)
	}
	if out != nil && len(data) > 0 {
		return json.Unmarshal(data, out)
	}
	return nil
}

func (a *App) inspectWhatsAppCloud(ctx context.Context, wabaID, phoneNumberID, token string) (whatsappCloudConfig, error) {
	if wabaID == "" || phoneNumberID == "" || token == "" {
		return whatsappCloudConfig{}, errors.New("WABA ID, Phone Number ID y token son obligatorios")
	}
	var result struct {
		Data []struct {
			ID                 string `json:"id"`
			DisplayPhoneNumber string `json:"display_phone_number"`
			VerifiedName       string `json:"verified_name"`
			QualityRating      string `json:"quality_rating"`
		} `json:"data"`
	}
	ep := "https://graph.facebook.com/" + a.cfg.MetaGraphVersion + "/" + url.PathEscape(wabaID) + "/phone_numbers?fields=id,display_phone_number,verified_name,quality_rating"
	if err := a.whatsappCloudGraph(ctx, http.MethodGet, ep, token, nil, &result); err != nil {
		return whatsappCloudConfig{}, err
	}
	for _, p := range result.Data {
		if p.ID == phoneNumberID {
			return whatsappCloudConfig{WABAID: wabaID, PhoneNumberID: phoneNumberID, DisplayPhone: p.DisplayPhoneNumber, VerifiedName: p.VerifiedName, QualityRating: p.QualityRating, WebhookURL: a.cfg.BaseURL + "/webhooks/meta/whatsapp", ProviderMode: "official_cloud"}, nil
		}
	}
	return whatsappCloudConfig{}, errors.New("el Phone Number ID no pertenece al WABA indicado")
}

func (a *App) configureWhatsAppCloud(ctx context.Context, c ChannelConnection, wabaID, phoneNumberID, token string, agentID int64) (map[string]any, error) {
	if token == "" {
		old, err := a.whatsappCloudCredentialsFor(c)
		if err != nil {
			return nil, err
		}
		token = old.AccessToken
	}
	if phoneNumberID == "" {
		phoneNumberID = c.ExternalAccountID
	}
	oldCfg := a.whatsappCloudConfigFor(c)
	if wabaID == "" {
		wabaID = oldCfg.WABAID
	}
	cfg, err := a.inspectWhatsAppCloud(ctx, strings.TrimSpace(wabaID), strings.TrimSpace(phoneNumberID), strings.TrimSpace(token))
	if err != nil {
		return nil, err
	}
	sub := "https://graph.facebook.com/" + a.cfg.MetaGraphVersion + "/" + url.PathEscape(cfg.WABAID) + "/subscribed_apps"
	if err := a.whatsappCloudGraph(ctx, http.MethodPost, sub, token, map[string]any{}, nil); err != nil {
		return nil, fmt.Errorf("WABA válido, pero no se pudo suscribir al webhook: %w", err)
	}
	cb, _ := json.Marshal(cfg)
	cr, _ := json.Marshal(whatsappCloudCredentials{AccessToken: token})
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := a.db.Exec(`UPDATE channel_connections SET encrypted_credentials=?,external_account_id=?,config_json=?,status='connected',assigned_agent_id=?,last_connected_at=?,last_error='',updated_at=? WHERE id=? AND tenant_id=?`, encryptLocal(string(cr), a.cfg.ChannelEncryptionKey), cfg.PhoneNumberID, string(cb), agentID, now, now, c.ID, c.TenantID); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "waba_id": cfg.WABAID, "phone_number_id": cfg.PhoneNumberID, "display_phone_number": cfg.DisplayPhone, "verified_name": cfg.VerifiedName, "quality_rating": cfg.QualityRating, "webhook_url": cfg.WebhookURL, "verify_token": a.cfg.WhatsAppVerifyToken}, nil
}

func (a *App) testWhatsAppCloud(ctx context.Context, c ChannelConnection) (map[string]any, error) {
	cr, err := a.whatsappCloudCredentialsFor(c)
	if err != nil {
		return nil, err
	}
	cfg := a.whatsappCloudConfigFor(c)
	live, err := a.inspectWhatsAppCloud(ctx, cfg.WABAID, cfg.PhoneNumberID, cr.AccessToken)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "waba_id": live.WABAID, "phone_number_id": live.PhoneNumberID, "display_phone_number": live.DisplayPhone, "verified_name": live.VerifiedName, "quality_rating": live.QualityRating, "webhook_url": a.cfg.BaseURL + "/webhooks/meta/whatsapp", "verify_token": a.cfg.WhatsAppVerifyToken, "official": true}, nil
}

func (a *App) sendWhatsAppCloudText(ctx context.Context, c ChannelConnection, to, text string) (string, error) {
	cr, err := a.whatsappCloudCredentialsFor(c)
	if err != nil {
		return "", err
	}
	cfg := a.whatsappCloudConfigFor(c)
	if cfg.PhoneNumberID == "" {
		return "", errors.New("Phone Number ID no configurado")
	}
	to = strings.TrimSpace(strings.TrimPrefix(to, "+"))
	var out struct {
		Messages []struct {
			ID string `json:"id"`
		} `json:"messages"`
	}
	ep := "https://graph.facebook.com/" + a.cfg.MetaGraphVersion + "/" + url.PathEscape(cfg.PhoneNumberID) + "/messages"
	body := map[string]any{"messaging_product": "whatsapp", "recipient_type": "individual", "to": to, "type": "text", "text": map[string]any{"preview_url": false, "body": text}}
	if err := a.whatsappCloudGraph(ctx, http.MethodPost, ep, cr.AccessToken, body, &out); err != nil {
		return "", err
	}
	if len(out.Messages) == 0 || out.Messages[0].ID == "" {
		return "", errors.New("Meta no devolvió ID del mensaje")
	}
	return out.Messages[0].ID, nil
}

func validMetaSignature(body []byte, header, secret string) bool {
	if secret == "" {
		return true
	}
	if !strings.HasPrefix(header, "sha256=") {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	got, err := hex.DecodeString(strings.TrimPrefix(header, "sha256="))
	if err != nil {
		return false
	}
	return hmac.Equal(mac.Sum(nil), got)
}

func (a *App) whatsappCloudWebhookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		if r.URL.Query().Get("hub.mode") == "subscribe" && r.URL.Query().Get("hub.verify_token") == a.cfg.WhatsAppVerifyToken {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(r.URL.Query().Get("hub.challenge")))
			return
		}
		http.Error(w, "verification failed", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
	if err != nil {
		http.Error(w, "bad request", 400)
		return
	}
	if !validMetaSignature(body, r.Header.Get("X-Hub-Signature-256"), a.cfg.MetaAppSecret) {
		http.Error(w, "invalid signature", 401)
		return
	}
	var payload struct {
		Object string `json:"object"`
		Entry  []struct {
			Changes []struct {
				Field string `json:"field"`
				Value struct {
					Metadata struct {
						DisplayPhoneNumber string `json:"display_phone_number"`
						PhoneNumberID      string `json:"phone_number_id"`
					} `json:"metadata"`
					Contacts []struct {
						Profile struct {
							Name string `json:"name"`
						} `json:"profile"`
						WAID string `json:"wa_id"`
					} `json:"contacts"`
					Messages []struct {
						From      string `json:"from"`
						ID        string `json:"id"`
						Timestamp string `json:"timestamp"`
						Type      string `json:"type"`
						Text      struct {
							Body string `json:"body"`
						} `json:"text"`
					} `json:"messages"`
					Statuses []struct {
						ID          string `json:"id"`
						Status      string `json:"status"`
						Timestamp   string `json:"timestamp"`
						RecipientID string `json:"recipient_id"`
					} `json:"statuses"`
				} `json:"value"`
			} `json:"changes"`
		} `json:"entry"`
	}
	if json.Unmarshal(body, &payload) != nil {
		http.Error(w, "bad payload", 400)
		return
	}
	for _, entry := range payload.Entry {
		for _, change := range entry.Changes {
			if change.Field != "messages" {
				continue
			}
			phoneID := change.Value.Metadata.PhoneNumberID
			var c ChannelConnection
			err := a.db.QueryRow(`SELECT id,tenant_id,public_id,type,name,status,external_account_id,assigned_agent_id,config_json,encrypted_credentials,last_connected_at,last_disconnected_at,last_message_at,last_error,created_at,updated_at FROM channel_connections WHERE type='whatsapp_cloud' AND external_account_id=? AND status<>'deleted' ORDER BY id LIMIT 1`, phoneID).Scan(&c.ID, &c.TenantID, &c.PublicID, &c.Type, &c.Name, &c.Status, &c.ExternalAccountID, &c.AssignedAgentID, &c.ConfigJSON, &c.EncryptedCredentials, &c.LastConnectedAt, &c.LastDisconnectedAt, &c.LastMessageAt, &c.LastError, &c.CreatedAt, &c.UpdatedAt)
			if err != nil {
				continue
			}
			contactNames := map[string]string{}
			for _, ct := range change.Value.Contacts {
				contactNames[ct.WAID] = ct.Profile.Name
			}
			for _, m := range change.Value.Messages {
				txt := m.Text.Body
				if txt == "" {
					txt = "[" + m.Type + "]"
				}
				ts := time.Now().UTC().Format(time.RFC3339)
				if m.Timestamp != "" {
					if n, e := time.Parse(time.RFC3339, m.Timestamp); e == nil {
						ts = n.UTC().Format(time.RFC3339)
					}
				}
				chat := "wacloud:" + m.From
				_, _ = a.db.Exec(`INSERT OR IGNORE INTO worktic_messages(tenant_id,channel_connection_id,channel,wa_id,chat_jid,sender_jid,direction,message_type,text,status,timestamp) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, c.TenantID, c.ID, "whatsapp_cloud", m.ID, chat, m.From, "in", m.Type, txt, "received", ts)
				_, _ = a.db.Exec(`INSERT INTO worktic_contacts(tenant_id,channel_connection_id,chat_jid,channel,phone,name,unread,updated_at) VALUES(?,?,?,?,?,?,1,?) ON CONFLICT(chat_jid) DO UPDATE SET tenant_id=excluded.tenant_id,channel_connection_id=excluded.channel_connection_id,channel=excluded.channel,phone=excluded.phone,name=CASE WHEN excluded.name<>'' THEN excluded.name ELSE worktic_contacts.name END,unread=worktic_contacts.unread+1,updated_at=excluded.updated_at`, c.TenantID, c.ID, chat, "whatsapp_cloud", m.From, contactNames[m.From], ts)
				_, _ = a.db.Exec(`UPDATE channel_connections SET last_message_at=?,last_error='',updated_at=? WHERE id=?`, ts, ts, c.ID)
				_ = a.syncCRMContactAt(c.TenantID, contactNames[m.From], m.From, "", "whatsapp_cloud", "conversation", chat, ts)
				_ = a.syncOpportunityFromConversation(c.TenantID, chat, "whatsapp_cloud", txt, ts)
				a.ensureInboxConversation(c.TenantID, chat)
				a.emitInboxEvent(c.TenantID, "message.received", map[string]any{"chat": chat, "channel": "whatsapp_cloud", "text": txt, "connection_id": c.ID})
				a.trackWhatsAppMarketingInboundV27(c.TenantID, c.ID, m.From, txt, ts)
			}
			for _, s := range change.Value.Statuses {
				_, _ = a.db.Exec(`UPDATE worktic_messages SET status=? WHERE tenant_id=? AND channel_connection_id=? AND wa_id=?`, s.Status, c.TenantID, c.ID, s.ID)
				statusAt := time.Now().UTC().Format(time.RFC3339)
				if s.Timestamp != "" {
					if sec, e := strconv.ParseInt(s.Timestamp, 10, 64); e == nil && sec > 0 {
						statusAt = time.Unix(sec, 0).UTC().Format(time.RFC3339)
					}
				}
				a.trackWhatsAppMarketingStatusV27(c.TenantID, c.ID, s.ID, s.Status, statusAt)
			}
		}
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("EVENT_RECEIVED"))
}

func (a *App) whatsappTemplatesHandler(w http.ResponseWriter, r *http.Request) {
	tid, _, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, 409)
		return
	}
	id := strings.TrimSpace(r.URL.Query().Get("connection_id"))
	if id == "" {
		writeError(w, errors.New("connection_id obligatorio"), 400)
		return
	}
	var c ChannelConnection
	if a.db.QueryRow(`SELECT id,tenant_id,public_id,type,name,status,external_account_id,assigned_agent_id,config_json,encrypted_credentials,last_connected_at,last_disconnected_at,last_message_at,last_error,created_at,updated_at FROM channel_connections WHERE id=? AND tenant_id=? AND type='whatsapp_cloud'`, id, tid).Scan(&c.ID, &c.TenantID, &c.PublicID, &c.Type, &c.Name, &c.Status, &c.ExternalAccountID, &c.AssignedAgentID, &c.ConfigJSON, &c.EncryptedCredentials, &c.LastConnectedAt, &c.LastDisconnectedAt, &c.LastMessageAt, &c.LastError, &c.CreatedAt, &c.UpdatedAt) != nil {
		writeError(w, errors.New("conexión WhatsApp Cloud no encontrada"), 404)
		return
	}
	cr, e := a.whatsappCloudCredentialsFor(c)
	if e != nil {
		writeError(w, e, 400)
		return
	}
	cfg := a.whatsappCloudConfigFor(c)
	ep := "https://graph.facebook.com/" + a.cfg.MetaGraphVersion + "/" + url.PathEscape(cfg.WABAID) + "/message_templates"
	if r.Method == http.MethodGet {
		var out any
		if e = a.whatsappCloudGraph(r.Context(), http.MethodGet, ep+"?limit=250&fields=id,name,status,category,language,quality_score,components", cr.AccessToken, nil, &out); e != nil {
			writeError(w, e, 502)
			return
		}
		writeJSON(w, out)
		return
	}
	if r.Method == http.MethodPost {
		var q struct {
			Name     string `json:"name"`
			Language string `json:"language"`
			Category string `json:"category"`
			Body     string `json:"body"`
		}
		_ = json.NewDecoder(r.Body).Decode(&q)
		if q.Name == "" || q.Body == "" {
			writeError(w, errors.New("name y body son obligatorios"), 400)
			return
		}
		if q.Language == "" {
			q.Language = "es"
		}
		if q.Category == "" {
			q.Category = "MARKETING"
		}
		payload := map[string]any{"name": q.Name, "language": q.Language, "category": q.Category, "components": []any{map[string]any{"type": "BODY", "text": q.Body}}}
		var out any
		if e = a.whatsappCloudGraph(r.Context(), http.MethodPost, ep, cr.AccessToken, payload, &out); e != nil {
			writeError(w, e, 502)
			return
		}
		writeJSON(w, out)
		return
	}
	http.Error(w, "Método no permitido", 405)
}
