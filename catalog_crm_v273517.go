package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode"
)

// V27.35.17: Optional, tenant-isolated product ↔ CRM links.
// New agents are opted OUT. A product mention cannot approve a sale or change stock.
func initCatalogCRMV273517(db *DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS catalog_agent_crm_settings_v273517 (
        tenant_id INTEGER NOT NULL, agent_id INTEGER NOT NULL,
        enabled INTEGER NOT NULL DEFAULT 0, updated_by INTEGER NOT NULL DEFAULT 0,
        updated_at TEXT NOT NULL DEFAULT '', PRIMARY KEY (tenant_id,agent_id))`)
	return err
}

func catalogAgentCRMAllowedV273517(db *DB, tenant, agent int64) bool {
	if !catalogAgentEnabledV273516(db, tenant, agent) {
		return false
	}
	var enabled, crm int
	var tools string
	err := db.QueryRow(`SELECT s.enabled,p.crm_enabled,a.tools FROM catalog_agent_crm_settings_v273517 s
        JOIN ai_agent_profiles_v22 p ON p.tenant_id=s.tenant_id AND p.agent_id=s.agent_id
        JOIN ai_agents a ON a.id=s.agent_id AND a.tenant_id=s.tenant_id AND a.status='active'
        WHERE s.tenant_id=? AND s.agent_id=?`, tenant, agent).Scan(&enabled, &crm, &tools)
	if err != nil || enabled != 1 || crm != 1 {
		return false
	}
	for _, v := range strings.Split(strings.ToLower(tools), ",") {
		if strings.TrimSpace(v) == "opportunity" {
			return true
		}
	}
	return false
}

// Product matching is conservative: never infer an order from broad terms such
// as "precio", "producto", or just "plan". Require an exact distinctive name
// phrase plus an explicit commercial intent. Multiple named products are allowed.
func catalogWordsV273517(s string) string {
	s = normalizeIntentText(s)
	return strings.Join(strings.FieldsFunc(s, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }), " ")
}
func catalogMentionedV273517(items []catalogCommercialItemV273516, message string) []catalogCommercialItemV273516 {
	msg := " " + catalogWordsV273517(message) + " "
	out := make([]catalogCommercialItemV273516, 0, 3)
	for _, p := range items {
		if !p.Available {
			continue
		}
		name := p.Name
		for _, sep := range []string{" - ", " — ", " ("} {
			if i := strings.Index(name, sep); i > 0 {
				name = name[:i]
			}
		}
		name = catalogWordsV273517(name)
		words := strings.Fields(name)
		if len(name) < 6 || len(words) < 2 || len(name) > 100 {
			continue
		}
		// "Plan" alone and other generic words do not qualify as a match.
		distinctive := false
		for _, w := range words {
			if len([]rune(w)) >= 5 && w != "producto" && w != "servicio" && w != "worktic" {
				distinctive = true
			}
		}
		if !distinctive {
			continue
		}
		if strings.Contains(msg, " "+name+" ") {
			out = append(out, p)
		}
		if len(out) >= 5 {
			break
		}
	}
	return out
}

// Called only after the assigned tenant agent successfully generated an answer.
// No contact identification -> no mutation. A repeat only refreshes the existing
// association; it never creates duplicate product rows or touches stock.
func (a *App) catalogAutoLinkFromMessageV273517(tenant, agent int64, chat, message, channel string) error {
	if tenant <= 0 || agent <= 0 || len(message) > 4000 || !catalogAgentCRMAllowedV273517(a.db, tenant, agent) {
		return nil
	}
	matched, interest, stage, score := detectCommercialOpportunity(message)
	if !matched {
		return nil
	}
	items, err := catalogReadV273516(a.db, tenant, true)
	if err != nil {
		return err
	}
	products := catalogMentionedV273517(items, message)
	if len(products) == 0 {
		return nil
	}
	chat = strings.TrimSpace(chat)
	if chat == "" || len(chat) > 250 {
		return nil
	}
	contact, err := a.contactIDForOpportunity(tenant, 0, "", "", chat)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var opportunity int64
	err = a.db.QueryRow(`SELECT id FROM crm_opportunities WHERE tenant_id=? AND contact_id=?
        AND COALESCE(deleted_at,'')='' AND stage NOT IN ('Ganado','Perdido')
        ORDER BY id DESC LIMIT 1`, tenant, contact).Scan(&opportunity)
	if errors.Is(err, sql.ErrNoRows) {
		opportunity, _, err = a.syncAutomaticOpportunity(tenant, contact, "conversation", "chat:"+chat, channel, interest, stage, score, "")
	}
	if err != nil || opportunity <= 0 {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, p := range products {
		res, e := tx.Exec(`INSERT INTO catalog_opportunity_items_v273516(tenant_id,opportunity_id,product_id,quantity,created_at)
          VALUES(?,?,?,1,?) ON CONFLICT(tenant_id,opportunity_id,product_id) DO NOTHING`, tenant, opportunity, p.ID, now)
		if e != nil {
			return e
		}
		n, _ := res.RowsAffected()
		if n == 1 {
			detail := fmt.Sprintf("agent_id=%d product_id=%d channel=%s (sin cambio de stock ni pago)", agent, p.ID, channel)
			if _, e = tx.Exec(`INSERT INTO catalog_audit_v273516(tenant_id,actor_id,action,entity_type,entity_id,details,created_at)
                VALUES(?,0,'auto_link_agent','opportunity',?,?,?)`, tenant, opportunity, detail, now); e != nil {
				return e
			}
		}
	}
	return tx.Commit()
}

func (a *App) catalogCRMSettingsV273517(w http.ResponseWriter, r *http.Request) {
	tenant, u, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	if !canManageAgents(u.Role) {
		writeError(w, errors.New("sin permiso de agentes"), 403)
		return
	}
	agent := catalogIntParamV273516(r, "agent_id")
	if agent <= 0 {
		writeError(w, errors.New("selecciona un agente"), 400)
		return
	}
	var exists int
	if err = a.db.QueryRow(`SELECT 1 FROM ai_agents WHERE id=? AND tenant_id=?`, agent, tenant).Scan(&exists); err != nil {
		writeError(w, errors.New("agente inexistente en esta empresa"), 404)
		return
	}
	switch r.Method {
	case http.MethodGet:
		var flag int
		err = a.db.QueryRow(`SELECT enabled FROM catalog_agent_crm_settings_v273517 WHERE tenant_id=? AND agent_id=?`, tenant, agent).Scan(&flag)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			writeError(w, err, 500)
			return
		}
		writeJSON(w, map[string]any{"agent_id": agent, "enabled": flag == 1, "effective": catalogAgentCRMAllowedV273517(a.db, tenant, agent)})
	case http.MethodPost:
		var in struct {
			Enabled bool `json:"enabled"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048)).Decode(&in) != nil {
			writeError(w, errors.New("configuración inválida"), 400)
			return
		}
		// Allow preconfiguration for a draft agent. Runtime still enforces all permissions.
		enabled := 0
		if in.Enabled {
			enabled = 1
		}
		now := time.Now().UTC().Format(time.RFC3339)
		_, err = a.db.Exec(`INSERT INTO catalog_agent_crm_settings_v273517(tenant_id,agent_id,enabled,updated_by,updated_at)
            VALUES(?,?,?,?,?) ON CONFLICT(tenant_id,agent_id) DO UPDATE SET enabled=excluded.enabled,updated_by=excluded.updated_by,updated_at=excluded.updated_at`, tenant, agent, enabled, u.ID, now)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		_ = a.catalogAuditV273516(tenant, u.ID, "agent_auto_crm_settings", "agent", agent, fmt.Sprintf("enabled=%v", in.Enabled))
		writeJSON(w, map[string]any{"ok": true, "enabled": in.Enabled, "effective": catalogAgentCRMAllowedV273517(a.db, tenant, agent)})
	default:
		http.Error(w, "Método no permitido", 405)
	}
}

