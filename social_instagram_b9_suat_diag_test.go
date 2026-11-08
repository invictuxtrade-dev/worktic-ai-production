package main

import (
	"strings"
	"testing"
)

func TestB9SUATDiagnosticRedacted(t *testing.T) {
	s := b9SignalsString(b9SUATSignals{"SYSTEM_USER", "YES", "YES", "YES", "PRESENT", "PRESENT"})
	for _, v := range []string{"SYSTEM_USER", "ig_targets=PRESENT", "app=YES", "basic=YES"} {
		if !strings.Contains(s, v) {
			t.Fatalf("missing diagnostic %s", v)
		}
	}
	if strings.Contains(s, "17841440790199494") {
		t.Fatal("account id leaked")
	}
}
func TestB9SUATTypeAllowlist(t *testing.T) {
	if b9SafeTokenType("BAD_SECRET_USER_ID") != "OTHER" {
		t.Fatal("untrusted token type passed through")
	}
}
