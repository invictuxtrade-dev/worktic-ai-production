package main

// V27.31: STRICTLY READ-ONLY model. Financial scenarios are neither invoices
// nor approved margins. No FX rate, provider tariff, or infrastructure price is
// silently assumed. Costs in USD cents, commercial list prices in USDT.
import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type unitAssumptionsV2731 struct {
	RateUSDPerUSDT    float64 `json:"usd_per_usdt"`
	RateSource        string  `json:"fx_source"`
	RateDate          string  `json:"fx_asof"`
	AnnualDiscountBPS int64   `json:"annual_discount_bps"`
	PartnersBPS       int64   `json:"partners_subscription_bps"`
	TopupPartnersBPS  int64   `json:"partners_topup_bps"`
	PaymentBPS        int64   `json:"payment_processing_bps"`
	RiskBPS           int64   `json:"variable_cost_buffer_bps"`
	TargetMarginBPS   int64   `json:"target_margin_bps"`
	CreditCostCents   int64   `json:"estimated_max_supplier_cost_per_credit_cents"`
	ResponseCostCents int64   `json:"estimated_max_supplier_cost_per_response_cents"`
	InfraCents        int64   `json:"estimated_infra_per_tenant_month_cents"`
	OtherCents        int64   `json:"estimated_other_per_tenant_month_cents"`
}

type unitResultV2731 struct {
	Code                   string  `json:"code"`
	Cycle                  string  `json:"cycle"`
	ListPriceUSDT          float64 `json:"list_price_usdt_monthly"`
	RevenueUSDCents        int64   `json:"modeled_monthly_revenue_usd_cents"`
	PartnersUSDCents       int64   `json:"modeled_partners_usd_cents"`
	ProcessingUSDCents     int64   `json:"modeled_processing_usd_cents"`
	CreditCostUSDCents     int64   `json:"modeled_included_credit_cost_usd_cents"`
	ResponseCostUSDCents   int64   `json:"modeled_response_cost_usd_cents"`
	RiskReserveUSDCents    int64   `json:"modeled_variable_cost_buffer_usd_cents"`
	InfrastructureUSDCents int64   `json:"modeled_infrastructure_usd_cents"`
	OtherUSDCents          int64   `json:"modeled_other_costs_usd_cents"`
	ContributionUSDCents   int64   `json:"modeled_contribution_usd_cents"`
	MarginBPS              int64   `json:"modeled_margin_bps"`
	MeetsTargetUnderInputs bool    `json:"meets_target_under_assumptions"`
}

func parseCostInputV2731(q map[string][]string) (unitAssumptionsV2731, []string) {
	v := unitAssumptionsV2731{}
	missing := []string{}
	readI := func(key string, max int64, dest *int64) {
		s := strings.TrimSpace(firstInputV2731(q, key))
		x, e := strconv.ParseInt(s, 10, 64)
		if s == "" || e != nil || x < 0 || x > max {
			missing = append(missing, key)
			return
		}
		*dest = x
	}
	readI("annual_discount_bps", 9500, &v.AnnualDiscountBPS)
	readI("partners_subscription_bps", 5000, &v.PartnersBPS)
	readI("partners_topup_bps", 5000, &v.TopupPartnersBPS)
	readI("payment_processing_bps", 3000, &v.PaymentBPS)
	readI("variable_cost_buffer_bps", 10000, &v.RiskBPS)
	readI("target_margin_bps", 9900, &v.TargetMarginBPS)
	readI("max_supplier_credit_cents", 10000, &v.CreditCostCents)
	readI("max_supplier_response_cents", 10000, &v.ResponseCostCents)
	readI("infra_per_tenant_month_cents", 10000000, &v.InfraCents)
	readI("other_per_tenant_month_cents", 10000000, &v.OtherCents)
	rateS := strings.TrimSpace(firstInputV2731(q, "usd_per_usdt"))
	rate, e := strconv.ParseFloat(rateS, 64)
	if rateS == "" || e != nil || math.IsNaN(rate) || math.IsInf(rate, 0) || rate <= 0 || rate > 10 {
		missing = append(missing, "usd_per_usdt")
	} else {
		v.RateUSDPerUSDT = rate
	}
	v.RateSource = strings.TrimSpace(firstInputV2731(q, "fx_source"))
	if len(v.RateSource) < 3 || len(v.RateSource) > 100 {
		missing = append(missing, "fx_source")
	}
	v.RateDate = strings.TrimSpace(firstInputV2731(q, "fx_asof"))
	t, e := time.Parse("2006-01-02", v.RateDate)
	now := time.Now().UTC()
	if e != nil || t.After(now.Add(24*time.Hour)) || t.Before(now.AddDate(0, 0, -7)) {
		missing = append(missing, "fx_asof_recent_7_days")
	}
	return v, missing
}
func firstInputV2731(q map[string][]string, key string) string {
	x := q[key]
	if len(x) == 0 {
		return ""
	}
	return x[0]
}
func roundNonnegativeV2731(f float64) int64 {
	if f <= 0 {
		return 0
	}
	return int64(math.Round(f))
}
func partBPSV2731(value, bps int64) int64 { return (value*bps + 5000) / 10000 }

