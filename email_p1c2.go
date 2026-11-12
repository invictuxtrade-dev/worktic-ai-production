package main

// Worktic AI Email P1C.2 — puente opt-in: correo comercial -> Conversaciones.
// El buzón, tenant, hilo y remitente forman la identidad; sin envío de email.
import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
)

func emailP1C2Enabled(accountID int64, accountEmail string) bool {
	return emailP1Flag("WORKTIC_EMAIL_P1C2_ENABLED") && emailP1C1Enabled(accountID, accountEmail)
}

// El hash oculta el Gmail threadId del ID del chat; separa remitentes de un hilo grupal.
// Evita choques entre tenants incluso si worktic_contacts tiene PK global chat_jid.
func emailP1C2ChatJID(tenant, account int64, threadID, providerID, sender string) string {
	if strings.TrimSpace(threadID) == "" {
		threadID = providerID
	}
	data := fmt.Sprintf("%d|%d|%s|%s", tenant, account, threadID, strings.ToLower(strings.TrimSpace(sender)))
	h := sha256.Sum256([]byte(data))
	return fmt.Sprintf("email:%d:%d:%s", tenant, account, hex.EncodeToString(h[:16]))
}

// Se crea al encender la función, antes de consultar Gmail. No reprocesa correos
// importados antes de la activación ni hace un backfill histórico no solicitado.
func (a *App) emailP1C2StartPilot(tenant, account int64) error {
	if tenant <= 0 || account <= 0 {
		return fmt.Errorf("identidad del piloto inválida")
	}
	for _, ddl := range []string{
		`CREATE TABLE IF NOT EXISTS email_p1c2_pilots (tenant_id BIGINT NOT NULL,account_id BIGINT NOT NULL,starting_message_id BIGINT NOT NULL,activated_at TEXT NOT NULL,PRIMARY KEY(tenant_id,account_id))`,
		`CREATE TABLE IF NOT EXISTS email_p1c2_links (tenant_id BIGINT NOT NULL,account_id BIGINT NOT NULL,provider_id TEXT NOT NULL,chat_jid TEXT NOT NULL,created_at TEXT NOT NULL,PRIMARY KEY(tenant_id,account_id,provider_id))`,
		`CREATE INDEX IF NOT EXISTS email_p1c2_links_chat ON email_p1c2_links(tenant_id,chat_jid)`,
	} {
		if _, e := a.db.Exec(ddl); e != nil {
			return e
		}
	}
	_, e := a.db.Exec(`INSERT INTO email_p1c2_pilots(tenant_id,account_id,starting_message_id,activated_at)
        SELECT ?,?,COALESCE(MAX(id),0),? FROM email_p1_messages WHERE tenant_id=? AND account_id=?
        ON CONFLICT(tenant_id,account_id) DO NOTHING`, tenant, account, emailP1Now(), tenant, account)
	return e
}

type emailP1C2Pending struct {
	ID                                                                    int64
	ProviderID, ThreadID, Sender, Subject, Body, When, Category, Priority string
}

// Recupera también sincronizaciones cuya persistencia en Email IA tuvo éxito
// pero cuya vinculación CRM/Conversaciones no se completó. Máximo 40 por vuelta.
func (a *App) emailP1C2Pump(tenant, account int64, accountEmail string) {
	if !a.emailP1C2AccountEnabled(tenant, account, accountEmail) {
		return
	}
	rows, e := a.db.Query(`SELECT m.id,m.provider_id,m.thread_id,m.sender,m.subject,m.body,m.received_at,m.category,m.priority
        FROM email_p1_messages m
        JOIN email_p1c2_pilots p ON p.tenant_id=m.tenant_id AND p.account_id=m.account_id
        WHERE m.tenant_id=? AND m.account_id=? AND m.id>p.starting_message_id
        AND m.direction='in' AND m.category IN ('Ventas','Soporte','Facturación','Agenda')
        AND NOT EXISTS(SELECT 1 FROM email_p1c2_links l WHERE l.tenant_id=m.tenant_id AND l.account_id=m.account_id AND l.provider_id=m.provider_id)
        ORDER BY m.received_at ASC,m.id ASC LIMIT 40`, tenant, account)
	if e != nil {
		log.Printf("[EMAIL-P1C2] cola temporalmente no disponible tenant=%d", tenant)
		return
	}
	pending := []emailP1C2Pending{}
	for rows.Next() {
		var m emailP1C2Pending
		if rows.Scan(&m.ID, &m.ProviderID, &m.ThreadID, &m.Sender, &m.Subject, &m.Body, &m.When, &m.Category, &m.Priority) == nil {
			pending = append(pending, m)
		}
	}
	readErr := rows.Err()
	rows.Close()
	if readErr != nil {
		log.Printf("[EMAIL-P1C2] lectura pendiente tenant=%d", tenant)
		return
	}
	for _, m := range pending {
		if strings.TrimSpace(m.Sender) == "" || strings.EqualFold(strings.TrimSpace(m.Sender), strings.TrimSpace(accountEmail)) {
			continue
		}
		// Usa el CRM existente, que respeta cuotas, aislamiento e identidad por email.
		if e := a.syncCRMContactAt(tenant, "", "", m.Sender, "email", "email_inbound", "email:"+m.Sender, m.When); e != nil {
			log.Printf("[EMAIL-P1C2] contacto pendiente tenant=%d", tenant)
			continue
		}
		if e := a.emailP1C2MirrorOne(tenant, account, m); e != nil {
			log.Printf("[EMAIL-P1C2] conversación pendiente tenant=%d", tenant)
		}
	}
}

