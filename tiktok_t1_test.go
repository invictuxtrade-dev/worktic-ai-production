package main

import (
	"testing"
	"time"
)

func TestTikTokT1InitialScope(t *testing.T) {
	if tiktokT1InitialScope != "user.info.basic" {
		t.Fatalf("scope inicial demasiado amplio: %q", tiktokT1InitialScope)
	}
	for _, tt := range []struct {
		s    string
		want bool
	}{
		{"user.info.basic", true}, {"user.info.basic,video.upload", true},
		{"video.publish", false}, {"user.info.basicish", false}, {"", false},
	} {
		if got := tiktokT1HasScope(tt.s, "user.info.basic"); got != tt.want {
			t.Fatalf("scope %q got %v", tt.s, got)
		}
	}
}
func TestTikTokT1ProfileValidation(t *testing.T) {
	var p tiktokT1Profile
	p.Data.User.OpenID = "id-123"
	p.Data.User.DisplayName = "Cuenta Test"
	if _, _, err := tiktokT1ValidateProfile(p, "different"); err == nil {
		t.Fatal("token id mismatch accepted")
	}
	if id, name, err := tiktokT1ValidateProfile(p, "id-123"); err != nil || id != "id-123" || name != "Cuenta Test" {
		t.Fatalf("profile: %q %q %v", id, name, err)
	}
	p.Error.Code = "invalid_token"
	if _, _, err := tiktokT1ValidateProfile(p, "id-123"); err == nil {
		t.Fatal("API error accepted")
	}
}
func TestTikTokT1RefreshNearExpiry(t *testing.T) {
	now := time.Date(2026, 11, 12, 14, 0, 0, 0, time.UTC)
	if tiktokT1TokenNeedsRefresh(now.Add(2*time.Hour).Format(time.RFC3339), now) {
		t.Fatal("unnecessary refresh")
	}
	if !tiktokT1TokenNeedsRefresh(now.Add(4*time.Minute).Format(time.RFC3339), now) {
		t.Fatal("missing near-expiry refresh")
	}
	if !tiktokT1TokenNeedsRefresh("", now) {
		t.Fatal("missing expiry should refresh")
	}
}
