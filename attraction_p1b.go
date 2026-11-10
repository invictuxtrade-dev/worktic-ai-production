package main

// Worktic AI — Atracción de Clientes P1B.
// Descubrimiento autónomo respaldado por fuentes web de OpenAI. Sin scraping de
// grupos privados, extracción de datos personales ni envíos de prospección.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type attractionCampaignP1B struct {
	ID            int64  `json:"id"`
	Mode          string `json:"mode"`
	Sector        string `json:"sector"`
	City          string `json:"city"`
	Country       string `json:"country"`
	IntervalHours int    `json:"interval_hours"`
	Enabled       bool   `json:"enabled"`
	NextRun       string `json:"next_run"`
	LastRun       string `json:"last_run"`
	LastStatus    string `json:"last_status"`
	LastError     string `json:"last_error"`
	LastFound     int    `json:"last_found"`
}

type attractionWebCandidateP1B struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Website   string `json:"website"`
	SourceURL string `json:"source_url"`
	Signal    string `json:"signal"`
}

type attractionWebResultP1B struct {
	Candidates []attractionWebCandidateP1B `json:"candidates"`
}

type attractionSearchResponseP1B struct {
	Status string `json:"status"`
	Output []struct {
		Type   string `json:"type"`
		Action struct {
			Sources []struct {
				URL string `json:"url"`
			} `json:"sources"`
		} `json:"action"`
		Content []struct {
			Type        string `json:"type"`
			Text        string `json:"text"`
			Annotations []struct {
				Type string `json:"type"`
				URL  string `json:"url"`
			} `json:"annotations"`
		} `json:"content"`
	} `json:"output"`
}

func attractionP1BAIEnabled() bool {
	return attractionP1Enabled() && strings.EqualFold(strings.TrimSpace(os.Getenv("WORKTIC_ATTRACTION_P1B_AI_ENABLED")), "true")
}

func attractionP1BModel() string {
	model := strings.TrimSpace(os.Getenv("WORKTIC_ATTRACTION_P1B_MODEL"))
	if model == "" {
		model = "gpt-6-luna"
	}
	// Restrict to known web-search capable models; no arbitrary endpoint names.
	switch model {
	case "gpt-6-luna", "gpt-5.4-mini", "gpt-6.1-sol", "gpt-6-astra":
		return model
	}
	return "gpt-6-luna"
}

func attractionP1BPerTenantDailyLimit() int {
	n, err := strconv.Atoi(os.Getenv("WORKTIC_ATTRACTION_P1B_DAILY_RUN_LIMIT"))
	if err != nil || n < 1 {
		return 2
	}
	if n > 10 {
		return 10
	}
	return n
}

func attractionP1BSchema(db *DB) error {
	ddl := `CREATE TABLE IF NOT EXISTS attraction_campaigns_p1b (
      id INTEGER PRIMARY KEY AUTOINCREMENT,
      tenant_id BIGINT NOT NULL,
      mode TEXT NOT NULL,
      sector TEXT NOT NULL,
      city TEXT NOT NULL,
      country TEXT NOT NULL,
      interval_hours INTEGER NOT NULL DEFAULT 24,
      enabled INTEGER NOT NULL DEFAULT 1,
      next_run_at TEXT NOT NULL,
      lease_until TEXT NOT NULL DEFAULT '',
      last_run_at TEXT NOT NULL DEFAULT '',
      last_status TEXT NOT NULL DEFAULT 'pending',
      last_error TEXT NOT NULL DEFAULT '',
      last_found INTEGER NOT NULL DEFAULT 0,
      created_at TEXT NOT NULL,
      updated_at TEXT NOT NULL,
      UNIQUE(tenant_id,mode,sector,city,country)
    )`
	if db.IsPostgres() {
		ddl = strings.Replace(ddl, "id INTEGER PRIMARY KEY AUTOINCREMENT", "id BIGSERIAL PRIMARY KEY", 1)
	}
	if _, err := db.Exec(ddl); err != nil {
		return err
	}
	ddl = `CREATE TABLE IF NOT EXISTS attraction_ai_runs_p1b (
      id INTEGER PRIMARY KEY AUTOINCREMENT,
      tenant_id BIGINT NOT NULL,
      campaign_id BIGINT NOT NULL,
      started_at TEXT NOT NULL,
      finished_at TEXT NOT NULL DEFAULT '',
      status TEXT NOT NULL DEFAULT 'running',
      found INTEGER NOT NULL DEFAULT 0,
      error_kind TEXT NOT NULL DEFAULT ''
    )`
	if db.IsPostgres() {
		ddl = strings.Replace(ddl, "id INTEGER PRIMARY KEY AUTOINCREMENT", "id BIGSERIAL PRIMARY KEY", 1)
	}
	if _, err := db.Exec(ddl); err != nil {
		return err
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS attraction_ai_budget_p1b (
        tenant_id BIGINT NOT NULL, day TEXT NOT NULL, used INTEGER NOT NULL DEFAULT 0,
        PRIMARY KEY (tenant_id, day)
    )`); err != nil {
		return err
	}
	_, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_attr_p1b_due ON attraction_campaigns_p1b(enabled,next_run_at)`)
	return err
}

