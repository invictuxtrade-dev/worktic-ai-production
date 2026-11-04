package main

// Worktic AI B1 V27.36.10 — opt-in scope for a single official WhatsApp
// business account and display phone. An enabled global switch is not enough:
// absent/mismatched allowlist values fail closed and prohibit auto-replies.
// This gate runs both at inbound enqueue and again before generation/send.

import (
	"encoding/json"
	"strings"
)

// waCloudAINormalizePhoneV273610 accepts only display-phone punctuation and
// ASCII digits. Unexpected characters or ambiguous values fail closed.
func waCloudAINormalizePhoneV273610(s string) string {
	s = strings.TrimSpace(s)
	var digits strings.Builder
	for _, ch := range s {
		switch {
		case ch >= '0' && ch <= '9':
			digits.WriteRune(ch)
		case ch == '+' || ch == ' ' || ch == '-' || ch == '(' || ch == ')':
			// Common E.164 display formatting.
		default:
			return ""
		}
	}
	v := digits.String()
	if len(v) < 8 || len(v) > 15 {
		return ""
	}
	return v
}

func waCloudAIAllDigitsV273610(s string) bool {
	if s == "" {
		return false
	}
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

// A Meta phone ID is unique among connected channels in the DB. Checking
// WABA + display phone + its mapped Meta phone ID makes the rollout specific
// to one verified official number without user access to internal DB IDs.
func waCloudAIAllowedConnectionV273610(c ChannelConnection) bool {
	if !waCloudAIEnabledV273609() || c.ID <= 0 || c.TenantID <= 0 ||
		c.Type != "whatsapp_cloud" || c.Status != "connected" || c.AssignedAgentID <= 0 {
		return false
	}
	waba := strings.TrimSpace(env("WORKTIC_WA_CLOUD_AI_ALLOWED_WABA_ID", ""))
	phone := waCloudAINormalizePhoneV273610(env("WORKTIC_WA_CLOUD_AI_ALLOWED_DISPLAY_PHONE", ""))
	if !waCloudAIAllDigitsV273610(waba) || phone == "" {
		return false
	}
	var cfg whatsappCloudConfig
	if err := json.Unmarshal([]byte(c.ConfigJSON), &cfg); err != nil {
		return false
	}
	// Never grant access using a claim from the incoming webhook. All fields
	// are sourced from the connected, tenant-scoped channel record.
	metaPhoneID := strings.TrimSpace(c.ExternalAccountID)
	if strings.TrimSpace(cfg.WABAID) != waba ||
		metaPhoneID == "" || !waCloudAIAllDigitsV273610(metaPhoneID) ||
		strings.TrimSpace(cfg.PhoneNumberID) != metaPhoneID ||
		waCloudAINormalizePhoneV273610(cfg.DisplayPhone) != phone {
		return false
	}
	return true
}
