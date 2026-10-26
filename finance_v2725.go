package main

// WorkticAI V27.25. Verified invoices are organizational facts; estimated
// reserves are NOT verified costs. Tenant allocations require explicit evidence.
// All mutations remain off unless an administrator enables a staging flag.
import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

var financePeriodRE = regexp.MustCompile(`^[0-9]{4}-(0[1-9]|1[0-2])$`)
var financeRefRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9:_./-]{2,159}$`)

func financeV2725WriteEnabled() bool { return os.Getenv("WORKTIC_FINANCE_V2725_WRITE") == "1" }
func financeV2725BillingEnabled() bool {
	return os.Getenv("WORKTIC_BILLING_V2725_SAFE_APPROVAL") == "1"
}
func financeValidPeriod(p string) bool { return financePeriodRE.MatchString(p) }
func financePeriod(r *http.Request) string {
	p := r.URL.Query().Get("period")
	if !financeValidPeriod(p) {
		return time.Now().UTC().Format("2006-01")
	}
	return p
}
func financeVendor(v string) bool {
	switch v {
	case "openai", "luma", "meta", "render", "redis", "storage", "messaging", "other":
		return true
	}
	return false
}
func financePaymentEndV2725(start time.Time, cycle string, billingDays int) (time.Time, error) {
	if cycle == "annual" {
		return start.AddDate(1, 0, 0), nil
	}
	if cycle == "monthly" && billingDays > 0 && billingDays <= 60 {
		return start.AddDate(0, 0, billingDays), nil
	}
	return time.Time{}, errors.New("ciclo de pago sin verificar; auditar antes de aprobar")
}

func initFinanceV2725Schema(db *DB) error {
	for _, q := range []string{
		`CREATE TABLE IF NOT EXISTS finance_invoices_v2725(provider TEXT NOT NULL, invoice_ref TEXT NOT NULL, period TEXT NOT NULL, total_cents INTEGER NOT NULL CHECK(total_cents>=0), currency TEXT NOT NULL DEFAULT 'USD', evidence_ref TEXT NOT NULL, reviewer_id INTEGER NOT NULL, created_at TEXT NOT NULL, PRIMARY KEY(provider,invoice_ref))`,
		`CREATE INDEX IF NOT EXISTS finance_invoices_period_v2725 ON finance_invoices_v2725(period,provider)`,
		`CREATE TABLE IF NOT EXISTS finance_actuals_v2725(provider TEXT NOT NULL, source_ref TEXT NOT NULL, invoice_ref TEXT NOT NULL, tenant_id INTEGER NOT NULL, period TEXT NOT NULL, operation TEXT NOT NULL, hold_id TEXT NOT NULL DEFAULT '', actual_cents INTEGER NOT NULL CHECK(actual_cents>=0), reviewer_id INTEGER NOT NULL, created_at TEXT NOT NULL, PRIMARY KEY(provider,source_ref))`,
		`CREATE INDEX IF NOT EXISTS finance_actuals_tenant_v2725 ON finance_actuals_v2725(tenant_id,period,provider)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS finance_actuals_hold_v2725 ON finance_actuals_v2725(hold_id) WHERE hold_id<>''`,
	} {
		if _, err := db.Exec(q); err != nil {
			return fmt.Errorf("v2725 schema: %w", err)
		}
	}
	// Unknown for pre-existing payments. Never infer annual from price or discount.
	_, err := db.Exec(`ALTER TABLE billing_payments ADD COLUMN billing_cycle TEXT NOT NULL DEFAULT 'unverified'`)
	if err != nil {
		msg := strings.ToLower(err.Error())
		if !strings.Contains(msg, "duplicate column") && !strings.Contains(msg, "already exists") {
			return err
		}
	}
	return nil
}

// Staging only: manual review with a single-transaction status transition.
// The existing compensation engine is invoked AFTER commit; its own dedupe
// remains authoritative. Billing reviewer must independently verify blockchain.
func (a *App) financePaymentReviewV2725(w http.ResponseWriter, r *http.Request, admin *User, id int64, action, note string) {
	if r.Method != http.MethodPut || id <= 0 {
		http.Error(w, "solicitud inválida", 400)
		return
	}
	tx, err := a.db.Begin()
	if err != nil {
		http.Error(w, "base de datos no disponible", 503)
		return
	}
	defer tx.Rollback()
	var uid int64
	var plan, cycle, status string
	err = tx.QueryRow(`SELECT user_id,plan_code,billing_cycle,status FROM billing_payments WHERE id=?`, id).Scan(&uid, &plan, &cycle, &status)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "pago no encontrado", 404)
		return
	}
	if err != nil {
		http.Error(w, "no se pudo leer el pago", 503)
		return
	}
	if status != "pending" {
		http.Error(w, "pago ya procesado", 409)
		return
	}
	// Serializa aprobaciones de pagos distintos de la misma empresa: una sola
	// cadena de renovación puede calcular la siguiente fecha de vencimiento.
	var tenantID int64
	if err = tx.QueryRow(`SELECT tenant_id FROM app_users WHERE id=?`, uid).Scan(&tenantID); err != nil || tenantID <= 0 {
		http.Error(w, "pago sin empresa titular válida", 409)
		return
	}
	if err = lockConsumptionTenantV2723(tx, a.db.IsPostgres(), tenantID); err != nil {
		http.Error(w, "no se pudo bloquear ciclo de la empresa", 503)
		return
	}
	if billingLifecycleV2728Enabled() && action == "approve" {
		var oldPlan, oldEndRaw string
		e := tx.QueryRow(`SELECT plan_code,ends_at FROM billing_subscriptions WHERE user_id=? AND status='active' AND ends_at>? ORDER BY ends_at DESC LIMIT 1`, uid, time.Now().UTC().Format(time.RFC3339)).Scan(&oldPlan, &oldEndRaw)
		if e != nil && !errors.Is(e, sql.ErrNoRows) {
			http.Error(w, "No se pudo validar el plan vigente", 503)
			return
		}
		oldEnd := time.Time{}
		if e == nil {
			oldEnd, e = time.Parse(time.RFC3339, oldEndRaw)
			if e != nil {
				http.Error(w, "Vencimiento incompatible", 409)
				return
			}
		}
		if _, e = billingTransitionV2728(oldPlan, oldEnd, plan, cycle, time.Now().UTC()); e != nil {
			http.Error(w, e.Error(), 409)
			return
		}
	}
	if action == "approve" && contractV2729Enabled() {
		if e := a.contractPreapproveV2729(tx, uid, plan, time.Now().UTC()); e != nil {
			http.Error(w, "Contrato no aprobado: "+e.Error(), 409)
			return
		}
	}
	if action == "approve" && cycle != "monthly" && cycle != "annual" {
		http.Error(w, "pago antiguo sin ciclo verificado; revisar manualmente", 409)
		return
	}
	result, err := tx.Exec(`UPDATE billing_payments SET status='processing' WHERE id=? AND status='pending'`, id)
	if err != nil {
		http.Error(w, "falló reserva del pago", 503)
		return
	}
	if n, _ := result.RowsAffected(); n != 1 {
		http.Error(w, "pago tomado por otro proceso", 409)
		return
	}
	now := time.Now().UTC()
	newStatus := "rejected"
	var subscriptionID int64
	if action == "approve" {
		var billingDays int
		err = tx.QueryRow(`SELECT billing_days FROM billing_plans WHERE code=? AND active=1`, plan).Scan(&billingDays)
		if err != nil {
			http.Error(w, "plan inválido", 409)
			return
		}
		start := now
		var oldPlan, oldEndRaw string
		oldErr := tx.QueryRow(`SELECT plan_code,ends_at FROM billing_subscriptions WHERE user_id=? AND status='active' ORDER BY ends_at DESC LIMIT 1`, uid).Scan(&oldPlan, &oldEndRaw)
		if oldErr != nil && !errors.Is(oldErr, sql.ErrNoRows) {
			http.Error(w, "error consultando renovación", 503)
			return
		}
		if oldPlan == plan {
			if t, e := time.Parse(time.RFC3339, oldEndRaw); e == nil && t.After(now) {
				start = t
			}
		}
		end, e := financePaymentEndV2725(start, cycle, billingDays)
		if e != nil {
			http.Error(w, e.Error(), 409)
			return
		}
		if _, err = tx.Exec(`UPDATE billing_subscriptions SET status='replaced' WHERE user_id=? AND status='active'`, uid); err != nil {
			http.Error(w, "no se pudo actualizar suscripción anterior", 503)
			return
		}
		res, e := tx.Exec(`INSERT INTO billing_subscriptions(user_id,plan_code,status,starts_at,ends_at,created_at) VALUES(?,?,'active',?,?,?)`, uid, plan, now.Format(time.RFC3339), end.Format(time.RFC3339), now.Format(time.RFC3339))
		if e != nil {
			http.Error(w, "error activando suscripción", 503)
			return
		}
		subscriptionID, e = res.LastInsertId()
		if e != nil || subscriptionID <= 0 {
			http.Error(w, "no se pudo confirmar suscripción", 503)
			return
		}
		if e = a.contractCaptureV2729(tx, uid, subscriptionID, plan, cycle, now, end); e != nil {
			http.Error(w, "no se pudo conservar derechos del contrato", 503)
			return
		}
		newStatus = "approved"
	}
	result, err = tx.Exec(`UPDATE billing_payments SET status=?,admin_note=?,reviewed_at=?,subscription_id=? WHERE id=? AND status='processing'`, newStatus, strings.TrimSpace(note), now.Format(time.RFC3339), subscriptionID, id)
	if err != nil {
		http.Error(w, "no se pudo cerrar pago", 503)
		return
	}
	if n, _ := result.RowsAffected(); n != 1 {
		http.Error(w, "pago incompatible", 409)
		return
	}
	if err = tx.Commit(); err != nil {
		http.Error(w, "no se pudo confirmar transacción", 503)
		return
	}
	partnerOK := true
	if action == "approve" {
		if err = a.processPartnerPaymentV2715(id); err != nil {
			partnerOK = false
			log.Printf("v2725: comisión pendiente conciliación para pago %d: %v", id, err)
		}
	}
	writeJSON(w, map[string]any{"ok": true, "status": newStatus, "billing_cycle": cycle, "subscription_id": subscriptionID, "partners_processed": partnerOK, "partner_reconciliation_required": !partnerOK})
}

type financeInvoiceV2725 struct {
	Provider       string `json:"provider"`
	InvoiceRef     string `json:"invoice_ref"`
	Period         string `json:"period"`
	TotalCents     int64  `json:"total_cents"`
	Currency       string `json:"currency"`
	EvidenceRef    string `json:"evidence_ref"`
	VerifiedSource bool   `json:"verified_source"`
}
type financeActualV2725 struct {
	Provider       string `json:"provider"`
	SourceRef      string `json:"source_ref"`
	InvoiceRef     string `json:"invoice_ref"`
	TenantID       int64  `json:"tenant_id"`
	Period         string `json:"period"`
	Operation      string `json:"operation"`
	HoldID         string `json:"hold_id"`
	ActualCents    int64  `json:"actual_cents"`
	VerifiedSource bool   `json:"verified_source"`
}

func financeInputOK(provider, ref, period string) bool {
	return financeVendor(provider) && financeRefRE.MatchString(ref) && financeValidPeriod(period)
}
func financePostGuard(w http.ResponseWriter, r *http.Request) bool {
	if !financeV2725WriteEnabled() {
		http.Error(w, "importación de costos deshabilitada: solo staging", 503)
		return false
	}
	if r.Header.Get("Content-Type") != "application/json" {
		http.Error(w, "Content-Type application/json requerido", 415)
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	return true
}

func (a *App) financeInvoicesV2725Handler(w http.ResponseWriter, r *http.Request) {
	admin, ok := a.requireSuperadmin(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		period := financePeriod(r)
		rows, err := a.db.Query(`SELECT i.provider,i.invoice_ref,i.period,i.total_cents,i.currency,i.evidence_ref,COALESCE((SELECT SUM(x.actual_cents) FROM finance_actuals_v2725 x WHERE x.provider=i.provider AND x.invoice_ref=i.invoice_ref),0) FROM finance_invoices_v2725 i WHERE i.period=? ORDER BY i.provider,i.invoice_ref LIMIT 500`, period)
		if err != nil {
			http.Error(w, "error leyendo facturas", 503)
			return
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var provider, ref, p, currency, evidence string
			var cents, allocated int64
			if rows.Scan(&provider, &ref, &p, &cents, &currency, &evidence, &allocated) != nil {
				http.Error(w, "error leyendo factura", 503)
				return
			}
			out = append(out, map[string]any{"provider": provider, "invoice_ref": ref, "period": p, "total_cents": cents, "allocated_cents": allocated, "unallocated_cents": cents - allocated, "currency": currency, "evidence_ref": evidence})
		}
		if rows.Err() != nil {
			http.Error(w, "facturas incompletas", 503)
			return
		}
		writeJSON(w, map[string]any{"period": period, "invoices": out, "source": "manual verified invoice import; values are NOT automatically downloaded"})
	case http.MethodPost:
		if !financePostGuard(w, r) {
			return
		}
		var x financeInvoiceV2725
		if json.NewDecoder(r.Body).Decode(&x) != nil || !x.VerifiedSource || !financeInputOK(x.Provider, x.InvoiceRef, x.Period) || x.Currency != "USD" || x.TotalCents < 0 || x.TotalCents > 10000000000 || !financeRefRE.MatchString(x.EvidenceRef) {
			http.Error(w, "factura inválida; requiere documento comprobado y USD", 400)
			return
		}
		res, err := a.db.Exec(`INSERT INTO finance_invoices_v2725(provider,invoice_ref,period,total_cents,currency,evidence_ref,reviewer_id,created_at) VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(provider,invoice_ref) DO NOTHING`, x.Provider, x.InvoiceRef, x.Period, x.TotalCents, x.Currency, x.EvidenceRef, admin.ID, time.Now().UTC().Format(time.RFC3339))
		if err != nil {
			http.Error(w, "error guardando factura", 503)
			return
		}
		n, _ := res.RowsAffected()
		if n != 1 {
			http.Error(w, "factura ya registrada; no se sobrescribe", 409)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "registered": true, "allocation": "pending", "manual_verification": true})
	default:
		http.Error(w, "método no permitido", 405)
	}
}

// A provider invoice can be allocated to known tenant requests only when a
// reviewer has evidence; total allocations never exceed the source invoice.
func (a *App) financeActualsV2725Handler(w http.ResponseWriter, r *http.Request) {
	admin, ok := a.requireSuperadmin(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		period := financePeriod(r)
		rows, err := a.db.Query(`SELECT provider,source_ref,invoice_ref,tenant_id,period,operation,hold_id,actual_cents,created_at FROM finance_actuals_v2725 WHERE period=? ORDER BY created_at DESC LIMIT 1000`, period)
		if err != nil {
			http.Error(w, "error leyendo costos", 503)
			return
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var p, ref, inv, mo, op, hold, created string
			var tid, cents int64
			if rows.Scan(&p, &ref, &inv, &tid, &mo, &op, &hold, &cents, &created) != nil {
				http.Error(w, "error leyendo asignación", 503)
				return
			}
			out = append(out, map[string]any{"provider": p, "source_ref": ref, "invoice_ref": inv, "tenant_id": tid, "period": mo, "operation": op, "hold_id": hold, "actual_cents": cents, "created_at": created})
		}
		if rows.Err() != nil {
			http.Error(w, "registro incompleto", 503)
			return
		}
		writeJSON(w, map[string]any{"period": period, "actuals": out, "verified_manually": true})
	case http.MethodPost:
		if !financePostGuard(w, r) {
			return
		}
		var x financeActualV2725
		if json.NewDecoder(r.Body).Decode(&x) != nil || !x.VerifiedSource || !financeInputOK(x.Provider, x.SourceRef, x.Period) || !financeRefRE.MatchString(x.InvoiceRef) || !financeRefRE.MatchString(x.Operation) || x.ActualCents < 0 || x.ActualCents > 10000000000 || x.TenantID < 0 {
			http.Error(w, "costo inválido; se exige factura y referencia verificadas", 400)
			return
		}
		tx, err := a.db.Begin()
		if err != nil {
			http.Error(w, "no se pudo comenzar transacción", 503)
			return
		}
		defer tx.Rollback()
		// Lock the invoice row for multi-instance PostgreSQL. SQLite serializes writes.
		var cap int64
		var month string
		query := `SELECT total_cents,period FROM finance_invoices_v2725 WHERE provider=? AND invoice_ref=?`
		if a.db.IsPostgres() {
			query += ` FOR UPDATE`
		}
		if err = tx.QueryRow(query, x.Provider, x.InvoiceRef).Scan(&cap, &month); err != nil {
			http.Error(w, "factura no registrada", 409)
			return
		}
		if month != x.Period {
			http.Error(w, "mes distinto al documento", 409)
			return
		}
		if x.HoldID != "" {
			if x.TenantID <= 0 || !financeRefRE.MatchString(x.HoldID) {
				http.Error(w, "reserva inválida", 400)
				return
			}
			var t int64
			var prov string
			if err = tx.QueryRow(`SELECT tenant_id,provider FROM creative_cost_ledger_v2720 WHERE id=?`, x.HoldID).Scan(&t, &prov); err != nil || t != x.TenantID || prov != x.Provider {
				http.Error(w, "reserva no coincide con empresa o proveedor", 409)
				return
			}
		}
		var used int64
		if err = tx.QueryRow(`SELECT COALESCE(SUM(actual_cents),0) FROM finance_actuals_v2725 WHERE provider=? AND invoice_ref=?`, x.Provider, x.InvoiceRef).Scan(&used); err != nil {
			http.Error(w, "no se pudo cotejar factura", 503)
			return
		}
		if x.ActualCents > cap-used {
			http.Error(w, "asignación supera la factura verificada", 409)
			return
		}
		res, err := tx.Exec(`INSERT INTO finance_actuals_v2725(provider,source_ref,invoice_ref,tenant_id,period,operation,hold_id,actual_cents,reviewer_id,created_at) VALUES(?,?,?,?,?,?,?,?,?,?) ON CONFLICT(provider,source_ref) DO NOTHING`, x.Provider, x.SourceRef, x.InvoiceRef, x.TenantID, x.Period, x.Operation, x.HoldID, x.ActualCents, admin.ID, time.Now().UTC().Format(time.RFC3339))
		if err != nil {
			http.Error(w, "duplicado o inconsistencia", 409)
			return
		}
		n, _ := res.RowsAffected()
		if n != 1 {
			http.Error(w, "referencia ya contabilizada", 409)
			return
		}
		if err = tx.Commit(); err != nil {
			http.Error(w, "no se pudo confirmar costo", 503)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "actual_cents": x.ActualCents, "currency": "USD", "attribution": "manual invoice verified"})
	default:
		http.Error(w, "método no permitido", 405)
	}
}

type financeTenantSummary struct {
	TenantID       int64   `json:"tenant_id"`
	EstimatedCents int64   `json:"estimated_cents"`
	VerifiedCents  int64   `json:"verified_cents"`
	Allocations    int64   `json:"allocations"`
	RevenueUSDT    float64 `json:"approved_revenue_usdt"`
}

func (a *App) financeTenantSummaryV2725(tid int64, period string) (financeTenantSummary, error) {
	out := financeTenantSummary{TenantID: tid}
	if err := a.db.QueryRow(`SELECT COALESCE(SUM(estimated_cents),0) FROM creative_cost_ledger_v2720 WHERE tenant_id=? AND period=? AND status='estimated'`, tid, period).Scan(&out.EstimatedCents); err != nil {
		return out, err
	}
	if err := a.db.QueryRow(`SELECT COALESCE(SUM(actual_cents),0),COUNT(*) FROM finance_actuals_v2725 WHERE tenant_id=? AND period=?`, tid, period).Scan(&out.VerifiedCents, &out.Allocations); err != nil {
		return out, err
	}
	if err := a.db.QueryRow(`SELECT COALESCE(SUM(p.amount_usdt),0) FROM billing_payments p JOIN app_users u ON u.id=p.user_id WHERE u.tenant_id=? AND p.status='approved' AND substr(p.reviewed_at,1,7)=?`, tid, period).Scan(&out.RevenueUSDT); err != nil {
		return out, err
	}
	return out, nil
}

// Tenant-facing statement intentionally never exposes wholesale supplier costs,
// invoice identifiers or platform margin. Those are superadmin-only.
func (a *App) financeTenantStatementV2725Handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "método no permitido", 405)
		return
	}
	tid, _, err := a.tenantFor(r)
	if err != nil {
		http.Error(w, "sesión requerida", 401)
		return
	}
	period := financePeriod(r)
	var estimated, verified int64
	if err = a.db.QueryRow(`SELECT COUNT(*) FROM creative_cost_ledger_v2720 WHERE tenant_id=? AND period=? AND status='estimated'`, tid, period).Scan(&estimated); err != nil {
		http.Error(w, "no se pudo leer historial", 503)
		return
	}
	if err = a.db.QueryRow(`SELECT COUNT(*) FROM finance_actuals_v2725 WHERE tenant_id=? AND period=?`, tid, period).Scan(&verified); err != nil {
		http.Error(w, "no se pudo leer conciliación", 503)
		return
	}
	writeJSON(w, map[string]any{"period": period, "provider_generations_recorded": estimated, "provider_generations_reconciled": verified, "customer_invoice": false, "message": "Información informativa. Créditos y pagos al cliente se consultan desde la billetera y suscripciones; este resumen no constituye una factura."})
}
func (a *App) financeSummaryV2725Handler(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireSuperadmin(w, r); !ok {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "método no permitido", 405)
		return
	}
	p := financePeriod(r)
	var invoices, assigned, unknownEst int64
	if err := a.db.QueryRow(`SELECT COALESCE(SUM(total_cents),0) FROM finance_invoices_v2725 WHERE period=?`, p).Scan(&invoices); err != nil {
		http.Error(w, "no se pudo leer facturas", 503)
		return
	}
	if err := a.db.QueryRow(`SELECT COALESCE(SUM(actual_cents),0) FROM finance_actuals_v2725 WHERE period=?`, p).Scan(&assigned); err != nil {
		http.Error(w, "no se pudo leer conciliaciones", 503)
		return
	}
	if err := a.db.QueryRow(`SELECT COALESCE(SUM(estimated_cents),0) FROM creative_cost_ledger_v2720 WHERE period=? AND status='estimated'`, p).Scan(&unknownEst); err != nil {
		http.Error(w, "no se pudo leer estimaciones", 503)
		return
	}
	var sales float64
	if err := a.db.QueryRow(`SELECT COALESCE(SUM(amount_usdt),0) FROM billing_payments WHERE status='approved' AND substr(reviewed_at,1,7)=?`, p).Scan(&sales); err != nil {
		http.Error(w, "no se pudo leer ventas", 503)
		return
	}
	rows, err := a.db.Query(`SELECT tenant_id,COALESCE(SUM(actual_cents),0) FROM finance_actuals_v2725 WHERE period=? GROUP BY tenant_id ORDER BY tenant_id LIMIT 500`, p)
	if err != nil {
		http.Error(w, "error de desglose", 503)
		return
	}
	defer rows.Close()
	byTenant := []map[string]any{}
	for rows.Next() {
		var tid, cents int64
		if rows.Scan(&tid, &cents) != nil {
			http.Error(w, "error desglose", 503)
			return
		}
		byTenant = append(byTenant, map[string]any{"tenant_id": tid, "verified_cents": cents})
	}
	if rows.Err() != nil {
		http.Error(w, "error de filas", 503)
		return
	}
	writeJSON(w, map[string]any{"period": p, "verified_invoices_usd_cents": invoices, "allocated_usd_cents": assigned, "unallocated_usd_cents": invoices - assigned, "estimated_operations_usd_cents": unknownEst, "sales_approved_usdt": sales, "verified_allocation_by_tenant": byTenant, "finance_complete": false, "why_not_net_margin": "USDT/USD FX, suscripción anual diferida, Partners, impuestos, cargos Meta/hosting y facturas faltantes", "record_creation_enabled": financeV2725WriteEnabled(), "billing_safe_approval_enabled": financeV2725BillingEnabled()})
}
