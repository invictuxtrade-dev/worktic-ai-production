package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// A generated 128-bit token identifies a single rendered public-form submission.
// Reposting the same request must not create a second lead or workflow event.
func newPublicFormTokenV273544() (string, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(id[:]), nil
}

func publicFormTokenV273544(raw string) (string, error) {
	if len(raw) == 32 {
		b, err := hex.DecodeString(raw)
		if err == nil && len(b) == 16 {
			return strings.ToLower(raw), nil
		}
	}
	// Old integrations may POST directly without first rendering /f/. Each of
	// these requests remains valid, but they do not have retry deduplication.
	return newPublicFormTokenV273544()
}

func initPublicFormAutomationV273544Schema(db *DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS automation_form_submissions_v273544(
        tenant_id INTEGER NOT NULL,
        form_id INTEGER NOT NULL,
        submission_key TEXT NOT NULL,
        lead_id INTEGER NOT NULL DEFAULT 0,
        event_id INTEGER NOT NULL DEFAULT 0,
        created_at TEXT NOT NULL,
        PRIMARY KEY(tenant_id,form_id,submission_key)
    );`)
	return err
}

func publicFormEventPayloadV273544(form LeadForm, leadID int64, r *http.Request, stamp string) map[string]any {
	return map[string]any{
		"lead_id":         leadID,
		"form_id":         form.ID,
		"form_name":       form.Name,
		"source":          "worktic_form",
		"name":            strings.TrimSpace(r.FormValue("name")),
		"phone":           strings.TrimSpace(r.FormValue("phone")),
		"email":           strings.TrimSpace(r.FormValue("email")),
		"city":            strings.TrimSpace(r.FormValue("city")),
		"interest":        strings.TrimSpace(r.FormValue("interest")),
		"score":           60,
		"privacy_consent": true, // Does NOT establish WhatsApp marketing opt-in.
		"submitted_at":    stamp,
	}
}

// The lead, dedup receipt and event are committed atomically. Events are held
// until existing CRM synchronization finishes so a workflow cannot race ahead
// of contact/opportunity creation. No automatic replay of ambiguous work.
func (a *App) storePublicFormSubmissionV273544(tid int64, form LeadForm, r *http.Request, stamp string) error {
	if tid <= 0 || form.ID <= 0 || form.TenantID != tid {
		return errors.New("formulario sin empresa válida")
	}
	key, err := publicFormTokenV273544(r.FormValue("_worktic_submission_id"))
	if err != nil {
		return err
	}
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	claimed, err := tx.Exec(`INSERT INTO automation_form_submissions_v273544(tenant_id,form_id,submission_key,created_at)
          VALUES(?,?,?,?) ON CONFLICT(tenant_id,form_id,submission_key) DO NOTHING`, tid, form.ID, key, stamp)
	if err != nil {
		return err
	}
	claimedRows, err := claimed.RowsAffected()
	if err != nil {
		return err
	}
	if claimedRows == 0 {
		// This exact render has already been submitted. No CRM updates, no event.
		return nil
	}

	name := strings.TrimSpace(r.FormValue("name"))
	phone := strings.TrimSpace(r.FormValue("phone"))
	email := strings.TrimSpace(r.FormValue("email"))
	interest := strings.TrimSpace(r.FormValue("interest"))
	res, err := tx.Exec(`INSERT INTO marketing_leads(tenant_id,form_id,source,name,phone,email,city,interest,score,status,utm_source,utm_campaign,consent,created_at)
          VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, tid, form.ID, "worktic_form", name, phone, email,
		strings.TrimSpace(r.FormValue("city")), interest, 60, "new",
		strings.TrimSpace(r.FormValue("utm_source")), strings.TrimSpace(r.FormValue("utm_campaign")), 1, stamp)
	if err != nil {
		return err
	}
	leadID, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("lead sin identificador: %w", err)
	}
	if leadID <= 0 {
		return errors.New("lead sin identificador")
	}

	eventPayload := publicFormEventPayloadV273544(form, leadID, r, stamp)
	// Only include a landing origin if it is published and linked to this form
	// in the SAME tenant; never trust a browser-provided landing ID alone.
	if landingID, parseErr := strconv.ParseInt(strings.TrimSpace(r.FormValue("wt_landing")), 10, 64); parseErr == nil && landingID > 0 {
		var verifiedLanding, campaignID int64
		if lookupErr := tx.QueryRow(`SELECT id,campaign_id FROM marketing_landings WHERE id=? AND tenant_id=? AND form_id=? AND published=1`, landingID, tid, form.ID).Scan(&verifiedLanding, &campaignID); lookupErr == nil {
			eventPayload["landing_id"] = verifiedLanding
			eventPayload["campaign_id"] = campaignID
		}
	}
	payload, err := json.Marshal(eventPayload)
	if err != nil {
		return err
	}
	event, err := tx.Exec(`INSERT INTO automation_events(tenant_id,event_type,source,payload_json,status,created_at)
          VALUES(?,'form.submitted','worktic_form',?,'awaiting_crm',?)`, tid, string(payload), stamp)
	if err != nil {
		return err
	}
	eventID, err := event.LastInsertId()
	if err != nil {
		return fmt.Errorf("evento sin identificador: %w", err)
	}
	if eventID <= 0 {
		return errors.New("evento sin identificador")
	}
	_, err = tx.Exec(`UPDATE automation_form_submissions_v273544 SET lead_id=?,event_id=? WHERE tenant_id=? AND form_id=? AND submission_key=?`, leadID, eventID, tid, form.ID, key)
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	// Legacy CRM integrations manage their own transactions and quotas. They
	// happen after the durable lead/event commit; no external effects are retried
	// automatically if this process is interrupted or a step fails.
	a.applyLeadAttributionFromRequestV24(tid, leadID, r)
	contactErr := a.syncCRMContactAt(tid, name, phone, email, "landing", "form", "", stamp)
	var opportunityErr error
	if contactErr == nil {
		opportunityErr = a.syncOpportunityFromLead(tid, leadID, name, phone, email, "landing", "form", interest, 60, stamp)
	}
	if contactErr != nil || opportunityErr != nil {
		log.Printf("form automation CRM sync requires review: tenant=%d form=%d lead=%d", tid, form.ID, leadID)
		_, updateErr := a.db.Exec(`UPDATE automation_events SET status='needs_review',last_error=?
            WHERE id=? AND tenant_id=? AND status='awaiting_crm'`, "No se completó la sincronización CRM; revisar sin repetir envíos", eventID, tid)
		if updateErr != nil {
			log.Printf("form automation review marker error: %v", updateErr)
		}
		return nil // Lead was committed: don't encourage duplicate submissions.
	}
	result, err := a.db.Exec(`UPDATE automation_events SET status='pending'
        WHERE id=? AND tenant_id=? AND status='awaiting_crm'`, eventID, tid)
	if err != nil {
		log.Printf("form automation release event error: %v", err)
		return nil
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		log.Printf("form automation event not released: tenant=%d event=%d", tid, eventID)
	}
	return nil
}

// If the process crashed between durable form submission and CRM sync, an
// operator must inspect it: blindly retrying could duplicate CRM side effects.
func (a *App) reviewStalePublicFormEventsV273544() {
	cutoff := time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339)
	_, err := a.db.Exec(`UPDATE automation_events SET status='needs_review',last_error=?
      WHERE event_type='form.submitted' AND source='worktic_form' AND status='awaiting_crm' AND created_at<?`,
		"Envío guardado pero sincronización CRM interrumpida; revisión manual necesaria", cutoff)
	if err != nil {
		log.Printf("form automation stale review: %v", err)
	}
}
