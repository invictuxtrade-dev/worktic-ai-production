package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type AutomationWorkflow struct {
	ID            int64           `json:"id"`
	TenantID      int64           `json:"tenant_id"`
	Name          string          `json:"name"`
	Description   string          `json:"description"`
	Status        string          `json:"status"`
	TriggerType   string          `json:"trigger_type"`
	TriggerConfig json.RawMessage `json:"trigger_config"`
	Graph         json.RawMessage `json:"graph"`
	Version       int             `json:"version"`
	Runs          int             `json:"runs"`
	Successes     int             `json:"successes"`
	Failures      int             `json:"failures"`
	LastRunAt     string          `json:"last_run_at"`
	CreatedAt     string          `json:"created_at"`
	UpdatedAt     string          `json:"updated_at"`
}

type automationNode struct {
	ID     string         `json:"id"`
	Type   string         `json:"type"`
	Label  string         `json:"label"`
	Config map[string]any `json:"config"`
}

type automationEdge struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Branch string `json:"branch"`
}
type automationGraph struct {
	Nodes []automationNode `json:"nodes"`
	Edges []automationEdge `json:"edges"`
}

func initAutomationV20Schema(db *DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS automation_workflows(
 id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL, name TEXT NOT NULL,
 description TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'draft',
 trigger_type TEXT NOT NULL DEFAULT 'manual', trigger_config TEXT NOT NULL DEFAULT '{}',
 graph_json TEXT NOT NULL DEFAULT '{"nodes":[],"edges":[]}', version INTEGER NOT NULL DEFAULT 1,
 runs INTEGER NOT NULL DEFAULT 0, successes INTEGER NOT NULL DEFAULT 0, failures INTEGER NOT NULL DEFAULT 0,
 last_run_at TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_automation_workflows_tenant ON automation_workflows(tenant_id,status,updated_at DESC);
CREATE TABLE IF NOT EXISTS automation_executions(
 id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL, workflow_id INTEGER NOT NULL,
 event_type TEXT NOT NULL DEFAULT '', source TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'running',
 current_node_id TEXT NOT NULL DEFAULT '', context_json TEXT NOT NULL DEFAULT '{}',
 error_message TEXT NOT NULL DEFAULT '', started_at TEXT NOT NULL, finished_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_automation_exec_tenant ON automation_executions(tenant_id,started_at DESC);
CREATE INDEX IF NOT EXISTS idx_automation_exec_workflow ON automation_executions(workflow_id,started_at DESC);
CREATE TABLE IF NOT EXISTS automation_execution_steps(
 id INTEGER PRIMARY KEY AUTOINCREMENT, execution_id INTEGER NOT NULL, node_id TEXT NOT NULL, node_type TEXT NOT NULL,
 status TEXT NOT NULL, input_json TEXT NOT NULL DEFAULT '{}', output_json TEXT NOT NULL DEFAULT '{}',
 error_message TEXT NOT NULL DEFAULT '', started_at TEXT NOT NULL, finished_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_automation_steps_execution ON automation_execution_steps(execution_id,id);
CREATE TABLE IF NOT EXISTS automation_events(
 id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL, event_type TEXT NOT NULL, source TEXT NOT NULL DEFAULT '',
 payload_json TEXT NOT NULL DEFAULT '{}', status TEXT NOT NULL DEFAULT 'pending', attempts INTEGER NOT NULL DEFAULT 0,
 last_error TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL, processed_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_automation_events_pending ON automation_events(status,created_at);
`)
	if err != nil {
		return err
	}
	if err := initAutomationWaitV273538Schema(db); err != nil {
		return err
	}
	if err := initAutomationDeliveryV273542Schema(db); err != nil {
		return err
	}
	// V27.35.83: Sales Hub receipts were defined but never initialized.
	// Without this table, assign_owner fails before writing any assignment.
	if err := initAutomationSalesHubV273545Schema(db); err != nil {
		return err
	}
	return initAutomationWAInboundV273546Schema(db)
}

func (a *App) automationWorkflowsHandler(w http.ResponseWriter, r *http.Request) {
	tid, u, err := a.tenantFor(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	if u == nil {
		writeError(w, errors.New("sesión inválida"), 401)
		return
	}
	switch r.Method {
	case http.MethodGet:
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		if id > 0 {
			x, e := a.automationWorkflowByID(tid, id)
			if e != nil {
				writeError(w, e, 404)
				return
			}
			writeJSON(w, x)
			return
		}
		rows, e := a.db.Query(`SELECT id,tenant_id,name,description,status,trigger_type,trigger_config,graph_json,version,runs,successes,failures,last_run_at,created_at,updated_at FROM automation_workflows WHERE tenant_id=? ORDER BY updated_at DESC`, tid)
		if e != nil {
			writeError(w, e, 500)
			return
		}
		defer rows.Close()
		out := []AutomationWorkflow{}
		for rows.Next() {
			var x AutomationWorkflow
			var tc, g string
			if rows.Scan(&x.ID, &x.TenantID, &x.Name, &x.Description, &x.Status, &x.TriggerType, &tc, &g, &x.Version, &x.Runs, &x.Successes, &x.Failures, &x.LastRunAt, &x.CreatedAt, &x.UpdatedAt) == nil {
				x.TriggerConfig = json.RawMessage(tc)
				x.Graph = json.RawMessage(g)
				out = append(out, x)
			}
		}
		writeJSON(w, out)
	case http.MethodPost, http.MethodPut:
		if !canManageAgents(u.Role) {
			writeError(w, errors.New("solo propietarios y administradores pueden editar automatizaciones"), 403)
			return
		}
		var x AutomationWorkflow
		if json.NewDecoder(r.Body).Decode(&x) != nil {
			writeError(w, errors.New("automatización inválida"), 400)
			return
		}
		x.Name = strings.TrimSpace(x.Name)
		if x.Name == "" {
			writeError(w, errors.New("nombre obligatorio"), 400)
			return
		}
		if x.TriggerType == "" {
			x.TriggerType = "manual"
		}
		if len(x.TriggerConfig) == 0 {
			x.TriggerConfig = []byte(`{}`)
		}
		if len(x.Graph) == 0 {
			x.Graph = []byte(`{"nodes":[],"edges":[]}`)
		}
		if err := automationValidTriggerConfigV273536(x.TriggerConfig, x.TriggerType); err != nil {
			writeError(w, err, 400)
			return
		}
		// Plantilla comercial: exigir selección explícita del formulario.
		// El filtro existente comprueba después que ese formulario sea del tenant.
		if x.Status == "active" && x.TriggerType == "form.submitted" {
			if err := automationValidateCaptacionScopeV273569(x.Graph, x.TriggerConfig); err != nil {
				writeError(w, err, 400)
				return
			}
		}
		var g automationGraph
		if json.Unmarshal(x.Graph, &g) != nil {
			writeError(w, errors.New("grafo inválido"), 400)
			return
		}
		// Drafts may be edited incrementally. Only publish executable graphs.
		if x.Status == "active" && x.TriggerType == "form.submitted" {
			var formFilter map[string]string
			if err := json.Unmarshal(x.TriggerConfig, &formFilter); err != nil {
				writeError(w, errors.New("filtro de formulario inválido"), 400)
				return
			}
			if formKey := strings.TrimSpace(formFilter["form_id"]); formKey != "" {
				formID, parseErr := strconv.ParseInt(formKey, 10, 64)
				if parseErr != nil || formID <= 0 {
					writeError(w, errors.New("identificador de formulario inválido"), 400)
					return
				}
				var count int
				lookupErr := a.db.QueryRow(`SELECT COUNT(*) FROM marketing_forms WHERE id=? AND tenant_id=? AND active=1`, formID, tid).Scan(&count)
				if lookupErr != nil || count != 1 {
					writeError(w, errors.New("selecciona un formulario activo de esta empresa"), 400)
					return
				}
			}
		}
		if x.Status == "active" {
			if _, err := buildAutomationExecutionPlan(g); err != nil {
				writeError(w, fmt.Errorf("no se puede activar: %w", err), 400)
				return
			}
			if err := validateAutomationActionsV273540(g); err != nil {
				writeError(w, fmt.Errorf("revisa la configuración antes de activar: %w", err), 400)
				return
			}
			if err := a.validateAutomationSalesHubReferencesV273545(tid, g); err != nil {
				writeError(w, fmt.Errorf("revisa los responsables y oportunidades: %w", err), 400)
				return
			}
			if err := automationValidateAINodeTypesV273549(automationAINodeTypesV273549(g), automationAIMaxNodesV273549()); err != nil {
				writeError(w, fmt.Errorf("limite de consumo IA: %w", err), 400)
				return
			}
			if err := a.validateAutomationAgentsV273547(tid, g); err != nil {
				writeError(w, fmt.Errorf("revisa los agentes IA: %w", err), 400)
				return
			}
		}
		if x.Status != "active" && x.Status != "paused" {
			x.Status = "draft"
		}
		now := time.Now().UTC().Format(time.RFC3339)
		if r.Method == http.MethodPost {
			// Never let a client-provided ID skip creation or quota checks.
			x.ID = 0
			if (commercialModuleGateEnabledV2722() || consumptionV2723Enabled()) && u.Role != "superadmin" {
				plan, _, planErr := a.activePlan(a.billingAccountUserID(u))
				if planErr != nil {
					writeError(w, errors.New("plan no disponible"), 503)
					return
				}
				if consumptionV2723Enabled() {
					id, e := a.createAutomationWorkflowV2723(tid, x, plan.MaxRules, now)
					if e != nil {
						writeError(w, e, 403)
						return
					}
					x.ID = id
				} else {
					var n int
					if e := a.db.QueryRow(`SELECT COUNT(*) FROM automation_workflows WHERE tenant_id=?`, tid).Scan(&n); e != nil {
						writeError(w, e, 503)
						return
					}
					if n >= plan.MaxRules {
						writeError(w, errors.New("límite de workflows del plan alcanzado"), 403)
						return
					}
				}
			}
			if x.ID == 0 {
				res, e := a.db.Exec(`INSERT INTO automation_workflows(tenant_id,name,description,status,trigger_type,trigger_config,graph_json,version,created_at,updated_at) VALUES(?,?,?,?,?,?,?,1,?,?)`, tid, x.Name, x.Description, x.Status, x.TriggerType, string(x.TriggerConfig), string(x.Graph), now, now)
				if e != nil {
					writeError(w, e, 500)
					return
				}
				x.ID, _ = res.LastInsertId()
			}
		} else {
			if x.ID <= 0 {
				writeError(w, errors.New("id inválido"), 400)
				return
			}
			res, e := a.db.Exec(`UPDATE automation_workflows SET name=?,description=?,status=?,trigger_type=?,trigger_config=?,graph_json=?,version=version+1,updated_at=? WHERE id=? AND tenant_id=?`, x.Name, x.Description, x.Status, x.TriggerType, string(x.TriggerConfig), string(x.Graph), now, x.ID, tid)
			if e != nil {
				writeError(w, e, 500)
				return
			}
			n, _ := res.RowsAffected()
			if n == 0 {
				writeError(w, errors.New("automatización no encontrada"), 404)
				return
			}
		}
		saved, _ := a.automationWorkflowByID(tid, x.ID)
		writeJSON(w, saved)
	case http.MethodDelete:
		if !canManageAgents(u.Role) {
			writeError(w, errors.New("sin permiso"), 403)
			return
		}
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		if id <= 0 {
			writeError(w, errors.New("id inválido"), 400)
			return
		}
		_, e := a.db.Exec(`DELETE FROM automation_workflows WHERE id=? AND tenant_id=?`, id, tid)
		if e != nil {
			writeError(w, e, 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	default:
		http.Error(w, "Método no permitido", 405)
	}
}

func (a *App) automationWorkflowByID(tid, id int64) (AutomationWorkflow, error) {
	var x AutomationWorkflow
	var tc, g string
	err := a.db.QueryRow(`SELECT id,tenant_id,name,description,status,trigger_type,trigger_config,graph_json,version,runs,successes,failures,last_run_at,created_at,updated_at FROM automation_workflows WHERE id=? AND tenant_id=?`, id, tid).Scan(&x.ID, &x.TenantID, &x.Name, &x.Description, &x.Status, &x.TriggerType, &tc, &g, &x.Version, &x.Runs, &x.Successes, &x.Failures, &x.LastRunAt, &x.CreatedAt, &x.UpdatedAt)
	x.TriggerConfig = json.RawMessage(tc)
	x.Graph = json.RawMessage(g)
	return x, err
}

func (a *App) automationExecutionsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}
	tid, u, err := a.tenantFor(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	// Historial completo (lista, detalle y salud) solo para roles autorizados.
	if u == nil || !canManageAgents(u.Role) {
		writeError(w, errors.New("sin permiso para consultar ejecuciones"), 403)
		return
	}
	// Read-only queue health. Never expose event payloads or another tenant's counts.
	if r.URL.Query().Get("health") == "1" {
		if u == nil || !canManageAgents(u.Role) {
			writeError(w, errors.New("sin permiso para consultar la cola"), 403)
			return
		}
		health, healthErr := a.automationQueueHealthV273537(tid)
		if healthErr != nil {
			writeError(w, healthErr, 503)
			return
		}
		writeJSON(w, health)
		return
	}
	if r.URL.Query().Get("ai_usage") == "1" {
		// Counts are completed or failed AI action steps, NOT tokens or money.
		// Financial metering is already performed by callOpenAI/commercialCore.
		start, end := automationAIMonthBoundsV273549(time.Now().UTC())
		var total, success, failed int64
		err := a.db.QueryRow(`SELECT COUNT(*),
		 COALESCE(SUM(CASE WHEN s.status='success' THEN 1 ELSE 0 END),0),
		 COALESCE(SUM(CASE WHEN s.status='failed' THEN 1 ELSE 0 END),0)
		 FROM automation_execution_steps s
		 JOIN automation_executions e ON e.id=s.execution_id
		 WHERE e.tenant_id=? AND s.node_type='ai_task'
		 AND s.started_at>=? AND s.started_at<?`, tid, start.Format(time.RFC3339), end.Format(time.RFC3339)).Scan(&total, &success, &failed)
		if err != nil {
			writeError(w, errors.New("no se pudo consultar el uso IA de automatizaciones"), 503)
			return
		}
		writeJSON(w, map[string]any{
			"period": start.Format("2006-01"), "total_actions": total,
			"successful_actions": success, "failed_actions": failed,
			"max_ai_nodes_per_workflow": automationAIMaxNodesV273549(),
			"metering_note":             "Conteos de pasos IA, no tokens ni costos. El medidor comercial existente controla y registra el consumo del proveedor.",
		})
		return
	}
	// Detail requests cannot enumerate other tenants' execution steps.
	if eid, _ := strconv.ParseInt(r.URL.Query().Get("execution_id"), 10, 64); eid > 0 {
		var owner int64
		if err := a.db.QueryRow(`SELECT tenant_id FROM automation_executions WHERE id=? AND tenant_id=?`, eid, tid).Scan(&owner); err != nil {
			writeError(w, errors.New("ejecución no encontrada"), 404)
			return
		}
		rows, err := a.db.Query(`SELECT node_id,node_type,status,error_message,started_at,finished_at FROM automation_execution_steps WHERE execution_id=? ORDER BY id`, eid)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		defer rows.Close()
		steps := []map[string]any{}
		for rows.Next() {
			var nodeID, nodeType, status, msg, start, finish string
			if err := rows.Scan(&nodeID, &nodeType, &status, &msg, &start, &finish); err != nil {
				writeError(w, err, 500)
				return
			}
			steps = append(steps, map[string]any{"node_id": nodeID, "node_type": nodeType, "status": status, "error_message": msg, "started_at": start, "finished_at": finish})
		}
		if err := rows.Err(); err != nil {
			writeError(w, err, 500)
			return
		}
		writeJSON(w, map[string]any{"execution_id": eid, "steps": steps})
		return
	}
	wid, _ := strconv.ParseInt(r.URL.Query().Get("workflow_id"), 10, 64)
	q := `SELECT id,workflow_id,event_type,source,status,current_node_id,error_message,started_at,finished_at FROM automation_executions WHERE tenant_id=?`
	args := []any{tid}
	if wid > 0 {
		q += ` AND workflow_id=?`
		args = append(args, wid)
	}
	q += ` ORDER BY id DESC LIMIT 100`
	rows, e := a.db.Query(q, args...)
	if e != nil {
		writeError(w, e, 500)
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, wf int64
		var ev, src, st, node, er, sa, fa string
		if rows.Scan(&id, &wf, &ev, &src, &st, &node, &er, &sa, &fa) == nil {
			out = append(out, map[string]any{"id": id, "workflow_id": wf, "event_type": ev, "source": src, "status": st, "current_node_id": node, "error_message": er, "started_at": sa, "finished_at": fa})
		}
	}
	writeJSON(w, out)
}

func (a *App) automationRunHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}
	tid, u, err := a.tenantFor(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	if u == nil || !canManageAgents(u.Role) {
		writeError(w, errors.New("sin permiso"), 403)
		return
	}
	var q struct {
		WorkflowID int64          `json:"workflow_id"`
		Payload    map[string]any `json:"payload"`
	}
	if json.NewDecoder(r.Body).Decode(&q) != nil || q.WorkflowID <= 0 {
		writeError(w, errors.New("workflow inválido"), 400)
		return
	}
	// The Test button must never send messages, call AI, or change CRM data.
	// Actual workflows execute only from the event worker when active.
	wf, e := a.automationWorkflowByID(tid, q.WorkflowID)
	if e != nil {
		writeError(w, e, 404)
		return
	}
	var g automationGraph
	if e = json.Unmarshal(wf.Graph, &g); e != nil {
		writeError(w, errors.New("grafo inválido"), 400)
		return
	}
	plan, e := buildAutomationExecutionPlan(g)
	if e != nil {
		writeError(w, e, 400)
		return
	}
	preview := previewAutomationPlan(plan, q.Payload)
	preview["trigger_type"] = wf.TriggerType
	preview["trigger_matches"] = automationTriggerMatchesV273536(wf.TriggerConfig, wf.TriggerType, q.Payload)
	writeJSON(w, preview)
}

func (a *App) enqueueAutomationEvent(tid int64, eventType, source string, payload any) error {
	b, _ := json.Marshal(payload)
	_, err := a.db.Exec(`INSERT INTO automation_events(tenant_id,event_type,source,payload_json,status,created_at) VALUES(?,?,?,?,'pending',?)`, tid, eventType, source, string(b), time.Now().UTC().Format(time.RFC3339))
	return err
}

func (a *App) runAutomationWorker() {
	ticker := time.NewTicker(2 * time.Second)
	reviewTicker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	defer reviewTicker.Stop()
	// The first reconciliation is read-only with respect to work execution:
	// stale claims are marked for investigation, never replayed.
	a.reviewStaleAutomationEventsV273541()
	a.reviewStalePublicFormEventsV273544()
	a.reviewStaleAutomationWaitsV273550()
	for {
		select {
		case <-ticker.C:
			a.consumeLegacyLeadOutbox()
			a.consumeAutomationEvents()
			a.consumeAutomationWaitsV273538()
		case <-reviewTicker.C:
			a.reviewStaleAutomationEventsV273541()
			a.reviewStalePublicFormEventsV273544()
			a.reviewStaleAutomationWaitsV273550()
		}
	}
}

// V27.35.41 — Lead outbox forwarding must be atomic. If the transaction
// fails at any point, both the event insert and outbox acknowledgement roll
// back together. Concurrent workers only transfer a pending row once.
func (a *App) consumeLegacyLeadOutbox() {
	rows, err := a.db.Query(`SELECT id,tenant_id,event_type,source,payload_json FROM lead_automation_outbox WHERE status='pending' ORDER BY id LIMIT 20`)
	if err != nil {
		return
	}
	type item struct {
		id, tid    int64
		ev, src, p string
	}
	items := make([]item, 0, 20)
	for rows.Next() {
		var x item
		if rows.Scan(&x.id, &x.tid, &x.ev, &x.src, &x.p) == nil {
			items = append(items, x)
		}
	}
	rows.Close() // Do not hold a read cursor while acquiring write transactions.
	for _, x := range items {
		tx, err := a.db.Begin()
		if err != nil {
			continue
		}
		claimed, err := tx.Exec(`UPDATE lead_automation_outbox SET status='processing',attempts=attempts+1 WHERE id=? AND tenant_id=? AND status='pending'`, x.id, x.tid)
		if err != nil {
			_ = tx.Rollback()
			continue
		}
		n, err := claimed.RowsAffected()
		if err != nil || n != 1 {
			_ = tx.Rollback()
			continue
		}
		stamp := time.Now().UTC().Format(time.RFC3339)
		_, err = tx.Exec(`INSERT INTO automation_events(tenant_id,event_type,source,payload_json,status,created_at) VALUES(?,?,?,?,'pending',?)`, x.tid, x.ev, x.src, x.p, stamp)
		if err != nil {
			_ = tx.Rollback()
			continue
		}
		ack, err := tx.Exec(`UPDATE lead_automation_outbox SET status='processed',processed_at=? WHERE id=? AND tenant_id=? AND status='processing'`, stamp, x.id, x.tid)
		if err != nil {
			_ = tx.Rollback()
			continue
		}
		n, err = ack.RowsAffected()
		if err != nil || n != 1 {
			_ = tx.Rollback()
			continue
		}
		// An ambiguous commit can be inspected via the outbox row. Do not
		// launch a second transfer outside this transaction.
		if err := tx.Commit(); err != nil {
			log.Printf("automation lead outbox commit: %v", err)
		}
	}
}
func (a *App) consumeAutomationEvents() {
	rows, e := a.db.Query(`SELECT id,tenant_id,event_type,source,payload_json FROM automation_events WHERE status='pending' ORDER BY id LIMIT 20`)
	if e != nil {
		return
	}
	type item struct {
		id, tid    int64
		ev, src, p string
	}
	xs := []item{}
	for rows.Next() {
		var x item
		if rows.Scan(&x.id, &x.tid, &x.ev, &x.src, &x.p) == nil {
			xs = append(xs, x)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return
	}
	rows.Close() // Release reader before any claim/write transaction.
	for _, x := range xs {
		claim, claimErr := a.db.Exec(`UPDATE automation_events SET status='processing',attempts=attempts+1,processed_at=? WHERE id=? AND tenant_id=? AND status='pending'`, time.Now().UTC().Format(time.RFC3339), x.id, x.tid)
		if claimErr != nil {
			continue
		}
		claimed, _ := claim.RowsAffected()
		if claimed != 1 {
			continue
		} // another instance claimed this event
		var p map[string]any
		_ = json.Unmarshal([]byte(x.p), &p)
		wrows, er := a.db.Query(`SELECT id,trigger_config FROM automation_workflows WHERE tenant_id=? AND status='active' AND trigger_type=?`, x.tid, x.ev)
		if er != nil {
			_, _ = a.db.Exec(`UPDATE automation_events SET status='failed',last_error=? WHERE id=? AND tenant_id=? AND status='processing'`, er.Error(), x.id, x.tid)
			continue
		}
		ids := []int64{}
		for wrows.Next() {
			var id int64
			var triggerConfig string
			if wrows.Scan(&id, &triggerConfig) == nil && automationTriggerMatchesV273536([]byte(triggerConfig), x.ev, p) {
				ids = append(ids, id)
			}
		}
		wrows.Close()
		failed := ""
		requiresReview := false
		for _, wid := range ids {
			// A delivery belongs to one tenant/event/workflow combination.
			// Reserve it BEFORE performing any side effects; never retry an
			// already reserved delivery automatically after a crash.
			claimed, claimErr := a.claimAutomationDeliveryV273542(x.tid, x.id, wid)
			if claimErr != nil {
				failed = claimErr.Error()
				break
			}
			if !claimed {
				// Existing reservation is not proof of completed execution.
				requiresReview = true
				failed = "evento ya reservado: requiere revisión manual antes de repetir"
				break
			}
			run, runErr := a.runAutomationWorkflow(context.Background(), x.tid, wid, x.ev, x.src, p)
			if err := a.finishAutomationDeliveryV273542(x.tid, x.id, wid, run, runErr); err != nil {
				failed = err.Error()
				break
			}
			if runErr != nil {
				failed = runErr.Error()
			}
		}
		if requiresReview {
			_, _ = a.db.Exec(`UPDATE automation_events SET status='needs_review',last_error=?,processed_at=? WHERE id=? AND tenant_id=? AND status='processing'`, failed, time.Now().UTC().Format(time.RFC3339), x.id, x.tid)
		} else if failed != "" {
			_, _ = a.db.Exec(`UPDATE automation_events SET status='failed',last_error=?,processed_at=? WHERE id=? AND tenant_id=? AND status='processing'`, failed, time.Now().UTC().Format(time.RFC3339), x.id, x.tid)
		} else {
			_, _ = a.db.Exec(`UPDATE automation_events SET status='processed',processed_at=? WHERE id=? AND tenant_id=? AND status='processing'`, time.Now().UTC().Format(time.RFC3339), x.id, x.tid)
		}
	}
}

func (a *App) runAutomationWorkflow(ctx context.Context, tid, wid int64, eventType, source string, payload map[string]any) (map[string]any, error) {
	wf, err := a.automationWorkflowByID(tid, wid)
	if err != nil {
		return nil, err
	}
	var g automationGraph
	if json.Unmarshal(wf.Graph, &g) != nil {
		return nil, errors.New("grafo inválido")
	}
	plan, err := buildAutomationExecutionPlan(g)
	if err != nil {
		return nil, err
	}
	// Runtime defense: published workflows predating this validation cannot
	// bypass the per-workflow cap after a configuration change.
	if err := automationValidateAINodeTypesV273549(automationAINodeTypesV273549(g), automationAIMaxNodesV273549()); err != nil {
		return nil, fmt.Errorf("limite de consumo IA: %w", err)
	}
	if consumptionV2723Enabled() {
		if len(g.Nodes) > 50 {
			return nil, errors.New("workflow excede 50 nodos permitidos en piloto")
		}
		if err := a.reserveAutomationRunV2723(tid); err != nil {
			return nil, err
		}
	}
	now := time.Now().UTC().Format(time.RFC3339)
	// Only audit metadata belongs in the execution record. Full payload stays
	// in the event queue or the durable wait snapshot when needed to resume.
	auditContext := automationAuditEventV273548(eventType, source, payload)
	res, e := a.db.Exec(`INSERT INTO automation_executions(tenant_id,workflow_id,event_type,source,status,context_json,started_at) VALUES(?,?,?,?, 'running',?,?)`, tid, wid, eventType, source, auditContext, now)
	if e != nil {
		return nil, e
	}
	eid, _ := res.LastInsertId()
	ctxMap := map[string]any{"event": payload, "event_type": eventType, "source": source, "_automation_execution_id": eid}
	waiting, runErr := a.runAutomationPathV273538(ctx, tid, wid, eid, plan.next(plan.start, true), plan, g, ctxMap)
	if waiting {
		return map[string]any{"execution_id": eid, "status": "waiting"}, nil
	}
	if err := a.completeAutomationExecutionV273538(tid, wid, eid, runErr); err != nil {
		return map[string]any{"execution_id": eid, "status": "needs_review"}, fmt.Errorf("la ejecución requiere revisión: %w", err)
	}
	if runErr != nil {
		return map[string]any{"execution_id": eid, "status": "failed"}, runErr
	}
	return map[string]any{"execution_id": eid, "status": "success"}, nil
}

func cfgString(m map[string]any, k string) string {
	if v, ok := m[k]; ok {
		return strings.TrimSpace(fmt.Sprint(v))
	}
	return ""
}
func cfgInt64(m map[string]any, k string) int64 {
	v := cfgString(m, k)
	n, _ := strconv.ParseInt(v, 10, 64)
	return n
}
func payloadString(ctx map[string]any, key string) string {
	// V27.35.47: condition nodes can read prior AI results, while ordinary
	// CRM/WhatsApp action fields still resolve to the original event values.
	return automationStateValueV273547(ctx, key)
}

func (a *App) executeAutomationNode(ctx context.Context, tid int64, n automationNode, state map[string]any) (map[string]any, error) {
	switch n.Type {
	case "condition":
		field := cfgString(n.Config, "field")
		op := cfgString(n.Config, "operator")
		want := cfgString(n.Config, "value")
		got := payloadString(state, field)
		match := false
		switch op {
		case "equals":
			match = strings.EqualFold(got, want)
		case "contains":
			match = strings.Contains(strings.ToLower(got), strings.ToLower(want))
		case "not_empty":
			match = got != ""
		default:
			match = got == want
		}
		return map[string]any{"match": match, "field": field, "value": got}, nil
	case "delay":
		sec := cfgInt64(n.Config, "seconds")
		if sec < 0 {
			sec = 0
		}
		if sec > 10 {
			sec = 10
		}
		if sec > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(sec) * time.Second):
			}
		}
		return map[string]any{"waited_seconds": sec}, nil
	case "tag_contact":
		cid := cfgInt64(n.Config, "contact_id")
		if cid == 0 {
			cid, _ = strconv.ParseInt(payloadString(state, "contact_id"), 10, 64)
		}
		if cid == 0 {
			phone := normalizeCRMPhone(payloadString(state, "phone"))
			email := normalizeCRMEmail(payloadString(state, "email"))
			externalID := payloadString(state, "external_id")
			_ = a.db.QueryRow(`SELECT id FROM crm_contacts WHERE tenant_id=? AND COALESCE(deleted_at,'')='' AND ((?<>'' AND phone=?) OR (?<>'' AND lower(email)=lower(?)) OR (?<>'' AND external_id=?)) ORDER BY id DESC LIMIT 1`, tid, phone, phone, email, email, externalID, externalID).Scan(&cid)
		}
		tag := cfgString(n.Config, "tag")
		if cid == 0 || tag == "" {
			return nil, errors.New("contacto o etiqueta faltante")
		}
		res, err := a.db.Exec(`UPDATE crm_contacts SET tags=CASE WHEN tags='' THEN ? WHEN instr(','||tags||',', ','||?||',')>0 THEN tags ELSE tags||','||? END,updated_at=? WHERE id=? AND tenant_id=? AND COALESCE(deleted_at,'')=''`, tag, tag, tag, time.Now().UTC().Format(time.RFC3339), cid, tid)
		if err != nil {
			return nil, err
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return nil, err
		}
		if affected == 0 {
			return nil, errors.New("contacto no encontrado en esta empresa")
		}
		return map[string]any{"contact_id": cid, "tag": tag}, nil
	case "move_pipeline":
		oid := cfgInt64(n.Config, "opportunity_id")
		if oid == 0 {
			oid, _ = strconv.ParseInt(payloadString(state, "opportunity_id"), 10, 64)
		}
		if oid == 0 {
			leadID := payloadString(state, "lead_id")
			if leadID != "" {
				_ = a.db.QueryRow(`SELECT id FROM crm_opportunities WHERE tenant_id=? AND source_ref=? AND COALESCE(deleted_at,'')='' ORDER BY id DESC LIMIT 1`, tid, "lead:"+leadID).Scan(&oid)
			}
		}
		stage := cfgString(n.Config, "stage")
		if oid == 0 || stage == "" {
			return nil, errors.New("oportunidad o etapa faltante")
		}
		res, err := a.db.Exec(`UPDATE crm_opportunities SET stage=?,automation_state='automatic',updated_at=?,last_activity_at=? WHERE id=? AND tenant_id=? AND COALESCE(deleted_at,'')='' AND COALESCE(manual_lock,0)=0`, stage, time.Now().UTC().Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339), oid, tid)
		if err != nil {
			return nil, err
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return nil, err
		}
		if affected == 0 {
			return nil, errors.New("oportunidad no encontrada o protegida contra cambios automáticos")
		}
		return map[string]any{"opportunity_id": oid, "stage": stage}, nil
	case "assign_owner":
		return a.assignAutomationSalesHubOwnerV273545(tid, n, state)
	case "send_whatsapp":
		conn := cfgInt64(n.Config, "connection_id")
		to := cfgString(n.Config, "to")
		if to == "" {
			to = payloadString(state, "phone")
		}
		// Verified inbound WhatsApp events identify the sender in chat, not phone.
		if to == "" && payloadString(state, "channel") == "whatsapp_cloud" {
			to = strings.TrimPrefix(payloadString(state, "chat"), "wacloud:")
		}
		text := cfgString(n.Config, "message")
		if conn == 0 || to == "" || text == "" {
			return nil, errors.New("WhatsApp requiere conexión, teléfono y mensaje")
		}
		text, err := expandAutomationMessageV273534(text, state)
		if err != nil {
			return nil, err
		}
		if err := a.authorizeAutomationWhatsAppV273546(tid, conn, to, state); err != nil {
			return nil, err
		}
		c, err := a.whatsappCloudConnectionForTenant(tid, conn)
		if err != nil {
			return nil, err
		}
		mid, err := a.sendWhatsAppCloudText(ctx, c, to, text)
		return map[string]any{"message_id": mid, "to": to}, err
	case "ai_task":
		return a.executeAutomationAgentV273547(tid, n, state)
	case "create_task":
		return a.createAutomationSalesHubTaskV273545(tid, n, state)
	case "webhook":
		target := cfgString(n.Config, "url")
		if target == "" {
			return nil, errors.New("webhook requiere URL")
		}
		payload := map[string]any{"tenant_id": tid, "event": state["event"], "automation": state}
		return a.callAutomationWebhookV26(ctx, target, payload)
	}
	return nil, fmt.Errorf("acción desconocida: %s", n.Type)
}

// Convert the graph to a simple pure data shape for quota validation.
func automationAINodeTypesV273549(g automationGraph) []string {
	types := make([]string, 0, len(g.Nodes))
	for _, n := range g.Nodes {
		types = append(types, n.Type)
	}
	return types
}
