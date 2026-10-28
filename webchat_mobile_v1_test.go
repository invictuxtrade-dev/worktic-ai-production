package main

import "testing"

func TestWebChatHashDoesNotExposeToken(t *testing.T) {
	token := "example-high-entropy-token-for-visitor-12345"
	if webHash(token) == token || len(webHash(token)) != 64 {
		t.Fatal("invalid token hash")
	}
	if webHash(token) != webHash(token) || webHash(token) == webHash(token+"a") {
		t.Fatal("token hash mismatch")
	}
}
func TestWebChatRateLimited(t *testing.T) {
	key := "webchat-test-" + randomToken(10)
	if !webRateLimit(key) {
		t.Fatal("first attempt unexpectedly rejected")
	}
	if webRateLimit(key) {
		t.Fatal("second attempt should be rate limited")
	}
}
