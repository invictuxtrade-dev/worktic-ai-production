package main

import "testing"

func graphV273534(nodes []automationNode, edges []automationEdge) automationGraph {
	return automationGraph{Nodes: nodes, Edges: edges}
}
func TestAutomationGraphV273534_BranchAndOrder(t *testing.T) {
	g := graphV273534([]automationNode{
		{ID: "b", Type: "tag_contact", Label: "Sí"},
		{ID: "n", Type: "webhook", Label: "No"},
		{ID: "t", Type: "trigger"},
		{ID: "c", Type: "condition", Config: map[string]any{"field": "score", "operator": "equals", "value": "80"}},
	}, []automationEdge{{From: "t", To: "c"}, {From: "c", To: "b", Branch: "yes"}, {From: "c", To: "n", Branch: "no"}})
	p, err := buildAutomationExecutionPlan(g)
	if err != nil {
		t.Fatal(err)
	}
	if p.next("t", true) != "c" || p.next("c", true) != "b" || p.next("c", false) != "n" {
		t.Fatal("branch routing failed")
	}
	dry := previewAutomationPlan(p, map[string]any{"score": "40"})
	steps := dry["steps"].([]map[string]any)
	if len(steps) != 2 || steps[1]["node_id"] != "n" || dry["executed_actions"] != 0 {
		t.Fatalf("unexpected dry-run: %#v", dry)
	}
}
func TestAutomationGraphV273534_RejectCycle(t *testing.T) {
	g := graphV273534([]automationNode{{ID: "t", Type: "trigger"}, {ID: "a", Type: "delay"}, {ID: "b", Type: "condition"}}, []automationEdge{{From: "t", To: "a"}, {From: "a", To: "b"}, {From: "b", To: "a"}})
	if _, err := buildAutomationExecutionPlan(g); err == nil {
		t.Fatal("cycle must be rejected")
	}
}
func TestAutomationGraphV273534_RejectFakeTask(t *testing.T) {
	g := graphV273534([]automationNode{{ID: "t", Type: "trigger"}, {ID: "a", Type: "create_task"}}, []automationEdge{{From: "t", To: "a"}})
	if _, err := buildAutomationExecutionPlan(g); err == nil {
		t.Fatal("fake action must be rejected")
	}
}
func TestAutomationGraphV273534_LegacyLinearCondition(t *testing.T) {
	g := graphV273534([]automationNode{{ID: "t", Type: "trigger"}, {ID: "c", Type: "condition"}, {ID: "a", Type: "tag_contact"}}, []automationEdge{{From: "t", To: "c"}, {From: "c", To: "a", Branch: "next"}})
	p, err := buildAutomationExecutionPlan(g)
	if err != nil {
		t.Fatal(err)
	}
	if p.next("c", false) != "" || p.next("c", true) != "a" {
		t.Fatal("legacy condition semantics changed")
	}
}
func TestAutomationGraphV273534_RejectDisconnected(t *testing.T) {
	g := graphV273534([]automationNode{{ID: "t", Type: "trigger"}, {ID: "a", Type: "tag_contact"}, {ID: "b", Type: "ai_task"}}, []automationEdge{{From: "t", To: "a"}})
	if _, err := buildAutomationExecutionPlan(g); err == nil {
		t.Fatal("disconnected nodes must be rejected")
	}
}
func TestAutomationGraphV273534_DurableLongWaitV273538(t *testing.T) {
	// V27.35.38 replaces the old 10-second limit with a durable wait of up to 30 days.
	// Protect against truncation by accepting 120 seconds and rejecting 30d+1s.
	g := graphV273534([]automationNode{{ID: "t", Type: "trigger"}, {ID: "a", Type: "delay", Config: map[string]any{"seconds": 120}}}, []automationEdge{{From: "t", To: "a"}})
	if _, err := buildAutomationExecutionPlan(g); err != nil {
		t.Fatalf("120-second persistent wait must be accepted: %v", err)
	}
	g.Nodes[1].Config["seconds"] = 30 * 24 * 60 * 60
	if _, err := buildAutomationExecutionPlan(g); err != nil {
		t.Fatalf("30-day persistent wait must be accepted: %v", err)
	}
	g.Nodes[1].Config["seconds"] = 30*24*60*60 + 1
	if _, err := buildAutomationExecutionPlan(g); err == nil {
		t.Fatal("wait longer than 30 days must be rejected")
	}
}

func TestAutomationGraphV273534_TemplateVariables(t *testing.T) {
	txt, err := expandAutomationMessageV273534("Hola {{name}}", map[string]any{"event": map[string]any{"name": "María"}})
	if err != nil || txt != "Hola María" {
		t.Fatalf("template: %s %v", txt, err)
	}
	if _, err := expandAutomationMessageV273534("Hola {{email}}", map[string]any{"event": map[string]any{}}); err == nil {
		t.Fatal("missing variable must fail")
	}
}
