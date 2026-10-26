package main

// WorkticAI V27.24 — transactionally limited, opt-in resource creation.
// Features are disabled by default. Costs remain unverified until supplier
// billing is reconciled; quota counters are NEVER interpreted as invoices.
import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func resourceGuardV2724Enabled() bool {
	return commercialEnabled() && os.Getenv("WORKTIC_RESOURCE_GUARD_V2724") == "1"
}

// Configuration is deliberately explicit: no commercial promises are inferred.
func resourceQuotaEnvV2724(plan, resource string) int64 {
	plan = strings.ToLower(strings.TrimSpace(plan))
	switch plan {
	case "business", "negocio":
		plan = "NEGOCIO"
	case "enterprise", "empresa":
		plan = "EMPRESA"
	case "personal":
		plan = "PERSONAL"
	default:
		return 0
	}
	key := "WORKTIC_" + strings.ToUpper(resource) + "_" + plan + "_V2724"
	s := strings.TrimSpace(os.Getenv(key))
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 0 || n > 10000000 {
		return 0
	}
	return n
}

func initResourceGuardV2724Schema(db *DB) error {
	// Only additive tables. No billing / Partners / existing records altered.
	for _, stmt := range []string{
		`CREATE TABLE IF NOT EXISTS resource_meter_v2724(tenant_id INTEGER NOT NULL, period TEXT NOT NULL, resource TEXT NOT NULL, attempts INTEGER NOT NULL DEFAULT 0, PRIMARY KEY(tenant_id,period,resource))`,
		`CREATE INDEX IF NOT EXISTS idx_resource_meter_v2724 ON resource_meter_v2724(resource,period)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("v2724 schema: %w", err)
		}
	}
	return nil
}

// One tenant lock, one COUNT, one INSERT, one COMMIT. All writers that enforce
// the same quota must use this helper; legacy webhook imports are not covered.
func (a *App) resourceInsertV2724(tid int64, kind string, max int64, insert string, args ...any) (sql.Result, error) {
	if tid <= 0 || max == 0 {
		return nil, errors.New("el plan no tiene cupo para este recurso")
	}
	countSQL := map[string]string{
		"products":      `SELECT COUNT(*) FROM crm_products WHERE tenant_id=?`,
		"contacts":      `SELECT COUNT(*) FROM crm_contacts WHERE tenant_id=? AND COALESCE(deleted_at,'')=''`,
		"appointments":  `SELECT COUNT(*) FROM crm_appointments WHERE tenant_id=? AND status NOT IN ('Cancelada','No asistió')`,
		"professionals": `SELECT COUNT(*) FROM agenda_professionals WHERE tenant_id=?`,
		"services":      `SELECT COUNT(*) FROM agenda_services WHERE tenant_id=?`,
	}[kind]
	if countSQL == "" {
		return nil, errors.New("tipo de cuota desconocido")
	}
	tx, err := a.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = lockConsumptionTenantV2723(tx, a.db.IsPostgres(), tid); err != nil {
		return nil, err
	}
	var count int64
	if err = tx.QueryRow(countSQL, tid).Scan(&count); err != nil {
		return nil, err
	}
	if max > 0 && count >= max {
		return nil, fmt.Errorf("límite de %s alcanzado para el plan (%d)", kind, max)
	}
	// Revalidate slot overlap INSIDE the same per-tenant transaction, to
	// prevent two appointments for the same professional and time window.
	if kind == "appointments" {
		if len(args) != 12 {
			return nil, errors.New("parámetros de cita inválidos")
		}
		var overlap int64
		err = tx.QueryRow(`SELECT COUNT(*) FROM crm_appointments WHERE tenant_id=? AND professional_id=? AND status NOT IN ('Cancelada','No asistió') AND starts_at < datetime(?, '+' || ? || ' minutes') AND datetime(starts_at, '+' || duration_minutes || ' minutes') > ?`, tid, args[9], args[4], args[5], args[4]).Scan(&overlap)
		if err != nil {
			return nil, err
		}
		if overlap > 0 {
			return nil, errors.New("ese horario ya está ocupado para el profesional seleccionado")
		}
	}
	result, err := tx.Exec(insert, args...)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

func (a *App) resourcePlanCapV2724(tid int64, resource string) (int64, error) {
	p, err := a.tenantEffectivePlanV2730(tid)
	if err != nil {
		return 0, err
	}
	switch resource {
	case "products":
		return int64(p.MaxProducts), nil
	case "contacts":
		return int64(p.MaxContacts), nil
	case "appointments", "professionals", "services", "analytics_events", "ads_syncs":
		return resourceQuotaEnvV2724(p.Code, resource), nil
	}
	return 0, errors.New("recurso desconocido")
}

// SQL-atomic metering for external sync requests and analytics events. A failed
// provider request still consumes an attempt, to bound costs from retries.
// A failed persistence in the SAME tx rolls the reservation back.
func reserveResourceMeterTxV2724(tx *Tx, tid int64, resource, period string, cap int64) error {
	if cap <= 0 {
		return fmt.Errorf("%s no habilitado (cupo cero)", resource)
	}
	_, err := tx.Exec(`INSERT INTO resource_meter_v2724(tenant_id,period,resource,attempts) VALUES(?,?,?,0) ON CONFLICT(tenant_id,period,resource) DO NOTHING`, tid, period, resource)
	if err != nil {
		return err
	}
	result, err := tx.Exec(`UPDATE resource_meter_v2724 SET attempts=attempts+1 WHERE tenant_id=? AND period=? AND resource=? AND attempts<?`, tid, period, resource, cap)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("cupo mensual de %s agotado (máximo %d)", resource, cap)
	}
	return nil
}
func (a *App) reserveResourceMeterV2724(tid int64, resource string) error {
	if !resourceGuardV2724Enabled() {
		return nil
	}
	cap, err := a.resourcePlanCapV2724(tid, resource)
	if err != nil {
		return err
	}
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = reserveResourceMeterTxV2724(tx, tid, resource, time.Now().UTC().Format("2006-01"), cap); err != nil {
		return err
	}
	return tx.Commit()
}

// Manual CRM creations must recheck identity *inside* the tenant lock,
// otherwise two near-simultaneous requests could both create a contact.
func (a *App) crmManualUpsertV2724(tid int64, x CRMContactRecord) (int64, bool, error) {
	max, err := a.resourcePlanCapV2724(tid, "contacts")
	if err != nil {
		return 0, false, err
	}
	tx, err := a.db.Begin()
	if err != nil {
		return 0, false, err
	}
	defer tx.Rollback()
	if err = lockConsumptionTenantV2723(tx, a.db.IsPostgres(), tid); err != nil {
		return 0, false, err
	}
	var id int64
	err = tx.QueryRow(`SELECT id FROM crm_contacts WHERE tenant_id=? AND ((?<>'' AND external_id=?) OR (?<>'' AND phone=?) OR (?<>'' AND lower(email)=lower(?))) ORDER BY id LIMIT 1`, tid, x.ExternalID, x.ExternalID, x.Phone, x.Phone, x.Email, x.Email).Scan(&id)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, false, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if err == nil && id > 0 {
		_, err = tx.Exec(`UPDATE crm_contacts SET name=CASE WHEN name='' AND ?<>'' THEN ? ELSE name END, phone=CASE WHEN phone='' THEN ? ELSE phone END, email=CASE WHEN email='' THEN ? ELSE email END, channel=?, stage=?,tags=?,notes=?,identity_locked=1,deleted_at='',updated_at=? WHERE id=? AND tenant_id=?`, x.Name, x.Name, x.Phone, x.Email, x.Channel, x.Stage, x.Tags, x.Notes, now, id, tid)
		if err != nil {
			return 0, false, err
		}
		return id, true, tx.Commit()
	}
	var used int64
	if err = tx.QueryRow(`SELECT COUNT(*) FROM crm_contacts WHERE tenant_id=? AND COALESCE(deleted_at,'')=''`, tid).Scan(&used); err != nil {
		return 0, false, err
	}
	if max == 0 || (max > 0 && used >= max) {
		return 0, false, fmt.Errorf("límite de contactos alcanzado (%d)", max)
	}
	result, err := tx.Exec(`INSERT INTO crm_contacts(tenant_id,name,phone,email,channel,source,external_id,stage,tags,notes,first_seen_at,last_activity_at,created_at,updated_at,identity_locked) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,1)`, tid, x.Name, x.Phone, x.Email, x.Channel, x.Source, x.ExternalID, x.Stage, x.Tags, x.Notes, now, now, now, now)
	if err != nil {
		return 0, false, err
	}
	id, err = result.LastInsertId()
	if err != nil {
		return 0, false, err
	}
	return id, false, tx.Commit()
}

// One AJAX endpoint for staging observability; read-only and tenant-scoped.
func (a *App) resourceSummaryV2724Handler(w http.ResponseWriter, r *http.Request) {
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
	if raw := r.URL.Query().Get("tenant_id"); raw != "" {
		if u.Role != "superadmin" {
			writeError(w, errors.New("sin permiso"), 403)
			return
		}
		var err error
		tid, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || tid <= 0 {
			writeError(w, errors.New("empresa inválida"), 400)
			return
		}
	}
	if tid <= 0 {
		writeError(w, errors.New("empresa requerida"), 400)
		return
	}
	period := time.Now().UTC().Format("2006-01")
	result := map[string]any{"tenant_id": tid, "period": period, "guard_active": resourceGuardV2724Enabled(), "costs_reconciled": false}
	limits := map[string]any{}
	for _, key := range []string{"products", "contacts", "appointments", "professionals", "services", "analytics_events", "ads_syncs"} {
		cap, err := a.resourcePlanCapV2724(tid, key)
		if err != nil {
			limits[key] = nil
		} else {
			limits[key] = cap
		}
	}
	result["limits"] = limits
	current := map[string]int64{}
	for key, stmt := range map[string]string{
		"products":      `SELECT COUNT(*) FROM crm_products WHERE tenant_id=?`,
		"contacts":      `SELECT COUNT(*) FROM crm_contacts WHERE tenant_id=? AND COALESCE(deleted_at,'')=''`,
		"appointments":  `SELECT COUNT(*) FROM crm_appointments WHERE tenant_id=? AND status NOT IN ('Cancelada','No asistió')`,
		"professionals": `SELECT COUNT(*) FROM agenda_professionals WHERE tenant_id=?`,
		"services":      `SELECT COUNT(*) FROM agenda_services WHERE tenant_id=?`,
	} {
		var n int64
		if err := a.db.QueryRow(stmt, tid).Scan(&n); err == nil {
			current[key] = n
		}
	}
	result["current_resources"] = current
	rows, err := a.db.Query(`SELECT resource,attempts FROM resource_meter_v2724 WHERE tenant_id=? AND period=? ORDER BY resource`, tid, period)
	if err != nil {
		writeError(w, errors.New("no fue posible leer el consumo"), 500)
		return
	}
	defer rows.Close()
	meters := map[string]int64{}
	for rows.Next() {
		var key string
		var n int64
		if rows.Scan(&key, &n) == nil {
			meters[key] = n
		}
	}
	if err = rows.Err(); err != nil {
		writeError(w, err, 500)
		return
	}
	result["monthly_meters"] = meters
	result["note"] = "Los límites de CRUD se aplican solo a las rutas instrumentadas. WhatsApp, importaciones y otras rutas heredadas requieren certificación aparte. Consumo externo expresado como intentos, no dólares."
	writeJSON(w, result)
}

// Attribution keys already present must not consume the monthly event budget.
// The event INSERT and the quota increment commit together. Importantly, a
// quota exceeded means event storage is skipped; use staging only until the
// customer retention policy, back-pressure and export are product-approved.
func (a *App) storeAnalyticsEventV2724(tid int64, eventKey, statement string, args ...any) error {
	cap, err := a.resourcePlanCapV2724(tid, "analytics_events")
	if err != nil {
		return err
	}
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = lockConsumptionTenantV2723(tx, a.db.IsPostgres(), tid); err != nil {
		return err
	}
	if eventKey != "" {
		var exists int
		if err = tx.QueryRow(`SELECT COUNT(*) FROM analytics_attribution_events_v24 WHERE tenant_id=? AND event_key=?`, tid, eventKey).Scan(&exists); err != nil {
			return err
		}
		if exists > 0 {
			return nil
		}
	}
	if err = reserveResourceMeterTxV2724(tx, tid, "analytics_events", time.Now().UTC().Format("2006-01"), cap); err != nil {
		return err
	}
	if _, err = tx.Exec(statement, args...); err != nil {
		return err
	}
	return tx.Commit()
}
