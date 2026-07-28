package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// messengerConversationSyncResponse models the subset of Meta's Conversations
// API used as a safety net when a real-time webhook is delayed or omitted.
type messengerConversationSyncResponse struct {
	Data []struct {
		ID           string `json:"id"`
		UpdatedTime  string `json:"updated_time"`
		Participants struct {
			Data []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"data"`
		} `json:"participants"`
		Messages struct {
			Data []struct {
				ID          string `json:"id"`
				Message     string `json:"message"`
				CreatedTime string `json:"created_time"`
				From        struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"from"`
				To struct {
					Data []struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					} `json:"data"`
				} `json:"to"`
				Attachments struct {
					Data []struct {
						ID       string `json:"id"`
						Name     string `json:"name"`
						MimeType string `json:"mime_type"`
					} `json:"data"`
				} `json:"attachments"`
			} `json:"data"`
		} `json:"messages"`
	} `json:"data"`
}

func parseMessengerGraphTime(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05-0700",
		"2006-01-02T15:04:05.000-0700",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC()
		}
	}
	return time.Time{}
}

func messengerConversationMessageText(message string, attachmentCount int) (string, string) {
	message = strings.TrimSpace(message)
	if message != "" {
		return message, "text"
	}
	if attachmentCount > 0 {
		return "[Adjunto recibido por Messenger]", "attachment"
	}
	return "", ""
}

func (a *App) messengerSyncCutoff(c ChannelConnection, now time.Time) time.Time {
	// On the first synchronization, recover a reasonable recent window without
	// answering months of historical inbox content. Subsequent runs overlap by
	// two minutes to protect against clock drift and eventual consistency.
	cutoff := now.Add(-2 * time.Hour)
	lastAt, _, _ := a.latestMessengerChannelEvent(c, "messenger_conversation_sync")
	if parsed := parseMessengerGraphTime(lastAt); !parsed.IsZero() {
		cutoff = parsed.Add(-2 * time.Minute)
	}
	return cutoff
}

func (a *App) messengerMessageAlreadyStored(c ChannelConnection, messageID string) bool {
	messageID = strings.TrimSpace(messageID)
	if messageID == "" {
		return false
	}
	var count int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM worktic_messages WHERE tenant_id=? AND channel_connection_id=? AND wa_id=?`, c.TenantID, c.ID, messageID).Scan(&count)
	return count > 0
}

func (a *App) storeMessengerSyncedMessage(c ChannelConnection, messageID, psid, contactName, text, messageType string, createdAt time.Time) (bool, error) {
	messageID = strings.TrimSpace(messageID)
	psid = strings.TrimSpace(psid)
	text = strings.TrimSpace(text)
	if psid == "" || text == "" {
		return false, errors.New("mensaje sincronizado sin remitente o contenido")
	}
	if messageID == "" {
		messageID = "messenger-sync-" + randomToken(12)
	}
	if a.messengerMessageAlreadyStored(c, messageID) {
		return false, nil
	}
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	if strings.TrimSpace(contactName) == "" {
		contactName = "Usuario Messenger"
	}
	if messageType == "" {
		messageType = "text"
	}
	chat := "messenger:" + psid
	timestamp := createdAt.UTC().Format(time.RFC3339Nano)

	insertResult, err := a.db.Exec(`INSERT OR IGNORE INTO worktic_messages(tenant_id,channel_connection_id,channel,wa_id,chat_jid,sender_jid,direction,message_type,text,status,timestamp) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, c.TenantID, c.ID, "messenger", messageID, chat, psid, "in", messageType, text, "received", timestamp)
	if err != nil {
		return false, err
	}
	if affected, _ := insertResult.RowsAffected(); affected == 0 {
		return false, nil
	}
	_, _ = a.db.Exec(`INSERT INTO worktic_contacts(tenant_id,channel_connection_id,chat_jid,channel,phone,name,unread,updated_at) VALUES(?,?,?,?,?,?,1,?) ON CONFLICT(chat_jid) DO UPDATE SET tenant_id=excluded.tenant_id,channel_connection_id=excluded.channel_connection_id,channel=excluded.channel,phone=excluded.phone,name=CASE WHEN worktic_contacts.name='' OR worktic_contacts.name='Usuario Messenger' THEN excluded.name ELSE worktic_contacts.name END,unread=worktic_contacts.unread+1,updated_at=excluded.updated_at`, c.TenantID, c.ID, chat, "messenger", psid, contactName, timestamp)
	_ = a.syncCRMContactAt(c.TenantID, contactName, "", "", "messenger", "conversation", chat, timestamp)
	_ = a.syncOpportunityFromConversation(c.TenantID, chat, "messenger", text, timestamp)
	_, _ = a.db.Exec(`UPDATE channel_connections SET last_message_at=?,last_error='',updated_at=? WHERE id=? AND tenant_id=?`, timestamp, time.Now().UTC().Format(time.RFC3339Nano), c.ID, c.TenantID)
	a.recordMessengerChannelEvent(c, "messenger_message_received", "received_via_conversations_api", map[string]any{
		"message_id":  messageID,
		"sender_id":   psid,
		"source_path": "conversations_api",
		"created_at":  timestamp,
	})
	return true, nil
}

