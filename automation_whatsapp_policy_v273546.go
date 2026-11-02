package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// WorkticAI V27.35.46. Automatic free-form WhatsApp texts are *replies*, not
// outbound campaigns. The runtime gate is deliberately disabled by default.
const automationWAMaxAgeV273546 = 23 * time.Hour // safety margin before Meta's 24h limit

func automationWAProviderTimestampV273546(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, errors.New("Meta no informó la hora del mensaje entrante")
	}
	if sec, err := strconv.ParseInt(raw, 10, 64); err == nil {
		if sec <= 0 {
			return time.Time{}, errors.New("hora de Meta inválida")
		}
		return time.Unix(sec, 0).UTC(), nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, errors.New("formato de hora de Meta inválido")
	}
	return t.UTC(), nil
}

func automationWAIsOptOutV273546(text string) bool {
	switch strings.ToLower(strings.TrimSpace(text)) {
	case "stop", "baja", "cancelar", "unsubscribe", "no más", "no mas":
		return true
	}
	return false
}

// Only a verified inbound webhook event for the same connection and exact
// sender may authorize a reply. Workflow text/target settings cannot grant consent.
func automationWAReplyIdentityV273546(state map[string]any, connectionID int64, to string) (string, string, error) {
	if strings.TrimSpace(fmt.Sprint(state["event_type"])) != "message.received" || strings.TrimSpace(fmt.Sprint(state["source"])) != "inbox" {
		return "", "", errors.New("WhatsApp automático solo permite respuestas a mensajes entrantes oficiales")
	}
	ev, _ := state["event"].(map[string]any)
	if ev == nil || strings.TrimSpace(fmt.Sprint(ev["channel"])) != "whatsapp_cloud" || cfgInt64(ev, "connection_id") != connectionID || connectionID <= 0 {
		return "", "", errors.New("el evento no pertenece a la conexión WhatsApp seleccionada")
	}
	chat, _ := ev["chat"].(string)
	if !strings.HasPrefix(chat, "wacloud:") {
		return "", "", errors.New("el evento no contiene un chat WhatsApp Cloud válido")
	}
	phone := strings.TrimPrefix(chat, "wacloud:")
	normalizedTo := strings.TrimPrefix(strings.TrimSpace(to), "+")
	if len(phone) < 8 || len(phone) > 15 || normalizedTo != phone {
		return "", "", errors.New("el número de destino debe coincidir con el remitente original")
	}
	for _, ch := range phone {
		if ch < '0' || ch > '9' {
			return "", "", errors.New("teléfono WhatsApp inválido")
		}
	}
	messageID, _ := ev["message_id"].(string)
	if strings.TrimSpace(messageID) == "" {
		return "", "", errors.New("el evento no tiene un identificador de mensaje Meta verificado")
	}
	return chat, messageID, nil
}

func automationWAWithinWindowV273546(timestamp string, now time.Time) error {
	t, err := automationWAProviderTimestampV273546(timestamp)
	if err != nil {
		return err
	}
	if t.After(now.Add(5*time.Minute)) || now.Sub(t) < 0 || now.Sub(t) >= automationWAMaxAgeV273546 {
		return errors.New("la ventana segura de atención WhatsApp está cerrada; se requiere plantilla aprobada")
	}
	return nil
}
