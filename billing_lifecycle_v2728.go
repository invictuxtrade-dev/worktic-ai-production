package main

// V27.28 — Opt-in lifecycle controls for checkout and subscription payments.
// This is a staging pilot. It does not confirm blockchain payment or implement
// upgrades, proration, automatic refunds, or a second Partner compensation flow.
import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

func billingLifecycleV2728Enabled() bool {
	return os.Getenv("WORKTIC_BILLING_LIFECYCLE_V2728") == "1" && financeV2725BillingEnabled()
}

// Only explicit contractual cycles are accepted in this pilot; the old
// billingCyclePrice default to monthly remains available while flag is OFF.
func billingCycleValidV2728(v string) bool { return v == "monthly" || v == "annual" }

// Paid-to-paid plan switches need a documented prorating/grandfathering policy.
// Reject them rather than silently shortening a customer's paid entitlement.
func billingTransitionV2728(oldPlan string, oldEnd time.Time, nextPlan, cycle string, now time.Time) (string, error) {
	if !billingCycleValidV2728(cycle) {
		return "", errors.New("selecciona explícitamente el ciclo mensual o anual")
	}
	if nextPlan != "personal" && nextPlan != "business" && nextPlan != "enterprise" {
		return "", errors.New("plan de destino inválido")
	}
	if oldPlan != "" && oldPlan != "free" && oldPlan != nextPlan && oldEnd.After(now) {
		return "blocked_change", errors.New("cambio de plan con tiempo pagado pendiente: requiere política de prorrateo y revisión administrativa; no envíes todavía otro pago")
	}
	if oldPlan == nextPlan && oldEnd.After(now) {
		return "renewal", nil
	}
	if oldPlan == nextPlan {
		return "reactivation", nil
	}
	return "new_subscription", nil
}

