package main

import (
	"os"
	"testing"
)

func TestResourceQuotaV2724FailClosed(t *testing.T) {
	k := "WORKTIC_ADS_SYNCS_NEGOCIO_V2724"
	old, exists := os.LookupEnv(k)
	defer func() {
		if exists {
			_ = os.Setenv(k, old)
		} else {
			_ = os.Unsetenv(k)
		}
	}()
	_ = os.Unsetenv(k)
	if resourceQuotaEnvV2724("business", "ads_syncs") != 0 {
		t.Fatal("missing config must default to 0")
	}
	_ = os.Setenv(k, "-1")
	if resourceQuotaEnvV2724("business", "ads_syncs") != 0 {
		t.Fatal("negative config accepted")
	}
	_ = os.Setenv(k, "42")
	if resourceQuotaEnvV2724("business", "ads_syncs") != 42 {
		t.Fatal("negocio alias not resolved")
	}
	if resourceQuotaEnvV2724("free", "ads_syncs") != 0 {
		t.Fatal("free must not consume metered services")
	}
	_ = os.Setenv(k, "9999999999999")
	if resourceQuotaEnvV2724("business", "ads_syncs") != 0 {
		t.Fatal("oversized config accepted")
	}
}
func TestResourceGuardV2724OptIn(t *testing.T) {
	key1, key2 := "WORKTIC_RESOURCE_GUARD_V2724", "WORKTIC_COMMERCIAL_ENABLE"
	a, aok := os.LookupEnv(key1)
	b, bok := os.LookupEnv(key2)
	defer func() {
		if aok {
			_ = os.Setenv(key1, a)
		} else {
			_ = os.Unsetenv(key1)
		}
		if bok {
			_ = os.Setenv(key2, b)
		} else {
			_ = os.Unsetenv(key2)
		}
	}()
	_ = os.Setenv(key1, "1")
	_ = os.Setenv(key2, "0")
	if resourceGuardV2724Enabled() {
		t.Fatal("requires commercial flag")
	}
	_ = os.Setenv(key2, "1")
	if !resourceGuardV2724Enabled() {
		t.Fatal("both flags should enable")
	}
}
