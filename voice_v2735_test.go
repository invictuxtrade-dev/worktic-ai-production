package main

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestVoiceNumberV2735(t *testing.T) {
	for _, tc := range []struct {
		n     string
		valid bool
	}{{"+573001234567", true}, {"+12025550123", true}, {"3001234567", false}, {"+abc", false}, {"+57+3001234567", false}, {"", false}} {
		if got := voiceNumber(tc.n); got != tc.valid {
			t.Errorf("phone %s got %v", tc.n, got)
		}
	}
}
func TestVoicePackagesV2735(t *testing.T) {
	pk := voicePackages(37)
	if len(pk) != 3 {
		t.Fatal("wrong number of packages")
	}
	if pk[0]["credits"] != int64(370) || pk[2]["credits"] != int64(3700) {
		t.Fatal("packages do not track admin price")
	}
}
func TestVoiceOpenAIWebhookSignatureV2735(t *testing.T) {
	secret := []byte("test-secret-not-real")
	t.Setenv("OPENAI_WEBHOOK_SECRET", "whsec_"+base64.StdEncoding.EncodeToString(secret))
	raw := []byte(`{"id":"evt_123","type":"live.transport.incoming"}`)
	id := "wh_test"
	stamp := time.Now().Unix()
	timestamp := strconvFormatIntVoiceTest(stamp)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(id + "." + timestamp + "." + string(raw)))
	req := httptest.NewRequest("POST", "https://worktic.test/webhooks/voice/openai", strings.NewReader(string(raw)))
	req.Header.Set("webhook-id", id)
	req.Header.Set("webhook-timestamp", timestamp)
	req.Header.Set("webhook-signature", "v1,"+base64.StdEncoding.EncodeToString(mac.Sum(nil)))
	if !voiceOpenAISignatureValid(req, raw) {
		t.Fatal("valid signature rejected")
	}
	raw[0] = '!'
	if voiceOpenAISignatureValid(req, raw) {
		t.Fatal("tampered payload accepted")
	}
}
func strconvFormatIntVoiceTest(n int64) string { return fmt.Sprintf("%d", n) }
func TestTwilioInboundSignatureV2735(t *testing.T) {
	t.Setenv("TWILIO_AUTH_TOKEN", "dummy-secret")
	t.Setenv("WORKTIC_PUBLIC_BASE_URL", "https://worktic.test")
	vals := "CallSid=CA123&From=%2B573001234567&To=%2B573009876543"
	req := httptest.NewRequest("POST", "https://worktic.test/webhooks/voice/twilio/inbound", strings.NewReader(vals))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// Twilio sorts decoded POST parameter names.
	input := "https://worktic.test/webhooks/voice/twilio/inboundCallSidCA123From+573001234567To+573009876543"
	mac := hmac.New(sha1.New, []byte("dummy-secret"))
	mac.Write([]byte(input))
	req.Header.Set("X-Twilio-Signature", base64.StdEncoding.EncodeToString(mac.Sum(nil)))
	if !voiceTwilioValid(req) {
		t.Fatal("signed provider request rejected")
	}
	req.Header.Set("X-Twilio-Signature", "forged")
	if voiceTwilioValid(req) {
		t.Fatal("forged provider request accepted")
	}
}
