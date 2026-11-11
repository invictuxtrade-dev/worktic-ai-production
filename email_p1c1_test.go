package main

import "testing"

func TestEmailP1C1ClassificationCases(t *testing.T) {
	cases := []struct {
		name, from, to, subject, body string
		h                             emailP1C1Headers
		category                      string
		crm                           bool
	}{
		{"sales", "cliente@example.com", "worktic@example.com", "Prueba 003", "Hola, estoy interesado en conocer los planes", emailP1C1Headers{}, "Ventas", true},
		{"support", "cliente@example.com", "worktic@example.com", "Ayuda", "Tengo un error en la aplicación", emailP1C1Headers{}, "Soporte", true},
		{"billing", "cliente@example.com", "worktic@example.com", "Mi factura", "Por favor ayudar", emailP1C1Headers{}, "Facturación", true},
		{"agenda", "cliente@example.com", "worktic@example.com", "Cita", "Quiero agendar una cita", emailP1C1Headers{}, "Agenda", true},
		{"newsletter_with_sales_words", "marketing@example.com", "worktic@example.com", "Conoce nuestros precios", "Quiero contratar nuestros planes", emailP1C1Headers{ListUnsubscribe: "<mailto:unsubscribe@example.com>"}, "Promoción", false},
		{"bulk", "comercial@example.com", "worktic@example.com", "Cotización", "Quiero contratar", emailP1C1Headers{Precedence: "bulk"}, "Notificación", false},
		{"auto_submitted", "alertas@example.com", "worktic@example.com", "Error de inicio de sesión", "Necesito soporte", emailP1C1Headers{AutoSubmitted: "auto-generated"}, "Notificación", false},
		{"noreply", "noreply@company.com", "worktic@example.com", "Consulta precios", "Estoy interesado", emailP1C1Headers{}, "Notificación", false},
		{"self", "WORKTIC@EXAMPLE.COM", "worktic@example.com", "Quiero información", "Hola", emailP1C1Headers{}, "Correo propio", false},
		{"unknown", "persona@example.com", "worktic@example.com", "Hola", "Buenas tardes", emailP1C1Headers{}, "Revisión", false},
		{"promo_subject", "promos@example.com", "worktic@example.com", "Black Friday", "Te invitamos a nuestra oferta", emailP1C1Headers{}, "Promoción", false},
		{"spam", "casino@example.com", "worktic@example.com", "Has ganado un premio", "Reclámalo", emailP1C1Headers{}, "Spam", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := emailP1C1Classify(c.from, c.to, c.subject, c.body, c.h)
			if got.Category != c.category || got.CreateCRM != c.crm {
				t.Fatalf("got %+v, want category=%q crm=%v", got, c.category, c.crm)
			}
		})
	}
}

func TestEmailP1C1ScopeFailClosed(t *testing.T) {
	t.Setenv("WORKTIC_EMAIL_P1B_ALLOWED_EMAILS", "worktic@example.com")
	t.Setenv("WORKTIC_EMAIL_P1B_ALLOWED_ACCOUNT_IDS", "15")
	t.Setenv("WORKTIC_EMAIL_P1B_FAST_SYNC_ENABLED", "true")
	t.Setenv("WORKTIC_EMAIL_P1C1_ENABLED", "false")
	if emailP1C1Enabled(15, "worktic@example.com") {
		t.Fatal("not disabled by default")
	}
	t.Setenv("WORKTIC_EMAIL_P1C1_ENABLED", "true")
	if !emailP1C1Enabled(15, "worktic@example.com") {
		t.Fatal("pilot account not allowed")
	}
	if emailP1C1Enabled(15, "other@example.com") {
		t.Fatal("other email accepted")
	}
	if emailP1C1Enabled(16, "worktic@example.com") {
		t.Fatal("other ID accepted")
	}
	t.Setenv("WORKTIC_EMAIL_P1B_ALLOWED_ACCOUNT_IDS", "")
	if emailP1C1Enabled(15, "worktic@example.com") {
		t.Fatal("empty allowlist accepted")
	}
}
