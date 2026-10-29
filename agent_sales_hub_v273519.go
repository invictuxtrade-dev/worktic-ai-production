package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// V27.35.19: Only internal CRM changes. Does not send outbound messages,
// approve a payment, change inventory, or bypass a human-owned opportunity.
func initAgentSalesHubV273519(db *DB) error {
	for _, statement := range []string{
		`CREATE TABLE IF NOT EXISTS agent_sales_hub_settings_v273519 (
            tenant_id INTEGER NOT NULL, agent_id INTEGER NOT NULL,
            enabled INTEGER NOT NULL DEFAULT 0,
            record_activity INTEGER NOT NULL DEFAULT 1,
            create_followups INTEGER NOT NULL DEFAULT 1,
            advance_stage INTEGER NOT NULL DEFAULT 0,
            followup_hours INTEGER NOT NULL DEFAULT 24,
            updated_by INTEGER NOT NULL DEFAULT 0,
            updated_at TEXT NOT NULL DEFAULT '',
            PRIMARY KEY(tenant_id,agent_id))`,
		`CREATE TABLE IF NOT EXISTS agent_sales_hub_processed_v273519 (
            tenant_id INTEGER NOT NULL, agent_id INTEGER NOT NULL, channel TEXT NOT NULL,
            chat_hash TEXT NOT NULL, message_hash TEXT NOT NULL,
            opportunity_id INTEGER NOT NULL, created_at TEXT NOT NULL,
            PRIMARY KEY(tenant_id,agent_id,channel,chat_hash,message_hash))`,
		`CREATE TABLE IF NOT EXISTS agent_sales_hub_followups_v273519 (
            tenant_id INTEGER NOT NULL, opportunity_id INTEGER NOT NULL,
            agent_id INTEGER NOT NULL, task_id INTEGER NOT NULL,
            PRIMARY KEY(tenant_id,opportunity_id,agent_id))`,
	} {
		if _, err := db.Exec(statement); err != nil {
			return err
		}
	}
	return nil
}

type agentSalesHubSettingsV273519 struct {
	Enabled         bool `json:"enabled"`
	RecordActivity  bool `json:"record_activity"`
	CreateFollowups bool `json:"create_followups"`
	AdvanceStage    bool `json:"advance_stage"`
	FollowupHours   int  `json:"followup_hours"`
}

type agentSalesHubAccessV273519 struct {
	CRM      bool `json:"crm"`
	Tasks    bool `json:"tasks"`
	Pipeline bool `json:"pipeline"`
}

func agentSalesHubSettingsReadV273519(db *DB, tenant, agent int64) (agentSalesHubSettingsV273519, error) {
	cfg := agentSalesHubSettingsV273519{RecordActivity: true, CreateFollowups: true, FollowupHours: 24}
	var enabled, activity, tasks, stage int
	err := db.QueryRow(`SELECT enabled,record_activity,create_followups,advance_stage,followup_hours
        FROM agent_sales_hub_settings_v273519 WHERE tenant_id=? AND agent_id=?`, tenant, agent).
		Scan(&enabled, &activity, &tasks, &stage, &cfg.FollowupHours)
	if errors.Is(err, sql.ErrNoRows) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	cfg.Enabled, cfg.RecordActivity, cfg.CreateFollowups, cfg.AdvanceStage = enabled == 1, activity == 1, tasks == 1, stage == 1
	return cfg, nil
}

