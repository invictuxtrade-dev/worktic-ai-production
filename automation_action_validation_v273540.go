package main

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// validateAutomationActionsV273540 rejects incomplete actions BEFORE activation.
// It does not claim to prove WhatsApp consent, connection health or external webhook safety;
// those require runtime authorization and compliance checks.
func validateAutomationActionsV273540(g automationGraph) error {
	for _, n := range g.Nodes {
		c := n.Config
		name := strings.TrimSpace(n.Label)
		if name == "" {
			name = n.Type
		}
		val := func(key string) string {
			if c == nil || c[key] == nil {
				return ""
			}
			return strings.TrimSpace(fmt.Sprint(c[key]))
		}
		var err error
		switch n.Type {
		case "condition":
			if val("field") == "" {
				err = errors.New("selecciona un campo de evento")
			}
			if op := val("operator"); op != "" && op != "equals" && op != "contains" && op != "not_empty" {
				err = errors.New("operador de condición no admitido")
			}
		case "send_whatsapp":
			id, parseErr := strconv.ParseInt(val("connection_id"), 10, 64)
			if parseErr != nil || id <= 0 {
				err = errors.New("selecciona una conexión válida de WhatsApp Cloud")
			}
			if val("message") == "" {
				err = errors.New("escribe el mensaje de WhatsApp")
			}
		case "tag_contact":
			if val("tag") == "" {
				err = errors.New("escribe una etiqueta")
			}
		case "move_pipeline":
			allowed := map[string]bool{"Nuevo": true, "Contactado": true, "Interesado": true, "Calificado": true, "Cotización": true, "Negociación": true, "Ganado": true, "Perdido": true}
			if !allowed[val("stage")] {
				err = errors.New("selecciona una etapa válida del pipeline")
			}
		case "assign_owner":
			id, parseErr := strconv.ParseInt(val("owner_user_id"), 10, 64)
			if parseErr != nil || id <= 0 {
				err = errors.New("selecciona un usuario activo para asignar la oportunidad")
			}
		case "webhook":
			parsed, parseErr := url.Parse(val("url"))
			if parseErr != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
				err = errors.New("el webhook necesita una URL HTTPS válida sin credenciales")
			}
		case "create_task":
			err = validateAutomationTaskSettingsV273545(c)
		}
		if err != nil {
			return fmt.Errorf("bloque %q: %w", name, err)
		}
	}
	return nil
}
