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

type CRMOpportunityRecord struct {
	ID              int64   `json:"id"`
	TenantID        int64   `json:"tenant_id"`
	ContactID       int64   `json:"contact_id"`
	Title           string  `json:"title"`
	Stage           string  `json:"stage"`
	Value           float64 `json:"value"`
	Owner           string  `json:"owner"`
	Source          string  `json:"source"`
	SourceRef       string  `json:"source_ref"`
	Channel         string  `json:"channel"`
	Interest        string  `json:"interest"`
	Score           int     `json:"score"`
	Probability     int     `json:"probability"`
	AutomationState string  `json:"automation_state"`
	FirstSeenAt     string  `json:"first_seen_at"`
	LastActivityAt  string  `json:"last_activity_at"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
	ClosedAt        string  `json:"closed_at"`
	ContactName     string  `json:"contact_name"`
	ContactPhone    string  `json:"contact_phone"`
	ContactEmail    string  `json:"contact_email"`
	ContactChannel  string  `json:"contact_channel"`
	LastMessage     string  `json:"last_message"`
	ManualLock      bool    `json:"manual_lock"`
}

func initCRMOpportunitiesPremiumSchema(db *sql.DB) error {
	migrations := []string{
		`ALTER TABLE crm_opportunities ADD COLUMN tenant_id INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE crm_opportunities ADD COLUMN source TEXT NOT NULL DEFAULT 'manual'`,
		`ALTER TABLE crm_opportunities ADD COLUMN source_ref TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE crm_opportunities ADD COLUMN channel TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE crm_opportunities ADD COLUMN interest TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE crm_opportunities ADD COLUMN score INTEGER NOT NULL DEFAULT 50`,
		`ALTER TABLE crm_opportunities ADD COLUMN probability INTEGER NOT NULL DEFAULT 10`,
		`ALTER TABLE crm_opportunities ADD COLUMN automation_state TEXT NOT NULL DEFAULT 'manual'`,
		`ALTER TABLE crm_opportunities ADD COLUMN first_seen_at TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE crm_opportunities ADD COLUMN last_activity_at TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE crm_opportunities ADD COLUMN created_at TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE crm_opportunities ADD COLUMN closed_at TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE crm_opportunities ADD COLUMN deleted_at TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE crm_opportunities ADD COLUMN manual_lock INTEGER NOT NULL DEFAULT 0`,
		`CREATE INDEX IF NOT EXISTS idx_crm_opportunities_tenant_stage ON crm_opportunities(tenant_id,stage,last_activity_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_crm_opportunities_tenant_contact ON crm_opportunities(tenant_id,contact_id,updated_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_crm_opportunities_tenant_source ON crm_opportunities(tenant_id,source,source_ref)`,
	}
	for _, query := range migrations {
		if _, err := db.Exec(query); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
			return err
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, _ = db.Exec(`UPDATE crm_opportunities SET
		tenant_id=CASE WHEN tenant_id=0 AND contact_id>0 THEN COALESCE((SELECT tenant_id FROM crm_contacts c WHERE c.id=crm_opportunities.contact_id),0) ELSE tenant_id END,
		source=CASE WHEN TRIM(source)='' THEN 'manual' ELSE source END,
		automation_state=CASE WHEN TRIM(automation_state)='' THEN 'manual' ELSE automation_state END,
		manual_lock=CASE WHEN source='manual' THEN 1 ELSE manual_lock END,
		first_seen_at=CASE WHEN TRIM(first_seen_at)='' THEN CASE WHEN TRIM(updated_at)='' THEN ? ELSE updated_at END ELSE first_seen_at END,
		last_activity_at=CASE WHEN TRIM(last_activity_at)='' THEN CASE WHEN TRIM(updated_at)='' THEN ? ELSE updated_at END ELSE last_activity_at END,
		created_at=CASE WHEN TRIM(created_at)='' THEN CASE WHEN TRIM(updated_at)='' THEN ? ELSE updated_at END ELSE created_at END,
		updated_at=CASE WHEN TRIM(updated_at)='' THEN ? ELSE updated_at END,
		probability=CASE WHEN probability<=0 THEN CASE stage WHEN 'Ganado' THEN 100 WHEN 'Negociación' THEN 80 WHEN 'Cotización' THEN 65 WHEN 'Calificado' THEN 45 WHEN 'Interesado' THEN 25 ELSE 10 END ELSE probability END`, now, now, now, now)

	var tenantCount int
	_ = db.QueryRow(`SELECT COUNT(*) FROM tenants`).Scan(&tenantCount)
	if tenantCount == 1 {
		var tenantID int64
		if db.QueryRow(`SELECT id FROM tenants LIMIT 1`).Scan(&tenantID) == nil && tenantID > 0 {
			_, _ = db.Exec(`UPDATE crm_opportunities SET tenant_id=? WHERE tenant_id=0`, tenantID)
		}
	}
	return syncAllExistingOpportunities(db)
}

func normalizeOpportunityStage(stage string) string {
	stage = strings.TrimSpace(stage)
	allowed := map[string]bool{
		"Nuevo": true, "Contactado": true, "Interesado": true, "Calificado": true,
		"Cotización": true, "Negociación": true, "Ganado": true, "Perdido": true,
	}
	if !allowed[stage] {
		return "Nuevo"
	}
	return stage
}

func opportunityStageRank(stage string) int {
	return map[string]int{
		"Nuevo": 0, "Contactado": 1, "Interesado": 2, "Calificado": 3,
		"Cotización": 4, "Negociación": 5, "Ganado": 6, "Perdido": 6,
	}[normalizeOpportunityStage(stage)]
}

func opportunityProbability(stage string) int {
	switch normalizeOpportunityStage(stage) {
	case "Ganado":
		return 100
	case "Perdido":
		return 0
	case "Negociación":
		return 80
	case "Cotización":
		return 65
	case "Calificado":
		return 45
	case "Interesado":
		return 25
	case "Contactado":
		return 15
	default:
		return 10
	}
}

func clampOpportunityScore(score int) int {
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}

func normalizeIntentText(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	replacer := strings.NewReplacer(
		"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n",
	)
	return replacer.Replace(text)
}

func detectCommercialOpportunity(text string) (bool, string, string, int) {
	normalized := normalizeIntentText(text)
	if normalized == "" {
		return false, "", "", 0
	}
	type signal struct {
		terms    []string
		interest string
		stage    string
		score    int
	}
	signals := []signal{
		{[]string{"quiero comprar", "quiero contratar", "como pago", "enlace de pago", "listo para comprar", "lo voy a comprar"}, "Compra o contratación", "Calificado", 90},
		{[]string{"cotizacion", "cotizar", "presupuesto", "propuesta comercial"}, "Solicitud de cotización", "Calificado", 82},
		{[]string{"agendar", "reservar", "separar cita", "quiero una cita", "demo", "demostracion"}, "Agenda o demostración", "Calificado", 78},
		{[]string{"precio", "precios", "cuanto cuesta", "cuanto vale", "valor", "tarifa", "planes", "plan mensual"}, "Consulta de precio", "Interesado", 70},
		{[]string{"me interesa", "estoy interesado", "quiero informacion", "mas informacion", "como funciona", "producto", "servicio", "disponibilidad"}, "Interés comercial", "Interesado", 62},
	}
	for _, signal := range signals {
		for _, term := range signal.terms {
			if strings.Contains(normalized, term) {
				return true, signal.interest, signal.stage, signal.score
			}
		}
	}
	return false, "", "", 0
}

func opportunityTitle(interest, contactName string) string {
	interest = strings.TrimSpace(interest)
	contactName = strings.TrimSpace(contactName)
	if interest != "" && contactName != "" {
		return interest + " · " + contactName
	}
	if interest != "" {
		return interest
	}
	if contactName != "" {
		return "Oportunidad · " + contactName
	}
	return "Nueva oportunidad"
}

func (a *App) contactIDForOpportunity(tenant int64, contactID int64, phone, email, externalID string) (int64, error) {
	if contactID > 0 {
		var id int64
		err := a.db.QueryRow(`SELECT id FROM crm_contacts WHERE id=? AND tenant_id=? AND COALESCE(deleted_at,'')=''`, contactID, tenant).Scan(&id)
		return id, err
	}
	return findCRMContactID(a.db, tenant, phone, email, externalID, 0)
}

func (a *App) syncAutomaticOpportunity(tenant, contactID int64, source, sourceRef, channel, interest, suggestedStage string, score int, activityAt string) (int64, bool, error) {
	if tenant <= 0 || contactID <= 0 {
		return 0, false, nil
	}
	if activityAt == "" {
		activityAt = time.Now().UTC().Format(time.RFC3339)
	}
	source = strings.TrimSpace(source)
	if source == "" {
		source = "automation"
	}
	channel = strings.TrimSpace(channel)
	interest = strings.TrimSpace(interest)
	suggestedStage = normalizeOpportunityStage(suggestedStage)
	score = clampOpportunityScore(score)
	if score == 0 {
		score = 50
	}

	var contactName, contactStage string
	if err := a.db.QueryRow(`SELECT name,stage FROM crm_contacts WHERE id=? AND tenant_id=? AND COALESCE(deleted_at,'')=''`, contactID, tenant).Scan(&contactName, &contactStage); err != nil {
		return 0, false, err
	}

	var id int64
	var currentTitle, currentStage, currentInterest string
	var currentScore, manualLock int
	err := a.db.QueryRow(`SELECT id,title,stage,interest,score,manual_lock FROM crm_opportunities
		WHERE tenant_id=? AND contact_id=? AND COALESCE(deleted_at,'')='' AND stage NOT IN ('Ganado','Perdido')
		ORDER BY last_activity_at DESC,id DESC LIMIT 1`, tenant, contactID).Scan(&id, &currentTitle, &currentStage, &currentInterest, &currentScore, &manualLock)

	now := time.Now().UTC().Format(time.RFC3339)
	if err == sql.ErrNoRows {
		title := opportunityTitle(interest, contactName)
		probability := opportunityProbability(suggestedStage)
		closedAt := ""
		if suggestedStage == "Ganado" || suggestedStage == "Perdido" {
			closedAt = activityAt
		}
		res, insertErr := a.db.Exec(`INSERT INTO crm_opportunities(
			tenant_id,contact_id,title,stage,value,owner,source,source_ref,channel,interest,score,probability,
			automation_state,first_seen_at,last_activity_at,created_at,updated_at,closed_at,deleted_at,manual_lock
		) VALUES(?,?,?,?,0,'',?,?,?,?,?,?,'automatic',?,?,?,?,?,'',0)`,
			tenant, contactID, title, suggestedStage, source, sourceRef, channel, interest, score, probability,
			activityAt, activityAt, activityAt, now, closedAt)
		if insertErr != nil {
			return 0, false, insertErr
		}
		id, _ = res.LastInsertId()
		if opportunityStageRank(contactStage) < opportunityStageRank(suggestedStage) {
			_, _ = a.db.Exec(`UPDATE crm_contacts SET stage=?,updated_at=? WHERE id=? AND tenant_id=?`, suggestedStage, now, contactID, tenant)
		}
		return id, true, nil
	}
	if err != nil {
		return 0, false, err
	}

	nextStage := currentStage
	if opportunityStageRank(suggestedStage) > opportunityStageRank(currentStage) {
		nextStage = suggestedStage
	}
	nextScore := currentScore
	if score > nextScore {
		nextScore = score
	}
	nextTitle := currentTitle
	nextInterest := currentInterest
	if manualLock == 0 {
		if interest != "" {
			nextInterest = interest
			nextTitle = opportunityTitle(interest, contactName)
		}
	}
	closedAt := ""
	if nextStage == "Ganado" || nextStage == "Perdido" {
		closedAt = now
	}
	_, err = a.db.Exec(`UPDATE crm_opportunities SET
		title=?,stage=?,source=CASE WHEN manual_lock=0 AND (source='' OR source='manual') THEN ? ELSE source END,
		source_ref=CASE WHEN manual_lock=0 AND source_ref='' THEN ? ELSE source_ref END,
		channel=CASE WHEN channel='' THEN ? ELSE channel END,
		interest=?,score=?,probability=?,last_activity_at=CASE WHEN last_activity_at<? THEN ? ELSE last_activity_at END,
		updated_at=?,closed_at=CASE WHEN ?<>'' THEN ? ELSE closed_at END,
		automation_state=CASE WHEN manual_lock=1 THEN automation_state ELSE 'automatic' END
		WHERE id=? AND tenant_id=?`, nextTitle, nextStage, source, sourceRef, channel, nextInterest, nextScore,
		opportunityProbability(nextStage), activityAt, activityAt, now, closedAt, closedAt, id, tenant)
	if err != nil {
		return 0, false, err
	}
	if opportunityStageRank(contactStage) < opportunityStageRank(nextStage) {
		_, _ = a.db.Exec(`UPDATE crm_contacts SET stage=?,updated_at=? WHERE id=? AND tenant_id=?`, nextStage, now, contactID, tenant)
	}
	return id, false, nil
}

func (a *App) syncLegacyOpportunityIfSingleTenant(externalID, channel, text, activityAt string) error {
	var tenantCount int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM tenants`).Scan(&tenantCount); err != nil || tenantCount != 1 {
		return err
	}
	var tenantID int64
	if err := a.db.QueryRow(`SELECT id FROM tenants LIMIT 1`).Scan(&tenantID); err != nil || tenantID <= 0 {
		return err
	}
	return a.syncOpportunityFromConversation(tenantID, externalID, channel, text, activityAt)
}

func (a *App) syncOpportunityFromConversation(tenant int64, externalID, channel, text, activityAt string) error {
	matched, interest, stage, score := detectCommercialOpportunity(text)
	if !matched {
		return nil
	}
	contactID, err := a.contactIDForOpportunity(tenant, 0, "", "", strings.TrimSpace(externalID))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return err
	}
	_, _, err = a.syncAutomaticOpportunity(tenant, contactID, "conversation", "chat:"+strings.TrimSpace(externalID), channel, interest, stage, score, activityAt)
	return err
}

