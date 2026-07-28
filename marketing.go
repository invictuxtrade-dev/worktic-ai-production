package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type MarketingCampaign struct {
	ID          int64   `json:"id"`
	TenantID    int64   `json:"tenant_id"`
	Name        string  `json:"name"`
	Mode        string  `json:"mode"`
	Objective   string  `json:"objective"`
	Product     string  `json:"product"`
	Platforms   string  `json:"platforms"`
	Destination string  `json:"destination"`
	Audience    string  `json:"audience"`
	BudgetDaily float64 `json:"budget_daily"`
	BudgetTotal float64 `json:"budget_total"`
	StartsAt    string  `json:"starts_at"`
	EndsAt      string  `json:"ends_at"`
	Status      string  `json:"status"`
	AIPlan      string  `json:"ai_plan"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}
type ContentItem struct {
	ID           int64  `json:"id"`
	TenantID     int64  `json:"tenant_id"`
	CampaignID   int64  `json:"campaign_id"`
	Platform     string `json:"platform"`
	Format       string `json:"format"`
	Title        string `json:"title"`
	Body         string `json:"body"`
	CTA          string `json:"cta"`
	MediaURL     string `json:"media_url"`
	ScheduledAt  string `json:"scheduled_at"`
	Status       string `json:"status"`
	PublishedURL string `json:"published_url"`
	CreatedAt    string `json:"created_at"`
}
type LeadForm struct {
	ID          int64  `json:"id"`
	TenantID    int64  `json:"tenant_id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Headline    string `json:"headline"`
	Description string `json:"description"`
	FieldsJSON  string `json:"fields_json"`
	ConsentText string `json:"consent_text"`
	ThankYou    string `json:"thank_you"`
	RedirectURL string `json:"redirect_url"`
	Active      bool   `json:"active"`
	CreatedAt   string `json:"created_at"`
}
type MarketingLead struct {
	ID          int64  `json:"id"`
	TenantID    int64  `json:"tenant_id"`
	CampaignID  int64  `json:"campaign_id"`
	FormID      int64  `json:"form_id"`
	Source      string `json:"source"`
	Name        string `json:"name"`
	Phone       string `json:"phone"`
	Email       string `json:"email"`
	City        string `json:"city"`
	Interest    string `json:"interest"`
	Score       int    `json:"score"`
	Status      string `json:"status"`
	UTMSource   string `json:"utm_source"`
	UTMCampaign string `json:"utm_campaign"`
	Consent     bool   `json:"consent"`
	CreatedAt   string `json:"created_at"`
}
type Creative struct {
	ID        int64  `json:"id"`
	TenantID  int64  `json:"tenant_id"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	URL       string `json:"url"`
	Notes     string `json:"notes"`
	CreatedAt string `json:"created_at"`
}

type MarketingLimits struct {
	Plan         string `json:"plan"`
	Organic      bool   `json:"organic"`
	Paid         bool   `json:"paid"`
	Combined     bool   `json:"combined"`
	MetaPublish  bool   `json:"meta_publish"`
	MaxCampaigns int    `json:"max_campaigns"`
	MaxScheduled int    `json:"max_scheduled"`
	MaxForms     int    `json:"max_forms"`
	MaxCreatives int    `json:"max_creatives"`
}

func initMarketingSchema(db *sql.DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS marketing_campaigns(id INTEGER PRIMARY KEY AUTOINCREMENT,tenant_id INTEGER NOT NULL,name TEXT NOT NULL,mode TEXT NOT NULL DEFAULT 'organic',objective TEXT NOT NULL DEFAULT '',product TEXT NOT NULL DEFAULT '',platforms TEXT NOT NULL DEFAULT 'facebook,instagram',destination TEXT NOT NULL DEFAULT 'whatsapp',audience TEXT NOT NULL DEFAULT '',budget_daily REAL NOT NULL DEFAULT 0,budget_total REAL NOT NULL DEFAULT 0,starts_at TEXT NOT NULL DEFAULT '',ends_at TEXT NOT NULL DEFAULT '',status TEXT NOT NULL DEFAULT 'draft',ai_plan TEXT NOT NULL DEFAULT '',created_at TEXT NOT NULL,updated_at TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS idx_marketing_campaigns_tenant ON marketing_campaigns(tenant_id);
CREATE TABLE IF NOT EXISTS marketing_content(id INTEGER PRIMARY KEY AUTOINCREMENT,tenant_id INTEGER NOT NULL,campaign_id INTEGER NOT NULL DEFAULT 0,platform TEXT NOT NULL DEFAULT 'facebook',format TEXT NOT NULL DEFAULT 'post',title TEXT NOT NULL DEFAULT '',body TEXT NOT NULL DEFAULT '',cta TEXT NOT NULL DEFAULT '',media_url TEXT NOT NULL DEFAULT '',scheduled_at TEXT NOT NULL DEFAULT '',status TEXT NOT NULL DEFAULT 'draft',published_url TEXT NOT NULL DEFAULT '',created_at TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS idx_marketing_content_tenant ON marketing_content(tenant_id);
CREATE TABLE IF NOT EXISTS marketing_forms(id INTEGER PRIMARY KEY AUTOINCREMENT,tenant_id INTEGER NOT NULL,name TEXT NOT NULL,slug TEXT NOT NULL,headline TEXT NOT NULL DEFAULT '',description TEXT NOT NULL DEFAULT '',fields_json TEXT NOT NULL DEFAULT '["name","phone","email"]',consent_text TEXT NOT NULL DEFAULT '',thank_you TEXT NOT NULL DEFAULT 'Gracias. Pronto te contactaremos.',redirect_url TEXT NOT NULL DEFAULT '',active INTEGER NOT NULL DEFAULT 1,created_at TEXT NOT NULL,UNIQUE(tenant_id,slug));
CREATE TABLE IF NOT EXISTS marketing_leads(id INTEGER PRIMARY KEY AUTOINCREMENT,tenant_id INTEGER NOT NULL,campaign_id INTEGER NOT NULL DEFAULT 0,form_id INTEGER NOT NULL DEFAULT 0,source TEXT NOT NULL DEFAULT 'worktic_form',name TEXT NOT NULL DEFAULT '',phone TEXT NOT NULL DEFAULT '',email TEXT NOT NULL DEFAULT '',city TEXT NOT NULL DEFAULT '',interest TEXT NOT NULL DEFAULT '',score INTEGER NOT NULL DEFAULT 50,status TEXT NOT NULL DEFAULT 'new',utm_source TEXT NOT NULL DEFAULT '',utm_campaign TEXT NOT NULL DEFAULT '',consent INTEGER NOT NULL DEFAULT 0,created_at TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS idx_marketing_leads_tenant ON marketing_leads(tenant_id);
CREATE TABLE IF NOT EXISTS marketing_creatives(id INTEGER PRIMARY KEY AUTOINCREMENT,tenant_id INTEGER NOT NULL,name TEXT NOT NULL,kind TEXT NOT NULL DEFAULT 'image',url TEXT NOT NULL DEFAULT '',notes TEXT NOT NULL DEFAULT '',created_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS marketing_meta_connections(id INTEGER PRIMARY KEY AUTOINCREMENT,tenant_id INTEGER NOT NULL UNIQUE,business_id TEXT NOT NULL DEFAULT '',ad_account_id TEXT NOT NULL DEFAULT '',page_id TEXT NOT NULL DEFAULT '',instagram_id TEXT NOT NULL DEFAULT '',access_token TEXT NOT NULL DEFAULT '',status TEXT NOT NULL DEFAULT 'disconnected',last_error TEXT NOT NULL DEFAULT '',updated_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS marketing_metrics(id INTEGER PRIMARY KEY AUTOINCREMENT,tenant_id INTEGER NOT NULL,campaign_id INTEGER NOT NULL,metric_date TEXT NOT NULL,spend REAL NOT NULL DEFAULT 0,impressions INTEGER NOT NULL DEFAULT 0,reach INTEGER NOT NULL DEFAULT 0,clicks INTEGER NOT NULL DEFAULT 0,leads INTEGER NOT NULL DEFAULT 0,conversations INTEGER NOT NULL DEFAULT 0,appointments INTEGER NOT NULL DEFAULT 0,sales INTEGER NOT NULL DEFAULT 0,revenue REAL NOT NULL DEFAULT 0,UNIQUE(tenant_id,campaign_id,metric_date));
`)
	return err
}

