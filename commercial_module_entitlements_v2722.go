package main

// V27.22 staged, opt-in module entitlements. A plan-specific feature check
// happens AFTER session authentication and BEFORE the routed handler.
// Does not replace per-resource quotas, row-level tenancy, provider budgets,
// subscription validation, or Worktic Partners' independent authorization.
import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"sort"
	"strings"
)

type commercialModuleV2722 struct {
	Key        string `json:"key"`
	Name       string `json:"name"`
	MinPlan    string `json:"min_plan"`
	Metering   string `json:"metering"`
	CostDriver string `json:"cost_driver"`
	State      string `json:"state"`
}

// State deliberately describes EXISTING enforcement, not a promise of full
// coverage. A module may be gated while its quota and vendor-cost integration
// still require additional verification.
var commercialModuleCatalogV2722 = []commercialModuleV2722{
	{"crm", "CRM y oportunidades", "free", "contactos", "PostgreSQL/soporte", "gate+nivel; revisar cuotas"},
	{"catalog", "Catálogo", "free", "productos", "almacenamiento/medios", "gate+nivel; revisar cuotas"},
	{"agenda", "Agenda", "free", "citas", "DB/notificaciones", "gate; cupo por auditar"},
	{"inbox", "Inbox omnicanal", "free", "canales", "Meta/WhatsApp/DB", "gate; tarifa de mensajería por auditar"},
	{"channels", "Conexiones de canales", "free", "canales por proveedor", "Meta/TG/WhatsApp", "gate; cuotas históricas"},
	{"marketing", "Marketing y campañas", "free", "campañas/formularios", "DB/IA", "gate; cuotas existentes"},
	{"landings", "Landing pages", "free", "formularios/páginas", "storage/transferencia", "gate; cuotas existentes"},
	{"social", "Social Hub", "free", "publicaciones", "API redes/almacenamiento", "gate; publicación/cuota por auditar"},
	{"studio", "Worktic Studio", "free", "créditos/proyectos", "OpenAI/Luma/almacenamiento", "gate + billetera V27.21"},
	{"agents", "Agentes IA avanzados", "personal", "respuestas IA", "OpenAI", "gate + cupo Core V27.21"},
	{"automations", "Workflows avanzados", "personal", "reglas/ejecuciones", "OpenAI/webhooks", "gate; cuota ejecuciones por auditar"},
	{"analytics", "Analytics y Attribution", "personal", "eventos/retención", "PostgreSQL/transferencia", "gate; volumen por auditar"},
	{"social_governance", "Social Governance/Autopilot", "business", "aprobaciones/repurpose", "API/IA", "gate; fanout por auditar"},
	{"ads", "Ads Center", "business", "cuentas/sincronizaciones", "Google/Meta/TikTok", "gate; proveedores y API externos"},
	{"meta_leads", "Meta Lead Ads", "business", "formularios/leads", "Meta/DB", "gate; cuota por auditar"},
	{"groups", "Grupos y prospección", "business", "grupos/envíos", "WhatsApp/red", "gate; cumplimiento y rate-limit por auditar"},
	{"whatsapp_marketing", "WhatsApp Marketing", "business", "campañas/mensajes", "Meta mensajes", "gate; costo real por país/template pendiente"},
}

// Match exact API boundaries. Webhooks, public lead forms, OAuth callbacks,
// platform admin and Partners have their own access/payment rules. Do NOT
// gate those routes using a user's subscription from an unauthenticated hook.
func commercialModuleForPathV2722(path string) string {
	switch {
	case strings.HasPrefix(path, "/api/ads/v25/"):
		return "ads"
	case strings.HasPrefix(path, "/api/analytics/v24/"):
		return "analytics"
	case strings.HasPrefix(path, "/api/social/approvals"), strings.HasPrefix(path, "/api/social/governance"), strings.HasPrefix(path, "/api/social/repurpose"), strings.HasPrefix(path, "/api/social/community"):
		return "social_governance"
	case strings.HasPrefix(path, "/api/social/"):
		return "social"
	case strings.HasPrefix(path, "/api/automations/"):
		return "automations"
	case strings.HasPrefix(path, "/api/agents/"):
		return "agents"
	case strings.HasPrefix(path, "/api/whatsapp-marketing/"):
		return "whatsapp_marketing"
	case path == "/api/rules", strings.HasPrefix(path, "/api/automations"):
		return "automations"
	case path == "/api/telegram", path == "/api/messenger", path == "/api/connect", path == "/api/disconnect", path == "/api/qr", path == "/api/logout":
		return "channels"
	case path == "/api/messages", path == "/api/conversations", path == "/api/read", path == "/api/contact", path == "/api/send":
		return "inbox"
	case strings.HasPrefix(path, "/api/whatsapp/embedded/"):
		return "channels"
	case strings.HasPrefix(path, "/api/groups"):
		return "groups"
	case strings.HasPrefix(path, "/api/marketing/meta-leads/"):
		return "meta_leads"
	case strings.HasPrefix(path, "/api/marketing/landings"):
		return "landings"
	case strings.HasPrefix(path, "/api/marketing/"):
		return "marketing"
	case strings.HasPrefix(path, "/api/crm/"):
		return "crm"
	case path == "/api/products", path == "/api/catalog/upload":
		return "catalog"
	case strings.HasPrefix(path, "/api/agenda/"), path == "/api/appointments":
		return "agenda"
	case strings.HasPrefix(path, "/api/inbox/"):
		return "inbox"
	case strings.HasPrefix(path, "/api/studio/"):
		return "studio"
	case strings.HasPrefix(path, "/api/channels/"), strings.HasPrefix(path, "/api/whatsapp/business/"), strings.HasPrefix(path, "/api/whatsapp/templates/"):
		return "channels"
	}
	return "" // unclassified endpoints remain visible in static inventory.
}