func (a *App) fetchMessengerConversations(c ChannelConnection, cr messengerCredentials, pageID string) (messengerConversationSyncResponse, string, error) {
	fields := "id,updated_time,participants,messages.limit(12){id,message,from,to,created_time,attachments}"
	values := url.Values{
		"platform": {"messenger"},
		"fields":   {fields},
		"limit":    {"25"},
	}
	var out messengerConversationSyncResponse
	path := "/" + url.PathEscape(pageID) + "/conversations"
	if err := a.messengerGraph(http.MethodGet, path, cr.PageToken, values, &out); err == nil {
		return out, "page_conversations_platform_messenger", nil
	} else {
		// Some Graph versions infer Messenger from the Page and reject the
		// platform parameter. Retry without it before surfacing the error.
		firstErr := err
		values.Del("platform")
		var fallback messengerConversationSyncResponse
		if retryErr := a.messengerGraph(http.MethodGet, path, cr.PageToken, values, &fallback); retryErr == nil {
			return fallback, "page_conversations", nil
		}
		return messengerConversationSyncResponse{}, "page_conversations", fmt.Errorf("%v", firstErr)
	}
}

func (a *App) syncMessengerConnectionFromAPI(c ChannelConnection, reason string) (map[string]any, error) {
	cr, err := a.messengerCredentialsFor(c)
	if err != nil || strings.TrimSpace(cr.PageToken) == "" {
		if err == nil {
			err = errors.New("Page Access Token no disponible")
		}
		return nil, err
	}
	pageID := strings.TrimSpace(c.ExternalAccountID)
	if pageID == "" {
		var cfg map[string]any
		_ = json.Unmarshal([]byte(c.ConfigJSON), &cfg)
		pageID = messengerAnyString(cfg["page_id"])
	}
	if pageID == "" {
		return nil, errors.New("Page ID no disponible")
	}

	now := time.Now().UTC()
	cutoff := a.messengerSyncCutoff(c, now)
	response, method, err := a.fetchMessengerConversations(c, cr, pageID)
	if err != nil {
		detail := map[string]any{"reason": reason, "method": method, "error": err.Error(), "cutoff": cutoff.Format(time.RFC3339Nano)}
		a.recordMessengerChannelEvent(c, "messenger_conversation_sync", "error", detail)
		_, _ = a.db.Exec(`UPDATE channel_connections SET last_error=?,updated_at=? WHERE id=? AND tenant_id=?`, "Sincronización Messenger: "+err.Error(), now.Format(time.RFC3339Nano), c.ID, c.TenantID)
		return detail, err
	}

	conversations := len(response.Data)
	seenMessages := 0
	imported := 0
	duplicates := 0
	ignoredOld := 0
	replied := 0
	for _, conversation := range response.Data {
		updated := parseMessengerGraphTime(conversation.UpdatedTime)
		if !updated.IsZero() && updated.Before(cutoff) {
			continue
		}

		// Meta normally returns newest messages first. Iterate backwards so the
		// local conversation preserves chronological order.
		var newestReplyPSID, newestReplyChat, newestReplyText string
		var newestReplyAt time.Time
		for i := len(conversation.Messages.Data) - 1; i >= 0; i-- {
			message := conversation.Messages.Data[i]
			created := parseMessengerGraphTime(message.CreatedTime)
			if !created.IsZero() && created.Before(cutoff) {
				ignoredOld++
				continue
			}
			seenMessages++
			fromID := strings.TrimSpace(message.From.ID)
			if fromID == "" || fromID == pageID {
				continue
			}
			text, messageType := messengerConversationMessageText(message.Message, len(message.Attachments.Data))
			if text == "" {
				continue
			}
			stored, storeErr := a.storeMessengerSyncedMessage(c, message.ID, fromID, message.From.Name, text, messageType, created)
			if storeErr != nil {
				a.recordMessengerChannelEvent(c, "messenger_processing_error", "conversation_sync_store", map[string]any{"error": storeErr.Error(), "message_id": message.ID})
				continue
			}
			if !stored {
				duplicates++
				continue
			}
			imported++
			if created.IsZero() {
				created = now
			}
			// Reply only to fresh messages. Older history is imported without
			// surprising the customer with a delayed automated response.
			if created.After(now.Add(-30*time.Minute)) && created.After(newestReplyAt) {
				newestReplyAt = created
				newestReplyPSID = fromID
				newestReplyChat = "messenger:" + fromID
				newestReplyText = text
			}
		}
		if newestReplyPSID != "" {
			replied++
			go a.maybeMessengerAIReply(c, newestReplyPSID, newestReplyChat, newestReplyText)
		}
	}

	status := "ok"
	if conversations == 0 {
		status = "ok_empty"
	}
	detail := map[string]any{
		"reason":        reason,
		"method":        method,
		"page_id":       pageID,
		"conversations": conversations,
		"messages_seen": seenMessages,
		"imported":      imported,
		"duplicates":    duplicates,
		"ignored_old":   ignoredOld,
		"ai_replies":    replied,
		"cutoff":        cutoff.Format(time.RFC3339Nano),
	}
	a.recordMessengerChannelEvent(c, "messenger_conversation_sync", status, detail)
	if imported > 0 {
		_, _ = a.db.Exec(`UPDATE channel_connections SET last_error='',updated_at=? WHERE id=? AND tenant_id=?`, now.Format(time.RFC3339Nano), c.ID, c.TenantID)
	}
	return detail, nil
}