func (a *App) tenantFor(r *http.Request) (int64, *User, error) {
	u := a.currentUser(r)
	if u == nil {
		return 0, nil, fmt.Errorf("sesión requerida")
	}
	if u.TenantID == 0 {
		return 0, u, fmt.Errorf("cuenta sin empresa o espacio asignado")
	}
	// El tenant se resuelve desde la sesión autenticada. Todos los miembros de
	// una empresa comparten el mismo tenant y, por tanto, las mismas conexiones,
	// calendario, publicaciones y métricas del Social Hub.
	return u.TenantID, u, nil
}
func marketingLimitsFor(plan string) MarketingLimits {
	switch plan {
	case "personal":
		return MarketingLimits{plan, true, false, false, false, 10, 30, 5, 50}
	case "business":
		return MarketingLimits{plan, true, true, true, true, 50, 150, 20, 300}
	case "enterprise":
		return MarketingLimits{plan, true, true, true, true, 250, 1000, 100, 3000}
	default:
		return MarketingLimits{"free", true, false, false, false, 2, 5, 1, 10}
	}
}
func (a *App) marketingLimits(r *http.Request, tenant int64) MarketingLimits {
	p, _, e := a.activePlan(tenant)
	if e != nil {
		return marketingLimitsFor("free")
	}
	return marketingLimitsFor(p.Code)
}

