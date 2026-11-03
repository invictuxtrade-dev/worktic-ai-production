package main

import "testing"

func TestWorkticLegalConsentV273600(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		accepted, authorized bool
		privacy, terms       string
		valid                bool
	}{
		{"valid", true, true, workticLegalVersionV273600, workticLegalVersionV273600, true},
		{"missing_terms", false, true, workticLegalVersionV273600, workticLegalVersionV273600, false},
		{"missing_privacy", true, false, workticLegalVersionV273600, workticLegalVersionV273600, false},
		{"stale_policy", true, true, "2025-old", workticLegalVersionV273600, false},
		{"stale_terms", true, true, workticLegalVersionV273600, "2025-old", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := workticLegalRegistrationValidV273600(tc.accepted, tc.authorized, tc.privacy, tc.terms)
			if (err == nil) != tc.valid {
				t.Fatalf("unexpected validation result: %v", err)
			}
		})
	}
}
