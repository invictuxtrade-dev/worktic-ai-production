package main

// V27.23: safeguards for a gated staging pilot. No vendor charges or paid
// product entitlements are inferred from the metering ledger. The unowned
// legacy rule table is intentionally preserved, not auto-migrated.
import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func consumptionV2723Enabled() bool {
	return commercialEnabled() && os.Getenv("WORKTIC_CONSUMPTION_GUARD_V2723") == "1"
}
func tenantRulesV2723Enabled() bool {
	return consumptionV2723Enabled() && os.Getenv("WORKTIC_TENANT_RULES_V2723") == "1"
}

func initConsumptionGuardV2723Schema(db *DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS consumption_tenant_locks_v2723(tenant_id INTEGER PRIMARY KEY)`,
		`CREATE TABLE IF NOT EXISTS worktic_auto_rules_v2723(
            id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL,
            name TEXT NOT NULL, match_type TEXT NOT NULL DEFAULT 'contains',
            keyword TEXT NOT NULL, response TEXT NOT NULL, enabled INTEGER NOT NULL DEFAULT 1,
            created_at TEXT NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS idx_rules_tenant_v2723 ON worktic_auto_rules_v2723(tenant_id,id)`,
		`CREATE TABLE IF NOT EXISTS automation_monthly_usage_v2723(
            tenant_id INTEGER NOT NULL, period TEXT NOT NULL,
            attempts INTEGER NOT NULL DEFAULT 0,
            PRIMARY KEY(tenant_id,period))`,
		`CREATE TABLE IF NOT EXISTS external_usage_v2723(
            id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL,
            provider TEXT NOT NULL, operation TEXT NOT NULL,
            external_ref TEXT NOT NULL, status TEXT NOT NULL,
            units INTEGER NOT NULL DEFAULT 1,
            estimated_cost_cents INTEGER,
            currency TEXT NOT NULL DEFAULT 'USD',
            created_at TEXT NOT NULL,
            UNIQUE(tenant_id,provider,operation,external_ref))`,
		`CREATE INDEX IF NOT EXISTS idx_external_usage_v2723 ON external_usage_v2723(tenant_id,provider,created_at)`,
	}
	for _, q := range statements {
		if _, err := db.Exec(q); err != nil {
			return fmt.Errorf("V27.23 schema: %w", err)
		}
	}
	return nil
}

func lockConsumptionTenantV2723(tx *Tx, isPostgres bool, tid int64) error {
	if tid < 1 {
		return errors.New("empresa no válida")
	}
	if _, err := tx.Exec(`INSERT INTO consumption_tenant_locks_v2723(tenant_id) VALUES(?) ON CONFLICT(tenant_id) DO NOTHING`, tid); err != nil {
		return err
	}
	if isPostgres {
		var locked int64
		return tx.QueryRow(`SELECT tenant_id FROM consumption_tenant_locks_v2723 WHERE tenant_id=? FOR UPDATE`, tid).Scan(&locked)
	}
	// SQLite: explicitly take a write lock before checking the quota.
	_, err := tx.Exec(`UPDATE consumption_tenant_locks_v2723 SET tenant_id=tenant_id WHERE tenant_id=?`, tid)
	return err
}

func (a *App) tenantPlanV2723(tid int64) (Plan, error) {
	var owner int64
	if tid <= 0 {
		return Plan{}, errors.New("empresa inválida")
	}
	if err := a.db.QueryRow(`SELECT owner_user_id FROM tenants WHERE id=?`, tid).Scan(&owner); err != nil || owner <= 0 {
		return Plan{}, errors.New("empresa sin titular de facturación")
	}
	p, _, err := a.activePlan(owner)
	return p, err
}

func automationMonthlyCapV2723(plan string) int64 {
	var envKey string
	switch strings.ToLower(strings.TrimSpace(plan)) {
	case "personal":
		envKey = "WORKTIC_AUTO_RUNS_PERSONAL_V2723"
	case "business", "negocio":
		envKey = "WORKTIC_AUTO_RUNS_NEGOCIO_V2723"
	case "enterprise", "empresa":
		envKey = "WORKTIC_AUTO_RUNS_EMPRESA_V2723"
	default:
		return 0
	}
	n, err := strconv.ParseInt(strings.TrimSpace(os.Getenv(envKey)), 10, 64)
	if err != nil || n < 0 || n > 1000000 {
		return 0
	} // fail closed until explicitly configured
	return n
}

// Every attempted workflow execution counts, including failed actions and retries:
// this prevents unlimited third-party calls through repeated failures.
func (a *App) reserveAutomationRunV2723(tid int64) error {
	if !consumptionV2723Enabled() {
		return nil
	}
	p, err := a.tenantPlanV2723(tid)
	if err != nil {
		return err
	}
	cap := automationMonthlyCapV2723(p.Code)
	if cap == 0 {
		return errors.New("ejecuciones de automatización no habilitadas para este plan")
	}
	period := time.Now().UTC().Format("2006-01")
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`INSERT INTO automation_monthly_usage_v2723(tenant_id,period,attempts) VALUES(?,?,0) ON CONFLICT(tenant_id,period) DO NOTHING`, tid, period); err != nil {
		return err
	}
	result, err := tx.Exec(`UPDATE automation_monthly_usage_v2723 SET attempts=attempts+1 WHERE tenant_id=? AND period=? AND attempts<?`, tid, period, cap)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("cupo mensual de %d ejecuciones agotado", cap)
	}
	return tx.Commit()
}

// A plan quota must be tested and the workflow inserted under one per-tenant DB lock.
func (a *App) createAutomationWorkflowV2723(tid int64, x AutomationWorkflow, max int, now string) (int64, error) {
	tx, err := a.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if err := lockConsumptionTenantV2723(tx, a.db.IsPostgres(), tid); err != nil {
		return 0, err
	}
	var n int
	if tenantRulesV2723Enabled() {
		if err := tx.QueryRow(`SELECT (SELECT COUNT(*) FROM automation_workflows WHERE tenant_id=?) + (SELECT COUNT(*) FROM worktic_auto_rules_v2723 WHERE tenant_id=?)`, tid, tid).Scan(&n); err != nil {
			return 0, err
		}
	} else if err := tx.QueryRow(`SELECT COUNT(*) FROM automation_workflows WHERE tenant_id=?`, tid).Scan(&n); err != nil {
		return 0, err
	}
	if max < 0 || n >= max {
		return 0, errors.New("límite de workflows del plan alcanzado")
	}
	res, err := tx.Exec(`INSERT INTO automation_workflows(tenant_id,name,description,status,trigger_type,trigger_config,graph_json,version,created_at,updated_at) VALUES(?,?,?,?,?,?,?,1,?,?)`, tid, x.Name, x.Description, x.Status, x.TriggerType, string(x.TriggerConfig), string(x.Graph), now, now)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

// Important: NULL estimated_cost_cents means UNKNOWN, not a zero-dollar message.
// A provider's actual bill depends on template category, country, delivery and contract.
func (a *App) recordExternalUsageV2723(tid int64, provider, operation, externalRef, status string) error {
	if tid <= 0 || externalRef == "" {
		return errors.New("empresa o referencia externa inválida")
	}
	_, err := a.db.Exec(`INSERT INTO external_usage_v2723(tenant_id,provider,operation,external_ref,status,units,estimated_cost_cents,created_at) VALUES(?,?,?,?,?,1,NULL,?) ON CONFLICT(tenant_id,provider,operation,external_ref) DO NOTHING`, tid, provider, operation, externalRef, status, time.Now().UTC().Format(time.RFC3339))
	return err
}

func (a *App) consumptionSummaryV2723Handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "método no permitido", 405)
		return
	}
	u := a.currentUser(r)
	if u == nil {
		writeError(w, errors.New("sesión requerida"), 401)
		return
	}
	tid := u.TenantID
	if r.URL.Query().Get("tenant_id") != "" && u.Role != "superadmin" {
		writeError(w, errors.New("sin permiso"), 403)
		return
	}
	var err error
	if u.Role == "superadmin" && r.URL.Query().Get("tenant_id") != "" {
		tid, err = strconv.ParseInt(r.URL.Query().Get("tenant_id"), 10, 64)
		if err != nil || tid <= 0 {
			writeError(w, errors.New("empresa inválida"), 400)
			return
		}
	}
	if tid <= 0 {
		writeError(w, errors.New("empresa no indicada"), 400)
		return
	}
	period := time.Now().UTC().Format("2006-01")
	var attempts int64
	_ = a.db.QueryRow(`SELECT COALESCE(attempts,0) FROM automation_monthly_usage_v2723 WHERE tenant_id=? AND period=?`, tid, period).Scan(&attempts)
	rows, err := a.db.Query(`SELECT provider,operation,status,COUNT(*) FROM external_usage_v2723 WHERE tenant_id=? AND created_at>=? GROUP BY provider,operation,status ORDER BY provider,operation,status`, tid, period+"-01T00:00:00Z")
	if err != nil {
		writeError(w, errors.New("no se pudo leer consumo"), 500)
		return
	}
	defer rows.Close()
	usage := []map[string]any{}
	for rows.Next() {
		var p, op, status string
		var n int64
		if rows.Scan(&p, &op, &status, &n) == nil {
			usage = append(usage, map[string]any{"provider": p, "operation": op, "status": status, "units": n, "cost_usd": nil})
		}
	}
	if err = rows.Err(); err != nil {
		writeError(w, errors.New("consulta de consumo incompleta"), 500)
		return
	}
	plan, planErr := a.tenantPlanV2723(tid)
	quota := int64(0)
	if planErr == nil {
		quota = automationMonthlyCapV2723(plan.Code)
	}
	writeJSON(w, map[string]any{"tenant_id": tid, "period": period, "automation_attempts": attempts, "automation_cap": quota, "external_usage": usage, "billing_reconciled": false, "costs_known": false, "note": "La cantidad de envíos es observada; el costo es desconocido hasta conciliar con Meta/proveedor. Sólo recoge las rutas instrumentadas."})
}

func (a *App) legacyRulesInventoryV2723Handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "método no permitido", 405)
		return
	}
	u := a.currentUser(r)
	if u == nil || u.Role != "superadmin" {
		writeError(w, errors.New("solo superadmin"), 403)
		return
	}
	rows, err := a.db.Query(`SELECT id,name,enabled FROM worktic_auto_rules ORDER BY id`)
	if err != nil {
		writeError(w, err, 500)
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id int64
		var name string
		var en int
		if rows.Scan(&id, &name, &en) == nil {
			items = append(items, map[string]any{"legacy_id": id, "name": name, "enabled": en == 1, "tenant_id": nil, "migration": "requiere asignación manual y validación de canal"})
		}
	}
	writeJSON(w, map[string]any{"legacy_rules": items, "auto_migrated": false, "note": "Sin propietario en la tabla histórica; ninguna regla se transfiere automáticamente."})
}

func (a *App) tenantRulesHandlerV2723(w http.ResponseWriter, r *http.Request) {
	tid, u, err := a.tenantFor(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	switch r.Method {
	case http.MethodGet:
		rows, err := a.db.Query(`SELECT id,name,match_type,keyword,response,enabled FROM worktic_auto_rules_v2723 WHERE tenant_id=? ORDER BY id DESC`, tid)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		defer rows.Close()
		out := []AutoRule{}
		for rows.Next() {
			var x AutoRule
			var e int
			if rows.Scan(&x.ID, &x.Name, &x.MatchType, &x.Keyword, &x.Response, &e) == nil {
				x.Enabled = e == 1
				out = append(out, x)
			}
		}
		writeJSON(w, out)
	case http.MethodPost, http.MethodPut, http.MethodDelete:
		if u == nil || !canManageAgents(u.Role) {
			writeError(w, errors.New("sin permiso para administrar reglas"), 403)
			return
		}
		if r.Method == http.MethodDelete {
			id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
			if err != nil || id <= 0 {
				writeError(w, errors.New("id inválido"), 400)
				return
			}
			res, err := a.db.Exec(`DELETE FROM worktic_auto_rules_v2723 WHERE id=? AND tenant_id=?`, id, tid)
			if err != nil {
				writeError(w, err, 500)
				return
			}
			n, _ := res.RowsAffected()
			if n == 0 {
				writeError(w, errors.New("regla no encontrada"), 404)
				return
			}
			writeJSON(w, map[string]any{"ok": true})
			return
		}
		var x AutoRule
		if err := json.NewDecoder(r.Body).Decode(&x); err != nil {
			writeError(w, errors.New("regla inválida"), 400)
			return
		}
		x.Name = strings.TrimSpace(x.Name)
		x.Keyword = strings.TrimSpace(x.Keyword)
		x.Response = strings.TrimSpace(x.Response)
		if x.Name == "" || x.Keyword == "" || x.Response == "" || len(x.Response) > 4096 {
			writeError(w, errors.New("campos inválidos"), 400)
			return
		}
		if x.MatchType != "exact" && x.MatchType != "starts" {
			x.MatchType = "contains"
		}
		en := 0
		if x.Enabled {
			en = 1
		}
		if r.Method == http.MethodPut {
			if x.ID <= 0 {
				writeError(w, errors.New("id inválido"), 400)
				return
			}
			res, err := a.db.Exec(`UPDATE worktic_auto_rules_v2723 SET name=?,match_type=?,keyword=?,response=?,enabled=? WHERE id=? AND tenant_id=?`, x.Name, x.MatchType, x.Keyword, x.Response, en, x.ID, tid)
			if err != nil {
				writeError(w, err, 500)
				return
			}
			n, _ := res.RowsAffected()
			if n == 0 {
				writeError(w, errors.New("regla no encontrada"), 404)
				return
			}
			writeJSON(w, map[string]any{"ok": true})
			return
		}
		p, err := a.tenantPlanV2723(tid)
		if err != nil {
			writeError(w, errors.New("plan no disponible"), 503)
			return
		}
		tx, err := a.db.Begin()
		if err != nil {
			writeError(w, err, 500)
			return
		}
		defer tx.Rollback()
		if err = lockConsumptionTenantV2723(tx, a.db.IsPostgres(), tid); err != nil {
			writeError(w, err, 503)
			return
		}
		var count int
		if err = tx.QueryRow(`SELECT (SELECT COUNT(*) FROM worktic_auto_rules_v2723 WHERE tenant_id=?) + (SELECT COUNT(*) FROM automation_workflows WHERE tenant_id=?)`, tid, tid).Scan(&count); err != nil {
			writeError(w, err, 503)
			return
		}
		if count >= p.MaxRules {
			writeError(w, errors.New("límite de reglas de tu plan alcanzado"), 403)
			return
		}
		res, err := tx.Exec(`INSERT INTO worktic_auto_rules_v2723(tenant_id,name,match_type,keyword,response,enabled,created_at) VALUES(?,?,?,?,?,?,?)`, tid, x.Name, x.MatchType, x.Keyword, x.Response, en, time.Now().UTC().Format(time.RFC3339))
		if err != nil {
			writeError(w, err, 500)
			return
		}
		x.ID, err = res.LastInsertId()
		if err != nil {
			writeError(w, err, 500)
			return
		}
		if err = tx.Commit(); err != nil {
			writeError(w, err, 503)
			return
		}
		writeJSON(w, x)
	default:
		http.Error(w, "método no permitido", 405)
	}
}
