package main

import "testing"

func TestSocialB6HasScope(t *testing.T) {
	for _, tc := range []struct {
		scopes, want string
		expected     bool
	}{
		{"business_management,pages_show_list", "pages_manage_posts", false},
		{"business_management,pages_manage_posts", "pages_manage_posts", true},
		{"business_management, pages_manage_posts, pages_show_list", "pages_manage_posts", true},
		{"pages_manage_posts_fake", "pages_manage_posts", false},
		{"instagram_content_publish", "instagram_content_publish", true},
	} {
		if got := socialB6HasScope(tc.scopes, tc.want); got != tc.expected {
			t.Errorf("socialB6HasScope(%q,%q)=%v want %v", tc.scopes, tc.want, got, tc.expected)
		}
	}
}
