package main

import (
	"errors"
	"time"
)

// V27.35.37. Read-only operational health. The worker does not automatically
// retry unfinished events: external sends are not idempotent yet.
type automationQueueHealthV273537 struct {
	WorkerEnabled      bool  `json:"worker_enabled"`
	Pending            int64 `json:"pending"`
	Processing         int64 `json:"processing"`
	Failed             int64 `json:"failed"`
	NeedsReview        int64 `json:"needs_review"`
	LeadPending        int64 `json:"lead_pending"`
	LeadNeedsReview    int64 `json:"lead_needs_review"`
	Processed          int64 `json:"processed"`
	PendingOlder30m    int64 `json:"pending_older_30m"`
	ProcessingOlder30m int64 `json:"processing_older_30m"`
	RunningOlder30m    int64 `json:"running_older_30m"`
	ScheduledWaits     int64 `json:"scheduled_waits"`
	OverdueWaits       int64 `json:"overdue_waits"`
	WaitsNeedReview    int64 `json:"waits_need_review"`
}

func (a *App) automationQueueHealthV273537(tenantID int64) (automationQueueHealthV273537, error) {
	result := automationQueueHealthV273537{WorkerEnabled: a.cfg.BackgroundWorkersEnabled}
	if tenantID <= 0 {
		return result, errors.New("empresa inválida")
	}

	rows, err := a.db.Query(`SELECT status, COUNT(*) FROM automation_events WHERE tenant_id=? GROUP BY status`, tenantID)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var state string
		var count int64
		if err := rows.Scan(&state, &count); err != nil {
			rows.Close()
			return result, err
		}
		applyAutomationQueueCountV273537(&result, state, count)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return result, err
	}
	if err := rows.Close(); err != nil {
		return result, err
	}

	cutoff := time.Now().UTC().Add(-30 * time.Minute).Format(time.RFC3339)
	checks := []struct {
		q   string
		dst *int64
	}{
		{`SELECT COUNT(*) FROM automation_events WHERE tenant_id=? AND status='pending' AND created_at<?`, &result.PendingOlder30m},
		{`SELECT COUNT(*) FROM automation_events WHERE tenant_id=? AND status='processing' AND COALESCE(NULLIF(processed_at,''),created_at)<?`, &result.ProcessingOlder30m},
		{`SELECT COUNT(*) FROM automation_executions WHERE tenant_id=? AND status='running' AND started_at<?`, &result.RunningOlder30m},
	}
	for _, c := range checks {
		if err := a.db.QueryRow(c.q, tenantID, cutoff).Scan(c.dst); err != nil {
			return result, err
		}
	}
	for _, check := range []struct {
		query string
		dst   *int64
	}{
		{`SELECT COUNT(*) FROM automation_waits_v273538 WHERE tenant_id=? AND status='pending'`, &result.ScheduledWaits},
		{`SELECT COUNT(*) FROM automation_waits_v273538 WHERE tenant_id=? AND status='pending' AND due_at<?`, &result.OverdueWaits},
		{`SELECT COUNT(*) FROM automation_waits_v273538 WHERE tenant_id=? AND status IN ('needs_review','processing')`, &result.WaitsNeedReview},
	} {
		if check.dst == &result.OverdueWaits {
			if err := a.db.QueryRow(check.query, tenantID, time.Now().UTC().Format(time.RFC3339)).Scan(check.dst); err != nil {
				return result, err
			}
		} else {
			if err := a.db.QueryRow(check.query, tenantID).Scan(check.dst); err != nil {
				return result, err
			}
		}
	}
	// The Meta Lead outbox is a separate first-stage queue, also tenant-scoped.
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM lead_automation_outbox WHERE tenant_id=? AND status='pending'`, tenantID).Scan(&result.LeadPending); err != nil {
		return result, err
	}
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM lead_automation_outbox WHERE tenant_id=? AND status='needs_review'`, tenantID).Scan(&result.LeadNeedsReview); err != nil {
		return result, err
	}
	return result, nil
}

// Pure projection function; unknown queue statuses cannot masquerade as successes.
func applyAutomationQueueCountV273537(h *automationQueueHealthV273537, state string, count int64) {
	if h == nil || count < 0 {
		return
	}
	switch state {
	case "pending":
		h.Pending += count
	case "processing":
		h.Processing += count
	case "failed":
		h.Failed += count
	case "needs_review":
		h.NeedsReview += count
	case "processed":
		h.Processed += count
	}
}
