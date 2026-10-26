package main

// WorkticAI V27.29 - immutable contract rights, opt-in staging.
// No retroactive assignment of historical subscriptions and no automated
// proration, plan swaps or Partner ledger mutations.
import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

type contractPayloadV2729 struct {
	Schema      int            `json:"schema"`
	Plan        Plan           `json:"plan"`
	ModulesJSON string         `json:"modules_json"`
	Creative    commercialRule `json:"creative"`
}

type contractTermV2729 struct {
	SubscriptionID int64  `json:"subscription_id"`
	OwnerID        int64  `json:"owner_user_id"`
	PlanCode       string `json:"plan_code"`
	VersionHash    string `json:"version_hash"`
	Cycle          string `json:"billing_cycle"`
	StartsAt       string `json:"starts_at"`
	EndsAt         string `json:"ends_at"`
}

func contractV2729Enabled() bool {
	return os.Getenv("WORKTIC_CONTRACT_ENTITLEMENTS_V2729") == "1" && billingLifecycleV2728Enabled()
}

func initContractV2729Schema(db *DB) error {
	for _, q := range []string{
		`CREATE TABLE IF NOT EXISTS contract_plan_versions_v2729 (
            version_hash TEXT PRIMARY KEY, plan_code TEXT NOT NULL,
            payload_json TEXT NOT NULL, captured_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS contract_subscription_terms_v2729 (
            subscription_id INTEGER PRIMARY KEY, owner_user_id INTEGER NOT NULL,
            plan_code TEXT NOT NULL, version_hash TEXT NOT NULL,
            billing_cycle TEXT NOT NULL, starts_at TEXT NOT NULL,
            ends_at TEXT NOT NULL, captured_at TEXT NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS contract_terms_owner_v2729
            ON contract_subscription_terms_v2729(owner_user_id, ends_at)`,
	} {
		if _, err := db.Exec(q); err != nil {
			return fmt.Errorf("v2729 schema: %w", err)
		}
	}
	return nil
}

func contractHashV2729(p contractPayloadV2729) (string, string, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return "", "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), string(b), nil
}

func contractReadCatalogTxV2729(tx *Tx, code string) (contractPayloadV2729, error) {
	var p contractPayloadV2729
	var on int
	code = strings.ToLower(strings.TrimSpace(code))
	if code == "" {
		return p, errors.New("plan contractual vacío")
	}
	err := tx.QueryRow(`SELECT id,code,name,description,price_usdt,billing_days,
        max_users,max_channels,max_contacts,max_ai_responses,max_products,max_rules,active,
        COALESCE(features_json,'{}')
        FROM billing_plans WHERE code=?`, code).Scan(
		&p.Plan.ID, &p.Plan.Code, &p.Plan.Name, &p.Plan.Description,
		&p.Plan.PriceUSDT, &p.Plan.BillingDays, &p.Plan.MaxUsers,
		&p.Plan.MaxChannels, &p.Plan.MaxContacts, &p.Plan.MaxAIResponses,
		&p.Plan.MaxProducts, &p.Plan.MaxRules, &on, &p.ModulesJSON)
	if err != nil {
		return p, fmt.Errorf("no se pudo leer el catálogo: %w", err)
	}
	p.Plan.Active = on == 1
	if !p.Plan.Active {
		return p, errors.New("plan no disponible para contratar")
	}
	var flags map[string]json.RawMessage
	if err = json.Unmarshal([]byte(p.ModulesJSON), &flags); err != nil {
		return p, errors.New("permisos del plan tienen JSON inválido")
	}
	// Re-encode JSON deterministically; whitespace/key order never changes the hash.
	normalized, e := json.Marshal(flags)
	if e != nil {
		return p, e
	}
	p.ModulesJSON = string(normalized)
	p.Creative, err = commercialReadRuleTx(tx, code)
	if err != nil {
		return p, fmt.Errorf("reglas creativas inexistentes: %w", err)
	}
	p.Schema = 1
	return p, nil
}

func contractReadTxV2729(tx *Tx, subscriptionID int64) (contractPayloadV2729, string, error) {
	var p contractPayloadV2729
	var raw, hash string
	err := tx.QueryRow(`SELECT v.payload_json, v.version_hash FROM contract_subscription_terms_v2729 t
        JOIN contract_plan_versions_v2729 v ON v.version_hash=t.version_hash
        WHERE t.subscription_id=?`, subscriptionID).Scan(&raw, &hash)
	if err != nil {
		return p, "", err
	}
	if err = json.Unmarshal([]byte(raw), &p); err != nil {
		return p, "", err
	}
	calc, _, err := contractHashV2729(p)
	if err != nil || calc != hash {
		return p, "", errors.New("integridad del contrato inválida")
	}
	return p, hash, nil
}

