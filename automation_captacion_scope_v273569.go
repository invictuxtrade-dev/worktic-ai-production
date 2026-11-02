package main

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
)

// V27.35.69: only the Captacion IA commercial template requires an explicit
// form selection. Generic workflows intentionally keep the "all forms" option.
// The existing workflow HTTP handler ALSO checks that the selected form is
// active and belongs to the authenticated tenant before activation.
func automationValidateCaptacionScopeV273569(rawGraph, rawConfig []byte) error {
	var graph struct {
		Nodes []struct {
			ID string `json:"id"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal(rawGraph, &graph); err != nil {
		return errors.New("grafo de captación inválido")
	}
	found := map[string]bool{}
	for _, node := range graph.Nodes {
		found[node.ID] = true
	}
	// Do not restrict other user-authored and legacy workflows.
	if !(found["wt566_inicio"] && found["wt566_ai"] && found["wt566_prioridad"] && found["wt566_tarea"]) {
		return nil
	}
	var cfg map[string]string
	if err := json.Unmarshal(rawConfig, &cfg); err != nil {
		return errors.New("configuración de formulario inválida")
	}
	selected := strings.TrimSpace(cfg["form_id"])
	if selected == "" {
		return errors.New("Captación IA: selecciona un formulario antes de activar")
	}
	id, err := strconv.ParseInt(selected, 10, 64)
	if err != nil || id <= 0 || strconv.FormatInt(id, 10) != selected {
		return errors.New("Captación IA: identificador de formulario inválido")
	}
	return nil
}
