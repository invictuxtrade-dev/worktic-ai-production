package main

import (
	"testing"
	"time"
)

func TestAutomationWaitReviewCutoffV273550(t *testing.T) {
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.FixedZone("Bogota", -5*60*60))
	got := automationWaitReviewCutoffV273550(now)
	if want := "2026-11-01T15:00:00Z"; got != want {
		t.Fatalf("cutoff=%q, want %q", got, want)
	}
}

func TestAutomationWaitReviewTimeBoundV273550(t *testing.T) {
	if automationWaitReviewAfterV273550 != 2*time.Hour {
		t.Fatal("must allow 2h for active work")
	}
}

func TestAutomationWaitReconciliationMessageV273550(t *testing.T) {
	if automationWaitReviewMessageV273550 == "" {
		t.Fatal("operator message required")
	}
}
