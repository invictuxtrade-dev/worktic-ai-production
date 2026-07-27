package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type AIAgent struct {
	ID            int64  `json:"id"`
	TenantID      int64  `json:"tenant_id"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	Description   string `json:"description"`
	Objective     string `json:"objective"`
	Tone          string `json:"tone"`
	Language      string `json:"language"`
	Instructions  string `json:"instructions"`
	Knowledge     string `json:"knowledge"`
	Greeting      string `json:"greeting"`
	AwayMessage   string `json:"away_message"`
	HandoffRules  string `json:"handoff_rules"`
	Tools         string `json:"tools"`
	Channels      string `json:"channels"`
	Status        string `json:"status"`
	IsDefault     bool   `json:"is_default"`
	MonthlyBudget int    `json:"monthly_budget"`
	UsedResponses int    `json:"used_responses"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

type AgentRoute struct {
	ID         int64  `json:"id"`
	TenantID   int64  `json:"tenant_id"`
	AgentID    int64  `json:"agent_id"`
	Name       string `json:"name"`
	Priority   int    `json:"priority"`
	Channel    string `json:"channel"`
	Intent     string `json:"intent"`
	Keyword    string `json:"keyword"`
	CampaignID int64  `json:"campaign_id"`
	LandingID  int64  `json:"landing_id"`
	GroupID    int64  `json:"group_id"`
	Enabled    bool   `json:"enabled"`
}

func initMultiAgentSchema(db *sql.DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS ai_agents (
 id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL, name TEXT NOT NULL,
 type TEXT NOT NULL DEFAULT 'general', description TEXT NOT NULL DEFAULT '', objective TEXT NOT NULL DEFAULT '',
 tone TEXT NOT NULL DEFAULT 'Profesional y cercano', language TEXT NOT NULL DEFAULT 'es',
 instructions TEXT NOT NULL DEFAULT '', knowledge TEXT NOT NULL DEFAULT '', greeting TEXT NOT NULL DEFAULT '',
 away_message TEXT NOT NULL DEFAULT '', handoff_rules TEXT NOT NULL DEFAULT '', tools TEXT NOT NULL DEFAULT 'contact,opportunity,appointment,handoff',
 channels TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'draft', is_default INTEGER NOT NULL DEFAULT 0,
 monthly_budget INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_ai_agents_tenant ON ai_agents(tenant_id,status);
CREATE TABLE IF NOT EXISTS ai_agent_routes (
 id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL, agent_id INTEGER NOT NULL,
 name TEXT NOT NULL, priority INTEGER NOT NULL DEFAULT 100, channel TEXT NOT NULL DEFAULT '*',
 intent TEXT NOT NULL DEFAULT '', keyword TEXT NOT NULL DEFAULT '', campaign_id INTEGER NOT NULL DEFAULT 0,
 landing_id INTEGER NOT NULL DEFAULT 0, group_id INTEGER NOT NULL DEFAULT 0, enabled INTEGER NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_ai_agent_routes_tenant ON ai_agent_routes(tenant_id,enabled,priority);
CREATE TABLE IF NOT EXISTS ai_agent_usage (
 id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL, agent_id INTEGER NOT NULL,
 period TEXT NOT NULL, channel TEXT NOT NULL DEFAULT '', conversations INTEGER NOT NULL DEFAULT 0,
 responses INTEGER NOT NULL DEFAULT 0, input_tokens INTEGER NOT NULL DEFAULT 0, output_tokens INTEGER NOT NULL DEFAULT 0,
 errors INTEGER NOT NULL DEFAULT 0, estimated_cost REAL NOT NULL DEFAULT 0,
 UNIQUE(tenant_id,agent_id,period,channel)
);
CREATE TABLE IF NOT EXISTS ai_agent_permissions (
 id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL, agent_id INTEGER NOT NULL,
 user_id INTEGER NOT NULL, can_view INTEGER NOT NULL DEFAULT 1, can_edit INTEGER NOT NULL DEFAULT 0,
 can_test INTEGER NOT NULL DEFAULT 1, can_pause INTEGER NOT NULL DEFAULT 0,
 UNIQUE(tenant_id,agent_id,user_id)
);
CREATE TABLE IF NOT EXISTS ai_agent_audit (
 id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL, agent_id INTEGER NOT NULL,
 user_id INTEGER NOT NULL, action TEXT NOT NULL, detail TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL
);
`)
	return err
}

func agentLimit(plan string) int {
	switch strings.ToLower(plan) {
	case "personal":
		return 2
	case "business":
		return 5
	case "enterprise":
		return 15
	default:
		return 1
	}
}

func (a *App) agentTenant(r *http.Request) (int64, *User, error) {
	return a.tenantForRequest(r)
}

// migrateAgentTenants normaliza instalaciones anteriores donde ai_agents.tenant_id
// guardaba el ID del propietario en vez del tenant empresarial real.
func migrateAgentTenants(db *sql.DB) error {
	rows, err := db.Query(`SELECT id,tenant_id FROM ai_agents`)
	if err != nil {
		return err
	}
	defer rows.Close()
	type pair struct{ id, oldTenant int64 }
	var agents []pair
	for rows.Next() {
		var x pair
		if rows.Scan(&x.id, &x.oldTenant) == nil {
			agents = append(agents, x)
		}
	}
	for _, x := range agents {
		var realTenant int64
		if db.QueryRow(`SELECT tenant_id FROM app_users WHERE id=?`, x.oldTenant).Scan(&realTenant) != nil || realTenant == 0 || realTenant == x.oldTenant {
			continue
		}
		_, _ = db.Exec(`UPDATE ai_agents SET tenant_id=? WHERE id=?`, realTenant, x.id)
		_, _ = db.Exec(`UPDATE ai_agent_routes SET tenant_id=? WHERE agent_id=?`, realTenant, x.id)
		_, _ = db.Exec(`UPDATE ai_agent_usage SET tenant_id=? WHERE agent_id=?`, realTenant, x.id)
		_, _ = db.Exec(`UPDATE ai_agent_permissions SET tenant_id=? WHERE agent_id=?`, realTenant, x.id)
		_, _ = db.Exec(`UPDATE ai_agent_audit SET tenant_id=? WHERE agent_id=?`, realTenant, x.id)
		_, _ = db.Exec(`UPDATE channel_connections SET assigned_agent_id=? WHERE tenant_id=? AND assigned_agent_id=?`, x.id, realTenant, x.id)
	}
	return nil
}

func canManageAgents(role string) bool {
	return role == "superadmin" || role == "owner" || role == "admin"
}

func (a *App) agentsHandler(w http.ResponseWriter, r *http.Request) {
	tenant, u, err := a.agentTenant(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	switch r.Method {
	case http.MethodGet:
		rows, e := a.db.Query(`SELECT a.id,a.tenant_id,a.name,a.type,a.description,a.objective,a.tone,a.language,a.instructions,a.knowledge,a.greeting,a.away_message,a.handoff_rules,a.tools,a.channels,a.status,a.is_default,a.monthly_budget,a.created_at,a.updated_at,
          COALESCE((SELECT SUM(responses) FROM ai_agent_usage x WHERE x.tenant_id=a.tenant_id AND x.agent_id=a.id AND x.period=?),0)
          FROM ai_agents a WHERE a.tenant_id=? ORDER BY a.is_default DESC,a.id`, time.Now().UTC().Format("2006-01"), tenant)
		if e != nil {
			writeError(w, e, 500)
			return
		}
		defer rows.Close()
		out := []AIAgent{}
		for rows.Next() {
			var x AIAgent
			var d int
			_ = rows.Scan(&x.ID, &x.TenantID, &x.Name, &x.Type, &x.Description, &x.Objective, &x.Tone, &x.Language, &x.Instructions, &x.Knowledge, &x.Greeting, &x.AwayMessage, &x.HandoffRules, &x.Tools, &x.Channels, &x.Status, &d, &x.MonthlyBudget, &x.CreatedAt, &x.UpdatedAt, &x.UsedResponses)
			x.IsDefault = d == 1
			out = append(out, x)
		}
		p, _, _ := a.activePlan(tenant)
		writeJSON(w, map[string]any{"agents": out, "limit": agentLimit(p.Code), "plan": p.Name, "can_manage": canManageAgents(u.Role)})
	case http.MethodPost:
		if !canManageAgents(u.Role) {
			writeError(w, errors.New("sin permiso para crear agentes"), 403)
			return
		}
		p, _, _ := a.activePlan(tenant)
		var count int
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM ai_agents WHERE tenant_id=?`, tenant).Scan(&count)
		if count >= agentLimit(p.Code) {
			writeError(w, fmt.Errorf("tu plan permite hasta %d agentes", agentLimit(p.Code)), 403)
			return
		}
		var q AIAgent
		_ = json.NewDecoder(r.Body).Decode(&q)
		if strings.TrimSpace(q.Name) == "" {
			writeError(w, errors.New("nombre obligatorio"), 400)
			return
		}
		now := time.Now().UTC().Format(time.RFC3339)
		def := 0
		if count == 0 || q.IsDefault {
			def = 1
			_, _ = a.db.Exec(`UPDATE ai_agents SET is_default=0 WHERE tenant_id=?`, tenant)
		}
		if q.Status == "" {
			q.Status = "draft"
		}
		if q.Language == "" {
			q.Language = "es"
		}
		if q.Tone == "" {
			q.Tone = "Profesional y cercano"
		}
		if q.Type == "" {
			q.Type = "general"
		}
		res, e := a.db.Exec(`INSERT INTO ai_agents(tenant_id,name,type,description,objective,tone,language,instructions,knowledge,greeting,away_message,handoff_rules,tools,channels,status,is_default,monthly_budget,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, tenant, q.Name, q.Type, q.Description, q.Objective, q.Tone, q.Language, q.Instructions, q.Knowledge, q.Greeting, q.AwayMessage, q.HandoffRules, q.Tools, q.Channels, q.Status, def, q.MonthlyBudget, now, now)
		if e != nil {
			writeError(w, e, 500)
			return
		}
		id, _ := res.LastInsertId()
		a.auditAgent(tenant, id, u.ID, "create", q.Name)
		writeJSON(w, map[string]any{"ok": true, "id": id})
	case http.MethodPut:
		if !canManageAgents(u.Role) {
			writeError(w, errors.New("sin permiso para editar agentes"), 403)
			return
		}
		var q AIAgent
		_ = json.NewDecoder(r.Body).Decode(&q)
		if q.ID == 0 {
			writeError(w, errors.New("agente inválido"), 400)
			return
		}
		d := 0
		if q.IsDefault {
			d = 1
			_, _ = a.db.Exec(`UPDATE ai_agents SET is_default=0 WHERE tenant_id=?`, tenant)
		}
		res, e := a.db.Exec(`UPDATE ai_agents SET name=?,type=?,description=?,objective=?,tone=?,language=?,instructions=?,knowledge=?,greeting=?,away_message=?,handoff_rules=?,tools=?,channels=?,status=?,is_default=?,monthly_budget=?,updated_at=? WHERE id=? AND tenant_id=?`, q.Name, q.Type, q.Description, q.Objective, q.Tone, q.Language, q.Instructions, q.Knowledge, q.Greeting, q.AwayMessage, q.HandoffRules, q.Tools, q.Channels, q.Status, d, q.MonthlyBudget, time.Now().UTC().Format(time.RFC3339), q.ID, tenant)
		if e != nil {
			writeError(w, e, 500)
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			writeError(w, errors.New("agente no encontrado"), 404)
			return
		}
		a.auditAgent(tenant, q.ID, u.ID, "update", q.Name)
		writeJSON(w, map[string]any{"ok": true})
	case http.MethodDelete:
		if !canManageAgents(u.Role) {
			writeError(w, errors.New("sin permiso para eliminar agentes"), 403)
			return
		}
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		var isDefault int
		if a.db.QueryRow(`SELECT is_default FROM ai_agents WHERE id=? AND tenant_id=?`, id, tenant).Scan(&isDefault) != nil {
			writeError(w, errors.New("agente no encontrado"), 404)
			return
		}
		if isDefault == 1 {
			writeError(w, errors.New("define otro agente principal antes de eliminar este"), 400)
			return
		}
		_, _ = a.db.Exec(`DELETE FROM ai_agent_routes WHERE tenant_id=? AND agent_id=?`, tenant, id)
		_, _ = a.db.Exec(`DELETE FROM ai_agent_permissions WHERE tenant_id=? AND agent_id=?`, tenant, id)
		_, e := a.db.Exec(`DELETE FROM ai_agents WHERE tenant_id=? AND id=?`, tenant, id)
		if e != nil {
			writeError(w, e, 500)
			return
		}
		a.auditAgent(tenant, id, u.ID, "delete", "")
		writeJSON(w, map[string]any{"ok": true})
	default:
		http.Error(w, "Método no permitido", 405)
	}
}

func (a *App) agentInstanceTestHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	tenant, u, err := a.agentTenant(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	var q struct {
		AgentID int64  `json:"agent_id"`
		Text    string `json:"text"`
	}
	_ = json.NewDecoder(r.Body).Decode(&q)
	if strings.TrimSpace(q.Text) == "" {
		writeError(w, errors.New("mensaje obligatorio"), 400)
		return
	}
	var ag AIAgent
	var d int
	err = a.db.QueryRow(`SELECT id,tenant_id,name,type,description,objective,tone,language,instructions,knowledge,greeting,away_message,handoff_rules,tools,channels,status,is_default,monthly_budget,created_at,updated_at FROM ai_agents WHERE id=? AND tenant_id=?`, q.AgentID, tenant).Scan(&ag.ID, &ag.TenantID, &ag.Name, &ag.Type, &ag.Description, &ag.Objective, &ag.Tone, &ag.Language, &ag.Instructions, &ag.Knowledge, &ag.Greeting, &ag.AwayMessage, &ag.HandoffRules, &ag.Tools, &ag.Channels, &ag.Status, &d, &ag.MonthlyBudget, &ag.CreatedAt, &ag.UpdatedAt)
	if err != nil {
		writeError(w, errors.New("agente no encontrado"), 404)
		return
	}
	if a.openAIKey() == "" {
		writeJSON(w, map[string]any{"reply": fmt.Sprintf("[%s - simulación local] Recibí: %s", ag.Name, q.Text), "simulated": true})
		return
	}
	system := fmt.Sprintf("Eres %s, agente %s. Objetivo: %s. Tono: %s. Instrucciones: %s. Conocimiento: %s. Herramientas permitidas: %s. Responde en %s.", ag.Name, ag.Type, ag.Objective, ag.Tone, ag.Instructions, ag.Knowledge, ag.Tools, ag.Language)
	reply, e := a.callOpenAI(system, q.Text)
	if e != nil {
		writeError(w, e, 502)
		return
	}
	period := time.Now().UTC().Format("2006-01")
	_, _ = a.db.Exec(`INSERT INTO ai_agent_usage(tenant_id,agent_id,period,channel,conversations,responses) VALUES(?,?,?,'simulator',1,1) ON CONFLICT(tenant_id,agent_id,period,channel) DO UPDATE SET conversations=conversations+1,responses=responses+1`, tenant, ag.ID, period)
	a.auditAgent(tenant, ag.ID, u.ID, "test", q.Text)
	writeJSON(w, map[string]any{"reply": reply})
}

func (a *App) agentRoutesHandler(w http.ResponseWriter, r *http.Request) {
	tenant, u, err := a.agentTenant(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	switch r.Method {
	case http.MethodGet:
		rows, e := a.db.Query(`SELECT id,tenant_id,agent_id,name,priority,channel,intent,keyword,campaign_id,landing_id,group_id,enabled FROM ai_agent_routes WHERE tenant_id=? ORDER BY priority,id`, tenant)
		if e != nil {
			writeError(w, e, 500)
			return
		}
		defer rows.Close()
		out := []AgentRoute{}
		for rows.Next() {
			var x AgentRoute
			var en int
			_ = rows.Scan(&x.ID, &x.TenantID, &x.AgentID, &x.Name, &x.Priority, &x.Channel, &x.Intent, &x.Keyword, &x.CampaignID, &x.LandingID, &x.GroupID, &en)
			x.Enabled = en == 1
			out = append(out, x)
		}
		writeJSON(w, out)
	case http.MethodPost:
		if !canManageAgents(u.Role) {
			writeError(w, errors.New("sin permiso"), 403)
			return
		}
		var q AgentRoute
		_ = json.NewDecoder(r.Body).Decode(&q)
		if q.AgentID == 0 || strings.TrimSpace(q.Name) == "" {
			writeError(w, errors.New("nombre y agente son obligatorios"), 400)
			return
		}
		en := 0
		if q.Enabled {
			en = 1
		}
		res, e := a.db.Exec(`INSERT INTO ai_agent_routes(tenant_id,agent_id,name,priority,channel,intent,keyword,campaign_id,landing_id,group_id,enabled) SELECT ?,?,?,?,?,?,?,?,?,?,? WHERE EXISTS(SELECT 1 FROM ai_agents WHERE id=? AND tenant_id=?)`, tenant, q.AgentID, q.Name, q.Priority, q.Channel, q.Intent, q.Keyword, q.CampaignID, q.LandingID, q.GroupID, en, q.AgentID, tenant)
		if e != nil {
			writeError(w, e, 500)
			return
		}
		id, _ := res.LastInsertId()
		writeJSON(w, map[string]any{"ok": true, "id": id})
	case http.MethodDelete:
		if !canManageAgents(u.Role) {
			writeError(w, errors.New("sin permiso"), 403)
			return
		}
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		_, e := a.db.Exec(`DELETE FROM ai_agent_routes WHERE id=? AND tenant_id=?`, id, tenant)
		if e != nil {
			writeError(w, e, 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	default:
		http.Error(w, "Método no permitido", 405)
	}
}

func (a *App) agentMetricsHandler(w http.ResponseWriter, r *http.Request) {
	tenant, _, err := a.agentTenant(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	period := r.URL.Query().Get("period")
	if period == "" {
		period = time.Now().UTC().Format("2006-01")
	}
	rows, e := a.db.Query(`SELECT a.id,a.name,COALESCE(SUM(u.conversations),0),COALESCE(SUM(u.responses),0),COALESCE(SUM(u.input_tokens),0),COALESCE(SUM(u.output_tokens),0),COALESCE(SUM(u.errors),0),COALESCE(SUM(u.estimated_cost),0) FROM ai_agents a LEFT JOIN ai_agent_usage u ON u.agent_id=a.id AND u.tenant_id=a.tenant_id AND u.period=? WHERE a.tenant_id=? GROUP BY a.id,a.name ORDER BY responses DESC`, period, tenant)
	if e != nil {
		writeError(w, e, 500)
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id int64
		var name string
		var c, res, inTok, outTok, errs int
		var cost float64
		_ = rows.Scan(&id, &name, &c, &res, &inTok, &outTok, &errs, &cost)
		out = append(out, map[string]any{"agent_id": id, "name": name, "conversations": c, "responses": res, "input_tokens": inTok, "output_tokens": outTok, "errors": errs, "estimated_cost": cost})
	}
	writeJSON(w, map[string]any{"period": period, "metrics": out})
}

func (a *App) agentPermissionsHandler(w http.ResponseWriter, r *http.Request) {
	tenant, u, err := a.agentTenant(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	if !canManageAgents(u.Role) {
		writeError(w, errors.New("sin permiso"), 403)
		return
	}
	if r.Method == http.MethodGet {
		rows, e := a.db.Query(`SELECT p.id,p.agent_id,p.user_id,u.name,u.role,p.can_view,p.can_edit,p.can_test,p.can_pause FROM ai_agent_permissions p JOIN app_users u ON u.id=p.user_id WHERE p.tenant_id=? ORDER BY p.agent_id,u.name`, tenant)
		if e != nil {
			writeError(w, e, 500)
			return
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var id, aid, uid int64
			var name, role string
			var v, e1, t, p int
			_ = rows.Scan(&id, &aid, &uid, &name, &role, &v, &e1, &t, &p)
			out = append(out, map[string]any{"id": id, "agent_id": aid, "user_id": uid, "name": name, "role": role, "can_view": v == 1, "can_edit": e1 == 1, "can_test": t == 1, "can_pause": p == 1})
		}
		writeJSON(w, out)
		return
	}
	if r.Method == http.MethodPost {
		var q struct {
			AgentID, UserID                     int64
			CanView, CanEdit, CanTest, CanPause bool
		}
		_ = json.NewDecoder(r.Body).Decode(&q)
		b := func(x bool) int {
			if x {
				return 1
			}
			return 0
		}
		_, e := a.db.Exec(`INSERT INTO ai_agent_permissions(tenant_id,agent_id,user_id,can_view,can_edit,can_test,can_pause) VALUES(?,?,?,?,?,?,?) ON CONFLICT(tenant_id,agent_id,user_id) DO UPDATE SET can_view=excluded.can_view,can_edit=excluded.can_edit,can_test=excluded.can_test,can_pause=excluded.can_pause`, tenant, q.AgentID, q.UserID, b(q.CanView), b(q.CanEdit), b(q.CanTest), b(q.CanPause))
		if e != nil {
			writeError(w, e, 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
		return
	}
	http.Error(w, "Método no permitido", 405)
}

func (a *App) auditAgent(tenant, agent, user int64, action, detail string) {
	_, _ = a.db.Exec(`INSERT INTO ai_agent_audit(tenant_id,agent_id,user_id,action,detail,created_at) VALUES(?,?,?,?,?,?)`, tenant, agent, user, action, detail, time.Now().UTC().Format(time.RFC3339))
}

// resolveAgent selects the most specific enabled route and falls back to the default agent.
func (a *App) resolveAgent(tenant int64, channel, intent, text string, campaign, landing, group int64) (int64, error) {
	rows, e := a.db.Query(`SELECT agent_id,channel,intent,keyword,campaign_id,landing_id,group_id FROM ai_agent_routes WHERE tenant_id=? AND enabled=1 ORDER BY priority,id`, tenant)
	if e != nil {
		return 0, e
	}
	defer rows.Close()
	for rows.Next() {
		var aid, c, l, g int64
		var ch, in, kw string
		_ = rows.Scan(&aid, &ch, &in, &kw, &c, &l, &g)
		if ch != "" && ch != "*" && !strings.EqualFold(ch, channel) {
			continue
		}
		if in != "" && !strings.EqualFold(in, intent) {
			continue
		}
		if kw != "" && !strings.Contains(strings.ToLower(text), strings.ToLower(kw)) {
			continue
		}
		if c != 0 && c != campaign {
			continue
		}
		if l != 0 && l != landing {
			continue
		}
		if g != 0 && g != group {
			continue
		}
		return aid, nil
	}
	var id int64
	e = a.db.QueryRow(`SELECT id FROM ai_agents WHERE tenant_id=? AND is_default=1 AND status='active' ORDER BY id LIMIT 1`, tenant).Scan(&id)
	return id, e
}

func (a *App) callOpenAI(system, user string) (string, error) {
	payload := map[string]any{"model": a.cfg.OpenAIModel, "input": system + "\n\nMensaje del usuario: " + user, "store": false}
	b, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, "https://api.openai.com/v1/responses", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+a.openAIKey())
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("OpenAI: %s", strings.TrimSpace(string(raw)))
	}
	var x struct {
		Output []struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
	}
	if json.Unmarshal(raw, &x) != nil {
		return "", errors.New("respuesta OpenAI inválida")
	}
	for _, o := range x.Output {
		for _, c := range o.Content {
			if c.Type == "output_text" && c.Text != "" {
				return strings.TrimSpace(c.Text), nil
			}
		}
	}
	return "", errors.New("OpenAI no devolvió texto")
}
