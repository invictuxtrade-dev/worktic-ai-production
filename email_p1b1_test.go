package main

import "testing"

func TestEmailP1B1PilotFailClosed(t *testing.T) {
	t.Setenv("WORKTIC_EMAIL_P1B_ALLOWED_EMAILS", "")
	t.Setenv("WORKTIC_EMAIL_P1B_ALLOWED_ACCOUNT_IDS", "1")
	if emailP1BAllowedAccount(1, "gerenciaworktic@gmail.com") {
		t.Fatal("piloto abierto sin lista de correos")
	}
	t.Setenv("WORKTIC_EMAIL_P1B_ALLOWED_EMAILS", "gerenciaworktic@gmail.com")
	t.Setenv("WORKTIC_EMAIL_P1B_ALLOWED_ACCOUNT_IDS", "")
	if emailP1BAllowedAccount(1, "gerenciaworktic@gmail.com") {
		t.Fatal("piloto abierto sin lista de IDs")
	}
}
func TestEmailP1B1PilotAllowlist(t *testing.T) {
	t.Setenv("WORKTIC_EMAIL_P1B_ALLOWED_EMAILS", " GerenciaWorktic@Gmail.com , otra@example.com ")
	t.Setenv("WORKTIC_EMAIL_P1B_ALLOWED_ACCOUNT_IDS", "1, 4")
	if !emailP1BAllowedAccount(1, "gerenciaworktic@gmail.com") {
		t.Fatal("cuenta autorizada bloqueada")
	}
	if !emailP1BAllowedAccount(4, "otra@example.com") {
		t.Fatal("segunda cuenta autorizada bloqueada")
	}
	if emailP1BAllowedAccount(2, "gerenciaworktic@gmail.com") {
		t.Fatal("ID de cuenta ajeno autorizado")
	}
	if emailP1BAllowedAccount(1, "otro@gmail.com") {
		t.Fatal("correo ajeno autorizado")
	}
}
func TestEmailP1B1DisabledByDefault(t *testing.T) {
	t.Setenv("WORKTIC_EMAIL_P1B_FAST_SYNC_ENABLED", "false")
	if emailP1Flag("WORKTIC_EMAIL_P1B_FAST_SYNC_ENABLED") {
		t.Fatal("piloto activado sin consentimiento")
	}
}
