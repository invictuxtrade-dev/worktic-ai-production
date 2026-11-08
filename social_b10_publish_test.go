package main

import "testing"

// B10 reuses B7's validated public-media URL contract for both Facebook and Instagram.
// This prevents passing internal /uploads paths directly to Instagram Graph API.
func TestB10InstagramMediaCanonicalURLs(t *testing.T) {
	base := "https://www.workticai.com"
	cases := []struct {
		name, raw, want string
		bad             bool
	}{
		{"image", "/uploads/social/1/foto.jpg", base + "/uploads/social/1/foto.jpg", false},
		{"video", "uploads/social/1/reel.mp4", base + "/uploads/social/1/reel.mp4", false},
		{"carousel", "/uploads/social/1/second.jpg", base + "/uploads/social/1/second.jpg", false},
		{"external", "https://cdn.example.com/ig.jpg", "https://cdn.example.com/ig.jpg", false},
		{"internal", "/api/social/posts", "", true},
		{"http", "http://cdn.example.com/ig.jpg", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := socialB7CanonicalMediaURL(base, tc.raw)
			if (err != nil) != tc.bad || (!tc.bad && got != tc.want) {
				t.Fatalf("got=%q err=%v want=%q bad=%v", got, err, tc.want, tc.bad)
			}
		})
	}
}
