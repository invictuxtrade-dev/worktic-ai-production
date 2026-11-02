package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestAutomationWhatsAppWebhookSignatureFailClosedV273546(t *testing.T) {
	body := []byte(`{"object":"whatsapp_business_account"}`)
	if validMetaSignature(body, "", "") {
		t.Fatal("missing Meta app secret must reject webhook")
	}
	if validMetaSignature(body, "", "secret") {
		t.Fatal("missing signature must reject webhook")
	}
	mac := hmac.New(sha256.New, []byte("secret"))
	_, _ = mac.Write(body)
	header := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if !validMetaSignature(body, header, "secret") {
		t.Fatal("valid HMAC signature rejected")
	}
	if validMetaSignature([]byte(`tampered`), header, "secret") {
		t.Fatal("tampered request accepted")
	}
}
