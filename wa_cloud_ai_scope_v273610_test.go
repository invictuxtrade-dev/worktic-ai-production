package main

import "testing"

func TestWACloudAINormalizePhoneV273610(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"+57 333 2450481", "573332450481"},
		{"57-333-2450481", "573332450481"},
		{"(57) 333 2450481", "573332450481"},
		{"+57 abc 2450481", ""},
		{"123", ""},
		{"", ""},
	} {
		if got := waCloudAINormalizePhoneV273610(tc.in); got != tc.want {
			t.Errorf("phone(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestWACloudAIAllowedConnectionV273610(t *testing.T) {
	c := ChannelConnection{
		ID: 7, TenantID: 5, Type: "whatsapp_cloud", Status: "connected",
		AssignedAgentID: 11, ExternalAccountID: "123456789987654",
		ConfigJSON: `{"waba_id":"357746977420597","phone_number_id":"123456789987654","display_phone_number":"+57 333 2450481"}`,
	}
	t.Setenv("WORKTIC_WA_CLOUD_AI_ENABLED", "true")
	t.Setenv("WORKTIC_WA_CLOUD_AI_ALLOWED_WABA_ID", "357746977420597")
	t.Setenv("WORKTIC_WA_CLOUD_AI_ALLOWED_DISPLAY_PHONE", "+57 333 2450481")
	if !waCloudAIAllowedConnectionV273610(c) {
		t.Fatal("correctly configured single channel must be allowed")
	}
	tests := []struct {
		name   string
		mutate func(*ChannelConnection)
	}{
		{"wrong tenant", func(v *ChannelConnection) { v.TenantID = 0 }},
		{"different channel", func(v *ChannelConnection) { v.ID = 0 }},
		{"disconnected", func(v *ChannelConnection) { v.Status = "draft" }},
		{"different type", func(v *ChannelConnection) { v.Type = "whatsapp" }},
		{"no agent", func(v *ChannelConnection) { v.AssignedAgentID = 0 }},
		{"different Meta phone ID", func(v *ChannelConnection) { v.ExternalAccountID = "444444444444444" }},
		{"different WABA", func(v *ChannelConnection) {
			v.ConfigJSON = `{"waba_id":"987654321000000","phone_number_id":"123456789987654","display_phone_number":"+57 333 2450481"}`
		}},
		{"different phone", func(v *ChannelConnection) {
			v.ConfigJSON = `{"waba_id":"357746977420597","phone_number_id":"123456789987654","display_phone_number":"+57 311 1111111"}`
		}},
		{"invalid config", func(v *ChannelConnection) { v.ConfigJSON = "{bad" }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v := c
			tc.mutate(&v)
			if waCloudAIAllowedConnectionV273610(v) {
				t.Fatal("failed open")
			}
		})
	}
	t.Setenv("WORKTIC_WA_CLOUD_AI_ALLOWED_WABA_ID", "")
	if waCloudAIAllowedConnectionV273610(c) {
		t.Fatal("missing WABA must fail closed")
	}
	t.Setenv("WORKTIC_WA_CLOUD_AI_ALLOWED_WABA_ID", "357746977420597")
	t.Setenv("WORKTIC_WA_CLOUD_AI_ALLOWED_DISPLAY_PHONE", "")
	if waCloudAIAllowedConnectionV273610(c) {
		t.Fatal("missing phone must fail closed")
	}
	t.Setenv("WORKTIC_WA_CLOUD_AI_ALLOWED_DISPLAY_PHONE", "+57 333 2450481")
	t.Setenv("WORKTIC_WA_CLOUD_AI_ENABLED", "false")
	if waCloudAIAllowedConnectionV273610(c) {
		t.Fatal("global off must fail closed")
	}
}
