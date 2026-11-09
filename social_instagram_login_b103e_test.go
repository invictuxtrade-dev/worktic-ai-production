package main

import (
	"errors"
	"strings"
	"testing"
)

func TestB103EProfileIdentityScopedVsProfessional(t *testing.T) {
	raw := []byte(`{"id":"7423920012111","user_id":"17841440790199494","username":"worktic.oficial","account_type":"BUSINESS"}`)
	name, professionalID, err := instagramB103DResolveProfile("7423920012111", raw)
	if err != nil || name != "worktic.oficial" || professionalID != "17841440790199494" {
		t.Fatalf("must accept a matching app-scoped ID while preserving professional ID: %q %q %v", name, professionalID, err)
	}
}
func TestB103EProfileRejectsUnrelatedAccount(t *testing.T) {
	raw := []byte(`{"id":"7423920012111","user_id":"17841440790199494","username":"worktic.oficial","account_type":"BUSINESS"}`)
	if _, _, err := instagramB103DResolveProfile("9999999999", raw); err == nil || err.Error() != "IG_PROFILE_ID_MISMATCH" {
		t.Fatalf("must reject unrelated account: %v", err)
	}
}
func TestB103EProfileAllowsMatchingProfessionalID(t *testing.T) {
	raw := []byte(`{"id":"7423920012111","user_id":"17841440790199494","username":"worktic.oficial","account_type":"CREATOR"}`)
	_, pro, err := instagramB103DResolveProfile("17841440790199494", raw)
	if err != nil || pro != "17841440790199494" {
		t.Fatal(err, pro)
	}
}
func TestB103EProfileRejectsMissingProfessionalID(t *testing.T) {
	raw := []byte(`{"id":"7423920012111","username":"worktic.oficial","account_type":"BUSINESS"}`)
	if _, _, err := instagramB103DResolveProfile("7423920012111", raw); err == nil || err.Error() != "IG_PROFILE_PRO_ID_MISSING" {
		t.Fatal(err)
	}
}
func TestB103EProfileRejectsPersonal(t *testing.T) {
	raw := []byte(`{"id":"7423920012111","user_id":"17841440790199494","username":"worktic.oficial","account_type":"PERSONAL"}`)
	if _, _, err := instagramB103DResolveProfile("7423920012111", raw); err == nil || err.Error() != "IG_PROFILE_NOT_PROFESSIONAL" {
		t.Fatal(err)
	}
}
func TestB103ESafeReasonsNeverLeakProviderMessages(t *testing.T) {
	for _, s := range []string{"IG_SHORT_EXCHANGE: access_token=SENSITIVE", "IG_SAVE_FAILED: client_secret=SENSITIVE", "unknown_private_token=SENSITIVE"} {
		message := instagramB103ESafeReason(errors.New(s))
		if strings.Contains(message, "SENSITIVE") || strings.Contains(message, "access_token") || strings.Contains(message, "client_secret") {
			t.Fatal(message)
		}
		if !strings.Contains(message, "[IG_") {
			t.Fatal(message)
		}
	}
}
