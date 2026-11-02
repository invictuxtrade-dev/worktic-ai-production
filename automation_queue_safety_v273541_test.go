package main

import (
	"testing"
	"time"
)

func TestAutomationReviewCutoffV273541(t *testing.T) {
	now := time.Date(2026, 10, 31, 23, 0, 0, 0, time.FixedZone("COL", -5*3600))
	got := automationReviewCutoffV273541(now)
	want := "2026-11-01T02:00:00Z" // 23:00 in COL = 04:00 UTC; review after 2h
	if got != want {
		t.Fatalf("cutoff=%s want=%s", got, want)
	}
	if automationClaimReviewAfterV273541 != 2*time.Hour {
		t.Fatal("unexpected review timeout")
	}
}
func TestAutomationReviewCutoffBoundaryV273541(t *testing.T) {
	now := time.Date(2026, 10, 31, 23, 0, 0, 0, time.UTC)
	got := automationReviewCutoffV273541(now)
	if got != "2026-10-31T21:00:00Z" {
		t.Fatalf("got %s", got)
	}
}