func (a *App) catalogOpportunityItemsV273517(w http.ResponseWriter, r *http.Request) {
	tenant, u, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	if !canManageCatalog(u.Role) {
		writeError(w, errors.New("sin permiso de CRM"), 403)
		return
	}
	opportunity := catalogIntParamV273516(r, "opportunity_id")
	if opportunity <= 0 {
		writeError(w, errors.New("oportunidad obligatoria"), 400)
		return
	}
	var exists int
	if err = a.db.QueryRow(`SELECT 1 FROM crm_opportunities WHERE tenant_id=? AND id=? AND COALESCE(deleted_at,'')=''`, tenant, opportunity).Scan(&exists); err != nil {
		writeError(w, errors.New("oportunidad inexistente"), 404)
		return
	}
	switch r.Method {
	case http.MethodGet:
		rows, err := a.db.Query(`SELECT i.product_id,p.name,p.type,p.currency,p.price,i.quantity,p.active
            FROM catalog_opportunity_items_v273516 i JOIN crm_products p ON p.id=i.product_id AND p.tenant_id=i.tenant_id
            WHERE i.tenant_id=? AND i.opportunity_id=? ORDER BY p.name`, tenant, opportunity)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		defer rows.Close()
		output := []map[string]any{}
		for rows.Next() {
			var pid int64
			var name, kind, currency string
			var price float64
			var qty, active int
			if err = rows.Scan(&pid, &name, &kind, &currency, &price, &qty, &active); err != nil {
				writeError(w, err, 500)
				return
			}
			output = append(output, map[string]any{"product_id": pid, "name": name, "type": kind, "currency": currency, "price": price, "quantity": qty, "active": active == 1})
		}
		if err = rows.Err(); err != nil {
			writeError(w, err, 500)
			return
		}
		writeJSON(w, output)
	case http.MethodDelete:
		product := catalogIntParamV273516(r, "product_id")
		if product <= 0 {
			writeError(w, errors.New("producto obligatorio"), 400)
			return
		}
		tx, err := a.db.Begin()
		if err != nil {
			writeError(w, err, 500)
			return
		}
		defer tx.Rollback()
		result, err := tx.Exec(`DELETE FROM catalog_opportunity_items_v273516 WHERE tenant_id=? AND opportunity_id=? AND product_id=?`, tenant, opportunity, product)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		n, _ := result.RowsAffected()
		if n == 0 {
			writeError(w, errors.New("asociación no encontrada"), 404)
			return
		}
		now := time.Now().UTC().Format(time.RFC3339)
		if _, err = tx.Exec(`INSERT INTO catalog_audit_v273516(tenant_id,actor_id,action,entity_type,entity_id,details,created_at)
            VALUES(?,?,'unlink','opportunity',?,?,?)`, tenant, u.ID, opportunity, fmt.Sprintf("product_id=%d", product), now); err != nil {
			writeError(w, err, 500)
			return
		}
		if err = tx.Commit(); err != nil {
			writeError(w, err, 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	default:
		http.Error(w, "Método no permitido", 405)
	}
}