// Tres escrituras + clave de idempotencia en una sola transacción.
// No se usa /api/send y no se genera ninguna tarea de salida.
func (a *App) emailP1C2MirrorOne(tenant, account int64, m emailP1C2Pending) error {
	chat := emailP1C2ChatJID(tenant, account, m.ThreadID, m.ProviderID, m.Sender)
	now := emailP1Now()
	mode := "human"
	// Consultar configuración antes de abrir la transacción evita bloquear SQLite.
	if a.emailP1D2AccountReady(tenant, account) && m.Category == "Ventas" && emailP1D2AllowedRecipient(m.Sender) {
		settings, err := a.emailP1D2GetSettings(tenant, account)
		if err == nil && m.When > settings.EnabledAt {
			mode = "ai"
		}
	}
	tx, e := a.db.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	r, e := tx.Exec(`INSERT INTO email_p1c2_links(tenant_id,account_id,provider_id,chat_jid,created_at)
        VALUES(?,?,?,?,?) ON CONFLICT(tenant_id,account_id,provider_id) DO NOTHING`, tenant, account, m.ProviderID, chat, now)
	if e != nil {
		return e
	}
	affected, e := r.RowsAffected()
	if e != nil {
		return e
	}
	if affected == 0 {
		return nil
	} // Duplicado: una ejecución anterior ya se comprometió.

	// El correo se muestra como identificador en Conversaciones; no es un teléfono.
	r, e = tx.Exec(`INSERT INTO worktic_contacts(tenant_id,channel_connection_id,chat_jid,channel,phone,name,unread,updated_at)
        VALUES(?,0,?,'email',?,?,1,?)
        ON CONFLICT(chat_jid) DO UPDATE SET unread=worktic_contacts.unread+1,
        updated_at=CASE WHEN worktic_contacts.updated_at<excluded.updated_at THEN excluded.updated_at ELSE worktic_contacts.updated_at END
        WHERE worktic_contacts.tenant_id=excluded.tenant_id AND worktic_contacts.channel='email'`,
		tenant, chat, m.Sender, m.Sender, m.When)
	if e != nil {
		return e
	}
	affected, e = r.RowsAffected()
	if e != nil {
		return e
	}
	if affected == 0 {
		return fmt.Errorf("chat reservado para otro canal/tenant")
	}
	text := emailP1Clean("Asunto: "+m.Subject+"\n\n"+m.Body, 8200)
	_, e = tx.Exec(`INSERT INTO worktic_messages(tenant_id,channel_connection_id,channel,wa_id,chat_jid,sender_jid,direction,message_type,text,status,timestamp)
        VALUES(?,0,'email',?,?,?,'in','text',?,'received',?)`, tenant, "email:"+m.ProviderID, chat, m.Sender, text, m.When)
	if e != nil {
		return e
	}
	priority := "normal"
	if m.Priority == "alta" {
		priority = "high"
	}
	_, e = tx.Exec(`INSERT INTO inbox_conversations(tenant_id,chat_jid,status,mode,priority,department,updated_at)
        VALUES(?,?,'open',?,?,?,?) ON CONFLICT(tenant_id,chat_jid) DO NOTHING`, tenant, chat,
		mode, priority, m.Category, now)
	if e != nil {
		return e
	}
	return tx.Commit()
}
