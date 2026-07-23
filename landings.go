package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type LandingPage struct {
	ID               int64  `json:"id"`
	TenantID         int64  `json:"tenant_id"`
	Name             string `json:"name"`
	Slug             string `json:"slug"`
	Template         string `json:"template"`
	Headline         string `json:"headline"`
	Subheadline      string `json:"subheadline"`
	Badge            string `json:"badge"`
	PrimaryCTA       string `json:"primary_cta"`
	PrimaryURL       string `json:"primary_url"`
	SecondaryCTA     string `json:"secondary_cta"`
	SecondaryURL     string `json:"secondary_url"`
	HeroImage        string `json:"hero_image"`
	BenefitsJSON     string `json:"benefits_json"`
	FeaturesJSON     string `json:"features_json"`
	TestimonialsJSON string `json:"testimonials_json"`
	FAQJSON          string `json:"faq_json"`
	FormID           int64  `json:"form_id"`
	CampaignID       int64  `json:"campaign_id"`
	Accent           string `json:"accent"`
	Published        bool   `json:"published"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
}

func initLandingSchema(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS marketing_landings(
      id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL,
      name TEXT NOT NULL, slug TEXT NOT NULL, template TEXT NOT NULL DEFAULT 'aurora',
      headline TEXT NOT NULL DEFAULT '', subheadline TEXT NOT NULL DEFAULT '', badge TEXT NOT NULL DEFAULT '',
      primary_cta TEXT NOT NULL DEFAULT 'Quiero más información', primary_url TEXT NOT NULL DEFAULT '#form',
      secondary_cta TEXT NOT NULL DEFAULT '', secondary_url TEXT NOT NULL DEFAULT '', hero_image TEXT NOT NULL DEFAULT '',
      benefits_json TEXT NOT NULL DEFAULT '[]', features_json TEXT NOT NULL DEFAULT '[]', testimonials_json TEXT NOT NULL DEFAULT '[]', faq_json TEXT NOT NULL DEFAULT '[]',
      form_id INTEGER NOT NULL DEFAULT 0, campaign_id INTEGER NOT NULL DEFAULT 0, accent TEXT NOT NULL DEFAULT '#7c3aed',
      published INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
      UNIQUE(tenant_id,slug));
      CREATE INDEX IF NOT EXISTS idx_marketing_landings_tenant ON marketing_landings(tenant_id);`)
	return err
}

func landingLimitFor(plan string) int {
	switch plan {
	case "personal":
		return 5
	case "business":
		return 25
	case "enterprise":
		return 150
	default:
		return 1
	}
}

