package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestB9InstagramAccountID(t *testing.T) {
	for _, c := range []struct {
		v     string
		valid bool
	}{
		{"17841400000000000", true}, {"", false}, {"anything", false}, {"12/../../", false}, {"-123", false}, {strings.Repeat("1", 129), false},
	} {
		if isMetaAccountID(c.v) != c.valid {
			t.Fatalf("unexpected id validation: %q", c.v)
		}
	}
}

func TestB9TemplateEscapesInstagramUsername(t *testing.T) {
	var out bytes.Buffer
	err := instagramB9SelectTemplate.Execute(&out, struct {
		Action, State string
		Accounts      []instagramB9Account
	}{"https://www.workticai.com/api/social/oauth/callback/instagram", "state", []instagramB9Account{{ID: "123", Username: `<script>alert(1)</script>`}}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "<script>") || !strings.Contains(out.String(), "&lt;script&gt;") {
		t.Fatal("username was not escaped")
	}
}
