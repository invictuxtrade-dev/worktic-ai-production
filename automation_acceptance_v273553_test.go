package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// All data is synthetic. These tests call pure routing/validation functions.
// They must never send a message, request AI, call webhooks or write to CRM.
func acceptanceGraphV273553() automationGraph {
	return automationGraph{
		Nodes: []automationNode{
			{ID: "t", Type: "trigger", Label: "Inicio"},
			{ID: "c", Type: "condition", Label: "Interesado", Config: map[string]any{"field": "interest", "operator": "equals", "value": "comprar"}},
			{ID: "yes", Type: "tag_contact", Label: "Cliente interesado", Config: map[string]any{"tag": "interesado"}},
			{ID: "no", Type: "tag_contact", Label: "Sin interés", Config: map[string]any{"tag": "sin_interes"}},
		},
		Edges: []automationEdge{
			{From: "t", To: "c", Branch: "next"},
			{From: "c", To: "yes", Branch: "yes"},
			{From: "c", To: "no", Branch: "no"},
		},
	}
}

func acceptanceTraceV273553(t *testing.T, result map[string]any, want []string) {
	t.Helper()
	if result["dry_run"] != true || result["executed_actions"] != 0 {
		t.Fatalf("simulation must have no side effects: %#v", result)
	}
	steps, ok := result["steps"].([]map[string]any)
	if !ok || len(steps) != len(want) {
		t.Fatalf("unexpected preview trace: %#v", result["steps"])
	}
	for i, id := range want {
		if steps[i]["node_id"] != id {
			t.Fatalf("step %d: got %v, want %q", i, steps[i]["node_id"], id)
		}
	}
}

func TestAcceptanceV273553_TrueFalseRoutingAndDryRun(t *testing.T) {
	plan, err := buildAutomationExecutionPlan(acceptanceGraphV273553())
	if err != nil {
		t.Fatal(err)
	}
	yes := previewAutomationPlan(plan, map[string]any{"interest": "comprar"})
	acceptanceTraceV273553(t, yes, []string{"c", "yes"})
	no := previewAutomationPlan(plan, map[string]any{"interest": "consultar"})
	acceptanceTraceV273553(t, no, []string{"c", "no"})
}

func TestAcceptanceV273553_InvalidGraphsRejected(t *testing.T) {
	g := acceptanceGraphV273553()
	// Add a back edge: graph must never execute cycles.
	g.Edges = append(g.Edges, automationEdge{From: "yes", To: "c", Branch: "next"})
	if _, err := buildAutomationExecutionPlan(g); err == nil {
		t.Fatal("cycle accepted")
	}
	g = acceptanceGraphV273553()
	g.Edges = g.Edges[:len(g.Edges)-1]
	if _, err := buildAutomationExecutionPlan(g); err == nil {
		t.Fatal("disconnected node accepted")
	}
	g = acceptanceGraphV273553()
	g.Nodes[2].ID = "c"
	if _, err := buildAutomationExecutionPlan(g); err == nil {
		t.Fatal("duplicate node accepted")
	}
}

func TestAcceptanceV273553_ChannelAndFormIsolation(t *testing.T) {
	for _, tc := range []struct {
		name    string
		filter  string
		event   string
		payload map[string]any
		want    bool
	}{
		{"Chat Web", `{"channel":"web"}`, "message.received", map[string]any{"channel": "web"}, true},
		{"Chat Web excludes Messenger", `{"channel":"web"}`, "message.received", map[string]any{"channel": "messenger"}, false},
		{"Missing channel fails closed", `{"channel":"web"}`, "message.received", map[string]any{}, false},
		{"WhatsApp Cloud", `{"channel":"whatsapp_cloud"}`, "message.received", map[string]any{"channel": "whatsapp_cloud"}, true},
		{"Social Instagram", `{"platform":"instagram"}`, "social.comment", map[string]any{"platform": "instagram"}, true},
		{"Social excludes Facebook", `{"platform":"instagram"}`, "social.comment", map[string]any{"platform": "facebook"}, false},
		{"Form 12", `{"form_id":"12"}`, "form.submitted", map[string]any{"form_id": 12}, true},
		{"Form excludes other", `{"form_id":"12"}`, "form.submitted", map[string]any{"form_id": 13}, false},
		{"Invalid filter", `{"channel":"unknown"}`, "message.received", map[string]any{"channel": "unknown"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := automationTriggerMatchesV273536([]byte(tc.filter), tc.event, tc.payload)
			if got != tc.want {
				t.Fatalf("filter=%s event=%s got=%v want=%v", tc.filter, tc.event, got, tc.want)
			}
		})
	}
}

func TestAcceptanceV273553_PersistentWaitBoundaries(t *testing.T) {
	for _, tc := range []struct {
		raw   any
		valid bool
	}{
		{"0", true}, {"10", true}, {"120", true}, {"3600", true}, {"2592000", true},
		{"2592001", false}, {"-1", false}, {"1.5", false}, {"abc", false},
	} {
		_, err := automationDelaySecondsV273538(map[string]any{"seconds": tc.raw})
		if (err == nil) != tc.valid {
			t.Fatalf("wait %v: err=%v valid=%v", tc.raw, err, tc.valid)
		}
	}
}

func TestAcceptanceV273553_MissingMessageVariablesFailClosed(t *testing.T) {
	msg, err := expandAutomationMessageV273534("Hola {{name}}", map[string]any{"event": map[string]any{"name": "Prueba"}})
	if err != nil || msg != "Hola Prueba" {
		t.Fatalf("template got %q, err=%v", msg, err)
	}
	_, err = expandAutomationMessageV273534("Hola {{name}}, tu cita {{date}}", map[string]any{"event": map[string]any{"name": "Prueba"}})
	if err == nil {
		t.Fatal("missing variable not rejected")
	}
}

func TestAcceptanceV273553_SerializationDoesNotChangeGraph(t *testing.T) {
	before := acceptanceGraphV273553()
	raw, err := json.Marshal(before)
	if err != nil {
		t.Fatal(err)
	}
	var roundtrip automationGraph
	if err = json.Unmarshal(raw, &roundtrip); err != nil {
		t.Fatal(err)
	}
	p, err := buildAutomationExecutionPlan(roundtrip)
	if err != nil {
		t.Fatal(err)
	}
	actual := previewAutomationPlan(p, map[string]any{"interest": "COMPRAR"})
	acceptanceTraceV273553(t, actual, []string{"c", "yes"})
	if strings.TrimSpace(string(raw)) == "" {
		t.Fatal("serialization unexpectedly empty")
	}
}
