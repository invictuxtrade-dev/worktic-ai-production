package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type CRMContactRecord struct {
	ID                int64  `json:"id"`
	TenantID          int64  `json:"tenant_id"`
	Name              string `json:"name"`
	Phone             string `json:"phone"`
	Email             string `json:"email"`
	Channel           string `json:"channel"`
	Source            string `json:"source"`
	ExternalID        string `json:"external_id"`
	Stage             string `json:"stage"`
	Tags              string `json:"tags"`
	Notes             string `json:"notes"`
	FirstSeenAt       string `json:"first_seen_at"`
	LastActivityAt    string `json:"last_activity_at"`
	CreatedAt         string `json:"created_at"`
	UpdatedAt         string `json:"updated_at"`
	ConversationCount int    `json:"conversation_count"`
	LeadCount         int    `json:"lead_count"`
	OpportunityCount  int    `json:"opportunity_count"`
	LastMessage       string `json:"last_message"`
}

func initCRMContactsPremiumSchema(db *sql.DB) error {
	migrations := []string{
		`ALTER TABLE crm_contacts ADD COLUMN tenant_id INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE crm_contacts ADD COLUMN source TEXT NOT NULL DEFAULT 'manual'`,
		`ALTER TABLE crm_contacts ADD COLUMN external_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE crm_contacts ADD COLUMN first_seen_at TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE crm_contacts ADD COLUMN last_activity_at TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE crm_contacts ADD COLUMN created_at TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE crm_contacts ADD COLUMN deleted_at TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE crm_contacts ADD COLUMN identity_locked INTEGER NOT NULL DEFAULT 0`,
		`CREATE INDEX IF NOT EXISTS idx_crm_contacts_tenant_activity ON crm_contacts(tenant_id,last_activity_at DESC,updated_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_crm_contacts_tenant_phone ON crm_contacts(tenant_id,phone)`,
		`CREATE INDEX IF NOT EXISTS idx_crm_contacts_tenant_email ON crm_contacts(tenant_id,email)`,
		`CREATE INDEX IF NOT EXISTS idx_crm_contacts_tenant_external ON crm_contacts(tenant_id,external_id)`,
	}
	for _, q := range migrations {
		if _, err := db.Exec(q); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
			return err
		}
	}
	// Conserva las identidades editadas en instalaciones anteriores. Este bloque
	// se ejecuta una sola vez; los contactos automáticos nuevos permanecen desbloqueados.
	var identityMigration string
	_ = db.QueryRow(`SELECT value FROM worktic_settings WHERE key='crm_identity_lock_v1'`).Scan(&identityMigration)
	if identityMigration == "" {
		_, _ = db.Exec(`UPDATE crm_contacts SET identity_locked=1`)
		_, _ = db.Exec(`INSERT OR REPLACE INTO worktic_settings(key,value) VALUES('crm_identity_lock_v1','done')`)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, _ = db.Exec(`UPDATE crm_contacts SET
		source=CASE WHEN TRIM(source)='' THEN CASE WHEN TRIM(channel)='' THEN 'manual' ELSE channel END ELSE source END,
		first_seen_at=CASE WHEN TRIM(first_seen_at)='' THEN CASE WHEN TRIM(updated_at)='' THEN ? ELSE updated_at END ELSE first_seen_at END,
		last_activity_at=CASE WHEN TRIM(last_activity_at)='' THEN CASE WHEN TRIM(updated_at)='' THEN ? ELSE updated_at END ELSE last_activity_at END,
		created_at=CASE WHEN TRIM(created_at)='' THEN CASE WHEN TRIM(updated_at)='' THEN ? ELSE updated_at END ELSE created_at END,
		updated_at=CASE WHEN TRIM(updated_at)='' THEN ? ELSE updated_at END`, now, now, now, now)

	// Solo adjudica datos legacy sin tenant cuando existe un único espacio.
	var tenantCount int
	_ = db.QueryRow(`SELECT COUNT(*) FROM tenants`).Scan(&tenantCount)
	if tenantCount == 1 {
		var tenantID int64
		if db.QueryRow(`SELECT id FROM tenants LIMIT 1`).Scan(&tenantID) == nil && tenantID > 0 {
			_, _ = db.Exec(`UPDATE crm_contacts SET tenant_id=? WHERE tenant_id=0`, tenantID)
		}
	}
	return syncAllExistingCRMContacts(db)
}

