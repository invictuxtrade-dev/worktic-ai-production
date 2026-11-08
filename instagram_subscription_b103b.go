package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// B10.3B: account webhook subscription management only. No message sending or AI.
// This uses the tenant's already-encrypted Unified Login credential; tokens and
// Graph error bodies never enter HTTP responses or logs.
func (a *App) instagramSubscriptionB103B(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	tid, user, err := a.tenantFor(r)
	if err != nil {
		http.Error(w, "Sesion requerida", http.StatusUnauthorized)
		return
	}
	if !socialPermissionsFor(user).ManageConnections {
		http.Error(w, "Permiso de administracion requerido", http.StatusForbidden)
		return
	}
	if !strings.EqualFold(strings.TrimSpace(os.Getenv("WORKTIC_IG_WEBHOOK_B103_ENABLED")), "true") {
		http.Error(w, "Webhook de Instagram desactivado", http.StatusServiceUnavailable)
		return
	}
	if r.Method == http.MethodPost {
		if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
			http.Error(w, "Content-Type invalido", http.StatusUnsupportedMediaType)
			return
		}
		origin := r.Header.Get("Origin")
		// For state changes require the browser Origin, not an optional Referer.
		if origin == "" || (origin != "https://"+r.Host && origin != "http://"+r.Host) {
			http.Error(w, "Origen invalido", http.StatusForbidden)
			return
		}
	}
	var connectionID int64
	if r.Method == http.MethodGet {
		// Keep POST only for mutations; GET cannot produce side effects.
		// Query parameters are identifiers, never credentials.
		q := r.URL.Query().Get("connection_id")
		if _, err := json.Number(q).Int64(); err != nil {
			http.Error(w, "connection_id invalido", http.StatusBadRequest)
			return
		}
		connectionID, _ = json.Number(q).Int64()
	} else {
		var q struct {
			ConnectionID int64  `json:"connection_id"`
			Action       string `json:"action"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&q); err != nil || q.Action != "subscribe" {
			http.Error(w, "accion invalida", http.StatusBadRequest)
			return
		}
		connectionID = q.ConnectionID
	}
	if connectionID <= 0 {
		http.Error(w, "connection_id invalido", http.StatusBadRequest)
		return
	}
	var encrypted, accountID string
	err = a.db.QueryRow(`SELECT encrypted_credentials,external_account_id FROM social_connections WHERE id=? AND tenant_id=? AND platform='instagram' AND provider_mode='official' AND status='connected'`, connectionID, tid).Scan(&encrypted, &accountID)
	if err != nil || accountID == "" {
		http.Error(w, "Cuenta Instagram no encontrada", http.StatusNotFound)
		return
	}
	var token socialToken
	if a.cfg.ChannelEncryptionKey == "" || json.Unmarshal([]byte(decryptLocal(encrypted, a.cfg.ChannelEncryptionKey)), &token) != nil || token.AccessToken == "" {
		http.Error(w, "Credenciales de cuenta no disponibles", http.StatusConflict)
		return
	}
	graphVersion := strings.TrimSpace(a.cfg.MetaGraphVersion)
	if !validB103BGraphVersion(graphVersion) {
		http.Error(w, "Version Graph invalida", http.StatusServiceUnavailable)
		return
	}
	endpoint := "https://graph.facebook.com/" + graphVersion + "/" + url.PathEscape(accountID) + "/subscribed_apps"
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	method := http.MethodGet
	var body io.Reader
	if r.Method == http.MethodPost {
		method = http.MethodPost
		form := url.Values{"subscribed_fields": {"messages,messaging_postbacks"}}
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		http.Error(w, "Solicitud no disponible", 503)
		return
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, "No se pudo consultar Meta", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		// Never echo untrusted Graph bodies, tokens, or account identifiers.
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "meta_http_status": resp.StatusCode, "action": method, "message": "Meta no acepto la operacion; revisar permisos y modalidad de token"})
		return
	}
	limited, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		http.Error(w, "Respuesta no disponible", 502)
		return
	}
	if method == http.MethodPost {
		var data struct {
			Success bool `json:"success"`
		}
		if json.Unmarshal(limited, &data) != nil || !data.Success {
			http.Error(w, "Meta no confirmo suscripcion", http.StatusBadGateway)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "action": "subscribed", "fields_requested": []string{"messages", "messaging_postbacks"}, "ai_enabled": false})
		return
	}
	fields, found, valid := parseB103BSubscription(limited)
	if !valid {
		http.Error(w, "Respuesta de Meta no reconocida", http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "action": "status", "subscribed": found, "fields": fields, "messages_subscribed": containsB103B(fields, "messages"), "ai_enabled": false})
}

func validB103BGraphVersion(s string) bool {
	if len(s) < 4 || len(s) > 12 || s[0] != 'v' {
		return false
	}
	dot := false
	for _, ch := range s[1:] {
		if ch == '.' && !dot {
			dot = true
			continue
		}
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return dot
}

func containsB103B(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}

func parseB103BSubscription(raw []byte) ([]string, bool, bool) {
	var out struct {
		Data []struct {
			SubscribedFields []string `json:"subscribed_fields"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &out) != nil {
		return nil, false, false
	}
	if len(out.Data) == 0 {
		return []string{}, false, true
	}
	known := map[string]bool{}
	for _, item := range out.Data {
		for _, f := range item.SubscribedFields {
			switch f {
			case "messages", "messaging_postbacks", "comments", "live_comments", "message_edit", "message_reactions", "messaging_referral":
				known[f] = true
			}
		}
	}
	result := []string{}
	for _, f := range []string{"messages", "messaging_postbacks", "comments", "live_comments", "message_edit", "message_reactions", "messaging_referral"} {
		if known[f] {
			result = append(result, f)
		}
	}
	return result, true, true
}
