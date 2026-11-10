package main

// B10.4A: Opt-in Instagram Login messaging, tenant-isolated inbound text DMs
// and manually initiated replies. AI is ALWAYS disabled in this phase.
// Never write webhook bodies, OAuth credentials or sender IDs to logs.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const instagramB104MessagesPermission = "instagram_business_manage_messages"

func instagramB104ScopeEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("WORKTIC_IG_B104_SCOPE_ENABLED")), "true")
}
func instagramB104InboxEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("WORKTIC_IG_B104_INBOX_ENABLED")), "true")
}
func instagramB104ManualEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("WORKTIC_IG_B104_MANUAL_SEND_ENABLED")), "true")
}

func instagramB104HasScope(scopes, scope string) bool {
	for _, s := range strings.Split(scopes, ",") {
		if strings.TrimSpace(s) == scope {
			return true
		}
	}
	return false
}

// Meta's short-token response uses a string of comma-separated permissions or
// a string array. An omitted/unknown shape MUST NOT be treated as a grant.
func instagramB104Granted(raw json.RawMessage, scope string) bool {
	if len(raw) == 0 {
		return false
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return instagramB104HasScope(s, scope)
	}
	var arr []string
	if json.Unmarshal(raw, &arr) != nil {
		return false
	}
	for _, grant := range arr {
		if grant == scope {
			return true
		}
	}
	return false
}

