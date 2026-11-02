package main

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// V27.35.45 — CRM Sales Hub is the single source of tasks/owners.
// A receipt and the business write share one DB transaction. This protects
// against a repeated node inside the same execution, not against all possible
// external duplicates: delivery claiming remains a separate safety layer.
func initAutomationSalesHubV273545Schema(db *DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS automation_sales_hub_effects_v273545(
        tenant_id INTEGER NOT NULL,
        execution_id INTEGER NOT NULL,
        node_id TEXT NOT NULL,
        effect_type TEXT NOT NULL,
        record_id INTEGER NOT NULL,
        created_at TEXT NOT NULL,
        PRIMARY KEY(tenant_id,execution_id,node_id,effect_type)
    );`)
	return err
}

func automationTaskDueHoursV273545(config map[string]any) (int64, error) {
	if config == nil {
		return 0, errors.New("indica el vencimiento de la tarea")
	}
	raw, ok := config["due_hours"]
	if !ok || raw == nil {
		return 0, errors.New("indica el vencimiento de la tarea")
	}
	hours, err := strconv.ParseInt(strings.TrimSpace(fmt.Sprint(raw)), 10, 64)
	if err != nil || hours < 1 || hours > 720 {
		return 0, errors.New("el vencimiento debe ser de 1 a 720 horas")
	}
	return hours, nil
}

func automationOptionalOwnerIDV273545(config map[string]any, required bool) (int64, error) {
	raw := cfgString(config, "owner_user_id")
	if raw == "" && !required {
		return 0, nil
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("selecciona un responsable válido de la empresa")
	}
	return id, nil
}

func validateAutomationTaskSettingsV273545(config map[string]any) error {
	title := cfgString(config, "title")
	if len([]rune(title)) < 3 || len([]rune(title)) > 180 {
		return errors.New("el título debe contener entre 3 y 180 caracteres")
	}
	if _, err := automationTaskDueHoursV273545(config); err != nil {
		return err
	}
	if _, err := automationOptionalOwnerIDV273545(config, false); err != nil {
		return err
	}
	return nil
}

func (a *App) validateAutomationSalesHubReferencesV273545(tid int64, graph automationGraph) error {
	for _, n := range graph.Nodes {
		if n.Type != "create_task" && n.Type != "assign_owner" {
			continue
		}
		owner, err := automationOptionalOwnerIDV273545(n.Config, n.Type == "assign_owner")
		if err != nil {
			return fmt.Errorf("%s: %w", n.Label, err)
		}
		if owner > 0 && !salesHubUserExistsV273518(a.db, tid, owner) {
			return fmt.Errorf("%s: el responsable está inactivo o pertenece a otra empresa", n.Label)
		}
		oid := cfgInt64(n.Config, "opportunity_id")
		if oid > 0 && !salesHubOpportunityExistsV273518(a.db, tid, oid) {
			return fmt.Errorf("%s: oportunidad fuera de la empresa o eliminada", n.Label)
		}
	}
	return nil
}

func (a *App) automationSalesHubOpportunityIDV273545(tid int64, config map[string]any, state map[string]any) (int64, error) {
	oid := cfgInt64(config, "opportunity_id")
	if oid <= 0 {
		oid, _ = strconv.ParseInt(payloadString(state, "opportunity_id"), 10, 64)
	}
	if oid <= 0 {
		lead := payloadString(state, "lead_id")
		if lead != "" {
			if err := a.db.QueryRow(`SELECT id FROM crm_opportunities WHERE tenant_id=? AND source_ref=? AND COALESCE(deleted_at,'')='' ORDER BY id DESC LIMIT 1`, tid, "lead:"+lead).Scan(&oid); err != nil && err != sql.ErrNoRows {
				return 0, err
			}
		}
	}
	if oid <= 0 {
		return 0, errors.New("el evento no identifica una oportunidad; utiliza un disparador de oportunidad o vincula el lead al CRM")
	}
	return oid, nil
}

// Caller owns an open transaction. We do not allow automations to modify
// opportunity editing by automation respects manual_lock. Creating a follow-up
// task is allowed on a manually locked opportunity, as in Sales Hub itself.
func automationSalesHubCheckOpportunityTxV273545(tx *Tx, tid, oid int64, allowManualLock bool) error {
	query := `SELECT id FROM crm_opportunities WHERE id=? AND tenant_id=? AND COALESCE(deleted_at,'')=''`
	if !allowManualLock {
		query += ` AND COALESCE(manual_lock,0)=0`
	}
	var id int64
	err := tx.QueryRow(query, oid, tid).Scan(&id)
	if err == sql.ErrNoRows {
		return errors.New("oportunidad no encontrada, eliminada o protegida contra modificaciones automáticas")
	}
	return err
}

func automationSalesHubEffectIDV273545(state map[string]any) (int64, error) {
	const maxJSONSafeInteger = int64(9007199254740991)
	var id int64
	switch value := state["_automation_execution_id"].(type) {
	case int64:
		id = value
	case int:
		id = int64(value)
	case string:
		id, _ = strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	case float64:
		// Stored waits are decoded by encoding/json to float64. Converting via
		// fmt.Sprint can produce scientific notation for IDs >= 1,000,000.
		if value > 0 && value <= float64(maxJSONSafeInteger) && value == float64(int64(value)) {
			id = int64(value)
		}
	}
	if id <= 0 || id > maxJSONSafeInteger {
		return 0, errors.New("falta un identificador de ejecución válido; acción no realizada")
	}
	return id, nil
}

func automationSalesHubPriorEffectV273545(tx *Tx, tid, eid int64, nodeID, kind string) (int64, bool, error) {
	var recordID int64
	err := tx.QueryRow(`SELECT record_id FROM automation_sales_hub_effects_v273545 WHERE tenant_id=? AND execution_id=? AND node_id=? AND effect_type=?`, tid, eid, nodeID, kind).Scan(&recordID)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return recordID, true, nil
}

func (a *App) createAutomationSalesHubTaskV273545(tid int64, n automationNode, state map[string]any) (map[string]any, error) {
	if err := validateAutomationTaskSettingsV273545(n.Config); err != nil {
		return nil, err
	}
	eid, err := automationSalesHubEffectIDV273545(state)
	if err != nil {
		return nil, err
	}
	oid, err := a.automationSalesHubOpportunityIDV273545(tid, n.Config, state)
	if err != nil {
		return nil, err
	}
	title, err := expandAutomationMessageV273534(cfgString(n.Config, "title"), state)
	if err != nil {
		return nil, err
	}
	title = strings.TrimSpace(title)
	if len([]rune(title)) < 3 || len([]rune(title)) > 180 {
		return nil, errors.New("título de tarea fuera del rango permitido")
	}
	hours, err := automationTaskDueHoursV273545(n.Config)
	if err != nil {
		return nil, err
	}
	selectedOwner, err := automationOptionalOwnerIDV273545(n.Config, false)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	stamp := now.Format(time.RFC3339)
	due := now.Add(time.Duration(hours) * time.Hour).Format(time.RFC3339)
	tx, err := a.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	prior, exists, err := automationSalesHubPriorEffectV273545(tx, tid, eid, n.ID, "task.create")
	if err != nil {
		return nil, err
	}
	if exists {
		return map[string]any{"task_id": prior, "opportunity_id": oid, "already_recorded": true}, nil
	}
	if err := automationSalesHubCheckOpportunityTxV273545(tx, tid, oid, true); err != nil {
		return nil, err
	}
	owner := selectedOwner
	if owner == 0 {
		err = tx.QueryRow(`SELECT owner_user_id FROM opportunity_hub_meta_v273518 WHERE tenant_id=? AND opportunity_id=?`, tid, oid).Scan(&owner)
		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}
	}
	if owner <= 0 {
		return nil, errors.New("asigna primero un responsable a la oportunidad o elige uno en el bloque de tarea")
	}
	if !salesHubUserExistsTxV273518(tx, tid, owner) {
		return nil, errors.New("responsable inactivo o fuera de esta empresa")
	}
	res, err := tx.Exec(`INSERT INTO opportunity_hub_tasks_v273518(tenant_id,opportunity_id,owner_user_id,title,due_at,status,created_by,updated_by,created_at,updated_at) VALUES(?,?,?,?,?,'open',0,0,?,?)`, tid, oid, owner, title, due, stamp, stamp)
	if err != nil {
		return nil, err
	}
	taskID, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	if err = salesHubAuditTxV273518(tx, tid, oid, 0, "task.created.automation", fmt.Sprintf("execution=%d node=%s task=%d due=%s", eid, n.ID, taskID, due)); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(`INSERT INTO automation_sales_hub_effects_v273545(tenant_id,execution_id,node_id,effect_type,record_id,created_at) VALUES(?,?,?,?,?,?)`, tid, eid, n.ID, "task.create", taskID, stamp); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("resultado de creación no confirmado; revisar antes de repetir: %w", err)
	}
	return map[string]any{"task_id": taskID, "opportunity_id": oid, "owner_user_id": owner, "due_at": due, "created": true}, nil
}

func (a *App) assignAutomationSalesHubOwnerV273545(tid int64, n automationNode, state map[string]any) (map[string]any, error) {
	eid, err := automationSalesHubEffectIDV273545(state)
	if err != nil {
		return nil, err
	}
	oid, err := a.automationSalesHubOpportunityIDV273545(tid, n.Config, state)
	if err != nil {
		return nil, err
	}
	ownerID, err := automationOptionalOwnerIDV273545(n.Config, true)
	if err != nil {
		return nil, err
	}
	tx, err := a.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	prior, exists, err := automationSalesHubPriorEffectV273545(tx, tid, eid, n.ID, "owner.assign")
	if err != nil {
		return nil, err
	}
	if exists {
		return map[string]any{"opportunity_id": prior, "owner_user_id": ownerID, "already_recorded": true}, nil
	}
	if err = automationSalesHubCheckOpportunityTxV273545(tx, tid, oid, false); err != nil {
		return nil, err
	}
	var ownerName string
	if err = tx.QueryRow(`SELECT name FROM app_users WHERE id=? AND tenant_id=? AND active=1`, ownerID, tid).Scan(&ownerName); err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("responsable no disponible o de otra empresa")
		}
		return nil, err
	}
	updated, err := tx.Exec(`UPDATE crm_opportunities SET owner=?,automation_state='automatic',updated_at=? WHERE id=? AND tenant_id=? AND COALESCE(deleted_at,'')='' AND COALESCE(manual_lock,0)=0`, ownerName, time.Now().UTC().Format(time.RFC3339), oid, tid)
	if err != nil {
		return nil, err
	}
	changed, err := updated.RowsAffected()
	if err != nil {
		return nil, err
	}
	if changed != 1 {
		return nil, errors.New("oportunidad bloqueada o no encontrada")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = tx.Exec(`INSERT INTO opportunity_hub_meta_v273518(tenant_id,opportunity_id,owner_user_id,updated_at) VALUES(?,?,?,?) ON CONFLICT(tenant_id,opportunity_id) DO UPDATE SET owner_user_id=excluded.owner_user_id,updated_at=excluded.updated_at`, tid, oid, ownerID, now)
	if err != nil {
		return nil, err
	}
	if err = salesHubAuditTxV273518(tx, tid, oid, 0, "owner.assigned.automation", fmt.Sprintf("execution=%d node=%s user_id=%d", eid, n.ID, ownerID)); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(`INSERT INTO automation_sales_hub_effects_v273545(tenant_id,execution_id,node_id,effect_type,record_id,created_at) VALUES(?,?,?,?,?,?)`, tid, eid, n.ID, "owner.assign", oid, now); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("resultado de asignación no confirmado; revisar antes de repetir: %w", err)
	}
	return map[string]any{"opportunity_id": oid, "owner_user_id": ownerID, "owner": ownerName, "assigned": true}, nil
}
