package main

import (
	"strings"
	"testing"
)

func TestTikTokP2PrivacyRequiresExplicitChoice(t *testing.T) {
	c := tiktokP2Creator{PrivacyLevelOptions: []string{"SELF_ONLY", "MUTUAL_FOLLOW_FRIENDS"}}
	if tiktokP2ValidateDirect(tiktokP2Submission{}, c) == nil {
		t.Fatal("must require selected privacy")
	}
	if tiktokP2ValidateDirect(tiktokP2Submission{PrivacyLevel: "PUBLIC_TO_EVERYONE"}, c) == nil {
		t.Fatal("must reject ungranted privacy")
	}
	if e := tiktokP2ValidateDirect(tiktokP2Submission{PrivacyLevel: "SELF_ONLY"}, c); e != nil {
		t.Fatal(e)
	}
}
func TestTikTokP2CommercialPrivacyAndInteractions(t *testing.T) {
	c := tiktokP2Creator{PrivacyLevelOptions: []string{"SELF_ONLY", "PUBLIC_TO_EVERYONE"}, CommentDisabled: true, DuetDisabled: true}
	if tiktokP2ValidateDirect(tiktokP2Submission{PrivacyLevel: "SELF_ONLY", BrandContent: true}, c) == nil {
		t.Fatal("branded private forbidden")
	}
	if tiktokP2ValidateDirect(tiktokP2Submission{PrivacyLevel: "PUBLIC_TO_EVERYONE", AllowComment: true}, c) == nil {
		t.Fatal("disabled comment cannot be enabled")
	}
	if tiktokP2ValidateDirect(tiktokP2Submission{PrivacyLevel: "PUBLIC_TO_EVERYONE", AllowDuet: true}, c) == nil {
		t.Fatal("disabled duet cannot be enabled")
	}
}
func TestTikTokP2VerifiedURL(t *testing.T) {
	prefix := "https://www.workticai.com/tiktok-videos/"
	for _, s := range []string{"http://www.workticai.com/tiktok-videos/1.mp4", "https://www.workticai.com.evil.example/tiktok-videos/1.mp4", "https://www.workticai.com/tiktok-videos-evil/1.mp4", "https://user:pass@www.workticai.com/tiktok-videos/1.mp4"} {
		if tiktokP2VerifiedVideoURL(s, prefix) == nil {
			t.Errorf("accepted invalid %s", s)
		}
	}
	if e := tiktokP2VerifiedVideoURL(prefix+"sample.mp4", prefix); e != nil {
		t.Fatal(e)
	}
}
func TestTikTokP2ProviderErrorParsing(t *testing.T) {
	var d struct {
		PublishID string `json:"publish_id"`
	}
	if tiktokP2Parse([]byte(`{"data":{"publish_id":"a"},"error":{"code":"scope_not_authorized"}}`), &d) == nil {
		t.Fatal("must reject provider error")
	}
	if e := tiktokP2Parse([]byte(`{"data":{"publish_id":"a"},"error":{"code":"ok"}}`), &d); e != nil || d.PublishID != "a" {
		t.Fatal(e, d)
	}
	if !strings.Contains(tiktokP2SafeCode("token:SECRET"), "provider_error") {
		t.Fatal("should sanitize codes")
	}
}
