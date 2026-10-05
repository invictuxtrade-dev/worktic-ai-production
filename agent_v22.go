package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type AgentV22Profile struct {
	AgentID               int64  `json:"agent_id"`
	RoleName              string `json:"role_name"`
	BusinessHours         string `json:"business_hours"`
	MemoryEnabled         bool   `json:"memory_enabled"`
	MemoryScope           string `json:"memory_scope"`
	CatalogEnabled        bool   `json:"catalog_enabled"`
	CRMEnabled            bool   `json:"crm_enabled"`
	AgendaEnabled         bool   `json:"agenda_enabled"`
	PipelineEnabled       bool   `json:"pipeline_enabled"`
	TasksEnabled          bool   `json:"tasks_enabled"`
	HumanHandoffEnabled   bool   `json:"human_handoff_enabled"`
	AutoHandoffKeywords   string `json:"auto_handoff_keywords"`
	MaxTurnsBeforeHandoff int    `json:"max_turns_before_handoff"`
}

func initAgentV22Schema(db *DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS ai_agent_profiles_v22 (
 tenant_id INTEGER NOT NULL, agent_id INTEGER NOT NULL,
 role_name TEXT NOT NULL DEFAULT '', business_hours TEXT NOT NULL DEFAULT '{}',
 memory_enabled INTEGER NOT NULL DEFAULT 1, memory_scope TEXT NOT NULL DEFAULT 'contact',
 catalog_enabled INTEGER NOT NULL DEFAULT 1, crm_enabled INTEGER NOT NULL DEFAULT 1,
 agenda_enabled INTEGER NOT NULL DEFAULT 1, pipeline_enabled INTEGER NOT NULL DEFAULT 1,
 tasks_enabled INTEGER NOT NULL DEFAULT 1, human_handoff_enabled INTEGER NOT NULL DEFAULT 1,
 auto_handoff_keywords TEXT NOT NULL DEFAULT 'humano,asesor,persona,representante',
 max_turns_before_handoff INTEGER NOT NULL DEFAULT 0,
 updated_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,agent_id)
);
CREATE TABLE IF NOT EXISTS ai_agent_memory_v22 (
 id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL, agent_id INTEGER NOT NULL,
 contact_key TEXT NOT NULL, contact_id INTEGER NOT NULL DEFAULT 0,
 summary TEXT NOT NULL DEFAULT '', facts_json TEXT NOT NULL DEFAULT '[]',
 preferences_json TEXT NOT NULL DEFAULT '{}', last_message TEXT NOT NULL DEFAULT '',
 turn_count INTEGER NOT NULL DEFAULT 0, last_seen_at TEXT NOT NULL, updated_at TEXT NOT NULL,
 UNIQUE(tenant_id,agent_id,contact_key)
);
CREATE INDEX IF NOT EXISTS idx_ai_memory_tenant_contact ON ai_agent_memory_v22(tenant_id,contact_key,updated_at);
CREATE TABLE IF NOT EXISTS ai_agent_knowledge_v22 (
 id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL, agent_id INTEGER NOT NULL DEFAULT 0,
 title TEXT NOT NULL, kind TEXT NOT NULL DEFAULT 'note', source_ref TEXT NOT NULL DEFAULT '',
 content TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'active', created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_ai_knowledge_tenant ON ai_agent_knowledge_v22(tenant_id,agent_id,status);
CREATE TABLE IF NOT EXISTS ai_agent_tool_audit_v22 (
 id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL, agent_id INTEGER NOT NULL,
 contact_key TEXT NOT NULL DEFAULT '', tool TEXT NOT NULL, action TEXT NOT NULL,
 input_json TEXT NOT NULL DEFAULT '{}', output_json TEXT NOT NULL DEFAULT '{}', status TEXT NOT NULL DEFAULT 'ok',
 created_at TEXT NOT NULL
);
`)
	return err
}

func (a *App) agentV22OverviewHandler(w http.ResponseWriter, r *http.Request) {
	tenant, _, err := a.agentTenant(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	var agents, active, memories, knowledge int
	_ = a.db.QueryRow(`SELECT COUNT(*),COALESCE(SUM(CASE WHEN status='active' THEN 1 ELSE 0 END),0) FROM ai_agents WHERE tenant_id=?`, tenant).Scan(&agents, &active)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM ai_agent_memory_v22 WHERE tenant_id=?`, tenant).Scan(&memories)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM ai_agent_knowledge_v22 WHERE tenant_id=? AND status='active'`, tenant).Scan(&knowledge)
	var principalID int64
	var principalName string
	_ = a.db.QueryRow(`SELECT id,name FROM ai_agents WHERE tenant_id=? AND is_default=1 ORDER BY id LIMIT 1`, tenant).Scan(&principalID, &principalName)
	writeJSON(w, map[string]any{"agents": agents, "active": active, "memories": memories, "knowledge": knowledge, "principal_id": principalID, "principal_name": principalName})
}

func (a *App) agentV22ProfileHandler(w http.ResponseWriter, r *http.Request) {
	tenant, u, err := a.agentTenant(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	agentID, _ := strconv.ParseInt(r.URL.Query().Get("agent_id"), 10, 64)
	if agentID == 0 {
		writeError(w, errors.New("agent_id obligatorio"), 400)
		return
	}
	if r.Method == http.MethodGet {
		var p AgentV22Profile
		var m, c, cr, ag, pl, ta, hh int
		e := a.db.QueryRow(`SELECT agent_id,role_name,business_hours,memory_enabled,memory_scope,catalog_enabled,crm_enabled,agenda_enabled,pipeline_enabled,tasks_enabled,human_handoff_enabled,auto_handoff_keywords,max_turns_before_handoff FROM ai_agent_profiles_v22 WHERE tenant_id=? AND agent_id=?`, tenant, agentID).Scan(&p.AgentID, &p.RoleName, &p.BusinessHours, &m, &p.MemoryScope, &c, &cr, &ag, &pl, &ta, &hh, &p.AutoHandoffKeywords, &p.MaxTurnsBeforeHandoff)
		if e == sql.ErrNoRows {
			p = AgentV22Profile{AgentID: agentID, MemoryEnabled: true, MemoryScope: "contact", CatalogEnabled: true, CRMEnabled: true, AgendaEnabled: true, PipelineEnabled: true, TasksEnabled: true, HumanHandoffEnabled: true, AutoHandoffKeywords: "humano,asesor,persona,representante"}
		} else if e != nil {
			writeError(w, e, 500)
			return
		}
		p.MemoryEnabled = m == 1 || e == sql.ErrNoRows
		p.CatalogEnabled = c == 1 || e == sql.ErrNoRows
		p.CRMEnabled = cr == 1 || e == sql.ErrNoRows
		p.AgendaEnabled = ag == 1 || e == sql.ErrNoRows
		p.PipelineEnabled = pl == 1 || e == sql.ErrNoRows
		p.TasksEnabled = ta == 1 || e == sql.ErrNoRows
		p.HumanHandoffEnabled = hh == 1 || e == sql.ErrNoRows
		writeJSON(w, p)
		return
	}
	if r.Method == http.MethodPost || r.Method == http.MethodPut {
		if !canManageAgents(u.Role) {
			writeError(w, errors.New("sin permiso"), 403)
			return
		}
		var p AgentV22Profile
		_ = json.NewDecoder(r.Body).Decode(&p)
		p.AgentID = agentID
		if p.MemoryScope == "" {
			p.MemoryScope = "contact"
		}
		now := time.Now().UTC().Format(time.RFC3339)
		_, e := a.db.Exec(`INSERT INTO ai_agent_profiles_v22(tenant_id,agent_id,role_name,business_hours,memory_enabled,memory_scope,catalog_enabled,crm_enabled,agenda_enabled,pipeline_enabled,tasks_enabled,human_handoff_enabled,auto_handoff_keywords,max_turns_before_handoff,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(tenant_id,agent_id) DO UPDATE SET role_name=excluded.role_name,business_hours=excluded.business_hours,memory_enabled=excluded.memory_enabled,memory_scope=excluded.memory_scope,catalog_enabled=excluded.catalog_enabled,crm_enabled=excluded.crm_enabled,agenda_enabled=excluded.agenda_enabled,pipeline_enabled=excluded.pipeline_enabled,tasks_enabled=excluded.tasks_enabled,human_handoff_enabled=excluded.human_handoff_enabled,auto_handoff_keywords=excluded.auto_handoff_keywords,max_turns_before_handoff=excluded.max_turns_before_handoff,updated_at=excluded.updated_at`, tenant, agentID, p.RoleName, p.BusinessHours, boolInt(p.MemoryEnabled), p.MemoryScope, boolInt(p.CatalogEnabled), boolInt(p.CRMEnabled), boolInt(p.AgendaEnabled), boolInt(p.PipelineEnabled), boolInt(p.TasksEnabled), boolInt(p.HumanHandoffEnabled), p.AutoHandoffKeywords, p.MaxTurnsBeforeHandoff, now)
		if e != nil {
			writeError(w, e, 500)
			return
		}
		a.auditAgent(tenant, agentID, u.ID, "v22_profile_update", p.RoleName)
		writeJSON(w, map[string]any{"ok": true})
		return
	}
	http.Error(w, "Método no permitido", 405)
}

func (a *App) agentV22KnowledgeHandler(w http.ResponseWriter, r *http.Request) {
	tenant, u, err := a.agentTenant(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	switch r.Method {
	case http.MethodGet:
		aid, _ := strconv.ParseInt(r.URL.Query().Get("agent_id"), 10, 64)
		rows, e := a.db.Query(`SELECT id,agent_id,title,kind,source_ref,content,status,created_at,updated_at FROM ai_agent_knowledge_v22 WHERE tenant_id=? AND (?=0 OR agent_id=0 OR agent_id=?) ORDER BY id DESC`, tenant, aid, aid)
		if e != nil {
			writeError(w, e, 500)
			return
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var id, ag int64
			var title, kind, src, content, status, ca, ua string
			_ = rows.Scan(&id, &ag, &title, &kind, &src, &content, &status, &ca, &ua)
			out = append(out, map[string]any{"id": id, "agent_id": ag, "title": title, "kind": kind, "source_ref": src, "content": content, "status": status, "created_at": ca, "updated_at": ua})
		}
		writeJSON(w, out)
	case http.MethodPost:
		if !canManageAgents(u.Role) {
			writeError(w, errors.New("sin permiso"), 403)
			return
		}
		var q struct {
			AgentID                         int64 `json:"agent_id"`
			Title, Kind, SourceRef, Content string
		}
		_ = json.NewDecoder(r.Body).Decode(&q)
		if strings.TrimSpace(q.Title) == "" || strings.TrimSpace(q.Content) == "" {
			writeError(w, errors.New("título y contenido obligatorios"), 400)
			return
		}
		if q.Kind == "" {
			q.Kind = "note"
		}
		now := time.Now().UTC().Format(time.RFC3339)
		res, e := a.db.Exec(`INSERT INTO ai_agent_knowledge_v22(tenant_id,agent_id,title,kind,source_ref,content,status,created_at,updated_at) VALUES(?,?,?,?,?,?, 'active',?,?)`, tenant, q.AgentID, q.Title, q.Kind, q.SourceRef, q.Content, now, now)
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
		_, e := a.db.Exec(`DELETE FROM ai_agent_knowledge_v22 WHERE tenant_id=? AND id=?`, tenant, id)
		if e != nil {
			writeError(w, e, 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	default:
		http.Error(w, "Método no permitido", 405)
	}
}

func (a *App) agentV22MemoryHandler(w http.ResponseWriter, r *http.Request) {
	tenant, u, err := a.agentTenant(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	switch r.Method {
	case http.MethodGet:
		aid, _ := strconv.ParseInt(r.URL.Query().Get("agent_id"), 10, 64)
		rows, e := a.db.Query(`SELECT id,agent_id,contact_key,contact_id,summary,facts_json,preferences_json,last_message,turn_count,last_seen_at,updated_at FROM ai_agent_memory_v22 WHERE tenant_id=? AND (?=0 OR agent_id=?) ORDER BY updated_at DESC LIMIT 200`, tenant, aid, aid)
		if e != nil {
			writeError(w, e, 500)
			return
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var id, ag, cid int64
			var key, sum, facts, prefs, last, seen, upd string
			var turns int
			_ = rows.Scan(&id, &ag, &key, &cid, &sum, &facts, &prefs, &last, &turns, &seen, &upd)
			out = append(out, map[string]any{"id": id, "agent_id": ag, "contact_key": key, "contact_id": cid, "summary": sum, "facts_json": facts, "preferences_json": prefs, "last_message": last, "turn_count": turns, "last_seen_at": seen, "updated_at": upd})
		}
		writeJSON(w, out)
	case http.MethodDelete:
		if !canManageAgents(u.Role) {
			writeError(w, errors.New("sin permiso"), 403)
			return
		}
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		_, e := a.db.Exec(`DELETE FROM ai_agent_memory_v22 WHERE tenant_id=? AND id=?`, tenant, id)
		if e != nil {
			writeError(w, e, 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	default:
		http.Error(w, "Método no permitido", 405)
	}
}

func (a *App) agentV22Context(tenant, agent int64, contactKey string) string {
	if agent == 0 {
		return ""
	}
	var p AgentV22Profile
	var mem, cat, crm, agenda, pipeline, tasks, handoff int
	_ = a.db.QueryRow(`SELECT agent_id,role_name,business_hours,memory_enabled,memory_scope,catalog_enabled,crm_enabled,agenda_enabled,pipeline_enabled,tasks_enabled,human_handoff_enabled,auto_handoff_keywords,max_turns_before_handoff FROM ai_agent_profiles_v22 WHERE tenant_id=? AND agent_id=?`, tenant, agent).Scan(&p.AgentID, &p.RoleName, &p.BusinessHours, &mem, &p.MemoryScope, &cat, &crm, &agenda, &pipeline, &tasks, &handoff, &p.AutoHandoffKeywords, &p.MaxTurnsBeforeHandoff)
	parts := []string{"CAPACIDADES V22:"}
	if p.RoleName != "" {
		parts = append(parts, "Rol operativo: "+p.RoleName)
	}
	tools := []string{}
	if cat == 1 {
		tools = append(tools, "catálogo")
	}
	if crm == 1 {
		tools = append(tools, "CRM")
	}
	if agenda == 1 {
		tools = append(tools, "agenda")
	}
	if pipeline == 1 {
		tools = append(tools, "pipeline")
	}
	if tasks == 1 {
		tools = append(tools, "tareas")
	}
	if handoff == 1 {
		tools = append(tools, "escalado humano")
	}
	if len(tools) > 0 {
		parts = append(parts, "Herramientas autorizadas: "+strings.Join(tools, ", "))
	}
	rows, err := a.db.Query(`SELECT title,content FROM ai_agent_knowledge_v22 WHERE tenant_id=? AND status='active' AND (agent_id=0 OR agent_id=?) ORDER BY agent_id DESC,id DESC LIMIT 8`, tenant, agent)
	if err == nil {
		defer rows.Close()
		ks := []string{}
		for rows.Next() {
			var t, c string
			_ = rows.Scan(&t, &c)
			ks = append(ks, t+": "+c)
		}
		if len(ks) > 0 {
			parts = append(parts, "Conocimiento empresarial adicional:\n"+strings.Join(ks, "\n"))
		}
	}
	if mem == 1 && contactKey != "" {
		var summary, facts string
		var turns int
		if a.db.QueryRow(`SELECT summary,facts_json,turn_count FROM ai_agent_memory_v22 WHERE tenant_id=? AND agent_id=? AND contact_key=?`, tenant, agent, contactKey).Scan(&summary, &facts, &turns) == nil {
			parts = append(parts, fmt.Sprintf("Memoria de este cliente (turnos %d): %s. Hechos: %s", turns, summary, facts))
		}
	}
	return strings.Join(parts, "\n")
}

func (a *App) agentV22Remember(tenant, agent int64, contactKey, userText, assistantText string) {
	if agent == 0 || strings.TrimSpace(contactKey) == "" {
		return
	}
	var enabled int = 1
	_ = a.db.QueryRow(`SELECT memory_enabled FROM ai_agent_profiles_v22 WHERE tenant_id=? AND agent_id=?`, tenant, agent).Scan(&enabled)
	if enabled == 0 {
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	summary := strings.TrimSpace(userText)
	if len(summary) > 500 {
		summary = summary[:500]
	}
	last := strings.TrimSpace(assistantText)
	if len(last) > 500 {
		last = last[:500]
	}
	_, _ = a.db.Exec(`INSERT INTO ai_agent_memory_v22(tenant_id,agent_id,contact_key,summary,last_message,turn_count,last_seen_at,updated_at) VALUES(?,?,?,?,?,1,?,?) ON CONFLICT(tenant_id,agent_id,contact_key) DO UPDATE SET summary=excluded.summary,last_message=excluded.last_message,turn_count=ai_agent_memory_v22.turn_count+1,last_seen_at=excluded.last_seen_at,updated_at=excluded.updated_at`, tenant, agent, contactKey, summary, last, now, now)
}
