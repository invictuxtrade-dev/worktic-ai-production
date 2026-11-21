package main

import (
	"strings"
	"testing"
)

func TestLinkedInL2VerifyReturnedScopes(t *testing.T) {
	for _, input := range []string{"openid profile w_member_social", "openid,profile,w_member_social", "w_member_social,openid,profile"} {
		if err := linkedinL2VerifyReturnedScopes(input); err != nil {
			t.Errorf("valid %q rejected: %v", input, err)
		}
	}
	for _, input := range []string{"", "openid profile", "openid profile w_member_social_extra", "w_member_social", "openid openid w_member_social"} {
		if err := linkedinL2VerifyReturnedScopes(input); err == nil {
			t.Errorf("invalid %q accepted", input)
		}
	}
}

func TestLinkedInL2PersonURN(t *testing.T) {
	if s, err := linkedinL2PersonURN("8Gz9zmaOTY"); err != nil || s != "urn:li:person:8Gz9zmaOTY" {
		t.Errorf("unexpected: %q, %v", s, err)
	}
	for _, id := range []string{"", "urn:li:organization:123", "../user", "a b", strings.Repeat("x", 129)} {
		if _, err := linkedinL2PersonURN(id); err == nil {
			t.Errorf("invalid person ID %q accepted", id)
		}
	}
}

func TestLinkedInL2PublishGuards(t *testing.T) {
	scope := "openid profile w_member_social"
	urn := "urn:li:person:8Gz9zmaOTY"
	if err := linkedinL2ValidatePublishRequest(scope, linkedinL2StoredScopes, urn, "post", "[]"); err != nil {
		t.Fatal(err)
	}
	for _, args := range []struct{ ts, cs, urn, format, media string }{
		{"openid profile", linkedinL2StoredScopes, urn, "post", "[]"},
		{scope, "openid,profile", urn, "post", "[]"},
		{scope, linkedinL2StoredScopes, "", "post", "[]"},
		{scope, linkedinL2StoredScopes, urn, "video", "[]"},
		{scope, linkedinL2StoredScopes, urn, "post", `[{"url":"https://example.org/image.jpg"}]`},
		{scope, linkedinL2StoredScopes, urn, "post", "not-json"},
	} {
		if err := linkedinL2ValidatePublishRequest(args.ts, args.cs, args.urn, args.format, args.media); err == nil {
			t.Errorf("unsafe publish accepted: %+v", args)
		}
	}
}