// Whole-month equivalent of a monthly or an annual prepaid contract. 100%
// worst-case use of all included quotas; not expected consumption/actual margin.
func modelPlanV2731(code, cycle string, monthlyUSDT float64, credits, responses int64, a unitAssumptionsV2731) (unitResultV2731, error) {
	v := unitResultV2731{Code: code, Cycle: cycle, ListPriceUSDT: monthlyUSDT}
	if cycle != "monthly" && cycle != "annual" {
		return v, fmt.Errorf("ciclo inválido")
	}
	if monthlyUSDT < 0 || monthlyUSDT > 1000000 || math.IsNaN(monthlyUSDT) || math.IsInf(monthlyUSDT, 0) || credits < 0 || credits > 10000000 || responses < 0 || responses > 10000000 || a.RateUSDPerUSDT <= 0 {
		return v, fmt.Errorf("datos no verificables")
	}
	price := monthlyUSDT
	if cycle == "annual" {
		price = price * float64(10000-a.AnnualDiscountBPS) / 10000
	}
	v.RevenueUSDCents = roundNonnegativeV2731(price * a.RateUSDPerUSDT * 100)
	v.PartnersUSDCents = partBPSV2731(v.RevenueUSDCents, a.PartnersBPS)
	v.ProcessingUSDCents = partBPSV2731(v.RevenueUSDCents, a.PaymentBPS)
	v.CreditCostUSDCents = credits * a.CreditCostCents
	v.ResponseCostUSDCents = responses * a.ResponseCostCents
	v.RiskReserveUSDCents = partBPSV2731(v.CreditCostUSDCents+v.ResponseCostUSDCents, a.RiskBPS)
	v.InfrastructureUSDCents = a.InfraCents
	v.OtherUSDCents = a.OtherCents
	v.ContributionUSDCents = v.RevenueUSDCents - v.PartnersUSDCents - v.ProcessingUSDCents - v.CreditCostUSDCents - v.ResponseCostUSDCents - v.RiskReserveUSDCents - v.InfrastructureUSDCents - v.OtherUSDCents
	if v.RevenueUSDCents > 0 {
		v.MarginBPS = int64(math.Round(float64(v.ContributionUSDCents) * 10000 / float64(v.RevenueUSDCents)))
	}
	v.MeetsTargetUnderInputs = v.RevenueUSDCents > 0 && v.MarginBPS >= a.TargetMarginBPS
	return v, nil
}
func modelPackV2731(code string, priceNominalUSDTcent, credits int64, a unitAssumptionsV2731) (map[string]any, error) {
	if priceNominalUSDTcent <= 0 || credits <= 0 || credits > 10000000 || priceNominalUSDTcent > 100000000 {
		return nil, fmt.Errorf("paquete inválido")
	}
	revenue := roundNonnegativeV2731(float64(priceNominalUSDTcent) * a.RateUSDPerUSDT)
	supplier := credits * a.CreditCostCents
	risk := partBPSV2731(supplier, a.RiskBPS)
	partners := partBPSV2731(revenue, a.TopupPartnersBPS)
	processing := partBPSV2731(revenue, a.PaymentBPS)
	margin := revenue - supplier - risk - partners - processing
	bps := int64(0)
	if revenue > 0 {
		bps = int64(math.Round(float64(margin) * 10000 / float64(revenue)))
	}
	return map[string]any{"code": code, "price_nominal_usdt_cents": priceNominalUSDTcent, "credits": credits, "modeled_monthly_equivalent": false, "revenue_usd_cents": revenue, "supplier_reserve_usd_cents": supplier, "cost_buffer_usd_cents": risk, "partners_usd_cents": partners, "processing_usd_cents": processing, "contribution_usd_cents": margin, "margin_bps": bps, "meets_target_under_assumptions": revenue > 0 && bps >= a.TargetMarginBPS, "prices_are_assumptions": true}, nil
}