func (a *App) attractionP1BConfigHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", 405)
		return
	}
	if _, _, err := a.tenantFor(r); err != nil {
		writeError(w, err, 401)
		return
	}
	writeJSON(w, map[string]any{
		"ok": true, "enabled": attractionP1BAIEnabled(),
		"key_configured":    strings.TrimSpace(a.openAIKey()) != "",
		"worker_configured": attractionP1BAIEnabled(),
		"model":             attractionP1BModel(), "daily_run_limit": attractionP1BPerTenantDailyLimit(),
	})
}

func (a *App) attractionP1BCampaignsHandler(w http.ResponseWriter, r *http.Request) {
	tenant, _, err := a.tenantFor(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	if !attractionP1BAIEnabled() {
		writeError(w, errors.New("Autopilot IA está desactivado"), 503)
		return
	}
	if err = attractionP1BSchema(a.db); err != nil {
		writeError(w, err, 500)
		return
	}
	switch r.Method {
	case http.MethodGet:
		rows, err := a.db.Query(`SELECT id,mode,sector,city,country,interval_hours,enabled,next_run_at,last_run_at,last_status,last_error,last_found FROM attraction_campaigns_p1b WHERE tenant_id=? ORDER BY id DESC LIMIT 20`, tenant)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		defer rows.Close()
		out := []attractionCampaignP1B{}
		for rows.Next() {
			var c attractionCampaignP1B
			var active int
			if err = rows.Scan(&c.ID, &c.Mode, &c.Sector, &c.City, &c.Country, &c.IntervalHours, &active, &c.NextRun, &c.LastRun, &c.LastStatus, &c.LastError, &c.LastFound); err != nil {
				writeError(w, err, 500)
				return
			}
			c.Enabled = active == 1
			out = append(out, c)
		}
		if err = rows.Err(); err != nil {
			writeError(w, err, 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "items": out})
	case http.MethodPost:
		r.Body = http.MaxBytesReader(w, r.Body, 8192)
		var c attractionCampaignP1B
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if dec.Decode(&c) != nil {
			writeError(w, errors.New("Datos de campaña inválidos"), 400)
			return
		}
		c.Mode = attractionP1BCleanMode(c.Mode)
		c.Sector = attractionP1Clean(c.Sector, 110)
		c.City = attractionP1Clean(c.City, 90)
		c.Country = attractionP1Clean(c.Country, 90)
		if c.Sector == "" || c.City == "" || c.Country == "" || c.Mode == "" {
			writeError(w, errors.New("Indica modalidad, sector, ciudad y país"), 400)
			return
		}
		if len([]rune(c.Sector)) < 3 {
			writeError(w, errors.New("El sector es demasiado corto"), 400)
			return
		}
		if c.IntervalHours != 24 && c.IntervalHours != 48 && c.IntervalHours != 72 {
			c.IntervalHours = 24
		}
		var count int
		if err = a.db.QueryRow(`SELECT COUNT(*) FROM attraction_campaigns_p1b WHERE tenant_id=?`, tenant).Scan(&count); err != nil {
			writeError(w, err, 500)
			return
		}
		if count >= 3 {
			writeError(w, errors.New("Límite de 3 campañas por empresa en esta versión"), 429)
			return
		}
		now := time.Now().UTC().Format(time.RFC3339)
		_, err = a.db.Exec(`INSERT INTO attraction_campaigns_p1b (tenant_id,mode,sector,city,country,interval_hours,enabled,next_run_at,created_at,updated_at) VALUES(?,?,?,?,?,?,1,?,?,?) ON CONFLICT(tenant_id,mode,sector,city,country) DO NOTHING`, tenant, c.Mode, c.Sector, c.City, c.Country, c.IntervalHours, now, now, now)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "message": "Campaña creada; se ejecutará automáticamente"})
	default:
		http.Error(w, "Método no permitido", 405)
	}
}

