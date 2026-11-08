package main

import (
	"encoding/hex"
	"log"
	"os"
	"strings"
)

// B10.3C: Signature secret for the Instagram webhook only.
// If a dedicated Instagram app secret is configured it is the ONLY accepted
// signer for this endpoint. The previous META_APP_SECRET behavior remains
// unchanged until that new variable is explicitly configured.
// No other Meta/WhatsApp/Messenger webhook reads this variable.
func b103cSigningSecret(metaAppSecret string) (secret, source string) {
	if igSecret := strings.TrimSpace(os.Getenv("WORKTIC_IG_WEBHOOK_APP_SECRET")); igSecret != "" {
		return igSecret, "instagram"
	}
	return metaAppSecret, "meta_legacy"
}

// Returns an internal diagnostic code; empty string means signature valid.
// Never returns the body, the signature value, any account identifier or secret.
func b103cVerifySignature(body []byte, signature, secret string) string {
	if secret == "" {
		return "secret_missing"
	}
	if signature == "" {
		return "header_missing"
	}
	if !strings.HasPrefix(signature, "sha256=") || len(signature) != 71 {
		return "header_malformed"
	}
	if _, err := hex.DecodeString(strings.TrimPrefix(signature, "sha256=")); err != nil {
		return "header_malformed"
	}
	if !validMetaSignature(body, signature, secret) {
		return "signature_mismatch"
	}
	return ""
}

// Diagnostic logs are opt-in, off by default. Do not log headers or bodies.
func b103cLogSignatureFailure(reason, source string) {
	if !strings.EqualFold(strings.TrimSpace(os.Getenv("WORKTIC_IG_WEBHOOK_DIAG")), "true") {
		return
	}
	log.Printf("B103C_IG_SIGNATURE reason=%s secret_source=%s", reason, source)
}
