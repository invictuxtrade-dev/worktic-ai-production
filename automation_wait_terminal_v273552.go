package main

import (
	"errors"
	"time"
)

// automationResumedOutcomeV273552 provides the exact terminal statuses for
// one resumed wait; it never triggers an action or retries a workflow.
func automationResumedOutcomeV273552(executionErr error) (string, string, string) {
	if executionErr != nil {
		return "failed", "failed", automationAuditErrorV273548(executionErr)
	}
	return "completed", "success", ""
}

// finishResumedAutomationWaitV273552 commits the wait, execution and workflow
// metrics in a SINGLE transaction. This replaces the former two-step finish
// (completeAutomationExecution + update automation_waits) which could leave a
// successful execution paired with a stranded 'processing' wait.
//
// A failed/unknown commit must never cause an automatic replay of side effects.
func (a *App) finishResumedAutomationWaitV273552(x automationWaitItemV273538, executionErr error) error {
	if x.id <= 0 || x.tid <= 0 || x.wid <= 0 || x.eid <= 0 {
		return errors.New("referencias de espera inválidas")
	}
	waitStatus, executionStatus, message := automationResumedOutcomeV273552(executionErr)
	stamp := time.Now().UTC().Format(time.RFC3339)

	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Claim only the exact processing wait of this tenant and execution.
	changed, err := tx.Exec(`UPDATE automation_waits_v273538
        SET status=?,last_error=?,updated_at=?
        WHERE id=? AND tenant_id=? AND workflow_id=? AND execution_id=? AND status='processing'`,
		waitStatus, message, stamp, x.id, x.tid, x.wid, x.eid)
	if err != nil {
		return err
	}
	n, err := changed.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return errors.New("la espera ya no está en procesamiento")
	}

	// Finalize only the running execution, never a terminal/other-tenant one.
	changed, err = tx.Exec(`UPDATE automation_executions
        SET status=?,error_message=?,finished_at=?
        WHERE id=? AND tenant_id=? AND workflow_id=? AND status='running'`,
		executionStatus, message, stamp, x.eid, x.tid, x.wid)
	if err != nil {
		return err
	}
	n, err = changed.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return errors.New("la ejecución fue modificada; no se finaliza la espera")
	}

	if executionStatus == "success" {
		changed, err = tx.Exec(`UPDATE automation_workflows
            SET runs=runs+1,successes=successes+1,last_run_at=?,updated_at=?
            WHERE id=? AND tenant_id=?`, stamp, stamp, x.wid, x.tid)
	} else {
		changed, err = tx.Exec(`UPDATE automation_workflows
            SET runs=runs+1,failures=failures+1,last_run_at=?,updated_at=?
            WHERE id=? AND tenant_id=?`, stamp, stamp, x.wid, x.tid)
	}
	if err != nil {
		return err
	}
	n, err = changed.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return errors.New("workflow no encontrado; finalización revertida")
	}

	return tx.Commit()
}
