package main

import "testing"

func TestAgentBuilderA1Clean(t *testing.T) {
	x := agentBuilderA1Clean(agentBuilderA1Proposal{Type: "MALICIOUS", Tone: "CUSTOM", Language: "ru", Tools: []string{"contact", "danger", "contact"}, Channels: []string{"telegram", "fake"}})
	if x.Type != "general" || x.Language != "es" || x.Tone != "Profesional y cercano" {
		t.Fatal(x)
	}
	if len(x.Tools) != 1 || x.Tools[0] != "contact" || len(x.Channels) != 1 || x.Channels[0] != "telegram" {
		t.Fatal(x)
	}
}

func TestAgentBuilderA1Parse(t *testing.T) {
	r, err := agentBuilderA1Parse("```json\n{\"reply\":\"Cuéntame de tu negocio\",\"agent\":{\"name\":\"Alicia\",\"type\":\"sales\",\"objective\":\"Vender\"}}\n```")
	if err != nil || r.Agent.Name != "Alicia" || r.Agent.Type != "sales" {
		t.Fatalf("%v %#v", err, r)
	}
	_, err = agentBuilderA1Parse(`{"agent":{"name":"X"}}`)
	if err == nil {
		t.Fatal("must reject missing chat reply")
	}
}
