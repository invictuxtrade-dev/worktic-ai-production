package main

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Delivery ledger prevents the same event being handed to the same workflow
// twice. This provides at-most-once DISPATCH per event ID, not exactly-once
// external effects and not deduplication across distinct source event IDs.
func initAutomationDeliveryV273542Schema(db *DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS automation_event_deliveries_v273542 (
        tenant_id INTEGER NOT NULL,
        event_id INTEGER NOT NULL,
        workflow_id INTEGER NOT NULL,
        status TEXT NOT NULL DEFAULT 'claimed',
        execution_id INTEGER NOT NULL DEFAULT 0,
        error_message TEXT NOT NULL DEFAULT '',
        created_at TEXT NOT NULL,
        completed_at TEXT NOT NULL DEFAULT '',
        PRIMARY KEY(tenant_id,event_id,workflow_id)
    );
    CREATE INDEX IF NOT EXISTS idx_automation_delivery_status_v273542
    ON automation_event_deliveries_v273542(tenant_id,status,created_at);`)
	return err
}

func (a *App) claimAutomationDeliveryV273542(tenantID, eventID, workflowID int64) (bool, error) {
	if tenantID <= 0 || eventID <= 0 || workflowID <= 0 {
		return false, errors.New("identificadores de entrega inválidos")
	}
	result, err := a.db.Exec(`INSERT INTO automation_event_deliveries_v273542
      (tenant_id,event_id,workflow_id,status,created_at)
      VALUES(?,?,?,'claimed',?)
      ON CONFLICT(tenant_id,event_id,workflow_id) DO NOTHING`,
		tenantID, eventID, workflowID, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return false, err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return changed == 1, nil
}

func automationDeliveryOutcomeV273542(result map[string]any, runErr error) (string, int64, string) {
	status := "success"
	message := ""
	if runErr != nil {
		message = runErr.Error()
		status = "failed"
	}
	var execID int64
	if result != nil {
		switch id := result["execution_id"].(type) {
		case int64:
			execID = id
		case int:
			execID = int64(id)
		}
		// The execution remains in the waiting queue. Delivery was accepted;
		// completion is tracked in automation_executions, not this dispatch ledger.
		if status == "success" && result["status"] == "waiting" {
			status = "accepted"
		}
		if result["status"] == "needs_review" {
			status = "needs_review"
		}
		if status == "success" && result["status"] == "failed" {
			status = "failed"
			message = "el workflow informó ejecución fallida"
		}
	}
	if len(message) > 600 {
		message = message[:600]
	}
	return status, execID, strings.TrimSpace(message)
}

func (a *App) finishAutomationDeliveryV273542(tid, eid, wid int64, result map[string]any, runErr error) error {
	status, executionID, msg := automationDeliveryOutcomeV273542(result, runErr)
	changed, err := a.db.Exec(`UPDATE automation_event_deliveries_v273542
        SET status=?,execution_id=?,error_message=?,completed_at=?
        WHERE tenant_id=? AND event_id=? AND workflow_id=? AND status='claimed'`,
		status, executionID, msg, time.Now().UTC().Format(time.RFC3339), tid, eid, wid)
	if err != nil {
		return fmt.Errorf("no se pudo finalizar entrega de automatización: %w", err)
	}
	n, err := changed.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return errors.New("entrega de automatización requiere revisión; no repetir acciones")
	}
	return nil
}
