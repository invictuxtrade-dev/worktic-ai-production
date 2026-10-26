package main

import (
	"math"
	"net/url"
	"strconv"
	"testing"
	"time"
)

func testEconomicsAssumptionsV2731() unitAssumptionsV2731 {
	return unitAssumptionsV2731{RateUSDPerUSDT: 1, AnnualDiscountBPS: 2000, PartnersBPS: 5000, TopupPartnersBPS: 1000, PaymentBPS: 300, RiskBPS: 2000, TargetMarginBPS: 2000, CreditCostCents: 3, ResponseCostCents: 1, InfraCents: 500, OtherCents: 200}
}
func TestFinanceV2731SubscriptionScenarios(t *testing.T) {
	a := testEconomicsAssumptionsV2731()
	annual, e := modelPlanV2731("business", "annual", 75, 120, 500, a)
	if e != nil {
		t.Fatal(e)
	}
	if annual.RevenueUSDCents != 6000 || annual.PartnersUSDCents != 3000 || annual.ProcessingUSDCents != 180 || annual.CreditCostUSDCents != 360 || annual.ResponseCostUSDCents != 500 || annual.RiskReserveUSDCents != 172 || annual.ContributionUSDCents != 1088 {
		t.Fatalf("unbalanced: %+v", annual)
	}
	if annual.MeetsTargetUnderInputs {
		t.Fatal("annual margin should fail 20% target")
	}
	monthly, e := modelPlanV2731("business", "monthly", 75, 120, 500, a)
	if e != nil || monthly.RevenueUSDCents != 7500 || !monthly.MeetsTargetUnderInputs {
		t.Fatalf("monthly unexpected: %+v / %v", monthly, e)
	}
	for _, period := range []string{"annually", "", "coupon"} {
		if _, e := modelPlanV2731("business", period, 75, 120, 500, a); e == nil {
			t.Fatalf("invalid period %s", period)
		}
	}
	for _, price := range []float64{-1, math.NaN(), math.Inf(1)} {
		if _, e := modelPlanV2731("business", "annual", price, 120, 500, a); e == nil {
			t.Fatalf("invalid price %v", price)
		}
	}
}
func TestFinanceV2731PackageMargins(t *testing.T) {
	a := testEconomicsAssumptionsV2731()
	pkg, e := modelPackV2731("creator", 1000, 100, a)
	if e != nil {
		t.Fatal(e)
	}
	if pkg["contribution_usd_cents"].(int64) != 510 {
		t.Fatalf("margin %+v", pkg)
	}
	a.CreditCostCents = 20
	pkg, e = modelPackV2731("creator", 1000, 100, a)
	if e != nil || pkg["meets_target_under_assumptions"].(bool) {
		t.Fatalf("expensive credits should fail %+v %v", pkg, e)
	}
}
func TestFinanceV2731NoSilentFXOrCosts(t *testing.T) {
	q := url.Values{}
	_, missing := parseCostInputV2731(q)
	if len(missing) < 10 {
		t.Fatalf("missing inputs not reported: %+v", missing)
	}
	a := testEconomicsAssumptionsV2731()
	all := map[string]int64{"annual_discount_bps": a.AnnualDiscountBPS, "partners_subscription_bps": a.PartnersBPS, "partners_topup_bps": a.TopupPartnersBPS, "payment_processing_bps": a.PaymentBPS, "variable_cost_buffer_bps": a.RiskBPS, "target_margin_bps": a.TargetMarginBPS, "max_supplier_credit_cents": a.CreditCostCents, "max_supplier_response_cents": a.ResponseCostCents, "infra_per_tenant_month_cents": a.InfraCents, "other_per_tenant_month_cents": a.OtherCents}
	for k, v := range all {
		q.Set(k, strconv.FormatInt(v, 10))
	}
	q.Set("usd_per_usdt", "1.0000")
	q.Set("fx_source", "quoted exchange")
	q.Set("fx_asof", time.Now().UTC().Format("2006-01-02"))
	_, missing = parseCostInputV2731(q)
	if len(missing) > 0 {
		t.Fatalf("all supplied but missing %+v", missing)
	}
	q.Set("partners_subscription_bps", "5001")
	_, missing = parseCostInputV2731(q)
	if len(missing) == 0 {
		t.Fatal("partners over 50% was accepted")
	}
	q.Set("partners_subscription_bps", "5000")
	q.Set("usd_per_usdt", "NaN")
	_, missing = parseCostInputV2731(q)
	if len(missing) == 0 {
		t.Fatal("NaN FX was accepted")
	}
}
