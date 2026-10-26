package main

// V27.20: cost-reservation guard for paid Worktic Studio API calls.
// Accounting is *estimated provider cost* in USD cents, NOT a customer
// credit balance, charge, receipt, or verified provider invoice.
import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

var errCreativeBudget = errors.New("presupuesto de generación agotado o todavía no configurado")

type creativeHold struct {
	ID        string
	TenantID  int64
	Cents     int64
	Period    string
	Operation string
}

func creativeEnvCents(name string) int64 {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return 0
	} // explicitly opt in; default denies paid vendor calls
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n < 0 || n > 100000000 {
		return 0
	}
	return n
}
func creativeTenantLimit(plan string) int64 {
	switch strings.ToLower(strings.TrimSpace(plan)) {
	case "personal":
		return creativeEnvCents("WORKTIC_COST_BUDGET_PERSONAL_CENTS")
	case "business", "negocio":
		return creativeEnvCents("WORKTIC_COST_BUDGET_NEGOCIO_CENTS")
	case "enterprise", "empresa":
		return creativeEnvCents("WORKTIC_COST_BUDGET_EMPRESA_CENTS")
	default:
		return 0
	}
}
func creativeQuote(action string, duration, chars int) int64 {
	switch action {
	case "video":
		if duration == 10 {
			return creativePositiveQuote("WORKTIC_COST_VIDEO10_CENTS", 99)
		}
		if duration == 5 {
			return creativePositiveQuote("WORKTIC_COST_VIDEO5_CENTS", 36)
		}
	case "image":
		return creativePositiveQuote("WORKTIC_COST_IMAGE_CENTS", 8)
	case "storyboard", "copy":
		return creativePositiveQuote("WORKTIC_COST_TEXT_CENTS", 5)
	case "narration":
		if chars < 1 || chars > 3500 {
			return 0
		}
		n := (int64(chars) + 999) / 1000
		return n * creativePositiveQuote("WORKTIC_COST_TTS_1000_CENTS", 10)
	}
	return 0
}
func creativePositiveQuote(name string, fallback int64) int64 {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n < 1 || n > 100000 {
		return fallback
	}
	return n
}
func creativeMonth(t time.Time) string { return t.UTC().Format("2006-01") }

