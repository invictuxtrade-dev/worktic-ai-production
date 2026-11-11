package main

// Email IA P1C.1 — clasificación conservadora, local y de solo lectura.
// Nunca envía correos, crea oportunidades o llama a modelos generativos.
import (
	"strings"
)

type emailP1C1Headers struct {
	ListUnsubscribe string
	AutoSubmitted   string
	Precedence      string
}

type emailP1C1Decision struct {
	Category  string
	Priority  string
	Summary   string
	CreateCRM bool
}

// El piloto se limita a las mismas cuentas explícitamente autorizadas en P1B.
// Sin bandera + lista de correo + ID de cuenta, se mantiene el comportamiento P1B.2.
func emailP1C1Enabled(accountID int64, accountEmail string) bool {
	return emailP1Flag("WORKTIC_EMAIL_P1C1_ENABLED") &&
		emailP1Flag("WORKTIC_EMAIL_P1B_FAST_SYNC_ENABLED") &&
		emailP1BAllowedAccount(accountID, accountEmail)
}

func emailP1C1Contains(s string, fragments ...string) bool {
	for _, f := range fragments {
		if strings.Contains(s, f) {
			return true
		}
	}
	return false
}

// Política fail-safe: una intención incierta NO crea contacto CRM automáticamente.
// El encabezado List-Unsubscribe tiene prioridad sobre palabras de venta, porque
// los boletines suelen decir "precios", "planes" o "contratar" en el cuerpo.
func emailP1C1Classify(sender, account, subject, body string, headers emailP1C1Headers) emailP1C1Decision {
	s := strings.ToLower(strings.TrimSpace(subject))
	b := strings.ToLower(strings.TrimSpace(body))
	combined := s + " " + b
	from := strings.ToLower(strings.TrimSpace(sender))
	me := strings.ToLower(strings.TrimSpace(account))
	d := emailP1C1Decision{Category: "Revisión", Priority: "normal", Summary: emailP1Clean(subject, 240)}
	if from == "" || me == "" {
		return d
	}
	if from == me {
		d.Category = "Correo propio"
		return d
	}

	local := strings.SplitN(from, "@", 2)[0]
	auto := strings.ToLower(strings.TrimSpace(headers.AutoSubmitted))
	precedence := strings.ToLower(strings.TrimSpace(headers.Precedence))
	if emailP1C1Contains(local, "no-reply", "noreply", "do-not-reply", "donotreply", "mailer-daemon", "postmaster", "notifications", "notification", "newsletter", "notificaciones") ||
		(auto != "" && auto != "no") ||
		emailP1C1Contains(precedence, "bulk", "list", "junk") {
		d.Category = "Notificación"
		return d
	}
	if strings.TrimSpace(headers.ListUnsubscribe) != "" ||
		emailP1C1Contains(s, "newsletter", "boletín", "boletin", "suscripción", "suscripcion", "oferta especial", "descuento exclusivo", "black friday", "cyber monday", "cupón promocional", "cupon promocional") ||
		emailP1C1Contains(combined, "para cancelar la suscripción", "para cancelar la suscripcion", "unsubscribe from this list", "manage your email preferences", "si no deseas recibir más correos", "si no deseas recibir mas correos") {
		d.Category = "Promoción"
		return d
	}
	if emailP1C1Contains(s, "has ganado un premio", "ganaste un premio", "claim your prize", "apuesta ganadora", "dinero fácil", "dinero facil") {
		d.Category = "Spam"
		return d
	}
	if emailP1C1Contains(combined, "tengo un error", "no puedo ingresar", "no puedo acceder", "no funciona", "necesito soporte", "necesito ayuda", "me falla", "problema con", "error en la", "support ticket") {
		d.Category = "Soporte"
		d.CreateCRM = true
	} else if emailP1C1Contains(combined, "solicito factura", "mi factura", "factura pendiente", "comprobante de pago", "mi pago", "cobro duplicado", "facturación", "facturacion") {
		d.Category = "Facturación"
		d.CreateCRM = true
	} else if emailP1C1Contains(combined, "agendar una cita", "agendar reunión", "agendar reunion", "reservar una cita", "quiero agendar", "programar una llamada", "concertar reunión", "concertar reunion") {
		d.Category = "Agenda"
		d.CreateCRM = true
	} else if emailP1C1Contains(combined, "cotización", "cotizacion", "solicitar precio", "solicito precio", "quiero conocer sus planes", "conocer los planes", "quiero información", "quiero informacion", "estoy interesado", "estoy interesada", "me interesa contratar", "quiero contratar", "me gustaría comprar", "me gustaria comprar", "solicito una propuesta", "quiero una demo", "quiero saber sus precios", "cuánto cuesta", "cuanto cuesta") {
		d.Category = "Ventas"
		d.CreateCRM = true
	}
	if d.CreateCRM && emailP1C1Contains(combined, "urgente", "inmediato", "reclamación", "reclamacion") {
		d.Priority = "alta"
	}
	return d
}