func agentSalesHubAccessReadV273519(db *DB, tenant, agent int64) agentSalesHubAccessV273519 {
	var crm, tasks, pipeline int
	var tools string
	err := db.QueryRow(`SELECT p.crm_enabled,p.tasks_enabled,p.pipeline_enabled,a.tools
        FROM ai_agent_profiles_v22 p JOIN ai_agents a ON a.id=p.agent_id AND a.tenant_id=p.tenant_id
        WHERE p.tenant_id=? AND p.agent_id=? AND a.status='active'`, tenant, agent).
		Scan(&crm, &tasks, &pipeline, &tools)
	if err != nil {
		return agentSalesHubAccessV273519{}
	}
	authorized := false
	for _, tool := range strings.Split(strings.ToLower(tools), ",") {
		if strings.TrimSpace(tool) == "opportunity" {
			authorized = true
			break
		}
	}
	return agentSalesHubAccessV273519{CRM: crm == 1 && authorized, Tasks: tasks == 1 && authorized, Pipeline: pipeline == 1 && authorized}
}

func (a *App) agentSalesHubSettingsHandlerV273519(w http.ResponseWriter, r *http.Request) {
	tenant, u, err := a.tenantForRequest(r)
	if err != nil || tenant <= 0 || u == nil {
		writeError(w, errors.New("sesión requerida"), 401)
		return
	}
	if !canManageAgents(u.Role) {
		writeError(w, errors.New("permiso de administración de agentes requerido"), 403)
		return
	}
	agent, _ := strconv.ParseInt(r.URL.Query().Get("agent_id"), 10, 64)
	if agent <= 0 {
		writeError(w, errors.New("selecciona un agente"), 400)
		return
	}
	var id int64
	if err = a.db.QueryRow(`SELECT id FROM ai_agents WHERE id=? AND tenant_id=?`, agent, tenant).Scan(&id); err != nil {
		writeError(w, errors.New("agente no encontrado en tu empresa"), 404)
		return
	}
	if r.Method == http.MethodGet {
		cfg, e := agentSalesHubSettingsReadV273519(a.db, tenant, agent)
		if e != nil {
			writeError(w, e, 500)
			return
		}
		access := agentSalesHubAccessReadV273519(a.db, tenant, agent)
		writeJSON(w, map[string]any{"agent_id": agent, "settings": cfg, "permissions": access, "effective": cfg.Enabled && access.CRM})
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	var cfg agentSalesHubSettingsV273519
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048)).Decode(&cfg) != nil {
		writeError(w, errors.New("configuración inválida"), 400)
		return
	}
	if cfg.FollowupHours < 1 || cfg.FollowupHours > 168 {
		writeError(w, errors.New("el seguimiento debe ser de 1 a 168 horas"), 400)
		return
	}
	flag := func(b bool) int {
		if b {
			return 1
		}
		return 0
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = a.db.Exec(`INSERT INTO agent_sales_hub_settings_v273519
        (tenant_id,agent_id,enabled,record_activity,create_followups,advance_stage,followup_hours,updated_by,updated_at)
        VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(tenant_id,agent_id) DO UPDATE SET
        enabled=excluded.enabled,record_activity=excluded.record_activity,
        create_followups=excluded.create_followups,advance_stage=excluded.advance_stage,
        followup_hours=excluded.followup_hours,updated_by=excluded.updated_by,updated_at=excluded.updated_at`,
		tenant, agent, flag(cfg.Enabled), flag(cfg.RecordActivity), flag(cfg.CreateFollowups), flag(cfg.AdvanceStage), cfg.FollowupHours, u.ID, now)
	if err != nil {
		writeError(w, err, 500)
		return
	}
	if err = a.catalogAuditV273516(tenant, u.ID, "agent_sales_hub_settings", "agent", agent,
		fmt.Sprintf("enabled=%t activity=%t tasks=%t stage=%t hours=%d", cfg.Enabled, cfg.RecordActivity, cfg.CreateFollowups, cfg.AdvanceStage, cfg.FollowupHours)); err != nil {
		// Commercial configuration is saved but missing an audit must be visible.
		writeError(w, err, 500)
		return
	}
	access := agentSalesHubAccessReadV273519(a.db, tenant, agent)
	writeJSON(w, map[string]any{"ok": true, "settings": cfg, "permissions": access, "effective": cfg.Enabled && access.CRM})
}

