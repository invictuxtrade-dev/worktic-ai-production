package main

import "time"

// After this period a claimed wait may have completed an outbound side effect.
// It must be inspected by an operator, NEVER automatically replayed.
const automationWaitReviewAfterV273550 = 2 * time.Hour

func automationWaitReviewCutoffV273550(now time.Time) string {
	return now.UTC().Add(-automationWaitReviewAfterV273550).Format(time.RFC3339)
}

const automationWaitReviewMessageV273550 = "Espera interrumpida durante procesamiento; comprobar efectos externos antes de cualquier nueva acción"
