package main

import (
	"log"
	"time"
)

// V27.35.50. A crash/restart can leave a durable wait in 'processing' forever.
// Reconcile only old claims. Do not resume or retry any action: the external
// action might have succeeded just before the interruption.
// Both the wait and its execution are marked in the SAME transaction.
func (a *App) reviewStaleAutomationWaitsV273550() {
	cutoff := automationWaitReviewCutoffV273550(time.Now())
	rows, err := a.db.Query(`SELECT id,tenant_id,execution_id
		FROM automation_waits_v273538
		WHERE status='processing' AND updated_at<?
		ORDER BY updated_at,id LIMIT 50`, cutoff)
	if err != nil {
		log.Printf("automation waits reconciliation query: %v", err)
		return
	}
	type staleWait struct{ id, tenantID, executionID int64 }
	items := make([]staleWait, 0, 50)
	for rows.Next() {
		var x staleWait
		if err := rows.Scan(&x.id, &x.tenantID, &x.executionID); err != nil {
			rows.Close()
			log.Printf("automation waits reconciliation scan: %v", err)
			return
		}
		items = append(items, x)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		log.Printf("automation waits reconciliation rows: %v", err)
		return
	}
	rows.Close() // Close the read cursor before starting write transactions.

	for _, x := range items {
		if err := a.reviewOneStaleAutomationWaitV273550(x.id, x.tenantID, x.executionID, cutoff); err != nil {
			log.Printf("automation waits reconciliation id=%d: %v", x.id, err)
		}
	}
}

func (a *App) reviewOneStaleAutomationWaitV273550(waitID, tenantID, executionID int64, cutoff string) error {
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stamp := time.Now().UTC().Format(time.RFC3339)
	res, err := tx.Exec(`UPDATE automation_waits_v273538
		SET status='needs_review',last_error=?,updated_at=?
		WHERE id=? AND tenant_id=? AND execution_id=?
		AND status='processing' AND updated_at<?`,
		automationWaitReviewMessageV273550, stamp, waitID, tenantID, executionID, cutoff)
	if err != nil {
		return err
	}
	changed, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if changed == 0 {
		// Another worker changed it, or the claim is no longer stale.
		return nil
	}
	_, err = tx.Exec(`UPDATE automation_executions
		SET status='needs_review',error_message=?
		WHERE id=? AND tenant_id=? AND status IN ('running','waiting')`,
		automationWaitReviewMessageV273550, executionID, tenantID)
	if err != nil {
		return err
	}
	return tx.Commit()
}
