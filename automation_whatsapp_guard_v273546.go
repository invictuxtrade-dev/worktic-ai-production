package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (a *App) authorizeAutomationWhatsAppV273546(tid, connectionID int64, to string, state map[string]any) error {
	// Default off, including on freshly installed deployments. This protects
	// existing workflows until opt-outs and consent are verified end-to-end.
	if !strings.EqualFold(strings.TrimSpace(env("WORKTIC_AUTOMATION_WA_FREEFORM_ENABLED", "false")), "true") {
		return errors.New("envío WhatsApp automático deshabilitado por seguridad; no hay envío")
	}
	chat, messageID, err := waCloudAutomationIdentityV273599(state, connectionID, to)
	if err != nil {
		return err
	}

	// Check actual persisted inbound evidence, not a payload field claiming opt-in.
	var inboundText, inboundTimestamp string
	err = a.db.QueryRow(`SELECT text,timestamp FROM worktic_messages
        WHERE tenant_id=? AND channel_connection_id=? AND channel='whatsapp_cloud'
          AND direction='in' AND chat_jid=? AND wa_id=?
        ORDER BY id DESC LIMIT 1`, tid, connectionID, chat, messageID).Scan(&inboundText, &inboundTimestamp)
	if err != nil {
		return errors.New("no se encontró un mensaje entrante verificado para ese cliente y conexión")
	}
	if err := automationWAWithinWindowV273546(inboundTimestamp, time.Now().UTC()); err != nil {
		return err
	}
	if automationWAIsOptOutV273546(inboundText) {
		return errors.New("el cliente solicitó no recibir mensajes")
	}
	var optouts int
	err = a.db.QueryRow(`SELECT COUNT(*) FROM worktic_messages WHERE tenant_id=? AND channel_connection_id=? AND channel='whatsapp_cloud' AND chat_jid=? AND direction='in' AND lower(trim(text)) IN ('stop','baja','cancelar','unsubscribe','no más','no mas')`, tid, connectionID, chat).Scan(&optouts)
	if err != nil {
		return fmt.Errorf("no se pudo verificar la exclusión de WhatsApp: %w", err)
	}
	if optouts > 0 {
		return errors.New("el cliente tiene una solicitud de baja registrada; requiere nueva autorización comprobable")
	}
	var mode, status string
	err = a.db.QueryRow(`SELECT mode,status FROM inbox_conversations WHERE tenant_id=? AND chat_jid=?`, tid, chat).Scan(&mode, &status)
	if err != nil {
		return errors.New("no se pudo verificar el estado de la conversación")
	}
	if mode == "human" || status == "closed" {
		return errors.New("conversación transferida a humano o cerrada; respuesta automática cancelada")
	}
	var removed int
	err = a.db.QueryRow(`SELECT COUNT(*) FROM inbox_removed_v273511 WHERE tenant_id=? AND chat_jid=?`, tid, chat).Scan(&removed)
	if err != nil {
		return fmt.Errorf("no se pudo comprobar la exclusión de conversación: %w", err)
	}
	if removed > 0 {
		return errors.New("conversación retirada del Inbox; no se enviará respuesta automática")
	}
	return nil
}

// Exactly-once *enqueue* of an identified inbound webhook event. Provider
// retries will not enqueue a second automation event for the same Meta ID.
// This transaction deliberately does not retry outbound messages.
func initAutomationWAInboundV273546Schema(db *DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS automation_wa_inbound_v273546(
        tenant_id INTEGER NOT NULL, connection_id INTEGER NOT NULL,
        meta_message_id TEXT NOT NULL, created_at TEXT NOT NULL,
        PRIMARY KEY(tenant_id,connection_id,meta_message_id)
    );`)
	return err
}

func (a *App) enqueueVerifiedWAInboundV273546(tid, connectionID int64, messageID, chat, txt, timestamp string) error {
	if tid <= 0 || connectionID <= 0 || strings.TrimSpace(messageID) == "" {
		return errors.New("mensaje entrante sin identificador Meta")
	}
	// No automation for events without reliable provider time or already old
	// when received (e.g., webhook backlog after days of downtime).
	if err := automationWAWithinWindowV273546(timestamp, time.Now().UTC()); err != nil {
		return err
	}
	payload := map[string]any{
		"chat": chat, "channel": "whatsapp_cloud", "text": txt,
		"connection_id": connectionID, "message_id": messageID,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	stamp := time.Now().UTC().Format(time.RFC3339)
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	receipt, err := tx.Exec(`INSERT OR IGNORE INTO automation_wa_inbound_v273546(tenant_id,connection_id,meta_message_id,created_at) VALUES(?,?,?,?)`, tid, connectionID, messageID, stamp)
	if err != nil {
		return err
	}
	n, err := receipt.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return nil
	} // already accepted, no further event
	_, err = tx.Exec(`INSERT INTO automation_events(tenant_id,event_type,source,payload_json,status,created_at) VALUES(?,'message.received','inbox',?,'pending',?)`, tid, string(b), stamp)
	if err != nil {
		return err
	}
	return tx.Commit()
}
