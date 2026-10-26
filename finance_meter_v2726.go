package main

// V27.26: operational metering, not billing. Never stores prompts, phone
// numbers, API keys or payloads. NULL cost means unknown, not free.
import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type meterEventV2726 struct {
	Key, HoldID, Provider, Operation, Model, Status, ProviderRef, UnitKind string
	TenantID, InputTokens, OutputTokens, Units                             int64
	EstimatedMicroUSD                                                      *int64
}

func meterEnabledV2726() bool { return os.Getenv("WORKTIC_METER_V2726") == "1" }
func initMeterV2726Schema(db *DB) error {
	for _, q := range []string{
		`CREATE TABLE IF NOT EXISTS finance_operation_meter_v2726 (
  event_key TEXT PRIMARY KEY, tenant_id INTEGER NOT NULL, provider TEXT NOT NULL,
  operation TEXT NOT NULL, model TEXT NOT NULL, status TEXT NOT NULL,
  provider_ref TEXT NOT NULL DEFAULT '', hold_id TEXT NOT NULL DEFAULT '',
  input_tokens INTEGER NOT NULL DEFAULT 0, output_tokens INTEGER NOT NULL DEFAULT 0,
  units INTEGER NOT NULL DEFAULT 0, unit_kind TEXT NOT NULL DEFAULT '',
  estimated_micro_usd INTEGER, pricing_status TEXT NOT NULL, created_at TEXT NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS finance_meter_period_v2726 ON finance_operation_meter_v2726(tenant_id,created_at,provider)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS finance_meter_hold_v2726 ON finance_operation_meter_v2726(hold_id) WHERE hold_id<>''`,
	} {
		if _, err := db.Exec(q); err != nil {
			return fmt.Errorf("meter V27.26: %w", err)
		}
	}
	return nil
}
func meterHoldV2726(h *creativeHold) string {
	if h != nil {
		return h.ID
	}
	return ""
}
func meterKeyV2726(prefix string, h *creativeHold) string {
	if h != nil && h.ID != "" {
		return prefix + ":" + h.ID
	}
	id, err := creativeID()
	if err != nil {
		return ""
	}
	return prefix + ":" + id
}
func meterRateV2726(key string) (int64, bool) {
	s := strings.TrimSpace(os.Getenv(key))
	if s == "" {
		return 0, false
	}
	n, err := strconv.ParseInt(s, 10, 64)
	return n, err == nil && n >= 0 && n <= 10000000000
}

