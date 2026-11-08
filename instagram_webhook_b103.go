package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// B10.3: signed delivery intake only. Never sends replies or invokes an AI agent.
// Stores hashes and event metadata, not message text or sender identifiers.
func (a *App) instagramWebhookB103(w http.ResponseWriter, r *http.Request) {
	verify := strings.TrimSpace(os.Getenv("WORKTIC_IG_WEBHOOK_VERIFY_TOKEN"))
	enabled := strings.EqualFold(os.Getenv("WORKTIC_IG_WEBHOOK_B103_ENABLED"), "true")
	secret, secretSource := b103cSigningSecret(a.cfg.MetaAppSecret)
	if !enabled || verify == "" || secret == "" {
		http.Error(w, "not configured", http.StatusServiceUnavailable)
		return
	}
	if r.Method == http.MethodGet {
		if r.URL.Query().Get("hub.mode") != "subscribe" || r.URL.Query().Get("hub.verify_token") != verify || r.URL.Query().Get("hub.challenge") == "" {
			http.Error(w, "verification failed", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.WriteString(w, r.URL.Query().Get("hub.challenge"))
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "payload too large or unreadable", http.StatusRequestEntityTooLarge)
		return
	}
	if reason := b103cVerifySignature(body, r.Header.Get("X-Hub-Signature-256"), secret); reason != "" {
		b103cLogSignatureFailure(reason, secretSource)
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}
	var env struct {
		Object string `json:"object"`
		Entry  []struct {
			ID        string `json:"id"`
			Messaging []struct {
				Sender struct {
					ID string `json:"id"`
				} `json:"sender"`
				Recipient struct {
					ID string `json:"id"`
				} `json:"recipient"`
				Timestamp int64 `json:"timestamp"`
				Message   *struct {
					MID    string `json:"mid"`
					IsEcho bool   `json:"is_echo"`
				} `json:"message"`
			} `json:"messaging"`
		} `json:"entry"`
	}
	if err := json.Unmarshal(body, &env); err != nil || env.Object != "instagram" {
		http.Error(w, "invalid instagram event", http.StatusBadRequest)
		return
	}
	if _, err := a.db.Exec(`CREATE TABLE IF NOT EXISTS instagram_delivery_b103 (tenant_id BIGINT NOT NULL, social_connection_id BIGINT NOT NULL, event_hash TEXT NOT NULL, event_kind TEXT NOT NULL, received_at TEXT NOT NULL, PRIMARY KEY (tenant_id,social_connection_id,event_hash))`); err != nil {
		http.Error(w, "storage unavailable", http.StatusServiceUnavailable)
		return
	}
	// Fail closed on ambiguous account ownership. No cross-tenant fallbacks.
	for _, e := range env.Entry {
		if e.ID == "" {
			continue
		}
		var tenantID, connectionID int64
		err := a.db.QueryRow(`SELECT tenant_id,id FROM social_connections WHERE platform='instagram' AND provider_mode='official' AND status='connected' AND external_account_id=?`, e.ID).Scan(&tenantID, &connectionID)
		if err != nil {
			http.Error(w, "account is not uniquely mapped", http.StatusConflict)
			return
		}
		var count int
		if err := a.db.QueryRow(`SELECT COUNT(*) FROM social_connections WHERE platform='instagram' AND provider_mode='official' AND status='connected' AND external_account_id=?`, e.ID).Scan(&count); err != nil || count != 1 {
			http.Error(w, "account ownership ambiguous", http.StatusConflict)
			return
		}
		for _, m := range e.Messaging {
			if m.Message == nil || m.Message.IsEcho || strings.TrimSpace(m.Message.MID) == "" {
				continue
			}
			// Domain separated hash prevents raw sender or message identifiers leaking into logs or table.
			sum := sha256.Sum256([]byte("b103:instagram:" + e.ID + ":" + m.Message.MID))
			key := hex.EncodeToString(sum[:])
			if _, err := a.db.Exec(`INSERT INTO instagram_delivery_b103(tenant_id,social_connection_id,event_hash,event_kind,received_at) VALUES(?,?,?,?,?) ON CONFLICT(tenant_id,social_connection_id,event_hash) DO NOTHING`, tenantID, connectionID, key, "message", time.Now().UTC().Format(time.RFC3339)); err != nil {
				http.Error(w, "storage unavailable", http.StatusServiceUnavailable)
				return
			}
		}
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "EVENT_RECEIVED")
}
