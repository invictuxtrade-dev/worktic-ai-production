package main

import (
	"encoding/json"
	"testing"
)

func TestWACloudAIChannelIdentityV273611(t *testing.T) {
	cfg, _ := json.Marshal(whatsappCloudConfig{
		WABAID: "123456789", PhoneNumberID: "987654321", DisplayPhone: "+57 (333) 245-0481",
	})
	c := ChannelConnection{
		ID: 11, TenantID: 22, Type: "whatsapp_cloud", Status: "connected",
		AssignedAgentID: 33, ExternalAccountID: "987654321", ConfigJSON: string(cfg),
	}
	if waba, phone, ok := waCloudAIChannelIdentityV273611(c); !ok || waba != "123456789" || phone != "987654321" {
		t.Fatalf("valid tenant-bound channel rejected: %v %v %v", waba, phone, ok)
	}
	cases := map[string]func(*ChannelConnection){
		"wrong_tenant":     func(c *ChannelConnection) { c.TenantID = 0 },
		"wrong_connection": func(c *ChannelConnection) { c.ID = 0 },
		"wrong_type":       func(c *ChannelConnection) { c.Type = "whatsapp_qr" },
		"paused":           func(c *ChannelConnection) { c.Status = "paused" },
		"no_agent":         func(c *ChannelConnection) { c.AssignedAgentID = 0 },
		"wrong_phone":      func(c *ChannelConnection) { c.ExternalAccountID = "0000" },
		"missing_config":   func(c *ChannelConnection) { c.ConfigJSON = "{}" },
		"malformed_config": func(c *ChannelConnection) { c.ConfigJSON = "not-json" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			changed := c
			mutate(&changed)
			if _, _, ok := waCloudAIChannelIdentityV273611(changed); ok {
				t.Fatal("untrusted channel was allowed")
			}
		})
	}
}

func TestWACloudAICustomerRolloutDefaultDisabledV273611(t *testing.T) {
	t.Setenv("WORKTIC_WA_CLOUD_AI_CUSTOMER_ROLLOUT", "false")
	if waCloudAICustomerRolloutV273611() {
		t.Fatal("customer rollout must be off unless explicitly enabled")
	}
	t.Setenv("WORKTIC_WA_CLOUD_AI_CUSTOMER_ROLLOUT", "true")
	if !waCloudAICustomerRolloutV273611() {
		t.Fatal("customer rollout did not read explicit true")
	}
}
