package main

import "testing"

func TestCoreCapsV2721(t *testing.T) {
	for _, x := range []struct {
		code      string
		old, want int64
	}{
		{"free", 50, 0}, {"personal", 2000, 150}, {"business", 10000, 500}, {"enterprise", 30000, 1500}, {"personal", 70, 70}, {"unknown", 150, 0},
	} {
		got := commercialCoreCapForPlan(x.code, x.old)
		if got != x.want {
			t.Errorf("%s got %d want %d", x.code, got, x.want)
		}
	}
}
func TestCommercialCostsDontMix(t *testing.T) {
	if commercialRuleAllows(commercialRule{Text: true}, "core_ai") {
		t.Fatal("core response cannot draw from creative wallet")
	}
	if !commercialRuleAllows(commercialRule{Text: true}, "copy") {
		t.Fatal("studio copy entitlement missing")
	}
}
