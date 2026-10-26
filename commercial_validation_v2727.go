package main

// WorkticAI V27.27 — read-only, superadmin-only commercial preflight.
// This endpoint does NOT publish plans, charge providers, approve invoices,
// alter subscriptions, or certify legal/financial profitability.
import (
	"net/http"
	"os"
	"strings"
	"time"
)

type commercialPreflightCheckV2727 struct {
	Key    string `json:"key"`
	Module string `json:"module"`
	Status string `json:"status"` // pass, block, manual
	Detail string `json:"detail"`
}

func commercialPreflightChecksV2727(stage string, values map[string]string) []commercialPreflightCheckV2727 {
	result := []commercialPreflightCheckV2727{}
	add := func(key, module, status, detail string) {
		result = append(result, commercialPreflightCheckV2727{Key: key, Module: module, Status: status, Detail: detail})
	}
	if strings.EqualFold(stage, "production") {
		add("isolation", "Infraestructura", "block", "Se requiere staging independiente; nunca ejecutar pruebas destructivas sobre producción.")
	} else {
		add("isolation", "Infraestructura", "manual", "Verificar PostgreSQL/Redis y disco separados de producción; APP_ENV por sí solo no demuestra aislamiento.")
	}
	if values["WORKTIC_COMMERCIAL_ENABLE"] == "1" {
		add("sale_switch", "Suscripciones", "manual", "El piloto comercial está activo en este entorno. Confirmar que no sea producción.")
	} else {
		add("sale_switch", "Suscripciones", "pass", "Los cobros y créditos nuevos permanecen desactivados.")
	}
	for _, row := range []struct{ Key, Mod, Env string }{
		{"module_gates", "Permisos", "WORKTIC_MODULE_GATES_V2722"},
		{"resource_quotas", "CRM, Catálogo y Agenda", "WORKTIC_RESOURCE_GUARD_V2724"},
		{"automation_runs", "Automatizaciones", "WORKTIC_CONSUMPTION_GUARD_V2723"},
		{"operation_meter", "OpenAI y Luma", "WORKTIC_METER_V2726"},
		{"tenant_hardening", "WhatsApp / Canales / CRM multiempresa", "WORKTIC_TENANT_HARDENING_V2730"},
	} {
		if values[row.Env] == "1" {
			add(row.Key, row.Mod, "manual", "Control activado para staging; requiere E2E y revisión de todas las rutas automáticas.")
		} else {
			add(row.Key, row.Mod, "block", "Control de consumo/permisos aún desactivado; no ofrecer este módulo como completamente limitado.")
		}
	}
	if values["WORKTIC_FINANCE_V2725_WRITE"] == "1" {
		add("finance_write", "Contabilidad", "manual", "Carga de facturas activa. Validar origen y autorización de cada importe.")
	} else {
		add("finance_write", "Contabilidad", "pass", "Carga de costos reales desactivada para evitar documentos no verificados.")
	}
	if values["WORKTIC_BILLING_LIFECYCLE_V2728"] == "1" && values["WORKTIC_BILLING_V2725_SAFE_APPROVAL"] == "1" {
		add("lifecycle_v2728", "Recorridos comerciales", "manual", "Control de ciclo/renovación y referencia blockchain compartida activo en staging; probar cambios de plan y solicitudes concurrentes.")
	} else {
		add("lifecycle_v2728", "Recorridos comerciales", "block", "V27.28 inactivo; aún no está protegido el checkout antiguo frente a cambios de plan y reutilización transversal de transacciones.")
	}
	if values["WORKTIC_BILLING_V2725_SAFE_APPROVAL"] == "1" {
		add("billing_approval", "Pagos", "manual", "Aprobación segura piloto activa; probar mensualidad, anualidad, renovación, reversión y Partners.")
	} else {
		add("billing_approval", "Pagos", "block", "Flujo nuevo de pagos sin validación integral en staging.")
	}
	if values["WORKTIC_COST_GLOBAL_MONTHLY_CENTS"] == "" || values["WORKTIC_COST_GLOBAL_MONTHLY_CENTS"] == "0" {
		add("global_budget", "IA y multimedia", "pass", "Presupuesto global sin habilitar; llamadas externas protegidas deben bloquearse.")
	} else {
		add("global_budget", "IA y multimedia", "manual", "Presupuesto global configurado; comprobar que soporta compromisos de planes y recargas.")
	}
	for _, m := range []string{"WhatsApp / Meta por país y categoría", "Infraestructura Render/Redis/PostgreSQL", "Gastos de pauta Ads externos", "Partners hasta 50% y reversos", "Copilot, streaming y reintentos", "Pagos USDT, conciliación blockchain y FX", "Reglas heredadas y rutas no homologadas", "Pruebas PostgreSQL multiinstancia", "Facturas verificadas y margen de contribución"} {
		add("external_"+strings.NewReplacer(" ", "_", "/", "_").Replace(strings.ToLower(m)), m, "manual", "No certificado automáticamente; anexar evidencia de pruebas y de costos externos antes de autorizar venta.")
	}
	return result
}

