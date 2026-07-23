package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type GroupLimits struct {
	Plan           string `json:"plan"`
	Enabled        bool   `json:"enabled"`
	Discovery      bool   `json:"discovery"`
	MaxManaged     int    `json:"max_managed"`
	MaxProspects   int    `json:"max_prospects"`
	AIReplies      bool   `json:"ai_replies"`
	ScheduledPosts bool   `json:"scheduled_posts"`
}

type ManagedGroup struct {
	ID             int64  `json:"id"`
	TenantID       int64  `json:"tenant_id"`
	Platform       string `json:"platform"`
	ExternalID     string `json:"external_id"`
	Name           string `json:"name"`
	URL            string `json:"url"`
	Niche          string `json:"niche"`
	Status         string `json:"status"`
	BotEnabled     bool   `json:"bot_enabled"`
	WelcomeMessage string `json:"welcome_message"`
	AutoReplyMode  string `json:"auto_reply_mode"`
	CreatedAt      string `json:"created_at"`
}

func initGroupsSchema(a *App) error {
	_, err := a.db.Exec(`
CREATE TABLE IF NOT EXISTS managed_groups(
 id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL, platform TEXT NOT NULL,
 external_id TEXT NOT NULL DEFAULT '', name TEXT NOT NULL, url TEXT NOT NULL DEFAULT '', niche TEXT NOT NULL DEFAULT '',
 status TEXT NOT NULL DEFAULT 'connected', bot_enabled INTEGER NOT NULL DEFAULT 1,
 welcome_message TEXT NOT NULL DEFAULT '', auto_reply_mode TEXT NOT NULL DEFAULT 'mention', created_at TEXT NOT NULL,
 UNIQUE(tenant_id,platform,external_id)
);
CREATE TABLE IF NOT EXISTS group_prospects(
 id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL, platform TEXT NOT NULL, name TEXT NOT NULL,
 url TEXT NOT NULL DEFAULT '', niche TEXT NOT NULL DEFAULT '', notes TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'saved', created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS group_activity(
 id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL, group_id INTEGER NOT NULL DEFAULT 0,
 platform TEXT NOT NULL, action TEXT NOT NULL, detail TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL
);
`)
	return err
}

func groupLimitsFor(plan string) GroupLimits {
	switch plan {
	case "personal":
		return GroupLimits{plan, true, true, 5, 20, true, false}
	case "business":
		return GroupLimits{plan, true, true, 30, 150, true, true}
	case "enterprise":
		return GroupLimits{plan, true, true, 200, 1000, true, true}
	default:
		return GroupLimits{"free", true, false, 1, 3, false, false}
	}
}

func (a *App) groupLimits(tenant int64) GroupLimits {
	p, _, err := a.activePlan(tenant)
	if err != nil {
		return groupLimitsFor("free")
	}
	return groupLimitsFor(p.Code)
}

