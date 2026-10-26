package main

import (
	"strings"
	"testing"
	"time"
)

func TestBillingLifecycleV2728Policies(t *testing.T) {
	now := time.Date(2026, 10, 23, 15, 0, 0, 0, time.UTC)
	future := now.AddDate(0, 1, 0)
	cases := []struct {
		name, old, target, cycle, want string
		end                            time.Time
		blocked                        bool
	}{
		{"new", "", "personal", "monthly", "new_subscription", time.Time{}, false},
		{"from_free", "free", "business", "annual", "new_subscription", future, false},
		{"renew_annual", "business", "business", "annual", "renewal", future, false},
		{"reactivate", "business", "business", "monthly", "reactivation", now.Add(-time.Hour), false},
		{"upgrade_block", "personal", "business", "monthly", "blocked_change", future, true},
		{"downgrade_block", "enterprise", "personal", "annual", "blocked_change", future, true},
		{"after_expiry", "enterprise", "personal", "annual", "new_subscription", now.Add(-time.Hour), false},
		{"invalid_cycle", "personal", "personal", "", "", future, true},
		{"invalid_plan", "free", "other", "monthly", "", future, true},
		{"free_destination", "business", "free", "annual", "", future, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := billingTransitionV2728(tc.old, tc.end, tc.target, tc.cycle, now)
			if got != tc.want || (err != nil) != tc.blocked {
				t.Fatalf("got=%s err=%v", got, err)
			}
		})
	}
}
func TestBillingCourtesyV2728Policies(t *testing.T) {
	now := time.Date(2026, 10, 23, 15, 0, 0, 0, time.UTC)
	future := now.AddDate(0, 2, 0)
	cases := []struct {
		old, source, target string
		end, proposed       time.Time
		reject              bool
	}{
		{"enterprise", "payment", "business", future, future, true},
		{"business", "payment", "business", future, future.AddDate(0, -1, 0), true},
		{"business", "payment", "business", future, future.AddDate(0, 1, 0), true},
		{"business", "courtesy", "personal", future, future.AddDate(0, -1, 0), false},
		{"free", "payment", "personal", future, future, false},
		{"business", "payment", "personal", now.Add(-time.Hour), future, false},
		{"enterprise", "payment", "enterprise", future, now.Add(-time.Hour), true},
	}
	for i, tc := range cases {
		err := billingCourtesyAllowedV2728(tc.old, tc.source, tc.end, tc.target, tc.proposed, now)
		if (err != nil) != tc.reject {
			t.Errorf("case %d: err=%v", i, err)
		}
	}
}
func TestBillingHashV2728(t *testing.T) {
	hash := "0x" + strings.Repeat("A", 64)
	if got := billingNormalizedHashV2728("BEP20", "  "+hash+"   "); got != "0x"+strings.Repeat("a", 64) {
		t.Fatal(got)
	}
	if !commercialTxHash("BEP20", hash) {
		t.Fatal("BEP20 válido rechazado")
	}
	if !commercialTxHash("TRC20", strings.Repeat("a", 64)) {
		t.Fatal("TRC20 válido rechazado")
	}
	if commercialTxHash("TRC20", hash) {
		t.Fatal("TRC20 con 0x aceptado")
	}
	for _, cycle := range []string{"monthly", "annual"} {
		if !billingCycleValidV2728(cycle) {
			t.Fatal(cycle)
		}
	}
	for _, cycle := range []string{"", "Monthly", "unverified", "trial"} {
		if billingCycleValidV2728(cycle) {
			t.Fatal(cycle)
		}
	}
}
