package main

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestAutomationWhatsAppTimestampV273546(t *testing.T) {
	now := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	if err := automationWAWithinWindowV273546(now.Add(-22*time.Hour).Format(time.RFC3339), now); err != nil {
		t.Fatal(err)
	}
	if err := automationWAWithinWindowV273546("", now); err == nil {
		t.Fatal("missing time accepted")
	}
	if err := automationWAWithinWindowV273546(now.Add(-24*time.Hour).Format(time.RFC3339), now); err == nil {
		t.Fatal("old window accepted")
	}
	if err := automationWAWithinWindowV273546(now.Add(6*time.Minute).Format(time.RFC3339), now); err == nil {
		t.Fatal("future accepted")
	}
	if err := automationWAWithinWindowV273546(strings.TrimSpace(" "+now.Add(-2*time.Hour).Format(time.RFC3339)+" "), now); err != nil {
		t.Fatal(err)
	}
	if err := automationWAWithinWindowV273546("garbage", now); err == nil {
		t.Fatal("invalid accepted")
	}
	if err := automationWAWithinWindowV273546(stringIntV273546(now.Unix()-300), now); err != nil {
		t.Fatalf("unix timestamp rejected: %v", err)
	}
}

func stringIntV273546(i int64) string { return strconv.FormatInt(i, 10) }

func TestAutomationWhatsAppIdentityV273546(t *testing.T) {
	ok := map[string]any{"source": "inbox", "event_type": "message.received", "event": map[string]any{"channel": "whatsapp_cloud", "connection_id": float64(7), "chat": "wacloud:573001234567", "message_id": "wamid.123"}}
	if _, _, err := automationWAReplyIdentityV273546(ok, 7, "+573001234567"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := automationWAReplyIdentityV273546(ok, 8, "573001234567"); err == nil {
		t.Fatal("other connection accepted")
	}
	if _, _, err := automationWAReplyIdentityV273546(ok, 7, "573001234568"); err == nil {
		t.Fatal("other recipient accepted")
	}
	ok["source"] = "manual"
	if _, _, err := automationWAReplyIdentityV273546(ok, 7, "573001234567"); err == nil {
		t.Fatal("manual request accepted")
	}
	ok["source"] = "inbox"
	ev := ok["event"].(map[string]any)
	delete(ev, "message_id")
	if _, _, err := automationWAReplyIdentityV273546(ok, 7, "573001234567"); err == nil {
		t.Fatal("unsigned event accepted")
	}
}
func TestAutomationWAOptOutV273546(t *testing.T) {
	for _, txt := range []string{"STOP", " baja ", "Unsubscribe", "no más"} {
		if !automationWAIsOptOutV273546(txt) {
			t.Fatalf("missed optout %q", txt)
		}
	}
	if automationWAIsOptOutV273546("quiero soporte") {
		t.Fatal("false positive")
	}
}