func (a *App) marketingLimitsHandler(w http.ResponseWriter, r *http.Request) {
	t, _, e := a.tenantFor(r)
	if e != nil {
		http.Error(w, e.Error(), 401)
		return
	}
	writeJSON(w, a.marketingLimits(r, t))
}
func (a *App) marketingCampaignsHandler(w http.ResponseWriter, r *http.Request) {
	t, _, e := a.tenantFor(r)
	if e != nil {
		http.Error(w, e.Error(), 401)
		return
	}
	lim := a.marketingLimits(r, t)
	switch r.Method {
	case "GET":
		rows, e := a.db.Query(`SELECT id,tenant_id,name,mode,objective,product,platforms,destination,audience,budget_daily,budget_total,starts_at,ends_at,status,ai_plan,created_at,updated_at FROM marketing_campaigns WHERE tenant_id=? ORDER BY id DESC`, t)
		if e != nil {
			http.Error(w, e.Error(), 500)
			return
		}
		defer rows.Close()
		out := []MarketingCampaign{}
		for rows.Next() {
			var x MarketingCampaign
			_ = rows.Scan(&x.ID, &x.TenantID, &x.Name, &x.Mode, &x.Objective, &x.Product, &x.Platforms, &x.Destination, &x.Audience, &x.BudgetDaily, &x.BudgetTotal, &x.StartsAt, &x.EndsAt, &x.Status, &x.AIPlan, &x.CreatedAt, &x.UpdatedAt)
			out = append(out, x)
		}
		writeJSON(w, out)
	case "POST":
		var x MarketingCampaign
		if json.NewDecoder(r.Body).Decode(&x) != nil || strings.TrimSpace(x.Name) == "" {
			http.Error(w, "nombre obligatorio", 400)
			return
		}
		if x.Mode == "paid" && !lim.Paid || x.Mode == "combined" && !lim.Combined {
			http.Error(w, "tu plan no incluye este tipo de campaña", 403)
			return
		}
		var n int
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM marketing_campaigns WHERE tenant_id=?`, t).Scan(&n)
		if n >= lim.MaxCampaigns {
			http.Error(w, "límite de campañas alcanzado", 403)
			return
		}
		now := time.Now().UTC().Format(time.RFC3339)
		res, e := a.db.Exec(`INSERT INTO marketing_campaigns(tenant_id,name,mode,objective,product,platforms,destination,audience,budget_daily,budget_total,starts_at,ends_at,status,ai_plan,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, t, x.Name, x.Mode, x.Objective, x.Product, x.Platforms, x.Destination, x.Audience, x.BudgetDaily, x.BudgetTotal, x.StartsAt, x.EndsAt, "draft", x.AIPlan, now, now)
		if e != nil {
			http.Error(w, e.Error(), 500)
			return
		}
		id, _ := res.LastInsertId()
		writeJSON(w, map[string]any{"id": id, "status": "draft"})
	case "PUT":
		var x MarketingCampaign
		if json.NewDecoder(r.Body).Decode(&x) != nil || x.ID == 0 {
			http.Error(w, "datos inválidos", 400)
			return
		}
		_, e := a.db.Exec(`UPDATE marketing_campaigns SET name=?,mode=?,objective=?,product=?,platforms=?,destination=?,audience=?,budget_daily=?,budget_total=?,starts_at=?,ends_at=?,status=?,updated_at=? WHERE id=? AND tenant_id=?`, x.Name, x.Mode, x.Objective, x.Product, x.Platforms, x.Destination, x.Audience, x.BudgetDaily, x.BudgetTotal, x.StartsAt, x.EndsAt, x.Status, time.Now().UTC().Format(time.RFC3339), x.ID, t)
		if e != nil {
			http.Error(w, e.Error(), 500)
			return
		}
		writeJSON(w, map[string]bool{"ok": true})
	case "DELETE":
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		_, _ = a.db.Exec(`DELETE FROM marketing_campaigns WHERE id=? AND tenant_id=?`, id, t)
		writeJSON(w, map[string]bool{"ok": true})
	default:
		w.WriteHeader(405)
	}
}

