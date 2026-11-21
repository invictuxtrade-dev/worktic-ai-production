package main

// YouTube Y1: opt-in channel connection separated from Gmail OAuth credentials.
// The first phase connects a channel with read-only access. Uploads and comments
// that require broader scopes are left disabled until explicit authorization.
import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"
)

const youtubeY1ReadScope = "https://www.googleapis.com/auth/youtube.readonly"
const youtubeY1StoredScopes = "youtube.readonly"
const youtubeY1StateCookie = "worktic_youtube_oauth_state"

func youtubeY1Credentials() (string, string, error) {
	id := strings.TrimSpace(os.Getenv("WORKTIC_YOUTUBE_CLIENT_ID"))
	secret := strings.TrimSpace(os.Getenv("WORKTIC_YOUTUBE_CLIENT_SECRET"))
	if id == "" || secret == "" {
		return "", "", errors.New("YouTube requiere WORKTIC_YOUTUBE_CLIENT_ID y WORKTIC_YOUTUBE_CLIENT_SECRET en Render")
	}
	return id, secret, nil
}

func youtubeY1ScopeIncludes(scopes, wanted string) bool {
	for _, s := range strings.FieldsFunc(scopes, func(r rune) bool { return r == ' ' || r == ',' || r == '\n' }) {
		if strings.EqualFold(strings.TrimSpace(s), wanted) {
			return true
		}
	}
	return false
}

// Need refresh only when near expiration. Empty/malformed expiry => refresh.
func youtubeY1NeedsRefresh(raw string, now time.Time) bool {
	exp, err := time.Parse(time.RFC3339, strings.TrimSpace(raw))
	return err != nil || !exp.After(now.Add(2*time.Minute))
}

func youtubeY1IssueStateCookie(w http.ResponseWriter, state string, baseURL string) {
	http.SetCookie(w, &http.Cookie{Name: youtubeY1StateCookie, Value: state, Path: "/api/social/oauth/callback/youtube", MaxAge: 900, Secure: strings.HasPrefix(strings.ToLower(baseURL), "https://"), HttpOnly: true, SameSite: http.SameSiteLaxMode})
}
func youtubeY1ValidStateCookie(r *http.Request, state string) bool {
	cookie, err := r.Cookie(youtubeY1StateCookie)
	return err == nil && len(cookie.Value) == len(state) && subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(state)) == 1
}
func youtubeY1ClearStateCookie(w http.ResponseWriter, baseURL string) {
	http.SetCookie(w, &http.Cookie{Name: youtubeY1StateCookie, Value: "", Path: "/api/social/oauth/callback/youtube", MaxAge: -1, Secure: strings.HasPrefix(strings.ToLower(baseURL), "https://"), HttpOnly: true, SameSite: http.SameSiteLaxMode})
}

func youtubeY1DiscoverChannel(ctx context.Context, accessToken string) (string, string, error) {
	var response struct {
		Items []struct {
			ID      string `json:"id"`
			Snippet struct {
				Title string `json:"title"`
			} `json:"snippet"`
		} `json:"items"`
	}
	err := apiJSON(ctx, "GET", "https://www.googleapis.com/youtube/v3/channels?part=snippet&mine=true", accessToken, nil, &response)
	if err != nil {
		return "", "", errors.New("YouTube no permitió consultar los canales. Verifica YouTube Data API v3 y el consentimiento OAuth")
	}
	if len(response.Items) == 0 {
		return "", "", errors.New("La cuenta de Google seleccionada no tiene un canal de YouTube disponible")
	}
	if len(response.Items) != 1 {
		return "", "", errors.New("Google devolvió varios canales; autoriza explícitamente un único canal")
	}
	id := strings.TrimSpace(response.Items[0].ID)
	title := strings.TrimSpace(response.Items[0].Snippet.Title)
	if id == "" {
		return "", "", errors.New("YouTube no devolvió un ID de canal válido")
	}
	if title == "" {
		title = "Canal de YouTube"
	}
	return id, title, nil
}