func (a *App) syncOpportunityFromLead(tenant, leadID int64, name, phone, email, channel, source, interest string, score int, activityAt string) error {
	contactID, err := a.contactIDForOpportunity(tenant, 0, normalizeCRMPhone(phone), normalizeCRMEmail(email), "")
	if err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return err
	}
	if strings.TrimSpace(interest) == "" {
		interest = "Lead captado"
	}
	stage := "Interesado"
	if score >= 75 {
		stage = "Calificado"
	}
	_, _, err = a.syncAutomaticOpportunity(tenant, contactID, source, fmt.Sprintf("lead:%d", leadID), channel, interest, stage, score, activityAt)
	return err
}

func (a *App) syncOpportunityFromContactStage(tenant, contactID int64, stage string) error {
	stage = normalizeOpportunityStage(stage)
	if opportunityStageRank(stage) < opportunityStageRank("Interesado") {
		return nil
	}
	var channel string
	if err := a.db.QueryRow(`SELECT channel FROM crm_contacts WHERE id=? AND tenant_id=? AND COALESCE(deleted_at,'')=''`, contactID, tenant).Scan(&channel); err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if stage == "Ganado" || stage == "Perdido" {
		var existingID int64
		err := a.db.QueryRow(`SELECT id FROM crm_opportunities WHERE tenant_id=? AND contact_id=? AND COALESCE(deleted_at,'')='' ORDER BY updated_at DESC,id DESC LIMIT 1`, tenant, contactID).Scan(&existingID)
		if err == nil && existingID > 0 {
			_, err = a.db.Exec(`UPDATE crm_opportunities SET stage=?,probability=?,closed_at=?,last_activity_at=?,updated_at=? WHERE id=? AND tenant_id=?`, stage, opportunityProbability(stage), now, now, now, existingID, tenant)
			return err
		}
		if err != nil && err != sql.ErrNoRows {
			return err
		}
	}
	_, _, err := a.syncAutomaticOpportunity(tenant, contactID, "crm", fmt.Sprintf("contact:%d", contactID), channel, "Seguimiento comercial", stage, 60+opportunityStageRank(stage)*5, now)
	return err
}