// Strictly read-only; no pricing decisions, subscription changes or provider
// calls. Requires complete explicit inputs to model USD profitability.
func (a *App) unitEconomicsV2731Handler(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireSuperadmin(w, r); !ok {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "método no permitido", 405)
		return
	}
	period := financePeriod(r)
	ass, missing := parseCostInputV2731(r.URL.Query())
	var meterTotal, meterUnknown, meterUnfinished, invoiceCount, unallocated int64
	err := a.db.QueryRow(`SELECT COUNT(*),COALESCE(SUM(CASE WHEN status='completed' AND estimated_micro_usd IS NULL THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN status NOT IN ('completed','failed') THEN 1 ELSE 0 END),0) FROM finance_operation_meter_v2726 WHERE substr(created_at,1,7)=?`, period).Scan(&meterTotal, &meterUnknown, &meterUnfinished)
	if err != nil {
		http.Error(w, "error consultando medidores", 503)
		return
	}
	err = a.db.QueryRow(`SELECT COUNT(*),COALESCE(SUM(i.total_cents-COALESCE((SELECT SUM(x.actual_cents) FROM finance_actuals_v2725 x WHERE x.provider=i.provider AND x.invoice_ref=i.invoice_ref),0)),0) FROM finance_invoices_v2725 i WHERE i.period=?`, period).Scan(&invoiceCount, &unallocated)
	if err != nil {
		http.Error(w, "error consultando facturas", 503)
		return
	}
	type planRow struct {
		Code, Name         string
		Price              float64
		Credits, Responses int64
	}
	plans := []planRow{}
	rows, err := a.db.Query(`SELECT p.code,p.name,p.price_usdt,r.monthly_credits,p.max_ai_responses FROM billing_plans p JOIN commercial_rules_v2721 r ON r.plan_code=p.code WHERE p.active=1 ORDER BY p.price_usdt,p.code`)
	if err != nil {
		http.Error(w, "planes comerciales no disponibles", 503)
		return
	}
	for rows.Next() {
		var x planRow
		if e := rows.Scan(&x.Code, &x.Name, &x.Price, &x.Credits, &x.Responses); e != nil {
			rows.Close()
			http.Error(w, "error leyendo planes", 503)
			return
		}
		plans = append(plans, x)
	}
	e := rows.Err()
	rows.Close()
	if e != nil {
		http.Error(w, "planes incompletos", 503)
		return
	}
	packages := []struct {
		Code           string
		Credits, Price int64
	}{}
	rows, err = a.db.Query(`SELECT code,credits,price_cents FROM commercial_packages_v2721 WHERE active=1 ORDER BY price_cents`)
	if err != nil {
		http.Error(w, "paquetes no disponibles", 503)
		return
	}
	for rows.Next() {
		var x struct {
			Code           string
			Credits, Price int64
		}
		if e := rows.Scan(&x.Code, &x.Credits, &x.Price); e != nil {
			rows.Close()
			http.Error(w, "error leyendo paquetes", 503)
			return
		}
		packages = append(packages, x)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		http.Error(w, "paquetes incompletos", 503)
		return
	}
	result := map[string]any{"period": period, "currency_costs": "USD", "currency_sales": "USDT", "fx_rate_status": "requires_manual_verified_source", "public_launch_approved": false, "supplier_metered_operations": meterTotal, "operations_with_unknown_supplier_price": meterUnknown, "unsettled_provider_operations": meterUnfinished, "invoices_registered": invoiceCount, "verified_invoice_usd_cents_unallocated": unallocated, "source": "current billing_plans, commercial_rules and effective pilot response caps; not historical contract snapshots", "note": "ESCENARIOS, NO RENTABILIDAD REAL. Faltan costos observados y pruebas en staging."}
	if len(missing) > 0 {
		result["status"] = "missing_required_financial_inputs"
		result["missing_inputs"] = missing
		writeJSON(w, result)
		return
	}
	sc := []unitResultV2731{}
	for _, p := range plans {
		for _, cycle := range []string{"monthly", "annual"} {
			v, e := modelPlanV2731(p.Code, cycle, p.Price, p.Credits, commercialCoreCapForPlan(p.Code, p.Responses), ass)
			if e != nil {
				http.Error(w, "plan incompatible", 409)
				return
			}
			sc = append(sc, v)
		}
	}
	packs := []map[string]any{}
	for _, p := range packages {
		v, e := modelPackV2731(p.Code, p.Price, p.Credits, ass)
		if e != nil {
			http.Error(w, "paquete incompatible", 409)
			return
		}
		packs = append(packs, v)
	}
	result["status"] = "scenario_only_not_verified"
	result["assumptions"] = ass
	result["plan_scenarios"] = sc
	result["credit_packages"] = packs
	result["known_unattributed_costs_block_launch"] = unallocated > 0
	result["unknown_supplier_pricing_blocks_launch"] = meterUnknown > 0
	result["fx_rate_status"] = "user_supplied_scenario_only"
	result["unverified_cost_categories"] = []string{"OpenAI billed tokens/images/voice by model", "Luma billed generation and refunds", "Meta category and destination pricing", "hosting CPU RAM and egress", "Redis and PostgreSQL usage", "Partner commissions, credits and chargebacks", "tax and payment processors", "tenant-specific service and support"}
	writeJSON(w, result)
}
