package main

// V27.30: tenant isolation of legacy endpoints and transaction-safe channel
// creation. Staging-only, off by default. NEVER gates signed inbound webhooks;
// they must be authenticated/tenant-resolved in their respective handlers.
import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

func tenantHardeningV2730Enabled() bool {
	return commercialEnabled() && os.Getenv("WORKTIC_TENANT_HARDENING_V2730") == "1"
}

// Only truly global legacy settings remain blocked. Inbox reads and metadata
// already filter by tenant_id; /api/send safely supports Cloud/Messenger.
// These handlers use global settings or global WhatsApp session/state. They
// cannot be promised as per-tenant features even if /api/channels/* is gated.
// Blocking only when staging hardening is on prevents any silent leakage.
var tenantLegacyGlobalEndpointsV2730 = map[string]string{
	"/api/connect":    "Usa Canales > Conexiones para conectar WhatsApp por empresa",
	"/api/disconnect": "Usa Canales > Conexiones para desconectar WhatsApp por empresa",
	"/api/qr":         "Usa el código QR de la conexión empresarial",
	"/api/logout":     "La sesión global de WhatsApp no está aislada por empresa",
	"/api/telegram":   "Usa Canales > Conexiones para administrar Telegram por empresa",
	"/api/messenger":  "Usa Canales > Conexiones para administrar Messenger por empresa",
}

// A single flag is not enough: legacy and modern writers must share one
// consistent contract and quota policy. Fail closed on tenant-specific routes
// when staging operators forget to enable a dependency.
func tenantHardeningDependenciesV2730() bool {
	return commercialModuleGateEnabledV2722() && resourceGuardV2724Enabled() && tenantRulesV2723Enabled() && contractV2729Enabled()
}

func tenantLegacyRouteDecisionV2730(path string, tenantRulesEnabled bool) (bool, string) {
	if path == "/api/rules" && !tenantRulesEnabled {
		return true, "Primero habilita reglas aisladas por empresa; la tabla heredada es global"
	}
	msg, legacy := tenantLegacyGlobalEndpointsV2730[path]
	return legacy, msg
}

func (a *App) tenantRouteGuardV2730(w http.ResponseWriter, r *http.Request) bool {
	if !tenantHardeningV2730Enabled() {
		return true
	}
	if !tenantHardeningDependenciesV2730() && commercialModuleForPathV2722(r.URL.Path) != "" {
		writeError(w, errors.New("endurecimiento multiempresa incompleto: activa permisos, recursos, reglas y contratos en staging"), http.StatusServiceUnavailable)
		return false
	}
	blocked, msg := tenantLegacyRouteDecisionV2730(r.URL.Path, tenantRulesV2723Enabled())
	if !blocked {
		return true
	}
	writeError(w, errors.New("Ruta heredada no disponible en modo multiempresa. "+msg), http.StatusForbidden)
	return false
}

func (a *App) tenantEffectivePlanV2730(tid int64) (Plan, error) {
	p, err := a.tenantPlanV2723(tid)
	if err != nil {
		return Plan{}, err
	}
	if tenantHardeningV2730Enabled() && contractV2729Enabled() && p.Code != "free" {
		contract, e := a.contractForTenantV2729(tid)
		if e != nil {
			return Plan{}, fmt.Errorf("derechos contractuales sin verificar: %w", e)
		}
		if contract.Plan.Code != p.Code {
			return Plan{}, errors.New("plan y contrato no coinciden")
		}
		return contract.Plan, nil
	}
	return p, nil
}