func initCreativeCostV2720Schema(db *DB) error {
	for _, stmt := range []string{
		`CREATE TABLE IF NOT EXISTS creative_cost_budgets_v2720 (tenant_id INTEGER NOT NULL, period TEXT NOT NULL, limit_cents INTEGER NOT NULL DEFAULT 0, reserved_cents INTEGER NOT NULL DEFAULT 0, spent_cents INTEGER NOT NULL DEFAULT 0, PRIMARY KEY(tenant_id,period))`,
		`CREATE TABLE IF NOT EXISTS creative_cost_ledger_v2720 (id TEXT PRIMARY KEY, tenant_id INTEGER NOT NULL, user_id INTEGER NOT NULL, period TEXT NOT NULL, operation TEXT NOT NULL, provider TEXT NOT NULL, estimated_cents INTEGER NOT NULL, status TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS creative_cost_ledger_tenant_v2720 ON creative_cost_ledger_v2720(tenant_id,period,status)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}
func creativeID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Reserve against GLOBAL and TENANT budgets within one SQL transaction.
// The conditional UPDATE is atomic even for simultaneous HTTP requests / Render workers.
func (a *App) creativeReserve(tenantID, userID int64, plan, operation, provider string, cents int64, duration, chars int) (*creativeHold, error) {
	if tenantID <= 0 || userID <= 0 || cents <= 0 {
		return nil, errors.New("reserva inválida")
	}
	globalLimit := creativeEnvCents("WORKTIC_COST_GLOBAL_MONTHLY_CENTS")
	tenantLimit := creativeTenantLimit(plan)
	if commercialEnabled() && globalLimit > 0 && tenantLimit > 0 {
		globalLimit += commercialPurchasedFundingCents(a.db, 0)
		tenantLimit += commercialPurchasedFundingCents(a.db, tenantID)
	}
	if globalLimit == 0 || tenantLimit == 0 {
		return nil, errCreativeBudget
	}
	period := creativeMonth(time.Now())
	id, err := creativeID()
	if err != nil {
		return nil, err
	}
	tx, err := a.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	for _, b := range []struct{ tenant, limit int64 }{{0, globalLimit}, {tenantID, tenantLimit}} {
		_, err = tx.Exec(`INSERT INTO creative_cost_budgets_v2720(tenant_id,period,limit_cents,reserved_cents,spent_cents) VALUES(?,?,?,0,0) ON CONFLICT(tenant_id,period) DO UPDATE SET limit_cents=excluded.limit_cents`, b.tenant, period, b.limit)
		if err != nil {
			return nil, err
		}
		res, e := tx.Exec(`UPDATE creative_cost_budgets_v2720 SET reserved_cents=reserved_cents+? WHERE tenant_id=? AND period=? AND spent_cents+reserved_cents+?<=limit_cents`, cents, b.tenant, period, cents)
		if e != nil {
			return nil, e
		}
		count, e := res.RowsAffected()
		if e != nil {
			return nil, e
		}
		if count != 1 {
			return nil, errCreativeBudget
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = tx.Exec(`INSERT INTO creative_cost_ledger_v2720(id,tenant_id,user_id,period,operation,provider,estimated_cents,status,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, id, tenantID, userID, period, operation, provider, cents, "held", now, now)
	if err != nil {
		return nil, err
	}
	hold := &creativeHold{ID: id, TenantID: tenantID, Cents: cents, Period: period, Operation: operation}
	if operation == "core_ai" {
		err = a.commercialCoreReserveTx(tx, hold, plan)
	} else {
		err = a.commercialReserveCreditsTx(tx, hold, plan, operation, duration, chars)
	}
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return hold, nil
}

// settle=true if the vendor request was attempted. Actual billing is not known;
// this conservatively counts its estimate even for ambiguous failures.
// settle=false releases the reservation ONLY when nothing was sent to provider.
func (a *App) creativeSettle(hold *creativeHold, settle bool) error {
	if hold == nil {
		return nil
	}
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	status := "released"
	if settle {
		status = "estimated"
	}
	result, err := tx.Exec(`UPDATE creative_cost_ledger_v2720 SET status=?,updated_at=? WHERE id=? AND tenant_id=? AND status='held'`, status, time.Now().UTC().Format(time.RFC3339Nano), hold.ID, hold.TenantID)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return errors.New("reserva ya liquidada")
	}
	spend := int64(0)
	if settle {
		spend = hold.Cents
	}
	for _, tid := range []int64{0, hold.TenantID} {
		res, e := tx.Exec(`UPDATE creative_cost_budgets_v2720 SET reserved_cents=reserved_cents-?,spent_cents=spent_cents+? WHERE tenant_id=? AND period=? AND reserved_cents>=?`, hold.Cents, spend, tid, hold.Period, hold.Cents)
		if e != nil {
			return e
		}
		affected, e := res.RowsAffected()
		if e != nil {
			return e
		}
		if affected != 1 {
			return errors.New("saldo de reserva incoherente")
		}
	}
	if hold.Operation == "core_ai" {
		err = a.commercialCoreSettleTx(tx, hold, settle)
	} else {
		err = a.commercialSettleCreditsTx(tx, hold, settle)
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (a *App) creativeReserveForStudio(w http.ResponseWriter, tid, uid int64, plan, operation, provider string, cents int64, duration, chars int) (*creativeHold, bool) {
	hold, err := a.creativeReserve(tid, uid, plan, operation, provider, cents, duration, chars)
	if err != nil {
		if errors.Is(err, errCreativeBudget) {
			http.Error(w, "Generación IA pausada: presupuesto de empresa o global insuficiente. Contacta al administrador.", http.StatusTooManyRequests)
		} else {
			http.Error(w, "Generación bloqueada por derechos, créditos o reserva financiera: "+err.Error(), http.StatusForbidden)
		}
		return nil, false
	}
	return hold, true
}

type creativeBudgetReport struct {
	TenantID  int64  `json:"tenant_id"`
	Period    string `json:"period"`
	Limit     int64  `json:"limit_cents"`
	Held      int64  `json:"reserved_cents"`
	Estimated int64  `json:"estimated_spent_cents"`
}

func (a *App) creativeCostStatusV2720Handler(w http.ResponseWriter, r *http.Request) {
	tid, u, err := a.tenantFor(r)
	if err != nil {
		http.Error(w, "Sesión requerida", 401)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", 405)
		return
	}
	plan, _, err := a.activePlan(a.billingAccountUserID(u))
	if err != nil {
		http.Error(w, "Plan no disponible", 503)
		return
	}
	period := creativeMonth(time.Now())
	b := creativeBudgetReport{TenantID: tid, Period: period, Limit: creativeTenantLimit(plan.Code) + commercialPurchasedFundingCents(a.db, tid)}
	err = a.db.QueryRow(`SELECT limit_cents,reserved_cents,spent_cents FROM creative_cost_budgets_v2720 WHERE tenant_id=? AND period=?`, tid, period).Scan(&b.Limit, &b.Held, &b.Estimated)
	if err != nil && err != sql.ErrNoRows {
		http.Error(w, "No se pudo leer presupuesto", 500)
		return
	}
	available := b.Limit - b.Held - b.Estimated
	if available < 0 {
		available = 0
	}
	writeJSON(w, map[string]any{"budget": b, "available_cents": available, "billing_enabled": false, "estimates_only": true, "quotes_cents": map[string]int64{"image": creativeQuote("image", 0, 0), "video_5s_720p": creativeQuote("video", 5, 0), "video_10s_720p": creativeQuote("video", 10, 0), "text": creativeQuote("copy", 0, 0), "tts_1000_characters": creativeQuote("narration", 0, 1000)}})
}

func (a *App) creativeAdminCostsV2720Handler(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireSuperadmin(w, r); !ok {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", 405)
		return
	}
	period := r.URL.Query().Get("period")
	if len(period) != 7 || period[4] != '-' {
		period = creativeMonth(time.Now())
	}
	rows, err := a.db.Query(`SELECT tenant_id,period,limit_cents,reserved_cents,spent_cents FROM creative_cost_budgets_v2720 WHERE period=? ORDER BY tenant_id LIMIT 500`, period)
	if err != nil {
		http.Error(w, "Error consultando presupuestos", 500)
		return
	}
	defer rows.Close()
	result := []creativeBudgetReport{}
	for rows.Next() {
		var x creativeBudgetReport
		if rows.Scan(&x.TenantID, &x.Period, &x.Limit, &x.Held, &x.Estimated) == nil {
			result = append(result, x)
		}
	}
	if rows.Err() != nil {
		http.Error(w, "Error en presupuestos", 500)
		return
	}
	operations := []map[string]any{}
	usage, err := a.db.Query(`SELECT operation,provider,status,COUNT(*),COALESCE(SUM(estimated_cents),0) FROM creative_cost_ledger_v2720 WHERE period=? GROUP BY operation,provider,status ORDER BY operation,provider,status`, period)
	if err != nil {
		http.Error(w, "No se pudo consultar el desglose de consumo", 500)
		return
	}
	for usage.Next() {
		var op, provider, status string
		var count, cents int64
		if usage.Scan(&op, &provider, &status, &count, &cents) == nil {
			operations = append(operations, map[string]any{"operation": op, "provider": provider, "status": status, "requests": count, "estimated_cents": cents})
		}
	}
	err = usage.Err()
	usage.Close()
	if err != nil {
		http.Error(w, "No se pudo completar el desglose", 500)
		return
	}
	writeJSON(w, map[string]any{"period": period, "budgets": result, "operations": operations, "pricing_source": "estimaciones configuradas; no equivalen a facturas verificadas", "customer_charges_enabled": false})
}

// Pure amount helpers kept small for targeted unit testing.
func creativeRequestQuote(action string, duration, chars int) (int64, error) {
	cents := creativeQuote(action, duration, chars)
	if cents <= 0 {
		return 0, fmt.Errorf("operación no cotizable: %s", action)
	}
	return cents, nil
}

// Used by JS to present transparent estimates before paid external actions.
func (a *App) creativeQuoteV2720Handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", 405)
		return
	}
	if _, _, err := a.tenantFor(r); err != nil {
		http.Error(w, "Sesión requerida", 401)
		return
	}
	action := r.URL.Query().Get("action")
	duration, _ := strconv.Atoi(r.URL.Query().Get("duration"))
	chars, _ := strconv.Atoi(r.URL.Query().Get("chars"))
	cents, err := creativeRequestQuote(action, duration, chars)
	if err != nil {
		http.Error(w, "Operación inválida", 400)
		return
	}
	writeJSON(w, map[string]any{"estimated_provider_cost_cents": cents, "currency": "USD", "not_a_customer_charge": !commercialEnabled(), "credit_debit_enabled": commercialEnabled(), "note": "Costo externo estimado; el débito se realiza en créditos si el piloto comercial está activado"})
}