func (a *App) contractReadV2729(subscriptionID int64) (contractPayloadV2729, string, error) {
	var p contractPayloadV2729
	var raw, hash string
	err := a.db.QueryRow(`SELECT v.payload_json,v.version_hash FROM contract_subscription_terms_v2729 t
        JOIN contract_plan_versions_v2729 v ON v.version_hash=t.version_hash
        WHERE t.subscription_id=?`, subscriptionID).Scan(&raw, &hash)
	if err != nil {
		return p, "", err
	}
	if err = json.Unmarshal([]byte(raw), &p); err != nil {
		return p, "", err
	}
	check, _, e := contractHashV2729(p)
	if e != nil || check != hash {
		return p, "", errors.New("integridad del contrato inválida")
	}
	return p, hash, nil
}

func contractRenewalCheckV2729(oldPlan, newPlan, oldHash, newHash string, oldEnds, now time.Time) error {
	if oldPlan == "" || oldPlan == "free" || !oldEnds.After(now) {
		return nil
	}
	if oldPlan != newPlan {
		return errors.New("cambio de plan con vigencia pendiente: requiere revisión y prorrateo")
	}
	if oldHash == "" {
		return errors.New("suscripción histórica sin beneficios contractuales verificados: auditar antes de renovar")
	}
	if oldHash != newHash {
		return errors.New("catálogo modificado durante vigencia pagada: conservar derechos o revisar renovación antes de cobrar")
	}
	return nil
}