func (a *App) messengerSyncDue(c ChannelConnection, now time.Time) bool {
	activeSeconds := envInt("MESSENGER_SYNC_ACTIVE_SECONDS", 10)
	idleSeconds := envInt("MESSENGER_SYNC_IDLE_SECONDS", 60)
	if activeSeconds < 5 {
		activeSeconds = 5
	}
	if activeSeconds > 60 {
		activeSeconds = 60
	}
	if idleSeconds < activeSeconds {
		idleSeconds = activeSeconds
	}
	if idleSeconds > 300 {
		idleSeconds = 300
	}
	lastSyncAt, lastSyncStatus, _ := a.latestMessengerChannelEvent(c, "messenger_conversation_sync")
	lastSync := parseMessengerGraphTime(lastSyncAt)
	if lastSync.IsZero() {
		return true
	}
	interval := time.Duration(idleSeconds) * time.Second
	lastActivity := parseMessengerGraphTime(c.LastMessageAt)
	lastWebhookAt, _, _ := a.latestMessengerChannelEvent(c, "messenger_webhook_http")
	webhookActivity := parseMessengerGraphTime(lastWebhookAt)
	if webhookActivity.After(lastActivity) {
		lastActivity = webhookActivity
	}
	if lastActivity.IsZero() {
		lastActivity = parseMessengerGraphTime(c.LastConnectedAt)
	}
	if (!lastActivity.IsZero() && now.Sub(lastActivity) <= 15*time.Minute) || lastSyncStatus == "error" {
		interval = time.Duration(activeSeconds) * time.Second
	}
	return now.Sub(lastSync) >= interval
}

func (a *App) syncAllMessengerConnections(force bool) {
	rows, err := a.db.Query(`SELECT id,tenant_id,public_id,type,name,status,external_account_id,assigned_agent_id,config_json,encrypted_credentials,last_connected_at,last_disconnected_at,last_message_at,last_error,created_at,updated_at FROM channel_connections WHERE type='messenger' AND status='connected' ORDER BY id`)
	if err != nil {
		log.Printf("[messenger sync] list connections error=%v", err)
		return
	}
	defer rows.Close()
	connections := make([]ChannelConnection, 0)
	for rows.Next() {
		var c ChannelConnection
		if err := rows.Scan(&c.ID, &c.TenantID, &c.PublicID, &c.Type, &c.Name, &c.Status, &c.ExternalAccountID, &c.AssignedAgentID, &c.ConfigJSON, &c.EncryptedCredentials, &c.LastConnectedAt, &c.LastDisconnectedAt, &c.LastMessageAt, &c.LastError, &c.CreatedAt, &c.UpdatedAt); err == nil {
			connections = append(connections, c)
		}
	}
	now := time.Now().UTC()
	for _, c := range connections {
		if !force && !a.messengerSyncDue(c, now) {
			continue
		}
		if _, err := a.syncMessengerConnectionFromAPI(c, "background"); err != nil {
			log.Printf("[messenger sync] connection=%d tenant=%d error=%v", c.ID, c.TenantID, err)
		}
	}
}

func (a *App) runMessengerConversationSync() {
	// A five-second scheduler lets active conversations synchronize every ten
	// seconds while idle pages back off to one minute by default.
	time.Sleep(8 * time.Second)
	a.syncAllMessengerConnections(true)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		a.syncAllMessengerConnections(false)
	}
}
