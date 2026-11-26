package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTikTokSBSecurityChecks(t *testing.T) {
	for _, tc := range []struct {
		name, origin, action, header, method string
		want                                 bool
	}{
		{"same origin", "https://www.workticai.com", "submit", "submit", "POST", true},
		{"apex rejected", "https://workticai.com", "submit", "submit", "POST", false},
		{"untrusted origin", "https://evil.example", "submit", "submit", "POST", false},
		{"missing origin", "", "submit", "submit", "POST", false},
		{"no action header", "https://www.workticai.com", "submit", "", "POST", false},
		{"wrong action", "https://www.workticai.com", "submit", "authorize", "POST", false},
		{"get rejected", "https://www.workticai.com", "submit", "submit", "GET", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, "https://www.workticai.com/api/social/tiktok/sandbox", nil)
			if tc.origin != "" {
				r.Header.Set("Origin", tc.origin)
			}
			if tc.header != "" {
				r.Header.Set("X-Worktic-TikTok-Sandbox", tc.header)
			}
			got := tiktokSBCheckOrigin(r, tc.action)
			if got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}
func TestTikTokSBScopeIsolation(t *testing.T) {
	if tiktokSBScopesGranted("user.info.basic") {
		t.Fatal("Profile-only cannot publish")
	}
	if !tiktokSBScopesGranted("user.info.basic,video.upload,video.publish") {
		t.Fatal("All scopes should pass")
	}
}
func TestTikTokSBPrivateOnly(t *testing.T) {
	base := tiktokSBSubmitRequest{VideoURL: "https://www.workticai.com/uploads/social/1/a.mp4", Title: "prueba", PrivacyLevel: "SELF_ONLY", Mode: "direct", RequestID: "abcdefgh123456", Confirmed: true}
	if err := tiktokSBValidateSubmit(base); err != nil {
		t.Fatal(err)
	}
	bad := base
	bad.PrivacyLevel = "PUBLIC_TO_EVERYONE"
	if tiktokSBValidateSubmit(bad) == nil {
		t.Fatal("Public posts must be blocked")
	}
	bad = base
	bad.VideoURL = "https://example.com/a.mp4"
	if tiktokSBValidateSubmit(bad) == nil {
		t.Fatal("External URL must be blocked")
	}
	bad = base
	bad.RequestID = "abc"
	if tiktokSBValidateSubmit(bad) == nil {
		t.Fatal("Invalid request ID must be blocked")
	}
	bad = base
	bad.Confirmed = false
	if tiktokSBValidateSubmit(bad) == nil {
		t.Fatal("Explicit consent is mandatory")
	}
}
func TestTikTokSBStateCookie(t *testing.T) {
	state := "tsb_" + strings.Repeat("a", 43)
	r := httptest.NewRequest("GET", tiktokSBCallbackURL+"?state="+state, nil)
	r.AddCookie(tiktokSBCookie(state, 600))
	if !tiktokSBStateMatches(r, state) {
		t.Fatal("Valid state failed")
	}
	if tiktokSBStateMatches(r, "tsb_"+strings.Repeat("b", 43)) {
		t.Fatal("Mismatched state accepted")
	}
	c := tiktokSBCookie(state, 600)
	if !c.Secure || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode {
		t.Fatal("Cookie security flags missing")
	}
}
