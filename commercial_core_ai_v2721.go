package main

// Core AI usage metering for V27.21. Independent from purchased Studio credits.
// Opt-in only. Every provider call must have a tenant identity or be explicitly
// marked as a Studio call already protected by its own credit reservation.
import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

func commercialCoreCap(code string) int64 {
	switch strings.ToLower(strings.TrimSpace(code)) {
	case "personal":
		return 150
	case "business", "negocio":
		return 500
	case "enterprise", "empresa":
		return 1500
	default:
		return 0
	}
}
func commercialCoreCostCents() int64 {
	return creativePositiveQuote("WORKTIC_COST_CORE_RESPONSE_CENTS", 1)
}
func commercialCoreCapForPlan(code string, existing int64) int64 {
	cap := commercialCoreCap(code)
	if existing >= 0 && existing < cap {
		cap = existing
	}
	return cap
}
func initCommercialCoreSchema(db *DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS commercial_core_usage_v2721(
 tenant_id INTEGER NOT NULL,period TEXT NOT NULL,limit_responses INTEGER NOT NULL DEFAULT 0,
 held INTEGER NOT NULL DEFAULT 0,used INTEGER NOT NULL DEFAULT 0,
 PRIMARY KEY(tenant_id,period))`)
	return err
}
func (a *App) commercialCoreReserveTx(tx *Tx, hold *creativeHold, plan string) error {
	if !commercialEnabled() {
		return nil
	}
	// Honor BOTH the commercial pilot cap and the active billing-plan cap.
	// Previously the wallet displayed min(billing, pilot), but this writer could
	// reserve up to pilot cap and deliver more responses than the customer bought.
	var billingCap int64
	if err := tx.QueryRow(`SELECT max_ai_responses FROM billing_plans WHERE code=? AND active=1`, plan).Scan(&billingCap); err != nil {
		return errors.New("no se pudo verificar el límite de IA contratado")
	}
	if contractV2729Enabled() && plan != "free" {
		var owner int64
		if e := tx.QueryRow(`SELECT owner_user_id FROM tenants WHERE id=?`, hold.TenantID).Scan(&owner); e != nil {
			return e
		}
		contract, e := a.contractForOwnerTxV2729(tx, owner)
		if e != nil {
			return errors.New("cupo de respuestas sin contrato verificado")
		}
		billingCap = int64(contract.Plan.MaxAIResponses)
	}
	cap := commercialCoreCapForPlan(plan, billingCap)
	// The purchased Studio wallet never extends core AI response allowances.
	if cap <= 0 {
		return errors.New("IA conversacional no incluida en este plan")
	}
	_, err := tx.Exec(`INSERT INTO commercial_core_usage_v2721(tenant_id,period,limit_responses,held,used) VALUES(?,?,?,0,0)
 ON CONFLICT(tenant_id,period) DO UPDATE SET limit_responses=excluded.limit_responses`, hold.TenantID, hold.Period, cap)
	if err != nil {
		return err
	}
	// Existing billing counters may already have consumed responses earlier
	// in the same month, before this pilot was enabled. Never grant a fresh
	// full allocation on top of the legacy usage; MAX avoids double charging
	// calls already mirrored in usage_monthly by commercialCoreSettleTx.
	var existingUsed int64
	if err = tx.QueryRow(`SELECT COALESCE(MAX(ai_responses),0) FROM usage_monthly
 WHERE user_id=(SELECT owner_user_id FROM tenants WHERE id=?) AND period=?`, hold.TenantID, hold.Period).Scan(&existingUsed); err != nil {
		return errors.New("no se pudo conciliar uso de IA del mes")
	}
	if existingUsed > 0 {
		if _, err = tx.Exec(`UPDATE commercial_core_usage_v2721
 SET used=CASE WHEN used<? THEN ? ELSE used END WHERE tenant_id=? AND period=?`, existingUsed, existingUsed, hold.TenantID, hold.Period); err != nil {
			return err
		}
	}
	r, err := tx.Exec(`UPDATE commercial_core_usage_v2721 SET held=held+1
 WHERE tenant_id=? AND period=? AND used+held+1<=limit_responses`, hold.TenantID, hold.Period)
	if err != nil {
		return err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return errors.New("se agotó el cupo mensual de respuestas IA del plan")
	}
	return nil
}
func (a *App) commercialCoreSettleTx(tx *Tx, hold *creativeHold, attempted bool) error {
	if !commercialEnabled() {
		return nil
	}
	spent := int64(0)
	if attempted {
		spent = 1
	}
	r, err := tx.Exec(`UPDATE commercial_core_usage_v2721 SET held=held-1,used=used+?
 WHERE tenant_id=? AND period=? AND held>=1`, spent, hold.TenantID, hold.Period)
	if err != nil {
		return err
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return errors.New("reserva de IA conversacional incoherente")
	}
	if attempted {
		var owner int64
		if err = tx.QueryRow(`SELECT owner_user_id FROM tenants WHERE id=?`, hold.TenantID).Scan(&owner); err != nil {
			return err
		}
		if owner <= 0 {
			return errors.New("titular de facturación inexistente")
		}
		_, err = tx.Exec(`INSERT INTO usage_monthly(user_id,period,ai_responses) VALUES(?,?,1)
 ON CONFLICT(user_id,period) DO UPDATE SET ai_responses=usage_monthly.ai_responses+1`, owner, hold.Period)
	}
	return err
}
func (a *App) commercialCoreBegin(tid int64) (*creativeHold, error) {
	if !commercialEnabled() {
		return nil, nil
	}
	if tid <= 0 {
		return nil, errors.New("empresa requerida para IA de pago")
	}
	var owner int64
	err := a.db.QueryRow(`SELECT owner_user_id FROM tenants WHERE id=?`, tid).Scan(&owner)
	if err != nil || owner <= 0 {
		return nil, errors.New("titular de empresa no disponible")
	}
	plan, _, err := a.activePlan(owner)
	if err != nil {
		return nil, err
	}
	if commercialCoreCapForPlan(plan.Code, int64(plan.MaxAIResponses)) <= 0 {
		return nil, errors.New("IA conversacional no incluida")
	}
	return a.creativeReserve(tid, owner, plan.Code, "core_ai", "openai", commercialCoreCostCents(), 0, 0)
}
func commercialCoreModel(configured string) string {
	if !commercialEnabled() {
		return configured
	}
	// Controlled model and output cap for an upper-bound pricing estimate.
	model := strings.TrimSpace(os.Getenv("WORKTIC_CORE_MODEL"))
	if model == "" {
		model = "gpt-4o-mini"
	}
	// Only audited cost model supported in pilot; do not allow arbitrary more expensive models.
	if model != "gpt-4o-mini" {
		return "gpt-4o-mini"
	}
	return model
}
func commercialCoreInput(system, user string) (string, string, error) {
	if !commercialEnabled() {
		return system, user, nil
	}
	if len(system)+len(user) > 12000 {
		return "", "", fmt.Errorf("solicitud supera el máximo de 12.000 caracteres")
	}
	return system, user, nil
}

// Central quota is intentionally separate from commercial Studio credit ledger.
func commercialCoreMonthlyCapCode(code string) int { return int(commercialCoreCap(code)) }
func commercialCoreMonth() string                  { return time.Now().UTC().Format("2006-01") }
func commercialCoreMaxOutput() int {
	if !commercialEnabled() {
		return 0
	}
	v, _ := strconv.Atoi(os.Getenv("WORKTIC_CORE_MAX_OUTPUT_TOKENS"))
	if v >= 128 && v <= 512 {
		return v
	}
	return 512
}

// Return a useful limit/payment explanation; do not expose upstream provider
// secrets or raw HTTP error payloads to the customer.
func commercialCoreUserError(err error, message string, ctx map[string]any) string {
	if err == nil {
		return copilotFallbackV274(message, ctx)
	}
	e := strings.ToLower(err.Error())
	if strings.Contains(e, "cupo mensual") || strings.Contains(e, "no incluida") {
		return "Alcanzaste el límite mensual de respuestas IA incluidas en tu plan. Puedes consultar el saldo en tu plan; el cupo se renueva el próximo mes."
	}
	if errors.Is(err, errCreativeBudget) || strings.Contains(e, "presupuesto") {
		return "Esta empresa alcanzó su presupuesto de generación IA o está temporalmente bloqueado para proteger sus costos. Contacta al administrador."
	}
	return copilotFallbackV274(message, ctx)
}
