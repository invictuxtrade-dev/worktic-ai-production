package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type productionRuntime struct {
	redis *redis.Client
	mu    sync.Mutex
	mem   map[string]*rateWindowV26
}

type rateWindowV26 struct {
	Count int
	Reset time.Time
}

type securityAuditV26 struct {
	Event string         `json:"event"`
	IP    string         `json:"ip"`
	Meta  map[string]any `json:"meta,omitempty"`
}

func validateProductionConfig(cfg Config) error {
	if cfg.AppEnv != "production" {
		return nil
	}
	if !strings.HasPrefix(strings.ToLower(cfg.BaseURL), "https://") {
		return errors.New("BASE_URL debe usar HTTPS en producción")
	}
	if cfg.DatabaseDriver != "postgres" && cfg.DatabaseDriver != "postgresql" && !strings.EqualFold(env("ALLOW_SQLITE_PRODUCTION", "false"), "true") {
		return errors.New("producción requiere PostgreSQL (DATABASE_URL); usa ALLOW_SQLITE_PRODUCTION=true solo para una emergencia temporal")
	}
	key := strings.TrimSpace(cfg.ChannelEncryptionKey)
	if len(key) < 32 || strings.Contains(strings.ToLower(key), "change-me") || key == cfg.AppName {
		return errors.New("CHANNEL_ENCRYPTION_KEY debe ser un secreto aleatorio de al menos 32 caracteres")
	}
	if cfg.RedisRequired && strings.TrimSpace(cfg.RedisURL) == "" {
		return errors.New("REDIS_REQUIRED=true pero REDIS_URL está vacío")
	}
	if cfg.MetaAppID != "" && (cfg.MetaAppSecret == "" || cfg.MessengerVerifyToken == "worktic_messenger_verify") {
		return errors.New("Meta está configurado pero faltan META_APP_SECRET o un MESSENGER_VERIFY_TOKEN seguro")
	}
	if cfg.WhatsAppVerifyToken == "worktic_whatsapp_verify" {
		return errors.New("WHATSAPP_VERIFY_TOKEN conserva el valor inseguro por defecto")
	}
	return nil
}

func newProductionRuntime(cfg Config) (*productionRuntime, error) {
	p := &productionRuntime{mem: map[string]*rateWindowV26{}}
	if strings.TrimSpace(cfg.RedisURL) == "" {
		return p, nil
	}
	opt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("REDIS_URL inválida: %w", err)
	}
	p.redis = redis.NewClient(opt)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err = p.redis.Ping(ctx).Err(); err != nil {
		if cfg.RedisRequired {
			return nil, fmt.Errorf("Redis no disponible: %w", err)
		}
		log.Printf("WARN Redis no disponible; rate limiting usa memoria local: %v", err)
		_ = p.redis.Close()
		p.redis = nil
	}
	return p, nil
}

func (p *productionRuntime) Close() {
	if p != nil && p.redis != nil {
		_ = p.redis.Close()
	}
}

func (p *productionRuntime) Ping(ctx context.Context) error {
	if p == nil || p.redis == nil {
		return nil
	}
	return p.redis.Ping(ctx).Err()
}

func (p *productionRuntime) allow(ctx context.Context, key string, limit int, window time.Duration) bool {
	if p == nil {
		return true
	}
	if p.redis != nil {
		bucket := fmt.Sprintf("worktic:rl:%s:%d", key, time.Now().Unix()/int64(window/time.Second))
		n, err := p.redis.Incr(ctx, bucket).Result()
		if err == nil {
			if n == 1 {
				_ = p.redis.Expire(ctx, bucket, window+5*time.Second).Err()
			}
			return n <= int64(limit)
		}
	}
	now := time.Now()
	p.mu.Lock()
	defer p.mu.Unlock()
	x := p.mem[key]
	if x == nil || now.After(x.Reset) {
		p.mem[key] = &rateWindowV26{Count: 1, Reset: now.Add(window)}
		return true
	}
	x.Count++
	if len(p.mem) > 20000 {
		for k, v := range p.mem {
			if now.After(v.Reset) {
				delete(p.mem, k)
			}
		}
	}
	return x.Count <= limit
}

func clientIPV26(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if x := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0]); x != "" {
			return x
		}
		if x := strings.TrimSpace(r.Header.Get("X-Real-IP")); x != "" {
			return x
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func validRequestIDV26(v string) bool {
	if v == "" || len(v) > 96 {
		return false
	}
	for _, r := range v {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

type responseRecorderV26 struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *responseRecorderV26) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *responseRecorderV26) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(p)
	w.bytes += n
	return n, err
}