func (a *App) marketingGenerateHandler(w http.ResponseWriter, r *http.Request) {
	t, _, e := a.tenantFor(r)
	if e != nil {
		http.Error(w, e.Error(), 401)
		return
	}
	lim := a.marketingLimits(r, t)
	if r.Method != "POST" {
		w.WriteHeader(405)
		return
	}
	var q struct {
		Mode, Objective, Product, Audience, Platforms, Destination, Tone string
		Days                                                             int
	}
	if json.NewDecoder(r.Body).Decode(&q) != nil {
		http.Error(w, "datos inválidos", 400)
		return
	}
	if q.Mode == "paid" && !lim.Paid || q.Mode == "combined" && !lim.Combined {
		http.Error(w, "tu plan no incluye esta modalidad", 403)
		return
	}
	prompt := fmt.Sprintf("Crea una estrategia de campaña %s para %s. Producto: %s. Público: %s. Plataformas: %s. Destino: %s. Tono: %s. Duración: %d días. Devuelve en español: concepto, oferta, 3 copies, 3 títulos, CTA, calendario de 7 contenidos, preguntas de formulario y flujo del bot. No prometas resultados.", q.Mode, q.Objective, q.Product, q.Audience, q.Platforms, q.Destination, q.Tone, q.Days)
	reply, e := a.aiResponse(prompt, "")
	if e != nil {
		reply = "ESTRATEGIA PROPUESTA\n\nObjetivo: " + q.Objective + "\nOferta: presenta " + q.Product + " con un beneficio claro y una llamada a la acción hacia " + q.Destination + ".\n\nContenido sugerido:\n1. Publicación educativa.\n2. Caso de uso o demostración.\n3. Preguntas frecuentes.\n4. Oferta con límite real.\n5. Invitación a conversar.\n\nRevisa y aprueba todos los textos antes de publicar."
	}
	writeJSON(w, map[string]any{"plan": reply, "generated": true})
}

