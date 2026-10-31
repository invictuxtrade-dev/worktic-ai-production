package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"worktic-omnichannel-ai-v3/internal/workticaccountmail"
	"worktic-omnichannel-ai-v3/internal/workticactivation"
)

var workticActivationV273528 *workticactivation.Verification
var workticCaptchaV273528 *workticactivation.Captcha
var workticCaptchaRateV273528 = newWorkticLimitV273528()

func workticVerificationEnabledV273528() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("WORKTIC_EMAIL_VERIFY_ENABLED")), "true")
}

func initWorkticActivationV273528(a *App, mux *http.ServeMux) error {
	mux.HandleFunc("/api/auth/email/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", 405)
			return
		}
		workticReplyV273528(w, 200, map[string]any{"enabled": workticVerificationEnabledV273528() && workticActivationV273528 != nil})
	})
	if !workticVerificationEnabledV273528() {
		log.Print("WorkticAI V27.35.28: email verification feature OFF; existing accounts unaffected")
		return nil
	}
	if !a.db.IsPostgres() {
		return errors.New("email activation requires PostgreSQL")
	}
	if !strings.EqualFold(os.Getenv("WORKTIC_MAIL_ENABLED"), "true") {
		return errors.New("mail disabled: email activation not enabled")
	}
	sender, err := workticaccountmail.MailConfigFromEnv().Sender()
	if err != nil {
		return err
	}
	s := &workticactivation.Verification{DB: a.db.raw, Sender: sender, BaseURL: strings.TrimSpace(os.Getenv("BASE_URL"))}
	if err = s.InitSchema(context.Background()); err != nil {
		return err
	}
	captcha := &workticactivation.Captcha{DB: a.db.raw}
	workticActivationV273528 = s
	workticCaptchaV273528 = captcha

	mux.HandleFunc("/api/auth/captcha/challenge", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
			return
		}
		if !workticCaptchaRateV273528.Allow("captcha:"+workticClientKeyV273528(r), 30, 10*time.Minute) {
			w.Header().Set("Retry-After", "600")
			http.Error(w, "Inténtalo más tarde", 429)
			return
		}
		captcha.HTTPChallenge(w, r)
	})
	mux.HandleFunc("/api/auth/email/verify", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
			return
		}
		var q struct {
			Token string `json:"token"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048)).Decode(&q); err != nil {
			workticReplyV273528(w, 400, map[string]any{"error": "Solicitud inválida"})
			return
		}
		name, email, err := s.Activate(r.Context(), q.Token)
		if err != nil {
			workticReplyV273528(w, 400, map[string]any{"error": "El enlace venció o ya se utilizó. Solicita uno nuevo."})
			return
		}
		a.enqueueWelcomeMailV273527(name, email)
		workticReplyV273528(w, 200, map[string]any{"ok": true, "verified": true})
	})
	mux.HandleFunc("/api/auth/email/resend", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
			return
		}
		var q struct {
			Email string `json:"email"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048)).Decode(&q); err != nil {
			workticReplyV273528(w, 400, map[string]any{"error": "Solicitud inválida"})
			return
		}
		if !workticCaptchaRateV273528.Allow("resend:"+workticClientKeyV273528(r), 12, time.Hour) {
			w.Header().Set("Retry-After", "3600")
			workticReplyV273528(w, 429, map[string]any{"error": "Demasiadas solicitudes"})
			return
		}
		name, email, token, err := s.Resend(r.Context(), q.Email)
		if err != nil {
			log.Print("WorkticAI: error al solicitar reenvío")
			workticReplyV273528(w, 503, map[string]any{"error": "Temporalmente no disponible"})
			return
		}
		if token != "" {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err = s.SendActivation(ctx, name, email, token); err != nil {
				log.Print("WorkticAI: no se pudo enviar reenlace de activación")
			}
		}
		workticReplyV273528(w, 200, map[string]any{"ok": true, "message": "Si tu cuenta está pendiente, enviaremos un enlace."})
	})

	// A periodic worker handles reminders and account expiry. It runs once
	// immediately and then once per hour; cleanup is fail-closed and bounded.
	go func() {
		run := func() {
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()
			if _, err := s.AnonymizeExpired(ctx); err != nil {
				log.Print("WorkticAI: activation cleanup failed")
			}
			if _, err := s.RemindDue(ctx, 20); err != nil {
				log.Print("WorkticAI: activation reminder failed")
			}
			if err := s.CleanupAuxiliary(ctx); err != nil {
				log.Print("WorkticAI: token cleanup failed")
			}
		}
		run()
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			run()
		}
	}()

	log.Print("WorkticAI V27.35.28: email activation & visual challenge enabled")
	return nil
}

