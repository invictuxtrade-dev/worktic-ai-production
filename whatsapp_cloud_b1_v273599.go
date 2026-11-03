package main

// Worktic B1.1 (V27.35.99): separate Meta Cloud conversations per physical
// connection; keep an already-owned legacy thread for continuity.
// Cloud delivery receipts are distinct from automation's event enqueue receipts.
import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

func waCloudValidPeerV273599(v string) bool {
	if len(v) < 7 || len(v) > 20 {
		return false
	}
	for _, r := range v {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func waCloudScopedChatV273599(connectionID int64, peer string) string {
	return fmt.Sprintf("wacloud:%d:%s", connectionID, peer)
}

// Legacy: wacloud:<recipient>; scoped: wacloud:<connectionID>:<recipient>.
// No guessing which of several connected numbers should send a reply.
func waCloudParseChatV273599(chat string) (connectionID int64, peer string, scoped bool, err error) {
	if !strings.HasPrefix(chat, "wacloud:") {
		return 0, "", false, errors.New("no es un chat de WhatsApp Cloud")
	}
	parts := strings.Split(strings.TrimPrefix(chat, "wacloud:"), ":")
	switch len(parts) {
	case 1:
		peer = parts[0]
	case 2:
		connectionID, err = strconv.ParseInt(parts[0], 10, 64)
		if err != nil || connectionID <= 0 {
			return 0, "", false, errors.New("conexión de WhatsApp Cloud inválida")
		}
		peer = parts[1]
		scoped = true
	default:
		return 0, "", false, errors.New("identificador de conversación inválido")
	}
	if !waCloudValidPeerV273599(peer) {
		return 0, "", false, errors.New("número de WhatsApp inválido")
	}
	return connectionID, peer, scoped, nil
}

func waCloudToForConnectionV273599(chat string, connectionID int64) (string, error) {
	id, peer, scoped, err := waCloudParseChatV273599(chat)
	if err != nil {
		return "", err
	}
	if scoped && id != connectionID {
		return "", errors.New("chat pertenece a otra conexión")
	}
	return peer, nil
}

func waCloudAutomationIdentityV273599(state map[string]any, connectionID int64, to string) (chat string, messageID string, err error) {
	chat = strings.TrimSpace(payloadString(state, "chat"))
	messageID = strings.TrimSpace(payloadString(state, "message_id"))
	if messageID == "" {
		return "", "", errors.New("falta ID del mensaje entrante verificado")
	}
	id, peer, scoped, e := waCloudParseChatV273599(chat)
	if e != nil {
		return "", "", e
	}
	if scoped && id != connectionID {
		return "", "", errors.New("conexión no corresponde al chat entrante")
	}
	if peer != strings.TrimPrefix(strings.TrimSpace(to), "+") {
		return "", "", errors.New("destinatario diferente al remitente del mensaje entrante")
	}
	c := strings.TrimSpace(payloadString(state, "connection_id"))
	if c != "" {
		claimed, e := strconv.ParseInt(c, 10, 64)
		if e != nil || claimed != connectionID {
			return "", "", errors.New("conexión del evento no coincide")
		}
	}
	return chat, messageID, nil
}

func initWACloudB1V273599Schema(db *DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS wa_cloud_receipts_v273599 (
        tenant_id BIGINT NOT NULL,connection_id BIGINT NOT NULL,
        meta_message_id TEXT NOT NULL,chat_jid TEXT NOT NULL,
        crm_done INTEGER NOT NULL DEFAULT 0,received_at TEXT NOT NULL,
        PRIMARY KEY(tenant_id,connection_id,meta_message_id)
    );`)
	return err
}

func (a *App) waCloudChatForInboundV273599(c ChannelConnection, peer string) string {
	legacy := "wacloud:" + peer
	var tenant, connection int64
	err := a.db.QueryRow(`SELECT tenant_id,channel_connection_id FROM worktic_contacts WHERE chat_jid=? AND channel='whatsapp_cloud'`, legacy).Scan(&tenant, &connection)
	if err == nil && tenant == c.TenantID && connection == c.ID {
		return legacy
	}
	return waCloudScopedChatV273599(c.ID, peer)
}

// New chats use a globally unique connection ID. For legacy chats, use the
// connection recorded on the contact: NEVER the first connection of a tenant.
func (a *App) waCloudConnectionForChatV273599(tid int64, chat string) (ChannelConnection, string, error) {
	var c ChannelConnection
	id, to, scoped, err := waCloudParseChatV273599(chat)
	if err != nil {
		return c, "", err
	}
	if !scoped {
		err = a.db.QueryRow(`SELECT channel_connection_id FROM worktic_contacts WHERE tenant_id=? AND chat_jid=? AND channel='whatsapp_cloud'`, tid, chat).Scan(&id)
		if err != nil || id <= 0 {
			return c, "", errors.New("chat antiguo sin conexión empresarial verificable")
		}
	}
	err = a.db.QueryRow(`SELECT id,tenant_id,public_id,type,name,status,external_account_id,assigned_agent_id,config_json,encrypted_credentials,last_connected_at,last_disconnected_at,last_message_at,last_error,created_at,updated_at
        FROM channel_connections WHERE tenant_id=? AND id=? AND type='whatsapp_cloud' AND status='connected'`, tid, id).Scan(
		&c.ID, &c.TenantID, &c.PublicID, &c.Type, &c.Name, &c.Status, &c.ExternalAccountID, &c.AssignedAgentID, &c.ConfigJSON, &c.EncryptedCredentials, &c.LastConnectedAt, &c.LastDisconnectedAt, &c.LastMessageAt, &c.LastError, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return c, "", errors.New("conexión WhatsApp Cloud de esta conversación no disponible")
	}
	return c, to, nil
}

// A single CRM contact can receive conversations over multiple official numbers.
// Its external_id may reference the first connection; resolve by tenant+phone
// when the new scoped chat has not yet been attached to that CRM contact.
func (a *App) waCloudOpportunityV273599(tenant int64, chat, peer, text, activityAt string) error {
	matched, interest, stage, score := detectCommercialOpportunity(text)
	if !matched {
		return nil
	}
	contactID, err := a.contactIDForOpportunity(tenant, 0, "", "", chat)
	if err == sql.ErrNoRows {
		contactID, err = a.contactIDForOpportunity(tenant, 0, normalizeCRMPhone(peer), "", "")
	}
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	_, _, err = a.syncAutomaticOpportunity(tenant, contactID, "conversation", "chat:"+chat, "whatsapp_cloud", interest, stage, score, activityAt)
	return err
}

func (a *App) waCloudStoreInboundV273599(c ChannelConnection, messageID, chat, from, kind, text, name, ts string) (bool, error) {
	tx, err := a.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	r, err := tx.Exec(`INSERT OR IGNORE INTO wa_cloud_receipts_v273599(tenant_id,connection_id,meta_message_id,chat_jid,received_at) VALUES(?,?,?,?,?)`, c.TenantID, c.ID, messageID, chat, ts)
	if err != nil {
		return false, err
	}
	affected, err := r.RowsAffected()
	if err != nil {
		return false, err
	}
	if affected == 0 {
		return false, nil
	} // no writes for duplicate webhook
	if _, err = tx.Exec(`INSERT INTO worktic_messages(tenant_id,channel_connection_id,channel,wa_id,chat_jid,sender_jid,direction,message_type,text,status,timestamp) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, c.TenantID, c.ID, "whatsapp_cloud", messageID, chat, from, "in", kind, text, "received", ts); err != nil {
		return false, err
	}
	if _, err = tx.Exec(`INSERT INTO worktic_contacts(tenant_id,channel_connection_id,chat_jid,channel,phone,name,unread,updated_at) VALUES(?,?,?,?,?,?,1,?)
       ON CONFLICT(chat_jid) DO UPDATE SET name=CASE WHEN excluded.name<>'' THEN excluded.name ELSE worktic_contacts.name END,
       unread=worktic_contacts.unread+1,updated_at=excluded.updated_at
       WHERE worktic_contacts.tenant_id=excluded.tenant_id AND worktic_contacts.channel_connection_id=excluded.channel_connection_id`, c.TenantID, c.ID, chat, "whatsapp_cloud", from, name, ts); err != nil {
		return false, err
	}
	// If a conflicting legacy key belonged to another tenant, fail rather than mutate it.
	var tenant, connection int64
	if err = tx.QueryRow(`SELECT tenant_id,channel_connection_id FROM worktic_contacts WHERE chat_jid=?`, chat).Scan(&tenant, &connection); err != nil {
		return false, err
	}
	if tenant != c.TenantID || connection != c.ID {
		return false, errors.New("colisión de identidad de WhatsApp Cloud")
	}
	if _, err = tx.Exec(`UPDATE channel_connections SET last_message_at=?,last_error='',updated_at=? WHERE id=? AND tenant_id=?`, ts, ts, c.ID, c.TenantID); err != nil {
		return false, err
	}
	if err = tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}
