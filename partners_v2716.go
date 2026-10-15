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

// V27.16 adds WorkticAI-assigned professional service opportunities for
// Activate Certified partners. These are intentionally separate from CRM
// opportunities and from the Human Sales marketplace: CRM remains the sales
// source of truth, while this table represents work commissioned by WorkticAI.
func initPartnersV2716Schema(db *DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS partner_service_assignments_v2716 (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 specialist_partner_user_id INTEGER NOT NULL,
 client_user_id INTEGER NOT NULL DEFAULT 0,
 client_name TEXT NOT NULL DEFAULT '',
 client_email TEXT NOT NULL DEFAULT '',
 client_company TEXT NOT NULL DEFAULT '',
 assignment_type TEXT NOT NULL DEFAULT 'implementation',
 title TEXT NOT NULL,
 description TEXT NOT NULL DEFAULT '',
 scope TEXT NOT NULL DEFAULT '',
 deliverables TEXT NOT NULL DEFAULT '',
 compensation_mode TEXT NOT NULL DEFAULT 'fixed',
 fee_amount REAL NOT NULL DEFAULT 0,
 commission_percent REAL NOT NULL DEFAULT 0,
 specialist_share_percent REAL NOT NULL DEFAULT 80,
 platform_share_percent REAL NOT NULL DEFAULT 20,
 currency TEXT NOT NULL DEFAULT 'USD',
 priority TEXT NOT NULL DEFAULT 'normal',
 due_at TEXT NOT NULL DEFAULT '',
 status TEXT NOT NULL DEFAULT 'assigned',
 admin_note TEXT NOT NULL DEFAULT '',
 specialist_note TEXT NOT NULL DEFAULT '',
 assigned_by INTEGER NOT NULL DEFAULT 0,
 ledger_id INTEGER NOT NULL DEFAULT 0,
 created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL,
 accepted_at TEXT NOT NULL DEFAULT '',
 rejected_at TEXT NOT NULL DEFAULT '',
 started_at TEXT NOT NULL DEFAULT '',
 submitted_at TEXT NOT NULL DEFAULT '',
 completed_at TEXT NOT NULL DEFAULT '',
 cancelled_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_partner_service_assignments_partner_v2716 ON partner_service_assignments_v2716(specialist_partner_user_id,status,created_at DESC);
CREATE INDEX IF NOT EXISTS idx_partner_service_assignments_status_v2716 ON partner_service_assignments_v2716(status,priority,created_at DESC);
`)
	return err
}

func partnerAssignmentTypeLabelV2716(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "implementation":
		return "Implementación WorkticAI"
	case "marketing_setup":
		return "Personalización de marketing"
	case "managed_marketing":
		return "Acompañamiento de marketing"
	case "optimization":
		return "Optimización y Growth"
	case "training":
		return "Capacitación / onboarding"
	case "human_sales":
		return "Human Sales"
	default:
		return "Proyecto WorkticAI"
	}
}

func validPartnerAssignmentTypeV2716(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "implementation", "marketing_setup", "managed_marketing", "optimization", "training", "human_sales":
		return true
	default:
		return false
	}
}

func validPartnerAssignmentPriorityV2716(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "low", "normal", "high", "urgent":
		return true
	default:
		return false
	}
}

func (a *App) partnerIsActivateCertifiedV2716(userID int64) bool {
	if userID <= 0 {
		return false
	}
	var status string
	if err := a.db.QueryRow(`SELECT activate_status FROM partner_profiles_v2714 WHERE user_id=? AND status='active'`, userID).Scan(&status); err != nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(status), "certified")
}

func (a *App) partnerAssignmentCountsV2716(userID int64) (assigned, active, submitted, completed int) {
	if userID <= 0 {
		return
	}
	_ = a.db.QueryRow(`SELECT
 COALESCE(SUM(CASE WHEN status='assigned' THEN 1 ELSE 0 END),0),
 COALESCE(SUM(CASE WHEN status='in_progress' THEN 1 ELSE 0 END),0),
 COALESCE(SUM(CASE WHEN status='submitted' THEN 1 ELSE 0 END),0),
 COALESCE(SUM(CASE WHEN status='completed' THEN 1 ELSE 0 END),0)
 FROM partner_service_assignments_v2716 WHERE specialist_partner_user_id=?`, userID).Scan(&assigned, &active, &submitted, &completed)
	return
}

func (a *App) partnerAssignmentsHandlerV2716(w http.ResponseWriter, r *http.Request) {
	u := a.currentUser(r)
	if u == nil {
		writeError(w, errors.New("sesión requerida"), http.StatusUnauthorized)
		return
	}
	switch r.Method {
	case http.MethodGet:
		rows, err := a.db.Query(`SELECT id,specialist_partner_user_id,client_user_id,client_name,client_email,client_company,assignment_type,title,description,scope,deliverables,compensation_mode,fee_amount,commission_percent,specialist_share_percent,platform_share_percent,currency,priority,due_at,status,admin_note,specialist_note,assigned_by,ledger_id,created_at,updated_at,accepted_at,rejected_at,started_at,submitted_at,completed_at,cancelled_at FROM partner_service_assignments_v2716 WHERE specialist_partner_user_id=? ORDER BY CASE status WHEN 'assigned' THEN 0 WHEN 'in_progress' THEN 1 WHEN 'submitted' THEN 2 WHEN 'completed' THEN 3 ELSE 4 END,id DESC LIMIT 250`, u.ID)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		defer rows.Close()
		items := []map[string]any{}
		for rows.Next() {
			var id, specialistID, clientUserID, assignedBy, ledgerID int64
			var clientName, clientEmail, clientCompany, assignmentType, title, description, scope, deliverables, compensationMode, currency, priority, dueAt, status, adminNote, specialistNote string
			var createdAt, updatedAt, acceptedAt, rejectedAt, startedAt, submittedAt, completedAt, cancelledAt string
			var feeAmount, commissionPercent, specialistShare, platformShare float64
			if rows.Scan(&id, &specialistID, &clientUserID, &clientName, &clientEmail, &clientCompany, &assignmentType, &title, &description, &scope, &deliverables, &compensationMode, &feeAmount, &commissionPercent, &specialistShare, &platformShare, &currency, &priority, &dueAt, &status, &adminNote, &specialistNote, &assignedBy, &ledgerID, &createdAt, &updatedAt, &acceptedAt, &rejectedAt, &startedAt, &submittedAt, &completedAt, &cancelledAt) == nil {
				items = append(items, map[string]any{
					"id": id, "specialist_partner_user_id": specialistID, "client_user_id": clientUserID,
					"client_name": clientName, "client_email": clientEmail, "client_company": clientCompany,
					"assignment_type": assignmentType, "assignment_type_label": partnerAssignmentTypeLabelV2716(assignmentType),
					"title": title, "description": description, "scope": scope, "deliverables": deliverables,
					"compensation_mode": compensationMode, "fee_amount": feeAmount, "commission_percent": commissionPercent,
					"specialist_share_percent": specialistShare, "platform_share_percent": platformShare,
					"currency": currency, "priority": priority, "due_at": dueAt, "status": status,
					"admin_note": adminNote, "specialist_note": specialistNote, "assigned_by": assignedBy, "ledger_id": ledgerID,
					"created_at": createdAt, "updated_at": updatedAt, "accepted_at": acceptedAt, "rejected_at": rejectedAt,
					"started_at": startedAt, "submitted_at": submittedAt, "completed_at": completedAt, "cancelled_at": cancelledAt,
				})
			}
		}
		assigned, active, submitted, completed := a.partnerAssignmentCountsV2716(u.ID)
		writeJSON(w, map[string]any{"items": items, "counts": map[string]any{"assigned": assigned, "active": active, "submitted": submitted, "completed": completed}, "certified": a.partnerIsActivateCertifiedV2716(u.ID)})

	case http.MethodPut:
		var q struct {
			ID     int64  `json:"id"`
			Action string `json:"action"`
			Note   string `json:"note"`
		}
		if json.NewDecoder(r.Body).Decode(&q) != nil || q.ID <= 0 {
			writeError(w, errors.New("asignación inválida"), 400)
			return
		}
		var specialistID int64
		var status string
		if err := a.db.QueryRow(`SELECT specialist_partner_user_id,status FROM partner_service_assignments_v2716 WHERE id=?`, q.ID).Scan(&specialistID, &status); err != nil || specialistID != u.ID {
			writeError(w, errors.New("oportunidad no encontrada"), 404)
			return
		}
		now := time.Now().UTC().Format(time.RFC3339)
		action := strings.ToLower(strings.TrimSpace(q.Action))
		switch action {
		case "accept":
			if status != "assigned" {
				writeError(w, errors.New("esta oportunidad ya fue respondida"), 400)
				return
			}
			if !a.partnerIsActivateCertifiedV2716(u.ID) || !a.partnerEarningEligibleV2715(u.ID) {
				writeError(w, errors.New("necesitas Activate Certified y una licencia WorkticAI vigente para aceptar proyectos"), 403)
				return
			}
			_, err := a.db.Exec(`UPDATE partner_service_assignments_v2716 SET status='in_progress',specialist_note=?,accepted_at=?,started_at=?,updated_at=? WHERE id=? AND specialist_partner_user_id=? AND status='assigned'`, strings.TrimSpace(q.Note), now, now, now, q.ID, u.ID)
			if err != nil {
				writeError(w, err, 500)
				return
			}
			a.partnerAuditV2714(u.ID, u.ID, "service_assignment.accepted", map[string]any{"assignment_id": q.ID})
			writeJSON(w, map[string]any{"ok": true, "status": "in_progress"})
		case "reject":
			if status != "assigned" {
				writeError(w, errors.New("esta oportunidad ya fue respondida"), 400)
				return
			}
			_, err := a.db.Exec(`UPDATE partner_service_assignments_v2716 SET status='rejected',specialist_note=?,rejected_at=?,updated_at=? WHERE id=? AND specialist_partner_user_id=? AND status='assigned'`, strings.TrimSpace(q.Note), now, now, q.ID, u.ID)
			if err != nil {
				writeError(w, err, 500)
				return
			}
			a.partnerAuditV2714(u.ID, u.ID, "service_assignment.rejected", map[string]any{"assignment_id": q.ID, "note": strings.TrimSpace(q.Note)})
			writeJSON(w, map[string]any{"ok": true, "status": "rejected"})
		case "submit":
			if status != "in_progress" {
				writeError(w, errors.New("solo puedes entregar un proyecto que esté en ejecución"), 400)
				return
			}
			if strings.TrimSpace(q.Note) == "" {
				writeError(w, errors.New("incluye una nota de entrega para WorkticAI"), 400)
				return
			}
			_, err := a.db.Exec(`UPDATE partner_service_assignments_v2716 SET status='submitted',specialist_note=?,submitted_at=?,updated_at=? WHERE id=? AND specialist_partner_user_id=? AND status='in_progress'`, strings.TrimSpace(q.Note), now, now, q.ID, u.ID)
			if err != nil {
				writeError(w, err, 500)
				return
			}
			a.partnerAuditV2714(u.ID, u.ID, "service_assignment.submitted", map[string]any{"assignment_id": q.ID})
			writeJSON(w, map[string]any{"ok": true, "status": "submitted"})
		default:
			writeError(w, errors.New("acción no soportada"), 400)
		}
	default:
		http.Error(w, "Método no permitido", 405)
	}
}

func (a *App) adminPartnerAssignmentsHandlerV2716(w http.ResponseWriter, r *http.Request) {
	admin, ok := a.requireSuperadmin(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		items, err := a.adminPartnerAssignmentItemsV2716()
		if err != nil {
			writeError(w, err, 500)
			return
		}
		rows, err := a.db.Query(`SELECT p.user_id,u.name,u.email,p.partner_code,p.rank_code,p.activate_status FROM partner_profiles_v2714 p JOIN app_users u ON u.id=p.user_id WHERE p.status='active' AND p.activate_status='certified' ORDER BY lower(u.name),u.id`)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		defer rows.Close()
		specialists := []map[string]any{}
		for rows.Next() {
			var uid int64
			var name, email, code, rank, cert string
			if rows.Scan(&uid, &name, &email, &code, &rank, &cert) == nil {
				specialists = append(specialists, map[string]any{"user_id": uid, "name": name, "email": email, "partner_code": code, "rank_code": rank, "activate_status": cert, "eligible": a.partnerEarningEligibleV2715(uid)})
			}
		}
		writeJSON(w, map[string]any{"items": items, "specialists": specialists})

	case http.MethodPost:
		var q struct {
			SpecialistPartnerUserID int64   `json:"specialist_partner_user_id"`
			ClientUserID            int64   `json:"client_user_id"`
			ClientName              string  `json:"client_name"`
			ClientEmail             string  `json:"client_email"`
			ClientCompany           string  `json:"client_company"`
			AssignmentType          string  `json:"assignment_type"`
			Title                   string  `json:"title"`
			Description             string  `json:"description"`
			Scope                   string  `json:"scope"`
			Deliverables            string  `json:"deliverables"`
			CompensationMode        string  `json:"compensation_mode"`
			FeeAmount               float64 `json:"fee_amount"`
			CommissionPercent       float64 `json:"commission_percent"`
			Currency                string  `json:"currency"`
			Priority                string  `json:"priority"`
			DueAt                   string  `json:"due_at"`
			AdminNote               string  `json:"admin_note"`
		}
		if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
			writeError(w, errors.New("datos inválidos"), 400)
			return
		}
		if q.SpecialistPartnerUserID <= 0 || strings.TrimSpace(q.Title) == "" {
			writeError(w, errors.New("selecciona un especialista y escribe el título del proyecto"), 400)
			return
		}
		q.AssignmentType = strings.ToLower(strings.TrimSpace(q.AssignmentType))
		if !validPartnerAssignmentTypeV2716(q.AssignmentType) {
			writeError(w, errors.New("tipo de proyecto inválido"), 400)
			return
		}
		q.Priority = strings.ToLower(strings.TrimSpace(q.Priority))
		if !validPartnerAssignmentPriorityV2716(q.Priority) {
			q.Priority = "normal"
		}
		q.CompensationMode = strings.ToLower(strings.TrimSpace(q.CompensationMode))
		if q.CompensationMode == "" {
			q.CompensationMode = "fixed"
		}
		if q.CompensationMode != "fixed" && q.CompensationMode != "sales_commission" {
			writeError(w, errors.New("modelo de pago inválido"), 400)
			return
		}
		if q.CompensationMode == "fixed" && q.FeeAmount <= 0 {
			writeError(w, errors.New("define el pago del proyecto"), 400)
			return
		}
		if q.CompensationMode == "sales_commission" && q.CommissionPercent <= 0 {
			writeError(w, errors.New("define el porcentaje de comisión comercial"), 400)
			return
		}
		if !a.partnerIsActivateCertifiedV2716(q.SpecialistPartnerUserID) || !a.partnerEarningEligibleV2715(q.SpecialistPartnerUserID) {
			writeError(w, errors.New("el especialista debe estar activo, validado, con Activate Certified y licencia WorkticAI vigente"), 400)
			return
		}
		q.Currency = strings.ToUpper(strings.TrimSpace(q.Currency))
		if q.Currency == "" {
			q.Currency = "USD"
		}
		specialistShare := a.partnerSettingFloatV2714("human_sales_specialist_percent", 80)
		platformShare := a.partnerSettingFloatV2714("human_sales_platform_percent", 20)
		now := time.Now().UTC().Format(time.RFC3339)
		res, err := a.db.Exec(`INSERT INTO partner_service_assignments_v2716(specialist_partner_user_id,client_user_id,client_name,client_email,client_company,assignment_type,title,description,scope,deliverables,compensation_mode,fee_amount,commission_percent,specialist_share_percent,platform_share_percent,currency,priority,due_at,status,admin_note,assigned_by,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,'assigned',?,?,?,?)`, q.SpecialistPartnerUserID, q.ClientUserID, strings.TrimSpace(q.ClientName), strings.ToLower(strings.TrimSpace(q.ClientEmail)), strings.TrimSpace(q.ClientCompany), q.AssignmentType, strings.TrimSpace(q.Title), strings.TrimSpace(q.Description), strings.TrimSpace(q.Scope), strings.TrimSpace(q.Deliverables), q.CompensationMode, roundUSDT(q.FeeAmount), q.CommissionPercent, specialistShare, platformShare, q.Currency, q.Priority, strings.TrimSpace(q.DueAt), strings.TrimSpace(q.AdminNote), admin.ID, now, now)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		id, _ := res.LastInsertId()
		a.partnerAuditV2714(admin.ID, q.SpecialistPartnerUserID, "service_assignment.created", map[string]any{"assignment_id": id, "type": q.AssignmentType, "fee_amount": q.FeeAmount, "compensation_mode": q.CompensationMode})
		writeJSON(w, map[string]any{"ok": true, "id": id})

	case http.MethodPut:
		var q struct {
			ID                      int64  `json:"id"`
			Action                  string `json:"action"`
			Note                    string `json:"note"`
			SpecialistPartnerUserID int64  `json:"specialist_partner_user_id"`
		}
		if json.NewDecoder(r.Body).Decode(&q) != nil || q.ID <= 0 {
			writeError(w, errors.New("asignación inválida"), 400)
			return
		}
		var specialistID, ledgerID int64
		var status, compensationMode, title, clientCompany, currency string
		var feeAmount, commissionPercent, specialistShare, platformShare float64
		if err := a.db.QueryRow(`SELECT specialist_partner_user_id,status,compensation_mode,fee_amount,commission_percent,specialist_share_percent,platform_share_percent,title,client_company,currency,ledger_id FROM partner_service_assignments_v2716 WHERE id=?`, q.ID).Scan(&specialistID, &status, &compensationMode, &feeAmount, &commissionPercent, &specialistShare, &platformShare, &title, &clientCompany, &currency, &ledgerID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, errors.New("asignación no encontrada"), 404)
			} else {
				writeError(w, err, 500)
			}
			return
		}
		now := time.Now().UTC().Format(time.RFC3339)
		action := strings.ToLower(strings.TrimSpace(q.Action))
		switch action {
		case "complete":
			if status != "submitted" && status != "in_progress" {
				writeError(w, errors.New("solo puede completarse una tarea en ejecución o entregada"), 400)
				return
			}
			if compensationMode == "fixed" && feeAmount > 0 && ledgerID == 0 {
				meta := map[string]any{"assignment_id": q.ID, "title": title, "client_company": clientCompany, "source": "worktic_service_assignment"}
				newLedgerID, err := a.addPartnerAmountMovementV2715(specialistID, "service_assignment", fmt.Sprintf("service_assignment:%d", q.ID), feeAmount, "available", meta)
				if err != nil {
					writeError(w, err, 500)
					return
				}
				ledgerID = newLedgerID
			}
			_, err := a.db.Exec(`UPDATE partner_service_assignments_v2716 SET status='completed',admin_note=?,ledger_id=?,completed_at=?,updated_at=? WHERE id=?`, strings.TrimSpace(q.Note), ledgerID, now, now, q.ID)
			if err != nil {
				writeError(w, err, 500)
				return
			}
			a.partnerAuditV2714(admin.ID, specialistID, "service_assignment.completed", map[string]any{"assignment_id": q.ID, "ledger_id": ledgerID, "fee_amount": feeAmount, "compensation_mode": compensationMode})
			writeJSON(w, map[string]any{"ok": true, "status": "completed", "ledger_id": ledgerID})
		case "cancel":
			if status == "completed" || status == "cancelled" {
				writeError(w, errors.New("la asignación ya está cerrada"), 400)
				return
			}
			_, err := a.db.Exec(`UPDATE partner_service_assignments_v2716 SET status='cancelled',admin_note=?,cancelled_at=?,updated_at=? WHERE id=?`, strings.TrimSpace(q.Note), now, now, q.ID)
			if err != nil {
				writeError(w, err, 500)
				return
			}
			a.partnerAuditV2714(admin.ID, specialistID, "service_assignment.cancelled", map[string]any{"assignment_id": q.ID, "note": strings.TrimSpace(q.Note)})
			writeJSON(w, map[string]any{"ok": true, "status": "cancelled"})
		case "reassign":
			if status == "completed" || status == "cancelled" {
				writeError(w, errors.New("la asignación ya está cerrada"), 400)
				return
			}
			if q.SpecialistPartnerUserID <= 0 || !a.partnerIsActivateCertifiedV2716(q.SpecialistPartnerUserID) || !a.partnerEarningEligibleV2715(q.SpecialistPartnerUserID) {
				writeError(w, errors.New("selecciona un especialista Activate Certified elegible"), 400)
				return
			}
			_, err := a.db.Exec(`UPDATE partner_service_assignments_v2716 SET specialist_partner_user_id=?,status='assigned',specialist_note='',accepted_at='',rejected_at='',started_at='',submitted_at='',admin_note=?,updated_at=? WHERE id=?`, q.SpecialistPartnerUserID, strings.TrimSpace(q.Note), now, q.ID)
			if err != nil {
				writeError(w, err, 500)
				return
			}
			a.partnerAuditV2714(admin.ID, q.SpecialistPartnerUserID, "service_assignment.reassigned", map[string]any{"assignment_id": q.ID, "previous_specialist": specialistID})
			writeJSON(w, map[string]any{"ok": true, "status": "assigned"})
		default:
			writeError(w, errors.New("acción no soportada"), 400)
		}
	default:
		http.Error(w, "Método no permitido", 405)
	}
}

func (a *App) adminPartnerAssignmentItemsV2716() ([]map[string]any, error) {
	rows, err := a.db.Query(`SELECT a.id,a.specialist_partner_user_id,u.name,u.email,p.partner_code,p.rank_code,a.client_user_id,a.client_name,a.client_email,a.client_company,a.assignment_type,a.title,a.description,a.scope,a.deliverables,a.compensation_mode,a.fee_amount,a.commission_percent,a.specialist_share_percent,a.platform_share_percent,a.currency,a.priority,a.due_at,a.status,a.admin_note,a.specialist_note,a.assigned_by,COALESCE(au.name,''),a.ledger_id,a.created_at,a.updated_at,a.accepted_at,a.rejected_at,a.started_at,a.submitted_at,a.completed_at,a.cancelled_at FROM partner_service_assignments_v2716 a JOIN app_users u ON u.id=a.specialist_partner_user_id LEFT JOIN partner_profiles_v2714 p ON p.user_id=a.specialist_partner_user_id LEFT JOIN app_users au ON au.id=a.assigned_by ORDER BY CASE a.status WHEN 'assigned' THEN 0 WHEN 'submitted' THEN 1 WHEN 'in_progress' THEN 2 WHEN 'completed' THEN 3 ELSE 4 END,a.id DESC LIMIT 500`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, specialistID, clientUserID, assignedBy, ledgerID int64
		var specialistName, specialistEmail, partnerCode, rankCode, clientName, clientEmail, clientCompany, assignmentType, title, description, scope, deliverables, compensationMode, currency, priority, dueAt, status, adminNote, specialistNote, assignedByName string
		var createdAt, updatedAt, acceptedAt, rejectedAt, startedAt, submittedAt, completedAt, cancelledAt string
		var feeAmount, commissionPercent, specialistShare, platformShare float64
		if rows.Scan(&id, &specialistID, &specialistName, &specialistEmail, &partnerCode, &rankCode, &clientUserID, &clientName, &clientEmail, &clientCompany, &assignmentType, &title, &description, &scope, &deliverables, &compensationMode, &feeAmount, &commissionPercent, &specialistShare, &platformShare, &currency, &priority, &dueAt, &status, &adminNote, &specialistNote, &assignedBy, &assignedByName, &ledgerID, &createdAt, &updatedAt, &acceptedAt, &rejectedAt, &startedAt, &submittedAt, &completedAt, &cancelledAt) == nil {
			items = append(items, map[string]any{
				"id": id, "specialist_partner_user_id": specialistID, "specialist_name": specialistName, "specialist_email": specialistEmail, "partner_code": partnerCode, "rank_code": rankCode,
				"client_user_id": clientUserID, "client_name": clientName, "client_email": clientEmail, "client_company": clientCompany,
				"assignment_type": assignmentType, "assignment_type_label": partnerAssignmentTypeLabelV2716(assignmentType), "title": title, "description": description, "scope": scope, "deliverables": deliverables,
				"compensation_mode": compensationMode, "fee_amount": feeAmount, "commission_percent": commissionPercent, "specialist_share_percent": specialistShare, "platform_share_percent": platformShare,
				"currency": currency, "priority": priority, "due_at": dueAt, "status": status, "admin_note": adminNote, "specialist_note": specialistNote,
				"assigned_by": assignedBy, "assigned_by_name": assignedByName, "ledger_id": ledgerID, "created_at": createdAt, "updated_at": updatedAt,
				"accepted_at": acceptedAt, "rejected_at": rejectedAt, "started_at": startedAt, "submitted_at": submittedAt, "completed_at": completedAt, "cancelled_at": cancelledAt,
			})
		}
	}
	return items, nil
}

func (a *App) adminPartnerAssignmentSummaryV2716() map[string]any {
	var assigned, inProgress, submitted, completed int
	var committed, completedValue float64
	_ = a.db.QueryRow(`SELECT
 COALESCE(SUM(CASE WHEN status='assigned' THEN 1 ELSE 0 END),0),
 COALESCE(SUM(CASE WHEN status='in_progress' THEN 1 ELSE 0 END),0),
 COALESCE(SUM(CASE WHEN status='submitted' THEN 1 ELSE 0 END),0),
 COALESCE(SUM(CASE WHEN status='completed' THEN 1 ELSE 0 END),0),
 COALESCE(SUM(CASE WHEN status IN ('assigned','in_progress','submitted') AND compensation_mode='fixed' THEN fee_amount ELSE 0 END),0),
 COALESCE(SUM(CASE WHEN status='completed' AND compensation_mode='fixed' THEN fee_amount ELSE 0 END),0)
 FROM partner_service_assignments_v2716`).Scan(&assigned, &inProgress, &submitted, &completed, &committed, &completedValue)
	return map[string]any{"assigned": assigned, "in_progress": inProgress, "submitted": submitted, "completed": completed, "open": assigned + inProgress + submitted, "committed_value": roundUSDT(committed), "completed_value": roundUSDT(completedValue)}
}

// Helper used by the frontend's expanded earning-path explanation.
func (a *App) partnerProgramEconomicsV2716() map[string]any {
	return map[string]any{
		"adoption_percent":               a.partnerSettingFloatV2714("adoption_percent", 4),
		"renewal_percent":                a.partnerSettingFloatV2714("renewal_percent", 2),
		"mentorship_percent":             a.partnerSettingFloatV2714("mentorship_percent", 2),
		"mentorship_min_rank":            a.partnerSettingTextV2715("mentorship_min_rank", "apex"),
		"human_sales_specialist_percent": a.partnerSettingFloatV2714("human_sales_specialist_percent", 80),
		"human_sales_platform_percent":   a.partnerSettingFloatV2714("human_sales_platform_percent", 20),
	}
}

// parseAssignmentIDV2716 is kept small so admin tools can link directly to an
// assignment in future without changing the route contract.
func parseAssignmentIDV2716(v string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	return n
}
