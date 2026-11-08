package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func b103cSignTestBody(body []byte, key string) string {
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestB103CDedicatedSecretPrecedence(t *testing.T) {
	t.Setenv("WORKTIC_IG_WEBHOOK_APP_SECRET", "ig-secret-test")
	secret, source := b103cSigningSecret("meta-secret-test")
	if secret != "ig-secret-test" || source != "instagram" {
		t.Fatalf("dedicated secret not selected: source=%q", source)
	}
	body := []byte(`{"object":"instagram","entry":[]}`)
	if reason := b103cVerifySignature(body, b103cSignTestBody(body, "ig-secret-test"), secret); reason != "" {
		t.Fatalf("dedicated signature rejected: %s", reason)
	}
	if reason := b103cVerifySignature(body, b103cSignTestBody(body, "meta-secret-test"), secret); reason != "signature_mismatch" {
		t.Fatalf("wrong signer must fail: %s", reason)
	}
	if reason := b103cVerifySignature([]byte(`tampered`), b103cSignTestBody(body, "ig-secret-test"), secret); reason != "signature_mismatch" {
		t.Fatalf("tampered body not rejected: %s", reason)
	}
}

func TestB103CLegacyFallback(t *testing.T) {
	t.Setenv("WORKTIC_IG_WEBHOOK_APP_SECRET", "")
	secret, source := b103cSigningSecret("meta-secret-test")
	if secret != "meta-secret-test" || source != "meta_legacy" {
		t.Fatalf("legacy behavior lost")
	}
	body := []byte("abc")
	if reason := b103cVerifySignature(body, b103cSignTestBody(body, secret), secret); reason != "" {
		t.Fatalf("legacy signature rejected: %s", reason)
	}
}

func TestB103CSignatureDiagnosticsFailClosed(t *testing.T) {
	body := []byte("abc")
	for _, tc := range []struct{ header, secret, want string }{
		{"", "ig-test", "header_missing"},
		{"sha1=abc", "ig-test", "header_malformed"},
		{"sha256=ffff", "ig-test", "header_malformed"},
		{"sha256=" + string(make([]byte, 64)), "ig-test", "header_malformed"},
		{b103cSignTestBody(body, "other"), "ig-test", "signature_mismatch"},
		{b103cSignTestBody(body, "ig-test"), "", "secret_missing"},
	} {
		if got := b103cVerifySignature(body, tc.header, tc.secret); got != tc.want {
			t.Errorf("diagnostic=%q want=%q", got, tc.want)
		}
	}
}