func (a *App) groupLimitsHandler(w http.ResponseWriter, r *http.Request) {
	t, _, err := a.tenantFor(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	writeJSON(w, a.groupLimits(t))
}

func (a *App) groupsHandler(w http.ResponseWriter, r *http.Request) {
	t, _, err := a.tenantFor(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	lim := a.groupLimits(t)
	switch r.Method {
	case http.MethodGet:
		rows, err := a.db.Query(`SELECT id,tenant_id,platform,external_id,name,url,niche,status,bot_enabled,welcome_message,auto_reply_mode,created_at FROM managed_groups WHERE tenant_id=? ORDER BY id DESC`, t)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		defer rows.Close()
		out := []ManagedGroup{}
		for rows.Next() {
			var g ManagedGroup
			var be int
			_ = rows.Scan(&g.ID, &g.TenantID, &g.Platform, &g.ExternalID, &g.Name, &g.URL, &g.Niche, &g.Status, &be, &g.WelcomeMessage, &g.AutoReplyMode, &g.CreatedAt)
			g.BotEnabled = be == 1
			out = append(out, g)
		}
		writeJSON(w, out)
	case http.MethodPost:
		var q ManagedGroup
		if json.NewDecoder(r.Body).Decode(&q) != nil || strings.TrimSpace(q.Name) == "" {
			writeError(w, errors.New("nombre obligatorio"), 400)
			return
		}
		var n int
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM managed_groups WHERE tenant_id=?`, t).Scan(&n)
		if n >= lim.MaxManaged {
			writeError(w, fmt.Errorf("tu plan permite hasta %d grupos gestionados", lim.MaxManaged), 403)
			return
		}
		p := strings.ToLower(strings.TrimSpace(q.Platform))
		if p != "whatsapp" && p != "telegram" && p != "facebook" {
			writeError(w, errors.New("plataforma inválida"), 400)
			return
		}
		ext := strings.TrimSpace(q.ExternalID)
		if ext == "" {
			ext = fmt.Sprintf("manual-%d", time.Now().UnixNano())
		}
		mode := q.AutoReplyMode
		if mode == "" {
			mode = "mention"
		}
		_, err = a.db.Exec(`INSERT INTO managed_groups(tenant_id,platform,external_id,name,url,niche,status,bot_enabled,welcome_message,auto_reply_mode,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, t, p, ext, strings.TrimSpace(q.Name), strings.TrimSpace(q.URL), strings.TrimSpace(q.Niche), "connected", boolInt(q.BotEnabled), q.WelcomeMessage, mode, time.Now().UTC().Format(time.RFC3339))
		if err != nil {
			writeError(w, err, 400)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	case http.MethodPut:
		var q ManagedGroup
		if json.NewDecoder(r.Body).Decode(&q) != nil || q.ID == 0 {
			writeError(w, errors.New("grupo inválido"), 400)
			return
		}
		_, err = a.db.Exec(`UPDATE managed_groups SET name=?,url=?,niche=?,bot_enabled=?,welcome_message=?,auto_reply_mode=? WHERE id=? AND tenant_id=?`, strings.TrimSpace(q.Name), strings.TrimSpace(q.URL), strings.TrimSpace(q.Niche), boolInt(q.BotEnabled), q.WelcomeMessage, q.AutoReplyMode, q.ID, t)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			writeError(w, errors.New("id obligatorio"), 400)
			return
		}
		_, err = a.db.Exec(`DELETE FROM managed_groups WHERE id=? AND tenant_id=?`, id, t)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	default:
		http.Error(w, "Método no permitido", 405)
	}
}

func (a *App) groupSendHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	t, _, err := a.tenantFor(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	var q struct {
		GroupID int64  `json:"group_id"`
		Text    string `json:"text"`
	}
	if json.NewDecoder(r.Body).Decode(&q) != nil || q.GroupID == 0 || strings.TrimSpace(q.Text) == "" {
		writeError(w, errors.New("grupo y mensaje obligatorios"), 400)
		return
	}
	var platform, externalID string
	if a.db.QueryRow(`SELECT platform,external_id FROM managed_groups WHERE id=? AND tenant_id=?`, q.GroupID, t).Scan(&platform, &externalID) != nil {
		writeError(w, errors.New("grupo no encontrado"), 404)
		return
	}
	if platform == "facebook" {
		writeError(w, errors.New("Facebook no permite publicar en cualquier grupo desde una app. Solo se habilita para grupos y permisos oficialmente autorizados."), 409)
		return
	}
	target := externalID
	if platform == "telegram" && !strings.HasPrefix(target, "telegram:") {
		target = "telegram:" + target
	}
	id, err := a.sendText(r.Context(), target, q.Text, "manual")
	if err != nil {
		writeError(w, err, 502)
		return
	}
	_, _ = a.db.Exec(`INSERT INTO group_activity(tenant_id,group_id,platform,action,detail,created_at) VALUES(?,?,?,?,?,?)`, t, q.GroupID, platform, "send", q.Text, time.Now().UTC().Format(time.RFC3339))
	writeJSON(w, map[string]any{"ok": true, "message_id": id})
}

func (a *App) groupDiscoveryHandler(w http.ResponseWriter, r *http.Request) {
	t, _, err := a.tenantFor(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	lim := a.groupLimits(t)
	if r.Method == http.MethodGet {
		niche := strings.TrimSpace(r.URL.Query().Get("niche"))
		if niche == "" {
			writeError(w, errors.New("escribe el nicho que deseas buscar"), 400)
			return
		}
		q := url.QueryEscape(niche)
		results := []map[string]string{
			{"platform": "facebook", "label": "Buscar grupos públicos en Facebook", "url": "https://www.facebook.com/search/groups/?q=" + q},
			{"platform": "telegram", "label": "Buscar comunidades públicas de Telegram", "url": "https://www.google.com/search?q=site%3At.me+" + q + "+grupo+telegram"},
			{"platform": "whatsapp", "label": "Buscar directorios y enlaces públicos de WhatsApp", "url": "https://www.google.com/search?q=site%3Achat.whatsapp.com+" + q},
		}
		writeJSON(w, map[string]any{"niche": niche, "discovery_enabled": lim.Discovery, "results": results, "notice": "Worktic no extrae miembros ni evade la privacidad. La búsqueda abre resultados públicos; el usuario debe revisar las reglas y solicitar acceso legítimamente."})
		return
	}
	if r.Method == http.MethodPost {
		if !lim.Discovery {
			writeError(w, errors.New("la búsqueda y guardado de prospectos de grupos no está disponible en el plan Free"), 403)
			return
		}
		var q struct{ Platform, Name, URL, Niche, Notes string }
		if json.NewDecoder(r.Body).Decode(&q) != nil || strings.TrimSpace(q.Name) == "" {
			writeError(w, errors.New("nombre obligatorio"), 400)
			return
		}
		var n int
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM group_prospects WHERE tenant_id=?`, t).Scan(&n)
		if n >= lim.MaxProspects {
			writeError(w, fmt.Errorf("tu plan permite guardar hasta %d grupos potenciales", lim.MaxProspects), 403)
			return
		}
		_, err = a.db.Exec(`INSERT INTO group_prospects(tenant_id,platform,name,url,niche,notes,status,created_at) VALUES(?,?,?,?,?,?,?,?)`, t, q.Platform, q.Name, q.URL, q.Niche, q.Notes, "saved", time.Now().UTC().Format(time.RFC3339))
		if err != nil {
			writeError(w, err, 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
		return
	}
	http.Error(w, "Método no permitido", 405)
}

func (a *App) groupProspectsHandler(w http.ResponseWriter, r *http.Request) {
	t, _, err := a.tenantFor(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	if r.Method == http.MethodDelete {
		_, err = a.db.Exec(`DELETE FROM group_prospects WHERE id=? AND tenant_id=?`, r.URL.Query().Get("id"), t)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
		return
	}
	rows, err := a.db.Query(`SELECT id,platform,name,url,niche,notes,status,created_at FROM group_prospects WHERE tenant_id=? ORDER BY id DESC`, t)
	if err != nil {
		writeError(w, err, 500)
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id int64
		var p, n, u, ni, no, s, c string
		_ = rows.Scan(&id, &p, &n, &u, &ni, &no, &s, &c)
		out = append(out, map[string]any{"id": id, "platform": p, "name": n, "url": u, "niche": ni, "notes": no, "status": s, "created_at": c})
	}
	writeJSON(w, out)
}
