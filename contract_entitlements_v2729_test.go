package main

import (
	"testing"
	"time"
)

func TestContractHashDeterministicV2729(t *testing.T) {
	p := contractPayloadV2729{Schema: 1, Plan: Plan{Code: "business", PriceUSDT: 75, MaxContacts: 3000}, ModulesJSON: `{"ads":true}`, Creative: commercialRule{Plan: "business", Monthly: 120}}
	a, raw, e := contractHashV2729(p)
	if e != nil || len(a) != 64 || raw == "" {
		t.Fatal(a, e)
	}
	b, _, e := contractHashV2729(p)
	if e != nil || a != b {
		t.Fatal("unstable hash", e)
	}
	p.Plan.MaxContacts = 1000
	c, _, _ := contractHashV2729(p)
	if a == c {
		t.Fatal("plan modifications must change contract version")
	}
	p.Plan.MaxContacts = 3000
	p.ModulesJSON = `{"ads":false}`
	d, _, _ := contractHashV2729(p)
	if a == d {
		t.Fatal("module change must change version")
	}
	p.ModulesJSON = `{"ads":true}`
	p.Creative.Monthly = 121
	f, _, _ := contractHashV2729(p)
	if a == f {
		t.Fatal("creative credits change must change version")
	}
}
func TestContractRenewalPolicyV2729(t *testing.T) {
	now := time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC)
	soon := now.Add(24 * time.Hour)
	past := now.Add(-time.Hour)
	cases := []struct {
		oldPlan, newPlan, oldHash, newHash string
		end                                time.Time
		blocked                            bool
	}{
		{"business", "business", "old", "old", soon, false},
		{"business", "business", "old", "new", soon, true},
		{"business", "business", "", "new", soon, true},
		{"business", "enterprise", "old", "new", soon, true},
		{"business", "business", "", "new", past, false},
		{"free", "personal", "", "new", soon, false},
		{"", "personal", "", "new", soon, false},
	}
	for _, c := range cases {
		e := contractRenewalCheckV2729(c.oldPlan, c.newPlan, c.oldHash, c.newHash, c.end, now)
		if (e != nil) != c.blocked {
			t.Errorf("case %+v: %v", c, e)
		}
	}
}
