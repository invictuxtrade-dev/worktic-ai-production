package main

import (
	"errors"
	"strings"
	"testing"
)

func TestSocialB7CanonicalMediaURL(t *testing.T) {
	const base = "https://www.workticai.com"
	cases := []struct {
		name, raw, want string
		invalid         bool
	}{
		{"public Worktic upload", "/uploads/social/1/foto.jpg", base + "/uploads/social/1/foto.jpg", false},
		{"relative Worktic upload", "uploads/social/1/foto.jpg", base + "/uploads/social/1/foto.jpg", false},
		{"HTTPS external", "https://cdn.example.com/photo.jpg?x=1", "https://cdn.example.com/photo.jpg?x=1", false},
		{"HTTPS with anchor", "https://cdn.example.com/photo.jpg#abc", "https://cdn.example.com/photo.jpg", false},
		{"non-HTTPS", "http://cdn.example.com/photo.jpg", "", true},
		{"empty", " ", "", true},
		{"internal endpoint", "/api/auth/login", "", true},
		{"localhost", "https://localhost/uploads/a.jpg", "", true},
		{"private address", "https://192.168.1.2/uploads/a.jpg", "", true},
		{"scheme-relative", "//evil.example.com/uploads/x.jpg", "", true},
		{"travelling path", "/uploads/../api/auth/login", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := socialB7CanonicalMediaURL(base, tc.raw)
			if (err != nil) != tc.invalid {
				t.Fatalf("err=%v, expected invalid=%v", err, tc.invalid)
			}
			if !tc.invalid && got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSocialB7PermanentErrors(t *testing.T) {
	if !socialB7IsPermanentMediaURLFailure(errors.New(`Meta: {"message":"(#100) url should represent a valid URL"}`)) {
		t.Fatal("Meta invalid URL must not be retried")
	}
	if socialB7IsPermanentMediaURLFailure(errors.New("timeout talking to Meta")) {
		t.Fatal("timeout is transient")
	}
	if socialB7IsPermanentMediaURLFailure(nil) {
		t.Fatal("nil error must not be permanent")
	}
	u, err := socialB7CanonicalMediaURL("https://www.workticai.com", "/uploads/social/1/x.jpg")
	if err != nil || !strings.HasPrefix(u, "https://") {
		t.Fatalf("url=%q, err=%v", u, err)
	}
}