func (w *responseRecorderV26) Flush() {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *responseRecorderV26) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (a *App) productionMiddleware(next http.Handler) http.Handler {
	base, _ := url.Parse(a.cfg.BaseURL)
	allowedOrigin := ""
	if base != nil {
		allowedOrigin = strings.ToLower(base.Scheme + "://" + base.Host)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		requestID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if !validRequestIDV26(requestID) {
			requestID = randomToken(12)
		}
		w.Header().Set("X-Request-ID", requestID)
		recorder := &responseRecorderV26{ResponseWriter: w}
		w = recorder
		defer func() {
			if r.URL.Path != "/healthz" && r.URL.Path != "/readyz" {
				status := recorder.status
				if status == 0 {
					status = http.StatusOK
				}
				log.Printf("http request_id=%s method=%s path=%s status=%d bytes=%d duration_ms=%d", requestID, r.Method, r.URL.Path, status, recorder.bytes, time.Since(started).Milliseconds())
			}
		}()
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, 20<<20)
		}
		ip := clientIPV26(r, a.cfg.TrustProxy)
		limit, window := 600, time.Minute
		if r.URL.Path == "/api/auth/login" {
			limit, window = 12, time.Minute
		}
		if r.URL.Path == "/api/auth/register" {
			limit, window = 6, 10*time.Minute
		}
		if strings.HasPrefix(r.URL.Path, "/webhooks/") {
			limit, window = 600, time.Minute
		}
		if a.prod != nil && !a.prod.allow(r.Context(), ip+":"+r.URL.Path, limit, window) {
			w.Header().Set("Retry-After", fmt.Sprintf("%d", int(window.Seconds())))
			writeError(w, errors.New("demasiadas solicitudes; intenta nuevamente más tarde"), http.StatusTooManyRequests)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") && r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
			if origin := strings.ToLower(strings.TrimSpace(r.Header.Get("Origin"))); origin != "" && allowedOrigin != "" && origin != allowedOrigin {
				writeError(w, errors.New("origen no permitido"), http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func hashSessionTokenV26(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func initProductionV26Schema(db *DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS security_audit_v26 (
 id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL DEFAULT 0, user_id INTEGER NOT NULL DEFAULT 0,
 event TEXT NOT NULL, ip TEXT NOT NULL DEFAULT '', metadata_json TEXT NOT NULL DEFAULT '{}', created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_security_audit_v26_tenant ON security_audit_v26(tenant_id,created_at);
CREATE TABLE IF NOT EXISTS production_migrations_v26 (
 migration_key TEXT PRIMARY KEY, status TEXT NOT NULL, details TEXT NOT NULL DEFAULT '', applied_at TEXT NOT NULL
);
`)
	return err
}

func (a *App) auditSecurityV26(r *http.Request, event string, meta map[string]any) {
	if a == nil || a.db == nil {
		return
	}
	var tid, uid int64
	if u := a.currentUser(r); u != nil {
		tid, uid = u.TenantID, u.ID
	}
	raw, _ := json.Marshal(meta)
	_, _ = a.db.Exec(`INSERT INTO security_audit_v26(tenant_id,user_id,event,ip,metadata_json,created_at) VALUES(?,?,?,?,?,?)`, tid, uid, event, clientIPV26(r, a.cfg.TrustProxy), string(raw), time.Now().UTC().Format(time.RFC3339))
}

func ensureBootstrapAdminV26(db *DB, cfg Config) error {
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM app_users`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	if cfg.BootstrapAdminEmail == "" || len(cfg.BootstrapAdminPassword) < 12 {
		return errors.New("base nueva sin usuarios: define BOOTSTRAP_ADMIN_EMAIL y BOOTSTRAP_ADMIN_PASSWORD (mínimo 12 caracteres) para el primer despliegue")
	}
	hash, err := hashPasswordSecure(cfg.BootstrapAdminPassword)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = db.Exec(`INSERT INTO app_users(name,email,password_hash,role,company,active,created_at,tenant_id) VALUES(?,?,?,'superadmin','Worktic',1,?,0)`, "Superadministrador", cfg.BootstrapAdminEmail, hash, now)
	if err != nil {
		return err
	}
	log.Printf("bootstrap seguro: superadmin creado para %s; elimina BOOTSTRAP_ADMIN_PASSWORD del entorno después del primer inicio", cfg.BootstrapAdminEmail)
	return nil
}

func runMaintenanceServerV26(cfg Config, db *DB) {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"maintenance":true}`))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"maintenance":true,"database":"ready"}`))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Retry-After", "300")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`<!doctype html><html lang="es"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Worktic AI · Actualización</title><style>body{font-family:system-ui;background:#09090f;color:#fff;display:grid;place-items:center;min-height:100vh;margin:0}.c{max-width:620px;padding:42px;border:1px solid #2b2940;border-radius:24px;background:#12111b;box-shadow:0 30px 80px #0008}h1{font-size:32px;margin:0 0 14px}p{color:#bbb7cc;line-height:1.6}.dot{display:inline-block;width:10px;height:10px;border-radius:50%;background:#8b12d6;margin-right:9px;box-shadow:0 0 20px #8b12d6}</style><div class="c"><h1><span class="dot"></span>Worktic AI se está actualizando</h1><p>Estamos aplicando una actualización de infraestructura y seguridad. Tus datos permanecen protegidos. Vuelve a intentarlo en unos minutos.</p></div></html>`))
	})
	srv := &http.Server{Addr: ":" + cfg.Port, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("maintenance server: %v", err)
	}
}