func workticReplyV273528(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func workticValidateRegisterV273528(w http.ResponseWriter, r *http.Request, captchaID string, captchaChoice int, honey string) bool {
	if !workticVerificationEnabledV273528() {
		return true
	}
	if strings.TrimSpace(honey) != "" {
		workticReplyV273528(w, 202, map[string]any{"ok": true, "pending_verification": true})
		return false
	}
	if !workticCaptchaRateV273528.Allow("register:"+workticClientKeyV273528(r), 12, time.Hour) {
		w.Header().Set("Retry-After", "3600")
		workticReplyV273528(w, 429, map[string]any{"error": "Demasiados intentos"})
		return false
	}
	if workticCaptchaV273528 == nil || workticActivationV273528 == nil {
		workticReplyV273528(w, 503, map[string]any{"error": "Registro temporalmente no disponible"})
		return false
	}
	if err := workticCaptchaV273528.Verify(r.Context(), captchaID, captchaChoice); err != nil {
		workticReplyV273528(w, 400, map[string]any{"error": "Desafío no válido. Resuélvelo nuevamente."})
		return false
	}
	return true
}

// The actual user and tenant are created by the existing registerHandler. This helper
// records verification in its SAME SQL transaction, before COMMIT.
func workticPreparePendingV273528(tx *Tx, id, tenantID int64, name, email string) (string, error) {
	if !workticVerificationEnabledV273528() {
		return "", nil
	}
	if workticActivationV273528 == nil {
		return "", errors.New("email activation unavailable")
	}
	return workticActivationV273528.CreatePending(context.Background(), tx.raw, id, tenantID, name, email)
}

func (a *App) workticSendActivationV273528(name, email, token string) {
	if !workticVerificationEnabledV273528() || workticActivationV273528 == nil || token == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	if err := workticActivationV273528.SendActivation(ctx, name, email, token); err != nil {
		log.Printf("WorkticAI: activation delivery failed; pending user can request resend")
	}
}

// A small per-instance backstop against bulk requests, additional to one-use
// challenges and email confirmation. TRUST_PROXY=true should only be enabled
// behind a trusted reverse proxy that normalizes X-Forwarded-For.
type workticBucketV273528 struct {
	Hits  int
	Until time.Time
}
type workticLimitV273528 struct {
	sync.Mutex
	Data map[string]workticBucketV273528
}

func newWorkticLimitV273528() *workticLimitV273528 {
	return &workticLimitV273528{Data: make(map[string]workticBucketV273528)}
}
func (l *workticLimitV273528) Allow(key string, max int, duration time.Duration) bool {
	l.Lock()
	defer l.Unlock()
	now := time.Now()
	if len(l.Data) > 50000 {
		for k, v := range l.Data {
			if now.After(v.Until) {
				delete(l.Data, k)
			}
		}
	}
	b := l.Data[key]
	if now.After(b.Until) {
		b = workticBucketV273528{Until: now.Add(duration)}
	}
	b.Hits++
	l.Data[key] = b
	return b.Hits <= max
}
func workticClientKeyV273528(r *http.Request) string {
	host := r.RemoteAddr
	if strings.EqualFold(os.Getenv("TRUST_PROXY"), "true") {
		f := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0])
		if f != "" {
			host = f
		}
	}
	return host
}
func workticNewUserActiveV273528() int {
	if workticVerificationEnabledV273528() {
		return 0
	}
	return 1
}

func (a *App) workticLoginPendingV273528(userID int64) bool {
	if !workticVerificationEnabledV273528() || workticActivationV273528 == nil {
		return false
	}
	var found int
	err := a.db.QueryRow(`SELECT 1 FROM worktic_email_activation_state WHERE user_id=? AND activated_at IS NULL AND purged_at IS NULL AND purge_after>NOW()`, userID).Scan(&found)
	return err == nil && found == 1
}
