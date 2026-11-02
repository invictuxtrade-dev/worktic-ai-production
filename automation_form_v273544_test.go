package main

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestPublicFormTokenV273544(t *testing.T) {
	a, e := newPublicFormTokenV273544()
	if e != nil || len(a) != 32 {
		t.Fatalf("token error %q: %v", a, e)
	}
	b, e := newPublicFormTokenV273544()
	if e != nil || a == b {
		t.Fatal("token reused")
	}
	same, e := publicFormTokenV273544(strings.ToUpper(a))
	if e != nil || same != a {
		t.Fatalf("stable key %q", same)
	}
	fallback, e := publicFormTokenV273544("invalid")
	if e != nil || len(fallback) != 32 {
		t.Fatal("fallback invalid")
	}
}

func TestPublicFormEventPayloadV273544(t *testing.T) {
	f := LeadForm{ID: 14, TenantID: 5, Name: "Test form"}
	body := url.Values{"name": {"Ada"}, "phone": {"573001111111"}, "interest": {"demo"}, "email": {"ada@example.com"}}
	r := httptest.NewRequest("POST", "/f/5/demo", strings.NewReader(body.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	p := publicFormEventPayloadV273544(f, 67, r, "2026-10-31T00:00:00Z")
	if p["form_id"] != int64(14) || p["lead_id"] != int64(67) || p["source"] != "worktic_form" {
		t.Fatalf("payload unexpected: %#v", p)
	}
	if _, ok := p["whatsapp_opt_in"]; ok {
		t.Fatal("privacy consent must not imply WA marketing opt-in")
	}
}