func (a *App) landingsHandler(w http.ResponseWriter, r *http.Request) {
	t, _, e := a.tenantFor(r)
	if e != nil {
		http.Error(w, e.Error(), 401)
		return
	}
	switch r.Method {
	case "GET":
		rows, e := a.db.Query(`SELECT id,tenant_id,name,slug,template,headline,subheadline,badge,primary_cta,primary_url,secondary_cta,secondary_url,hero_image,benefits_json,features_json,testimonials_json,faq_json,form_id,campaign_id,accent,published,created_at,updated_at FROM marketing_landings WHERE tenant_id=? ORDER BY id DESC`, t)
		if e != nil {
			http.Error(w, e.Error(), 500)
			return
		}
		defer rows.Close()
		out := []LandingPage{}
		for rows.Next() {
			var x LandingPage
			var pub int
			_ = rows.Scan(&x.ID, &x.TenantID, &x.Name, &x.Slug, &x.Template, &x.Headline, &x.Subheadline, &x.Badge, &x.PrimaryCTA, &x.PrimaryURL, &x.SecondaryCTA, &x.SecondaryURL, &x.HeroImage, &x.BenefitsJSON, &x.FeaturesJSON, &x.TestimonialsJSON, &x.FAQJSON, &x.FormID, &x.CampaignID, &x.Accent, &pub, &x.CreatedAt, &x.UpdatedAt)
			x.Published = pub == 1
			out = append(out, x)
		}
		writeJSON(w, out)
	case "POST":
		var x LandingPage
		if json.NewDecoder(r.Body).Decode(&x) != nil || strings.TrimSpace(x.Name) == "" {
			http.Error(w, "nombre obligatorio", 400)
			return
		}
		p, _, _ := a.activePlan(t)
		var n int
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM marketing_landings WHERE tenant_id=?`, t).Scan(&n)
		if n >= landingLimitFor(p.Code) {
			http.Error(w, "límite de landing pages alcanzado para tu plan", 403)
			return
		}
		normalizeLanding(&x)
		now := time.Now().UTC().Format(time.RFC3339)
		res, e := a.db.Exec(`INSERT INTO marketing_landings(tenant_id,name,slug,template,headline,subheadline,badge,primary_cta,primary_url,secondary_cta,secondary_url,hero_image,benefits_json,features_json,testimonials_json,faq_json,form_id,campaign_id,accent,published,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, t, x.Name, x.Slug, x.Template, x.Headline, x.Subheadline, x.Badge, x.PrimaryCTA, x.PrimaryURL, x.SecondaryCTA, x.SecondaryURL, x.HeroImage, x.BenefitsJSON, x.FeaturesJSON, x.TestimonialsJSON, x.FAQJSON, x.FormID, x.CampaignID, x.Accent, boolInt(x.Published), now, now)
		if e != nil {
			http.Error(w, e.Error(), 400)
			return
		}
		id, _ := res.LastInsertId()
		writeJSON(w, map[string]any{"id": id, "public_url": fmt.Sprintf("/l/%d/%s", t, x.Slug)})
	case "PUT":
		var x LandingPage
		if json.NewDecoder(r.Body).Decode(&x) != nil || x.ID == 0 {
			http.Error(w, "datos inválidos", 400)
			return
		}
		normalizeLanding(&x)
		_, e := a.db.Exec(`UPDATE marketing_landings SET name=?,slug=?,template=?,headline=?,subheadline=?,badge=?,primary_cta=?,primary_url=?,secondary_cta=?,secondary_url=?,hero_image=?,benefits_json=?,features_json=?,testimonials_json=?,faq_json=?,form_id=?,campaign_id=?,accent=?,published=?,updated_at=? WHERE id=? AND tenant_id=?`, x.Name, x.Slug, x.Template, x.Headline, x.Subheadline, x.Badge, x.PrimaryCTA, x.PrimaryURL, x.SecondaryCTA, x.SecondaryURL, x.HeroImage, x.BenefitsJSON, x.FeaturesJSON, x.TestimonialsJSON, x.FAQJSON, x.FormID, x.CampaignID, x.Accent, boolInt(x.Published), time.Now().UTC().Format(time.RFC3339), x.ID, t)
		if e != nil {
			http.Error(w, e.Error(), 400)
			return
		}
		writeJSON(w, map[string]bool{"ok": true})
	case "DELETE":
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		_, _ = a.db.Exec(`DELETE FROM marketing_landings WHERE id=? AND tenant_id=?`, id, t)
		writeJSON(w, map[string]bool{"ok": true})
	default:
		w.WriteHeader(405)
	}
}

func normalizeLanding(x *LandingPage) {
	x.Slug = slugify(x.Slug)
	if x.Slug == "" {
		x.Slug = slugify(x.Name)
	}
	if x.Template == "" {
		x.Template = "aurora"
	}
	if x.Accent == "" {
		x.Accent = "#7c3aed"
	}
	if x.PrimaryCTA == "" {
		x.PrimaryCTA = "Quiero más información"
	}
	if x.PrimaryURL == "" {
		x.PrimaryURL = "#form"
	}
	if x.BenefitsJSON == "" {
		x.BenefitsJSON = `["Respuesta inmediata","Atención personalizada","Proceso simple"]`
	}
	if x.FeaturesJSON == "" {
		x.FeaturesJSON = `[]`
	}
	if x.TestimonialsJSON == "" {
		x.TestimonialsJSON = `[]`
	}
	if x.FAQJSON == "" {
		x.FAQJSON = `[]`
	}
}

func (a *App) landingGenerateHandler(w http.ResponseWriter, r *http.Request) {
	t, _, e := a.tenantFor(r)
	if e != nil {
		http.Error(w, e.Error(), 401)
		return
	}
	_ = t
	var q struct{ Name, Product, Audience, Objective, Tone string }
	if json.NewDecoder(r.Body).Decode(&q) != nil {
		http.Error(w, "datos inválidos", 400)
		return
	}
	prompt := fmt.Sprintf(`Crea el contenido de una landing page premium en español para: %s. Producto/servicio: %s. Público: %s. Objetivo: %s. Tono: %s. Devuelve SOLO JSON válido con estas claves: headline, subheadline, badge, primary_cta, benefits (array de 3 textos), features (array de 6 objetos con title y text), testimonials (array de 2 objetos con quote y author), faq (array de 4 objetos con question y answer). No prometas resultados garantizados.`, q.Name, q.Product, q.Audience, q.Objective, q.Tone)
	reply, e := a.aiResponse(prompt, "")
	if e != nil {
		writeJSON(w, map[string]any{"headline": "Convierte más oportunidades con " + q.Product, "subheadline": "Una propuesta clara, profesional y diseñada para que tus visitantes den el siguiente paso.", "badge": "Solución profesional", "primary_cta": "Solicitar información", "benefits": []string{"Atención rápida", "Experiencia sencilla", "Acompañamiento personalizado"}, "features": []map[string]string{{"title": "Diseñado para tu objetivo", "text": "Comunica el valor de tu oferta de forma clara."}, {"title": "Captura integrada", "text": "Los datos llegan directamente a Worktic."}, {"title": "Seguimiento comercial", "text": "Conecta cada registro con tu CRM."}}, "testimonials": []any{}, "faq": []any{}})
		return
	}
	var obj any
	if json.Unmarshal([]byte(strings.TrimSpace(reply)), &obj) != nil {
		writeJSON(w, map[string]any{"raw": reply})
		return
	}
	writeJSON(w, obj)
}

