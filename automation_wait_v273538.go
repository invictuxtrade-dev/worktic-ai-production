package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"
)

// V27.35.38: durable, at-most-once resume of waits.
// A wait that was claimed but interrupted during an external action is left
// for human review. It must never be silently retried and resend a message.
const automationMaxWaitSecondsV273538 = 30 * 24 * 60 * 60

func automationDelaySecondsV273538(c map[string]any) (int64, error) {
	raw, ok := c["seconds"]
	if !ok {
		return 0, errors.New("configura una espera en segundos")
	}
	var v int64
	switch x := raw.(type) {
	case string:
		n, err := strconv.ParseInt(x, 10, 64)
		if err != nil {
			return 0, errors.New("la espera debe ser un número entero de segundos")
		}
		v = n
	case int:
		v = int64(x)
	case int64:
		v = x
	case float64:
		if x != float64(int64(x)) {
			return 0, errors.New("la espera debe ser un número entero")
		}
		v = int64(x)
	default:
		return 0, errors.New("la espera debe ser numérica")
	}
	if v < 0 || v > automationMaxWaitSecondsV273538 {
		return 0, fmt.Errorf("la espera debe estar entre 0 y %d segundos (30 días)", automationMaxWaitSecondsV273538)
	}
	return v, nil
}

func initAutomationWaitV273538Schema(db *DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS automation_waits_v273538(
      id INTEGER PRIMARY KEY AUTOINCREMENT,
      tenant_id INTEGER NOT NULL, execution_id INTEGER NOT NULL UNIQUE,
      workflow_id INTEGER NOT NULL, due_at TEXT NOT NULL, next_node_id TEXT NOT NULL DEFAULT '',
      graph_json TEXT NOT NULL, state_json TEXT NOT NULL,
      status TEXT NOT NULL DEFAULT 'pending', last_error TEXT NOT NULL DEFAULT '',
      created_at TEXT NOT NULL, updated_at TEXT NOT NULL
    );
    CREATE INDEX IF NOT EXISTS idx_automation_waits_v273538_due ON automation_waits_v273538(status,due_at);`)
	return err
}

// This atomic transaction persists the graph snapshot, state and execution
// status before the event worker acknowledges the originating event.
func (a *App) scheduleAutomationWaitV273538(tid, wid, eid int64, node automationNode, plan *automationExecutionPlan, graph automationGraph, state map[string]any, seconds int64) error {
	graphJSON, err := json.Marshal(graph)
	if err != nil {
		return err
	}
	stateJSON, err := json.Marshal(state)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	due := now.Add(time.Duration(seconds) * time.Second).Format(time.RFC3339)
	stamp := now.Format(time.RFC3339)
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	updated, err := tx.Exec(`UPDATE automation_waits_v273538 SET due_at=?,next_node_id=?,graph_json=?,state_json=?,status='pending',last_error='',updated_at=? WHERE tenant_id=? AND execution_id=?`, due, plan.next(node.ID, true), string(graphJSON), string(stateJSON), stamp, tid, eid)
	if err != nil {
		return err
	}
	changed, err := updated.RowsAffected()
	if err != nil {
		return err
	}
	if changed == 0 {
		_, err = tx.Exec(`INSERT INTO automation_waits_v273538(tenant_id,execution_id,workflow_id,due_at,next_node_id,graph_json,state_json,status,created_at,updated_at) VALUES(?,?,?,?,?,?,?,'pending',?,?)`, tid, eid, wid, due, plan.next(node.ID, true), string(graphJSON), string(stateJSON), stamp, stamp)
		if err != nil {
			return err
		}
	}
	output, _ := json.Marshal(map[string]any{"scheduled_seconds": seconds, "resume_at": due})
	_, err = tx.Exec(`INSERT INTO automation_execution_steps(execution_id,node_id,node_type,status,input_json,output_json,error_message,started_at,finished_at) VALUES(?,?,?,'waiting',?,?,'',?,?)`, eid, node.ID, node.Type, automationAuditInputV273548(state), string(output), stamp, stamp)
	if err != nil {
		return err
	}
	res, err := tx.Exec(`UPDATE automation_executions SET status='waiting',current_node_id=? WHERE id=? AND tenant_id=? AND status='running'`, node.ID, eid, tid)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return errors.New("la ejecución ya no está activa; espera no programada")
	}
	return tx.Commit()
}

// Runs only the saved connected path; side effects are NEVER reexecuted from
// the start after a wait. The snapshot prevents editing the workflow mid-run.
func (a *App) runAutomationPathV273538(ctx context.Context, tid, wid, eid int64, start string, plan *automationExecutionPlan, graph automationGraph, state map[string]any) (bool, error) {
	for cursor := start; cursor != ""; {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		node, ok := plan.nodes[cursor]
		if !ok {
			return false, fmt.Errorf("nodo de continuación desconocido: %s", cursor)
		}
		if node.Type == "delay" {
			seconds, err := automationDelaySecondsV273538(node.Config)
			if err != nil {
				return false, err
			}
			if seconds > 10 {
				// No outbound action is performed while the execution waits.
				if err := a.scheduleAutomationWaitV273538(tid, wid, eid, node, plan, graph, state, seconds); err != nil {
					return false, err
				}
				return true, nil
			}
		}
		if _, err := a.db.Exec(`UPDATE automation_executions SET current_node_id=? WHERE id=? AND tenant_id=?`, node.ID, eid, tid); err != nil {
			return false, err
		}
		started := time.Now().UTC().Format(time.RFC3339)
		out, execErr := a.executeAutomationNode(ctx, tid, node, state)
		// The state (and AI replies) may include PII. Keep only non-sensitive
		// metadata in steps; actual workflow state remains in memory/waits.
		input := automationAuditInputV273548(state)
		output := automationAuditOutputV273548(out)
		stepStatus := "success"
		errorText := ""
		if execErr != nil {
			stepStatus = "failed"
			errorText = automationAuditErrorV273548(execErr)
		}
		if _, err := a.db.Exec(`INSERT INTO automation_execution_steps(execution_id,node_id,node_type,status,input_json,output_json,error_message,started_at,finished_at) VALUES(?,?,?,?,?,?,?,?,?)`, eid, node.ID, node.Type, stepStatus, input, output, errorText, started, time.Now().UTC().Format(time.RFC3339)); err != nil {
			return false, fmt.Errorf("no se pudo guardar la auditoría de %s: %w", node.ID, err)
		}
		if execErr != nil {
			return false, execErr
		}
		state[node.ID] = out
		if node.Type == "ai_task" {
			// Alias for the most recent AI result, retained across persisted waits.
			state["ai"] = out
		}
		matched := true
		if node.Type == "condition" {
			matched, _ = out["match"].(bool)
		}
		cursor = plan.next(node.ID, matched)
	}
	return false, nil
}

func (a *App) completeAutomationExecutionV273538(tid, wid, eid int64, executionErr error) error {
	state, message := "success", ""
	if executionErr != nil {
		state, message = "failed", automationAuditErrorV273548(executionErr)
	}
	finish := time.Now().UTC().Format(time.RFC3339)
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	updated, err := tx.Exec(`UPDATE automation_executions SET status=?,error_message=?,finished_at=? WHERE id=? AND tenant_id=? AND status='running'`, state, message, finish, eid, tid)
	if err != nil {
		return err
	}
	count, err := updated.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return errors.New("ejecución finalizada anteriormente o modificada por otro worker")
	}
	if state == "success" {
		_, err = tx.Exec(`UPDATE automation_workflows SET runs=runs+1,successes=successes+1,last_run_at=?,updated_at=? WHERE id=? AND tenant_id=?`, finish, finish, wid, tid)
	} else {
		_, err = tx.Exec(`UPDATE automation_workflows SET runs=runs+1,failures=failures+1,last_run_at=?,updated_at=? WHERE id=? AND tenant_id=?`, finish, finish, wid, tid)
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

// Reads only due pending waits. Claims are compare-and-swap to support multiple
// instances. Processing claims NEVER auto-recover: potential external sends
// might already have happened, so human inspection is required.
func (a *App) consumeAutomationWaitsV273538() {
	now := time.Now().UTC().Format(time.RFC3339)
	rows, err := a.db.Query(`SELECT id,tenant_id,workflow_id,execution_id,next_node_id,graph_json,state_json FROM automation_waits_v273538 WHERE status='pending' AND due_at<=? ORDER BY due_at,id LIMIT 20`, now)
	if err != nil {
		return
	}
	var due []automationWaitItemV273538
	for rows.Next() {
		var x automationWaitItemV273538
		if rows.Scan(&x.id, &x.tid, &x.wid, &x.eid, &x.next, &x.graph, &x.state) == nil {
			due = append(due, x)
		}
	}
	rows.Close()
	for _, x := range due {
		// Paused workflows must not resume external actions. If a workflow
		// was deleted, leave the wait for explicit inspection instead.
		var status string
		if err := a.db.QueryRow(`SELECT status FROM automation_workflows WHERE id=? AND tenant_id=?`, x.wid, x.tid).Scan(&status); err != nil || status != "active" {
			continue
		}
		claim, err := a.db.Exec(`UPDATE automation_waits_v273538 SET status='processing',updated_at=? WHERE id=? AND tenant_id=? AND status='pending'`, now, x.id, x.tid)
		if err != nil {
			continue
		}
		affected, err := claim.RowsAffected()
		if err != nil || affected != 1 {
			continue
		}
		// The persisted wait, not a new inbound event, resumes an existing execution.
		var executionStatus string
		if err = a.db.QueryRow(`SELECT status FROM automation_executions WHERE id=? AND tenant_id=? AND workflow_id=?`, x.eid, x.tid, x.wid).Scan(&executionStatus); err != nil || executionStatus != "waiting" {
			a.failAutomationWaitV273538(x, errors.New("ejecución no encontrada o no está esperando"))
			continue
		}
		var graph automationGraph
		var state map[string]any
		if err = json.Unmarshal([]byte(x.graph), &graph); err == nil {
			err = json.Unmarshal([]byte(x.state), &state)
		}
		var plan *automationExecutionPlan
		if err == nil {
			plan, err = buildAutomationExecutionPlan(graph)
		}
		if err != nil || state == nil {
			if err == nil {
				err = errors.New("contexto de espera vacío")
			}
			a.failAutomationWaitV273538(x, err)
			continue
		}
		update, updateErr := a.db.Exec(`UPDATE automation_executions SET status='running' WHERE id=? AND tenant_id=? AND status='waiting'`, x.eid, x.tid)
		if updateErr != nil {
			a.failAutomationWaitV273538(x, updateErr)
			continue
		}
		count, updateErr := update.RowsAffected()
		if updateErr != nil || count != 1 {
			a.failAutomationWaitV273538(x, errors.New("otro worker modificó la ejecución"))
			continue
		}
		waiting, execErr := a.runAutomationPathV273538(context.Background(), x.tid, x.wid, x.eid, x.next, plan, graph, state)
		if waiting {
			continue
		} // a later delay atomically resets this same wait row to pending
		// V27.35.52: close the wait + execution + workflow counters atomically.
		// NEVER replay the preceding nodes after an uncertain completion.
		if finalErr := a.finishResumedAutomationWaitV273552(x, execErr); finalErr != nil {
			if reviewErr := a.markAutomationWaitFailureForReviewV273551(x, finalErr); reviewErr != nil {
				log.Printf("automation wait completion review id=%d: %v", x.id, reviewErr)
			}
		}

	}
}

type automationWaitItemV273538 struct {
	id, tid, wid, eid  int64
	next, graph, state string
}

func (a *App) failAutomationWaitV273538(x automationWaitItemV273538, err error) {
	if err == nil {
		err = errors.New("espera inválida")
	}
	// V27.35.51: both review updates must commit atomically.
	// Never retry an action after an uncertain external result.
	if reviewErr := a.markAutomationWaitFailureForReviewV273551(x, err); reviewErr != nil {
		log.Printf("automation wait review id=%d: %v", x.id, reviewErr)
	}
}
