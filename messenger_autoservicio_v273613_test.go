package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMessengerSelfServicePageOwnershipV273613(t *testing.T) {
	valid := messengerOAuthPage{ID: "1234567", AccessToken: "fake", Tasks: []string{"MESSAGING", "MANAGE"}}
	if !messengerCanManagePage(valid) {
		t.Fatal("authorized page rejected")
	}
	valid.Tasks = []string{"MESSAGING"}
	if messengerCanManagePage(valid) {
		t.Fatal("page without MANAGE accepted")
	}
	valid.Tasks = []string{"MANAGE", "MESSAGING"}
	valid.AccessToken = ""
	if messengerCanManagePage(valid) {
		t.Fatal("page without token accepted")
	}
}
func TestMessengerSelfServiceWebhookSplitV273613(t *testing.T) {
	raw := []byte(`{"object":"page","entry":[{"id":"123","messaging":[{"sender":{"id":"A"}}]},{"id":"456","messaging":[{"sender":{"id":"B"}}]},{"id":"123","messaging":[{"sender":{"id":"C"}}]}]}`)
	groups, err := messengerSplitPageEntries(raw)
	if err != nil || len(groups) != 2 || len(groups["123"]) != 2 || len(groups["456"]) != 1 {
		t.Fatalf("split error: %v / %+v", err, groups)
	}
	for page, entries := range groups {
		for _, entry := range entries {
			var x struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(entry, &x); err != nil || x.ID != page {
				t.Fatalf("entry crossed page %q into %q", x.ID, page)
			}
		}
	}
	if _, err := messengerSplitPageEntries([]byte(`{"object":"page","entry":[{"messaging":[]}]}`)); err == nil {
		t.Fatal("missing page id accepted")
	}
	if _, err := messengerSplitPageEntries([]byte(`{"object":"user","entry":[]}`)); err == nil {
		t.Fatal("wrong object accepted")
	}
}
func TestMessengerSelfServiceSignatureV273613(t *testing.T) {
	if got := messengerSignedBody("secret", []byte(`{"object":"page"}`)); !strings.HasPrefix(got, "sha256=") || len(got) != 71 {
		t.Fatalf("bad signature: %q", got)
	}
}
func TestMessengerSelfServiceDisabledByDefaultV273613(t *testing.T) {
	t.Setenv("WORKTIC_MESSENGER_SELFSERVICE_ENABLED", "")
	t.Setenv("WORKTIC_MESSENGER_APP_WEBHOOK_READY", "")
	if messengerSelfServiceEnabled() || messengerWebhookConfigured() {
		t.Fatal("fail-closed flags were not off")
	}
}
