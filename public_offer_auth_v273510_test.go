package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// A visitor without a session may read public prices and benefits, but
// never a private/administrative endpoint. Exact routes only.
func TestPublicOfferUnauthenticatedAccessV273510(t *testing.T) {
	a := &App{}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := a.authMiddleware(next)
	cases := []struct {
		method string
		path   string
		want   int
	}{
		{http.MethodGet, "/api/public/offer/v2734", http.StatusNoContent},
		{http.MethodGet, "/api/public/offer/v2734?billing=monthly", http.StatusNoContent},
		{http.MethodPost, "/api/public/offer/v2734", http.StatusUnauthorized},
		{http.MethodGet, "/api/public/offer/v2734/private", http.StatusUnauthorized},
		{http.MethodGet, "/api/admin/plans", http.StatusUnauthorized},
		{http.MethodGet, "/api/admin/full/plans", http.StatusUnauthorized},
		{http.MethodGet, "/api/billing/payments", http.StatusUnauthorized},
		{http.MethodGet, "/api/commercial/wallet", http.StatusUnauthorized},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("%s %s: got HTTP %d; want %d; body=%s", tc.method, tc.path, rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}