func (a *App) marketingContentHandler(w http.ResponseWriter, r *http.Request) {
	t, _, e := a.tenantFor(r)
	if e != nil {
		http.Error(w, e.Error(), 401)
		return
	}
	lim := a.marketingLimits(r, t)
	switch r.Method {
	case "GET":
		rows, e := a.db.Query(`SELECT id,tenant_id,campaign_id,platform,format,title,body,cta,media_url,scheduled_at,status,published_url,created_at FROM marketing_content WHERE tenant_id=? ORDER BY COALESCE(NULLIF(scheduled_at,''),created_at)`, t)
		if e != nil {
			http.Error(w, e.Error(), 500)
			return
		}
		defer rows.Close()
		out := []ContentItem{}
		for rows.Next() {
			var x ContentItem
			_ = rows.Scan(&x.ID, &x.TenantID, &x.CampaignID, &x.Platform, &x.Format, &x.Title, &x.Body, &x.CTA, &x.MediaURL, &x.ScheduledAt, &x.Status, &x.PublishedURL, &x.CreatedAt)
			out = append(out, x)
		}
		writeJSON(w, out)
	case "POST":
		var x ContentItem
		if json.NewDecoder(r.Body).Decode(&x) != nil {
			http.Error(w, "datos inválidos", 400)
			return
		}
		var n int
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM marketing_content WHERE tenant_id=? AND status IN ('draft','scheduled')`, t).Scan(&n)
		if n >= lim.MaxScheduled {
			http.Error(w, "límite de contenido alcanzado", 403)
			return
		}
		now := time.Now().UTC().Format(time.RFC3339)
		res, e := a.db.Exec(`INSERT INTO marketing_content(tenant_id,campaign_id,platform,format,title,body,cta,media_url,scheduled_at,status,published_url,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, t, x.CampaignID, x.Platform, x.Format, x.Title, x.Body, x.CTA, x.MediaURL, x.ScheduledAt, x.Status, "", now)
		if e != nil {
			http.Error(w, e.Error(), 500)
			return
		}
		id, _ := res.LastInsertId()
		writeJSON(w, map[string]any{"id": id})
	case "DELETE":
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		_, _ = a.db.Exec(`DELETE FROM marketing_content WHERE id=? AND tenant_id=?`, id, t)
		writeJSON(w, map[string]bool{"ok": true})
	default:
		w.WriteHeader(405)
	}
}

func (a *App) marketingFormsHandler(w http.ResponseWriter, r *http.Request) {
	t, _, e := a.tenantFor(r)
	if e != nil {
		http.Error(w, e.Error(), 401)
		return
	}
	lim := a.marketingLimits(r, t)
	switch r.Method {
	case "GET":
		rows, e := a.db.Query(`SELECT id,tenant_id,name,slug,headline,description,fields_json,consent_text,thank_you,redirect_url,active,created_at FROM marketing_forms WHERE tenant_id=? ORDER BY id DESC`, t)
		if e != nil {
			http.Error(w, e.Error(), 500)
			return
		}
		defer rows.Close()
		out := []LeadForm{}
		for rows.Next() {
			var x LeadForm
			var active int
			_ = rows.Scan(&x.ID, &x.TenantID, &x.Name, &x.Slug, &x.Headline, &x.Description, &x.FieldsJSON, &x.ConsentText, &x.ThankYou, &x.RedirectURL, &active, &x.CreatedAt)
			x.Active = active == 1
			out = append(out, x)
		}
		writeJSON(w, out)
	case "POST":
		var x LeadForm
		if json.NewDecoder(r.Body).Decode(&x) != nil || x.Name == "" {
			http.Error(w, "nombre obligatorio", 400)
			return
		}
		var n int
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM marketing_forms WHERE tenant_id=?`, t).Scan(&n)
		if n >= lim.MaxForms {
			http.Error(w, "límite de formularios alcanzado", 403)
			return
		}
		x.Slug = slugify(x.Slug)
		if x.Slug == "" {
			x.Slug = slugify(x.Name)
		}
		if x.FieldsJSON == "" {
			x.FieldsJSON = `["name","phone","email","city","interest"]`
		}
		res, e := a.db.Exec(`INSERT INTO marketing_forms(tenant_id,name,slug,headline,description,fields_json,consent_text,thank_you,redirect_url,active,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, t, x.Name, x.Slug, x.Headline, x.Description, x.FieldsJSON, x.ConsentText, x.ThankYou, x.RedirectURL, 1, time.Now().UTC().Format(time.RFC3339))
		if e != nil {
			http.Error(w, e.Error(), 400)
			return
		}
		id, _ := res.LastInsertId()
		writeJSON(w, map[string]any{"id": id, "public_url": fmt.Sprintf("/f/%d/%s", t, x.Slug)})
	default:
		w.WriteHeader(405)
	}
}

