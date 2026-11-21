package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLinkedInL1Credentials(t *testing.T) {
	if linkedinL1CredentialsReady("id", "", strings.Repeat("x", 32)) == nil {
		t.Fatal("Secret required")
	}
	if linkedinL1CredentialsReady("id", "secret", "short") == nil {
		t.Fatal("Secure encryption key required")
	}
	if err := linkedinL1CredentialsReady("id", "secret", strings.Repeat("x", 32)); err != nil {
		t.Fatal(err)
	}
}
func TestLinkedInL1LeastPrivilege(t *testing.T) {
	if linkedinL1HasScope(linkedinL1StoredScopes, "w_member_social") {
		t.Fatal("Do not claim publish scope")
	}
	if linkedinL1HasScope("w_member_socialized", "w_member_social") {
		t.Fatal("Substring matched")
	}
	if !linkedinL1HasScope("openid profile w_member_social", "w_member_social") {
		t.Fatal("Exact scope should match")
	}
	if err := linkedinL1ValidateTokenScopes("openid profile"); err != nil {
		t.Fatal(err)
	}
	if err := linkedinL1ValidateTokenScopes("openid"); err == nil {
		t.Fatal("missing profile should fail")
	}
}
func TestLinkedInL1Profile(t *testing.T) {
	if linkedinL1ValidateIdentity("") == nil {
		t.Fatal("must require stable ID")
	}
	if v := linkedinL1ProfileName("Ana", "Diaz", ""); v != "Ana Diaz" {
		t.Fatalf("name %q", v)
	}
}
func TestLinkedInL1StateCookie(t *testing.T) {
	state := strings.Repeat("s", 43)
	w := httptest.NewRecorder()
	linkedinL1IssueStateCookie(w, state, "https://www.workticai.com")
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].Secure || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteLaxMode {
		t.Fatal("invalid cookie security")
	}
	r := httptest.NewRequest(http.MethodGet, "https://www.workticai.com/api/social/oauth/callback/linkedin", nil)
	r.AddCookie(cookies[0])
	if !linkedinL1ValidStateCookie(r, state) {
		t.Fatal("valid state rejected")
	}
	if linkedinL1ValidStateCookie(r, state+"x") {
		t.Fatal("mismatched state accepted")
	}
}
