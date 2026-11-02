package main

import (
	"errors"
	"time"
)

// V27.35.51: when a claimed durable wait cannot be resumed, its review state
// and the associated execution state are changed in one transaction.
// No workflow is rerun, and no external request is made here.
// A terminal execution (success/failed) is not overwritten; the wait itself
// remains flagged for human review of the mismatch.
func (a *App) markAutomationWaitFailureForReviewV273551(x automationWaitItemV273538, cause error) error {
	if x.id <= 0 || x.tid <= 0 || x.wid <= 0 || x.eid <= 0 {
		return errors.New("referencias de espera inválidas")
	}
	if cause == nil {
		cause = errors.New("espera inválida")
	}
	message := automationAuditErrorV273548(cause)
	stamp := time.Now().UTC().Format(time.RFC3339)

	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.Exec(`UPDATE automation_waits_v273538
        SET status='needs_review',last_error=?,updated_at=?
        WHERE id=? AND tenant_id=? AND workflow_id=? AND execution_id=?
          AND status='processing'`, message, stamp, x.id, x.tid, x.wid, x.eid)
	if err != nil {
		return err
	}
	changed, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if changed == 0 {
		// Another worker already modified this wait. Nothing to roll forward.
		return nil
	}
	if changed != 1 {
		return errors.New("espera afectó múltiples filas")
	}

	// Keep terminal executions untouched, but atomically mark an open one.
	// The WHERE protects tenant/workflow boundaries on both tables.
	_, err = tx.Exec(`UPDATE automation_executions
        SET status='needs_review',error_message=?
        WHERE id=? AND tenant_id=? AND workflow_id=? AND status IN ('waiting','running')`,
		message, x.eid, x.tid, x.wid)
	if err != nil {
		return err
	}
	return tx.Commit()
}
