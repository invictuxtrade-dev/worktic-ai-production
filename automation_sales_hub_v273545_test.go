package main

import (
	"strings"
	"testing"
)

func TestAutomationTaskConfigV273545(t *testing.T) {
	for _, tt := range []struct {
		name    string
		config  map[string]any
		success bool
	}{
		{"tarea válida", map[string]any{"title": "Contactar al cliente", "due_hours": 24}, true},
		{"tarea con usuario", map[string]any{"title": "Enviar propuesta", "due_hours": "48", "owner_user_id": "12"}, true},
		{"sin título", map[string]any{"due_hours": 24}, false},
		{"vencimiento omitido", map[string]any{"title": "Llamar al cliente"}, false},
		{"cero horas", map[string]any{"title": "Llamar al cliente", "due_hours": 0}, false},
		{"más de 30 días", map[string]any{"title": "Llamar al cliente", "due_hours": 721}, false},
		{"horas fraccionarias", map[string]any{"title": "Llamar al cliente", "due_hours": "3.5"}, false},
		{"usuario no numérico", map[string]any{"title": "Llamar al cliente", "due_hours": 24, "owner_user_id": "otro"}, false},
		{"usuario cero", map[string]any{"title": "Llamar al cliente", "due_hours": 24, "owner_user_id": "0"}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := validateAutomationTaskSettingsV273545(tt.config)
			if (got == nil) != tt.success {
				t.Fatalf("success=%v, error=%v", tt.success, got)
			}
		})
	}
}
func TestOwnerIDV273545(t *testing.T) {
	if _, err := automationOptionalOwnerIDV273545(map[string]any{}, true); err == nil {
		t.Fatal("assign_owner debe exigir usuario")
	}
	if id, err := automationOptionalOwnerIDV273545(map[string]any{"owner_user_id": "31"}, true); err != nil || id != 31 {
		t.Fatalf("id=%d err=%v", id, err)
	}
	if id, err := automationOptionalOwnerIDV273545(map[string]any{}, false); err != nil || id != 0 {
		t.Fatalf("id=%d err=%v", id, err)
	}
}
func TestSalesHubActionValidationV273545(t *testing.T) {
	for _, tt := range []struct {
		node  automationNode
		valid bool
	}{
		{automationNode{Type: "create_task", Label: "Seguimiento", Config: map[string]any{"title": "Llamar mañana", "due_hours": 24}}, true},
		{automationNode{Type: "assign_owner", Label: "Responsable", Config: map[string]any{"owner_user_id": "8"}}, true},
		{automationNode{Type: "assign_owner", Label: "Responsable", Config: map[string]any{"owner": "usuario texto"}}, false},
		{automationNode{Type: "create_task", Label: "Seguimiento", Config: map[string]any{"title": "Corto", "due_hours": 800}}, false},
	} {
		err := validateAutomationActionsV273540(automationGraph{Nodes: []automationNode{tt.node}})
		if (err == nil) != tt.valid {
			t.Fatalf("node %v expected %v err %v", tt.node.Type, tt.valid, err)
		}
		if err != nil && !strings.Contains(err.Error(), "bloque") {
			t.Fatalf("error sin contexto: %v", err)
		}
	}
}

func TestSalesHubExecutionIDAfterWaitV273545(t *testing.T) {
	for _, caseValue := range []any{int64(1800000), float64(1800000), "1800000"} {
		id, err := automationSalesHubEffectIDV273545(map[string]any{"_automation_execution_id": caseValue})
		if err != nil || id != 1800000 {
			t.Fatalf("valor=%v id=%d err=%v", caseValue, id, err)
		}
	}
	for _, bad := range []any{nil, 0.5, float64(9007199254740992), "invalid"} {
		if _, err := automationSalesHubEffectIDV273545(map[string]any{"_automation_execution_id": bad}); err == nil {
			t.Fatalf("debió rechazar: %v", bad)
		}
	}
}
