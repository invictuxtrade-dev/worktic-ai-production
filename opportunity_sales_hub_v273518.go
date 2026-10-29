package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Sales Hub stores follow-up information separately from the financial ledger.
// It does not approve payments, mark orders paid or change inventory.
func initOpportunitySalesHubV273518(db *DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS opportunity_hub_meta_v273518 (
            tenant_id INTEGER NOT NULL, opportunity_id INTEGER NOT NULL,
            owner_user_id INTEGER NOT NULL DEFAULT 0,
            currency TEXT NOT NULL DEFAULT 'COP', loss_reason TEXT NOT NULL DEFAULT '',
            updated_at TEXT NOT NULL DEFAULT '', PRIMARY KEY(tenant_id,opportunity_id))`,
		`CREATE TABLE IF NOT EXISTS opportunity_hub_tasks_v273518 (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            tenant_id INTEGER NOT NULL, opportunity_id INTEGER NOT NULL,
            owner_user_id INTEGER NOT NULL DEFAULT 0, title TEXT NOT NULL,
            due_at TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'open',
            created_by INTEGER NOT NULL, updated_by INTEGER NOT NULL,
            created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS idx_opp_hub_tasks_tenant_due ON opportunity_hub_tasks_v273518(tenant_id,status,due_at)`,
		`CREATE TABLE IF NOT EXISTS opportunity_hub_events_v273518 (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            tenant_id INTEGER NOT NULL, opportunity_id INTEGER NOT NULL,
            actor_id INTEGER NOT NULL DEFAULT 0, event_type TEXT NOT NULL,
            details TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS idx_opp_hub_events_tenant_opp ON opportunity_hub_events_v273518(tenant_id,opportunity_id,id)`,
	}
	for _, query := range statements {
		if _, err := db.Exec(query); err != nil {
			return err
		}
	}
	return nil
}

func salesHubUserExistsV273518(db *DB, tenant, id int64) bool {
	if id == 0 {
		return true
	}
	var found int64
	return db.QueryRow(`SELECT id FROM app_users WHERE id=? AND tenant_id=? AND active=1`, id, tenant).Scan(&found) == nil
}
func salesHubUserExistsTxV273518(tx *Tx, tenant, id int64) bool {
	if id == 0 {
		return true
	}
	var found int64
	return tx.QueryRow(`SELECT id FROM app_users WHERE id=? AND tenant_id=? AND active=1`, id, tenant).Scan(&found) == nil
}
func salesHubOpportunityExistsV273518(db *DB, tenant, id int64) bool {
	if id <= 0 {
		return false
	}
	var found int64
	return db.QueryRow(`SELECT id FROM crm_opportunities WHERE id=? AND tenant_id=? AND COALESCE(deleted_at,'')=''`, id, tenant).Scan(&found) == nil
}
func salesHubManagerV273518(role string) bool {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "superadmin", "owner", "admin", "supervisor":
		return true
	}
	return false
}
func salesHubCanEditV273518(db *DB, tenant, opportunity, userID int64, role string) bool {
	if salesHubManagerV273518(role) {
		return true
	}
	var assigned int64
	err := db.QueryRow(`SELECT owner_user_id FROM opportunity_hub_meta_v273518 WHERE tenant_id=? AND opportunity_id=?`, tenant, opportunity).Scan(&assigned)
	return err == nil && assigned > 0 && assigned == userID
}
func salesHubAuditV273518(db *DB, tenant, opp, actor int64, kind, details string) error {
	if len(details) > 1600 {
		details = details[:1600]
	}
	_, err := db.Exec(`INSERT INTO opportunity_hub_events_v273518(tenant_id,opportunity_id,actor_id,event_type,details,created_at) VALUES(?,?,?,?,?,?)`, tenant, opp, actor, kind, details, time.Now().UTC().Format(time.RFC3339))
	return err
}
func salesHubAuditTxV273518(tx *Tx, tenant, opp, actor int64, kind, details string) error {
	if len(details) > 1600 {
		details = details[:1600]
	}
	_, err := tx.Exec(`INSERT INTO opportunity_hub_events_v273518(tenant_id,opportunity_id,actor_id,event_type,details,created_at) VALUES(?,?,?,?,?,?)`, tenant, opp, actor, kind, details, time.Now().UTC().Format(time.RFC3339))
	return err
}

func salesHubDateV273518(raw string) (string, error) {
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return "", errors.New("indica una fecha y hora válida")
	}
	if parsed.Before(time.Now().Add(-366*24*time.Hour)) || parsed.After(time.Now().AddDate(5, 0, 0)) {
		return "", errors.New("fecha fuera del rango permitido")
	}
	return parsed.UTC().Format(time.RFC3339), nil
}

func (a *App) opportunitySalesHubV273518(w http.ResponseWriter, r *http.Request) {
	tenant, u, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, http.StatusUnauthorized)
		return
	}
	if tenant <= 0 || u == nil {
		writeError(w, errors.New("empresa inválida"), 401)
		return
	}
	opportunity, _ := strconv.ParseInt(r.URL.Query().Get("opportunity_id"), 10, 64)
	if opportunity > 0 && !salesHubOpportunityExistsV273518(a.db, tenant, opportunity) {
		writeError(w, errors.New("oportunidad no encontrada en tu empresa"), 404)
		return
	}
	if r.Method == http.MethodGet {
		a.salesHubReadV273518(w, tenant, opportunity)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	if opportunity <= 0 {
		writeError(w, errors.New("selecciona una oportunidad"), 400)
		return
	}
	if !salesHubCanEditV273518(a.db, tenant, opportunity, u.ID, u.Role) {
		writeError(w, errors.New("solo un responsable asignado o administrador puede modificar este negocio"), 403)
		return
	}
	var input struct {
		Action      string `json:"action"`
		Title       string `json:"title"`
		Text        string `json:"text"`
		DueAt       string `json:"due_at"`
		TaskID      int64  `json:"task_id"`
		OwnerUserID int64  `json:"owner_user_id"`
		Currency    string `json:"currency"`
		LossReason  string `json:"loss_reason"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&input) != nil {
		writeError(w, errors.New("datos inválidos"), 400)
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := a.db.Begin()
	if err != nil {
		writeError(w, err, 500)
		return
	}
	defer tx.Rollback()
	switch input.Action {
	case "note.add":
		text := strings.TrimSpace(input.Text)
		if len(text) < 2 || len(text) > 2000 {
			writeError(w, errors.New("la nota debe contener entre 2 y 2000 caracteres"), 400)
			return
		}
		if err = salesHubAuditTxV273518(tx, tenant, opportunity, u.ID, "note.added", text); err != nil {
			writeError(w, err, 500)
			return
		}
	case "task.create":
		title := strings.TrimSpace(input.Title)
		if len(title) < 3 || len(title) > 180 {
			writeError(w, errors.New("la tarea debe contener entre 3 y 180 caracteres"), 400)
			return
		}
		due, e := salesHubDateV273518(input.DueAt)
		if e != nil {
			writeError(w, e, 400)
			return
		}
		owner := input.OwnerUserID
		if owner == 0 {
			owner = u.ID
		}
		if !salesHubUserExistsTxV273518(tx, tenant, owner) {
			writeError(w, errors.New("responsable inválido"), 400)
			return
		}
		if !salesHubManagerV273518(u.Role) && owner != u.ID {
			writeError(w, errors.New("sin permiso de asignación"), 403)
			return
		}
		res, e := tx.Exec(`INSERT INTO opportunity_hub_tasks_v273518(tenant_id,opportunity_id,owner_user_id,title,due_at,status,created_by,updated_by,created_at,updated_at) VALUES(?,?,?,?,?,'open',?,?,?,?)`, tenant, opportunity, owner, title, due, u.ID, u.ID, now, now)
		if e != nil {
			writeError(w, e, 500)
			return
		}
		taskID, _ := res.LastInsertId()
		if e = salesHubAuditTxV273518(tx, tenant, opportunity, u.ID, "task.created", fmt.Sprintf("task=%d title=%s due=%s", taskID, title, due)); e != nil {
			writeError(w, e, 500)
			return
		}
	case "task.complete", "task.reopen", "task.cancel":
		if input.TaskID <= 0 {
			writeError(w, errors.New("tarea inválida"), 400)
			return
		}
		status := map[string]string{"task.complete": "done", "task.reopen": "open", "task.cancel": "cancelled"}[input.Action]
		res, e := tx.Exec(`UPDATE opportunity_hub_tasks_v273518 SET status=?,updated_by=?,updated_at=? WHERE id=? AND tenant_id=? AND opportunity_id=? AND status<>'cancelled'`, status, u.ID, now, input.TaskID, tenant, opportunity)
		if e != nil {
			writeError(w, e, 500)
			return
		}
		count, _ := res.RowsAffected()
		if count == 0 {
			writeError(w, errors.New("tarea no encontrada"), 404)
			return
		}
		if e = salesHubAuditTxV273518(tx, tenant, opportunity, u.ID, input.Action, fmt.Sprintf("task=%d", input.TaskID)); e != nil {
			writeError(w, e, 500)
			return
		}
	case "meta.update":
		if !salesHubManagerV273518(u.Role) {
			writeError(w, errors.New("solo administradores asignan responsable y moneda"), 403)
			return
		}
		currency := strings.ToUpper(strings.TrimSpace(input.Currency))
		if currency != "COP" && currency != "USD" && currency != "USDT" && currency != "EUR" {
			writeError(w, errors.New("moneda no admitida"), 400)
			return
		}
		if !salesHubUserExistsTxV273518(tx, tenant, input.OwnerUserID) {
			writeError(w, errors.New("usuario de otra empresa o inactivo"), 400)
			return
		}
		reason := strings.TrimSpace(input.LossReason)
		if len(reason) > 500 {
			writeError(w, errors.New("motivo demasiado largo"), 400)
			return
		}
		_, err = tx.Exec(`INSERT INTO opportunity_hub_meta_v273518(tenant_id,opportunity_id,owner_user_id,currency,loss_reason,updated_at) VALUES(?,?,?,?,?,?) ON CONFLICT(tenant_id,opportunity_id) DO UPDATE SET owner_user_id=excluded.owner_user_id,currency=excluded.currency,loss_reason=excluded.loss_reason,updated_at=excluded.updated_at`, tenant, opportunity, input.OwnerUserID, currency, reason, now)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		if err = salesHubAuditTxV273518(tx, tenant, opportunity, u.ID, "meta.updated", fmt.Sprintf("owner_user_id=%d currency=%s reason=%s", input.OwnerUserID, currency, reason)); err != nil {
			writeError(w, err, 500)
			return
		}
	default:
		writeError(w, errors.New("acción no permitida"), 400)
		return
	}
	if err = tx.Commit(); err != nil {
		writeError(w, err, 500)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}
func (a *App) salesHubRecordNoteV273518(tenant, opportunity, actor int64, note string) error {
	return salesHubAuditV273518(a.db, tenant, opportunity, actor, "note.added", note)
}
func (a *App) salesHubReadV273518(w http.ResponseWriter, tenant, opportunity int64) {
	users := []map[string]any{}
	rows, err := a.db.Query(`SELECT id,name,role FROM app_users WHERE tenant_id=? AND active=1 ORDER BY name LIMIT 250`, tenant)
	if err != nil {
		writeError(w, err, 500)
		return
	}
	for rows.Next() {
		var id int64
		var name, role string
		if rows.Scan(&id, &name, &role) == nil {
			users = append(users, map[string]any{"id": id, "name": name, "role": role})
		}
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		writeError(w, err, 500)
		return
	}
	rows.Close()
	meta := []map[string]any{}
	q := `SELECT m.opportunity_id,m.owner_user_id,m.currency,m.loss_reason,m.updated_at FROM opportunity_hub_meta_v273518 m JOIN crm_opportunities o ON o.id=m.opportunity_id AND o.tenant_id=m.tenant_id AND COALESCE(o.deleted_at,'')='' WHERE m.tenant_id=?`
	args := []any{tenant}
	if opportunity > 0 {
		q += ` AND m.opportunity_id=?`
		args = append(args, opportunity)
	}
	q += ` ORDER BY m.opportunity_id DESC LIMIT 500`
	rows, err = a.db.Query(q, args...)
	if err != nil {
		writeError(w, err, 500)
		return
	}
	for rows.Next() {
		var id, owner int64
		var currency, reason, at string
		if rows.Scan(&id, &owner, &currency, &reason, &at) == nil {
			meta = append(meta, map[string]any{"opportunity_id": id, "owner_user_id": owner, "currency": currency, "loss_reason": reason, "updated_at": at})
		}
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		writeError(w, err, 500)
		return
	}
	rows.Close()
	tasks := []map[string]any{}
	query := `SELECT t.id,t.opportunity_id,t.owner_user_id,t.title,t.due_at,t.status,t.created_by,t.updated_at FROM opportunity_hub_tasks_v273518 t JOIN crm_opportunities o ON o.id=t.opportunity_id AND o.tenant_id=t.tenant_id AND COALESCE(o.deleted_at,'')='' WHERE t.tenant_id=? AND t.status<>'cancelled'`
	args = []any{tenant}
	if opportunity > 0 {
		query += ` AND t.opportunity_id=?`
		args = append(args, opportunity)
	}
	query += ` ORDER BY CASE WHEN t.status='open' THEN 0 ELSE 1 END,t.due_at ASC,t.id DESC LIMIT 500`
	rows, err = a.db.Query(query, args...)
	if err != nil {
		writeError(w, err, 500)
		return
	}
	for rows.Next() {
		var id, opp, owner, creator int64
		var title, due, status, updated string
		if rows.Scan(&id, &opp, &owner, &title, &due, &status, &creator, &updated) == nil {
			tasks = append(tasks, map[string]any{"id": id, "opportunity_id": opp, "owner_user_id": owner, "title": title, "due_at": due, "status": status, "created_by": creator, "updated_at": updated})
		}
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		writeError(w, err, 500)
		return
	}
	rows.Close()
	events := []map[string]any{}
	if opportunity > 0 {
		rows, err = a.db.Query(`SELECT e.id,e.actor_id,e.event_type,e.details,e.created_at FROM opportunity_hub_events_v273518 e WHERE e.tenant_id=? AND e.opportunity_id=? ORDER BY e.id DESC LIMIT 100`, tenant, opportunity)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		for rows.Next() {
			var id, actor int64
			var typ, details, at string
			if rows.Scan(&id, &actor, &typ, &details, &at) == nil {
				events = append(events, map[string]any{"id": id, "actor_id": actor, "event_type": typ, "details": details, "created_at": at})
			}
		}
		if err = rows.Err(); err != nil {
			rows.Close()
			writeError(w, err, 500)
			return
		}
		rows.Close()
	}
	writeJSON(w, map[string]any{"users": users, "meta": meta, "tasks": tasks, "events": events})
}
