package main

import (
	"testing"
)

func TestAutomationDurableWaitValidationV273538(t *testing.T) {
	cases := []struct {
		input any
		want  int64
		valid bool
	}{
		{"0", 0, true}, {"10", 10, true}, {"11", 11, true}, {"3600", 3600, true}, {"86400", 86400, true}, {"2592000", 2592000, true},
		{"-1", 0, false}, {"2592001", 0, false}, {"1.5", 0, false}, {"abc", 0, false}, {1.2, 0, false},
	}
	for _, c := range cases {
		got, err := automationDelaySecondsV273538(map[string]any{"seconds": c.input})
		if (err == nil) != c.valid || (c.valid && got != c.want) {
			t.Errorf("wait %v: got=%d err=%v want=%d valid=%v", c.input, got, err, c.want, c.valid)
		}
	}
	if _, err := automationDelaySecondsV273538(map[string]any{}); err == nil {
		t.Fatal("missing wait must fail")
	}
}

func TestAutomationDurableWaitGraphV273538(t *testing.T) {
	g := automationGraph{Nodes: []automationNode{
		{ID: "start", Type: "trigger"},
		{ID: "pause", Type: "delay", Config: map[string]any{"seconds": 86400}},
		{ID: "done", Type: "tag_contact", Config: map[string]any{"tag": "followup"}},
	}, Edges: []automationEdge{{From: "start", To: "pause", Branch: "next"}, {From: "pause", To: "done", Branch: "next"}}}
	plan, err := buildAutomationExecutionPlan(g)
	if err != nil {
		t.Fatal(err)
	}
	if got := plan.next("pause", true); got != "done" {
		t.Fatalf("resume lost connection: %s", got)
	}
	g.Nodes[1].Config["seconds"] = 2592001
	if _, err = buildAutomationExecutionPlan(g); err == nil {
		t.Fatal("excessive wait must fail")
	}
}