func agentSalesHubSignalV273519(message string) (bool, string, string, int) {
	if len(message) > 4000 {
		return false, "", "", 0
	}
	matched, interest, stage, score := detectCommercialOpportunity(message)
	if !matched {
		return false, "", "", 0
	}
	// A lone, generic word like "producto" or "precio" does not justify an action.
	m := normalizeIntentText(message)
	explicit := []string{"me interesa", "estoy interesado", "estoy interesada", "quiero informacion", "quiero comprar",
		"quiero contratar", "como pago", "enlace de pago", "listo para comprar", "lo voy a comprar", "cotizacion", "cotizar",
		"presupuesto", "propuesta comercial", "agendar", "reservar", "quiero una cita", "demostracion", "cuanto cuesta", "cuanto vale"}
	for _, term := range explicit {
		if strings.Contains(m, term) {
			return true, interest, stage, score
		}
	}
	return false, "", "", 0
}

func agentSalesHubHashV273519(s string) string {
	b := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(s))))
	return hex.EncodeToString(b[:])
}

// Runs only after a specialized agent generated an answer to an inbound message.
// All mutations are tenant-scoped and idempotent per normalized message+chat.
func (a *App) agentSalesHubFromMessageV273519(tenant, agent int64, chat, message, channel string) error {
	if tenant <= 0 || agent <= 0 || len(chat) > 250 || strings.TrimSpace(chat) == "" {
		return nil
	}
	cfg, err := agentSalesHubSettingsReadV273519(a.db, tenant, agent)
	if err != nil {
		return err
	}
	if !cfg.Enabled {
		return nil
	}
	perms := agentSalesHubAccessReadV273519(a.db, tenant, agent)
	if !perms.CRM {
		return nil
	}
	matched, interest, stage, score := agentSalesHubSignalV273519(message)
	if !matched {
		return nil
	}
	contact, err := a.contactIDForOpportunity(tenant, 0, "", "", strings.TrimSpace(chat))
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var opp int64
	var currentStage string
	var locked int
	err = a.db.QueryRow(`SELECT id,stage,manual_lock FROM crm_opportunities WHERE tenant_id=? AND contact_id=?
        AND COALESCE(deleted_at,'')='' AND stage NOT IN ('Ganado','Perdido') ORDER BY id DESC LIMIT 1`, tenant, contact).
		Scan(&opp, &currentStage, &locked)
	if errors.Is(err, sql.ErrNoRows) {
		// No automatic pipeline progression unless the separate opt-in is enabled.
		initial := "Nuevo"
		if cfg.AdvanceStage && perms.Pipeline {
			initial = stage
		}
		opp, _, err = a.syncAutomaticOpportunity(tenant, contact, "conversation", "chat:"+strings.TrimSpace(chat), channel, interest, initial, score, "")
		if err != nil {
			return err
		}
		currentStage = initial
	} else if err != nil {
		return err
	}
	if opp <= 0 {
		return nil
	}
	now := time.Now().UTC()
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`INSERT INTO agent_sales_hub_processed_v273519
        (tenant_id,agent_id,channel,chat_hash,message_hash,opportunity_id,created_at)
        VALUES(?,?,?,?,?,?,?) ON CONFLICT(tenant_id,agent_id,channel,chat_hash,message_hash) DO NOTHING`,
		tenant, agent, strings.ToLower(channel), agentSalesHubHashV273519(chat), agentSalesHubHashV273519(message), opp, now.Format(time.RFC3339))
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return nil
	}
	if cfg.RecordActivity {
		if err = salesHubAuditTxV273518(tx, tenant, opp, 0, "agent.interest.detected",
			fmt.Sprintf("Agente #%d · %s · %s (señal de interés, no pago)", agent, channel, interest)); err != nil {
			return err
		}
	}
	if cfg.AdvanceStage && perms.Pipeline && locked == 0 && opportunityStageRank(stage) > opportunityStageRank(currentStage) {
		result, e := tx.Exec(`UPDATE crm_opportunities SET stage=?,probability=?,updated_at=?
            WHERE tenant_id=? AND id=? AND manual_lock=0 AND stage=? AND stage NOT IN ('Ganado','Perdido')`,
			stage, opportunityProbability(stage), now.Format(time.RFC3339), tenant, opp, currentStage)
		if e != nil {
			return e
		}
		changed, _ := result.RowsAffected()
		if changed > 0 {
			if err = salesHubAuditTxV273518(tx, tenant, opp, 0, "agent.stage.advanced",
				fmt.Sprintf("Agente #%d · %s → %s", agent, currentStage, stage)); err != nil {
				return err
			}
		}
	}
	if cfg.CreateFollowups && perms.Tasks {
		// One open agent task per opportunity: a new interaction does not spam tasks.
		var previous int64
		var existing sql.NullInt64
		err = tx.QueryRow(`SELECT f.task_id,t.id FROM agent_sales_hub_followups_v273519 f
            LEFT JOIN opportunity_hub_tasks_v273518 t ON t.id=f.task_id AND t.tenant_id=f.tenant_id
            AND t.opportunity_id=f.opportunity_id AND t.status='open'
            WHERE f.tenant_id=? AND f.opportunity_id=? AND f.agent_id=?`, tenant, opp, agent).Scan(&previous, &existing)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if !existing.Valid {
			// A different agent should not create a second open AI follow-up.
			var another int64
			e := tx.QueryRow(`SELECT t.id FROM agent_sales_hub_followups_v273519 f
                JOIN opportunity_hub_tasks_v273518 t ON t.id=f.task_id AND t.tenant_id=f.tenant_id
                AND t.opportunity_id=f.opportunity_id AND t.status='open'
                WHERE f.tenant_id=? AND f.opportunity_id=? LIMIT 1`, tenant, opp).Scan(&another)
			if e == nil {
				return tx.Commit()
			}
			if !errors.Is(e, sql.ErrNoRows) {
				return e
			}
			var owner int64
			e = tx.QueryRow(`SELECT owner_user_id FROM opportunity_hub_meta_v273518 WHERE tenant_id=? AND opportunity_id=?`, tenant, opp).Scan(&owner)
			if e != nil && !errors.Is(e, sql.ErrNoRows) {
				return e
			}
			if owner > 0 && !salesHubUserExistsTxV273518(tx, tenant, owner) {
				owner = 0
			}
			title := "Contactar al cliente sobre su interés comercial"
			if interest == "Solicitud de cotización" {
				title = "Preparar y dar seguimiento a cotización"
			}
			due := now.Add(time.Duration(cfg.FollowupHours) * time.Hour).Format(time.RFC3339)
			result, e := tx.Exec(`INSERT INTO opportunity_hub_tasks_v273518
                (tenant_id,opportunity_id,owner_user_id,title,due_at,status,created_by,updated_by,created_at,updated_at)
                VALUES(?,?,?,?,?,'open',0,0,?,?)`, tenant, opp, owner, title, due, now.Format(time.RFC3339), now.Format(time.RFC3339))
			if e != nil {
				return e
			}
			taskID, e := result.LastInsertId()
			if e != nil {
				return e
			}
			_, e = tx.Exec(`INSERT INTO agent_sales_hub_followups_v273519(tenant_id,opportunity_id,agent_id,task_id)
                VALUES(?,?,?,?) ON CONFLICT(tenant_id,opportunity_id,agent_id)
                DO UPDATE SET task_id=excluded.task_id`, tenant, opp, agent, taskID)
			if e != nil {
				return e
			}
			if err = salesHubAuditTxV273518(tx, tenant, opp, 0, "agent.followup.created",
				fmt.Sprintf("Agente #%d · tarea #%d · próximo seguimiento: %s", agent, taskID, due)); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}