// Cost in micro USD, using explicitly configured micro USD per million tokens.
func meterTokenPriceV2726(model string, in, out int64) *int64 {
	if model == "" || model != strings.TrimSpace(os.Getenv("WORKTIC_METER_OPENAI_MODEL")) || in < 0 || out < 0 || in > 10000000 || out > 10000000 {
		return nil
	}
	ir, ok1 := meterRateV2726("WORKTIC_METER_OPENAI_INPUT_MICROUSD_PER_MILLION")
	or, ok2 := meterRateV2726("WORKTIC_METER_OPENAI_OUTPUT_MICROUSD_PER_MILLION")
	if !ok1 || !ok2 {
		return nil
	}
	p := (in*ir + out*or + 999999) / 1000000
	return &p
}
func meterFlatPriceV2726(op, model, detail string, units int64) *int64 {
	if units <= 0 || units > 10000000 {
		return nil
	}
	key := ""
	switch op {
	case "studio_image":
		if model != "" && model == strings.TrimSpace(os.Getenv("WORKTIC_METER_IMAGE_MODEL")) {
			switch detail {
			case "1024x1024":
				key = "WORKTIC_METER_IMAGE_1024_MICROUSD"
			case "1536x1024", "1024x1536":
				key = "WORKTIC_METER_IMAGE_LARGE_MICROUSD"
			}
		}
	case "studio_tts":
		if model != "" && model == strings.TrimSpace(os.Getenv("WORKTIC_METER_TTS_MODEL")) {
			key = "WORKTIC_METER_TTS_MICROUSD_PER_MILLION_CHARS"
		}
	case "studio_video":
		if model == "ray-3.2" && model == strings.TrimSpace(os.Getenv("WORKTIC_METER_VIDEO_MODEL")) && detail == "720p" {
			switch units {
			case 5:
				key = "WORKTIC_METER_VIDEO_5S_720P_MICROUSD"
			case 10:
				key = "WORKTIC_METER_VIDEO_10S_720P_MICROUSD"
			}
		}
	}
	if key == "" {
		return nil
	}
	rate, ok := meterRateV2726(key)
	if !ok {
		return nil
	}
	p := rate
	if op == "studio_tts" {
		p = (units*rate + 999999) / 1000000
	}
	return &p
}
func meterParseOpenAIUsageV2726(raw []byte) (ref, model string, in, out int64, ok bool) {
	var v struct {
		ID    string `json:"id"`
		Model string `json:"model"`
		Usage *struct {
			Input      int64 `json:"input_tokens"`
			Output     int64 `json:"output_tokens"`
			Prompt     int64 `json:"prompt_tokens"`
			Completion int64 `json:"completion_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(raw, &v) != nil {
		return
	}
	ref, model = v.ID, v.Model
	if v.Usage != nil {
		in, out = v.Usage.Input, v.Usage.Output
		if in == 0 {
			in = v.Usage.Prompt
		}
		if out == 0 {
			out = v.Usage.Completion
		}
		ok = true
	}
	return
}
func (a *App) meterRecordV2726(e meterEventV2726) {
	if !meterEnabledV2726() || e.Key == "" || e.Provider == "" || e.Operation == "" || e.InputTokens < 0 || e.OutputTokens < 0 || e.Units < 0 {
		return
	}
	if e.TenantID < 0 {
		e.TenantID = 0
	}
	pricing := "unpriced"
	var estimated any
	if e.EstimatedMicroUSD != nil {
		pricing = "configured_rate_estimate"
		estimated = *e.EstimatedMicroUSD
	}
	_, err := a.db.Exec(`INSERT INTO finance_operation_meter_v2726(event_key,tenant_id,provider,operation,model,status,provider_ref,hold_id,input_tokens,output_tokens,units,unit_kind,estimated_micro_usd,pricing_status,created_at)
 VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(event_key) DO NOTHING`, e.Key, e.TenantID, e.Provider, e.Operation, e.Model, e.Status, e.ProviderRef, e.HoldID, e.InputTokens, e.OutputTokens, e.Units, e.UnitKind, estimated, pricing, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		log.Printf("meter V27.26: cannot save operations record: %v", err)
	}
}
func (a *App) meterCoreV2726(tid int64, hold *creativeHold, raw []byte, status string) {
	ref, model, in, out, usage := meterParseOpenAIUsageV2726(raw)
	if model == "" {
		model = commercialCoreModel(a.cfg.OpenAIModel)
	}
	e := meterEventV2726{Key: meterKeyV2726("core", hold), HoldID: meterHoldV2726(hold), TenantID: tid,
		Provider: "openai", Operation: "text_response", Model: model, ProviderRef: ref, Status: status,
		InputTokens: in, OutputTokens: out, Units: 1, UnitKind: "request"}
	if status == "completed" && usage {
		e.EstimatedMicroUSD = meterTokenPriceV2726(model, in, out)
	}
	a.meterRecordV2726(e)
}

// Finance data and vendor wholesale rates are superadmin-only.
func (a *App) meterAdminV2726Handler(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireSuperadmin(w, r); !ok {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "método no permitido", 405)
		return
	}
	period := financePeriod(r)
	rows, err := a.db.Query(`SELECT provider,operation,status,pricing_status,COUNT(*),COALESCE(SUM(input_tokens),0),COALESCE(SUM(output_tokens),0),COALESCE(SUM(units),0),COUNT(estimated_micro_usd),COALESCE(SUM(estimated_micro_usd),0)
 FROM finance_operation_meter_v2726 WHERE substr(created_at,1,7)=? GROUP BY provider,operation,status,pricing_status ORDER BY provider,operation,status LIMIT 300`, period)
	if err != nil {
		http.Error(w, "error de medición", 503)
		return
	}
	defer rows.Close()
	operations := []map[string]any{}
	for rows.Next() {
		var provider, op, status, pricing string
		var n, in, out, units, priced, micros int64
		if rows.Scan(&provider, &op, &status, &pricing, &n, &in, &out, &units, &priced, &micros) != nil {
			http.Error(w, "error de lectura", 503)
			return
		}
		operations = append(operations, map[string]any{"provider": provider, "operation": op, "status": status, "pricing_status": pricing, "operations": n, "input_tokens": in, "output_tokens": out, "units": units, "priced_operations": priced, "estimated_micro_usd": micros})
	}
	if rows.Err() != nil {
		http.Error(w, "lectura incompleta", 503)
		return
	}
	ext, err := a.db.Query(`SELECT provider,operation,status,COUNT(*),COALESCE(SUM(units),0) FROM external_usage_v2723 WHERE substr(created_at,1,7)=? GROUP BY provider,operation,status LIMIT 300`, period)
	if err != nil {
		http.Error(w, "error de mensajería", 503)
		return
	}
	defer ext.Close()
	external := []map[string]any{}
	for ext.Next() {
		var p, o, s string
		var n, units int64
		if ext.Scan(&p, &o, &s, &n, &units) != nil {
			http.Error(w, "error leyendo mensajería", 503)
			return
		}
		external = append(external, map[string]any{"provider": p, "operation": o, "status": s, "operations": n, "units": units, "pricing_status": "unpriced"})
	}
	if ext.Err() != nil {
		http.Error(w, "mensajería incompleta", 503)
		return
	}
	tenantSummary := []map[string]any{}
	perTenant, err := a.db.Query(`SELECT tenant_id,provider,operation,COUNT(*),
        COALESCE(SUM(estimated_micro_usd),0),COUNT(estimated_micro_usd)
        FROM finance_operation_meter_v2726 WHERE substr(created_at,1,7)=?
        GROUP BY tenant_id,provider,operation ORDER BY tenant_id,provider LIMIT 500`, period)
	if err != nil {
		http.Error(w, "error desglose empresas", 503)
		return
	}
	defer perTenant.Close()
	for perTenant.Next() {
		var tid, total, estimated, priced int64
		var p, o string
		if perTenant.Scan(&tid, &p, &o, &total, &estimated, &priced) != nil {
			http.Error(w, "error desglose", 503)
			return
		}
		tenantSummary = append(tenantSummary, map[string]any{"tenant_id": tid, "provider": p, "operation": o,
			"operations": total, "priced_operations": priced, "estimated_micro_usd": estimated})
	}
	if perTenant.Err() != nil {
		http.Error(w, "desglose incompleto", 503)
		return
	}
	// Internal quotas consume platform resources but have no direct vendor price.
	internal := []map[string]any{}
	records, err := a.db.Query(`SELECT resource,SUM(attempts),COUNT(DISTINCT tenant_id)
       FROM resource_meter_v2724 WHERE period=? GROUP BY resource LIMIT 100`, period)
	if err != nil {
		http.Error(w, "error contadores internos", 503)
		return
	}
	defer records.Close()
	for records.Next() {
		var name string
		var attempts, tenants int64
		if records.Scan(&name, &attempts, &tenants) != nil {
			http.Error(w, "error contadores", 503)
			return
		}
		internal = append(internal, map[string]any{"resource": name, "attempts": attempts, "companies": tenants, "pricing_status": "unpriced"})
	}
	if records.Err() != nil {
		http.Error(w, "error iterando contadores", 503)
		return
	}
	var automated int64
	if err := a.db.QueryRow(`SELECT COALESCE(SUM(attempts),0) FROM automation_monthly_usage_v2723 WHERE period=?`, period).Scan(&automated); err != nil {
		http.Error(w, "error workflows", 503)
		return
	}
	internal = append(internal, map[string]any{"resource": "automation_workflow_runs", "attempts": automated, "pricing_status": "unpriced"})
	writeJSON(w, map[string]any{"period": period, "enabled": meterEnabledV2726(),
		"operational_meter": operations, "by_tenant": tenantSummary, "external_unpriced": external,
		"internal_unpriced": internal, "supplier_bills_are_verified": false,
		"note": "Tarifas configuradas y métricas de uso no equivalen a una factura; precios faltantes = costo desconocido. Los costos reales se concilian en V27.25."})
}

// Poll updates only the final status; the price remains an unverified estimate.
func (a *App) meterVideoStateV2726(tid int64, providerRef, state string) {
	if !meterEnabledV2726() || tid <= 0 || providerRef == "" {
		return
	}
	switch state {
	case "queued", "processing", "completed", "failed":
	default:
		return
	}
	_, err := a.db.Exec(`UPDATE finance_operation_meter_v2726 SET status=?
       WHERE tenant_id=? AND provider='luma' AND operation='studio_video' AND provider_ref=?`,
		state, tid, providerRef)
	if err != nil {
		log.Printf("meter V27.26: cannot update video status: %v", err)
	}
}
