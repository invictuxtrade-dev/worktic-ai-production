package main

import (
	"errors"
	"os"
	"testing"
	"time"
)

func TestCreativeQuoteRates(t *testing.T) {
	for _, test := range []struct {
		action         string
		seconds, chars int
		expected       int64
	}{
		{"video", 5, 0, 36}, {"video", 10, 0, 99}, {"image", 0, 0, 8},
		{"copy", 0, 0, 5}, {"storyboard", 0, 0, 5},
		{"narration", 0, 1, 10}, {"narration", 0, 1000, 10}, {"narration", 0, 1001, 20},
		{"video", 15, 0, 0}, {"narration", 0, 4000, 0},
	} {
		if got := creativeQuote(test.action, test.seconds, test.chars); got != test.expected {
			t.Errorf("%s/%d/%d: got %d, want %d", test.action, test.seconds, test.chars, got, test.expected)
		}
	}
}
func TestCreativeBudgetOptInAndFailClosed(t *testing.T) {
	os.Unsetenv("WORKTIC_COST_BUDGET_PERSONAL_CENTS")
	if v := creativeTenantLimit("personal"); v != 0 {
		t.Fatalf("budget should default to 0, got %d", v)
	}
	t.Setenv("WORKTIC_COST_BUDGET_PERSONAL_CENTS", "-1")
	if v := creativeTenantLimit("personal"); v != 0 {
		t.Fatal("negative budget must fail closed")
	}
	t.Setenv("WORKTIC_COST_BUDGET_PERSONAL_CENTS", "75")
	if v := creativeTenantLimit("personal"); v != 75 {
		t.Fatalf("expected 75, got %d", v)
	}
	if v := creativeTenantLimit("free"); v != 0 {
		t.Fatal("free plan cannot spend")
	}
}
func TestCreativeMonthAndInvalidQuote(t *testing.T) {
	if v := creativeMonth(time.Date(2026, 10, 31, 23, 0, 0, 0, time.UTC)); v != "2026-10" {
		t.Fatal(v)
	}
	if _, err := creativeRequestQuote("unknown", 0, 0); err == nil {
		t.Fatal("unexpected quote")
	}
	if !errors.Is(errCreativeBudget, errCreativeBudget) {
		t.Fatal("sentinel error")
	}
}