var landingTpl = template.Must(template.New("landing").Funcs(template.FuncMap{"safe": func(s string) template.HTML { return template.HTML(s) }}).Parse(`<!doctype html><html lang="es"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>{{.Name}}</title><meta name="description" content="{{.Subheadline}}"><style>:root{--a:{{.Accent}};--bg:#f7f5ff;--ink:#17112f}*{box-sizing:border-box}body{margin:0;font-family:Inter,system-ui,Arial;color:var(--ink);background:linear-gradient(145deg,#fff,var(--bg))}a{text-decoration:none}.wrap{max-width:1120px;margin:auto;padding:0 24px}.nav{height:72px;display:flex;align-items:center;justify-content:space-between}.logo{font-weight:900;font-size:20px}.btn{display:inline-flex;padding:14px 24px;border-radius:12px;background:var(--a);color:white;font-weight:800;box-shadow:0 10px 28px color-mix(in srgb,var(--a) 28%,transparent)}.hero{padding:80px 0 65px;display:grid;grid-template-columns:1.15fr .85fr;gap:50px;align-items:center}.badge{display:inline-block;padding:7px 13px;border-radius:99px;background:color-mix(in srgb,var(--a) 12%,white);color:var(--a);font-weight:800;font-size:13px}.hero h1{font-size:clamp(42px,7vw,74px);line-height:.98;letter-spacing:-.045em;margin:20px 0}.hero p{font-size:20px;line-height:1.65;color:#655b7e}.heroimg{min-height:420px;border-radius:30px;background:linear-gradient(135deg,color-mix(in srgb,var(--a) 85%,#fff),#e879f9);box-shadow:0 35px 90px #7c3aed26;background-image:url('{{.HeroImage}}');background-size:cover;background-position:center}.benefits{display:grid;grid-template-columns:repeat(3,1fr);gap:16px;margin:30px 0}.benefit,.card,.formbox{background:white;border:1px solid #e8e2ff;border-radius:20px;padding:24px;box-shadow:0 12px 40px #7c3aed10}.section{padding:80px 0}.section h2{text-align:center;font-size:42px}.grid{display:grid;grid-template-columns:repeat(3,1fr);gap:18px}.formbox{max-width:720px;margin:auto}.formbox input,.formbox textarea{width:100%;padding:14px;border:1px solid #ddd6fe;border-radius:11px;margin:7px 0 14px;font:inherit}.formbox button{border:0}.faq details{background:white;border:1px solid #e8e2ff;border-radius:14px;padding:18px 20px;margin:10px 0}.faq summary{font-weight:800;cursor:pointer}footer{text-align:center;padding:45px;color:#776c91}@media(max-width:800px){.wrap{padding:0 18px}.nav{height:64px}.hero{grid-template-columns:1fr;padding:48px 0 42px;gap:28px}.hero h1{font-size:clamp(38px,12vw,58px)}.hero p{font-size:17px}.heroimg{min-height:280px;border-radius:22px}.grid,.benefits{grid-template-columns:1fr}.section{padding:54px 0}.section h2{font-size:32px}.nav .btn{display:none}.btn{width:100%;justify-content:center;text-align:center}.hero a+span,.hero br{display:none}}@media(max-width:480px){.wrap{padding:0 14px}.hero{padding-top:34px}.heroimg{min-height:220px}.benefit,.card,.formbox{padding:18px;border-radius:16px}.section h2{font-size:28px}.formbox input,.formbox textarea{font-size:16px}}</style></head><body><div class="wrap"><div class="nav"><div class="logo">{{.Name}}</div><a class="btn" href="{{.PrimaryURL}}">{{.PrimaryCTA}}</a></div><section class="hero"><div><span class="badge">{{.Badge}}</span><h1>{{.Headline}}</h1><p>{{.Subheadline}}</p><a class="btn" href="{{.PrimaryURL}}">{{.PrimaryCTA}}</a>{{if .SecondaryCTA}} &nbsp; <a href="{{.SecondaryURL}}">{{.SecondaryCTA}}</a>{{end}}<div class="benefits">{{range .Benefits}}<div class="benefit">✓ {{.}}</div>{{end}}</div></div><div class="heroimg"></div></section><section class="section"><h2>Todo lo necesario para avanzar</h2><div class="grid">{{range .Features}}<div class="card"><h3>{{.Title}}</h3><p>{{.Text}}</p></div>{{end}}</div></section>{{if .Testimonials}}<section class="section"><h2>Experiencias</h2><div class="grid">{{range .Testimonials}}<div class="card"><p>“{{.Quote}}”</p><b>{{.Author}}</b></div>{{end}}</div></section>{{end}}{{if .FormHTML}}<section class="section" id="form"><div class="formbox">{{safe .FormHTML}}</div></section>{{end}}{{if .FAQ}}<section class="section faq"><h2>Preguntas frecuentes</h2>{{range .FAQ}}<details><summary>{{.Question}}</summary><p>{{.Answer}}</p></details>{{end}}</section>{{end}}</div><footer>Creado con Worktic AI</footer></body></html>`))

