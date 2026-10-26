package main

import (
	"os"
	"testing"
)

func TestAutomationQuotaV2723FailsClosed(t *testing.T) {
	key := "WORKTIC_AUTO_RUNS_PERSONAL_V2723"
	old, existed := os.LookupEnv(key)
	defer func() {
		if existed {
			_ = os.Setenv(key, old)
		} else {
			_ = os.Unsetenv(key)
		}
	}()
	_ = os.Unsetenv(key)
	if automationMonthlyCapV2723("personal") != 0 || automationMonthlyCapV2723("free") != 0 {
		t.Fatal("quota without env must deny")
	}
	_ = os.Setenv(key, "-1")
	if automationMonthlyCapV2723("personal") != 0 {
		t.Fatal("negative quota allowed")
	}
	_ = os.Setenv(key, "17")
	if automationMonthlyCapV2723("personal") != 17 {
		t.Fatal("configured quota ignored")
	}
}
func TestConsumptionFlagV2723RequiresCommercial(t *testing.T) {
	k1, k2 := "WORKTIC_COMMERCIAL_ENABLE", "WORKTIC_CONSUMPTION_GUARD_V2723"
	v1, e1 := os.LookupEnv(k1)
	v2, e2 := os.LookupEnv(k2)
	defer func() {
		if e1 {
			_ = os.Setenv(k1, v1)
		} else {
			_ = os.Unsetenv(k1)
		}
		if e2 {
			_ = os.Setenv(k2, v2)
		} else {
			_ = os.Unsetenv(k2)
		}
	}()
	_ = os.Setenv(k2, "1")
	_ = os.Setenv(k1, "0")
	if consumptionV2723Enabled() {
		t.Fatal("pilot should require commercial flag")
	}
	_ = os.Setenv(k1, "1")
	if !consumptionV2723Enabled() {
		t.Fatal("both flags should enable")
	}
}