func commercialModuleGateEnabledV2722() bool {
	return commercialEnabled() && os.Getenv("WORKTIC_MODULE_GATES_V2722") == "1"
}
func commercialPlanTierV2722(code string) int {
	switch strings.ToLower(strings.TrimSpace(code)) {
	case "personal":
		return 1
	case "business", "negocio":
		return 2
	case "enterprise", "empresa":
		return 3
	default:
		return 0
	}
}
func commercialModuleAllowedV2722(plan, module, overridesJSON string) bool {
	if module == "" {
		return true
	}
	need := -1
	for _, m := range commercialModuleCatalogV2722 {
		if m.Key == module {
			need = commercialPlanTierV2722(m.MinPlan)
			break
		}
	}
	if need == -1 {
		return false
	}
	allowed := commercialPlanTierV2722(plan) >= need
	if strings.TrimSpace(overridesJSON) != "" {
		var flags map[string]json.RawMessage
		if json.Unmarshal([]byte(overridesJSON), &flags) == nil {
			if v, ok := flags[module]; ok {
				var b bool
				if json.Unmarshal(v, &b) == nil {
					allowed = b
				}
			}
		}
	}
	return allowed
}
func (a *App) commercialModulesForUserV2722(u *User) (string, string, error) {
	if u == nil || u.TenantID <= 0 {
		return "", "", errors.New("empresa autenticada requerida")
	}
	owner := a.billingAccountUserID(u)
	p, sub, err := a.activePlan(owner)
	if err != nil {
		return "", "", err
	}
	flags := "{}"
	if contractV2729Enabled() && p.Code != "free" {
		contract, _, e := a.contractReadV2729(sub.ID)
		if e != nil {
			return "", "", e
		}
		return p.Code, contract.ModulesJSON, nil
	}
	if e := a.db.QueryRow(`SELECT COALESCE(features_json,'{}') FROM billing_plans WHERE code=?`, p.Code).Scan(&flags); e != nil {
		return "", "", e // fail closed when enabled; no permissions if no configuration
	}
	return p.Code, flags, nil
}
func (a *App) commercialModuleGuardV2722(w http.ResponseWriter, r *http.Request) bool {
	if !commercialModuleGateEnabledV2722() {
		return true
	}
	m := commercialModuleForPathV2722(r.URL.Path)
	if m == "" {
		return true
	}
	u := a.currentUser(r)
	if u == nil {
		writeError(w, errors.New("sesión requerida"), http.StatusUnauthorized)
		return false
	}
	if u.Role == "superadmin" {
		return true
	}
	code, flags, err := a.commercialModulesForUserV2722(u)
	if err != nil {
		writeError(w, errors.New("no se pudo verificar tu plan comercial"), http.StatusServiceUnavailable)
		return false
	}
	if !commercialModuleAllowedV2722(code, m, flags) {
		writeError(w, errors.New("esta funcionalidad no está incluida en tu plan; consulta los beneficios de tu suscripción"), http.StatusForbidden)
		return false
	}
	return true
}
func (a *App) commercialFullEntitlementsV2722Handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, errors.New("método no permitido"), http.StatusMethodNotAllowed)
		return
	}
	u := a.currentUser(r)
	if u == nil {
		writeError(w, errors.New("sesión requerida"), http.StatusUnauthorized)
		return
	}
	code, flags, err := a.commercialModulesForUserV2722(u)
	if err != nil {
		writeError(w, errors.New("no se pudo consultar el plan"), http.StatusServiceUnavailable)
		return
	}
	catalog := make([]map[string]any, 0, len(commercialModuleCatalogV2722))
	for _, m := range commercialModuleCatalogV2722 {
		catalog = append(catalog, map[string]any{"key": m.Key, "name": m.Name, "allowed": commercialModuleAllowedV2722(code, m.Key, flags), "min_plan": m.MinPlan, "metering": m.Metering, "cost_driver": m.CostDriver, "audit_state": m.State})
	}
	sort.Slice(catalog, func(i, j int) bool { return catalog[i]["key"].(string) < catalog[j]["key"].(string) })
	writeJSON(w, map[string]any{"plan_code": code, "module_gates_active": commercialModuleGateEnabledV2722(), "commercial_active": commercialEnabled(), "modules": catalog, "note": "Los permisos no certifican límites de volumen ni costos externos; consulta el informe de auditoría V27.22."})
}