type landingView struct {
	LandingPage
	Benefits     []string
	Features     []struct{ Title, Text string }
	Testimonials []struct{ Quote, Author string }
	FAQ          []struct{ Question, Answer string }
	FormHTML     string
}

func (a *App) publicLandingHandler(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 3 {
		return
	}
	tenant, _ := strconv.ParseInt(parts[1], 10, 64)
	slug := parts[2]
	var x LandingPage
	var pub int
	e := a.db.QueryRow(`SELECT id,tenant_id,name,slug,template,headline,subheadline,badge,primary_cta,primary_url,secondary_cta,secondary_url,hero_image,benefits_json,features_json,testimonials_json,faq_json,form_id,campaign_id,accent,published,created_at,updated_at FROM marketing_landings WHERE tenant_id=? AND slug=?`, tenant, slug).Scan(&x.ID, &x.TenantID, &x.Name, &x.Slug, &x.Template, &x.Headline, &x.Subheadline, &x.Badge, &x.PrimaryCTA, &x.PrimaryURL, &x.SecondaryCTA, &x.SecondaryURL, &x.HeroImage, &x.BenefitsJSON, &x.FeaturesJSON, &x.TestimonialsJSON, &x.FAQJSON, &x.FormID, &x.CampaignID, &x.Accent, &pub, &x.CreatedAt, &x.UpdatedAt)
	if e != nil || pub != 1 {
		http.NotFound(w, r)
		return
	}
	v := landingView{LandingPage: x}
	_ = json.Unmarshal([]byte(x.BenefitsJSON), &v.Benefits)
	_ = json.Unmarshal([]byte(x.FeaturesJSON), &v.Features)
	_ = json.Unmarshal([]byte(x.TestimonialsJSON), &v.Testimonials)
	_ = json.Unmarshal([]byte(x.FAQJSON), &v.FAQ)
	if x.FormID > 0 {
		var f LeadForm
		var active int
		if a.db.QueryRow(`SELECT id,tenant_id,name,slug,headline,description,fields_json,consent_text,thank_you,redirect_url,active,created_at FROM marketing_forms WHERE id=? AND tenant_id=?`, x.FormID, tenant).Scan(&f.ID, &f.TenantID, &f.Name, &f.Slug, &f.Headline, &f.Description, &f.FieldsJSON, &f.ConsentText, &f.ThankYou, &f.RedirectURL, &active, &f.CreatedAt) == nil && active == 1 {
			v.FormHTML = fmt.Sprintf(`<h2>%s</h2><p>%s</p><form method="post" action="/f/%d/%s"><input name="name" placeholder="Nombre" required><input name="phone" placeholder="Teléfono"><input name="email" type="email" placeholder="Correo"><input name="city" placeholder="Ciudad"><textarea name="interest" placeholder="¿Qué te interesa?"></textarea><label><input style="width:auto" name="consent" type="checkbox" required> %s</label><br><br><button class="btn">%s</button></form>`, template.HTMLEscapeString(f.Headline), template.HTMLEscapeString(f.Description), tenant, f.Slug, template.HTMLEscapeString(f.ConsentText), template.HTMLEscapeString(x.PrimaryCTA))
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = landingTpl.Execute(w, v)
}
