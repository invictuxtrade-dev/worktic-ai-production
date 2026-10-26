package main

import "testing"

func TestTenantLegacyRoutesV2730(t *testing.T) {
	for _, path := range []string{"/api/connect", "/api/disconnect", "/api/qr", "/api/logout", "/api/telegram", "/api/messenger"} {
		if blocked, _ := tenantLegacyRouteDecisionV2730(path, true); !blocked {
			t.Errorf("global route %s must be blocked", path)
		}
	}
	// Existing inbox APIs already scope their SQL by tenant_id.
	for _, path := range []string{"/api/conversations", "/api/messages", "/api/read", "/api/contact", "/api/send", "/api/channels/connections", "/webhooks/meta/whatsapp"} {
		if blocked, _ := tenantLegacyRouteDecisionV2730(path, true); blocked {
			t.Errorf("tenant-scoped route %s unexpectedly blocked", path)
		}
	}
	if blocked, _ := tenantLegacyRouteDecisionV2730("/api/rules", false); !blocked {
		t.Fatal("global rules must not be exposed")
	}
	if blocked, _ := tenantLegacyRouteDecisionV2730("/api/rules", true); blocked {
		t.Fatal("tenant rules route must be available")
	}
}

func TestModuleRoutingLegacyCompatibilityV2730(t *testing.T) {
	want := map[string]string{
		"/api/channels/connections":       "channels",
		"/api/whatsapp/embedded/complete": "channels",
		"/api/telegram":                   "channels",
		"/api/messenger":                  "channels",
		"/api/send":                       "inbox",
		"/api/messages":                   "inbox",
		"/api/rules":                      "automations",
		"/api/automations/workflows":      "automations",
		"/api/crm/contacts":               "crm",
		"/api/studio/ai":                  "studio",
		"/api/ads/v25/sync":               "ads",
	}
	for path, expected := range want {
		if got := commercialModuleForPathV2722(path); got != expected {
			t.Errorf("%s: expected %s, got %s", path, expected, got)
		}
	}
	// Public callbacks cannot be gated with session-derived commercial plan.
	for _, path := range []string{"/webhooks/meta/whatsapp", "/webhooks/meta/leads", "/l/a-public-landing", "/r/partner-link"} {
		if got := commercialModuleForPathV2722(path); got != "" {
			t.Errorf("public callback %s gated as %s", path, got)
		}
	}
}
