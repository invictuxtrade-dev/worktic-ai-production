package main

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// automationExecutionPlan is built before any workflow has side effects.
// Edges, not serialized node order, determine the next action.
type automationExecutionPlan struct {
	start string
	nodes map[string]automationNode
	edges map[string]map[string]string
}

func automationEdgeBranch(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "next":
		return "next", nil
	case "yes", "true", "si", "sí":
		return "yes", nil
	case "no", "false":
		return "no", nil
	default:
		return "", fmt.Errorf("rama no admitida: %q", raw)
	}
}

func buildAutomationExecutionPlan(g automationGraph) (*automationExecutionPlan, error) {
	if len(g.Nodes) < 2 {
		return nil, errors.New("el flujo necesita un disparador y al menos una acción")
	}
	if len(g.Nodes) > 50 {
		return nil, errors.New("el flujo excede el máximo de 50 nodos")
	}
	allowed := map[string]bool{
		"trigger": true, "condition": true, "delay": true, "send_whatsapp": true,
		"ai_task": true, "tag_contact": true, "move_pipeline": true, "assign_owner": true,
		"create_task": true, "webhook": true,
	}
	p := &automationExecutionPlan{nodes: make(map[string]automationNode), edges: make(map[string]map[string]string)}
	for _, node := range g.Nodes {
		node.ID = strings.TrimSpace(node.ID)
		if node.ID == "" || len(node.ID) > 128 {
			return nil, errors.New("identificador de nodo vacío o demasiado largo")
		}
		if !allowed[node.Type] {
			return nil, fmt.Errorf("tipo de nodo no admitido: %s", node.Type)
		}
		if _, exists := p.nodes[node.ID]; exists {
			return nil, fmt.Errorf("nodo duplicado: %s", node.ID)
		}
		if node.Type == "trigger" {
			if p.start != "" {
				return nil, errors.New("solo puede existir un disparador")
			}
			p.start = node.ID
		}
		if node.Type == "delay" {
			if _, err := automationDelaySecondsV273538(node.Config); err != nil {
				return nil, err
			}
		}
		p.nodes[node.ID] = node
	}
	if p.start == "" {
		return nil, errors.New("falta el nodo disparador")
	}
	for _, e := range g.Edges {
		from, fromOK := p.nodes[e.From]
		_, toOK := p.nodes[e.To]
		if !fromOK || !toOK || e.From == e.To {
			return nil, fmt.Errorf("conexión inválida: %s → %s", e.From, e.To)
		}
		if e.To == p.start {
			return nil, errors.New("el disparador no puede recibir conexiones")
		}
		branch, err := automationEdgeBranch(e.Branch)
		if err != nil {
			return nil, err
		}
		if from.Type != "condition" && branch != "next" {
			return nil, fmt.Errorf("solo las condiciones admiten ramas Sí/No: %s", e.From)
		}
		if p.edges[e.From] == nil {
			p.edges[e.From] = make(map[string]string)
		}
		if p.edges[e.From][branch] != "" {
			return nil, fmt.Errorf("rama duplicada %s en nodo %s", branch, e.From)
		}
		p.edges[e.From][branch] = e.To
	}
	for from, branches := range p.edges {
		if p.nodes[from].Type == "condition" && branches["next"] != "" && (branches["yes"] != "" || branches["no"] != "") {
			return nil, fmt.Errorf("la condición %s mezcla salida lineal con ramas Sí/No", from)
		}
	}
	// Check for cycles and unreachable nodes before performing real actions.
	visited := make(map[string]bool)
	visiting := make(map[string]bool)
	var walk func(string) error
	walk = func(id string) error {
		if visiting[id] {
			return fmt.Errorf("el flujo contiene un ciclo en %s", id)
		}
		if visited[id] {
			return nil
		}
		visiting[id] = true
		for _, to := range p.edges[id] {
			if err := walk(to); err != nil {
				return err
			}
		}
		visiting[id] = false
		visited[id] = true
		return nil
	}
	if err := walk(p.start); err != nil {
		return nil, err
	}
	if len(visited) != len(p.nodes) {
		return nil, errors.New("hay nodos desconectados del disparador; conecta o elimina esos nodos")
	}
	return p, nil
}

func (p *automationExecutionPlan) next(id string, matched bool) string {
	n := p.nodes[id]
	if n.Type != "condition" {
		return p.edges[id]["next"]
	}
	if matched {
		if yes := p.edges[id]["yes"]; yes != "" {
			return yes
		}
		return p.edges[id]["next"] // backwards-compatible linear flow
	}
	return p.edges[id]["no"] // no false branch: stop safely
}

// Preview routes through conditions but NEVER runs a business action, writes to
// the database, calls AI, sends WhatsApp, or invokes a webhook.
func previewAutomationPlan(plan *automationExecutionPlan, payload map[string]any) map[string]any {
	if payload == nil {
		payload = map[string]any{}
	}
	steps := make([]map[string]any, 0, len(plan.nodes))
	cursor := plan.next(plan.start, true)
	state := map[string]any{"event": payload}
	for cursor != "" {
		n := plan.nodes[cursor]
		step := map[string]any{"node_id": n.ID, "type": n.Type, "label": n.Label, "result": "simulated_only"}
		matched := true
		if n.Type == "condition" {
			field, op, want := cfgString(n.Config, "field"), cfgString(n.Config, "operator"), cfgString(n.Config, "value")
			got := payloadString(state, field)
			switch op {
			case "equals":
				matched = strings.EqualFold(got, want)
			case "contains":
				matched = strings.Contains(strings.ToLower(got), strings.ToLower(want))
			case "not_empty":
				matched = got != ""
			default:
				matched = got == want
			}
			step["result"] = "condition_evaluated"
			step["matched"] = matched
		}
		steps = append(steps, step)
		cursor = plan.next(n.ID, matched)
	}
	return map[string]any{"dry_run": true, "status": "simulated", "steps": steps, "executed_actions": 0}
}

var automationTemplateTokenV273534 = regexp.MustCompile(`\{\{\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*\}\}`)

func expandAutomationMessageV273534(message string, state map[string]any) (string, error) {
	ev, _ := state["event"].(map[string]any)
	var missing []string
	result := automationTemplateTokenV273534.ReplaceAllStringFunc(message, func(tag string) string {
		match := automationTemplateTokenV273534.FindStringSubmatch(tag)
		key := match[1]
		if ev == nil || ev[key] == nil || strings.TrimSpace(fmt.Sprint(ev[key])) == "" {
			missing = append(missing, key)
			return tag
		}
		return strings.TrimSpace(fmt.Sprint(ev[key]))
	})
	if len(missing) != 0 {
		return "", fmt.Errorf("faltan variables para el mensaje: %s", strings.Join(missing, ", "))
	}
	if strings.Contains(result, "{{") || strings.Contains(result, "}}") {
		return "", errors.New("formato de variables del mensaje no admitido")
	}
	if len(result) > 4096 {
		return "", errors.New("mensaje demasiado largo")
	}
	return result, nil
}
