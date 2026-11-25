package main

import "testing"

func TestTikTokP2MediaTenantIsolation(t *testing.T) {
	cases := []string{
		"https://www.workticai.com/uploads/social/2/sample.mp4", // different tenant
		"https://workticai.com/uploads/social/1/sample.mp4",     // redirects
		"https://www.workticai.com/uploads/social/1/../2/sample.mp4",
		"https://www.workticai.com/uploads/social/1/%2e%2e/sample.mp4",
		"https://www.workticai.com/uploads/social/1/sample.mp4?download=1",
		"https://www.workticai.com/uploads/social/1/sample.mov",
		"https://www.workticai.com/uploads/social/1/sample.mp4/other",
		"https://www.workticai.com.evil.com/uploads/social/1/sample.mp4",
		"http://www.workticai.com/uploads/social/1/sample.mp4",
		"https://www.workticai.com/uploads/social/1/.hidden.mp4",
		"https://www.workticai.com/uploads/social/11/sample.mp4",
	}
	for _, raw := range cases {
		if _, _, err := tiktokP2TenantMediaPath(1, raw, ""); err == nil {
			t.Errorf("accepted invalid media URL %q", raw)
		}
	}
	allowed := "https://www.workticai.com/uploads/social/1/1795616414592798570_1.mp4"
	path, name, err := tiktokP2TenantMediaPath(1, allowed, "")
	if err != nil || path != "/uploads/social/1/1795616414592798570_1.mp4" || name != "1795616414592798570_1.mp4" {
		t.Fatalf("expected tenant owned URL, got %q / %q / %v", path, name, err)
	}
}

func TestTikTokP2MediaConfiguredOrigin(t *testing.T) {
	url := "https://videos.workticai.com/uploads/social/8/test-01.mp4"
	if _, _, err := tiktokP2TenantMediaPath(8, url, "https://videos.workticai.com/uploads/social/"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := tiktokP2TenantMediaPath(8, "https://www.workticai.com/uploads/social/8/test-01.mp4", "https://videos.workticai.com/uploads/social/"); err == nil {
		t.Fatal("mismatched host should fail")
	}
}
