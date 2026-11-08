package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestB103SignatureFailClosed(t *testing.T) {
	b := []byte(`{"object":"instagram","entry":[]}`)
	if validMetaSignature(b, "", "abc") || validMetaSignature(b, "sha256=00", "abc") || validMetaSignature(b, "sha256=00", "") {
		t.Fatal("signature bypass")
	}
	mac := hmac.New(sha256.New, []byte("abc"))
	_, _ = mac.Write(b)
	if !validMetaSignature(b, "sha256="+hex.EncodeToString(mac.Sum(nil)), "abc") {
		t.Fatal("valid signature rejected")
	}
}
func TestB103DisabledFailsClosed(t *testing.T) {
	t.Setenv("WORKTIC_IG_WEBHOOK_B103_ENABLED", "false")
	t.Setenv("WORKTIC_IG_WEBHOOK_VERIFY_TOKEN", "test")
	a := &App{}
	req := httptest.NewRequest(http.MethodGet, "/webhooks/meta/instagram?hub.mode=subscribe&hub.verify_token=test&hub.challenge=abc", nil)
	w := httptest.NewRecorder()
	a.instagramWebhookB103(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("got %d", w.Code)
	}
	if strings.Contains(w.Body.String(), "abc") {
		t.Fatal("challenge leaked while disabled")
	}
}