func syncAllExistingOpportunities(db *sql.DB) error {
	rows, err := db.Query(`SELECT id FROM tenants ORDER BY id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	var tenantIDs []int64
	for rows.Next() {
		var id int64
		if rows.Scan(&id) == nil && id > 0 {
			tenantIDs = append(tenantIDs, id)
		}
	}
	app := &App{db: db}
	for _, tenant := range tenantIDs {
		if err := app.syncExistingOpportunitiesForTenant(tenant); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) syncExistingOpportunitiesForTenant(tenant int64) error {
	type leadSeed struct {
		id                                              int64
		name, phone, email, source, interest, createdAt string
		score                                           int
	}
	leadRows, err := a.db.Query(`SELECT id,name,phone,email,source,interest,score,created_at FROM marketing_leads WHERE tenant_id=? ORDER BY id`, tenant)
	if err == nil {
		var leads []leadSeed
		for leadRows.Next() {
			var seed leadSeed
			if leadRows.Scan(&seed.id, &seed.name, &seed.phone, &seed.email, &seed.source, &seed.interest, &seed.score, &seed.createdAt) == nil {
				leads = append(leads, seed)
			}
		}
		leadRows.Close()
		for _, seed := range leads {
			if seed.source == "" || seed.source == "worktic_form" {
				seed.source = "form"
			}
			_ = a.syncOpportunityFromLead(tenant, seed.id, seed.name, seed.phone, seed.email, "landing", seed.source, seed.interest, seed.score, seed.createdAt)
		}
	}

	contactRows, err := a.db.Query(`SELECT id,stage FROM crm_contacts WHERE tenant_id=? AND COALESCE(deleted_at,'')='' AND stage IN ('Interesado','Calificado','Cotización','Negociación','Ganado','Perdido')`, tenant)
	if err == nil {
		var contacts []struct {
			id    int64
			stage string
		}
		for contactRows.Next() {
			var contact struct {
				id    int64
				stage string
			}
			if contactRows.Scan(&contact.id, &contact.stage) == nil {
				contacts = append(contacts, contact)
			}
		}
		contactRows.Close()
		for _, contact := range contacts {
			_ = a.syncOpportunityFromContactStage(tenant, contact.id, contact.stage)
		}
	}

	messageRows, err := a.db.Query(`SELECT chat_jid,channel,text,timestamp FROM worktic_messages WHERE tenant_id=? AND direction='in' ORDER BY id DESC LIMIT 1000`, tenant)
	if err == nil {
		type messageSeed struct{ chat, channel, text, timestamp string }
		var messages []messageSeed
		for messageRows.Next() {
			var message messageSeed
			if messageRows.Scan(&message.chat, &message.channel, &message.text, &message.timestamp) == nil {
				messages = append(messages, message)
			}
		}
		messageRows.Close()
		for _, message := range messages {
			_ = a.syncOpportunityFromConversation(tenant, message.chat, message.channel, message.text, message.timestamp)
		}
	}
	return nil
}

func (a *App) opportunitiesPremiumHandler(w http.ResponseWriter, r *http.Request) {
	tenant, _, err := a.tenantFor(r)
	if err != nil {
		writeError(w, err, http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodGet:
		rows, err := a.db.Query(`SELECT
			o.id,o.tenant_id,o.contact_id,o.title,o.stage,o.value,o.owner,o.source,o.source_ref,o.channel,o.interest,
			o.score,o.probability,o.automation_state,o.first_seen_at,o.last_activity_at,o.created_at,o.updated_at,o.closed_at,o.manual_lock,
			COALESCE(c.name,''),COALESCE(c.phone,''),COALESCE(c.email,''),COALESCE(c.channel,''),
			CASE WHEN COALESCE(c.external_id,'')<>'' THEN COALESCE((SELECT m.text FROM worktic_messages m WHERE m.chat_jid=c.external_id AND (m.tenant_id=c.tenant_id OR m.tenant_id=0) ORDER BY m.id DESC LIMIT 1),'') ELSE '' END
		FROM crm_opportunities o
		LEFT JOIN crm_contacts c ON c.id=o.contact_id AND c.tenant_id=o.tenant_id
		WHERE o.tenant_id=? AND COALESCE(o.deleted_at,'')=''
		ORDER BY CASE o.stage WHEN 'Negociación' THEN 0 WHEN 'Cotización' THEN 1 WHEN 'Calificado' THEN 2 WHEN 'Interesado' THEN 3 WHEN 'Nuevo' THEN 4 ELSE 5 END,
		COALESCE(NULLIF(o.last_activity_at,''),o.updated_at) DESC,o.id DESC`, tenant)
		if err != nil {
			writeError(w, err, http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		out := []CRMOpportunityRecord{}
		for rows.Next() {
			var item CRMOpportunityRecord
			var manualLock int
			if err := rows.Scan(&item.ID, &item.TenantID, &item.ContactID, &item.Title, &item.Stage, &item.Value, &item.Owner, &item.Source, &item.SourceRef, &item.Channel, &item.Interest, &item.Score, &item.Probability, &item.AutomationState, &item.FirstSeenAt, &item.LastActivityAt, &item.CreatedAt, &item.UpdatedAt, &item.ClosedAt, &manualLock, &item.ContactName, &item.ContactPhone, &item.ContactEmail, &item.ContactChannel, &item.LastMessage); err != nil {
				writeError(w, err, http.StatusInternalServerError)
				return
			}
			item.ManualLock = manualLock == 1
			out = append(out, item)
		}
		writeJSON(w, out)

	case http.MethodPost:
		var item CRMOpportunityRecord
		if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
			writeError(w, errors.New("datos de oportunidad inválidos"), http.StatusBadRequest)
			return
		}
		item.Title = strings.TrimSpace(item.Title)
		item.Stage = normalizeOpportunityStage(item.Stage)
		item.Owner = strings.TrimSpace(item.Owner)
		item.Interest = strings.TrimSpace(item.Interest)
		item.Channel = strings.TrimSpace(item.Channel)
		if item.Title == "" {
			writeError(w, errors.New("escribe el nombre de la oportunidad"), http.StatusBadRequest)
			return
		}
		if item.Value < 0 {
			item.Value = 0
		}
		if item.ContactID > 0 {
			if _, err := a.contactIDForOpportunity(tenant, item.ContactID, "", "", ""); err != nil {
				writeError(w, errors.New("el contacto relacionado no pertenece a este espacio"), http.StatusBadRequest)
				return
			}
			var existingID int64
			err := a.db.QueryRow(`SELECT id FROM crm_opportunities WHERE tenant_id=? AND contact_id=? AND COALESCE(deleted_at,'')='' AND stage NOT IN ('Ganado','Perdido') ORDER BY id DESC LIMIT 1`, tenant, item.ContactID).Scan(&existingID)
			if err == nil && existingID > 0 {
				now := time.Now().UTC().Format(time.RFC3339)
				_, err = a.db.Exec(`UPDATE crm_opportunities SET title=?,stage=?,value=?,owner=?,channel=?,interest=?,score=?,probability=?,automation_state=CASE WHEN automation_state='automatic' THEN 'assisted' ELSE automation_state END,manual_lock=1,last_activity_at=?,updated_at=? WHERE id=? AND tenant_id=?`, item.Title, item.Stage, item.Value, item.Owner, item.Channel, item.Interest, clampOpportunityScore(item.Score), opportunityProbability(item.Stage), now, now, existingID, tenant)
				if err != nil {
					writeError(w, err, http.StatusInternalServerError)
					return
				}
				_, _ = a.db.Exec(`UPDATE crm_contacts SET stage=?,updated_at=? WHERE id=? AND tenant_id=?`, item.Stage, now, item.ContactID, tenant)
				writeJSON(w, map[string]any{"ok": true, "id": existingID, "merged": true})
				return
			}
		}
		now := time.Now().UTC().Format(time.RFC3339)
		closedAt := ""
		if item.Stage == "Ganado" || item.Stage == "Perdido" {
			closedAt = now
		}
		res, err := a.db.Exec(`INSERT INTO crm_opportunities(tenant_id,contact_id,title,stage,value,owner,source,source_ref,channel,interest,score,probability,automation_state,first_seen_at,last_activity_at,created_at,updated_at,closed_at,deleted_at,manual_lock) VALUES(?,?,?,?,?,?,'manual','',?,?,?,?,'manual',?,?,?,?,?,'',1)`, tenant, item.ContactID, item.Title, item.Stage, item.Value, item.Owner, item.Channel, item.Interest, clampOpportunityScore(item.Score), opportunityProbability(item.Stage), now, now, now, now, closedAt)
		if err != nil {
			writeError(w, err, http.StatusInternalServerError)
			return
		}
		item.ID, _ = res.LastInsertId()
		if item.ContactID > 0 {
			_, _ = a.db.Exec(`UPDATE crm_contacts SET stage=?,updated_at=? WHERE id=? AND tenant_id=?`, item.Stage, now, item.ContactID, tenant)
		}
		writeJSON(w, map[string]any{"ok": true, "id": item.ID, "merged": false})

	case http.MethodPut:
		var item CRMOpportunityRecord
		if err := json.NewDecoder(r.Body).Decode(&item); err != nil || item.ID <= 0 {
			writeError(w, errors.New("oportunidad inválida"), http.StatusBadRequest)
			return
		}
		item.Title = strings.TrimSpace(item.Title)
		item.Stage = normalizeOpportunityStage(item.Stage)
		item.Owner = strings.TrimSpace(item.Owner)
		item.Interest = strings.TrimSpace(item.Interest)
		item.Channel = strings.TrimSpace(item.Channel)
		if item.Title == "" {
			writeError(w, errors.New("escribe el nombre de la oportunidad"), http.StatusBadRequest)
			return
		}
		if item.ContactID > 0 {
			if _, err := a.contactIDForOpportunity(tenant, item.ContactID, "", "", ""); err != nil {
				writeError(w, errors.New("el contacto relacionado no pertenece a este espacio"), http.StatusBadRequest)
				return
			}
		}
		now := time.Now().UTC().Format(time.RFC3339)
		closedAt := ""
		if item.Stage == "Ganado" || item.Stage == "Perdido" {
			closedAt = now
		}
		res, err := a.db.Exec(`UPDATE crm_opportunities SET contact_id=?,title=?,stage=?,value=?,owner=?,channel=?,interest=?,score=?,probability=?,automation_state=CASE WHEN automation_state='automatic' THEN 'assisted' ELSE automation_state END,manual_lock=1,last_activity_at=?,updated_at=?,closed_at=? WHERE id=? AND tenant_id=? AND COALESCE(deleted_at,'')=''`, item.ContactID, item.Title, item.Stage, item.Value, item.Owner, item.Channel, item.Interest, clampOpportunityScore(item.Score), opportunityProbability(item.Stage), now, now, closedAt, item.ID, tenant)
		if err != nil {
			writeError(w, err, http.StatusInternalServerError)
			return
		}
		affected, _ := res.RowsAffected()
		if affected == 0 {
			writeError(w, errors.New("oportunidad no encontrada"), http.StatusNotFound)
			return
		}
		if item.ContactID > 0 {
			_, _ = a.db.Exec(`UPDATE crm_contacts SET stage=?,updated_at=? WHERE id=? AND tenant_id=?`, item.Stage, now, item.ContactID, tenant)
		}
		writeJSON(w, map[string]any{"ok": true, "id": item.ID})

	case http.MethodDelete:
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		if id <= 0 {
			writeError(w, errors.New("oportunidad inválida"), http.StatusBadRequest)
			return
		}
		now := time.Now().UTC().Format(time.RFC3339)
		res, err := a.db.Exec(`UPDATE crm_opportunities SET deleted_at=?,updated_at=? WHERE id=? AND tenant_id=? AND COALESCE(deleted_at,'')=''`, now, now, id, tenant)
		if err != nil {
			writeError(w, err, http.StatusInternalServerError)
			return
		}
		affected, _ := res.RowsAffected()
		if affected == 0 {
			writeError(w, errors.New("oportunidad no encontrada"), http.StatusNotFound)
			return
		}
		writeJSON(w, map[string]any{"ok": true})

	default:
		writeError(w, errors.New("método no permitido"), http.StatusMethodNotAllowed)
	}
}