func (a *App) commercialPreflightV2727Handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}
	if _, ok := a.requireSuperadmin(w, r); !ok {
		return
	}
	keys := []string{"WORKTIC_COMMERCIAL_ENABLE", "WORKTIC_MODULE_GATES_V2722", "WORKTIC_RESOURCE_GUARD_V2724", "WORKTIC_CONSUMPTION_GUARD_V2723", "WORKTIC_METER_V2726", "WORKTIC_FINANCE_V2725_WRITE", "WORKTIC_BILLING_V2725_SAFE_APPROVAL", "WORKTIC_BILLING_LIFECYCLE_V2728", "WORKTIC_COST_GLOBAL_MONTHLY_CENTS", "WORKTIC_TENANT_HARDENING_V2730"}
	values := map[string]string{}
	for _, k := range keys {
		values[k] = strings.TrimSpace(os.Getenv(k))
	}
	checks := commercialPreflightChecksV2727(a.cfg.AppEnv, values)
	add := func(key, mod, status, detail string) {
		checks = append(checks, commercialPreflightCheckV2727{Key: key, Module: mod, Status: status, Detail: detail})
	}
	planOK := true
	for _, plan := range commercialPlanSeed() {
		var price float64
		var active, users, contacts, channels, core int64
		err := a.db.QueryRow(`SELECT price_usdt,active,max_users,max_contacts,max_channels,max_ai_responses FROM billing_plans WHERE code=?`, plan.Plan).Scan(&price, &active, &users, &contacts, &channels, &core)
		if err != nil || active != 1 || users < 1 || contacts < 1 || channels < 1 || core < 0 || (plan.Plan != "free" && price <= 0) || (plan.Plan == "free" && price != 0) {
			add("billing_"+plan.Plan, "Planes", "block", "Falta un plan activo o alguno de sus límites/precios es incompatible; verificar billing_plans en staging.")
			planOK = false
		} else {
			add("billing_"+plan.Plan, "Planes", "pass", "Plan activo y cuotas básicas configuradas en base de datos; los precios deben contrastarse con la web y contratos.")
		}
		var monthly, projects, storage int64
		if e := a.db.QueryRow(`SELECT monthly_credits,max_projects,storage_mb FROM commercial_rules_v2721 WHERE plan_code=?`, plan.Plan).Scan(&monthly, &projects, &storage); e != nil || monthly < 0 || projects < 0 || storage < 0 || (plan.Plan == "free" && monthly != 0) {
			add("creative_rule_"+plan.Plan, "Studio", "block", "Regla comercial ausente o inválida en commercial_rules_v2721.")
			planOK = false
		} else {
			add("creative_rule_"+plan.Plan, "Studio", "pass", "La matriz tiene créditos y límites de proyectos/almacenamiento; quedan E2E por verificar.")
		}
	}
	var packages int64
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM commercial_packages_v2721 WHERE active=1 AND price_cents>0 AND credits>0`).Scan(&packages); err != nil || packages != 5 {
		add("packages", "Recargas", "block", "Los cinco paquetes válidos de créditos no coinciden con el catálogo piloto.")
	} else {
		add("packages", "Recargas", "pass", "Existen cinco paquetes activos y con precio; cobrar/validar pagos requiere E2E.")
	}
	var oldRules int64
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM worktic_auto_rules`).Scan(&oldRules); err != nil {
		add("legacy_rules", "Automatizaciones", "block", "No fue posible revisar reglas heredadas globales.")
	} else if oldRules > 0 {
		add("legacy_rules", "Automatizaciones", "block", "Existen reglas heredadas sin tenant_id; no asignar propietario automáticamente.")
	} else {
		add("legacy_rules", "Automatizaciones", "manual", "Sin filas legacy al consultar; comprobar que ninguna ruta global permita altas nuevas.")
	}
	var unverified int64
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM billing_payments WHERE billing_cycle='unverified' AND status='pending'`).Scan(&unverified); err != nil {
		add("unverified_payments", "Facturación", "block", "No fue posible consultar los pagos antiguos sin ciclo verificado.")
	} else if unverified > 0 {
		add("unverified_payments", "Facturación", "block", "Hay pagos pendientes sin ciclo mensual/anual verificado.")
	} else {
		add("unverified_payments", "Facturación", "pass", "No hay pagos pendientes con ciclo sin verificar.")
	}
	var unpriced int64
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM finance_operation_meter_v2726 WHERE pricing_status='unpriced'`).Scan(&unpriced); err != nil {
		add("unpriced_operations", "Costeo", "block", "No fue posible consultar operaciones sin tarifa.")
	} else if unpriced > 0 {
		add("unpriced_operations", "Costeo", "manual", "Se registraron operaciones sin tarifa comprobable; conciliar proveedor y modelo.")
	} else {
		add("unpriced_operations", "Costeo", "manual", "No hay operaciones unpriced en el medidor; cero filas no demuestra cobertura ni facturación real.")
	}
	var pass, block, manual int
	for _, c := range checks {
		switch c.Status {
		case "pass":
			pass++
		case "block":
			block++
		default:
			manual++
		}
	}
	writeJSON(w, map[string]any{
		"version": "27.27", "checked_at_utc": time.Now().UTC().Format(time.RFC3339), "checks": checks,
		"summary": map[string]int{"pass": pass, "block": block, "manual": manual}, "plans_basic_checks_passed": planOK,
		"public_launch_approved": false, "reads_only": true, "makes_vendor_calls": false,
		"note": "El preflight es evidencia técnica parcial. Para levantar bloqueos manuales se requieren pruebas E2E, facturas verificadas, control de pagos/Partners y aprobación financiera independiente. No activa ventas.",
	})
}
