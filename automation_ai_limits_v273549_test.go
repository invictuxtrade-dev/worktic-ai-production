package main

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestAutomationAIWorkflowLimitsV273549(t *testing.T) {
	cases := []struct {
		name    string
		types   []string
		max     int
		blocked bool
	}{
		{"no ai", []string{"trigger", "condition", "tag_contact"}, 5, false},
		{"one ai", []string{"trigger", "ai_task"}, 5, false},
		{"five ai", []string{"ai_task", "ai_task", "ai_task", "ai_task", "ai_task"}, 5, false},
		{"six ai", []string{"ai_task", "ai_task", "ai_task", "ai_task", "ai_task", "ai_task"}, 5, true},
		{"invalid max", []string{"trigger"}, 0, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := automationValidateAINodeTypesV273549(c.types, c.max)
			if (err != nil) != c.blocked {
				t.Fatalf("unexpected: %v", err)
			}
			if c.name == "six ai" && !strings.Contains(err.Error(), "6") {
				t.Fatal(err)
			}
		})
	}
}
func TestAutomationAIMaxEnvV273549(t *testing.T) {
	prior, exists := os.LookupEnv("WORKTIC_AUTOMATION_MAX_AI_NODES")
	defer func() {
		if exists {
			_ = os.Setenv("WORKTIC_AUTOMATION_MAX_AI_NODES", prior)
		} else {
			_ = os.Unsetenv("WORKTIC_AUTOMATION_MAX_AI_NODES")
		}
	}()
	for _, c := range []struct {
		raw  string
		want int
	}{{"", 5}, {"4", 4}, {"100", 5}, {"oops", 5}, {"0", 5}} {
		_ = os.Setenv("WORKTIC_AUTOMATION_MAX_AI_NODES", c.raw)
		if n := automationAIMaxNodesV273549(); n != c.want {
			t.Fatalf("%q: %d", c.raw, n)
		}
	}
}
func TestAutomationAIMonthBoundsV273549(t *testing.T) {
	a, b := automationAIMonthBoundsV273549(time.Date(2026, 12, 31, 23, 40, 0, 0, time.FixedZone("LOCAL", -5*3600)))
	if a.Format("2006-01-02") != "2027-01-01" || b.Format("2006-01-02") != "2027-02-01" {
		t.Fatalf("expected UTC month boundaries: %v %v", a, b)
	}
}