func instagramB104Chat(tid, socialConnectionID int64, sender string) string {
	return "instagram:" + strconv.FormatInt(tid, 10) + ":" + strconv.FormatInt(socialConnectionID, 10) + ":" + sender
}
func instagramB104ValidSender(s string) bool {
	if len(s) < 6 || len(s) > 64 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// Minimal supported Instagram webhook envelope; ignore other event types.
type instagramB104Envelope struct {
	Object string `json:"object"`
	Entry  []struct {
		ID        string `json:"id"`
		Messaging []struct {
			Sender struct {
				ID string `json:"id"`
			} `json:"sender"`
			Recipient struct {
				ID string `json:"id"`
			} `json:"recipient"`
			Timestamp int64 `json:"timestamp"`
			Message   *struct {
				MID    string `json:"mid"`
				Text   string `json:"text"`
				IsEcho bool   `json:"is_echo"`
			} `json:"message"`
		} `json:"messaging"`
	} `json:"entry"`
}

// Safe extractor: never reinterpret an echo or a non-text event as an inbound DM.
type instagramB104DM struct {
	AccountID, SenderID, MID, Text, Chat string
	At                                   time.Time
}

func instagramB104Extract(body []byte) ([]instagramB104DM, error) {
	var env instagramB104Envelope
	if err := json.Unmarshal(body, &env); err != nil || env.Object != "instagram" {
		return nil, errors.New("IG_B104_INVALID_EVENT")
	}
	out := make([]instagramB104DM, 0)
	for _, e := range env.Entry {
		for _, m := range e.Messaging {
			if m.Message == nil || m.Message.IsEcho || (m.Message.Text == "" && m.Message.MID == "") {
				continue
			}
			if strings.TrimSpace(m.Message.Text) == "" {
				continue
			} // attachments are B10.4B
			if len(m.Message.Text) > 4000 || len(m.Message.MID) > 1024 || len(m.Message.MID) == 0 || !instagramB104ValidSender(e.ID) || !instagramB104ValidSender(m.Sender.ID) || m.Recipient.ID != e.ID || m.Sender.ID == e.ID {
				return nil, errors.New("IG_B104_BAD_MESSAGE_IDENTITY")
			}
			at := time.Now().UTC()
			if m.Timestamp > 946684800000 && m.Timestamp < 4102444800000 {
				at = time.UnixMilli(m.Timestamp).UTC()
			}
			out = append(out, instagramB104DM{AccountID: e.ID, SenderID: m.Sender.ID, MID: m.Message.MID, Text: m.Message.Text, At: at})
		}
	}
	return out, nil
}

func instagramB104Hash(account, mid string) string {
	hash := sha256.Sum256([]byte("b104:instagram:" + account + ":" + mid))
	return hex.EncodeToString(hash[:])
}

func (a *App) instagramB104Receive(w http.ResponseWriter, r *http.Request, body []byte) {
	events, err := instagramB104Extract(body)
	if err != nil {
		http.Error(w, "invalid instagram message", http.StatusBadRequest)
		return
	}
	if _, err = a.db.Exec(`CREATE TABLE IF NOT EXISTS instagram_b104_receipts (
      tenant_id BIGINT NOT NULL, social_connection_id BIGINT NOT NULL,
      event_hash TEXT NOT NULL, received_at TEXT NOT NULL,
      PRIMARY KEY(tenant_id,social_connection_id,event_hash))`); err != nil {
		http.Error(w, "receipt store unavailable", http.StatusServiceUnavailable)
		return
	}
	accepted, duplicates := 0, 0
	for _, msg := range events {
		var tid, sid int64
		var count int
		var scopes, cfgJSON string
		err = a.db.QueryRow(`SELECT COUNT(*),COALESCE(MIN(tenant_id),0),COALESCE(MIN(id),0),COALESCE(MIN(scopes),''),COALESCE(MIN(config_json),'') FROM social_connections WHERE platform='instagram' AND provider_mode='official' AND status='connected' AND external_account_id=?`, msg.AccountID).Scan(&count, &tid, &sid, &scopes, &cfgJSON)
		if err != nil || count != 1 || tid <= 0 || sid <= 0 {
			http.Error(w, "instagram ownership unresolved", http.StatusConflict)
			return
		}
		var cfg socialProviderConfig
		if json.Unmarshal([]byte(cfgJSON), &cfg) != nil || cfg.InstagramAuthMethod != "instagram_login" || !instagramB104HasScope(scopes, instagramB104MessagesPermission) {
			http.Error(w, "instagram messaging not authorized", http.StatusConflict)
			return
		}
		chat := instagramB104Chat(tid, sid, msg.SenderID)
		stamp := msg.At.Format(time.RFC3339Nano)
		hash := instagramB104Hash(msg.AccountID, msg.MID)
		tx, e := a.db.Begin()
		if e != nil {
			http.Error(w, "message storage unavailable", http.StatusServiceUnavailable)
			return
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		result, e := tx.Exec(`INSERT INTO instagram_b104_receipts(tenant_id,social_connection_id,event_hash,received_at) VALUES(?,?,?,?) ON CONFLICT(tenant_id,social_connection_id,event_hash) DO NOTHING`, tid, sid, hash, now)
		if e != nil {
			_ = tx.Rollback()
			http.Error(w, "receipt storage unavailable", http.StatusServiceUnavailable)
			return
		}
		affected, e := result.RowsAffected()
		if e != nil {
			_ = tx.Rollback()
			http.Error(w, "receipt status unavailable", 503)
			return
		}
		if affected == 0 {
			_ = tx.Rollback()
			duplicates++
			continue
		}
		_, e = tx.Exec(`INSERT INTO worktic_messages(tenant_id,channel_connection_id,channel,wa_id,chat_jid,sender_jid,direction,message_type,text,status,timestamp) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, tid, sid, "instagram", msg.MID, chat, msg.SenderID, "in", "text", msg.Text, "received", stamp)
		if e != nil {
			_ = tx.Rollback()
			http.Error(w, "message storage unavailable", 503)
			return
		}
		result, e = tx.Exec(`INSERT INTO worktic_contacts(tenant_id,channel_connection_id,chat_jid,channel,phone,name,unread,updated_at) VALUES(?,?,?,?,?,?,1,?) ON CONFLICT(chat_jid) DO UPDATE SET unread=worktic_contacts.unread+1,updated_at=excluded.updated_at WHERE worktic_contacts.tenant_id=excluded.tenant_id AND worktic_contacts.channel_connection_id=excluded.channel_connection_id AND worktic_contacts.channel='instagram'`, tid, sid, chat, "instagram", "", "Usuario Instagram", stamp)
		if e != nil {
			_ = tx.Rollback()
			http.Error(w, "contact storage unavailable", 503)
			return
		}
		affected, e = result.RowsAffected()
		if e != nil || affected != 1 {
			_ = tx.Rollback()
			http.Error(w, "contact ownership mismatch", http.StatusConflict)
			return
		}
		if e = tx.Commit(); e != nil {
			http.Error(w, "message commit unavailable", 503)
			return
		}
		accepted++
		// CRM and Inbox are auxiliary; they must never cause duplicate storage on webhook retry.
		a.ensureInboxConversation(tid, chat)
		if crmErr := a.syncCRMContactAt(tid, "Usuario Instagram", "", "", "instagram", "conversation", chat, stamp); crmErr != nil && strings.EqualFold(os.Getenv("WORKTIC_IG_WEBHOOK_DIAG"), "true") {
			log.Print("B104A_IG_CRM_SYNC_FAILED")
		}
		a.emitInboxEvent(tid, "message.received", map[string]any{"chat": chat, "channel": "instagram", "connection_id": sid, "text": msg.Text})
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("WORKTIC_IG_WEBHOOK_DIAG")), "true") {
		log.Printf("B104A_IG_RECEIPT accepted=%d duplicates=%d ignored=%d", accepted, duplicates, len(events)-accepted-duplicates)
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "EVENT_RECEIVED")
}

// Opt-in manual send; never fall back to a legacy/global WhatsApp connection.
func (a *App) instagramB104SendText(ctx context.Context, tid int64, chat, text string) (string, error) {
	if !instagramB104ManualEnabled() || !instagramB104InboxEnabled() {
		return "", errors.New("envio manual Instagram no habilitado")
	}
	if !strings.HasPrefix(chat, "instagram:") || tid <= 0 || strings.TrimSpace(text) == "" || len([]rune(text)) > 1000 {
		return "", errors.New("mensaje Instagram invalido")
	}
	var sid int64
	var sender, enc, account, scopes, cfgJSON, expiry string
	err := a.db.QueryRow(`SELECT c.channel_connection_id,c.phone,s.encrypted_credentials,s.external_account_id,s.scopes,s.config_json,s.token_expires_at FROM worktic_contacts c JOIN social_connections s ON s.id=c.channel_connection_id AND s.tenant_id=c.tenant_id WHERE c.tenant_id=? AND c.chat_jid=? AND c.channel='instagram' AND s.platform='instagram' AND s.status='connected' AND s.provider_mode='official'`, tid, chat).Scan(&sid, &sender, &enc, &account, &scopes, &cfgJSON, &expiry)
	if err != nil {
		return "", errors.New("conversacion Instagram no autorizada")
	}
	// The contact's phone is intentionally blank. Derive the sender from the namespaced chat ID.
	prefix := "instagram:" + strconv.FormatInt(tid, 10) + ":" + strconv.FormatInt(sid, 10) + ":"
	if !strings.HasPrefix(chat, prefix) {
		return "", errors.New("identidad de conversacion invalida")
	}
	sender = strings.TrimPrefix(chat, prefix)
	if !instagramB104ValidSender(sender) || !instagramB104ValidSender(account) || !instagramB104HasScope(scopes, instagramB104MessagesPermission) {
		return "", errors.New("Instagram sin permiso de mensajes")
	}
	var cfg socialProviderConfig
	if json.Unmarshal([]byte(cfgJSON), &cfg) != nil || cfg.InstagramAuthMethod != "instagram_login" {
		return "", errors.New("modalidad Instagram incompatible")
	}
	if expiry != "" {
		if exp, e := time.Parse(time.RFC3339, expiry); e != nil || time.Now().UTC().After(exp) {
			return "", errors.New("token Instagram vencido o no verificable")
		}
	}
	if a.cfg.ChannelEncryptionKey == "" {
		return "", errors.New("cifrado no configurado")
	}
	var tok socialToken
	if json.Unmarshal([]byte(decryptLocal(enc, a.cfg.ChannelEncryptionKey)), &tok) != nil || tok.AccessToken == "" {
		return "", errors.New("credencial Instagram no disponible")
	}
	version := strings.TrimSpace(a.cfg.MetaGraphVersion)
	if !validB103BGraphVersion(version) {
		return "", errors.New("version Graph no valida")
	}
	payload, _ := json.Marshal(map[string]any{"recipient": map[string]string{"id": sender}, "message": map[string]string{"text": text}})
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, "https://graph.instagram.com/"+version+"/"+url.PathEscape(account)+"/messages", bytes.NewReader(payload))
	if e != nil {
		return "", errors.New("solicitud de envio invalida")
	}
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, e := http.DefaultClient.Do(req)
	if e != nil {
		return "", errors.New("Instagram no respondio al envio")
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("Instagram rechazo envio (HTTP %d)", resp.StatusCode)
	}
	var out struct {
		MessageID   string `json:"message_id"`
		RecipientID string `json:"recipient_id"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 65536)).Decode(&out) != nil || out.MessageID == "" || (out.RecipientID != "" && out.RecipientID != sender) {
		return "", errors.New("confirmacion de mensaje Instagram invalida")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, e = a.db.Exec(`INSERT INTO worktic_messages(tenant_id,channel_connection_id,channel,wa_id,chat_jid,sender_jid,direction,message_type,text,status,timestamp) VALUES(?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT DO NOTHING`, tid, sid, "instagram", out.MessageID, chat, account, "out", "text", text, "sent", now); e != nil {
		return "", errors.New("Instagram envio pero no se pudo registrar")
	}
	return out.MessageID, nil
}

func (a *App) instagramB104Readiness(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "method not allowed", 405)
		return
	}
	tid, user, err := a.tenantFor(r)
	if err != nil {
		http.Error(w, "sesion requerida", 401)
		return
	}
	if !socialPermissionsFor(user).ManageConnections {
		http.Error(w, "acceso denegado", 403)
		return
	}
	id, e := strconv.ParseInt(r.URL.Query().Get("connection_id"), 10, 64)
	if e != nil || id <= 0 {
		http.Error(w, "connection_id invalido", 400)
		return
	}
	var scopes, mode, expiry string
	err = a.db.QueryRow(`SELECT scopes,config_json,token_expires_at FROM social_connections WHERE id=? AND tenant_id=? AND platform='instagram' AND provider_mode='official' AND status='connected'`, id, tid).Scan(&scopes, &mode, &expiry)
	if err == sql.ErrNoRows {
		http.Error(w, "cuenta no encontrada", 404)
		return
	}
	if err != nil {
		http.Error(w, "consulta no disponible", 503)
		return
	}
	var cfg socialProviderConfig
	_ = json.Unmarshal([]byte(mode), &cfg)
	writeJSON(w, map[string]any{"ok": true, "instagram_login": cfg.InstagramAuthMethod == "instagram_login", "basic_scope": instagramB104HasScope(scopes, "instagram_business_basic"), "messages_scope": instagramB104HasScope(scopes, instagramB104MessagesPermission), "scope_request_enabled": instagramB104ScopeEnabled(), "inbox_enabled": instagramB104InboxEnabled(), "manual_send_enabled": instagramB104ManualEnabled(), "ai_enabled": false, "token_expires_at": expiry})
}
