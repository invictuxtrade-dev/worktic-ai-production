package main

// V27.21 - billing-safe Creative Credits. This is an additive, opt-in pilot.
// All monetary provider costs remain *estimates*. Card/crypto confirmation is
// NEVER inferred from client-provided transaction hashes.
import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type commercialRule struct {
	Plan        string `json:"plan"`
	Monthly     int64  `json:"monthly_credits"`
	Editor      bool   `json:"editor"`
	Image       bool   `json:"image_ai"`
	Video       bool   `json:"video_ai"`
	Text        bool   `json:"text_ai"`
	Voice       bool   `json:"voice_ai"`
	MaxProjects int64  `json:"max_projects"`
	StorageMB   int64  `json:"storage_mb"`
}
type commercialPack struct {
	Code       string `json:"code"`
	Name       string `json:"name"`
	Credits    int64  `json:"credits"`
	PriceCents int64  `json:"price_cents"`
	Active     bool   `json:"active"`
}

func commercialEnabled() bool { return os.Getenv("WORKTIC_COMMERCIAL_ENABLE") == "1" }
func commercialPlanSeed() []commercialRule {
	return []commercialRule{
		{"free", 0, true, false, false, false, false, 3, 100},
		{"personal", 30, true, true, true, true, true, 30, 1024},
		{"business", 120, true, true, true, true, true, 150, 5120},
		{"enterprise", 300, true, true, true, true, true, 1000, 20480},
	}
}
func commercialPacks() []commercialPack {
	return []commercialPack{
		{"mini", "Mini", 30, 350, true}, {"creator", "Creator", 100, 1000, true},
		{"growth", "Growth", 300, 2700, true}, {"pro", "Pro Studio", 1000, 8000, true},
		{"agency", "Agency", 3000, 21000, true},
	}
}
func commercialBool(x bool) int {
	if x {
		return 1
	}
	return 0
}
func initCommercialV2721Schema(db *DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS commercial_rules_v2721 (plan_code TEXT PRIMARY KEY, monthly_credits INTEGER NOT NULL, editor INTEGER NOT NULL, image_ai INTEGER NOT NULL, video_ai INTEGER NOT NULL, text_ai INTEGER NOT NULL, voice_ai INTEGER NOT NULL, max_projects INTEGER NOT NULL, storage_mb INTEGER NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS commercial_packages_v2721 (code TEXT PRIMARY KEY, name TEXT NOT NULL, credits INTEGER NOT NULL, price_cents INTEGER NOT NULL, active INTEGER NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS commercial_plan_wallet_v2721 (tenant_id INTEGER NOT NULL, period TEXT NOT NULL, plan_code TEXT NOT NULL, allotted INTEGER NOT NULL, reserved INTEGER NOT NULL DEFAULT 0, spent INTEGER NOT NULL DEFAULT 0, PRIMARY KEY(tenant_id,period))`,
		`CREATE TABLE IF NOT EXISTS commercial_bought_wallet_v2721 (tenant_id INTEGER PRIMARY KEY, balance INTEGER NOT NULL DEFAULT 0, reserved INTEGER NOT NULL DEFAULT 0, spent INTEGER NOT NULL DEFAULT 0)`,
		`CREATE TABLE IF NOT EXISTS commercial_holds_v2721 (id TEXT PRIMARY KEY, tenant_id INTEGER NOT NULL, period TEXT NOT NULL, operation TEXT NOT NULL, plan_credits INTEGER NOT NULL DEFAULT 0, bought_credits INTEGER NOT NULL DEFAULT 0, status TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS commercial_holds_tenant_v2721 ON commercial_holds_v2721(tenant_id,period,status)`,
		`CREATE TABLE IF NOT EXISTS commercial_topups_v2721 (id TEXT PRIMARY KEY, tenant_id INTEGER NOT NULL, requested_by INTEGER NOT NULL, package_code TEXT NOT NULL, credits INTEGER NOT NULL, price_cents INTEGER NOT NULL, network TEXT NOT NULL, tx_hash TEXT NOT NULL UNIQUE, status TEXT NOT NULL, reviewed_by INTEGER NOT NULL DEFAULT 0, reviewed_at TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS commercial_topups_tenant_v2721 ON commercial_topups_v2721(tenant_id,created_at)`,
	}
	for _, s := range statements {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("esquema créditos: %w", err)
		}
	}
	if err := initCommercialCoreSchema(db); err != nil {
		return err
	}
	// Enlaza el ciclo asíncrono de Luma con la reserva: imprescindible para
	// reembolsos acreditados por el proveedor.
	if _, err := db.Exec(`ALTER TABLE studio_video_jobs_v2719 ADD COLUMN creative_hold_id TEXT NOT NULL DEFAULT ''`); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column") && !strings.Contains(strings.ToLower(err.Error()), "already exists") {
		return fmt.Errorf("migración Luma hold: %w", err)
	}
	for _, p := range commercialPlanSeed() {
		if _, err := db.Exec(`INSERT INTO commercial_rules_v2721(plan_code,monthly_credits,editor,image_ai,video_ai,text_ai,voice_ai,max_projects,storage_mb) VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(plan_code) DO NOTHING`, p.Plan, p.Monthly, commercialBool(p.Editor), commercialBool(p.Image), commercialBool(p.Video), commercialBool(p.Text), commercialBool(p.Voice), p.MaxProjects, p.StorageMB); err != nil {
			return err
		}
	}
	for _, p := range commercialPacks() {
		if _, err := db.Exec(`INSERT INTO commercial_packages_v2721(code,name,credits,price_cents,active) VALUES(?,?,?,?,?) ON CONFLICT(code) DO NOTHING`, p.Code, p.Name, p.Credits, p.PriceCents, commercialBool(p.Active)); err != nil {
			return err
		}
	}
	return nil
}
func commercialReadRule(db *DB, code string) (commercialRule, error) {
	var r commercialRule
	var e, i, v, t, vo int
	err := db.QueryRow(`SELECT plan_code,monthly_credits,editor,image_ai,video_ai,text_ai,voice_ai,max_projects,storage_mb FROM commercial_rules_v2721 WHERE plan_code=?`, strings.ToLower(code)).Scan(&r.Plan, &r.Monthly, &e, &i, &v, &t, &vo, &r.MaxProjects, &r.StorageMB)
	r.Editor = e == 1
	r.Image = i == 1
	r.Video = v == 1
	r.Text = t == 1
	r.Voice = vo == 1
	return r, err
}
func commercialReadRuleTx(tx *Tx, code string) (commercialRule, error) {
	var r commercialRule
	var e, i, v, t, vo int
	err := tx.QueryRow(`SELECT plan_code,monthly_credits,editor,image_ai,video_ai,text_ai,voice_ai,max_projects,storage_mb FROM commercial_rules_v2721 WHERE plan_code=?`, strings.ToLower(code)).Scan(&r.Plan, &r.Monthly, &e, &i, &v, &t, &vo, &r.MaxProjects, &r.StorageMB)
	r.Editor = e == 1
	r.Image = i == 1
	r.Video = v == 1
	r.Text = t == 1
	r.Voice = vo == 1
	return r, err
}
func commercialCreditsQuote(action string, duration, chars int) (int64, error) {
	switch action {
	case "image":
		return 4, nil
	case "copy", "storyboard":
		return 2, nil
	case "video":
		if duration == 5 {
			return 15, nil
		}
		if duration == 10 {
			return 45, nil
		}
	case "narration":
		if chars >= 1 && chars <= 3500 {
			return int64((chars+999)/1000) * 5, nil
		}
	}
	return 0, errors.New("operación sin precio comercial")
}
func commercialRuleAllows(r commercialRule, action string) bool {
	switch action {
	case "image":
		return r.Image
	case "video":
		return r.Video
	case "copy", "storyboard":
		return r.Text
	case "narration":
		return r.Voice
	}
	return false
}

// Per-credit reserved vendor cost ceiling of 2.5 USD cents: the example
// allocations (30/120/300 at $0.75/$3/$7.50) cannot be exceeded by any mix.
func commercialQuoteWithinBudget(vendorCents, credits int64) bool {
	return credits > 0 && vendorCents > 0 && vendorCents*2 <= credits*5
}
func commercialWalletPeriod() string { return time.Now().UTC().Format("2006-01") }
func commercialWalletEnsureTx(tx *Tx, tid int64, period string, r commercialRule) error {
	_, err := tx.Exec(`INSERT INTO commercial_plan_wallet_v2721(tenant_id,period,plan_code,allotted,reserved,spent) VALUES(?,?,?,?,0,0) ON CONFLICT(tenant_id,period) DO UPDATE SET plan_code=excluded.plan_code,allotted=excluded.allotted`, tid, period, r.Plan, r.Monthly)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO commercial_bought_wallet_v2721(tenant_id,balance,reserved,spent) VALUES(?,0,0,0) ON CONFLICT(tenant_id) DO NOTHING`, tid)
	return err
}

// Called INSIDE V27.20's provider budget transaction. Both vendor reserve
// and customer-credit reserve commit together, or neither commits.
func (a *App) commercialReserveCreditsTx(tx *Tx, hold *creativeHold, plan, operation string, duration, chars int) error {
	if !commercialEnabled() {
		return nil
	}
	rule, err := a.contractStudioRuleTxV2729(tx, hold.TenantID, plan)
	if err != nil {
		return errors.New("plan creativo no configurado")
	}
	if !commercialRuleAllows(rule, operation) {
		return errors.New("función no incluida en el plan")
	}
	cost, err := commercialCreditsQuote(operation, duration, chars)
	if err != nil {
		return err
	}
	if !commercialQuoteWithinBudget(hold.Cents, cost) {
		return errors.New("tarifa de proveedor excede presupuesto por crédito; solicitar ajuste administrativo")
	}
	if err = commercialWalletEnsureTx(tx, hold.TenantID, hold.Period, rule); err != nil {
		return err
	}
	var allotted, used, reserved int64
	err = tx.QueryRow(`SELECT allotted,spent,reserved FROM commercial_plan_wallet_v2721 WHERE tenant_id=? AND period=?`, hold.TenantID, hold.Period).Scan(&allotted, &used, &reserved)
	if err != nil {
		return err
	}
	planAvail := allotted - used - reserved
	if planAvail < 0 {
		planAvail = 0
	}
	fromPlan := cost
	if fromPlan > planAvail {
		fromPlan = planAvail
	}
	fromPurchased := cost - fromPlan
	if fromPlan > 0 {
		res, e := tx.Exec(`UPDATE commercial_plan_wallet_v2721 SET reserved=reserved+? WHERE tenant_id=? AND period=? AND allotted-spent-reserved>=?`, fromPlan, hold.TenantID, hold.Period, fromPlan)
		if e != nil {
			return e
		}
		if n, _ := res.RowsAffected(); n != 1 {
			return errCreativeBudget
		}
	}
	if fromPurchased > 0 {
		res, e := tx.Exec(`UPDATE commercial_bought_wallet_v2721 SET reserved=reserved+? WHERE tenant_id=? AND balance-reserved>=?`, fromPurchased, hold.TenantID, fromPurchased)
		if e != nil {
			return e
		}
		if n, _ := res.RowsAffected(); n != 1 {
			return errors.New("créditos creativos insuficientes")
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = tx.Exec(`INSERT INTO commercial_holds_v2721(id,tenant_id,period,operation,plan_credits,bought_credits,status,created_at,updated_at) VALUES(?,?,?,?,?,?,'held',?,?)`, hold.ID, hold.TenantID, hold.Period, operation, fromPlan, fromPurchased, now, now)
	return err
}
func (a *App) commercialSettleCreditsTx(tx *Tx, hold *creativeHold, settle bool) error {
	if !commercialEnabled() {
		return nil
	}
	var fromPlan, fromBought int64
	err := tx.QueryRow(`SELECT plan_credits,bought_credits FROM commercial_holds_v2721 WHERE id=? AND tenant_id=? AND status='held'`, hold.ID, hold.TenantID).Scan(&fromPlan, &fromBought)
	if err != nil {
		return err
	}
	spentPlan, spentBought := int64(0), int64(0)
	status := "released"
	if settle {
		status = "spent"
		spentPlan = fromPlan
		spentBought = fromBought
	}
	if fromPlan > 0 {
		res, e := tx.Exec(`UPDATE commercial_plan_wallet_v2721 SET reserved=reserved-?,spent=spent+? WHERE tenant_id=? AND period=? AND reserved>=?`, fromPlan, spentPlan, hold.TenantID, hold.Period, fromPlan)
		if e != nil {
			return e
		}
		if n, _ := res.RowsAffected(); n != 1 {
			return errors.New("reserva del plan no encontrada")
		}
	}
	if fromBought > 0 {
		res, e := tx.Exec(`UPDATE commercial_bought_wallet_v2721 SET reserved=reserved-?,balance=balance-?,spent=spent+? WHERE tenant_id=? AND reserved>=?`, fromBought, spentBought, spentBought, hold.TenantID, fromBought)
		if e != nil {
			return e
		}
		if n, _ := res.RowsAffected(); n != 1 {
			return errors.New("reserva comprada no encontrada")
		}
	}
	res, err := tx.Exec(`UPDATE commercial_holds_v2721 SET status=?,updated_at=? WHERE id=? AND status='held'`, status, time.Now().UTC().Format(time.RFC3339Nano), hold.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return errors.New("reserva de créditos ya liquidada")
	}
	return nil
}
func (a *App) commercialWalletData(tid int64, plan string) (map[string]any, error) {
	r, err := a.contractStudioRuleV2729(tid, plan)
	if err != nil {
		return nil, err
	}
	period := commercialWalletPeriod()
	// Setup on first read. Transaction ensures lazy month allocation remains idempotent.
	tx, e := a.db.Begin()
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	if err = commercialWalletEnsureTx(tx, tid, period, r); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	var a1, s1, r1, b1, breserved, bspent int64
	err = a.db.QueryRow(`SELECT allotted,spent,reserved FROM commercial_plan_wallet_v2721 WHERE tenant_id=? AND period=?`, tid, period).Scan(&a1, &s1, &r1)
	if err != nil {
		return nil, err
	}
	err = a.db.QueryRow(`SELECT balance,reserved,spent FROM commercial_bought_wallet_v2721 WHERE tenant_id=?`, tid).Scan(&b1, &breserved, &bspent)
	if err != nil {
		return nil, err
	}
	available := a1 - s1 - r1
	if available < 0 {
		available = 0
	}
	boughtAvailable := b1 - breserved
	if boughtAvailable < 0 {
		boughtAvailable = 0
	}
	coreCap := commercialCoreCap(plan)
	var currentCap int64
	if contractV2729Enabled() && plan != "free" {
		contract, e := a.contractForTenantV2729(tid)
		if e != nil {
			return nil, errors.New("contrato requerido para calcular cupo de IA")
		}
		currentCap = int64(contract.Plan.MaxAIResponses)
		coreCap = commercialCoreCapForPlan(plan, currentCap)
	} else if e := a.db.QueryRow(`SELECT max_ai_responses FROM billing_plans WHERE code=?`, plan).Scan(&currentCap); e == nil {
		coreCap = commercialCoreCapForPlan(plan, currentCap)
	}
	var coreHeld, coreUsed int64
	_ = a.db.QueryRow(`SELECT held,used FROM commercial_core_usage_v2721 WHERE tenant_id=? AND period=?`, tid, period).Scan(&coreHeld, &coreUsed)
	// Read side must show the same effective consumed amount as the atomic
	// reservation writer (V27.27); otherwise the client sees an inflated saldo
	// after activating the pilot mid-month.
	var legacyUsed int64
	if e := a.db.QueryRow(`SELECT COALESCE(MAX(ai_responses),0) FROM usage_monthly
 WHERE user_id=(SELECT owner_user_id FROM tenants WHERE id=?) AND period=?`, tid, period).Scan(&legacyUsed); e != nil {
		return nil, errors.New("no se pudo validar el contador mensual de IA")
	}
	if legacyUsed > coreUsed {
		coreUsed = legacyUsed
	}
	coreAvailable := coreCap - coreHeld - coreUsed
	if coreAvailable < 0 {
		coreAvailable = 0
	}
	return map[string]any{"core_ai_monthly_limit": coreCap, "core_ai_used": coreUsed, "core_ai_reserved": coreHeld, "core_ai_available": coreAvailable, "period": period, "plan": plan, "monthly_grant": a1, "plan_spent": s1, "plan_reserved": r1, "plan_available": available, "purchased_balance": b1, "purchased_reserved": breserved, "purchased_spent": bspent, "purchased_available": boughtAvailable, "available_credits": available + boughtAvailable, "rule": r, "enabled": commercialEnabled(), "cost_guards_required": true}, nil
}
func commercialSetJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func (a *App) commercialCatalogV2721Handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", 405)
		return
	}
	rows, err := a.db.Query(`SELECT p.code,p.name,p.price_usdt,p.max_users,p.max_channels,p.max_contacts,p.max_ai_responses,p.max_products,p.max_rules,r.monthly_credits,r.editor,r.image_ai,r.video_ai,r.text_ai,r.voice_ai,r.max_projects,r.storage_mb FROM billing_plans p JOIN commercial_rules_v2721 r ON r.plan_code=p.code WHERE p.active=1 ORDER BY p.price_usdt,p.id`)
	if err != nil {
		http.Error(w, "Catálogo no disponible", 503)
		return
	}
	defer rows.Close()
	plans := []map[string]any{}
	for rows.Next() {
		var code, name string
		var price float64
		var us, ch, contacts, ai, products, rules, monthly, editor, img, vid, txt, voice, proj, storage int64
		if rows.Scan(&code, &name, &price, &us, &ch, &contacts, &ai, &products, &rules, &monthly, &editor, &img, &vid, &txt, &voice, &proj, &storage) == nil {
			plans = append(plans, map[string]any{"code": code, "name": name, "price_usdt": price, "max_users": us, "max_channels": ch, "max_contacts": contacts, "max_ai_responses": ai, "max_products": products, "max_rules": rules, "monthly_credits": monthly, "studio_editor": editor == 1, "studio_image": img == 1, "studio_video": vid == 1, "studio_text": txt == 1, "studio_voice": voice == 1, "max_projects": proj, "storage_mb": storage})
		}
	}
	if rows.Err() != nil {
		http.Error(w, "Error catálogo", 503)
		return
	}
	packs := []commercialPack{}
	pr, err := a.db.Query(`SELECT code,name,credits,price_cents,active FROM commercial_packages_v2721 WHERE active=1 ORDER BY price_cents`)
	if err != nil {
		http.Error(w, "Paquetes no disponibles", 503)
		return
	}
	for pr.Next() {
		var p commercialPack
		var act int
		if pr.Scan(&p.Code, &p.Name, &p.Credits, &p.PriceCents, &act) == nil {
			p.Active = act == 1
			packs = append(packs, p)
		}
	}
	err = pr.Err()
	pr.Close()
	if err != nil {
		http.Error(w, "Error paquetes", 503)
		return
	}
	commercialSetJSON(w, 200, map[string]any{"plans": plans, "packages": packs, "currency": "USD/USDT", "sales_enabled": commercialEnabled(), "provider_estimates_only": true, "monthly_credits_reset_utc": true, "discounts_and_payments_require_verification": true})
}
func (a *App) commercialWalletV2721Handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", 405)
		return
	}
	tid, u, err := a.tenantFor(r)
	if err != nil {
		http.Error(w, "Inicia sesión", 401)
		return
	}
	p, _, err := a.activePlan(a.billingAccountUserID(u))
	if err != nil {
		http.Error(w, "Plan no disponible", 503)
		return
	}
	data, err := a.commercialWalletData(tid, p.Code)
	if err != nil {
		http.Error(w, "Saldo no disponible", 503)
		return
	}
	commercialSetJSON(w, 200, data)
}
func (a *App) commercialQuoteV2721Handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", 405)
		return
	}
	_, u, err := a.tenantFor(r)
	if err != nil {
		http.Error(w, "Inicia sesión", 401)
		return
	}
	p, _, err := a.activePlan(a.billingAccountUserID(u))
	if err != nil {
		http.Error(w, "Plan no disponible", 503)
		return
	}
	rule, err := a.contractStudioRuleV2729(u.TenantID, p.Code)
	if err != nil {
		http.Error(w, "Plan no configurado", 503)
		return
	}
	action := r.URL.Query().Get("action")
	duration, _ := strconv.Atoi(r.URL.Query().Get("duration"))
	chars, _ := strconv.Atoi(r.URL.Query().Get("chars"))
	credits, err := commercialCreditsQuote(action, duration, chars)
	if err != nil {
		http.Error(w, "Operación inválida", 400)
		return
	}
	cents := creativeQuote(action, duration, chars)
	allowed := commercialRuleAllows(rule, action) && commercialQuoteWithinBudget(cents, credits)
	commercialSetJSON(w, 200, map[string]any{"action": action, "credits": credits, "estimated_vendor_cost_cents": cents, "enabled": commercialEnabled(), "allowed_by_plan_and_cost": allowed, "not_an_invoice": true})
}
func commercialTxHash(network, hash string) bool {
	if network == "BEP20" {
		ok, _ := regexp.MatchString(`^0x[0-9a-fA-F]{64}$`, hash)
		return ok
	}
	if network == "TRC20" {
		ok, _ := regexp.MatchString(`^[0-9a-fA-F]{64}$`, hash)
		return ok
	}
	return false
}
func (a *App) commercialTopupV2721Handler(w http.ResponseWriter, r *http.Request) {
	tid, u, err := a.tenantFor(r)
	if err != nil {
		http.Error(w, "Inicia sesión", 401)
		return
	}
	p, _, err := a.activePlan(a.billingAccountUserID(u))
	if err != nil {
		http.Error(w, "Plan no disponible", 503)
		return
	}
	if p.Code == "free" {
		http.Error(w, "Se requiere plan de pago activo para comprar créditos", 403)
		return
	}
	if r.Method == http.MethodGet {
		rows, err := a.db.Query(`SELECT id,package_code,credits,price_cents,network,tx_hash,status,created_at FROM commercial_topups_v2721 WHERE tenant_id=? ORDER BY created_at DESC LIMIT 100`, tid)
		if err != nil {
			http.Error(w, "Error consultando recargas", 503)
			return
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var id, code, network, hash, status, created string
			var credits, price int64
			if rows.Scan(&id, &code, &credits, &price, &network, &hash, &status, &created) == nil {
				out = append(out, map[string]any{"id": id, "package": code, "credits": credits, "price_cents": price, "network": network, "tx_hash": hash, "status": status, "created_at": created})
			}
		}
		commercialSetJSON(w, 200, map[string]any{"topups": out})
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	if !commercialEnabled() {
		http.Error(w, "Las recargas todavía no están habilitadas al público", 503)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var q struct {
		Package string `json:"package"`
		Network string `json:"network"`
		TxHash  string `json:"tx_hash"`
	}
	if json.NewDecoder(r.Body).Decode(&q) != nil {
		http.Error(w, "Solicitud inválida", 400)
		return
	}
	q.Package = strings.ToLower(strings.TrimSpace(q.Package))
	q.Network = strings.ToUpper(strings.TrimSpace(q.Network))
	q.TxHash = strings.TrimSpace(q.TxHash)
	if !commercialTxHash(q.Network, q.TxHash) {
		http.Error(w, "Hash o red inválidos", 400)
		return
	}
	if q.Network == "BEP20" && a.cfg.USDTBEP20Address == "" || q.Network == "TRC20" && a.cfg.USDTTRC20Address == "" {
		http.Error(w, "Billetera de pago no configurada", 503)
		return
	}
	var credits, price int64
	if a.db.QueryRow(`SELECT credits,price_cents FROM commercial_packages_v2721 WHERE code=? AND active=1`, q.Package).Scan(&credits, &price) != nil {
		http.Error(w, "Paquete no disponible", 400)
		return
	}
	var exists int
	if a.db.QueryRow(`SELECT COUNT(*) FROM billing_payments WHERE LOWER(tx_hash)=LOWER(?)`, q.TxHash).Scan(&exists) != nil || exists != 0 {
		http.Error(w, "Transacción ya usada en una suscripción", 409)
		return
	}
	id, err := creativeID()
	if err != nil {
		http.Error(w, "No se pudo crear la solicitud", 500)
		return
	}
	if billingLifecycleV2728Enabled() {
		tx, e := a.db.Begin()
		if e != nil {
			http.Error(w, "Transacción no disponible", 503)
			return
		}
		defer tx.Rollback()
		if e = billingClaimTxV2728(tx, q.Network, q.TxHash, "studio_topup", id, tid); e != nil {
			http.Error(w, e.Error(), 409)
			return
		}
		_, e = tx.Exec(`INSERT INTO commercial_topups_v2721(id,tenant_id,requested_by,package_code,credits,price_cents,network,tx_hash,status,created_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, id, tid, u.ID, q.Package, credits, price, q.Network, q.TxHash, "pending", time.Now().UTC().Format(time.RFC3339))
		if e != nil {
			http.Error(w, "La recarga ya existe", 409)
			return
		}
		if e = tx.Commit(); e != nil {
			http.Error(w, "No se pudo confirmar la solicitud", 503)
			return
		}
	} else {
		_, err = a.db.Exec(`INSERT INTO commercial_topups_v2721(id,tenant_id,requested_by,package_code,credits,price_cents,network,tx_hash,status,created_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, id, tid, u.ID, q.Package, credits, price, q.Network, q.TxHash, "pending", time.Now().UTC().Format(time.RFC3339))
		if err != nil {
			http.Error(w, "La transacción ya fue presentada", 409)
			return
		}
	}
	commercialSetJSON(w, 201, map[string]any{"ok": true, "id": id, "status": "pending", "credited": false, "verification": "manual; el administrador debe comprobar el abono real en blockchain"})
}
func (a *App) commercialAdminV2721Handler(w http.ResponseWriter, r *http.Request) {
	admin, ok := a.requireSuperadmin(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		rows, err := a.db.Query(`SELECT id,tenant_id,requested_by,package_code,credits,price_cents,network,tx_hash,status,reviewed_by,reviewed_at,created_at FROM commercial_topups_v2721 ORDER BY CASE WHEN status='pending' THEN 0 ELSE 1 END,created_at DESC LIMIT 300`)
		if err != nil {
			http.Error(w, "Error consultando solicitudes", 503)
			return
		}
		defer rows.Close()
		result := []map[string]any{}
		for rows.Next() {
			var id, pc, network, hash, status, reviewed, created string
			var tenant, uid, credits, price, reviewer int64
			if rows.Scan(&id, &tenant, &uid, &pc, &credits, &price, &network, &hash, &status, &reviewer, &reviewed, &created) == nil {
				result = append(result, map[string]any{"id": id, "tenant_id": tenant, "user_id": uid, "package": pc, "credits": credits, "price_cents": price, "network": network, "tx_hash": hash, "status": status, "reviewed_by": reviewer, "reviewed_at": reviewed, "created_at": created})
			}
		}
		commercialSetJSON(w, 200, map[string]any{"topups": result, "manual_blockchain_verification_required": true})
	case http.MethodPost:
		if !commercialEnabled() {
			http.Error(w, "Activación comercial deshabilitada", 503)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		var q struct {
			ID        string `json:"id"`
			Action    string `json:"action"`
			Confirmed bool   `json:"verified_on_chain"`
		}
		if json.NewDecoder(r.Body).Decode(&q) != nil {
			http.Error(w, "Solicitud inválida", 400)
			return
		}
		if q.Action != "approve" && q.Action != "reject" && q.Action != "refund" {
			http.Error(w, "Acción inválida", 400)
			return
		}
		if q.Action == "approve" && !q.Confirmed {
			http.Error(w, "Confirma que verificaste el importe, token USDT, red y destinatario en blockchain", 400)
			return
		}
		tx, err := a.db.Begin()
		if err != nil {
			http.Error(w, "Error al iniciar la revisión", 503)
			return
		}
		defer tx.Rollback()
		var tenant, credits int64
		var status string
		if err = tx.QueryRow(`SELECT tenant_id,credits,status FROM commercial_topups_v2721 WHERE id=?`, q.ID).Scan(&tenant, &credits, &status); err != nil {
			http.Error(w, "Solicitud no encontrada", 404)
			return
		}
		expected, newStatus := "pending", "rejected"
		delta := int64(0)
		switch q.Action {
		case "approve":
			newStatus = "approved"
			delta = credits
		case "refund":
			expected = "approved"
			newStatus = "refunded"
			delta = -credits
		}
		if status != expected {
			http.Error(w, "Solicitud ya procesada o estado incompatible", 409)
			return
		}
		res, err := tx.Exec(`UPDATE commercial_topups_v2721 SET status=?,reviewed_by=?,reviewed_at=? WHERE id=? AND status=?`, newStatus, admin.ID, time.Now().UTC().Format(time.RFC3339), q.ID, expected)
		if err != nil {
			http.Error(w, "No se pudo actualizar", 503)
			return
		}
		if n, _ := res.RowsAffected(); n != 1 {
			http.Error(w, "Revisión concurrente, inténtalo de nuevo", 409)
			return
		}
		if delta != 0 {
			if _, err = tx.Exec(`INSERT INTO commercial_bought_wallet_v2721(tenant_id,balance,reserved,spent) VALUES(?,0,0,0) ON CONFLICT(tenant_id) DO NOTHING`, tenant); err != nil {
				http.Error(w, "Error billetera", 503)
				return
			}
			// V27.25: no se permite dejar la billetera negativa al revertir
			// créditos ya consumidos o reservados. Requiere conciliación manual.
			res, e := tx.Exec(`UPDATE commercial_bought_wallet_v2721 SET balance=balance+? WHERE tenant_id=? AND balance+?>=reserved`, delta, tenant, delta)
			if e != nil {
				http.Error(w, "Error actualizando billetera", 503)
				return
			}
			if n, _ := res.RowsAffected(); n != 1 {
				http.Error(w, "No es posible revertir créditos consumidos o reservados; realizar conciliación manual", 409)
				return
			}
		}
		if err = tx.Commit(); err != nil {
			http.Error(w, "No se pudo confirmar revisión", 503)
			return
		}
		commercialSetJSON(w, 200, map[string]any{"ok": true, "status": newStatus, "balance_change": delta, "caution": "La verificación bancaria/blockchain la realiza el administrador; no es automática."})
	default:
		http.Error(w, "Método no permitido", 405)
	}
}
func (a *App) commercialRulesAdminV2721Handler(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireSuperadmin(w, r); !ok {
		return
	}
	if r.Method == http.MethodGet {
		plans := []commercialRule{}
		for _, s := range []string{"free", "personal", "business", "enterprise"} {
			if p, err := commercialReadRule(a.db, s); err == nil {
				plans = append(plans, p)
			}
		}
		commercialSetJSON(w, 200, map[string]any{"rules": plans})
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	var rule commercialRule
	if json.NewDecoder(r.Body).Decode(&rule) != nil {
		http.Error(w, "JSON inválido", 400)
		return
	}
	var exists int
	if a.db.QueryRow(`SELECT COUNT(*) FROM billing_plans WHERE code=?`, rule.Plan).Scan(&exists) != nil || exists != 1 {
		http.Error(w, "Plan desconocido", 400)
		return
	}
	if rule.Monthly < 0 || rule.Monthly > 100000 || rule.MaxProjects < 0 || rule.MaxProjects > 100000 || rule.StorageMB < 0 || rule.StorageMB > 10000000 {
		http.Error(w, "Límites inválidos", 400)
		return
	}
	if rule.Plan == "free" && (rule.Monthly != 0 || rule.Image || rule.Video || rule.Voice || rule.Text) {
		http.Error(w, "Free no debe consumir API externas sin presupuesto", 400)
		return
	}
	_, err := a.db.Exec(`UPDATE commercial_rules_v2721 SET monthly_credits=?,editor=?,image_ai=?,video_ai=?,text_ai=?,voice_ai=?,max_projects=?,storage_mb=? WHERE plan_code=?`, rule.Monthly, commercialBool(rule.Editor), commercialBool(rule.Image), commercialBool(rule.Video), commercialBool(rule.Text), commercialBool(rule.Voice), rule.MaxProjects, rule.StorageMB, rule.Plan)
	if err != nil {
		http.Error(w, "No se pudo guardar reglas", 500)
		return
	}
	commercialSetJSON(w, 200, map[string]any{"ok": true, "rule": rule, "note": "Los cambios de límites comerciales son efectivos en nuevos pedidos; revisar impactos de suscripciones existentes."})
}

// Purchased credits are prepaid; reserve a matching additional vendor budget,
// otherwise a buyer could own credits that the tenant guard always blocks.
// Base global cap remains an administrative emergency ceiling for included-plan
// budgets. The purchased component is funded by verified paid credits only.
func commercialPurchasedFundingCents(db *DB, tenantID int64) int64 {
	if !commercialEnabled() {
		return 0
	}
	// Only unused purchased credits plus purchased credits spent IN THIS MONTH
	// can extend this month's provider budget. Historical spending must not
	// permanently increase future monthly budgets.
	var unused, spentThisMonth int64
	var err error
	if tenantID == 0 {
		err = db.QueryRow(`SELECT COALESCE(SUM(CASE WHEN balance>0 THEN balance ELSE 0 END),0) FROM commercial_bought_wallet_v2721`).Scan(&unused)
		if err == nil {
			err = db.QueryRow(`SELECT COALESCE(SUM(bought_credits),0) FROM commercial_holds_v2721 WHERE period=? AND status='spent'`, commercialWalletPeriod()).Scan(&spentThisMonth)
		}
	} else {
		err = db.QueryRow(`SELECT CASE WHEN balance>0 THEN balance ELSE 0 END FROM commercial_bought_wallet_v2721 WHERE tenant_id=?`, tenantID).Scan(&unused)
		if err == sql.ErrNoRows {
			err = nil
			unused = 0
		}
		if err == nil {
			err = db.QueryRow(`SELECT COALESCE(SUM(bought_credits),0) FROM commercial_holds_v2721 WHERE tenant_id=? AND period=? AND status='spent'`, tenantID, commercialWalletPeriod()).Scan(&spentThisMonth)
		}
	}
	if err != nil || unused < 0 || spentThisMonth < 0 || unused+spentThisMonth > 100000000 {
		return 0
	}
	return ((unused+spentThisMonth)*5 + 1) / 2
}

// Refund only when Luma reports a documented, definitive refundable failure.
// This is idempotent and adjusts both credit buckets and provider estimations.
func (a *App) creativeRefundByHoldID(tenantID int64, id string) error {
	if id == "" || tenantID <= 0 {
		return errors.New("reserva no identificada")
	}
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var period string
	var cents int64
	if err = tx.QueryRow(`SELECT period,estimated_cents FROM creative_cost_ledger_v2720 WHERE id=? AND tenant_id=? AND status='estimated'`, id, tenantID).Scan(&period, &cents); err != nil {
		return err
	}
	res, err := tx.Exec(`UPDATE creative_cost_ledger_v2720 SET status='refunded',updated_at=? WHERE id=? AND tenant_id=? AND status='estimated'`, time.Now().UTC().Format(time.RFC3339Nano), id, tenantID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return errors.New("reserva ya reembolsada")
	}
	for _, tid := range []int64{0, tenantID} {
		res, e := tx.Exec(`UPDATE creative_cost_budgets_v2720 SET spent_cents=spent_cents-? WHERE tenant_id=? AND period=? AND spent_cents>=?`, cents, tid, period, cents)
		if e != nil {
			return e
		}
		if n, _ := res.RowsAffected(); n != 1 {
			return errors.New("reembolso de costo incoherente")
		}
	}
	// A hold without a commercial record was made during the pilot with
	// credit billing disabled: refund only the provider estimate.
	var planCredits, boughtCredits int64
	e := tx.QueryRow(`SELECT plan_credits,bought_credits FROM commercial_holds_v2721 WHERE id=? AND tenant_id=? AND status='spent'`, id, tenantID).Scan(&planCredits, &boughtCredits)
	if e == nil {
		if planCredits > 0 {
			res, e := tx.Exec(`UPDATE commercial_plan_wallet_v2721 SET spent=spent-? WHERE tenant_id=? AND period=? AND spent>=?`, planCredits, tenantID, period, planCredits)
			if e != nil {
				return e
			}
			if n, _ := res.RowsAffected(); n != 1 {
				return errors.New("reembolso plan incoherente")
			}
		}
		if boughtCredits > 0 {
			res, e := tx.Exec(`UPDATE commercial_bought_wallet_v2721 SET balance=balance+?,spent=spent-? WHERE tenant_id=? AND spent>=?`, boughtCredits, boughtCredits, tenantID, boughtCredits)
			if e != nil {
				return e
			}
			if n, _ := res.RowsAffected(); n != 1 {
				return errors.New("reembolso comprado incoherente")
			}
		}
		if _, e := tx.Exec(`UPDATE commercial_holds_v2721 SET status='refunded',updated_at=? WHERE id=? AND tenant_id=? AND status='spent'`, time.Now().UTC().Format(time.RFC3339Nano), id, tenantID); e != nil {
			return e
		}
	} else if e != sql.ErrNoRows {
		return e
	}
	return tx.Commit()
}
func commercialExampleFinancial(planPrice int64, discount, partnerPct, credits, centsPerCredit int64) (int64, error) {
	if planPrice < 0 || discount < 0 || discount > 100 || partnerPct < 0 || partnerPct > 50 || credits < 0 || centsPerCredit < 0 {
		return 0, errors.New("suposición financiera inválida")
	}
	revenue := planPrice * (100 - discount) / 100
	partner := revenue * partnerPct / 100
	return revenue - partner - credits*centsPerCredit, nil
}

// Serializes all Worktic Studio/Social Hub file writes by tenant, then checks
// the single shared Galería Multimedia quota. PostgreSQL uses an advisory xact
// lock and SQLite uses its single connection transaction. The file writer must
// commit the asset row using this returned transaction before releasing it.
func commercialLockTenantTx(tx *Tx, tid int64) error {
	if tid <= 0 {
		return errors.New("empresa inválida")
	}
	if tx.dialect == "postgres" || tx.dialect == "postgresql" {
		var locked int64
		if err := tx.QueryRow(`SELECT 1 FROM (SELECT pg_advisory_xact_lock(?)) AS worktic_lock`, int64(2721000000)+tid).Scan(&locked); err != nil {
			return err
		}
	}
	return nil
}
func (a *App) commercialAssetWriteTx(tid, added int64) (*Tx, error) {
	if tid <= 0 || added <= 0 {
		return nil, errors.New("archivo inválido")
	}
	var ownerID int64
	if err := a.db.QueryRow(`SELECT owner_user_id FROM tenants WHERE id=? AND status='active'`, tid).Scan(&ownerID); err != nil {
		return nil, errors.New("empresa no disponible")
	}
	plan, _, err := a.activePlan(ownerID)
	if err != nil {
		return nil, err
	}
	rule, err := a.contractStudioRuleV2729(tid, plan.Code)
	if err != nil {
		return nil, err
	}
	if !rule.Editor || rule.StorageMB <= 0 {
		return nil, errors.New("almacenamiento no incluido en tu plan")
	}
	tx, err := a.db.Begin()
	if err != nil {
		return nil, err
	}
	if err = commercialLockTenantTx(tx, tid); err != nil {
		tx.Rollback()
		return nil, err
	}
	var used int64
	if err = tx.QueryRow(`SELECT COALESCE(SUM(size_bytes),0) FROM social_media_assets WHERE tenant_id=? AND url LIKE ?`, tid, "/uploads/social/"+strconv.FormatInt(tid, 10)+"/%").Scan(&used); err != nil {
		tx.Rollback()
		return nil, err
	}
	// Strictly smaller than limit when new bytes added. No conversion to float.
	capBytes := rule.StorageMB * 1024 * 1024
	if added > capBytes || used > capBytes-added {
		tx.Rollback()
		return nil, fmt.Errorf("almacenamiento del plan agotado (%d MB)", rule.StorageMB)
	}
	return tx, nil
}
