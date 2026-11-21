package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Regression: voice recognition/recording is allowed only on our own origin;
// other sensitive permissions stay forbidden. User browser consent is still required.
func TestPermissionsPolicyA51OwnOriginMicrophone(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	for _, path := range []string{"/app.html", "/api/agents/builder-a1"} {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		secureHeaders(next).ServeHTTP(rr, req)
		policy := rr.Header().Get("Permissions-Policy")
		for _, want := range []string{"microphone=(self)", "camera=()", "geolocation=()", "payment=()"} {
			if !strings.Contains(policy, want) {
				t.Fatalf("path %s: missing %s in Permissions-Policy: %q", path, want, policy)
			}
		}
		if strings.Contains(policy, "microphone=()") || strings.Contains(policy, "microphone=*") {
			t.Fatalf("unsafe or disabled microphone policy in %q", policy)
		}
	}
}
