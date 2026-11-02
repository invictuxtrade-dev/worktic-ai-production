package main

import (
	"strings"
	"testing"
)

func TestActivationV273540(t *testing.T) {
	cases := []struct {
		name string
		node automationNode
		ok   bool
	}{
		{"WhatsApp bien configurado", automationNode{Type: "send_whatsapp", Config: map[string]any{"connection_id": "3", "message": "Hola"}}, true},
		{"WhatsApp sin canal", automationNode{Type: "send_whatsapp", Config: map[string]any{"message": "Hola"}}, false},
		{"WhatsApp sin mensaje", automationNode{Type: "send_whatsapp", Config: map[string]any{"connection_id": "3"}}, false},
		{"Condición válida", automationNode{Type: "condition", Config: map[string]any{"field": "score", "operator": "equals"}}, true},
		{"Condición sin campo", automationNode{Type: "condition", Config: map[string]any{"operator": "equals"}}, false},
		{"Condición operador ilegal", automationNode{Type: "condition", Config: map[string]any{"field": "score", "operator": "multiply"}}, false},
		{"Etiqueta", automationNode{Type: "tag_contact", Config: map[string]any{"tag": "cliente"}}, true},
		{"Etiqueta vacía", automationNode{Type: "tag_contact"}, false},
		{"Pipeline", automationNode{Type: "move_pipeline", Config: map[string]any{"stage": "Contactado"}}, true},
		{"Pipeline inválido", automationNode{Type: "move_pipeline", Config: map[string]any{"stage": "Desconocido"}}, false},
		{"Responsable vacío", automationNode{Type: "assign_owner"}, false},
		{"Webhook seguro", automationNode{Type: "webhook", Config: map[string]any{"url": "https://example.org/hook"}}, true},
		{"Webhook inseguro", automationNode{Type: "webhook", Config: map[string]any{"url": "http://localhost:8080/hook"}}, false},
		{"Tarea falsa bloqueada", automationNode{Type: "create_task"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateAutomationActionsV273540(automationGraph{Nodes: []automationNode{c.node}})
			if (err == nil) != c.ok {
				t.Fatalf("esperaba ok=%v; error=%v", c.ok, err)
			}
			if !c.ok && err != nil && !strings.Contains(err.Error(), "bloque") {
				t.Fatalf("sin contexto: %v", err)
			}
		})
	}
}
