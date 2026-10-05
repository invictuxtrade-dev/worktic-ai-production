package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	return err
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
		var g automationGraph
		if json.Unmarshal(x.Graph, &g) != nil {
			writeError(w, errors.New("grafo inválido"), 400)
			return
		}
		if x.Status != "active" && x.Status != "paused" {
			x.Status = "draft"
		}
		now := time.Now().UTC().Format(time.RFC3339)
		if r.Method == http.MethodPost {
			res, e := a.db.Exec(`INSERT INTO automation_workflows(tenant_id,name,description,status,trigger_type,trigger_config,graph_json,version,created_at,updated_at) VALUES(?,?,?,?,?,?,?,1,?,?)`, tid, x.Name, x.Description, x.Status, x.TriggerType, string(x.TriggerConfig), string(x.Graph), now, now)
			if e != nil {
				writeError(w, e, 500)
				return
			}
			x.ID, _ = res.LastInsertId()
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
	tid, _, err := a.tenantFor(r)
	if err != nil {
		writeError(w, err, 401)
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
	x, e := a.runAutomationWorkflow(context.Background(), tid, q.WorkflowID, "manual", "manual", q.Payload)
	if e != nil {
		writeError(w, e, 400)
		return
	}
	writeJSON(w, x)
}

func (a *App) enqueueAutomationEvent(tid int64, eventType, source string, payload any) error {
	b, _ := json.Marshal(payload)
	_, err := a.db.Exec(`INSERT INTO automation_events(tenant_id,event_type,source,payload_json,status,created_at) VALUES(?,?,?,?,'pending',?)`, tid, eventType, source, string(b), time.Now().UTC().Format(time.RFC3339))
	return err
}

func (a *App) runAutomationWorker() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		a.consumeLegacyLeadOutbox()
		a.consumeAutomationEvents()
	}
}
func (a *App) consumeLegacyLeadOutbox() {
	rows, e := a.db.Query(`SELECT id,tenant_id,event_type,source,payload_json FROM lead_automation_outbox WHERE status='pending' ORDER BY id LIMIT 20`)
	if e != nil {
		return
	}
	defer rows.Close()
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
	for _, x := range xs {
		var p map[string]any
		_ = json.Unmarshal([]byte(x.p), &p)
		if err := a.enqueueAutomationEvent(x.tid, x.ev, x.src, p); err == nil {
			_, _ = a.db.Exec(`UPDATE lead_automation_outbox SET status='processed',processed_at=? WHERE id=?`, time.Now().UTC().Format(time.RFC3339), x.id)
		}
	}
}
func (a *App) consumeAutomationEvents() {
	rows, e := a.db.Query(`SELECT id,tenant_id,event_type,source,payload_json FROM automation_events WHERE status='pending' ORDER BY id LIMIT 20`)
	if e != nil {
		return
	}
	defer rows.Close()
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
	for _, x := range xs {
		_, _ = a.db.Exec(`UPDATE automation_events SET status='processing',attempts=attempts+1 WHERE id=?`, x.id)
		var p map[string]any
		_ = json.Unmarshal([]byte(x.p), &p)
		wrows, er := a.db.Query(`SELECT id FROM automation_workflows WHERE tenant_id=? AND status='active' AND trigger_type=?`, x.tid, x.ev)
		if er != nil {
			_, _ = a.db.Exec(`UPDATE automation_events SET status='failed',last_error=? WHERE id=?`, er.Error(), x.id)
			continue
		}
		ids := []int64{}
		for wrows.Next() {
			var id int64
			if wrows.Scan(&id) == nil {
				ids = append(ids, id)
			}
		}
		wrows.Close()
		failed := ""
		for _, wid := range ids {
			if _, er = a.runAutomationWorkflow(context.Background(), x.tid, wid, x.ev, x.src, p); er != nil {
				failed = er.Error()
			}
		}
		if failed != "" {
			_, _ = a.db.Exec(`UPDATE automation_events SET status='failed',last_error=?,processed_at=? WHERE id=?`, failed, time.Now().UTC().Format(time.RFC3339), x.id)
		} else {
			_, _ = a.db.Exec(`UPDATE automation_events SET status='processed',processed_at=? WHERE id=?`, time.Now().UTC().Format(time.RFC3339), x.id)
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
	now := time.Now().UTC().Format(time.RFC3339)
	b, _ := json.Marshal(payload)
	res, e := a.db.Exec(`INSERT INTO automation_executions(tenant_id,workflow_id,event_type,source,status,context_json,started_at) VALUES(?,?,?,?, 'running',?,?)`, tid, wid, eventType, source, string(b), now)
	if e != nil {
		return nil, e
	}
	eid, _ := res.LastInsertId()
	ctxMap := map[string]any{"event": payload, "event_type": eventType, "source": source}
	status := "success"
	errMsg := ""
	for _, n := range g.Nodes {
		if n.Type == "trigger" {
			continue
		}
		_, _ = a.db.Exec(`UPDATE automation_executions SET current_node_id=? WHERE id=?`, n.ID, eid)
		stepStart := time.Now().UTC().Format(time.RFC3339)
		out, er := a.executeAutomationNode(ctx, tid, n, ctxMap)
		ob, _ := json.Marshal(out)
		stepStatus := "success"
		em := ""
		if er != nil {
			stepStatus = "failed"
			em = er.Error()
			status = "failed"
			errMsg = em
		}
		_, _ = a.db.Exec(`INSERT INTO automation_execution_steps(execution_id,node_id,node_type,status,input_json,output_json,error_message,started_at,finished_at) VALUES(?,?,?,?,?,?,?,?,?)`, eid, n.ID, n.Type, stepStatus, string(b), string(ob), em, stepStart, time.Now().UTC().Format(time.RFC3339))
		if er != nil {
			break
		}
		ctxMap[n.ID] = out
		if n.Type == "condition" {
			if matched, ok := out["match"].(bool); ok && !matched {
				ctxMap["condition_stopped_at"] = n.ID
				break
			}
		}
	}
	finish := time.Now().UTC().Format(time.RFC3339)
	_, _ = a.db.Exec(`UPDATE automation_executions SET status=?,error_message=?,finished_at=? WHERE id=?`, status, errMsg, finish, eid)
	if status == "success" {
		_, _ = a.db.Exec(`UPDATE automation_workflows SET runs=runs+1,successes=successes+1,last_run_at=?,updated_at=? WHERE id=?`, finish, finish, wid)
	} else {
		_, _ = a.db.Exec(`UPDATE automation_workflows SET runs=runs+1,failures=failures+1,last_run_at=?,updated_at=? WHERE id=?`, finish, finish, wid)
	}
	if errMsg != "" {
		return map[string]any{"execution_id": eid, "status": status}, errors.New(errMsg)
	}
	return map[string]any{"execution_id": eid, "status": status}, nil
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
	ev, _ := ctx["event"].(map[string]any)
	if v, ok := ev[key]; ok {
		return strings.TrimSpace(fmt.Sprint(v))
	}
	return ""
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
		_, err := a.db.Exec(`UPDATE crm_contacts SET tags=CASE WHEN tags='' THEN ? WHEN instr(','||tags||',', ','||?||',')>0 THEN tags ELSE tags||','||? END,updated_at=? WHERE id=? AND tenant_id=?`, tag, tag, tag, time.Now().UTC().Format(time.RFC3339), cid, tid)
		return map[string]any{"contact_id": cid, "tag": tag}, err
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
		_, err := a.db.Exec(`UPDATE crm_opportunities SET stage=?,automation_state='automatic',updated_at=?,last_activity_at=? WHERE id=? AND tenant_id=?`, stage, time.Now().UTC().Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339), oid, tid)
		return map[string]any{"opportunity_id": oid, "stage": stage}, err
	case "assign_owner":
		oid := cfgInt64(n.Config, "opportunity_id")
		if oid == 0 {
			oid, _ = strconv.ParseInt(payloadString(state, "opportunity_id"), 10, 64)
		}
		owner := cfgString(n.Config, "owner")
		if oid == 0 || owner == "" {
			return nil, errors.New("oportunidad o responsable faltante")
		}
		_, err := a.db.Exec(`UPDATE crm_opportunities SET owner=?,automation_state='automatic',updated_at=? WHERE id=? AND tenant_id=?`, owner, time.Now().UTC().Format(time.RFC3339), oid, tid)
		return map[string]any{"opportunity_id": oid, "owner": owner}, err
	case "send_whatsapp":
		conn := cfgInt64(n.Config, "connection_id")
		to := cfgString(n.Config, "to")
		if to == "" {
			to = payloadString(state, "phone")
		}
		text := cfgString(n.Config, "message")
		if conn == 0 || to == "" || text == "" {
			return nil, errors.New("WhatsApp requiere conexión, teléfono y mensaje")
		}
		c, err := a.whatsappCloudConnectionForTenant(tid, conn)
		if err != nil {
			return nil, err
		}
		mid, err := a.sendWhatsAppCloudText(ctx, c, to, text)
		return map[string]any{"message_id": mid, "to": to}, err
	case "ai_task":
		prompt := cfgString(n.Config, "prompt")
		if prompt == "" {
			prompt = "Analiza el evento y resume el siguiente paso comercial."
		}
		eventJSON, _ := json.Marshal(state["event"])
		reply, err := a.callOpenAI("Eres un agente de automatización WorkticAI. "+prompt, string(eventJSON))
		return map[string]any{"reply": reply}, err
	case "create_task":
		return map[string]any{"created": true, "title": cfgString(n.Config, "title"), "note": "registrado en ejecución; módulo de tareas dedicado se enlazará cuando esté disponible"}, nil
	case "webhook":
		target := cfgString(n.Config, "url")
		if target == "" {
			return nil, errors.New("webhook requiere URL")
		}
		payload := map[string]any{"tenant_id": tid, "event": state["event"], "automation": state}
		return a.callAutomationWebhookV26(ctx, target, payload)
	}
	return map[string]any{"skipped": true}, nil
}
