package main

import (
	"testing"
	"time"
)

func TestFinanceCyclesV2725(t *testing.T) {
	start := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	annual, err := financePaymentEndV2725(start, "annual", 30)
	if err != nil || !annual.Equal(start.AddDate(1, 0, 0)) {
		t.Fatalf("anual: %v %s", err, annual)
	}
	monthly, err := financePaymentEndV2725(start, "monthly", 30)
	if err != nil || !monthly.Equal(start.AddDate(0, 0, 30)) {
		t.Fatalf("mensual: %v %s", err, monthly)
	}
	for _, v := range []string{"unverified", "", "annual_fake"} {
		if _, err := financePaymentEndV2725(start, v, 30); err == nil {
			t.Fatalf("ciclo inválido aprobado: %q", v)
		}
	}
}
func TestFinanceReferencesV2725(t *testing.T) {
	if !financeInputOK("luma", "luma-req-123", "2026-10") {
		t.Fatal("rechaza entrada válida")
	}
	for _, tc := range []struct{ p, r, m string }{{"fake", "luma-req-123", "2026-10"}, {"luma", "!!!", "2026-10"}, {"luma", "luma-req-123", "2026-13"}} {
		if financeInputOK(tc.p, tc.r, tc.m) {
			t.Fatalf("aceptó inválido %+v", tc)
		}
	}
}
