package main

import "testing"

func TestCommercialModulePathV2722(t *testing.T) {
	cases := map[string]string{
		"/api/ads/v25/overview": "ads", "/api/analytics/v24/export.csv": "analytics",
		"/api/social/approvals": "social_governance", "/api/social/posts": "social",
		"/api/marketing/meta-leads/pages": "meta_leads", "/api/marketing/landings": "landings",
		"/api/marketing/campaigns": "marketing", "/api/automations/workflows": "automations",
		"/api/crm/contacts": "crm", "/api/catalog/upload": "catalog",
		"/api/studio/video/generate": "studio", "/api/whatsapp-marketing/v27/action": "whatsapp_marketing",
		"/api/partners/me": "", "/api/admin/plans": "", "/webhooks/meta/leads": "", "/l/some-landing": "",
	}
	for url, expected := range cases {
		if got := commercialModuleForPathV2722(url); got != expected {
			t.Errorf("%s: got %s want %s", url, got, expected)
		}
	}
}
func TestCommercialModulePlansV2722(t *testing.T) {
	for _, tc := range []struct {
		plan, module string
		allowed      bool
	}{
		{"free", "crm", true}, {"free", "ads", false},
		{"personal", "analytics", true}, {"personal", "ads", false},
		{"negocio", "ads", true}, {"business", "social_governance", true},
		{"empresa", "whatsapp_marketing", true}, {"unknown", "ads", false},
		{"personal", "ads", true},
	} {
		overrides := "{}"
		if tc.plan == "personal" && tc.module == "ads" && tc.allowed {
			overrides = `{"ads":true}`
		}
		got := commercialModuleAllowedV2722(tc.plan, tc.module, overrides)
		if got != tc.allowed {
			t.Errorf("plan=%s feature=%s want %v got %v", tc.plan, tc.module, tc.allowed, got)
		}
	}
	if commercialModuleAllowedV2722("empresa", "ads", `{"ads":false}`) {
		t.Error("admin override must revoke module")
	}
	if commercialModuleAllowedV2722("free", "ads", `{"ads":"true"}`) {
		t.Error("string must not grant access")
	}
	if commercialModuleAllowedV2722("empresa", "unlisted", "{}") {
		t.Error("unknown module should not be granted")
	}
}
