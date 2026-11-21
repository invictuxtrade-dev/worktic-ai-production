package main

// LinkedIn L1: least-privilege OpenID Connect profile connection.
// A profile-only authorization never implies permission to publish.
import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
)

const linkedinL1Scope = "openid profile"
const linkedinL1StoredScopes = "openid,profile"
const linkedinL1StateCookie = "worktic_linkedin_oauth_state"

func linkedinL1CredentialsReady(id, secret, encKey string) error {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(secret) == "" {
		return errors.New("LinkedIn requiere LINKEDIN_CLIENT_ID y LINKEDIN_CLIENT_SECRET en Render")
	}
	if len(strings.TrimSpace(encKey)) < 32 {
		return errors.New("LinkedIn requiere CHANNEL_ENCRYPTION_KEY segura de al menos 32 caracteres")
	}
	return nil
}

func linkedinL1HasScope(scopes, requested string) bool {
	for _, s := range strings.FieldsFunc(scopes, func(r rune) bool { return r == ' ' || r == ',' || r == '\n' }) {
		if strings.EqualFold(strings.TrimSpace(s), requested) {
			return true
		}
	}
	return false
}

func linkedinL1ValidateTokenScopes(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return nil
	} // LinkedIn can omit the scope field; userinfo verifies the grant.
	if !linkedinL1HasScope(raw, "openid") || !linkedinL1HasScope(raw, "profile") {
		return errors.New("LinkedIn no concedió el acceso al perfil solicitado; revisa Sign In with LinkedIn using OpenID Connect")
	}
	return nil
}

func linkedinL1ProfileName(given, family, name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = strings.TrimSpace(strings.TrimSpace(given) + " " + strings.TrimSpace(family))
	}
	if name == "" {
		name = "Perfil de LinkedIn"
	}
	return name
}

func linkedinL1ValidateIdentity(id string) error {
	if strings.TrimSpace(id) == "" || len(id) > 1024 {
		return errors.New("LinkedIn no devolvió un identificador de perfil válido")
	}
	return nil
}

func linkedinL1DiscoverProfile(ctx context.Context, token string) (string, string, error) {
	var me struct {
		Sub        string `json:"sub"`
		Name       string `json:"name"`
		GivenName  string `json:"given_name"`
		FamilyName string `json:"family_name"`
	}
	if strings.TrimSpace(token) == "" {
		return "", "", errors.New("LinkedIn: token de acceso ausente")
	}
	if err := apiJSON(ctx, http.MethodGet, "https://api.linkedin.com/v2/userinfo", token, nil, &me); err != nil {
		return "", "", errors.New("LinkedIn no permitió consultar el perfil; verifica OAuth y los productos de tu app")
	}
	if err := linkedinL1ValidateIdentity(me.Sub); err != nil {
		return "", "", err
	}
	return strings.TrimSpace(me.Sub), linkedinL1ProfileName(me.GivenName, me.FamilyName, me.Name), nil
}

func linkedinL1IssueStateCookie(w http.ResponseWriter, state, base string) {
	http.SetCookie(w, &http.Cookie{Name: linkedinL1StateCookie, Value: state, Path: "/api/social/oauth/callback/linkedin", MaxAge: 900, Secure: strings.HasPrefix(strings.ToLower(base), "https://"), HttpOnly: true, SameSite: http.SameSiteLaxMode})
}
func linkedinL1ValidStateCookie(r *http.Request, state string) bool {
	c, err := r.Cookie(linkedinL1StateCookie)
	return err == nil && len(c.Value) == len(state) && subtle.ConstantTimeCompare([]byte(c.Value), []byte(state)) == 1
}
func linkedinL1ClearStateCookie(w http.ResponseWriter, base string) {
	http.SetCookie(w, &http.Cookie{Name: linkedinL1StateCookie, Path: "/api/social/oauth/callback/linkedin", MaxAge: -1, Secure: strings.HasPrefix(strings.ToLower(base), "https://"), HttpOnly: true, SameSite: http.SameSiteLaxMode})
}
