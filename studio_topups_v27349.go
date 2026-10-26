package main

// V27.34.9 — Studio recharge storefront switch, editable packages and payment instructions.
// Approvals remain exclusively manual and require independent blockchain evidence.
import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

func initStudioTopupsV27349Schema(db *DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS studio_topups_settings_v27349 (
 id INTEGER PRIMARY KEY,enabled INTEGER NOT NULL DEFAULT 0,updated_by INTEGER NOT NULL DEFAULT 0,updated_at TEXT NOT NULL DEFAULT '')`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO studio_topups_settings_v27349(id,enabled) VALUES(1,0) ON CONFLICT(id) DO NOTHING`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS studio_topups_audit_v27349 (
 id TEXT PRIMARY KEY, actor_id INTEGER NOT NULL, event_type TEXT NOT NULL, item_code TEXT NOT NULL,
 old_value TEXT NOT NULL,new_value TEXT NOT NULL,created_at TEXT NOT NULL)`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS studio_topup_verifications_v27349 (
 topup_id TEXT PRIMARY KEY,reviewer_id INTEGER NOT NULL,network TEXT NOT NULL,
 verified_hash TEXT NOT NULL,verified_wallet TEXT NOT NULL,verified_amount_usdt REAL NOT NULL,
 verified_at TEXT NOT NULL)`)
	return err
}
func studioTopupsAdminEnabledV27349(db *DB) bool {
	var enabled int
	return db.QueryRow(`SELECT enabled FROM studio_topups_settings_v27349 WHERE id=1`).Scan(&enabled) == nil && enabled == 1
}

