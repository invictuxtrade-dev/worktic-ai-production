package main

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

func TestB103DParseShortFlat(t *testing.T) {
	raw := []byte(`{"access_token":"TOKEN_EXAMPLE","user_id":"17841440790199494","permissions":"instagram_business_basic"}`)
	got, id, err := instagramB103DParseShort(raw)
	if err != nil || got.AccessToken != "TOKEN_EXAMPLE" || id != "17841440790199494" {
		t.Fatalf("flat parse invalid: %q %v", id, err)
	}
}
func TestB103DParseWrapped(t *testing.T) {
	raw := []byte(`{"data":[{"access_token":"TOKEN_EXAMPLE","user_id":17841440790199494,"permissions":["instagram_business_basic"]}]}`)
	_, id, err := instagramB103DParseShort(raw)
	if err != nil || id != "17841440790199494" {
		t.Fatalf("wrapped parse invalid: %q %v", id, err)
	}
}
func TestB103DRejectAmbiguousOrMissingPermission(t *testing.T) {
	cases := []string{
		`{"data":[]}`,
		`{"data":[{"access_token":"a","user_id":"123"},{"access_token":"b","user_id":"456"}]}`,
		`{"access_token":"a","user_id":"123","permissions":"instagram_business_manage_messages"}`,
		`{"access_token":"a","user_id":"not_a_number"}`,
	}
	for _, c := range cases {
		if _, _, err := instagramB103DParseShort([]byte(c)); err == nil {
			t.Fatalf("accepted invalid payload: %s", c)
		}
	}
}
func TestB103DInstagramOAuthHostAndScopes(t *testing.T) {
	t.Setenv("WORKTIC_IG_LOGIN_APP_ID", "1864897741350893")
	t.Setenv("WORKTIC_IG_LOGIN_APP_SECRET", "EXAMPLE_SECRET_NOT_REAL")
	u, err := instagramB103DAuthorizationURL(strings.Repeat("s", 32), "https://www.workticai.com/api/social/oauth/callback/instagram")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(u)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Host != "www.instagram.com" || parsed.Path != "/oauth/authorize" {
		t.Fatalf("wrong auth host: %v", parsed)
	}
	if parsed.Query().Get("scope") != "instagram_business_basic" {
		t.Fatalf("wrong scope: %v", parsed.Query().Get("scope"))
	}
	if strings.Contains(u, "EXAMPLE_SECRET_NOT_REAL") {
		t.Fatal("secret leaked to browser")
	}
}
func TestB103DIDParsing(t *testing.T) {
	if got := instagramB103DID(json.RawMessage(`17841440790199494`)); got != "17841440790199494" {
		t.Fatalf("wrong numeric ID %s", got)
	}
	if got := instagramB103DID(json.RawMessage(`"17841440790199494"`)); got != "17841440790199494" {
		t.Fatalf("wrong string ID %s", got)
	}
}