func normalizeCRMPhone(v string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(v) {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func normalizeCRMEmail(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
}

func normalizeCRMStage(v string) string {
	v = strings.TrimSpace(v)
	allowed := map[string]bool{
		"Nuevo": true, "Contactado": true, "Interesado": true, "Calificado": true,
		"Cotización": true, "Negociación": true, "Ganado": true, "Perdido": true,
	}
	if !allowed[v] {
		return "Nuevo"
	}
	return v
}

func findCRMContactID(db *sql.DB, tenant int64, phone, email, externalID string, excludeID int64) (int64, error) {
	phone = normalizeCRMPhone(phone)
	email = normalizeCRMEmail(email)
	externalID = strings.TrimSpace(externalID)
	var id int64
	err := db.QueryRow(`SELECT id FROM crm_contacts
		WHERE tenant_id=? AND id<>? AND (
			(?<>'' AND external_id=?) OR
			(?<>'' AND phone=?) OR
			(?<>'' AND lower(email)=lower(?))
		) ORDER BY id LIMIT 1`, tenant, excludeID,
		externalID, externalID, phone, phone, email, email).Scan(&id)
	return id, err
}

func (a *App) syncCRMContact(tenant int64, name, phone, email, channel, source, externalID string) error {
	return a.syncCRMContactAt(tenant, name, phone, email, channel, source, externalID, time.Now().UTC().Format(time.RFC3339))
}

func (a *App) syncCRMContactAt(tenant int64, name, phone, email, channel, source, externalID, activityAt string) error {
	if tenant <= 0 {
		return nil
	}
	name = strings.TrimSpace(name)
	phone = normalizeCRMPhone(phone)
	email = normalizeCRMEmail(email)
	channel = strings.TrimSpace(channel)
	source = strings.TrimSpace(source)
	externalID = strings.TrimSpace(externalID)
	if channel == "" {
		channel = "manual"
	}
	if source == "" {
		source = channel
	}
	if activityAt == "" {
		activityAt = time.Now().UTC().Format(time.RFC3339)
	}
	if name == "" && phone == "" && email == "" && externalID == "" {
		return nil
	}

	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var id int64
	var deletedAt string
	err = tx.QueryRow(`SELECT id,deleted_at FROM crm_contacts
		WHERE tenant_id=? AND (
			(?<>'' AND external_id=?) OR
			(?<>'' AND phone=?) OR
			(?<>'' AND lower(email)=lower(?))
		) ORDER BY CASE WHEN external_id=? AND ?<>'' THEN 0 WHEN phone=? AND ?<>'' THEN 1 ELSE 2 END,id LIMIT 1`,
		tenant, externalID, externalID, phone, phone, email, email,
		externalID, externalID, phone, phone).Scan(&id, &deletedAt)
	if err == sql.ErrNoRows {
		_, err = tx.Exec(`INSERT INTO crm_contacts(
			tenant_id,name,phone,email,channel,source,external_id,stage,tags,notes,
			first_seen_at,last_activity_at,created_at,updated_at
		) VALUES(?,?,?,?,?,?,?,'Nuevo','','',?,?,?,?)`,
			tenant, name, phone, email, channel, source, externalID,
			activityAt, activityAt, activityAt, activityAt)
		if err != nil {
			return err
		}
		return tx.Commit()
	}
	if err != nil {
		return err
	}
	// Un contacto eliminado permanece oculto durante el backfill histórico.
	// Solo una interacción realmente posterior vuelve a activarlo.
	if deletedAt != "" && activityAt <= deletedAt {
		return nil
	}
	_, err = tx.Exec(`UPDATE crm_contacts SET
		name=CASE WHEN ?<>'' AND (identity_locked=0 OR name='') THEN ? ELSE name END,
		phone=CASE WHEN ?<>'' AND (identity_locked=0 OR phone='') THEN ? ELSE phone END,
		email=CASE WHEN ?<>'' AND (identity_locked=0 OR email='') THEN ? ELSE email END,
		channel=CASE WHEN ?<>'' THEN ? ELSE channel END,
		source=CASE WHEN source='' OR source='manual' THEN ? ELSE source END,
		external_id=CASE WHEN external_id='' AND ?<>'' THEN ? ELSE external_id END,
		first_seen_at=CASE WHEN first_seen_at='' OR first_seen_at>? THEN ? ELSE first_seen_at END,
		last_activity_at=CASE WHEN last_activity_at='' OR last_activity_at<? THEN ? ELSE last_activity_at END,
		deleted_at=CASE WHEN deleted_at<>'' AND ? > deleted_at THEN '' ELSE deleted_at END,
		updated_at=?
		WHERE id=? AND tenant_id=?`,
		name, name, phone, phone, email, email, channel, channel, source,
		externalID, externalID, activityAt, activityAt, activityAt, activityAt, activityAt,
		time.Now().UTC().Format(time.RFC3339), id, tenant)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func syncAllExistingCRMContacts(db *sql.DB) error {
	rows, err := db.Query(`SELECT id FROM tenants ORDER BY id`)
	if err != nil {
		return err
	}
	ids := []int64{}
	for rows.Next() {
		var id int64
		if rows.Scan(&id) == nil && id > 0 {
			ids = append(ids, id)
		}
	}
	rows.Close()
	for _, id := range ids {
		if err := syncExistingCRMContactsForTenantDB(db, id); err != nil {
			return err
		}
	}
	return nil
}

func syncExistingCRMContactsForTenantDB(db *sql.DB, tenant int64) error {
	app := &App{db: db}
	type conversationSeed struct {
		chat, channel, phone, name, updated string
	}
	conversationSeeds := []conversationSeed{}
	rows, err := db.Query(`SELECT chat_jid,channel,phone,name,updated_at FROM worktic_contacts WHERE tenant_id=?`, tenant)
	if err == nil {
		for rows.Next() {
			var seed conversationSeed
			if rows.Scan(&seed.chat, &seed.channel, &seed.phone, &seed.name, &seed.updated) == nil {
				conversationSeeds = append(conversationSeeds, seed)
			}
		}
		rows.Close()
	}
	for _, seed := range conversationSeeds {
		crmPhone := seed.phone
		if seed.channel != "whatsapp" && seed.channel != "whatsapp_qr" {
			crmPhone = ""
		}
		_ = app.syncCRMContactAt(tenant, seed.name, crmPhone, "", seed.channel, "conversation", seed.chat, seed.updated)
	}

	type leadSeed struct {
		name, phone, email, source, created string
	}
	leadSeeds := []leadSeed{}
	leadRows, leadErr := db.Query(`SELECT name,phone,email,source,created_at FROM marketing_leads WHERE tenant_id=?`, tenant)
	if leadErr == nil {
		for leadRows.Next() {
			var seed leadSeed
			if leadRows.Scan(&seed.name, &seed.phone, &seed.email, &seed.source, &seed.created) == nil {
				leadSeeds = append(leadSeeds, seed)
			}
		}
		leadRows.Close()
	}
	for _, seed := range leadSeeds {
		if seed.source == "" || seed.source == "worktic_form" {
			seed.source = "form"
		}
		_ = app.syncCRMContactAt(tenant, seed.name, seed.phone, seed.email, "landing", seed.source, "", seed.created)
	}
	return nil
}

func (a *App) crmContactsPremiumHandler(w http.ResponseWriter, r *http.Request) {
	tenant, user, err := a.tenantFor(r)
	if err != nil {
		writeError(w, err, http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodGet:
		_ = syncExistingCRMContactsForTenantDB(a.db, tenant)
		rows, err := a.db.Query(`SELECT
			c.id,c.tenant_id,c.name,c.phone,c.email,c.channel,c.source,c.external_id,c.stage,c.tags,c.notes,
			c.first_seen_at,c.last_activity_at,c.created_at,c.updated_at,
			CASE WHEN c.external_id<>'' THEN (SELECT COUNT(*) FROM worktic_messages m WHERE m.chat_jid=c.external_id AND (m.tenant_id=c.tenant_id OR m.tenant_id=0)) ELSE 0 END,
			(SELECT COUNT(*) FROM marketing_leads l WHERE l.tenant_id=c.tenant_id AND ((c.phone<>'' AND REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(l.phone,'+',''),' ',''),'-',''),'(',''),')',''),'.','')=c.phone) OR (c.email<>'' AND lower(l.email)=lower(c.email)))),
			(SELECT COUNT(*) FROM crm_opportunities o WHERE o.contact_id=c.id),
			CASE WHEN c.external_id<>'' THEN COALESCE((SELECT text FROM worktic_messages m WHERE m.chat_jid=c.external_id AND (m.tenant_id=c.tenant_id OR m.tenant_id=0) ORDER BY m.id DESC LIMIT 1),'') ELSE '' END
		FROM crm_contacts c WHERE c.tenant_id=? AND COALESCE(c.deleted_at,'')=''
		ORDER BY COALESCE(NULLIF(c.last_activity_at,''),c.updated_at) DESC,c.id DESC`, tenant)
		if err != nil {
			writeError(w, err, http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		out := []CRMContactRecord{}
		for rows.Next() {
			var x CRMContactRecord
			if err := rows.Scan(&x.ID, &x.TenantID, &x.Name, &x.Phone, &x.Email, &x.Channel, &x.Source, &x.ExternalID, &x.Stage, &x.Tags, &x.Notes, &x.FirstSeenAt, &x.LastActivityAt, &x.CreatedAt, &x.UpdatedAt, &x.ConversationCount, &x.LeadCount, &x.OpportunityCount, &x.LastMessage); err != nil {
				writeError(w, err, http.StatusInternalServerError)
				return
			}
			out = append(out, x)
		}
		if err := rows.Err(); err != nil {
			writeError(w, err, http.StatusInternalServerError)
			return
		}
		writeJSON(w, out)

	case http.MethodPost:
		var x CRMContactRecord
		if err := json.NewDecoder(r.Body).Decode(&x); err != nil {
			writeError(w, errors.New("datos de contacto inválidos"), http.StatusBadRequest)
			return
		}
		x.Name = strings.TrimSpace(x.Name)
		x.Phone = normalizeCRMPhone(x.Phone)
		x.Email = normalizeCRMEmail(x.Email)
		x.ExternalID = strings.TrimSpace(x.ExternalID)
		x.Channel = strings.TrimSpace(x.Channel)
		x.Source = strings.TrimSpace(x.Source)
		x.Stage = normalizeCRMStage(x.Stage)
		x.Tags = strings.TrimSpace(x.Tags)
		x.Notes = strings.TrimSpace(x.Notes)
		if x.Name == "" && x.Phone == "" && x.Email == "" {
			writeError(w, errors.New("escribe al menos el nombre, teléfono o correo"), http.StatusBadRequest)
			return
		}
		if x.Channel == "" {
			x.Channel = "manual"
		}
		if x.Source == "" {
			x.Source = "manual"
		}

		existingID, findErr := findCRMContactID(a.db, tenant, x.Phone, x.Email, x.ExternalID, 0)
		isNew := findErr == sql.ErrNoRows
		if findErr != nil && findErr != sql.ErrNoRows {
			writeError(w, findErr, http.StatusInternalServerError)
			return
		}
		if isNew && user != nil && user.Role != "superadmin" {
			plan, _, err := a.activePlan(a.billingAccountUserID(user))
			if err != nil {
				writeError(w, err, http.StatusInternalServerError)
				return
			}
			var used int
			if err := a.db.QueryRow(`SELECT COUNT(*) FROM crm_contacts WHERE tenant_id=? AND COALESCE(deleted_at,'')=''`, tenant).Scan(&used); err != nil {
				writeError(w, err, http.StatusInternalServerError)
				return
			}
			if plan.MaxContacts >= 0 && used >= plan.MaxContacts {
				writeError(w, fmt.Errorf("alcanzaste el límite de %d contactos de tu plan %s", plan.MaxContacts, plan.Name), http.StatusForbidden)
				return
			}
		}
		now := time.Now().UTC().Format(time.RFC3339)
		if isNew && x.Phone == "" && x.Email == "" && x.ExternalID == "" {
			res, err := a.db.Exec(`INSERT INTO crm_contacts(tenant_id,name,phone,email,channel,source,external_id,stage,tags,notes,first_seen_at,last_activity_at,created_at,updated_at,identity_locked) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,1)`, tenant, x.Name, x.Phone, x.Email, x.Channel, x.Source, x.ExternalID, x.Stage, x.Tags, x.Notes, now, now, now, now)
			if err != nil {
				writeError(w, err, http.StatusInternalServerError)
				return
			}
			x.ID, _ = res.LastInsertId()
		} else {
			if err := a.syncCRMContact(tenant, x.Name, x.Phone, x.Email, x.Channel, x.Source, x.ExternalID); err != nil {
				writeError(w, err, http.StatusInternalServerError)
				return
			}
			if isNew {
				if err := a.db.QueryRow(`SELECT id FROM crm_contacts WHERE tenant_id=? AND ((?<>'' AND external_id=?) OR (?<>'' AND phone=?) OR (?<>'' AND lower(email)=lower(?))) ORDER BY id DESC LIMIT 1`, tenant, x.ExternalID, x.ExternalID, x.Phone, x.Phone, x.Email, x.Email).Scan(&x.ID); err != nil {
					writeError(w, err, http.StatusInternalServerError)
					return
				}
			} else {
				x.ID = existingID
			}
			_, err := a.db.Exec(`UPDATE crm_contacts SET stage=?,tags=?,notes=?,identity_locked=1,deleted_at='',updated_at=? WHERE id=? AND tenant_id=?`, x.Stage, x.Tags, x.Notes, now, x.ID, tenant)
			if err != nil {
				writeError(w, err, http.StatusInternalServerError)
				return
			}
		}
		writeJSON(w, map[string]any{"ok": true, "id": x.ID, "merged": !isNew})

	case http.MethodPut:
		var x CRMContactRecord
		if err := json.NewDecoder(r.Body).Decode(&x); err != nil || x.ID <= 0 {
			writeError(w, errors.New("contacto inválido"), http.StatusBadRequest)
			return
		}
		x.Name = strings.TrimSpace(x.Name)
		x.Phone = normalizeCRMPhone(x.Phone)
		x.Email = normalizeCRMEmail(x.Email)
		x.ExternalID = strings.TrimSpace(x.ExternalID)
		x.Channel = strings.TrimSpace(x.Channel)
		x.Source = strings.TrimSpace(x.Source)
		x.Stage = normalizeCRMStage(x.Stage)
		x.Tags = strings.TrimSpace(x.Tags)
		x.Notes = strings.TrimSpace(x.Notes)
		if x.Name == "" && x.Phone == "" && x.Email == "" {
			writeError(w, errors.New("escribe al menos el nombre, teléfono o correo"), http.StatusBadRequest)
			return
		}
		if x.Channel == "" {
			x.Channel = "manual"
		}
		if x.Source == "" {
			x.Source = "manual"
		}
		if duplicateID, err := findCRMContactID(a.db, tenant, x.Phone, x.Email, x.ExternalID, x.ID); err == nil && duplicateID > 0 {
			writeError(w, fmt.Errorf("el teléfono, correo o identificador ya pertenece al contacto #%d", duplicateID), http.StatusConflict)
			return
		} else if err != nil && err != sql.ErrNoRows {
			writeError(w, err, http.StatusInternalServerError)
			return
		}
		now := time.Now().UTC().Format(time.RFC3339)
		res, err := a.db.Exec(`UPDATE crm_contacts SET name=?,phone=?,email=?,channel=?,source=?,external_id=?,stage=?,tags=?,notes=?,identity_locked=1,updated_at=? WHERE id=? AND tenant_id=? AND COALESCE(deleted_at,'')=''`, x.Name, x.Phone, x.Email, x.Channel, x.Source, x.ExternalID, x.Stage, x.Tags, x.Notes, now, x.ID, tenant)
		if err != nil {
			writeError(w, err, http.StatusInternalServerError)
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			writeError(w, errors.New("contacto no encontrado"), http.StatusNotFound)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "id": x.ID})

	case http.MethodDelete:
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		if id <= 0 {
			writeError(w, errors.New("contacto inválido"), http.StatusBadRequest)
			return
		}
		now := time.Now().UTC().Format(time.RFC3339)
		res, err := a.db.Exec(`UPDATE crm_contacts SET deleted_at=?,updated_at=? WHERE id=? AND tenant_id=? AND COALESCE(deleted_at,'')=''`, now, now, id, tenant)
		if err != nil {
			writeError(w, err, http.StatusInternalServerError)
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			writeError(w, errors.New("contacto no encontrado"), http.StatusNotFound)
			return
		}
		writeJSON(w, map[string]any{"ok": true})

	default:
		writeError(w, errors.New("método no permitido"), http.StatusMethodNotAllowed)
	}
}