// Fail closed: selling unusable credits is prohibited. No other commercial flags change.
func studioTopupsBudgetsReadyV27349() bool {
	return creativeEnvCents("WORKTIC_COST_GLOBAL_MONTHLY_CENTS") > 0 &&
		creativeEnvCents("WORKTIC_COST_BUDGET_PERSONAL_CENTS") > 0 &&
		creativeEnvCents("WORKTIC_COST_BUDGET_NEGOCIO_CENTS") > 0 &&
		creativeEnvCents("WORKTIC_COST_BUDGET_EMPRESA_CENTS") > 0
}
func (a *App) studioTopupsEnabledV27349() bool {
	return studioTopupsAdminEnabledV27349(a.db) && studioCreditsEnabled() && studioTopupsBudgetsReadyV27349() && strings.TrimSpace(a.openAIKey()) != "" && studioVideoKey() != "" &&
		(strings.TrimSpace(a.cfg.USDTBEP20Address) != "" || strings.TrimSpace(a.cfg.USDTTRC20Address) != "")
}
func (a *App) studioTopupsReadConfigV27349() map[string]any {
	enabled := studioTopupsAdminEnabledV27349(a.db)
	return map[string]any{
		"admin_enabled": enabled, "topups_enabled": a.studioTopupsEnabledV27349(),
		"studio_generation_enabled":   studioCreditsEnabled() && studioTopupsBudgetsReadyV27349(),
		"image_provider_configured":   strings.TrimSpace(a.openAIKey()) != "",
		"video_provider_configured":   studioVideoKey() != "",
		"addresses":                   map[string]string{"BEP20": strings.TrimSpace(a.cfg.USDTBEP20Address), "TRC20": strings.TrimSpace(a.cfg.USDTTRC20Address)},
		"manual_payment_verification": true,
		"currency":                    "USDT", "auto_credit": false,
	}
}
func (a *App) studioTopupsConfigV27349Handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", 405)
		return
	}
	if _, _, err := a.tenantFor(r); err != nil {
		http.Error(w, "Sesión requerida", 401)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	commercialSetJSON(w, 200, a.studioTopupsReadConfigV27349())
}
func studioTopupsAuditV27349(tx *Tx, actor int64, action, code, old, newVal string) error {
	id, err := creativeID()
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO studio_topups_audit_v27349(id,actor_id,event_type,item_code,old_value,new_value,created_at) VALUES(?,?,?,?,?,?,?)`, id, actor, action, code, old, newVal, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}
func studioTopupsValidatePackV27349(code, name string, credits, price int64) error {
	code = strings.TrimSpace(code)
	name = strings.TrimSpace(name)
	if ok, _ := regexp.MatchString(`^[a-z0-9_-]{2,24}$`, code); !ok {
		return errors.New("Código de paquete inválido")
	}
	if len(name) < 2 || len(name) > 64 {
		return errors.New("Nombre de paquete inválido")
	}
	if credits < 1 || credits > 50000 || price < credits*5 || price > 5000000 {
		return errors.New("Precio o créditos fuera de rango: exige al menos US$0,05 por crédito")
	}
	return nil
}
func (a *App) studioTopupsAdminV27349Handler(w http.ResponseWriter, r *http.Request) {
	admin, ok := a.requireSuperadmin(w, r)
	if !ok {
		return
	}
	if r.Method == http.MethodGet {
		rows, err := a.db.Query(`SELECT code,name,credits,price_cents,active FROM commercial_packages_v2721 ORDER BY price_cents,code`)
		if err != nil {
			http.Error(w, "Paquetes no disponibles", 503)
			return
		}
		defer rows.Close()
		packs := []commercialPack{}
		for rows.Next() {
			var pack commercialPack
			var active int
			if err = rows.Scan(&pack.Code, &pack.Name, &pack.Credits, &pack.PriceCents, &active); err != nil {
				http.Error(w, "Error leyendo paquetes", 503)
				return
			}
			pack.Active = active == 1
			packs = append(packs, pack)
		}
		if rows.Err() != nil {
			http.Error(w, "Error consultando paquetes", 503)
			return
		}
		reply := a.studioTopupsReadConfigV27349()
		reply["packages"] = packs
		commercialSetJSON(w, 200, reply)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	var q struct {
		Action     string `json:"action"`
		Enabled    bool   `json:"enabled"`
		Code       string `json:"code"`
		Name       string `json:"name"`
		Credits    int64  `json:"credits"`
		PriceCents int64  `json:"price_cents"`
		Active     bool   `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
		http.Error(w, "Solicitud inválida", 400)
		return
	}
	// Prevent sales enable without a usable generation provider budget and a known payout address.
	if q.Action == "enable" && (!studioCreditsEnabled() || !studioTopupsBudgetsReadyV27349() || strings.TrimSpace(a.openAIKey()) == "" || studioVideoKey() == "" || (strings.TrimSpace(a.cfg.USDTBEP20Address) == "" && strings.TrimSpace(a.cfg.USDTTRC20Address) == "")) {
		http.Error(w, "Primero configura presupuestos de generación Studio y una dirección USDT oficial en Render", 409)
		return
	}
	if q.Action != "enable" && q.Action != "disable" && q.Action != "save_package" {
		http.Error(w, "Acción no admitida", 400)
		return
	}
	q.Code = strings.ToLower(strings.TrimSpace(q.Code))
	q.Name = strings.TrimSpace(q.Name)
	if q.Action == "save_package" {
		if err := studioTopupsValidatePackV27349(q.Code, q.Name, q.Credits, q.PriceCents); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
	}
	tx, err := a.db.Begin()
	if err != nil {
		http.Error(w, "No se pudo iniciar transacción", 503)
		return
	}
	defer tx.Rollback()
	old, newVal, code := "", "", ""
	switch q.Action {
	case "enable", "disable":
		var oldEnabled int
		if err = tx.QueryRow(`SELECT enabled FROM studio_topups_settings_v27349 WHERE id=1`).Scan(&oldEnabled); err != nil {
			http.Error(w, "Configuración no disponible", 503)
			return
		}
		v := 0
		if q.Action == "enable" {
			v = 1
		}
		_, err = tx.Exec(`UPDATE studio_topups_settings_v27349 SET enabled=?,updated_by=?,updated_at=? WHERE id=1`, v, admin.ID, time.Now().UTC().Format(time.RFC3339Nano))
		old = fmt.Sprint(oldEnabled)
		newVal = fmt.Sprint(v)
		code = "studio_topups"
	case "save_package":
		var oldName string
		var oldCredits, oldPrice int64
		var oldActive int
		e := tx.QueryRow(`SELECT name,credits,price_cents,active FROM commercial_packages_v2721 WHERE code=?`, q.Code).Scan(&oldName, &oldCredits, &oldPrice, &oldActive)
		if e != nil && !errors.Is(e, sql.ErrNoRows) {
			http.Error(w, "No se pudo revisar paquete", 503)
			return
		}
		if e == nil {
			old = fmt.Sprintf("%s|%d|%d|%d", oldName, oldCredits, oldPrice, oldActive)
		}
		active := 0
		if q.Active {
			active = 1
		}
		_, err = tx.Exec(`INSERT INTO commercial_packages_v2721(code,name,credits,price_cents,active) VALUES(?,?,?,?,?) ON CONFLICT(code) DO UPDATE SET name=excluded.name,credits=excluded.credits,price_cents=excluded.price_cents,active=excluded.active`, q.Code, q.Name, q.Credits, q.PriceCents, active)
		newVal = fmt.Sprintf("%s|%d|%d|%d", q.Name, q.Credits, q.PriceCents, active)
		code = q.Code
	}
	if err != nil {
		http.Error(w, "No se pudo guardar configuración", 503)
		return
	}
	if err = studioTopupsAuditV27349(tx, admin.ID, q.Action, code, old, newVal); err != nil {
		http.Error(w, "Error en auditoría", 503)
		return
	}
	if err = tx.Commit(); err != nil {
		http.Error(w, "No se pudo confirmar configuración", 503)
		return
	}
	commercialSetJSON(w, 200, map[string]any{"ok": true, "action": q.Action, "payment_verification": "manual"})
}
