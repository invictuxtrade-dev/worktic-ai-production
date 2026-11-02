package main

import (
	"log"
	"time"
)

// Events interrupted while processing must be inspected by an operator.
// Repeating them automatically may duplicate a WhatsApp message, webhook,
// AI charge, or CRM action already performed before the interruption.
const automationClaimReviewAfterV273541 = 2 * time.Hour

func automationReviewCutoffV273541(now time.Time) string {
	return now.UTC().Add(-automationClaimReviewAfterV273541).Format(time.RFC3339)
}

func (a *App) reviewStaleAutomationEventsV273541() {
	// processed_at stores the claim timestamp while status is processing.
	// Legacy events claimed before V27.35.41 use created_at as fallback.
	_, err := a.db.Exec(`UPDATE automation_events
       SET status='needs_review',last_error=CASE WHEN last_error='' THEN ? ELSE last_error END
       WHERE status='processing' AND COALESCE(NULLIF(processed_at,''),created_at)<?`,
		"Ejecución interrumpida o excedió 2 horas; revisar antes de repetir acciones", automationReviewCutoffV273541(time.Now()))
	if err != nil {
		log.Printf("automation claim reconciliation: %v", err)
	}
}
