package main

import (
	"os"
	"testing"
)

func TestB9InternalFailsClosedWithoutEnvironment(t *testing.T) {
	for _, k := range []string{"WORKTIC_B9_INTERNAL_MODE", "WORKTIC_B9_INTERNAL_TENANT_ID", "WORKTIC_B9_INTERNAL_IG_ID", "WORKTIC_B9_INTERNAL_IG_TOKEN", "WORKTIC_B9_INTERNAL_CONFIRM_KEY", "WORKTIC_B9_INTERNAL_ORIGIN"} {
		old, exists := os.LookupEnv(k)
		os.Unsetenv(k)
		t.Cleanup(func() {
			if exists {
				os.Setenv(k, old)
			} else {
				os.Unsetenv(k)
			}
		})
	}
	if _, ok := b9InternalSettingsFromEnv(); ok {
		t.Fatal("internal access must be disabled")
	}
}