// Always called BEFORE consuming the approved payment in the SAME SQL tx.
func (a *App) contractPreapproveV2729(tx *Tx, owner int64, nextPlan string, now time.Time) error {
	if !contractV2729Enabled() {
		return nil
	}
	proposed, err := contractReadCatalogTxV2729(tx, nextPlan)
	if err != nil {
		return err
	}
	newHash, _, err := contractHashV2729(proposed)
	if err != nil {
		return err
	}
	var previousID int64
	var previousCode, oldEndRaw string
	err = tx.QueryRow(`SELECT id,plan_code,ends_at FROM billing_subscriptions
        WHERE user_id=? AND status='active' AND ends_at>? ORDER BY ends_at DESC LIMIT 1`,
		owner, now.Format(time.RFC3339)).Scan(&previousID, &previousCode, &oldEndRaw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	oldEnd, err := time.Parse(time.RFC3339, oldEndRaw)
	if err != nil {
		return err
	}
	previousHash := ""
	if previousCode != "free" {
		_, previousHash, err = contractReadTxV2729(tx, previousID)
		if errors.Is(err, sql.ErrNoRows) {
			previousHash = ""
		} else if err != nil {
			return err
		}
	}
	return contractRenewalCheckV2729(previousCode, nextPlan, previousHash, newHash, oldEnd, now)
}

func (a *App) contractCaptureV2729(tx *Tx, owner, subscriptionID int64, plan, cycle string, start, end time.Time) error {
	if !contractV2729Enabled() {
		return nil
	}
	if owner <= 0 || subscriptionID <= 0 || !billingCycleValidV2728(cycle) || !end.After(start) {
		return errors.New("datos del contrato inválidos")
	}
	p, err := contractReadCatalogTxV2729(tx, plan)
	if err != nil {
		return err
	}
	hash, raw, err := contractHashV2729(p)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = tx.Exec(`INSERT INTO contract_plan_versions_v2729(version_hash,plan_code,payload_json,captured_at)
        VALUES(?,?,?,?) ON CONFLICT(version_hash) DO NOTHING`, hash, plan, raw, now)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO contract_subscription_terms_v2729
        (subscription_id,owner_user_id,plan_code,version_hash,billing_cycle,starts_at,ends_at,captured_at)
        VALUES(?,?,?,?,?,?,?,?)`, subscriptionID, owner, plan, hash, cycle,
		start.Format(time.RFC3339), end.Format(time.RFC3339), now)
	return err
}

func (a *App) contractForOwnerTxV2729(tx *Tx, owner int64) (contractPayloadV2729, error) {
	var subID int64
	err := tx.QueryRow(`SELECT id FROM billing_subscriptions
        WHERE user_id=? AND status='active' AND ends_at>? ORDER BY ends_at DESC LIMIT 1`, owner,
		time.Now().UTC().Format(time.RFC3339)).Scan(&subID)
	if err != nil {
		return contractPayloadV2729{}, err
	}
	p, _, err := contractReadTxV2729(tx, subID)
	return p, err
}

func (a *App) contractForTenantV2729(tid int64) (contractPayloadV2729, error) {
	var owner int64
	if err := a.db.QueryRow(`SELECT owner_user_id FROM tenants WHERE id=?`, tid).Scan(&owner); err != nil {
		return contractPayloadV2729{}, err
	}
	_, sub, err := a.activePlan(owner)
	if err != nil {
		return contractPayloadV2729{}, err
	}
	p, _, err := a.contractReadV2729(sub.ID)
	return p, err
}

// Checkout and preview use the same validation as approval; the approval
// repeats it inside its transaction to close time-of-check/time-of-use races.
func (a *App) contractPreflightV2729(owner int64, nextPlan string) error {
	if !contractV2729Enabled() {
		return nil
	}
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	return a.contractPreapproveV2729(tx, owner, nextPlan, time.Now().UTC())
}

func (a *App) contractStudioRuleV2729(tid int64, plan string) (commercialRule, error) {
	if !contractV2729Enabled() || plan == "free" {
		return commercialReadRule(a.db, plan)
	}
	p, err := a.contractForTenantV2729(tid)
	if err != nil {
		return commercialRule{}, err
	}
	if p.Plan.Code != plan {
		return commercialRule{}, errors.New("plan contractual no coincide")
	}
	return p.Creative, nil
}
func (a *App) contractStudioRuleTxV2729(tx *Tx, tid int64, plan string) (commercialRule, error) {
	if !contractV2729Enabled() || plan == "free" {
		return commercialReadRuleTx(tx, plan)
	}
	var owner int64
	if err := tx.QueryRow(`SELECT owner_user_id FROM tenants WHERE id=?`, tid).Scan(&owner); err != nil {
		return commercialRule{}, err
	}
	p, err := a.contractForOwnerTxV2729(tx, owner)
	if err != nil {
		return commercialRule{}, err
	}
	if p.Plan.Code != plan {
		return commercialRule{}, errors.New("plan contractual no coincide")
	}
	return p.Creative, nil
}

func (a *App) contractV2729Handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", 405)
		return
	}
	u := a.currentUser(r)
	if u == nil || u.TenantID <= 0 {
		http.Error(w, "Sesión empresarial requerida", 401)
		return
	}
	owner := a.billingAccountUserID(u)
	var sub Subscription
	err := a.db.QueryRow(`SELECT id,user_id,plan_code,status,starts_at,ends_at,created_at
        FROM billing_subscriptions WHERE user_id=? AND status='active' AND ends_at>?
        ORDER BY ends_at DESC LIMIT 1`, owner, time.Now().UTC().Format(time.RFC3339)).Scan(
		&sub.ID, &sub.UserID, &sub.PlanCode, &sub.Status, &sub.StartsAt, &sub.EndsAt, &sub.CreatedAt)
	if err != nil {
		http.Error(w, "Suscripción no disponible", 503)
		return
	}
	if sub.PlanCode == "free" {
		writeJSON(w, map[string]any{"version": "27.29", "status": "free_no_paid_contract", "plan_code": "free", "snapshot_required": false, "contract_enabled": contractV2729Enabled()})
		return
	}
	p, hash, err := a.contractReadV2729(sub.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusConflict)
			writeJSON(w, map[string]any{"version": "27.29", "status": "legacy_audit_required", "contract_enabled": contractV2729Enabled(), "message": "Este contrato no tiene snapshot verificado; se requiere auditoría manual antes de activar los derechos versionados."})
			return
		}
		http.Error(w, "No se pudo verificar la integridad del contrato", 503)
		return
	}
	writeJSON(w, map[string]any{"version": "27.29", "status": "captured", "contract_enabled": contractV2729Enabled(),
		"subscription_id": sub.ID, "plan_code": p.Plan.Code, "version_hash": hash,
		"starts_at": sub.StartsAt, "ends_at": sub.EndsAt, "rights": p, "actual_payment_verified": false})
}

func (a *App) contractAuditV2729Handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", 405)
		return
	}
	if _, ok := a.requireSuperadmin(w, r); !ok {
		return
	}
	var unversioned, versioned int64
	now := time.Now().UTC().Format(time.RFC3339)
	err := a.db.QueryRow(`SELECT COUNT(*) FROM billing_subscriptions s
        LEFT JOIN contract_subscription_terms_v2729 t ON t.subscription_id=s.id
        WHERE s.status='active' AND s.plan_code<>'free' AND s.ends_at>? AND t.subscription_id IS NULL`, now).Scan(&unversioned)
	if err != nil {
		http.Error(w, "Auditoría no disponible", 503)
		return
	}
	err = a.db.QueryRow(`SELECT COUNT(*) FROM contract_subscription_terms_v2729`).Scan(&versioned)
	if err != nil {
		http.Error(w, "Auditoría no disponible", 503)
		return
	}
	writeJSON(w, map[string]any{"version": "27.29", "snapshot_mode_active": contractV2729Enabled(),
		"subscriptions_with_snapshot": versioned, "legacy_paid_active_without_snapshot": unversioned,
		"launch_approved": false, "legacy_migration_automatic": false,
		"note": "Cualquier contrato antiguo sin evidencias necesita auditoría antes del cambio de modo."})
}
