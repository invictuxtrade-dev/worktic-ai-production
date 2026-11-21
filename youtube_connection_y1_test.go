package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestYouTubeY1SeparateCredentials(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "gmail-in-use")
	t.Setenv("GOOGLE_CLIENT_SECRET", "gmail-secret-in-use")
	t.Setenv("WORKTIC_YOUTUBE_CLIENT_ID", "")
	t.Setenv("WORKTIC_YOUTUBE_CLIENT_SECRET", "")
	if _, _, err := youtubeY1Credentials(); err == nil {
		t.Fatal("must not reuse Gmail credentials")
	}
	t.Setenv("WORKTIC_YOUTUBE_CLIENT_ID", "youtube-client")
	t.Setenv("WORKTIC_YOUTUBE_CLIENT_SECRET", "youtube-secret")
	id, secret, err := youtubeY1Credentials()
	if err != nil || id != "youtube-client" || secret != "youtube-secret" {
		t.Fatalf("unexpected: %s %s %v", id, secret, err)
	}
}
func TestYouTubeY1StrictScope(t *testing.T) {
	if youtubeY1ScopeIncludes("youtube.readonly", "youtube.upload") {
		t.Fatal("read-only must not allow upload")
	}
	if youtubeY1ScopeIncludes("youtube.uploaded", "youtube.upload") {
		t.Fatal("partial scope matched")
	}
	if !youtubeY1ScopeIncludes("youtube.readonly,youtube.upload", "youtube.upload") {
		t.Fatal("expected exact upload scope")
	}
}
func TestYouTubeY1RefreshExpiry(t *testing.T) {
	now := time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC)
	if youtubeY1NeedsRefresh(now.Add(30*time.Minute).Format(time.RFC3339), now) {
		t.Fatal("refreshed live token")
	}
	if !youtubeY1NeedsRefresh(now.Add(time.Minute).Format(time.RFC3339), now) {
		t.Fatal("must refresh near expiry")
	}
	if !youtubeY1NeedsRefresh("", now) {
		t.Fatal("must refresh when expiry unknown")
	}
}
func TestYouTubeY1ConsentState(t *testing.T) {
	w := httptest.NewRecorder()
	state := strings.Repeat("a", 43)
	youtubeY1IssueStateCookie(w, state, "https://www.workticai.com")
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || !cookies[0].Secure || cookies[0].SameSite != http.SameSiteLaxMode {
		t.Fatal("OAuth cookie protections absent")
	}
	r := httptest.NewRequest("GET", "https://www.workticai.com/api/social/oauth/callback/youtube", nil)
	r.AddCookie(cookies[0])
	if !youtubeY1ValidStateCookie(r, state) || youtubeY1ValidStateCookie(r, state+"b") {
		t.Fatal("OAuth callback cookie invalid")
	}
}