func (a *App) marketingLeadsHandler(w http.ResponseWriter, r *http.Request) {
	t, _, e := a.tenantFor(r)
	if e != nil {
		http.Error(w, e.Error(), 401)
		return
	}
	rows, e := a.db.Query(`SELECT id,tenant_id,campaign_id,form_id,source,name,phone,email,city,interest,score,status,utm_source,utm_campaign,consent,created_at FROM marketing_leads WHERE tenant_id=? ORDER BY id DESC LIMIT 500`, t)
	if e != nil {
		http.Error(w, e.Error(), 500)
		return
	}
	defer rows.Close()
	out := []MarketingLead{}
	for rows.Next() {
		var x MarketingLead
		var consent int
		_ = rows.Scan(&x.ID, &x.TenantID, &x.CampaignID, &x.FormID, &x.Source, &x.Name, &x.Phone, &x.Email, &x.City, &x.Interest, &x.Score, &x.Status, &x.UTMSource, &x.UTMCampaign, &consent, &x.CreatedAt)
		x.Consent = consent == 1
		out = append(out, x)
	}
	writeJSON(w, out)
}
func (a *App) marketingCreativesHandler(w http.ResponseWriter, r *http.Request) {
	t, _, e := a.tenantFor(r)
	if e != nil {
		http.Error(w, e.Error(), 401)
		return
	}
	lim := a.marketingLimits(r, t)
	switch r.Method {
	case "GET":
		rows, _ := a.db.Query(`SELECT id,tenant_id,name,kind,url,notes,created_at FROM marketing_creatives WHERE tenant_id=? ORDER BY id DESC`, t)
		defer rows.Close()
		out := []Creative{}
		for rows.Next() {
			var x Creative
			_ = rows.Scan(&x.ID, &x.TenantID, &x.Name, &x.Kind, &x.URL, &x.Notes, &x.CreatedAt)
			out = append(out, x)
		}
		writeJSON(w, out)
	case "POST":
		var x Creative
		_ = json.NewDecoder(r.Body).Decode(&x)
		var n int
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM marketing_creatives WHERE tenant_id=?`, t).Scan(&n)
		if n >= lim.MaxCreatives {
			http.Error(w, "límite de creativos alcanzado", 403)
			return
		}
		res, e := a.db.Exec(`INSERT INTO marketing_creatives(tenant_id,name,kind,url,notes,created_at) VALUES(?,?,?,?,?,?)`, t, x.Name, x.Kind, x.URL, x.Notes, time.Now().UTC().Format(time.RFC3339))
		if e != nil {
			http.Error(w, e.Error(), 500)
			return
		}
		id, _ := res.LastInsertId()
		writeJSON(w, map[string]any{"id": id})
	default:
		w.WriteHeader(405)
	}
}

func (a *App) marketingOverviewHandler(w http.ResponseWriter, r *http.Request) {
	t, _, e := a.tenantFor(r)
	if e != nil {
		http.Error(w, e.Error(), 401)
		return
	}
	out := map[string]any{}
	for k, q := range map[string]string{"campaigns": "SELECT COUNT(*) FROM marketing_campaigns WHERE tenant_id=?", "scheduled": "SELECT COUNT(*) FROM marketing_content WHERE tenant_id=? AND status='scheduled'", "leads": "SELECT COUNT(*) FROM marketing_leads WHERE tenant_id=?", "forms": "SELECT COUNT(*) FROM marketing_forms WHERE tenant_id=?", "creatives": "SELECT COUNT(*) FROM marketing_creatives WHERE tenant_id=?"} {
		var n int
		_ = a.db.QueryRow(q, t).Scan(&n)
		out[k] = n
	}
	out["limits"] = a.marketingLimits(r, t)
	writeJSON(w, out)
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	r := strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ñ", "n", " ", "-", "_", "-")
	s = r.Replace(s)
	var b strings.Builder
	for _, c := range s {
		if c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' {
			b.WriteRune(c)
		}
	}
	return strings.Trim(b.String(), "-")
}

func (a *App) publicLeadFormHandler(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 3 {
		return
	}
	tenant, _ := strconv.ParseInt(parts[1], 10, 64)
	slug := parts[2]
	var f LeadForm
	var active int
	e := a.db.QueryRow(`SELECT id,tenant_id,name,slug,headline,description,fields_json,consent_text,thank_you,redirect_url,active,created_at FROM marketing_forms WHERE tenant_id=? AND slug=?`, tenant, slug).Scan(&f.ID, &f.TenantID, &f.Name, &f.Slug, &f.Headline, &f.Description, &f.FieldsJSON, &f.ConsentText, &f.ThankYou, &f.RedirectURL, &active, &f.CreatedAt)
	if e != nil || active != 1 {
		http.NotFound(w, r)
		return
	}
	if r.Method == "POST" {
		_ = r.ParseForm()
		consent := r.FormValue("consent") == "on"
		leadName := strings.TrimSpace(r.FormValue("name"))
		leadPhone := strings.TrimSpace(r.FormValue("phone"))
		leadEmail := strings.TrimSpace(r.FormValue("email"))
		createdAt := time.Now().UTC().Format(time.RFC3339)
		_, e := a.db.Exec(`INSERT INTO marketing_leads(tenant_id,form_id,source,name,phone,email,city,interest,score,status,utm_source,utm_campaign,consent,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, tenant, f.ID, "worktic_form", leadName, leadPhone, leadEmail, r.FormValue("city"), r.FormValue("interest"), 60, "new", r.FormValue("utm_source"), r.FormValue("utm_campaign"), boolInt(consent), createdAt)
		if e == nil {
			_ = a.syncCRMContactAt(tenant, leadName, leadPhone, leadEmail, "landing", "form", "", createdAt)
		}
		if e != nil {
			http.Error(w, "No fue posible registrar", 500)
			return
		}
		if f.RedirectURL != "" {
			http.Redirect(w, r, f.RedirectURL, 303)
			return
		}
		fmt.Fprintf(w, "<!doctype html><html><meta charset=utf-8><meta name=viewport content='width=device-width'><style>body{font-family:Arial;background:#f7f5ff;display:grid;place-items:center;min-height:100vh}.box{background:white;padding:40px;border-radius:24px;max-width:560px;box-shadow:0 20px 60px #7c3aed22}</style><div class=box><h1>¡Gracias!</h1><p>%s</p></div></html>", f.ThankYou)
		return
	}
	fmt.Fprintf(w, "<!doctype html><html lang=es><meta charset=utf-8><meta name=viewport content='width=device-width'><title>%s</title><style>body{font-family:Arial;background:linear-gradient(135deg,#f7f5ff,#fff);margin:0;padding:30px;color:#18103a}.box{background:white;padding:34px;border-radius:24px;max-width:620px;margin:30px auto;box-shadow:0 20px 70px #7c3aed22}input,textarea{width:100%%;box-sizing:border-box;padding:13px;margin:7px 0 15px;border:1px solid #ddd6fe;border-radius:10px}button{background:linear-gradient(135deg,#7c3aed,#d946ef);color:white;border:0;padding:14px 24px;border-radius:10px;font-weight:bold}</style><div class=box><h1>%s</h1><p>%s</p><form method=post><input name=name placeholder='Nombre' required><input name=phone placeholder='Teléfono'><input name=email type=email placeholder='Correo'><input name=city placeholder='Ciudad'><textarea name=interest placeholder='¿Qué te interesa?'></textarea><label><input style='width:auto' name=consent type=checkbox required> %s</label><br><br><button>Enviar información</button></form></div></html>", f.Headline, f.Headline, f.Description, f.ConsentText)
}
func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