// Claims are shared between plan checkout and creative credit topups; the
// unique index resolves simultaneous submissions from multiple Render workers.
func initBillingLifecycleV2728Schema(db *DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS billing_payment_claims_v2728 (
        network TEXT NOT NULL, tx_hash TEXT NOT NULL, source_type TEXT NOT NULL,
        source_id TEXT NOT NULL, tenant_id INTEGER NOT NULL, claimed_at TEXT NOT NULL,
        PRIMARY KEY(network,tx_hash))`)
	return err
}
func billingNormalizedHashV2728(network, hash string) string {
	return strings.ToLower(strings.TrimSpace(hash))
}
func billingClaimTxV2728(tx *Tx, network, hash, sourceType, sourceID string, tenant int64) error {
	network = strings.ToUpper(strings.TrimSpace(network))
	hash = billingNormalizedHashV2728(network, hash)
	if !commercialTxHash(network, hash) || tenant <= 0 || sourceID == "" || (sourceType != "subscription" && sourceType != "studio_topup" && sourceType != "voice_topup") {
		return errors.New("referencia de pago inválida")
	}
	// Pre-existing rows are not in the shared ledger: block their reuse too.
	var oldCount int
	if err := tx.QueryRow(`SELECT (SELECT COUNT(*) FROM billing_payments WHERE LOWER(tx_hash)=?) + (SELECT COUNT(*) FROM commercial_topups_v2721 WHERE LOWER(tx_hash)=?) + (SELECT COUNT(*) FROM voice_topups_v2735 WHERE LOWER(payment_reference)=LOWER(?))`, hash, hash, network+":"+hash).Scan(&oldCount); err != nil {
		return err
	}
	if oldCount > 0 {
		return errors.New("transacción ya registrada en suscripciones o créditos")
	}
	_, err := tx.Exec(`INSERT INTO billing_payment_claims_v2728(network,tx_hash,source_type,source_id,tenant_id,claimed_at) VALUES(?,?,?,?,?,?)`, network, hash, sourceType, sourceID, tenant, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return errors.New("referencia ya reclamada por otra operación")
	}
	return nil
}

func (a *App) billingOwnerV2728(u *User) (int64, error) {
	if u == nil || u.TenantID <= 0 {
		return 0, errors.New("sesión empresarial requerida")
	}
	var owner int64
	if err := a.db.QueryRow(`SELECT owner_user_id FROM tenants WHERE id=?`, u.TenantID).Scan(&owner); err != nil || owner <= 0 {
		return 0, errors.New("no se pudo comprobar el titular")
	}
	if owner != u.ID && u.Role != "superadmin" {
		return 0, errors.New("solo el titular de facturación puede presentar pagos")
	}
	return owner, nil
}
func (a *App) billingActiveSnapshotV2728(owner int64) (string, time.Time, error) {
	var code, endStr string
	err := a.db.QueryRow(`SELECT plan_code,ends_at FROM billing_subscriptions WHERE user_id=? AND status='active' AND ends_at>? ORDER BY ends_at DESC LIMIT 1`, owner, time.Now().UTC().Format(time.RFC3339)).Scan(&code, &endStr)
	if errors.Is(err, sql.ErrNoRows) {
		return "", time.Time{}, nil
	}
	if err != nil {
		return "", time.Time{}, err
	}
	end, e := time.Parse(time.RFC3339, endStr)
	if e != nil {
		return "", time.Time{}, fmt.Errorf("fecha de vencimiento inválida: %w", e)
	}
	return code, end, nil
}

func (a *App) billingPreviewV2728Handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", 405)
		return
	}
	u := a.currentUser(r)
	if u == nil {
		http.Error(w, "Sesión requerida", 401)
		return
	}
	owner, err := a.billingOwnerV2728(u)
	if err != nil {
		http.Error(w, err.Error(), 403)
		return
	}
	next := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("plan_code")))
	cycle := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("billing_cycle")))
	var price float64
	var days int
	if err = a.db.QueryRow(`SELECT price_usdt,billing_days FROM billing_plans WHERE code=? AND active=1`, next).Scan(&price, &days); err != nil || price <= 0 {
		http.Error(w, "Plan no disponible", 400)
		return
	}
	oldPlan, oldEnd, err := a.billingActiveSnapshotV2728(owner)
	if err != nil {
		http.Error(w, "No se pudo consultar la suscripción", 503)
		return
	}
	operation, policyErr := billingTransitionV2728(oldPlan, oldEnd, next, cycle, time.Now().UTC())
	if policyErr == nil {
		policyErr = a.contractPreflightV2729(owner, next)
	}
	plan := Plan{PriceUSDT: price}
	amount, _ := billingCyclePrice(plan, cycle)
	if !billingCycleValidV2728(cycle) {
		amount = 0
	}
	var end string
	if policyErr == nil {
		start := time.Now().UTC()
		if operation == "renewal" {
			start = oldEnd
		}
		if planned, e := financePaymentEndV2725(start, cycle, days); e == nil {
			end = planned.Format(time.RFC3339)
		}
	}
	msg := ""
	if policyErr != nil {
		msg = policyErr.Error()
	}
	oldEndDisplay := ""
	if !oldEnd.IsZero() {
		oldEndDisplay = oldEnd.Format(time.RFC3339)
	}
	writeJSON(w, map[string]any{
		"version": "27.28", "staging_pilot_enabled": billingLifecycleV2728Enabled(),
		"checkout_allowed": policyErr == nil && billingLifecycleV2728Enabled(),
		"owner_user_id":    owner, "current_plan": oldPlan, "current_ends_at": oldEndDisplay,
		"requested_plan": next, "cycle": cycle, "type": operation, "amount_before_promotions_usdt": amount,
		"projected_ends_at": end, "policy_message": msg, "actual_paid": false, "preview_only": true,
		"note": "Importe previo a cupones. No prueba pago ni autoriza activación; prorrateo de cambios de plan no implementado."})
}

// Read-only audit. Does not silently migrate old payments or mutate Partners.
func (a *App) billingLifecycleAuditV2728Handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", 405)
		return
	}
	if _, ok := a.requireSuperadmin(w, r); !ok {
		return
	}
	metrics := map[string]int64{}
	queries := map[string]string{
		"unverified_pending_cycles":        `SELECT COUNT(*) FROM billing_payments WHERE billing_cycle='unverified' AND status='pending'`,
		"pending_subscription_payments":    `SELECT COUNT(*) FROM billing_payments WHERE status='pending'`,
		"pending_topups":                   `SELECT COUNT(*) FROM commercial_topups_v2721 WHERE status='pending'`,
		"overlapping_active_subscriptions": `SELECT COUNT(*) FROM billing_subscriptions a JOIN billing_subscriptions b ON a.user_id=b.user_id AND a.id<b.id WHERE a.status='active' AND b.status='active' AND a.ends_at>? AND b.ends_at>?`,
		"cross_product_hash_collisions":    `SELECT COUNT(*) FROM billing_payments p JOIN commercial_topups_v2721 t ON LOWER(p.tx_hash)=LOWER(t.tx_hash)`,
	}
	for key, q := range queries {
		var n int64
		var err error
		if key == "overlapping_active_subscriptions" {
			now := time.Now().UTC().Format(time.RFC3339)
			err = a.db.QueryRow(q, now, now).Scan(&n)
		} else {
			err = a.db.QueryRow(q).Scan(&n)
		}
		if err != nil {
			http.Error(w, "Auditoría no disponible: "+key, 503)
			return
		}
		metrics[key] = n
	}
	writeJSON(w, map[string]any{"version": "27.28", "metrics": metrics, "pilot_enabled": billingLifecycleV2728Enabled(), "changes_plan_automatically": false, "verified_blockchain": false, "launch_approved": false, "note": "Las colisiones, solapamientos y pagos antiguos requieren análisis antes de habilitar venta pública."})
}

// Admin-granted courtesy must never replace a still-paid subscription with a
// shorter or different benefit without a separately reviewed compensation.
func billingCourtesyAllowedV2728(currentPlan, currentSource string, currentEnd time.Time, proposedPlan string, proposedEnd, now time.Time) error {
	if !proposedEnd.After(now) {
		return errors.New("cortesía con vencimiento inválido")
	}
	if currentEnd.After(now) && currentPlan != "" && currentSource != "courtesy" && currentPlan != "free" {
		// Replacing a paid subscription with a courtesy, even at the SAME plan
		// and duration, severs billing_payments.subscription_id and can make
		// refunds leave paid entitlements active. Block outright.
		return errors.New("una cortesía no puede reemplazar una suscripción pagada activa; requiere revisión financiera independiente")
	}
	return nil
}