// Every connection writer must use the same per-tenant lock in hardened mode.
// This makes total/per-type checks and INSERT one atomic transaction.
func (a *App) createTenantChannelV2730(tid, creator int64, kind, name, config, publicID string, agentID int64, maxTotal, maxType int, now string) (int64, error) {
	if tid <= 0 || creator <= 0 || maxTotal <= 0 || maxType <= 0 {
		return 0, errors.New("canal no incluido en el plan")
	}
	switch kind {
	case "whatsapp_qr", "whatsapp_cloud", "telegram", "messenger":
	default:
		return 0, errors.New("tipo de canal desconocido")
	}
	if strings.TrimSpace(publicID) == "" || strings.TrimSpace(now) == "" {
		return 0, errors.New("referencia de canal inválida")
	}
	tx, err := a.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if err = lockConsumptionTenantV2723(tx, a.db.IsPostgres(), tid); err != nil {
		return 0, err
	}
	var total, byType int
	if err = tx.QueryRow(`SELECT COUNT(*) FROM channel_connections WHERE tenant_id=? AND status<>'deleted'`, tid).Scan(&total); err != nil {
		return 0, err
	}
	if err = tx.QueryRow(`SELECT COUNT(*) FROM channel_connections WHERE tenant_id=? AND type=? AND status<>'deleted'`, tid, kind).Scan(&byType); err != nil {
		return 0, err
	}
	var webTotal int
	if err = tx.QueryRow(`SELECT COUNT(*) FROM mobile_web_channels_v1 WHERE tenant_id=?`, tid).Scan(&webTotal); err != nil {
		return 0, err
	}
	total += webTotal
	if total >= maxTotal {
		return 0, fmt.Errorf("tu plan admite como máximo %d conexiones", maxTotal)
	}
	if byType >= maxType {
		return 0, fmt.Errorf("cupo de conexiones %s agotado (%d)", kind, maxType)
	}
	result, err := tx.Exec(`INSERT INTO channel_connections(tenant_id,public_id,type,name,status,assigned_agent_id,config_json,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?)`, tid, publicID, kind, name, "draft", agentID, config, now, now)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	if _, err = tx.Exec(`INSERT INTO channel_audit(tenant_id,connection_id,user_id,action,detail,created_at) VALUES(?,?,?,?,?,?)`, tid, id, creator, "create", kind, now); err != nil {
		return 0, err
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

// Called only while holding a per-tenant DB lock, including on webhook imports
// and previously deleted contacts restored by a new real interaction.
func crmTenantQuotaTxV2730(tx *Tx, tid int64, maxContacts int) error {
	if maxContacts <= 0 {
		return errors.New("no quedan contactos disponibles en este plan")
	}
	var used int
	err := tx.QueryRow(`SELECT COUNT(*) FROM crm_contacts WHERE tenant_id=? AND COALESCE(deleted_at,'')=''`, tid).Scan(&used)
	if err != nil {
		return err
	}
	if used >= maxContacts {
		return fmt.Errorf("cupo de CRM agotado (%d contactos)", maxContacts)
	}
	return nil
}

func tenantGuardReportV2730() map[string]any {
	return map[string]any{
		"version": "27.30", "enabled": tenantHardeningV2730Enabled(),
		"legacy_endpoints_blocked_when_enabled": len(tenantLegacyGlobalEndpointsV2730),
		"rules_mode_ready":                      tenantRulesV2723Enabled(),
		"dependencies_ready":                    tenantHardeningDependenciesV2730(),
		"contract_snapshots_ready":              contractV2729Enabled(),
		"financial_clearance":                   false, "production_approved": false,
		"remaining_manual_evidence": []string{
			"PostgreSQL real: 2 tenants + concurrent channel/CRM imports",
			"QA and UI migration from /api/send and global legacy settings",
			"WhatsApp/Meta callbacks: signature, tenant resolution, retry, paid units",
			"Contractual limits on non-channel resources + automated backfills",
			"Workers/scheduled jobs + provider reconciliation + complete route inventory",
		},
		"checked_at": time.Now().UTC().Format(time.RFC3339),
	}
}

func (a *App) tenantGuardReportV2730Handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "método no permitido", 405)
		return
	}
	if _, ok := a.requireSuperadmin(w, r); !ok {
		return
	}
	writeJSON(w, tenantGuardReportV2730())
}
