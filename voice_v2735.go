package main

// WorkticAI Voice V27.35. Financially isolated from Studio; every operation is
// tenant-scoped and voice remains disabled until explicitly enabled and configured.
import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

var voiceVoicesV2735 = []map[string]string{
	{"id": "marin", "name": "Marin · Cálida y cercana", "style": "femenina"},
	{"id": "cedar", "name": "Cedar · Serena y natural", "style": "masculina"},
	{"id": "coral", "name": "Coral · Clara y dinámica", "style": "femenina"},
	{"id": "alloy", "name": "Alloy · Neutra", "style": "neutra"},
	{"id": "sage", "name": "Sage · Tranquila", "style": "femenina"},
	{"id": "verse", "name": "Verse · Conversacional", "style": "masculina"},
	{"id": "ash", "name": "Ash · Expresiva", "style": "masculina"},
	{"id": "ballad", "name": "Ballad · Suave", "style": "neutra"},
}

func voiceID() string {
	var b [16]byte
	if _, e := rand.Read(b[:]); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b[:])
}
func voiceNow() string { return time.Now().UTC().Format(time.RFC3339Nano) }
func voiceBoolEnv(k string) bool {
	return strings.EqualFold(os.Getenv(k), "true") || os.Getenv(k) == "1"
}
func voiceKey(k string) string { return strings.TrimSpace(os.Getenv(k)) }
func voiceOutboundAllowed(phone string) bool {
	allowed := voiceKey("WORKTIC_VOICE_ALLOWED_OUTBOUND_PREFIXES")
	if allowed == "" {
		allowed = "+57"
	} // Conservative Colombian launch default.
	for _, prefix := range strings.Split(allowed, ",") {
		prefix = strings.TrimSpace(prefix)
		if len(prefix) > 1 && strings.HasPrefix(phone, prefix) {
			return true
		}
	}
	return false
}
func voiceNumber(n string) bool {
	if len(n) < 9 || len(n) > 16 || n[0] != '+' {
		return false
	}
	for _, c := range n[1:] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
func voiceValid(v string) bool {
	for _, x := range voiceVoicesV2735 {
		if x["id"] == v {
			return true
		}
	}
	return false
}
func voiceEnabled() bool    { return voiceBoolEnv("WORKTIC_VOICE_ENABLED") }
func voiceConfigured() bool { return voiceEnabled() && voiceKey("OPENAI_API_KEY") != "" }
func voiceSIPReady() bool {
	return voiceConfigured() && voiceKey("OPENAI_WEBHOOK_SECRET") != "" && voiceKey("OPENAI_PROJECT_ID") != "" && voiceKey("TWILIO_AUTH_TOKEN") != "" && voiceKey("WORKTIC_PUBLIC_BASE_URL") != ""
}
func voiceExternalURL(path string) string {
	return strings.TrimRight(voiceKey("WORKTIC_PUBLIC_BASE_URL"), "/") + path
}
func voiceDefaultPrice() int64 { return 30 } // 30 credits = USD 0.30; editable in admin before selling.

func initVoiceV2735Schema(db *DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS voice_config_v2735 (key TEXT PRIMARY KEY,value TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS voice_wallet_v2735(tenant_id BIGINT PRIMARY KEY, included BIGINT NOT NULL DEFAULT 0, purchased BIGINT NOT NULL DEFAULT 0, included_period TEXT NOT NULL DEFAULT '', included_plan TEXT NOT NULL DEFAULT '');
 CREATE TABLE IF NOT EXISTS voice_channels_v2735(id TEXT PRIMARY KEY,tenant_id BIGINT NOT NULL,agent_id BIGINT NOT NULL,name TEXT NOT NULL,phone TEXT NOT NULL DEFAULT '',voice TEXT NOT NULL DEFAULT 'marin', enabled INTEGER NOT NULL DEFAULT 0,max_minutes INTEGER NOT NULL DEFAULT 3,created_at TEXT NOT NULL,updated_at TEXT NOT NULL);
 CREATE UNIQUE INDEX IF NOT EXISTS idx_voice_channels_phone ON voice_channels_v2735(phone) WHERE phone<>'';
 CREATE INDEX IF NOT EXISTS idx_voice_channels_tenant ON voice_channels_v2735(tenant_id);
 CREATE TABLE IF NOT EXISTS voice_calls_v2735(id TEXT PRIMARY KEY,tenant_id BIGINT NOT NULL,channel_id TEXT NOT NULL,agent_id BIGINT NOT NULL,direction TEXT NOT NULL,phone TEXT NOT NULL DEFAULT '',provider_sid TEXT NOT NULL DEFAULT '',openai_id TEXT NOT NULL DEFAULT '',status TEXT NOT NULL,held_included BIGINT NOT NULL DEFAULT 0,held_purchased BIGINT NOT NULL DEFAULT 0,price_per_minute BIGINT NOT NULL DEFAULT 30,charged BIGINT NOT NULL DEFAULT 0,elapsed_seconds INTEGER NOT NULL DEFAULT 0,created_at TEXT NOT NULL,finished_at TEXT NOT NULL DEFAULT '',error_code TEXT NOT NULL DEFAULT '');
 CREATE UNIQUE INDEX IF NOT EXISTS idx_voice_calls_provider ON voice_calls_v2735(provider_sid) WHERE provider_sid<>'';
 CREATE INDEX IF NOT EXISTS idx_voice_calls_tenant ON voice_calls_v2735(tenant_id,created_at);
 CREATE TABLE IF NOT EXISTS voice_topups_v2735(id TEXT PRIMARY KEY,tenant_id BIGINT NOT NULL,package_code TEXT NOT NULL,credits BIGINT NOT NULL,price_cents BIGINT NOT NULL,status TEXT NOT NULL DEFAULT 'pending',payment_reference TEXT NOT NULL DEFAULT '',created_at TEXT NOT NULL,reviewed_at TEXT NOT NULL DEFAULT '',reviewer_id BIGINT NOT NULL DEFAULT 0);
 CREATE INDEX IF NOT EXISTS idx_voice_topups_tenant ON voice_topups_v2735(tenant_id,created_at);
 CREATE TABLE IF NOT EXISTS voice_movements_v2735(id TEXT PRIMARY KEY,tenant_id BIGINT NOT NULL,call_id TEXT NOT NULL DEFAULT '',kind TEXT NOT NULL,credits BIGINT NOT NULL,created_at TEXT NOT NULL);
 CREATE INDEX IF NOT EXISTS idx_voice_movements_tenant ON voice_movements_v2735(tenant_id,created_at);
 CREATE TABLE IF NOT EXISTS voice_events_v2735(id TEXT PRIMARY KEY,created_at TEXT NOT NULL);`)
	return err
}
func (a *App) voiceSetting(key string, fallback int64) int64 {
	var raw string
	if a.db.QueryRow(`SELECT value FROM voice_config_v2735 WHERE key=?`, key).Scan(&raw) != nil {
		return fallback
	}
	n, e := strconv.ParseInt(raw, 10, 64)
	if e != nil || n < 0 {
		return fallback
	}
	return n
}
func (a *App) voiceTextSetting(key string) string {
	var value string
	if a.db.QueryRow(`SELECT value FROM voice_config_v2735 WHERE key=?`, key).Scan(&value) != nil {
		return ""
	}
	return value
}
func (a *App) voiceTopupsEnabled() bool {
	return voiceConfigured() && a.voiceSetting("sales_enabled", 0) == 1 && a.voiceTextSetting("payment_instructions") != ""
}
func (a *App) voicePrice() int64 {
	n := a.voiceSetting("price_per_minute_credits", voiceDefaultPrice())
	if n < 10 || n > 1000 {
		return voiceDefaultPrice()
	}
	return n
}
func (a *App) voiceEnterpriseIncluded() int64 {
	n := a.voiceSetting("enterprise_monthly_credits", 300)
	if n > 100000 {
		return 300
	}
	return n
}
func voicePackages(price int64) []map[string]any {
	return []map[string]any{
		{"code": "voice_10", "name": "Inicio · 10 minutos", "credits": price * 10, "price_cents": price * 10},
		{"code": "voice_30", "name": "Crecimiento · 30 minutos", "credits": price * 30, "price_cents": price * 30},
		{"code": "voice_100", "name": "Escala · 100 minutos", "credits": price * 100, "price_cents": price * 100},
	}
}
func (a *App) voicePlan(tenant int64) (string, error) {
	var owner int64
	if e := a.db.QueryRow(`SELECT owner_user_id FROM tenants WHERE id=?`, tenant).Scan(&owner); e != nil {
		return "", e
	}
	plan, _, e := a.activePlan(owner)
	if e != nil {
		return "", e
	}
	return plan.Code, nil
}

// A monthly grant belongs only to an active Enterprise subscription. Old unused
// monthly credits expire; purchased credits are never affected by renewal.
func (a *App) voiceEnsureWallet(tenant int64) (string, error) {
	code, e := a.voicePlan(tenant)
	if e != nil {
		return "", e
	}
	now := time.Now().UTC().Format("2006-01")
	allowance := int64(0)
	if code == "enterprise" && voiceEnabled() {
		allowance = a.voiceEnterpriseIncluded()
	}
	tx, e := a.db.Begin()
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	if _, e = tx.Exec(`INSERT INTO voice_wallet_v2735(tenant_id) VALUES(?) ON CONFLICT(tenant_id) DO NOTHING`, tenant); e != nil {
		return "", e
	}
	_, e = tx.Exec(`UPDATE voice_wallet_v2735 SET included=?,included_period=?,included_plan=? WHERE tenant_id=? AND (included_period<>? OR included_plan<>?)`, allowance, now, code, tenant, now, code)
	if e != nil {
		return "", e
	}
	return code, tx.Commit()
}
func (a *App) voiceBalances(tenant int64) (int64, int64, error) {
	var included, purchased int64
	e := a.db.QueryRow(`SELECT included,purchased FROM voice_wallet_v2735 WHERE tenant_id=?`, tenant).Scan(&included, &purchased)
	return included, purchased, e
}
func (a *App) voiceReserve(tenant int64, channel, dir, phone, provider string, agent int64, maxMinutes int) (string, error) {
	if !voiceConfigured() {
		return "", errors.New("Llamadas IA todavía no está habilitado por la plataforma")
	}
	if maxMinutes < 1 || maxMinutes > 15 {
		return "", errors.New("duración fuera de límites")
	}
	if _, err := a.voiceEnsureWallet(tenant); err != nil {
		return "", err
	}
	var activeCount int
	if e := a.db.QueryRow(`SELECT COUNT(*) FROM voice_calls_v2735 WHERE tenant_id=? AND status IN ('reserved','accepting','active','uncertain')`, tenant).Scan(&activeCount); e != nil {
		return "", e
	}
	if activeCount >= 2 {
		return "", errors.New("Máximo 2 llamadas simultáneas por empresa")
	}
	rate := a.voicePrice()
	cost := int64(maxMinutes) * rate
	id := voiceID()
	tx, err := a.db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	var inc, buy int64
	// An atomic conditional UPDATE prevents concurrent calls from overdrawing.
	err = tx.QueryRow(`SELECT included,purchased FROM voice_wallet_v2735 WHERE tenant_id=?`, tenant).Scan(&inc, &buy)
	if err != nil {
		return "", err
	}
	if inc+buy < cost {
		return "", errors.New("saldo de Llamadas IA insuficiente para reservar la duración máxima")
	}
	useInc := inc
	if useInc > cost {
		useInc = cost
	}
	useBuy := cost - useInc
	res, err := tx.Exec(`UPDATE voice_wallet_v2735 SET included=included-?,purchased=purchased-? WHERE tenant_id=? AND included>=? AND purchased>=?`, useInc, useBuy, tenant, useInc, useBuy)
	if err != nil {
		return "", err
	}
	affected, _ := res.RowsAffected()
	if affected != 1 {
		return "", errors.New("saldo cambió durante la reserva")
	}
	_, err = tx.Exec(`INSERT INTO voice_calls_v2735(id,tenant_id,channel_id,agent_id,direction,phone,provider_sid,status,held_included,held_purchased,price_per_minute,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, id, tenant, channel, agent, dir, phone, provider, "reserved", useInc, useBuy, rate, voiceNow())
	if err != nil {
		return "", err
	}
	_, err = tx.Exec(`INSERT INTO voice_movements_v2735(id,tenant_id,call_id,kind,credits,created_at) VALUES(?,?,?,?,?,?)`, voiceID(), tenant, id, "reserve", -cost, voiceNow())
	if err != nil {
		return "", err
	}
	return id, tx.Commit()
}

// Only trusted server-side lifecycle callbacks or observers may settle a call.
// Replayed provider events cannot credit the wallet twice.
func (a *App) voiceSettle(id, status string, seconds int, errorCode string) error {
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var tenant, inc, buy, price int64
	var current, created string
	err = tx.QueryRow(`SELECT tenant_id,held_included,held_purchased,price_per_minute,status,created_at FROM voice_calls_v2735 WHERE id=?`, id).Scan(&tenant, &inc, &buy, &price, &current, &created)
	if err != nil {
		return err
	}
	if current == "completed" || current == "failed" || current == "cancelled" {
		return nil
	}
	if seconds < 0 {
		seconds = 0
	}
	held := inc + buy
	charged := int64((seconds+59)/60) * price
	if seconds > 0 && charged == 0 {
		charged = price
	}
	if charged > held {
		charged = held
	}
	if status == "failed" || status == "cancelled" {
		charged = 0
	}
	// refund purchased first? Refund follows the original funding split (included first).
	spentInc := charged
	if spentInc > inc {
		spentInc = inc
	}
	spentBuy := charged - spentInc
	refundedInc := inc - spentInc
	refundedBuy := buy - spentBuy
	var grantPeriod, grantPlan string
	if err = tx.QueryRow(`SELECT included_period,included_plan FROM voice_wallet_v2735 WHERE tenant_id=?`, tenant).Scan(&grantPeriod, &grantPlan); err != nil {
		return err
	}
	if grantPeriod != created[:7] || grantPlan != "enterprise" {
		refundedInc = 0
	} // expired monthly grant cannot roll over
	_, err = tx.Exec(`UPDATE voice_wallet_v2735 SET included=included+?,purchased=purchased+? WHERE tenant_id=?`, refundedInc, refundedBuy, tenant)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`UPDATE voice_calls_v2735 SET status=?,charged=?,elapsed_seconds=?,error_code=?,finished_at=? WHERE id=? AND status NOT IN ('completed','failed','cancelled')`, status, charged, seconds, errorCode, voiceNow(), id)
	if err != nil {
		return err
	}
	if refundedInc+refundedBuy > 0 {
		_, err = tx.Exec(`INSERT INTO voice_movements_v2735(id,tenant_id,call_id,kind,credits,created_at) VALUES(?,?,?,?,?,?)`, voiceID(), tenant, id, "refund", refundedInc+refundedBuy, voiceNow())
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}
func voiceReadJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 32768)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		http.Error(w, "Solicitud inválida", 400)
		return false
	}
	return true
}
func (a *App) voiceOverviewHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", 405)
		return
	}
	tid, _, err := a.tenantFor(r)
	if err != nil {
		http.Error(w, "Sesión requerida", 401)
		return
	}
	plan, err := a.voiceEnsureWallet(tid)
	if err != nil {
		http.Error(w, "No se pudo verificar la suscripción", 503)
		return
	}
	inc, buy, err := a.voiceBalances(tid)
	if err != nil {
		http.Error(w, "Billetera no disponible", 503)
		return
	}
	channels := []map[string]any{}
	rows, err := a.db.Query(`SELECT id,name,phone,agent_id,voice,enabled,max_minutes FROM voice_channels_v2735 WHERE tenant_id=? ORDER BY created_at DESC`, tid)
	if err == nil {
		for rows.Next() {
			var id, name, phone, voice string
			var agent int64
			var enabled, max int
			if rows.Scan(&id, &name, &phone, &agent, &voice, &enabled, &max) == nil {
				channels = append(channels, map[string]any{"id": id, "name": name, "phone": phone, "agent_id": agent, "voice": voice, "enabled": enabled == 1, "max_minutes": max})
			}
		}
		rows.Close()
	}
	agents := []map[string]any{}
	ag, err := a.db.Query(`SELECT id,name,status FROM ai_agents WHERE tenant_id=? ORDER BY id`, tid)
	if err == nil {
		for ag.Next() {
			var id int64
			var name, status string
			if ag.Scan(&id, &name, &status) == nil {
				agents = append(agents, map[string]any{"id": id, "name": name, "status": status})
			}
		}
		ag.Close()
	}
	calls := []map[string]any{}
	cr, err := a.db.Query(`SELECT id,direction,phone,status,charged,elapsed_seconds,created_at FROM voice_calls_v2735 WHERE tenant_id=? ORDER BY created_at DESC LIMIT 50`, tid)
	if err == nil {
		for cr.Next() {
			var id, dir, phone, status, date string
			var charged int64
			var secs int
			if cr.Scan(&id, &dir, &phone, &status, &charged, &secs, &date) == nil {
				calls = append(calls, map[string]any{"id": id, "direction": dir, "phone": phone, "status": status, "charged": charged, "seconds": secs, "created_at": date})
			}
		}
		cr.Close()
	}
	tops := []map[string]any{}
	tr, err := a.db.Query(`SELECT id,package_code,credits,price_cents,status,created_at FROM voice_topups_v2735 WHERE tenant_id=? ORDER BY created_at DESC LIMIT 25`, tid)
	if err == nil {
		for tr.Next() {
			var id, code, status, date string
			var credits, cents int64
			if tr.Scan(&id, &code, &credits, &cents, &status, &date) == nil {
				tops = append(tops, map[string]any{"id": id, "code": code, "credits": credits, "price_cents": cents, "status": status, "date": date})
			}
		}
		tr.Close()
	}
	// Studio continues to be the source of truth for Studio balances.
	commercialSetJSON(w, 200, map[string]any{"version": "27.35", "plan": plan, "wallet": map[string]any{"included": inc, "purchased": buy, "available": inc + buy, "period": time.Now().UTC().Format("2006-01")}, "price_per_minute_credits": a.voicePrice(), "enterprise_monthly_credits": a.voiceEnterpriseIncluded(), "active": voiceConfigured(), "sip_ready": voiceSIPReady(), "voices": voiceVoicesV2735, "channels": channels, "agents": agents, "calls": calls, "topups": tops, "packages": voicePackages(a.voicePrice()), "mode": "manual_payment_review", "sales_enabled": a.voiceTopupsEnabled(), "payment_instructions": a.voiceTextSetting("payment_instructions"), "payment_url": a.voiceTextSetting("payment_url")})
}
func (a *App) voiceChannelHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	tid, _, e := a.tenantFor(r)
	if e != nil {
		http.Error(w, "Sesión requerida", 401)
		return
	}
	var q struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		Phone      string `json:"phone"`
		Voice      string `json:"voice"`
		AgentID    int64  `json:"agent_id"`
		Enabled    bool   `json:"enabled"`
		MaxMinutes int    `json:"max_minutes"`
	}
	if !voiceReadJSON(w, r, &q) {
		return
	}
	if !a.voiceTopupsEnabled() {
		http.Error(w, "Recargas de voz aún no habilitadas o sin datos de pago", 409)
		return
	}
	plan, e := a.voicePlan(tid)
	if e != nil || plan == "free" {
		http.Error(w, "Se requiere un plan de pago para Llamadas IA", 403)
		return
	}
	q.Name = strings.TrimSpace(q.Name)
	q.Phone = strings.TrimSpace(q.Phone)
	if len(q.Name) < 3 || len(q.Name) > 80 || !voiceValid(q.Voice) || q.AgentID <= 0 || q.MaxMinutes < 1 || q.MaxMinutes > 15 || (q.Phone != "" && !voiceNumber(q.Phone)) {
		http.Error(w, "Configuración inválida", 400)
		return
	}
	var status string
	e = a.db.QueryRow(`SELECT status FROM ai_agents WHERE tenant_id=? AND id=?`, tid, q.AgentID).Scan(&status)
	if e != nil || (q.Enabled && status != "active") {
		http.Error(w, "Agente inexistente o no disponible", 400)
		return
	}
	if q.Enabled && !voiceConfigured() {
		http.Error(w, "La plataforma no ha habilitado todavía el proveedor de voz", 409)
		return
	}
	enabled := 0
	if q.Enabled {
		enabled = 1
	}
	if q.ID != "" {
		res, e := a.db.Exec(`UPDATE voice_channels_v2735 SET name=?,phone=?,voice=?,agent_id=?,enabled=?,max_minutes=?,updated_at=? WHERE id=? AND tenant_id=?`, q.Name, q.Phone, q.Voice, q.AgentID, enabled, q.MaxMinutes, voiceNow(), q.ID, tid)
		if e != nil {
			http.Error(w, "Número ya asignado o datos inválidos", 409)
			return
		}
		n, _ := res.RowsAffected()
		if n != 1 {
			http.Error(w, "Canal no encontrado", 404)
			return
		}
		commercialSetJSON(w, 200, map[string]any{"id": q.ID, "saved": true})
		return
	}
	id := voiceID()
	_, e = a.db.Exec(`INSERT INTO voice_channels_v2735(id,tenant_id,agent_id,name,phone,voice,enabled,max_minutes,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, id, tid, q.AgentID, q.Name, q.Phone, q.Voice, enabled, q.MaxMinutes, voiceNow(), voiceNow())
	if e != nil {
		http.Error(w, "No se pudo crear el canal; verifica el número", 409)
		return
	}
	commercialSetJSON(w, 200, map[string]any{"id": id, "saved": true})
}
func (a *App) voiceTopupsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	tid, _, e := a.tenantFor(r)
	if e != nil {
		http.Error(w, "Sesión requerida", 401)
		return
	}
	if !a.voiceTopupsEnabled() {
		http.Error(w, "Recargas de voz no habilitadas", 403)
		return
	}
	var q struct {
		Package string `json:"package"`
		Network string `json:"network"`
		TxHash  string `json:"tx_hash"`
	}
	if !voiceReadJSON(w, r, &q) {
		return
	}
	q.Package = strings.TrimSpace(q.Package)
	q.Network = strings.ToUpper(strings.TrimSpace(q.Network))
	q.TxHash = strings.TrimSpace(q.TxHash)
	if !commercialTxHash(q.Network, q.TxHash) {
		http.Error(w, "Red o hash de transacción inválidos", 400)
		return
	}
	if (q.Network == "BEP20" && strings.TrimSpace(a.cfg.USDTBEP20Address) == "") ||
		(q.Network == "TRC20" && strings.TrimSpace(a.cfg.USDTTRC20Address) == "") {
		http.Error(w, "No existe dirección oficial para esta red", 503)
		return
	}
	plan, e := a.voicePlan(tid)
	if e != nil || plan == "free" {
		http.Error(w, "Se requiere un plan de pago para recargar Llamadas IA", 403)
		return
	}
	var matched map[string]any
	for _, p := range voicePackages(a.voicePrice()) {
		if p["code"] == q.Package {
			matched = p
			break
		}
	}
	if matched == nil {
		http.Error(w, "Paquete de voz inválido", 400)
		return
	}
	id := voiceID()
	tx, e := a.db.Begin()
	if e != nil {
		http.Error(w, "No se pudo preparar la operación", 503)
		return
	}
	defer tx.Rollback()
	// The same transaction hash cannot be used for subscription, Studio and Voice.
	if e = billingClaimTxV2728(tx, q.Network, q.TxHash, "voice_topup", id, tid); e != nil {
		http.Error(w, "La transacción ya está registrada o no es válida", 409)
		return
	}
	ref := q.Network + ":" + strings.ToLower(q.TxHash)
	if _, e = tx.Exec(`INSERT INTO voice_topups_v2735(id,tenant_id,package_code,credits,price_cents,payment_reference,status,created_at) VALUES(?,?,?,?,?,?,'pending',?)`, id, tid, q.Package, matched["credits"], matched["price_cents"], ref, voiceNow()); e != nil {
		http.Error(w, "No se pudo registrar la solicitud", 503)
		return
	}
	if e = tx.Commit(); e != nil {
		http.Error(w, "Error guardando la solicitud", 503)
		return
	}
	commercialSetJSON(w, 201, map[string]any{"id": id, "status": "pending", "message": "Solicitud registrada. Los créditos requieren aprobación manual.", "credited": false})
}
func (a *App) voiceAdminHandler(w http.ResponseWriter, r *http.Request) {
	admin, ok := a.requireSuperadmin(w, r)
	if !ok {
		return
	}
	if r.Method == http.MethodGet {
		pending := []map[string]any{}
		rows, e := a.db.Query(`SELECT id,tenant_id,credits,price_cents,payment_reference,created_at FROM voice_topups_v2735 WHERE status='pending' ORDER BY created_at DESC LIMIT 100`)
		if e == nil {
			for rows.Next() {
				var id, ref, date string
				var tid, credits, cents int64
				if rows.Scan(&id, &tid, &credits, &cents, &ref, &date) == nil {
					pending = append(pending, map[string]any{"id": id, "tenant_id": tid, "credits": credits, "price_cents": cents, "reference": ref, "date": date})
				}
			}
			rows.Close()
		}
		commercialSetJSON(w, 200, map[string]any{"enabled": voiceEnabled(), "provider_ready": voiceConfigured(), "sip_ready": voiceSIPReady(), "price_per_minute_credits": a.voicePrice(), "enterprise_monthly_credits": a.voiceEnterpriseIncluded(), "pending": pending,
			"sales_enabled": a.voiceTopupsEnabled(), "sales_enabled_setting": a.voiceSetting("sales_enabled", 0) == 1,
			"payment_instructions": a.voiceTextSetting("payment_instructions"), "payment_url": a.voiceTextSetting("payment_url")})
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	var q struct {
		Action              string `json:"action"`
		ID                  string `json:"id"`
		Price               int64  `json:"price_per_minute_credits"`
		Enterprise          int64  `json:"enterprise_monthly_credits"`
		SalesEnabled        bool   `json:"sales_enabled"`
		PaymentInstructions string `json:"payment_instructions"`
		PaymentURL          string `json:"payment_url"`
	}
	if !voiceReadJSON(w, r, &q) {
		return
	}
	if q.Action == "settings" {
		if q.Price < 10 || q.Price > 1000 || q.Enterprise < 0 || q.Enterprise > 100000 || len(q.PaymentInstructions) > 500 || len(q.PaymentURL) > 300 {
			http.Error(w, "Tarifas fuera de límites", 400)
			return
		}
		if q.PaymentURL != "" {
			u, e := url.Parse(q.PaymentURL)
			if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
				http.Error(w, "Enlace de pago HTTPS inválido", 400)
				return
			}
		}
		tx, e := a.db.Begin()
		if e != nil {
			http.Error(w, "BD no disponible", 503)
			return
		}
		defer tx.Rollback()
		sales := "0"
		if q.SalesEnabled {
			sales = "1"
		}
		for _, entry := range [][2]string{{"price_per_minute_credits", strconv.FormatInt(q.Price, 10)}, {"enterprise_monthly_credits", strconv.FormatInt(q.Enterprise, 10)}, {"sales_enabled", sales}, {"payment_instructions", strings.TrimSpace(q.PaymentInstructions)}, {"payment_url", strings.TrimSpace(q.PaymentURL)}} {
			if _, e = tx.Exec(`INSERT INTO voice_config_v2735(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, entry[0], entry[1]); e != nil {
				http.Error(w, "No se pudo guardar", 503)
				return
			}
		}
		if e = tx.Commit(); e != nil {
			http.Error(w, "No se pudo guardar", 503)
			return
		}
		commercialSetJSON(w, 200, map[string]any{"saved": true})
		return
	}
	if q.Action != "approve" && q.Action != "reject" {
		http.Error(w, "Acción desconocida", 400)
		return
	}
	if len(q.ID) != 32 {
		http.Error(w, "ID inválido", 400)
		return
	}
	tx, e := a.db.Begin()
	if e != nil {
		http.Error(w, "BD no disponible", 503)
		return
	}
	defer tx.Rollback()
	var tid, credits int64
	var state string
	e = tx.QueryRow(`SELECT tenant_id,credits,status FROM voice_topups_v2735 WHERE id=?`, q.ID).Scan(&tid, &credits, &state)
	if e != nil {
		http.Error(w, "Solicitud inexistente", 404)
		return
	}
	if state != "pending" {
		http.Error(w, "Solicitud ya revisada", 409)
		return
	}
	newState := "rejected"
	if q.Action == "approve" {
		newState = "approved"
	}
	res, e := tx.Exec(`UPDATE voice_topups_v2735 SET status=?,reviewed_at=?,reviewer_id=? WHERE id=? AND status='pending'`, newState, voiceNow(), admin.ID, q.ID)
	if e != nil {
		http.Error(w, "Error de revisión", 503)
		return
	}
	count, _ := res.RowsAffected()
	if count != 1 {
		http.Error(w, "Solicitud revisada simultáneamente", 409)
		return
	}
	if newState == "approved" {
		if _, e = tx.Exec(`INSERT INTO voice_wallet_v2735(tenant_id) VALUES(?) ON CONFLICT(tenant_id) DO NOTHING`, tid); e != nil {
			http.Error(w, "No se pudo abrir billetera", 503)
			return
		}
		if _, e = tx.Exec(`UPDATE voice_wallet_v2735 SET purchased=purchased+? WHERE tenant_id=?`, credits, tid); e != nil {
			http.Error(w, "No se pudo acreditar", 503)
			return
		}
		if _, e = tx.Exec(`INSERT INTO voice_movements_v2735(id,tenant_id,kind,credits,created_at) VALUES(?,?,?,?,?)`, voiceID(), tid, "topup", credits, voiceNow()); e != nil {
			http.Error(w, "No se pudo registrar movimiento", 503)
			return
		}
	}
	if e = tx.Commit(); e != nil {
		http.Error(w, "No se pudo guardar revisión", 503)
		return
	}
	commercialSetJSON(w, 200, map[string]any{"status": newState, "id": q.ID})
}
func voicePrompt(agent AIAgent) string {
	info := strings.Join([]string{agent.Objective, agent.Instructions, agent.Knowledge}, "\n")
	if len(info) > 12000 {
		info = info[:12000]
	}
	return "Eres " + agent.Name + ", agente de WorkticAI de la empresa. Habla en español latino claro, amable y natural. Di desde el inicio que eres una asistente o asistente de inteligencia artificial. No inventes precios, disponibilidad, pagos ni resultados. Solicita confirmación antes de registrar datos y deriva al equipo humano si algo excede la información. No ejecutes herramientas ni prometas agendamientos que no estén conectados. Mantén respuestas cortas y conversacionales.\n" + info
}
func (a *App) voiceAgent(tenant, agentID int64) (AIAgent, error) {
	var ag AIAgent
	err := a.db.QueryRow(`SELECT name,objective,instructions,knowledge FROM ai_agents WHERE tenant_id=? AND id=?`, tenant, agentID).Scan(&ag.Name, &ag.Objective, &ag.Instructions, &ag.Knowledge)
	return ag, err
}
func (a *App) voiceChannel(id string) (tenant, agent int64, voice string, max int, err error) {
	var enabled int
	err = a.db.QueryRow(`SELECT tenant_id,agent_id,voice,max_minutes,enabled FROM voice_channels_v2735 WHERE id=?`, id).Scan(&tenant, &agent, &voice, &max, &enabled)
	if err == nil && enabled != 1 {
		err = errors.New("canal inactivo")
	}
	return
}
func voicePostOpenAI(ctx context.Context, route string, body any) ([]byte, int, error) {
	var content io.Reader
	if body != nil {
		b, e := json.Marshal(body)
		if e != nil {
			return nil, 0, e
		}
		content = bytes.NewReader(b)
	}
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/"+route, content)
	if e != nil {
		return nil, 0, e
	}
	req.Header.Set("Authorization", "Bearer "+voiceKey("OPENAI_API_KEY"))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 15 * time.Second}
	resp, e := client.Do(req)
	if e != nil {
		return nil, 0, e
	}
	defer resp.Body.Close()
	b, e := io.ReadAll(io.LimitReader(resp.Body, 524288))
	return b, resp.StatusCode, e
}
func (a *App) voiceBrowserHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	tid, _, e := a.tenantFor(r)
	if e != nil {
		http.Error(w, "Sesión requerida", 401)
		return
	}
	var q struct {
		ChannelID string `json:"channel_id"`
		SDP       string `json:"sdp"`
	}
	if !voiceReadJSON(w, r, &q) {
		return
	}
	if len(q.SDP) < 50 || len(q.SDP) > 24000 {
		http.Error(w, "SDP inválido", 400)
		return
	}
	tenant, agent, voice, max, e := a.voiceChannel(q.ChannelID)
	if e != nil || tenant != tid {
		http.Error(w, "Canal inactivo o no autorizado", 403)
		return
	}
	if max > 3 {
		max = 3
	} // browser trials have a strict 3-minute safety cap.
	ag, e := a.voiceAgent(tid, agent)
	if e != nil {
		http.Error(w, "Agente no disponible", 503)
		return
	}
	call, e := a.voiceReserve(tid, q.ChannelID, "browser", "", "", agent, max)
	if e != nil {
		http.Error(w, e.Error(), 409)
		return
	}
	payload := map[string]any{"session": map[string]any{"model": "gpt-live-1", "instructions": voicePrompt(ag), "audio": map[string]any{"output": map[string]any{"voice": voice}}}, "transport": map[string]any{"type": "webrtc", "sdp": q.SDP}}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	response, code, err := voicePostOpenAI(ctx, "live/sessions", payload)
	if err != nil || code != 201 {
		_ = a.voiceSettle(call, "failed", 0, "provider_connection")
		http.Error(w, "No fue posible conectar con el proveedor de voz", 502)
		return
	}
	var ans struct {
		Session struct {
			ID string `json:"id"`
		} `json:"session"`
		Transport struct {
			SDP string `json:"sdp"`
		} `json:"transport"`
	}
	if json.Unmarshal(response, &ans) != nil || ans.Session.ID == "" || ans.Transport.SDP == "" {
		_ = a.voiceSettle(call, "failed", 0, "invalid_provider_answer")
		http.Error(w, "Respuesta no válida del proveedor", 502)
		return
	}
	_, _ = a.db.Exec(`UPDATE voice_calls_v2735 SET openai_id=?,status='active' WHERE id=?`, ans.Session.ID, call)
	// The server retains charging authority; the client never writes usage.
	go a.voiceObserveLive(call, ans.Session.ID, max)
	commercialSetJSON(w, 200, map[string]any{"sdp": ans.Transport.SDP, "call_id": call, "max_seconds": max * 60})
}
func voiceTwilioValid(r *http.Request) bool {
	secret := voiceKey("TWILIO_AUTH_TOKEN")
	base := voiceKey("WORKTIC_PUBLIC_BASE_URL")
	if secret == "" || base == "" || r.ParseForm() != nil {
		return false
	}
	full := voiceExternalURL(r.URL.RequestURI())
	if len(r.URL.RawQuery) > 0 { // exact public URL includes query exactly once.
		full = voiceExternalURL(r.URL.Path) + "?" + r.URL.RawQuery
	}
	keys := make([]string, 0, len(r.PostForm))
	for key := range r.PostForm {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		full += key + r.PostForm.Get(key)
	}
	mac := hmac.New(sha1.New, []byte(secret))
	mac.Write([]byte(full))
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	provided := r.Header.Get("X-Twilio-Signature")
	return provided != "" && hmac.Equal([]byte(provided), []byte(expected))
}
func voiceTwiML(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	_, _ = io.WriteString(w, "<?xml version=\"1.0\" encoding=\"UTF-8\"?><Response>"+body+"</Response>")
}
func (a *App) voiceTwilioInboundHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	if !voiceTwilioValid(r) {
		http.Error(w, "Firma inválida", 403)
		return
	}
	if !voiceSIPReady() {
		voiceTwiML(w, "<Say language=\"es-MX\">Este servicio no está disponible.</Say>")
		return
	}
	from := r.PostForm.Get("From")
	to := r.PostForm.Get("To")
	sid := r.PostForm.Get("CallSid")
	if !voiceNumber(to) || sid == "" {
		http.Error(w, "Datos telefónicos inválidos", 400)
		return
	}
	var channel string
	var tenant, agent int64
	var max int
	err := a.db.QueryRow(`SELECT id,tenant_id,agent_id,max_minutes FROM voice_channels_v2735 WHERE phone=? AND enabled=1 LIMIT 1`, to).Scan(&channel, &tenant, &agent, &max)
	if err != nil {
		voiceTwiML(w, "<Say language=\"es-MX\">Número no configurado.</Say>")
		return
	}
	// Twilio may retry the same inbound webhook. Reuse the original reservation.
	var call string
	err = a.db.QueryRow(`SELECT id FROM voice_calls_v2735 WHERE provider_sid=?`, sid).Scan(&call)
	if err == sql.ErrNoRows {
		call, err = a.voiceReserve(tenant, channel, "inbound", from, sid, agent, max)
	}
	if err != nil {
		voiceTwiML(w, "<Say language=\"es-MX\">En este momento no podemos atender tu llamada.</Say>")
		return
	}
	project := voiceKey("OPENAI_PROJECT_ID")
	if !strings.HasPrefix(project, "proj_") {
		http.Error(w, "Proyecto SIP sin configurar", 503)
		return
	}
	sip := "sip:" + project + "@sip.api.openai.com;transport=tls?x-worktic-call=" + call
	var escaped bytes.Buffer
	_ = xml.EscapeText(&escaped, []byte(sip))
	voiceTwiML(w, fmt.Sprintf("<Say language=\"es-MX\">Te atenderá un agente de inteligencia artificial.</Say><Dial timeLimit=\"%d\" action=\"%s\" method=\"POST\"><Sip>%s</Sip></Dial>", max*60, voiceExternalURL("/webhooks/voice/twilio/status"), escaped.String()))
}
func (a *App) voiceTwilioStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	if !voiceTwilioValid(r) {
		http.Error(w, "Firma inválida", 403)
		return
	}
	sid := r.PostForm.Get("CallSid")
	if sid == "" {
		http.Error(w, "Llamada inválida", 400)
		return
	}
	var id string
	err := a.db.QueryRow(`SELECT id FROM voice_calls_v2735 WHERE provider_sid=?`, sid).Scan(&id)
	if err != nil {
		http.Error(w, "Llamada desconocida", 404)
		return
	}
	duration, _ := strconv.Atoi(r.PostForm.Get("DialCallDuration"))
	if duration == 0 {
		duration, _ = strconv.Atoi(r.PostForm.Get("CallDuration"))
	}
	outcome := r.PostForm.Get("DialCallStatus")
	status := "completed"
	if outcome == "failed" || outcome == "busy" || outcome == "no-answer" || outcome == "canceled" {
		status = "failed"
	}
	if duration < 0 {
		duration = 0
	}
	if e := a.voiceSettle(id, status, duration, outcome); e != nil {
		http.Error(w, "Error al liquidar", 503)
		return
	}
	voiceTwiML(w, "")
}

