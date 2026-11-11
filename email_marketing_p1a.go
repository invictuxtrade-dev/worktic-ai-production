package main

// Worktic AI Email Marketing P1A: suscripción doble opt-in, campañas aprobadas,
// límite de envíos y baja RFC 8058. No usa los buzones personales para envíos masivos.
import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func emailMktEnabled() bool {
	return emailP1Enabled() && emailP1Flag("WORKTIC_EMAIL_MARKETING_P1_ENABLED")
}
func emailMktSchema(db *DB) error {
	identity := "id INTEGER PRIMARY KEY AUTOINCREMENT"
	if db.IsPostgres() {
		identity = "id BIGSERIAL PRIMARY KEY"
	}
	ddl := []string{
		`CREATE TABLE IF NOT EXISTS email_mkt_p1_forms (tenant_id BIGINT PRIMARY KEY,slug TEXT NOT NULL UNIQUE,created_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS email_mkt_p1_subscribers (` + identity + `,tenant_id BIGINT NOT NULL,email TEXT NOT NULL,status TEXT NOT NULL DEFAULT 'pending',token_hash TEXT NOT NULL DEFAULT '',expires_at TEXT NOT NULL DEFAULT '',confirmation_sent_at TEXT NOT NULL DEFAULT '',confirmed_at TEXT NOT NULL DEFAULT '',created_at TEXT NOT NULL,updated_at TEXT NOT NULL,UNIQUE(tenant_id,email))`,
		`CREATE TABLE IF NOT EXISTS email_mkt_p1_campaigns (` + identity + `,tenant_id BIGINT NOT NULL,name TEXT NOT NULL,subject TEXT NOT NULL,body TEXT NOT NULL,status TEXT NOT NULL DEFAULT 'draft',created_at TEXT NOT NULL,updated_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS email_mkt_p1_deliveries (tenant_id BIGINT NOT NULL,campaign_id BIGINT NOT NULL,subscriber_id BIGINT NOT NULL,status TEXT NOT NULL DEFAULT 'reserved',created_at TEXT NOT NULL,updated_at TEXT NOT NULL,PRIMARY KEY(tenant_id,campaign_id,subscriber_id))`,
		`CREATE TABLE IF NOT EXISTS email_mkt_p1_budget (tenant_id BIGINT NOT NULL,day_utc TEXT NOT NULL,used INTEGER NOT NULL DEFAULT 0,PRIMARY KEY(tenant_id,day_utc))`,
		`CREATE TABLE IF NOT EXISTS email_mkt_p1_ai_budget (tenant_id BIGINT NOT NULL,day_utc TEXT NOT NULL,used INTEGER NOT NULL DEFAULT 0,PRIMARY KEY(tenant_id,day_utc))`,
	}
	for _, q := range ddl {
		if _, e := db.Exec(q); e != nil {
			return e
		}
	}
	return nil
}
func (a *App) emailMktTenant(w http.ResponseWriter, r *http.Request) (int64, bool) {
	tid, ok := a.emailP1Tenant(w, r)
	if !ok {
		return 0, false
	}
	if !emailMktEnabled() {
		writeError(w, errors.New("Email Marketing no está habilitado"), 503)
		return 0, false
	}
	if e := emailMktSchema(a.db); e != nil {
		http.Error(w, "Email Marketing no disponible", 503)
		return 0, false
	}
	return tid, true
}
func emailMktSendingReady() bool {
	return emailMktEnabled() && emailP1Flag("WORKTIC_EMAIL_MARKETING_P1_SEND_ENABLED") && len(strings.TrimSpace(os.Getenv("WORKTIC_EMAIL_MARKETING_SENDGRID_API_KEY"))) > 25 && len(strings.TrimSpace(os.Getenv("WORKTIC_EMAIL_MARKETING_UNSUBSCRIBE_SECRET"))) >= 32 && strings.TrimSpace(os.Getenv("WORKTIC_EMAIL_MARKETING_FROM")) != ""
}
func emailMktTenantAllowed(tid int64) bool {
	// Explicit tenant approval by platform administrator; empty allowlist means nobody can send.
	allow := strings.Split(os.Getenv("WORKTIC_EMAIL_MARKETING_ALLOWED_TENANTS"), ",")
	for _, p := range allow {
		if strings.TrimSpace(p) == strconv.FormatInt(tid, 10) {
			return true
		}
	}
	return false
}
func emailMktUnsubToken(tid, id int64) string {
	data := fmt.Sprintf("%d.%d", tid, id)
	key := os.Getenv("WORKTIC_EMAIL_MARKETING_UNSUBSCRIBE_SECRET")
	if len(key) < 32 {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write([]byte(data))
	return base64.RawURLEncoding.EncodeToString([]byte(data)) + "." + hex.EncodeToString(mac.Sum(nil))
}
func emailMktVerifyUnsub(token string) (int64, int64, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 || len(token) > 200 {
		return 0, 0, false
	}
	raw, e := base64.RawURLEncoding.DecodeString(parts[0])
	if e != nil {
		return 0, 0, false
	}
	fields := strings.Split(string(raw), ".")
	if len(fields) != 2 {
		return 0, 0, false
	}
	tid, e := strconv.ParseInt(fields[0], 10, 64)
	if e != nil || tid <= 0 {
		return 0, 0, false
	}
	id, e := strconv.ParseInt(fields[1], 10, 64)
	if e != nil || id <= 0 {
		return 0, 0, false
	}
	expected := emailMktUnsubToken(tid, id)
	if len(expected) != len(token) || subtle.ConstantTimeCompare([]byte(expected), []byte(token)) != 1 {
		return 0, 0, false
	}
	return tid, id, true
}
func emailMktPublicBase(a *App) string { return strings.TrimSuffix(a.cfg.BaseURL, "/") }
func emailMktSendGrid(ctx context.Context, fromName, to, subject, body, unsub string) error {
	from, e := emailP1Address(os.Getenv("WORKTIC_EMAIL_MARKETING_FROM"))
	if e != nil {
		return errors.New("remitente de marketing no configurado")
	}
	to, e = emailP1Address(to)
	if e != nil {
		return e
	}
	headers := map[string]string{}
	if unsub != "" {
		headers["List-Unsubscribe"] = "<" + unsub + ">"
		headers["List-Unsubscribe-Post"] = "List-Unsubscribe=One-Click"
	}
	data := map[string]any{"personalizations": []any{map[string]any{"to": []any{map[string]string{"email": to}}, "subject": emailP1Header(subject)}}, "from": map[string]string{"email": from, "name": emailP1Clean(fromName, 80)}, "content": []any{map[string]string{"type": "text/plain", "value": body}}, "headers": headers}
	raw, _ := json.Marshal(data)
	reqCtx, cancel := context.WithTimeout(ctx, 16*time.Second)
	defer cancel()
	b, status, e := emailP1HTTP(reqCtx, "POST", "https://api.sendgrid.com/v3/mail/send", os.Getenv("WORKTIC_EMAIL_MARKETING_SENDGRID_API_KEY"), bytes.NewReader(raw), "application/json")
	_ = b
	if e != nil {
		return e
	}
	if status != 202 {
		return fmt.Errorf("proveedor no aceptó el mensaje (%d)", status)
	}
	return nil
}
func (a *App) emailMktProfile(w http.ResponseWriter, r *http.Request) {
	tid, ok := a.emailMktTenant(w, r)
	if !ok {
		return
	}
	if r.Method != "GET" {
		http.Error(w, "Método no permitido", 405)
		return
	}
	slug := ""
	e := a.db.QueryRow(`SELECT slug FROM email_mkt_p1_forms WHERE tenant_id=?`, tid).Scan(&slug)
	if e != nil {
		slug = emailP1Random()
		if slug == "" {
			http.Error(w, "error interno", 503)
			return
		}
		_, e = a.db.Exec(`INSERT INTO email_mkt_p1_forms(tenant_id,slug,created_at) VALUES(?,?,?) ON CONFLICT(tenant_id) DO NOTHING`, tid, slug, emailP1Now())
		if e != nil {
			http.Error(w, "error de formulario", 503)
			return
		}
		_ = a.db.QueryRow(`SELECT slug FROM email_mkt_p1_forms WHERE tenant_id=?`, tid).Scan(&slug)
	}
	var confirmed, pending int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM email_mkt_p1_subscribers WHERE tenant_id=? AND status='confirmed'`, tid).Scan(&confirmed)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM email_mkt_p1_subscribers WHERE tenant_id=? AND status='pending'`, tid).Scan(&pending)
	writeJSON(w, map[string]any{"ok": true, "form_url": emailMktPublicBase(a) + "/email/subscribe/" + slug, "subscribers_confirmed": confirmed, "subscribers_pending": pending, "send_enabled": emailMktSendingReady() && emailMktTenantAllowed(tid), "ai_ready": a.cfg.OpenAIAPIKey != ""})
}
func (a *App) emailMktCampaigns(w http.ResponseWriter, r *http.Request) {
	tid, ok := a.emailMktTenant(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case "GET":
		rows, e := a.db.Query(`SELECT c.id,c.name,c.subject,c.body,c.status,c.created_at,(SELECT COUNT(*) FROM email_mkt_p1_deliveries d WHERE d.tenant_id=c.tenant_id AND d.campaign_id=c.id AND d.status='accepted') FROM email_mkt_p1_campaigns c WHERE c.tenant_id=? ORDER BY c.id DESC LIMIT 100`, tid)
		if e != nil {
			http.Error(w, "error campañas", 503)
			return
		}
		defer rows.Close()
		type entry struct {
			ID       int64  `json:"id"`
			Name     string `json:"name"`
			Subject  string `json:"subject"`
			Body     string `json:"body"`
			Status   string `json:"status"`
			Created  string `json:"created"`
			Accepted int    `json:"accepted"`
		}
		out := []entry{}
		for rows.Next() {
			var x entry
			if rows.Scan(&x.ID, &x.Name, &x.Subject, &x.Body, &x.Status, &x.Created, &x.Accepted) == nil {
				out = append(out, x)
			}
		}
		writeJSON(w, map[string]any{"ok": true, "items": out})
	case "POST":
		if !a.emailP1CanAdmin(r) {
			writeError(w, errors.New("Acción reservada a administradores"), 403)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
		var q struct {
			Name    string `json:"name"`
			Subject string `json:"subject"`
			Body    string `json:"body"`
		}
		if json.NewDecoder(r.Body).Decode(&q) != nil {
			http.Error(w, "datos inválidos", 400)
			return
		}
		q.Name = emailP1Clean(q.Name, 100)
		q.Subject = emailP1Clean(q.Subject, 170)
		q.Body = emailP1Clean(q.Body, 6000)
		if q.Name == "" || q.Subject == "" || len(q.Body) < 20 {
			writeError(w, errors.New("Completa nombre, asunto y cuerpo de campaña"), 400)
			return
		}
		var count int
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM email_mkt_p1_campaigns WHERE tenant_id=?`, tid).Scan(&count)
		if count >= 50 {
			writeError(w, errors.New("Límite de campañas alcanzado"), 429)
			return
		}
		now := emailP1Now()
		_, e := a.db.Exec(`INSERT INTO email_mkt_p1_campaigns(tenant_id,name,subject,body,status,created_at,updated_at) VALUES(?,?,?,?,'draft',?,?)`, tid, q.Name, q.Subject, q.Body, now, now)
		if e != nil {
			http.Error(w, "no se pudo guardar", 503)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "status": "draft"})
	default:
		http.Error(w, "Método no permitido", 405)
	}
}
func (a *App) emailMktControl(w http.ResponseWriter, r *http.Request) {
	if !a.emailP1CanAdmin(r) {
		writeError(w, errors.New("Acción reservada a administradores"), 403)
		return
	}

	tid, ok := a.emailMktTenant(w, r)
	if !ok {
		return
	}
	if r.Method != "POST" {
		http.Error(w, "Método no permitido", 405)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 2048)
	var q struct {
		ID      int64  `json:"id"`
		Action  string `json:"action"`
		Confirm bool   `json:"confirm"`
	}
	if json.NewDecoder(r.Body).Decode(&q) != nil || q.ID < 1 {
		http.Error(w, "datos inválidos", 400)
		return
	}
	next := ""
	switch q.Action {
	case "activate":
		if !q.Confirm || !emailMktSendingReady() || !emailMktTenantAllowed(tid) {
			writeError(w, errors.New("Envío no autorizado o proveedor no configurado"), 403)
			return
		}
		next = "active"
	case "pause":
		next = "paused"
	default:
		http.Error(w, "acción inválida", 400)
		return
	}
	allowed := "draft"
	if next == "paused" {
		allowed = "active"
	} else {
		allowed = "paused"
	}
	res, e := a.db.Exec(`UPDATE email_mkt_p1_campaigns SET status=?,updated_at=? WHERE id=? AND tenant_id=? AND status IN (?,?)`, next, emailP1Now(), q.ID, tid, allowed, "draft")
	if e != nil {
		http.Error(w, "no se pudo cambiar", 503)
		return
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		http.Error(w, "campaña no encontrada o estado incompatible", 409)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "status": next})
}
func (a *App) emailMktGenerate(w http.ResponseWriter, r *http.Request) {
	if !a.emailP1CanAdmin(r) {
		writeError(w, errors.New("Acción reservada a administradores"), 403)
		return
	}

	tid, ok := a.emailMktTenant(w, r)
	if !ok {
		return
	}
	if r.Method != "POST" {
		http.Error(w, "Método no permitido", 405)
		return
	}
	if a.cfg.OpenAIAPIKey == "" {
		writeError(w, errors.New("Motor IA no configurado"), 503)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var q struct {
		Topic    string `json:"topic"`
		Audience string `json:"audience"`
	}
	if json.NewDecoder(r.Body).Decode(&q) != nil {
		http.Error(w, "datos inválidos", 400)
		return
	}
	q.Topic = emailP1Clean(q.Topic, 300)
	q.Audience = emailP1Clean(q.Audience, 150)
	if q.Topic == "" || q.Audience == "" {
		http.Error(w, "tema y audiencia requeridos", 400)
		return
	}
	day := time.Now().UTC().Format("2006-01-02")
	_, _ = a.db.Exec(`INSERT INTO email_mkt_p1_ai_budget(tenant_id,day_utc,used) VALUES(?,?,0) ON CONFLICT(tenant_id,day_utc) DO NOTHING`, tid, day)
	res, e := a.db.Exec(`UPDATE email_mkt_p1_ai_budget SET used=used+1 WHERE tenant_id=? AND day_utc=? AND used<5`, tid, day)
	n, _ := res.RowsAffected()
	if e != nil || n != 1 {
		writeError(w, errors.New("Límite diario de redacciones alcanzado"), 429)
		return
	}
	payload := map[string]any{"model": a.cfg.OpenAIModel, "max_output_tokens": 500, "input": []map[string]string{{"role": "system", "content": "Eres redactor de Worktic AI. Redacta un email comercial para destinatarios que dieron consentimiento. No inventes testimonios, urgencia, descuentos ni resultados. No incluyas variables engañosas. Responde solo JSON: subject (máx 120 caracteres) y body (texto plano, máx 1500 caracteres)."}, {"role": "user", "content": "Público: " + q.Audience + ". Tema: " + q.Topic}}}
	b, _ := json.Marshal(payload)
	ctx, cancel := context.WithTimeout(r.Context(), 22*time.Second)
	defer cancel()
	raw, status, e := emailP1HTTP(ctx, "POST", "https://api.openai.com/v1/responses", a.cfg.OpenAIAPIKey, bytes.NewReader(b), "application/json")
	if e != nil || status != 200 {
		http.Error(w, "Motor IA temporalmente no disponible", 502)
		return
	}
	var out struct {
		Output []struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
	}
	if json.Unmarshal(raw, &out) != nil {
		http.Error(w, "respuesta no válida", 502)
		return
	}
	for _, o := range out.Output {
		for _, c := range o.Content {
			var s struct {
				Subject string `json:"subject"`
				Body    string `json:"body"`
			}
			if json.Unmarshal([]byte(c.Text), &s) == nil && s.Subject != "" && s.Body != "" {
				writeJSON(w, map[string]any{"ok": true, "subject": emailP1Clean(s.Subject, 150), "body": emailP1Clean(s.Body, 1800)})
				return
			}
		}
	}
	http.Error(w, "No fue posible generar borrador", 502)
}
func (a *App) emailMktSubscriberHandler(w http.ResponseWriter, r *http.Request) {
	if !emailMktEnabled() {
		http.NotFound(w, r)
		return
	}
	if e := emailMktSchema(a.db); e != nil {
		http.Error(w, "servicio no disponible", 503)
		return
	}
	slug := strings.TrimPrefix(r.URL.Path, "/email/subscribe/")
	if slug == "" || len(slug) > 100 || strings.Contains(slug, "/") {
		http.NotFound(w, r)
		return
	}
	var tid int64
	e := a.db.QueryRow(`SELECT tenant_id FROM email_mkt_p1_forms WHERE slug=?`, slug).Scan(&tid)
	if e != nil {
		http.NotFound(w, r)
		return
	}
	switch r.Method {
	case "GET":
		var tenantName string
		_ = a.db.QueryRow(`SELECT name FROM tenants WHERE id=?`, tid).Scan(&tenantName)
		if tenantName == "" {
			tenantName = "la empresa que compartió este formulario"
		}
		emailMktHTML(w, "Suscripción Worktic AI", `<h1>Recibir novedades por email</h1><p>Empresa: <strong>`+html.EscapeString(tenantName)+`</strong></p><p>Confirma que deseas recibir comunicaciones de esta empresa. Podrás darte de baja en cualquier momento.</p><form method="POST"><label>Tu correo electrónico <input type="email" name="email" required maxlength="254"></label><label><input type="checkbox" name="consent" value="yes" required> Autorizo recibir correos de marketing</label><input name="website" style="display:none" autocomplete="off" tabindex="-1"><button>Solicitar confirmación</button></form>`)
	case "POST":
		r.Body = http.MaxBytesReader(w, r.Body, 3072)
		if r.ParseForm() != nil {
			http.Error(w, "solicitud inválida", 400)
			return
		}
		if r.FormValue("website") != "" || r.FormValue("consent") != "yes" {
			http.Error(w, "confirma tu consentimiento", 400)
			return
		}
		addr, e := emailP1Address(r.FormValue("email"))
		if e != nil {
			http.Error(w, "correo inválido", 400)
			return
		}
		var added int
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM email_mkt_p1_subscribers WHERE tenant_id=? AND created_at>?`, tid, time.Now().UTC().Add(-1*time.Hour).Format(time.RFC3339)).Scan(&added)
		if added > 30 {
			http.Error(w, "demasiadas solicitudes", 429)
			return
		}
		if !emailMktSendingReady() || !emailMktTenantAllowed(tid) {
			http.Error(w, "La suscripción no está disponible todavía", 503)
			return
		}
		var last, status string
		_ = a.db.QueryRow(`SELECT confirmation_sent_at,status FROM email_mkt_p1_subscribers WHERE tenant_id=? AND email=?`, tid, addr).Scan(&last, &status)
		if status == "confirmed" {
			emailMktHTML(w, "Solicitud recibida", "<h1>Solicitud recibida</h1><p>Si corresponde, recibirás un correo de confirmación.</p>")
			return
		}
		if t, e := time.Parse(time.RFC3339, last); e == nil && time.Since(t) < time.Hour {
			emailMktHTML(w, "Solicitud recibida", "<h1>Solicitud recibida</h1><p>Revisa tu correo de confirmación.</p>")
			return
		}
		rawToken := emailP1Random()
		now := emailP1Now()
		expires := time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339)
		_, e = a.db.Exec(`INSERT INTO email_mkt_p1_subscribers(tenant_id,email,status,token_hash,expires_at,confirmation_sent_at,created_at,updated_at) VALUES(?,?,'pending',?,?,?,?,?) ON CONFLICT(tenant_id,email) DO UPDATE SET status='pending',token_hash=excluded.token_hash,expires_at=excluded.expires_at,confirmation_sent_at=excluded.confirmation_sent_at,updated_at=excluded.updated_at`, tid, addr, emailP1Hash(rawToken), expires, now, now, now)
		if e != nil {
			http.Error(w, "no se pudo registrar la solicitud", 503)
			return
		}
		link := emailMktPublicBase(a) + "/email/confirm/" + rawToken
		msg := "Se solicitó una suscripción de novedades a través de Worktic AI.\n\nConfirma tu dirección: " + link + "\n\nSi no fuiste tú, ignora este correo."
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		e = emailMktSendGrid(ctx, "Worktic AI", addr, "Confirma tu suscripción", msg, "")
		if e != nil {
			log.Printf("[EMAIL-MARKETING-P1] no se pudo enviar confirmación")
		}
		emailMktHTML(w, "Solicitud recibida", "<h1>Solicitud recibida</h1><p>Revisa tu correo y confirma la suscripción. Si no llega, inténtalo nuevamente más tarde.</p>")
	default:
		http.Error(w, "Método no permitido", 405)
	}
}
func emailMktHTML(w http.ResponseWriter, title, body string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = io.WriteString(w, `<!doctype html><html lang="es"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>`+html.EscapeString(title)+`</title><style>body{font:16px system-ui;background:#f7f7fc;color:#171735;max-width:520px;margin:8vh auto;padding:26px}main{background:white;border-radius:20px;padding:32px;box-shadow:0 8px 32px #14143b12}button{background:#6544ec;color:white;border:0;border-radius:9px;padding:12px 20px;cursor:pointer}input[type=email]{display:block;width:100%;padding:12px;box-sizing:border-box;margin:10px 0 15px}label{display:block;margin:12px 0}</style><main><small>Worktic AI · Email Marketing</small>`+body+`</main></html>`)
}
func (a *App) emailMktConfirm(w http.ResponseWriter, r *http.Request) {
	if !emailMktEnabled() {
		http.NotFound(w, r)
		return
	}
	if e := emailMktSchema(a.db); e != nil {
		http.Error(w, "servicio no disponible", 503)
		return
	}
	token := strings.TrimPrefix(r.URL.Path, "/email/confirm/")
	if token == "" || len(token) > 200 || strings.Contains(token, "/") {
		http.NotFound(w, r)
		return
	}
	var id, tid int64
	var status string
	e := a.db.QueryRow(`SELECT id,tenant_id,status FROM email_mkt_p1_subscribers WHERE token_hash=? AND expires_at>?`, emailP1Hash(token), emailP1Now()).Scan(&id, &tid, &status)
	if e != nil {
		http.Error(w, "Enlace vencido o inválido", 400)
		return
	}
	if r.Method == "GET" {
		emailMktHTML(w, "Confirmar suscripción", `<h1>Confirmar suscripción</h1><p>Pulsa el botón para confirmar que deseas recibir novedades.</p><form method="POST"><button>Confirmar mi correo</button></form>`)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "Método no permitido", 405)
		return
	}
	_, e = a.db.Exec(`UPDATE email_mkt_p1_subscribers SET status='confirmed',confirmed_at=?,token_hash='',updated_at=? WHERE id=? AND tenant_id=? AND status='pending'`, emailP1Now(), emailP1Now(), id, tid)
	if e != nil {
		http.Error(w, "error de confirmación", 503)
		return
	}
	emailMktHTML(w, "Correo confirmado", "<h1>Suscripción confirmada</h1><p>Ya puedes recibir comunicaciones y darte de baja cuando quieras.</p>")
}
func (a *App) emailMktUnsubscribe(w http.ResponseWriter, r *http.Request) {
	if !emailMktEnabled() {
		http.NotFound(w, r)
		return
	}
	if e := emailMktSchema(a.db); e != nil {
		http.Error(w, "servicio no disponible", 503)
		return
	}
	token := strings.TrimPrefix(r.URL.Path, "/email/unsubscribe/")
	tid, id, valid := emailMktVerifyUnsub(token)
	if !valid {
		http.NotFound(w, r)
		return
	}
	if r.Method == "GET" {
		emailMktHTML(w, "Cancelar suscripción", "<h1>Cancelar suscripción</h1><p>Si deseas dejar de recibir mensajes de marketing, confirma tu baja.</p><form method=\"POST\"><button>Cancelar mi suscripción</button></form>")
		return
	}
	if r.Method != "POST" {
		http.Error(w, "Método no permitido", 405)
		return
	}
	_, e := a.db.Exec(`UPDATE email_mkt_p1_subscribers SET status='unsubscribed',updated_at=? WHERE tenant_id=? AND id=?`, emailP1Now(), tid, id)
	if e != nil {
		http.Error(w, "no se pudo registrar la baja", 503)
		return
	}
	if strings.Contains(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") && strings.Contains(r.Header.Get("User-Agent"), "Google") {
		w.WriteHeader(200)
		return
	}
	emailMktHTML(w, "Baja confirmada", "<h1>Suscripción cancelada</h1><p>No recibirás más correos de marketing de esta empresa.</p>")
}
func (a *App) runEmailMarketingP1Worker() {
	log.Printf("[EMAIL-MARKETING-P1] worker preparado (solo campañas aprobadas)")
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()
	for {
		if emailMktSendingReady() {
			if e := emailMktSchema(a.db); e == nil {
				rows, e := a.db.Query(`SELECT id,tenant_id,subject,body FROM email_mkt_p1_campaigns WHERE status='active' ORDER BY id LIMIT 60`)
				if e == nil {
					type campaign struct {
						id, tid       int64
						subject, body string
					}
					cs := []campaign{}
					for rows.Next() {
						var c campaign
						if rows.Scan(&c.id, &c.tid, &c.subject, &c.body) == nil {
							cs = append(cs, c)
						}
					}
					rows.Close()
					for _, c := range cs {
						if emailMktTenantAllowed(c.tid) {
							a.emailMktSendBatch(c.id, c.tid, c.subject, c.body)
						}
					}
				}
			}
		}
		<-ticker.C
	}
}
func (a *App) emailMktSendBatch(id, tid int64, subject, body string) {
	// Cada proceso toma máximo 5 abonados por campaña y respeta el límite diario por tenant.
	rows, e := a.db.Query(`SELECT s.id,s.email FROM email_mkt_p1_subscribers s WHERE s.tenant_id=? AND s.status='confirmed' AND NOT EXISTS (SELECT 1 FROM email_mkt_p1_deliveries d WHERE d.tenant_id=s.tenant_id AND d.subscriber_id=s.id AND d.campaign_id=?) ORDER BY s.id LIMIT 5`, tid, id)
	if e != nil {
		return
	}
	type recipient struct {
		id    int64
		email string
	}
	to := []recipient{}
	for rows.Next() {
		var x recipient
		if rows.Scan(&x.id, &x.email) == nil {
			to = append(to, x)
		}
	}
	rows.Close()
	if len(to) == 0 {
		_, _ = a.db.Exec(`UPDATE email_mkt_p1_campaigns SET status='completed',updated_at=? WHERE tenant_id=? AND id=? AND status='active'`, emailP1Now(), tid, id)
		return
	}
	day := time.Now().UTC().Format("2006-01-02")
	_, _ = a.db.Exec(`INSERT INTO email_mkt_p1_budget(tenant_id,day_utc,used) VALUES(?,?,0) ON CONFLICT(tenant_id,day_utc) DO NOTHING`, tid, day)
	for _, x := range to {
		if !emailMktSendingReady() || !emailMktTenantAllowed(tid) {
			return
		}
		var st string
		if a.db.QueryRow(`SELECT status FROM email_mkt_p1_campaigns WHERE tenant_id=? AND id=?`, tid, id).Scan(&st) != nil || st != "active" {
			return
		}
		_, e := a.db.Exec(`INSERT INTO email_mkt_p1_deliveries(tenant_id,campaign_id,subscriber_id,status,created_at,updated_at) VALUES(?,?,?,'reserved',?,?) ON CONFLICT(tenant_id,campaign_id,subscriber_id) DO NOTHING`, tid, id, x.id, emailP1Now(), emailP1Now())
		if e != nil {
			continue
		}
		res, e := a.db.Exec(`UPDATE email_mkt_p1_deliveries SET status='sending',updated_at=? WHERE tenant_id=? AND campaign_id=? AND subscriber_id=? AND status='reserved'`, emailP1Now(), tid, id, x.id)
		n, _ := res.RowsAffected()
		if e != nil || n != 1 {
			continue
		}
		res, e = a.db.Exec(`UPDATE email_mkt_p1_budget SET used=used+1 WHERE tenant_id=? AND day_utc=? AND used<50`, tid, day)
		n, _ = res.RowsAffected()
		if e != nil || n != 1 {
			_, _ = a.db.Exec(`DELETE FROM email_mkt_p1_deliveries WHERE tenant_id=? AND campaign_id=? AND subscriber_id=? AND status='sending'`, tid, id, x.id)
			return
		}
		// Verificar la baja otra vez antes de cada envío.
		var state string
		_ = a.db.QueryRow(`SELECT status FROM email_mkt_p1_subscribers WHERE tenant_id=? AND id=?`, tid, x.id).Scan(&state)
		if state != "confirmed" {
			_, _ = a.db.Exec(`UPDATE email_mkt_p1_deliveries SET status='suppressed' WHERE tenant_id=? AND campaign_id=? AND subscriber_id=?`, tid, id, x.id)
			continue
		}
		unsub := emailMktPublicBase(a) + "/email/unsubscribe/" + emailMktUnsubToken(tid, x.id)
		content := body + "\n\n---\nPara cancelar tu suscripción: " + unsub
		ctx, cancel := context.WithTimeout(context.Background(), 18*time.Second)
		e = emailMktSendGrid(ctx, "Worktic AI", x.email, subject, content, unsub)
		cancel()
		status := "accepted"
		if e != nil {
			status = "unknown"
			log.Printf("[EMAIL-MARKETING-P1] estado de entrega incierto para campaña %d tenant %d", id, tid)
		}
		_, _ = a.db.Exec(`UPDATE email_mkt_p1_deliveries SET status=?,updated_at=? WHERE tenant_id=? AND campaign_id=? AND subscriber_id=?`, status, emailP1Now(), tid, id, x.id)
	}
}