func (a *App) callAutomationWebhookV26(ctx context.Context, rawURL string, payload map[string]any) (map[string]any, error) {
	target, err := validateOutboundURLV26(rawURL, a.cfg.AutomationWebhookAllowedHosts)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
			if err != nil || len(ips) == 0 {
				return nil, fmt.Errorf("no se pudo resolver el host del webhook")
			}
			for _, ip := range ips {
				if !publicIPV26(ip) {
					return nil, fmt.Errorf("webhook bloqueado: el host resuelve a una red privada o reservada")
				}
			}
			d := net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
			return d.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          20,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 6 * time.Second,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return errors.New("demasiados redirects")
			}
			_, err := validateOutboundURLV26(req.URL.String(), a.cfg.AutomationWebhookAllowedHosts)
			return err
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target.String(), strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "WorkticAI-Automation/26")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	limited, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return nil, err
	}
	result := map[string]any{"status_code": resp.StatusCode, "response": string(limited)}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return result, fmt.Errorf("webhook respondió HTTP %d", resp.StatusCode)
	}
	return result, nil
}

func validateOutboundURLV26(rawURL, allowedHosts string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Hostname() == "" {
		return nil, errors.New("URL de webhook inválida")
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return nil, errors.New("los webhooks de automatización deben usar HTTPS")
	}
	if u.User != nil {
		return nil, errors.New("webhook con credenciales embebidas no permitido")
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if strings.TrimSpace(allowedHosts) != "" {
		allowed := false
		for _, item := range strings.Split(allowedHosts, ",") {
			item = strings.ToLower(strings.TrimSpace(strings.TrimSuffix(item, ".")))
			if item != "" && (host == item || strings.HasSuffix(host, "."+item)) {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, errors.New("host de webhook fuera de la allowlist")
		}
	}
	lookupCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	ips, err := net.DefaultResolver.LookupIP(lookupCtx, "ip", host)
	if err != nil || len(ips) == 0 {
		return nil, errors.New("no se pudo resolver el host del webhook")
	}
	for _, ip := range ips {
		if !publicIPV26(ip) {
			return nil, errors.New("webhook bloqueado: destino privado o reservado")
		}
	}
	return u, nil
}

func publicIPV26(ip net.IP) bool {
	if ip == nil || ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() {
		return false
	}
	// Carrier-grade NAT 100.64.0.0/10 and documentation/benchmark ranges are not valid webhook targets.
	if v4 := ip.To4(); v4 != nil {
		x := uint32(v4[0])<<24 | uint32(v4[1])<<16 | uint32(v4[2])<<8 | uint32(v4[3])
		ranges := [][2]uint32{
			{0x64400000, 0x647fffff}, // 100.64.0.0/10
			{0xc0000000, 0xc00000ff}, // 192.0.0.0/24
			{0xc0000200, 0xc00002ff}, // 192.0.2.0/24
			{0xc6336400, 0xc63364ff}, // 198.51.100.0/24
			{0xcb007100, 0xcb0071ff}, // 203.0.113.0/24
			{0xc6120000, 0xc613ffff}, // 198.18.0.0/15
			{0xf0000000, 0xffffffff}, // 240.0.0.0/4 + broadcast
		}
		for _, r := range ranges {
			if x >= r[0] && x <= r[1] {
				return false
			}
		}
	}
	// RFC 3849 IPv6 documentation prefix 2001:db8::/32 must never be used as a live webhook destination.
	if v6 := ip.To16(); v6 != nil && ip.To4() == nil {
		if v6[0] == 0x20 && v6[1] == 0x01 && v6[2] == 0x0d && v6[3] == 0xb8 {
			return false
		}
	}
	return true
}