// Standard Webhooks signing scheme: secret base64 payload prefixed whsec_.
func voiceOpenAISignatureValid(r *http.Request, body []byte) bool {
	raw := voiceKey("OPENAI_WEBHOOK_SECRET")
	raw = strings.TrimPrefix(raw, "whsec_")
	secret, e := base64.StdEncoding.DecodeString(raw)
	if e != nil {
		return false
	}
	id, stamp, sig := r.Header.Get("webhook-id"), r.Header.Get("webhook-timestamp"), r.Header.Get("webhook-signature")
	sec, e := strconv.ParseInt(stamp, 10, 64)
	if id == "" || e != nil || time.Since(time.Unix(sec, 0)) > 5*time.Minute || time.Until(time.Unix(sec, 0)) > 5*time.Minute {
		return false
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(id + "." + stamp + "." + string(body)))
	expected := mac.Sum(nil)
	for _, part := range strings.FieldsFunc(sig, func(c rune) bool { return c == ' ' || c == ',' }) {
		if strings.HasPrefix(part, "v1,") {
			part = strings.TrimPrefix(part, "v1,")
		}
		if part == "v1" {
			continue
		}
		b, err := base64.StdEncoding.DecodeString(part)
		if err == nil && hmac.Equal(expected, b) {
			return true
		}
	}
	return false
}
func (a *App) voiceOpenAIWebhookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	b, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 65536))
	if e != nil || !voiceOpenAISignatureValid(r, b) {
		http.Error(w, "Firma inválida", 403)
		return
	}
	var ev struct {
		ID   string `json:"id"`
		Type string `json:"type"`
		Data struct {
			SessionID  string `json:"session_id"`
			CallID     string `json:"call_id"`
			SIPHeaders []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"sip_headers"`
		} `json:"data"`
	}
	if json.Unmarshal(b, &ev) != nil {
		http.Error(w, "Evento inválido", 400)
		return
	}
	if ev.Type != "live.transport.incoming" && ev.Type != "realtime.call.incoming" && ev.Type != "live.call.incoming" {
		w.WriteHeader(204)
		return
	}
	call := ""
	for _, h := range ev.Data.SIPHeaders {
		if strings.EqualFold(h.Name, "x-worktic-call") {
			call = h.Value
		}
	}
	if len(call) != 32 {
		w.WriteHeader(204)
		return
	}
	var tid, agent int64
	var channel, stat string
	e = a.db.QueryRow(`SELECT tenant_id,agent_id,channel_id,status FROM voice_calls_v2735 WHERE id=?`, call).Scan(&tid, &agent, &channel, &stat)
	if e != nil || stat != "reserved" || !voiceSIPReady() {
		w.WriteHeader(204)
		return
	}
	ag, e := a.voiceAgent(tid, agent)
	if e != nil {
		http.Error(w, "Agente no disponible", 503)
		return
	}
	_, _, voice, max, e := a.voiceChannel(channel)
	if e != nil {
		http.Error(w, "Canal no disponible", 503)
		return
	}
	sessionID := ev.Data.SessionID
	apiRoute := "live/sessions/" + url.PathEscape(sessionID) + "/accept"
	payload := map[string]any{"session": map[string]any{"type": "live", "model": "gpt-live-1", "instructions": voicePrompt(ag), "audio": map[string]any{"output": map[string]any{"voice": voice}}}}
	if sessionID == "" { // Legacy Realtime webhook is not accepted as GPT-Live to avoid dual processing.
		w.WriteHeader(204)
		return
	}
	// Transition to 'accepting' exactly once, before calling a billable provider.
	res, e := a.db.Exec(`UPDATE voice_calls_v2735 SET status='accepting',openai_id=? WHERE id=? AND status='reserved'`, sessionID, call)
	if e != nil {
		http.Error(w, "Estado no disponible", 503)
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		w.WriteHeader(204)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	_, code, err := voicePostOpenAI(ctx, apiRoute, payload)
	if err != nil || code != 200 {
		_ = a.voiceSettle(call, "failed", 0, "accept_failed")
		http.Error(w, "No fue posible aceptar la llamada", 502)
		return
	}
	_, _ = a.db.Exec(`UPDATE voice_calls_v2735 SET status='active' WHERE id=? AND status='accepting'`, call)
	go a.voiceObserveLive(call, sessionID, max)
	w.WriteHeader(204)
}

// Outbound calling is intentionally off by default. Every number needs a
// consent record with recorded evidence, even if the user ticks a checkbox.
func initVoiceConsentV2735(db *DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS voice_consents_v2735(tenant_id BIGINT NOT NULL,phone TEXT NOT NULL,source TEXT NOT NULL,recorded_at TEXT NOT NULL,revoked INTEGER NOT NULL DEFAULT 0,PRIMARY KEY(tenant_id,phone));`)
	return err
}
func (a *App) voiceConsentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	tid, _, err := a.tenantFor(r)
	if err != nil {
		http.Error(w, "Sesión requerida", 401)
		return
	}
	var q struct {
		Phone   string `json:"phone"`
		Source  string `json:"source"`
		Revoked bool   `json:"revoked"`
	}
	if !voiceReadJSON(w, r, &q) {
		return
	}
	if !voiceNumber(q.Phone) || len(q.Source) > 250 || (!q.Revoked && len(strings.TrimSpace(q.Source)) < 8) {
		http.Error(w, "Número o evidencia insuficiente", 400)
		return
	}
	revoked := 0
	if q.Revoked {
		revoked = 1
	}
	_, err = a.db.Exec(`INSERT INTO voice_consents_v2735(tenant_id,phone,source,recorded_at,revoked) VALUES(?,?,?,?,?) ON CONFLICT(tenant_id,phone) DO UPDATE SET source=excluded.source,recorded_at=excluded.recorded_at,revoked=excluded.revoked`, tid, q.Phone, q.Source, voiceNow(), revoked)
	if err != nil {
		http.Error(w, "No fue posible registrar autorización", 503)
		return
	}
	commercialSetJSON(w, 200, map[string]any{"saved": true, "revoked": q.Revoked})
}
func voiceOutboundHours() bool {
	loc, e := time.LoadLocation("America/Bogota")
	if e != nil {
		return false
	}
	now := time.Now().In(loc)
	return now.Weekday() != time.Saturday && now.Weekday() != time.Sunday && now.Hour() >= 9 && now.Hour() < 17
}
func (a *App) voiceOutboundHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	tid, _, e := a.tenantFor(r)
	if e != nil {
		http.Error(w, "Sesión requerida", 401)
		return
	}
	if !voiceBoolEnv("WORKTIC_VOICE_OUTBOUND_ENABLED") || !voiceConfigured() {
		http.Error(w, "Llamadas salientes no habilitadas", 409)
		return
	}
	if !voiceOutboundHours() {
		http.Error(w, "Horario conservador de contacto: lunes a viernes, de 9:00 a 17:00 Colombia", 409)
		return
	}
	var q struct {
		ChannelID string `json:"channel_id"`
		Phone     string `json:"phone"`
	}
	if !voiceReadJSON(w, r, &q) {
		return
	}
	if !voiceNumber(q.Phone) || !voiceOutboundAllowed(q.Phone) {
		http.Error(w, "Destino no habilitado para este país/prefijo", 400)
		return
	}
	var valid int
	if a.db.QueryRow(`SELECT 1 FROM voice_consents_v2735 WHERE tenant_id=? AND phone=? AND revoked=0`, tid, q.Phone).Scan(&valid) != nil {
		http.Error(w, "El contacto no tiene una autorización verificable o retiró su consentimiento", 403)
		return
	}
	tenant, agent, voice, max, e := a.voiceChannel(q.ChannelID)
	if e != nil || tenant != tid {
		http.Error(w, "Canal no autorizado", 403)
		return
	}
	var caller string
	e = a.db.QueryRow(`SELECT phone FROM voice_channels_v2735 WHERE id=? AND tenant_id=?`, q.ChannelID, tid).Scan(&caller)
	if e != nil || !voiceNumber(caller) {
		http.Error(w, "Este canal necesita un número de origen válido", 409)
		return
	}
	provider := voiceKey("WORKTIC_SIP_PROVIDER_URL")
	username := voiceKey("WORKTIC_SIP_USERNAME")
	password := voiceKey("WORKTIC_SIP_PASSWORD")
	if !strings.HasPrefix(provider, "sips:") || username == "" || password == "" {
		http.Error(w, "Troncal SIP de salida no configurada", 503)
		return
	}
	ag, e := a.voiceAgent(tid, agent)
	if e != nil {
		http.Error(w, "Agente no disponible", 503)
		return
	}
	call, e := a.voiceReserve(tid, q.ChannelID, "outbound", q.Phone, "", agent, max)
	if e != nil {
		http.Error(w, e.Error(), 409)
		return
	}
	payload := map[string]any{"session": map[string]any{"model": "gpt-live-1", "instructions": voicePrompt(ag), "audio": map[string]any{"output": map[string]any{"voice": voice}}}, "transport": map[string]any{"type": "sip", "destination": q.Phone, "trunk": map[string]any{"provider_url": provider, "caller_number": caller, "auth": map[string]any{"type": "digest", "username": username, "password": password}}}}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	response, code, err := voicePostOpenAI(ctx, "live/sessions", payload)
	if err != nil || code != 201 { // On an ambiguous timeout, DO NOT retry: duplicate phone calls are possible.
		if err != nil {
			_, _ = a.db.Exec(`UPDATE voice_calls_v2735 SET status='uncertain',error_code='provider_timeout' WHERE id=?`, call)
		} else {
			_ = a.voiceSettle(call, "failed", 0, "provider_rejected")
		}
		http.Error(w, "El proveedor no confirmó la llamada. No la repitas sin revisar el historial.", 502)
		return
	}
	var ans struct {
		Session struct {
			ID string `json:"id"`
		} `json:"session"`
	}
	if json.Unmarshal(response, &ans) != nil || ans.Session.ID == "" {
		_, _ = a.db.Exec(`UPDATE voice_calls_v2735 SET status='uncertain',error_code='missing_session_id' WHERE id=?`, call)
		http.Error(w, "Respuesta incompleta del proveedor", 502)
		return
	}
	_, _ = a.db.Exec(`UPDATE voice_calls_v2735 SET openai_id=?,status='active' WHERE id=?`, ans.Session.ID, call)
	go a.voiceObserveLive(call, ans.Session.ID, max)
	commercialSetJSON(w, 201, map[string]any{"id": call, "status": "dialing", "max_minutes": max})
}