func attractionP1BCleanMode(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "business", "community", "both":
		return strings.ToLower(strings.TrimSpace(s))
	}
	return ""
}

func (a *App) attractionP1BControlHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	tenant, _, err := a.tenantFor(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	if !attractionP1BAIEnabled() {
		writeError(w, errors.New("Autopilot IA desactivado"), 503)
		return
	}
	if err = attractionP1BSchema(a.db); err != nil {
		writeError(w, err, 500)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 2048)
	var req struct {
		ID     int64  `json:"id"`
		Action string `json:"action"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if dec.Decode(&req) != nil || req.ID <= 0 {
		writeError(w, errors.New("Solicitud inválida"), 400)
		return
	}
	active := 0
	switch req.Action {
	case "pause":
		active = 0
	case "resume":
		active = 1
	default:
		writeError(w, errors.New("Acción no permitida"), 400)
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := a.db.Exec(`UPDATE attraction_campaigns_p1b SET enabled=?, next_run_at=CASE WHEN ?=1 THEN ? ELSE next_run_at END,updated_at=? WHERE id=? AND tenant_id=?`, active, active, now, now, req.ID, tenant)
	if err != nil {
		writeError(w, err, 500)
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeError(w, errors.New("Campaña no encontrada"), 404)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

// Solo los resultados cuya URL sea una fuente devuelta por web_search se admiten.
// Una respuesta inventada, sin web_search, genera cero hallazgos.
func attractionP1BVerifiedCandidates(response attractionSearchResponseP1B) ([]attractionWebCandidateP1B, error) {
	sources := map[string]bool{}
	searched := false
	textOut := ""
	for _, o := range response.Output {
		if o.Type == "web_search_call" {
			searched = true
			for _, s := range o.Action.Sources {
				if k := attractionP1BCanonicalURL(s.URL); k != "" {
					sources[k] = true
				}
			}
		}
		if o.Type == "message" {
			for _, c := range o.Content {
				if c.Type == "output_text" {
					textOut += c.Text
				}
				for _, an := range c.Annotations {
					if an.Type == "url_citation" {
						if k := attractionP1BCanonicalURL(an.URL); k != "" {
							sources[k] = true
						}
					}
				}
			}
		}
	}
	if response.Status != "completed" || !searched || len(sources) == 0 {
		return nil, errors.New("OpenAI no devolvió una búsqueda verificable")
	}
	var parsed attractionWebResultP1B
	if err := json.Unmarshal([]byte(textOut), &parsed); err != nil {
		return nil, errors.New("La respuesta de IA no tiene el formato esperado")
	}
	out := []attractionWebCandidateP1B{}
	for _, x := range parsed.Candidates {
		if len(out) >= 8 {
			break
		}
		k := attractionP1BCanonicalURL(x.SourceURL)
		if k == "" || !sources[k] {
			continue
		}
		if x.Kind != "business" && x.Kind != "group_signal" {
			continue
		}
		x.Name = attractionP1Clean(x.Name, 180)
		x.Signal = attractionP1Clean(x.Signal, 700)
		if x.Name == "" {
			continue
		}
		if x.Website != "" {
			u, err := attractionP1SourceURL(x.Website)
			if err != nil {
				x.Website = ""
			} else {
				websiteKey := attractionP1BCanonicalURL(u)
				websiteHost := ""
				sourceHost := ""
				if parsed, err := url.Parse(websiteKey); err == nil {
					websiteHost = strings.TrimPrefix(parsed.Hostname(), "www.")
				}
				if parsed, err := url.Parse(k); err == nil {
					sourceHost = strings.TrimPrefix(parsed.Hostname(), "www.")
				}
				if websiteHost == "" || (!sources[websiteKey] && websiteHost != sourceHost) {
					x.Website = ""
				} else {
					x.Website = u
				}
			}
		}
		x.SourceURL = k
		out = append(out, x)
	}
	return out, nil
}

func attractionP1BCanonicalURL(s string) string {
	u, err := url.Parse(strings.TrimSpace(s))
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" || u.User != nil {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".local") {
		return ""
	}
	if u.Port() != "" {
		return ""
	}
	u.Scheme = "https"
	u.Host = host
	u.Fragment = ""
	// Avoid tracking parameters and redirect variations; cited URL remains directly clickable.
	u.RawQuery = ""
	u.Path = strings.TrimRight(u.Path, "/")
	if u.Path == "" {
		u.Path = "/"
	}
	return u.String()
}

func attractionP1BDedupeKey(x attractionWebCandidateP1B, city, country string) string {
	site := attractionP1BCanonicalURL(x.Website)
	key := strings.ToLower(strings.TrimSpace(x.Name)) + "|" + strings.ToLower(strings.TrimSpace(city)) + "|" + strings.ToLower(strings.TrimSpace(country))
	if site != "" {
		if u, err := url.Parse(site); err == nil {
			key += "|" + u.Hostname()
		}
	}
	sum := sha256.Sum256([]byte("p1b|" + key))
	return hex.EncodeToString(sum[:])
}

func attractionP1BWebPrompt(c attractionCampaignP1B) string {
	kind := "B2B companies with official websites"
	if c.Mode == "community" {
		kind = "publicly indexed, legitimate business needs expressed by named companies in public communities or business forums"
	}
	if c.Mode == "both" {
		kind = "B2B companies with official websites AND publicly indexed business opportunities in lawful public forums"
	}
	return fmt.Sprintf(`Search the live web for up to 8 REAL %s in %s, %s. Sector: %s.
Return only established, named organizations (not individuals); exclude private groups, member lists, personal profiles, guessed email addresses or phones.
For each candidate, source_url MUST be an exact URL from the web search tool's returned sources. No invented web addresses, claims, contacts, or purchase intent.
"signal" must precisely distinguish a documented, specific business need from a general possible fit; if no buying signal exists write "Posible encaje sectorial; intención de compra NO verificada".
"kind" is business or group_signal; use group_signal ONLY for publicly visible requests explicitly from an identifiable organization.
"website" must be an actual public organization website found in results, otherwise an empty string.
Return Spanish text; no fabricated results; an empty candidates array is acceptable.`, kind, c.City, c.Country, c.Sector)
}

func attractionP1BSearchSchema() map[string]any {
	props := map[string]any{}
	for _, k := range []string{"name", "website", "source_url", "signal"} {
		props[k] = map[string]any{"type": "string"}
	}
	props["kind"] = map[string]any{"type": "string", "enum": []string{"business", "group_signal"}}
	item := map[string]any{"type": "object", "additionalProperties": false, "properties": props, "required": []string{"name", "kind", "website", "source_url", "signal"}}
	return map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{"candidates": map[string]any{"type": "array", "items": item}}, "required": []string{"candidates"}}
}

func (a *App) attractionP1BSearch(ctx context.Context, c attractionCampaignP1B) ([]attractionWebCandidateP1B, error) {
	key := strings.TrimSpace(a.openAIKey())
	if key == "" {
		return nil, errors.New("Falta OPENAI_API_KEY en el servidor")
	}
	requestBody := map[string]any{
		"model":             attractionP1BModel(),
		"input":             attractionP1BWebPrompt(c),
		"tools":             []map[string]any{{"type": "web_search", "search_context_size": "medium"}},
		"tool_choice":       "required",
		"max_tool_calls":    2,
		"reasoning":         map[string]any{"effort": "low"},
		"include":           []string{"web_search_call.action.sources"},
		"text":              map[string]any{"format": map[string]any{"type": "json_schema", "name": "attraction_verified_businesses", "strict": true, "schema": attractionP1BSearchSchema()}},
		"max_output_tokens": 2200,
		"store":             false,
	}
	raw, _ := json.Marshal(requestBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/responses", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("falló llamada a OpenAI: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 { // Avoid logging API response: it may include information not intended for logs.
		return nil, fmt.Errorf("OpenAI HTTP %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return nil, err
	}
	var parsed attractionSearchResponseP1B
	if err := json.Unmarshal(b, &parsed); err != nil {
		return nil, errors.New("Respuesta OpenAI inválida")
	}
	return attractionP1BVerifiedCandidates(parsed)
}

func (a *App) runAttractionP1BWorker() {
	if !attractionP1BAIEnabled() {
		return
	}
	if err := attractionP1BSchema(a.db); err != nil {
		log.Printf("[ATTRACTION-P1B] schema error: %v", err)
		return
	}
	log.Printf("[ATTRACTION-P1B] scheduler activo; solo investigación y CRM, sin envíos")
	a.attractionP1BTick()
	ticker := time.NewTicker(90 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		if !attractionP1BAIEnabled() {
			return
		}
		a.attractionP1BTick()
	}
}

func (a *App) attractionP1BTick() {
	now := time.Now().UTC().Format(time.RFC3339)
	// After a crash, a lease older than its expiry must be eligible again.
	_, _ = a.db.Exec(`UPDATE attraction_campaigns_p1b SET lease_until='',next_run_at=?,last_status='retry' WHERE last_status='running' AND lease_until<>'' AND lease_until<?`, now, now)
	rows, err := a.db.Query(`SELECT id,tenant_id,mode,sector,city,country,interval_hours FROM attraction_campaigns_p1b WHERE enabled=1 AND next_run_at<=? AND (lease_until='' OR lease_until<?) ORDER BY next_run_at,id LIMIT 8`, now, now)
	if err != nil {
		log.Printf("[ATTRACTION-P1B] no se pudo consultar cola: %v", err)
		return
	}
	type workItem struct {
		c      attractionCampaignP1B
		tenant int64
	}
	queue := []workItem{}
	for rows.Next() {
		var it workItem
		if err := rows.Scan(&it.c.ID, &it.tenant, &it.c.Mode, &it.c.Sector, &it.c.City, &it.c.Country, &it.c.IntervalHours); err == nil {
			queue = append(queue, it)
		}
	}
	rows.Close()
	for _, it := range queue {
		a.attractionP1BClaimAndRun(it.tenant, it.c)
	}
}

func (a *App) attractionP1BClaimAndRun(tenant int64, c attractionCampaignP1B) {
	now := time.Now().UTC()
	leaseUntil := now.Add(6 * time.Minute).Format(time.RFC3339)
	next := now.Add(time.Duration(c.IntervalHours) * time.Hour).Format(time.RFC3339)
	startedAt := now.Format(time.RFC3339Nano)
	today := now.Format("2006-01-02")
	res, err := a.db.Exec(`UPDATE attraction_campaigns_p1b SET lease_until=?,next_run_at=?,last_status='running',last_error='',updated_at=? WHERE id=? AND tenant_id=? AND enabled=1 AND next_run_at<=? AND (lease_until='' OR lease_until<?)`, leaseUntil, next, now.Format(time.RFC3339), c.ID, tenant, now.Format(time.RFC3339), now.Format(time.RFC3339))
	if err != nil {
		return
	}
	affected, err := res.RowsAffected()
	if err != nil || affected != 1 {
		return
	}
	_, err = a.db.Exec(`INSERT INTO attraction_ai_budget_p1b(tenant_id,day,used) VALUES(?,?,0) ON CONFLICT(tenant_id,day) DO NOTHING`, tenant, today)
	if err != nil {
		return
	}
	budget, err := a.db.Exec(`UPDATE attraction_ai_budget_p1b SET used=used+1 WHERE tenant_id=? AND day=? AND used<?`, tenant, today, attractionP1BPerTenantDailyLimit())
	if err != nil {
		return
	}
	granted, _ := budget.RowsAffected()
	if granted != 1 {
		midnight := now.Truncate(24 * time.Hour).Add(24 * time.Hour).Format(time.RFC3339)
		_, _ = a.db.Exec(`UPDATE attraction_campaigns_p1b SET lease_until='',next_run_at=?,last_status='daily_limit',last_error='Límite diario alcanzado',updated_at=? WHERE id=? AND tenant_id=?`, midnight, now.Format(time.RFC3339), c.ID, tenant)
		return
	}
	if _, err = a.db.Exec(`INSERT INTO attraction_ai_runs_p1b(tenant_id,campaign_id,started_at) VALUES(?,?,?)`, tenant, c.ID, startedAt); err != nil {
		_, _ = a.db.Exec(`UPDATE attraction_campaigns_p1b SET last_status='error',last_error='No se pudo crear auditoría',lease_until='' WHERE id=? AND tenant_id=?`, c.ID, tenant)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	items, searchErr := a.attractionP1BSearch(ctx, c)
	cancel()
	added := 0
	if searchErr == nil {
		for _, item := range items {
			if c.Mode == "business" && item.Kind != "business" {
				continue
			}
			if c.Mode == "community" && item.Kind != "group_signal" {
				continue
			}
			x := AttractionCandidateP1{Name: item.Name, Kind: item.Kind, Website: item.Website, SourceURL: item.SourceURL, Signal: item.Signal, City: c.City, Country: c.Country, Niche: c.Sector}
			x.Score = attractionP1Score(x)
			if x.Kind == "group_signal" && strings.TrimSpace(x.Signal) == "" {
				continue
			}
			nowText := time.Now().UTC().Format(time.RFC3339)
			key := attractionP1BDedupeKey(item, c.City, c.Country)
			res, e := a.db.Exec(`INSERT INTO attraction_candidates_p1 (tenant_id,source_key,name,kind,city,country,niche,website,email,phone,source_url,signal,score,status,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,'review',?,?) ON CONFLICT(tenant_id,source_key) DO NOTHING`, tenant, key, x.Name, x.Kind, x.City, x.Country, x.Niche, x.Website, "", "", x.SourceURL, x.Signal, x.Score, nowText, nowText)
			if e != nil {
				log.Printf("[ATTRACTION-P1B] error guardando hallazgo: %v", e)
				continue
			}
			rowsAffected, _ := res.RowsAffected()
			if rowsAffected == 0 {
				continue
			}
			added++
			// CRM automático solo para empresas identificables; nunca para perfiles de grupos.
			if x.Kind == "business" && x.Website != "" {
				var id int64
				if e = a.db.QueryRow(`SELECT id FROM attraction_candidates_p1 WHERE tenant_id=? AND source_key=?`, tenant, key).Scan(&id); e == nil {
					if e = a.attractionP1BAutoCRM(tenant, id, x); e != nil {
						log.Printf("[ATTRACTION-P1B] hallazgo guardado; CRM pendiente: %v", e)
					}
				}
			}
		}
	}
	status := "success"
	kind := ""
	publicError := ""
	if searchErr != nil {
		status = "error"
		kind = "search_failed"
		publicError = "La búsqueda con IA falló. Revisa la configuración y el saldo de OpenAI."
		log.Printf("[ATTRACTION-P1B] búsqueda fallida tenant=%d campaign=%d: %v", tenant, c.ID, searchErr)
	}
	finishedAt := time.Now().UTC().Format(time.RFC3339)
	_, _ = a.db.Exec(`UPDATE attraction_ai_runs_p1b SET finished_at=?,status=?,found=?,error_kind=? WHERE tenant_id=? AND campaign_id=? AND started_at=?`, finishedAt, status, added, kind, tenant, c.ID, startedAt)
	_, _ = a.db.Exec(`UPDATE attraction_campaigns_p1b SET lease_until='',last_run_at=?,last_status=?,last_error=?,last_found=?,updated_at=? WHERE id=? AND tenant_id=?`, finishedAt, status, publicError, added, finishedAt, c.ID, tenant)
}

func (a *App) attractionP1BAutoCRM(tenant, id int64, x AttractionCandidateP1) error {
	// El CRM aloja una ficha de empresa sin datos personales ni envío automático.
	ext := fmt.Sprintf("attraction:p1:%d", id)
	if err := a.syncCRMContact(tenant, x.Name, "", "", "attraction_ai", "attraction", ext); err != nil {
		return err
	}
	contactID, err := a.contactIDForOpportunity(tenant, 0, "", "", ext)
	if err != nil || contactID <= 0 {
		return errors.New("Contacto CRM no encontrado")
	}
	opportunityID, _, err := a.syncAutomaticOpportunity(tenant, contactID, "attraction", ext, "attraction_ai", x.Niche, "Nuevo", x.Score, "")
	if err != nil || opportunityID <= 0 {
		return errors.New("No se pudo crear oportunidad")
	}
	_, err = a.db.Exec(`UPDATE attraction_candidates_p1 SET status='promoted',crm_contact_id=?,crm_opportunity_id=?,updated_at=? WHERE id=? AND tenant_id=? AND status='review'`, contactID, opportunityID, time.Now().UTC().Format(time.RFC3339), id, tenant)
	return err
}
