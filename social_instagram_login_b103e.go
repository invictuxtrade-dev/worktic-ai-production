package main

import "strings"

// User-visible diagnostics contain only whitelisted stage codes, never tokens,
// OAuth codes, secrets, URLs, provider bodies or user identifiers.
func instagramB103ESafeReason(err error) string {
	if err == nil {
		return "sin error [IG_OK]"
	}
	msg := err.Error()
	cases := []struct{ code, description string }{
		{"IG_ALREADY_CONNECTED", "cuenta ya vinculada"},
		{"IG_ENCRYPTION_NOT_CONFIGURED", "cifrado no disponible"},
		{"IG_CODE_INVALID", "codigo OAuth invalido"},
		{"IG_SHORT_EXCHANGE", "intercambio de codigo rechazado por Instagram"},
		{"IG_SHORT_FORMAT", "respuesta de token inicial no reconocida"},
		{"IG_LONG_EXCHANGE", "intercambio de token largo rechazado"},
		{"IG_LONG_INVALID", "token largo no valido"},
		{"IG_PROFILE_FETCH", "Instagram no permitio consultar el perfil"},
		{"IG_PROFILE_FORMAT", "respuesta del perfil no valida"},
		{"IG_PROFILE_ID_MISMATCH", "el perfil no corresponde a la autorizacion"},
		{"IG_PROFILE_PRO_ID_MISSING", "falta ID de cuenta profesional"},
		{"IG_PROFILE_USERNAME_MISSING", "falta nombre de usuario"},
		{"IG_PROFILE_NOT_PROFESSIONAL", "Instagram no confirma cuenta profesional"},
		{"IG_DUPLICATE_CHECK", "fallo verificacion de exclusividad"},
		{"IG_SAVE_FAILED", "no fue posible guardar la conexion"},
	}
	for _, c := range cases {
		if strings.HasPrefix(msg, c.code) {
			return c.description + " [" + c.code + "]"
		}
	}
	return "error interno de autorizacion [IG_UNCLASSIFIED]"
}
