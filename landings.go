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
	PremiumJSON      string `json:"premium_json"`
	FormID           int64  `json:"form_id"`
	CampaignID       int64  `json:"campaign_id"`
	Accent           string `json:"accent"`
	Published        bool   `json:"published"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
}

const landingSelectColumns = `id,tenant_id,name,slug,template,headline,subheadline,badge,primary_cta,primary_url,secondary_cta,secondary_url,hero_image,benefits_json,features_json,testimonials_json,faq_json,premium_json,form_id,campaign_id,accent,published,created_at,updated_at`

type landingScanner interface {
	Scan(dest ...any) error
}

func scanLanding(s landingScanner, x *LandingPage) error {
	var published int
	err := s.Scan(&x.ID, &x.TenantID, &x.Name, &x.Slug, &x.Template, &x.Headline, &x.Subheadline, &x.Badge, &x.PrimaryCTA, &x.PrimaryURL, &x.SecondaryCTA, &x.SecondaryURL, &x.HeroImage, &x.BenefitsJSON, &x.FeaturesJSON, &x.TestimonialsJSON, &x.FAQJSON, &x.PremiumJSON, &x.FormID, &x.CampaignID, &x.Accent, &published, &x.CreatedAt, &x.UpdatedAt)
	x.Published = published == 1
	return err
}

func initLandingSchema(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS marketing_landings(
      id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL,
      name TEXT NOT NULL, slug TEXT NOT NULL, template TEXT NOT NULL DEFAULT 'aurora',
      headline TEXT NOT NULL DEFAULT '', subheadline TEXT NOT NULL DEFAULT '', badge TEXT NOT NULL DEFAULT '',
      primary_cta TEXT NOT NULL DEFAULT 'Quiero más información', primary_url TEXT NOT NULL DEFAULT '#contact',
      secondary_cta TEXT NOT NULL DEFAULT '', secondary_url TEXT NOT NULL DEFAULT '', hero_image TEXT NOT NULL DEFAULT '',
      benefits_json TEXT NOT NULL DEFAULT '[]', features_json TEXT NOT NULL DEFAULT '[]', testimonials_json TEXT NOT NULL DEFAULT '[]', faq_json TEXT NOT NULL DEFAULT '[]',
      premium_json TEXT NOT NULL DEFAULT '{}', form_id INTEGER NOT NULL DEFAULT 0, campaign_id INTEGER NOT NULL DEFAULT 0, accent TEXT NOT NULL DEFAULT '#7c3aed',
      published INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
      UNIQUE(tenant_id,slug));
      CREATE INDEX IF NOT EXISTS idx_marketing_landings_tenant ON marketing_landings(tenant_id);`)
	if err != nil {
		return err
	}
	_, _ = db.Exec(`ALTER TABLE marketing_landings ADD COLUMN premium_json TEXT NOT NULL DEFAULT '{}'`)
	return nil
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

func validateLandingJSON(label, raw string) error {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var value []any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return fmt.Errorf("%s contiene datos inválidos", label)
	}
	return nil
}

func (a *App) validateLandingRelations(tenant int64, x *LandingPage) error {
	if x.FormID > 0 {
		var n int
		if err := a.db.QueryRow(`SELECT COUNT(*) FROM marketing_forms WHERE id=? AND tenant_id=?`, x.FormID, tenant).Scan(&n); err != nil || n == 0 {
			return fmt.Errorf("el formulario relacionado no existe o no pertenece a este espacio")
		}
	}
	if x.CampaignID > 0 {
		var n int
		if err := a.db.QueryRow(`SELECT COUNT(*) FROM marketing_campaigns WHERE id=? AND tenant_id=?`, x.CampaignID, tenant).Scan(&n); err != nil || n == 0 {
			return fmt.Errorf("la campaña relacionada no existe o no pertenece a este espacio")
		}
	}
	return nil
}

func prepareLandingForSave(x *LandingPage) error {
	x.Name = strings.TrimSpace(x.Name)
	x.Headline = strings.TrimSpace(x.Headline)
	x.Subheadline = strings.TrimSpace(x.Subheadline)
	x.Badge = strings.TrimSpace(x.Badge)
	x.PrimaryCTA = strings.TrimSpace(x.PrimaryCTA)
	x.PrimaryURL = normalizePublicURL(x.PrimaryURL)
	x.SecondaryCTA = strings.TrimSpace(x.SecondaryCTA)
	x.SecondaryURL = normalizePublicURL(x.SecondaryURL)
	x.HeroImage = strings.TrimSpace(x.HeroImage)
	x.Template = strings.TrimSpace(x.Template)
	x.Accent = normalizeLandingAccent(x.Accent)
	if x.Name == "" {
		return fmt.Errorf("el nombre interno es obligatorio")
	}
	if x.Headline == "" {
		return fmt.Errorf("el título principal es obligatorio")
	}
	if err := validateLandingJSON("beneficios", x.BenefitsJSON); err != nil {
		return err
	}
	if err := validateLandingJSON("características", x.FeaturesJSON); err != nil {
		return err
	}
	if err := validateLandingJSON("testimonios", x.TestimonialsJSON); err != nil {
		return err
	}
	if err := validateLandingJSON("preguntas frecuentes", x.FAQJSON); err != nil {
		return err
	}
	premium, err := normalizeLandingPremiumJSON(x.PremiumJSON)
	if err != nil {
		return err
	}
	x.PremiumJSON = premium
	normalizeLanding(x)
	if x.Slug == "" {
		return fmt.Errorf("no fue posible generar la URL pública; escribe un nombre o slug válido")
	}
	return nil
}

func normalizeLandingAccent(value string) string {
	value = strings.TrimSpace(value)
	if len(value) != 7 || value[0] != '#' {
		return "#7c3aed"
	}
	for _, r := range value[1:] {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return "#7c3aed"
		}
	}
	return value
}

func landingDBError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if strings.Contains(strings.ToLower(msg), "unique") {
		return fmt.Errorf("ya existe una landing con ese slug; cambia la URL pública")
	}
	return fmt.Errorf("no fue posible guardar la landing: %s", msg)
}

func (a *App) landingsHandler(w http.ResponseWriter, r *http.Request) {
	tenant, user, err := a.tenantFor(r)
	if err != nil {
		writeError(w, err, http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodGet:
		rows, err := a.db.Query(`SELECT `+landingSelectColumns+` FROM marketing_landings WHERE tenant_id=? ORDER BY id DESC`, tenant)
		if err != nil {
			writeError(w, err, http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		out := []LandingPage{}
		for rows.Next() {
			var x LandingPage
			if err := scanLanding(rows, &x); err != nil {
				writeError(w, err, http.StatusInternalServerError)
				return
			}
			out = append(out, x)
		}
		if err := rows.Err(); err != nil {
			writeError(w, err, http.StatusInternalServerError)
			return
		}
		writeJSON(w, out)

	case http.MethodPost:
		var x LandingPage
		if err := json.NewDecoder(r.Body).Decode(&x); err != nil {
			writeError(w, fmt.Errorf("datos de landing inválidos"), http.StatusBadRequest)
			return
		}
		if err := prepareLandingForSave(&x); err != nil {
			writeError(w, err, http.StatusBadRequest)
			return
		}
		if err := a.validateLandingRelations(tenant, &x); err != nil {
			writeError(w, err, http.StatusBadRequest)
			return
		}

		billingUserID := a.billingAccountUserID(user)
		plan, _, err := a.activePlan(billingUserID)
		if err != nil {
			writeError(w, fmt.Errorf("no fue posible validar el plan: %w", err), http.StatusInternalServerError)
			return
		}
		var used int
		if err := a.db.QueryRow(`SELECT COUNT(*) FROM marketing_landings WHERE tenant_id=?`, tenant).Scan(&used); err != nil {
			writeError(w, err, http.StatusInternalServerError)
			return
		}
		limit := landingLimitFor(plan.Code)
		if limit >= 0 && used >= limit {
			writeError(w, fmt.Errorf("alcanzaste el límite de %d landing page(s) de tu plan %s; tus datos siguen guardados como borrador local", limit, plan.Name), http.StatusForbidden)
			return
		}

		now := time.Now().UTC().Format(time.RFC3339)
		tx, err := a.db.Begin()
		if err != nil {
			writeError(w, err, http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()
		res, err := tx.Exec(`INSERT INTO marketing_landings(tenant_id,name,slug,template,headline,subheadline,badge,primary_cta,primary_url,secondary_cta,secondary_url,hero_image,benefits_json,features_json,testimonials_json,faq_json,premium_json,form_id,campaign_id,accent,published,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, tenant, x.Name, x.Slug, x.Template, x.Headline, x.Subheadline, x.Badge, x.PrimaryCTA, x.PrimaryURL, x.SecondaryCTA, x.SecondaryURL, x.HeroImage, x.BenefitsJSON, x.FeaturesJSON, x.TestimonialsJSON, x.FAQJSON, x.PremiumJSON, x.FormID, x.CampaignID, x.Accent, boolInt(x.Published), now, now)
		if err != nil {
			writeError(w, landingDBError(err), http.StatusBadRequest)
			return
		}
		x.ID, err = res.LastInsertId()
		if err != nil || x.ID <= 0 {
			writeError(w, fmt.Errorf("el servidor no pudo confirmar el identificador de la landing"), http.StatusInternalServerError)
			return
		}
		if err := tx.Commit(); err != nil {
			writeError(w, fmt.Errorf("no fue posible confirmar el guardado: %w", err), http.StatusInternalServerError)
			return
		}
		x.TenantID = tenant
		x.CreatedAt = now
		x.UpdatedAt = now
		writeJSON(w, map[string]any{"ok": true, "id": x.ID, "public_url": fmt.Sprintf("/l/%d/%s", tenant, x.Slug), "landing": x})

	case http.MethodPut:
		var x LandingPage
		if err := json.NewDecoder(r.Body).Decode(&x); err != nil || x.ID <= 0 {
			writeError(w, fmt.Errorf("landing inválida"), http.StatusBadRequest)
			return
		}
		if err := prepareLandingForSave(&x); err != nil {
			writeError(w, err, http.StatusBadRequest)
			return
		}
		if err := a.validateLandingRelations(tenant, &x); err != nil {
			writeError(w, err, http.StatusBadRequest)
			return
		}
		now := time.Now().UTC().Format(time.RFC3339)
		res, err := a.db.Exec(`UPDATE marketing_landings SET name=?,slug=?,template=?,headline=?,subheadline=?,badge=?,primary_cta=?,primary_url=?,secondary_cta=?,secondary_url=?,hero_image=?,benefits_json=?,features_json=?,testimonials_json=?,faq_json=?,premium_json=?,form_id=?,campaign_id=?,accent=?,published=?,updated_at=? WHERE id=? AND tenant_id=?`, x.Name, x.Slug, x.Template, x.Headline, x.Subheadline, x.Badge, x.PrimaryCTA, x.PrimaryURL, x.SecondaryCTA, x.SecondaryURL, x.HeroImage, x.BenefitsJSON, x.FeaturesJSON, x.TestimonialsJSON, x.FAQJSON, x.PremiumJSON, x.FormID, x.CampaignID, x.Accent, boolInt(x.Published), now, x.ID, tenant)
		if err != nil {
			writeError(w, landingDBError(err), http.StatusBadRequest)
			return
		}
		affected, _ := res.RowsAffected()
		if affected == 0 {
			writeError(w, fmt.Errorf("landing no encontrada o sin acceso"), http.StatusNotFound)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "id": x.ID, "public_url": fmt.Sprintf("/l/%d/%s", tenant, x.Slug)})

	case http.MethodDelete:
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		if id <= 0 {
			writeError(w, fmt.Errorf("landing inválida"), http.StatusBadRequest)
			return
		}
		res, err := a.db.Exec(`DELETE FROM marketing_landings WHERE id=? AND tenant_id=?`, id, tenant)
		if err != nil {
			writeError(w, err, http.StatusInternalServerError)
			return
		}
		affected, _ := res.RowsAffected()
		if affected == 0 {
			writeError(w, fmt.Errorf("landing no encontrada"), http.StatusNotFound)
			return
		}
		writeJSON(w, map[string]any{"ok": true})

	default:
		writeError(w, fmt.Errorf("método no permitido"), http.StatusMethodNotAllowed)
	}
}

func normalizeLanding(x *LandingPage) {
	x.Slug = slugify(x.Slug)
	if x.Slug == "" {
		x.Slug = slugify(x.Name)
	}
	switch x.Template {
	case "minimal", "bold", "midnight":
	default:
		x.Template = "aurora"
	}
	if x.Accent == "" {
		x.Accent = "#7c3aed"
	}
	if x.PrimaryCTA == "" {
		x.PrimaryCTA = "Quiero más información"
	}
	if x.PrimaryURL == "" {
		x.PrimaryURL = "#contact"
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
	if x.PremiumJSON == "" {
		x.PremiumJSON = `{}`
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
	prompt := fmt.Sprintf(`Crea el contenido de una landing page premium en español para: %s. Producto/servicio: %s. Público: %s. Objetivo: %s. Tono: %s. Devuelve SOLO JSON válido con estas claves: headline, subheadline, badge, primary_cta, benefits (array de 3 textos breves), features (array de 6 objetos con title y text), testimonials (array de 3 objetos con quote y author), faq (array de 5 objetos con question y answer), stats (array de 3 objetos con value y label), trust_text. No prometas resultados garantizados.`, q.Name, q.Product, q.Audience, q.Objective, q.Tone)
	reply, e := a.aiResponse(prompt, "")
	if e != nil {
		writeJSON(w, map[string]any{"headline": "Convierte más oportunidades con " + q.Product, "subheadline": "Una propuesta clara, profesional y diseñada para que tus visitantes den el siguiente paso.", "badge": "Solución profesional", "primary_cta": "Solicitar información", "benefits": []string{"Atención rápida", "Experiencia sencilla", "Acompañamiento personalizado"}, "features": []map[string]string{{"title": "Diseñado para tu objetivo", "text": "Comunica el valor de tu oferta de forma clara."}, {"title": "Captura integrada", "text": "Los datos llegan directamente a Worktic."}, {"title": "Seguimiento comercial", "text": "Conecta cada registro con tu CRM."}}, "testimonials": []any{}, "faq": []any{}, "stats": []map[string]string{{"value": "24/7", "label": "Atención disponible"}, {"value": "1 solo lugar", "label": "Canales centralizados"}, {"value": "Automático", "label": "Seguimiento comercial"}}, "trust_text": "Atención rápida, segura y personalizada"})
		return
	}
	var obj any
	if json.Unmarshal([]byte(strings.TrimSpace(reply)), &obj) != nil {
		writeJSON(w, map[string]any{"raw": reply})
		return
	}
	writeJSON(w, obj)
}

var landingTpl = template.Must(template.New("landing-premium").Funcs(template.FuncMap{
	"safe": func(s string) template.HTML { return template.HTML(s) },
}).Parse(`<!doctype html>
<html lang="es">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1,viewport-fit=cover">
<title>{{.Name}}</title>
<meta name="description" content="{{.Subheadline}}">
<meta property="og:title" content="{{.Headline}}">
<meta property="og:description" content="{{.Subheadline}}">
{{if .HeroImage}}<meta property="og:image" content="{{.HeroImage}}">{{end}}
<meta name="theme-color" content="{{.Accent}}">
<style>
:root{--a:{{.Accent}};--a2:#d946ef;--ink:#14102a;--muted:#655e78;--line:#e9e5f2;--surface:#fff;--soft:#f7f5fb;--shadow:0 24px 70px rgba(34,20,64,.12);--radius:26px}
*{box-sizing:border-box}html{scroll-behavior:smooth}body{margin:0;font-family:Inter,ui-sans-serif,system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;color:var(--ink);background:#fff;line-height:1.6}body.theme-midnight{--ink:#f8f6ff;--muted:#c7c0d8;--line:#302944;--surface:#171325;--soft:#100d1b;background:#0e0b17}body.theme-bold{--soft:#fff5fb}.container{width:min(1160px,calc(100% - 40px));margin:auto}a{text-decoration:none;color:inherit}.site-nav{position:sticky;top:0;z-index:40;background:color-mix(in srgb,var(--surface) 88%,transparent);backdrop-filter:blur(18px);border-bottom:1px solid color-mix(in srgb,var(--line) 70%,transparent)}.nav-inner{min-height:76px;display:flex;align-items:center;justify-content:space-between;gap:20px}.brand{display:flex;align-items:center;gap:12px;font-weight:900;letter-spacing:-.02em}.brand-mark{width:38px;height:38px;border-radius:13px;background:linear-gradient(135deg,var(--a),var(--a2));box-shadow:0 10px 28px color-mix(in srgb,var(--a) 28%,transparent);position:relative}.brand-mark:after{content:"";position:absolute;inset:10px;border-radius:7px;border:2px solid #fff}.nav-actions{display:flex;align-items:center;gap:10px}.btn{display:inline-flex;align-items:center;justify-content:center;gap:9px;min-height:50px;padding:0 22px;border-radius:14px;background:linear-gradient(135deg,var(--a),color-mix(in srgb,var(--a) 78%,var(--a2)));color:#fff;font-weight:850;box-shadow:0 14px 34px color-mix(in srgb,var(--a) 28%,transparent);transition:.2s transform,.2s box-shadow}.btn:hover{transform:translateY(-2px);box-shadow:0 18px 38px color-mix(in srgb,var(--a) 35%,transparent)}.btn.secondary{background:var(--surface);color:var(--ink);border:1px solid var(--line);box-shadow:none}.hero{position:relative;overflow:hidden;padding:88px 0 62px;background:radial-gradient(circle at 90% 10%,color-mix(in srgb,var(--a) 16%,transparent),transparent 35%),linear-gradient(180deg,var(--soft),var(--surface))}.hero:before{content:"";position:absolute;width:520px;height:520px;border-radius:50%;background:color-mix(in srgb,var(--a2) 9%,transparent);filter:blur(20px);left:-280px;top:-300px}.hero-grid{position:relative;display:grid;grid-template-columns:minmax(0,1.02fr) minmax(380px,.98fr);gap:64px;align-items:center}.hero.center .hero-grid{grid-template-columns:1fr;text-align:center}.hero.center .hero-copy{max-width:870px;margin:auto}.hero.center .hero-actions,.hero.center .trust-line{justify-content:center}.hero.center .media-card{max-width:900px;margin:18px auto 0}.eyebrow{display:inline-flex;align-items:center;gap:8px;padding:8px 13px;border-radius:999px;background:color-mix(in srgb,var(--a) 11%,var(--surface));border:1px solid color-mix(in srgb,var(--a) 22%,var(--line));color:var(--a);font-size:13px;font-weight:900;letter-spacing:.04em;text-transform:uppercase}.hero h1{font-size:clamp(44px,6vw,72px);line-height:1.02;letter-spacing:-.055em;margin:22px 0 22px;max-width:780px}.hero.center h1{margin-inline:auto}.hero-lead{font-size:clamp(18px,2vw,21px);color:var(--muted);max-width:720px;margin:0 0 30px}.hero-actions{display:flex;flex-wrap:wrap;gap:12px}.trust-line{display:flex;align-items:center;gap:10px;margin-top:22px;color:var(--muted);font-size:14px}.trust-dot{display:inline-flex;width:24px;height:24px;border-radius:50%;align-items:center;justify-content:center;background:#dcfce7;color:#15803d;font-weight:900}.media-card{position:relative;aspect-ratio:4/3;border-radius:32px;overflow:hidden;background:linear-gradient(145deg,color-mix(in srgb,var(--a) 20%,#fff),#eee9fb);border:1px solid color-mix(in srgb,var(--a) 18%,var(--line));box-shadow:var(--shadow)}.media-card img{width:100%;height:100%;display:block;object-fit:{{.Premium.HeroFit}};object-position:center}.video-frame{position:absolute;inset:0;width:100%;height:100%;border:0}.media-placeholder{height:100%;display:grid;place-items:center;text-align:center;padding:40px;color:var(--muted)}.media-placeholder strong{display:block;color:var(--ink);font-size:24px}.channel-mini{position:absolute;left:22px;right:22px;bottom:20px;display:flex;flex-wrap:wrap;gap:8px}.channel-pill{display:inline-flex;align-items:center;gap:8px;padding:9px 12px;border-radius:999px;background:rgba(255,255,255,.92);color:#181126;font-size:13px;font-weight:850;box-shadow:0 10px 26px rgba(24,17,38,.14)}.channel-icon{width:22px;height:22px;border-radius:7px;display:grid;place-items:center;background:var(--a);color:#fff;font-size:11px;font-weight:900}.stats{position:relative;margin-top:-2px;padding:0 0 36px;background:var(--surface)}.stats-grid{display:grid;grid-template-columns:repeat(3,1fr);gap:14px}.stat{padding:23px;border:1px solid var(--line);background:var(--surface);border-radius:20px;text-align:center;box-shadow:0 10px 30px rgba(30,18,55,.05)}.stat strong{display:block;font-size:28px;letter-spacing:-.03em;color:var(--a)}.stat span{color:var(--muted);font-size:14px}.benefits-wrap{padding:28px 0 10px}.benefits{display:grid;grid-template-columns:repeat(3,1fr);gap:14px}.benefit{display:flex;gap:12px;align-items:flex-start;padding:19px 20px;border:1px solid var(--line);border-radius:18px;background:var(--surface);font-weight:720}.benefit-check{flex:0 0 28px;width:28px;height:28px;border-radius:9px;background:color-mix(in srgb,var(--a) 12%,var(--surface));color:var(--a);display:grid;place-items:center;font-weight:950}.section{padding:92px 0}.section.soft{background:var(--soft)}.section-head{max-width:760px;margin:0 auto 42px;text-align:center}.section-head .kicker{color:var(--a);font-weight:900;text-transform:uppercase;letter-spacing:.08em;font-size:12px}.section h2{font-size:clamp(34px,4.5vw,52px);line-height:1.08;letter-spacing:-.045em;margin:10px 0 14px}.section-head p{margin:0;color:var(--muted);font-size:18px}.feature-grid{display:grid;grid-template-columns:repeat(3,1fr);gap:18px}.card{background:var(--surface);border:1px solid var(--line);border-radius:22px;padding:28px;box-shadow:0 14px 38px rgba(30,18,55,.055)}.feature-number{width:42px;height:42px;border-radius:13px;background:linear-gradient(135deg,var(--a),var(--a2));color:#fff;display:grid;place-items:center;font-weight:900;margin-bottom:22px}.card h3{font-size:20px;margin:0 0 10px}.card p{color:var(--muted);margin:0}.video-section .video-shell{max-width:980px;margin:auto;aspect-ratio:16/9;border-radius:28px;overflow:hidden;background:#111;box-shadow:var(--shadow);border:1px solid var(--line);position:relative}.video-section iframe{width:100%;height:100%;border:0}.testimonial-grid{display:grid;grid-template-columns:repeat(3,1fr);gap:18px}.quote{font-size:17px;line-height:1.7;margin:0 0 22px}.author{display:flex;align-items:center;gap:12px;font-weight:850}.author-avatar{width:42px;height:42px;border-radius:50%;display:grid;place-items:center;background:color-mix(in srgb,var(--a) 14%,var(--surface));color:var(--a)}.contact-panel{display:grid;grid-template-columns:.85fr 1.15fr;gap:26px;align-items:stretch;padding:34px;border-radius:30px;background:linear-gradient(135deg,color-mix(in srgb,var(--a) 11%,var(--surface)),var(--surface));border:1px solid color-mix(in srgb,var(--a) 18%,var(--line));box-shadow:var(--shadow)}.contact-copy h2{text-align:left;margin-top:8px}.contact-copy p{color:var(--muted);font-size:17px}.contact-options{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:12px}.contact-card{display:flex;align-items:center;gap:14px;padding:17px;border-radius:17px;background:var(--surface);border:1px solid var(--line);transition:.2s}.contact-card:hover{transform:translateY(-2px);border-color:color-mix(in srgb,var(--a) 45%,var(--line));box-shadow:0 14px 30px rgba(30,18,55,.08)}.contact-card .channel-icon{width:42px;height:42px;border-radius:13px}.contact-card b{display:block}.contact-card small{color:var(--muted)}.formbox{max-width:820px;margin:auto;background:var(--surface);border:1px solid var(--line);border-radius:28px;padding:38px;box-shadow:var(--shadow)}.formbox h2{text-align:left;margin:0 0 8px}.formbox>p{color:var(--muted);margin-top:0}.formbox form{display:grid;grid-template-columns:1fr 1fr;gap:14px}.formbox form input,.formbox form textarea{width:100%;padding:14px 15px;border:1px solid var(--line);border-radius:13px;background:var(--surface);color:var(--ink);font:inherit;outline:none}.formbox form input:focus,.formbox form textarea:focus{border-color:var(--a);box-shadow:0 0 0 4px color-mix(in srgb,var(--a) 12%,transparent)}.formbox form textarea,.formbox form label,.formbox form br,.formbox form button{grid-column:1/-1}.formbox form label{display:flex;gap:8px;align-items:flex-start;font-size:13px;color:var(--muted)}.formbox form button{border:0;cursor:pointer}.faq-list{max-width:900px;margin:auto}.faq-list details{background:var(--surface);border:1px solid var(--line);border-radius:17px;margin:11px 0;padding:0 21px}.faq-list summary{cursor:pointer;font-weight:850;padding:20px 30px 20px 0;position:relative}.faq-list summary:after{content:"+";position:absolute;right:0;font-size:24px;color:var(--a);top:14px}.faq-list details[open] summary:after{content:"−"}.faq-list details p{color:var(--muted);padding:0 0 20px;margin:0}.footer{border-top:1px solid var(--line);padding:36px 0;color:var(--muted)}.footer-inner{display:flex;justify-content:space-between;gap:20px;align-items:center}.floating-channels{position:fixed;right:18px;bottom:22px;z-index:50;display:grid;gap:9px}.floating-channel{width:52px;height:52px;border-radius:17px;display:grid;place-items:center;background:var(--surface);border:1px solid var(--line);box-shadow:0 15px 35px rgba(25,16,43,.18);font-weight:900;color:var(--a)}.mobile-sticky{display:none}
@media(max-width:920px){.hero{padding-top:58px}.hero-grid{grid-template-columns:1fr;gap:38px}.hero-copy{max-width:780px}.media-card{max-width:760px;width:100%}.feature-grid,.testimonial-grid{grid-template-columns:repeat(2,1fr)}.contact-panel{grid-template-columns:1fr}.benefits,.stats-grid{grid-template-columns:1fr 1fr}.nav-actions .btn.secondary{display:none}}
@media(max-width:640px){.container{width:min(100% - 28px,1160px)}.site-nav{position:relative}.nav-inner{min-height:66px}.nav-actions{display:none}.hero{padding:46px 0 42px}.hero h1{font-size:clamp(38px,12vw,54px)}.hero-lead{font-size:17px}.hero-actions{display:grid}.hero-actions .btn{width:100%}.media-card{border-radius:22px;aspect-ratio:4/3}.channel-mini{left:12px;right:12px;bottom:12px}.channel-pill{padding:7px 9px;font-size:11px}.stats{padding-top:8px}.stats-grid,.benefits,.feature-grid,.testimonial-grid,.contact-options{grid-template-columns:1fr}.section{padding:64px 0}.section-head{margin-bottom:28px}.section h2{font-size:34px}.card{padding:22px}.contact-panel{padding:22px;border-radius:22px}.formbox{padding:22px;border-radius:22px}.formbox form{grid-template-columns:1fr}.footer-inner{display:grid;gap:5px;text-align:center}.floating-channels{display:none}.mobile-sticky{position:fixed;left:10px;right:10px;bottom:calc(10px + env(safe-area-inset-bottom));z-index:55;display:flex;align-items:center;gap:8px;padding:8px;background:rgba(255,255,255,.94);border:1px solid var(--line);border-radius:18px;box-shadow:0 18px 45px rgba(25,16,43,.22);backdrop-filter:blur(16px)}body.theme-midnight .mobile-sticky{background:rgba(23,19,37,.94)}.mobile-sticky .btn{flex:1;min-height:46px}.mobile-channel{width:46px;height:46px;border-radius:14px;display:grid;place-items:center;background:var(--surface);border:1px solid var(--line);color:var(--a);font-weight:900}.footer{padding-bottom:100px}}
</style>
</head>
<body class="theme-{{.TemplateClass}}">
<nav class="site-nav"><div class="container nav-inner"><a class="brand" href="#top"><span class="brand-mark"></span><span>{{.BrandName}}</span></a><div class="nav-actions">{{if .SecondaryCTA}}<a class="btn secondary" href="{{.SecondaryURL}}">{{.SecondaryCTA}}</a>{{end}}<a class="btn" href="{{.PrimaryURL}}">{{.PrimaryCTA}}</a></div></div></nav>
<main id="top">
<section class="hero {{.Premium.HeroLayout}}"><div class="container hero-grid"><div class="hero-copy"><span class="eyebrow">{{.Badge}}</span><h1>{{.Headline}}</h1><p class="hero-lead">{{.Subheadline}}</p><div class="hero-actions"><a class="btn" href="{{.PrimaryURL}}">{{.PrimaryCTA}}</a>{{if .SecondaryCTA}}<a class="btn secondary" href="{{.SecondaryURL}}">{{.SecondaryCTA}}</a>{{end}}</div><div class="trust-line"><span class="trust-dot">✓</span><span>{{.Premium.TrustText}}</span></div></div><div class="media-card">{{if and (eq .Premium.MediaType "video") .VideoEmbed}}<iframe class="video-frame" src="{{.VideoEmbed}}" title="Video de {{.Name}}" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share" allowfullscreen></iframe>{{else if .HeroImage}}<img src="{{.HeroImage}}" alt="{{.Premium.HeroAlt}}" loading="eager">{{else}}<div class="media-placeholder"><div><strong>Tu propuesta merece una gran imagen</strong><span>Agrega una imagen de 1600 × 1200 px o un video de YouTube/Vimeo desde el editor.</span></div></div>{{end}}{{if and .Channels (ne .Premium.MediaType "video")}}<div class="channel-mini">{{range .Channels}}<a class="channel-pill" href="{{.URL}}" target="_blank" rel="noopener"><span class="channel-icon">{{if eq .Type "whatsapp"}}WA{{else if eq .Type "telegram"}}TG{{else if eq .Type "messenger"}}MS{{else if eq .Type "instagram"}}IG{{else}}↗{{end}}</span>{{.Label}}</a>{{end}}</div>{{end}}</div></div></section>
{{if .Premium.Stats}}<section class="stats"><div class="container stats-grid">{{range .Premium.Stats}}<div class="stat"><strong>{{.Value}}</strong><span>{{.Label}}</span></div>{{end}}</div></section>{{end}}
{{if .Benefits}}<section class="benefits-wrap"><div class="container benefits">{{range .Benefits}}<div class="benefit"><span class="benefit-check">✓</span><span>{{.}}</span></div>{{end}}</div></section>{{end}}
{{if .Features}}<section class="section soft" id="features"><div class="container"><div class="section-head"><span class="kicker">Una experiencia completa</span><h2>Todo lo necesario para dar el siguiente paso</h2><p>Presenta tu oferta con claridad, confianza y una experiencia diseñada para convertir visitas en conversaciones reales.</p></div><div class="feature-grid">{{range $i,$f := .Features}}<article class="card"><div class="feature-number">{{if eq $i 0}}01{{else if eq $i 1}}02{{else if eq $i 2}}03{{else if eq $i 3}}04{{else if eq $i 4}}05{{else}}06{{end}}</div><h3>{{$f.Title}}</h3><p>{{$f.Text}}</p></article>{{end}}</div></div></section>{{end}}
{{if and .VideoEmbed (eq .Premium.VideoPlacement "section") (ne .Premium.MediaType "video")}}<section class="section video-section"><div class="container"><div class="section-head"><span class="kicker">Conoce la solución</span><h2>Mira cómo funciona</h2><p>Explica tu propuesta con un video claro, profesional y fácil de reproducir desde cualquier dispositivo.</p></div><div class="video-shell"><iframe src="{{.VideoEmbed}}" title="Video de {{.Name}}" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share" allowfullscreen></iframe></div></div></section>{{end}}
{{if .Testimonials}}<section class="section"><div class="container"><div class="section-head"><span class="kicker">Confianza comprobada</span><h2>Lo que dicen nuestros clientes</h2></div><div class="testimonial-grid">{{range .Testimonials}}<article class="card"><p class="quote">“{{.Quote}}”</p><div class="author"><span class="author-avatar">★</span><span>{{.Author}}</span></div></article>{{end}}</div></div></section>{{end}}
{{if and .Premium.ShowContactPanel .Channels}}<section class="section soft" id="contact"><div class="container"><div class="contact-panel"><div class="contact-copy"><span class="eyebrow">Contacto directo</span><h2>{{.Premium.ContactTitle}}</h2><p>{{.Premium.ContactSubtitle}}</p></div><div class="contact-options">{{range .Channels}}<a class="contact-card" href="{{.URL}}" target="_blank" rel="noopener"><span class="channel-icon">{{if eq .Type "whatsapp"}}WA{{else if eq .Type "telegram"}}TG{{else if eq .Type "messenger"}}MS{{else if eq .Type "instagram"}}IG{{else if eq .Type "facebook"}}FB{{else}}↗{{end}}</span><span><b>{{.Label}}</b><small>{{.Name}}</small></span></a>{{end}}</div></div></div></section>{{end}}
{{if .FormHTML}}<section class="section" id="form"><div class="container"><div class="formbox">{{safe .FormHTML}}</div></div></section>{{end}}
{{if .FAQ}}<section class="section soft"><div class="container"><div class="section-head"><span class="kicker">Resuelve tus dudas</span><h2>Preguntas frecuentes</h2></div><div class="faq-list">{{range .FAQ}}<details><summary>{{.Question}}</summary><p>{{.Answer}}</p></details>{{end}}</div></div></section>{{end}}
</main>
<footer class="footer"><div class="container footer-inner"><strong>{{.BrandName}}</strong><span>{{.FooterText}}</span></div></footer>
{{if and .Premium.ShowFloatingChannels .Channels}}<div class="floating-channels">{{range .Channels}}<a class="floating-channel" href="{{.URL}}" title="{{.Label}}" target="_blank" rel="noopener">{{if eq .Type "whatsapp"}}WA{{else if eq .Type "telegram"}}TG{{else if eq .Type "messenger"}}MS{{else if eq .Type "instagram"}}IG{{else}}↗{{end}}</a>{{end}}</div>{{end}}
{{if .Premium.ShowStickyCTA}}<div class="mobile-sticky">{{range .Channels}}{{if eq .Type "whatsapp"}}<a class="mobile-channel" href="{{.URL}}" target="_blank" rel="noopener">WA</a>{{end}}{{end}}<a class="btn" href="{{.PrimaryURL}}">{{.PrimaryCTA}}</a></div>{{end}}
</body></html>`))

type landingView struct {
	LandingPage
	Premium       landingPremiumConfig
	Benefits      []string
	Features      []struct{ Title, Text string }
	Testimonials  []struct{ Quote, Author string }
	FAQ           []struct{ Question, Answer string }
	FormHTML      string
	VideoEmbed    template.URL
	Channels      []landingContactOption
	BrandName     string
	FooterText    string
	TemplateClass string
}

func (a *App) publicLandingHandler(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 3 {
		http.NotFound(w, r)
		return
	}
	tenant, _ := strconv.ParseInt(parts[1], 10, 64)
	slug := parts[2]
	var x LandingPage
	if err := scanLanding(a.db.QueryRow(`SELECT `+landingSelectColumns+` FROM marketing_landings WHERE tenant_id=? AND slug=?`, tenant, slug), &x); err != nil || !x.Published {
		http.NotFound(w, r)
		return
	}
	v := landingView{LandingPage: x, Premium: parseLandingPremium(x.PremiumJSON), TemplateClass: x.Template}
	_ = json.Unmarshal([]byte(x.BenefitsJSON), &v.Benefits)
	_ = json.Unmarshal([]byte(x.FeaturesJSON), &v.Features)
	_ = json.Unmarshal([]byte(x.TestimonialsJSON), &v.Testimonials)
	_ = json.Unmarshal([]byte(x.FAQJSON), &v.FAQ)
	v.VideoEmbed = landingVideoEmbed(v.Premium.VideoURL)
	allChannels := a.landingContactOptions(tenant, v.Premium.ContactMessage)
	v.Channels = filterLandingContactOptions(allChannels, v.Premium.ChannelKeys)
	v.BrandName = firstNonEmpty(v.Premium.BrandName, x.Name)
	v.Premium.HeroAlt = firstNonEmpty(v.Premium.HeroAlt, v.BrandName)
	v.FooterText = firstNonEmpty(v.Premium.FooterText, "Creado con Worktic AI")
	if x.PrimaryURL == "#contact" && (!v.Premium.ShowContactPanel || len(v.Channels) == 0) {
		if x.FormID > 0 {
			v.PrimaryURL = "#form"
		} else {
			v.PrimaryURL = "#top"
		}
	}
	if x.FormID > 0 {
		var f LeadForm
		var active int
		if a.db.QueryRow(`SELECT id,tenant_id,name,slug,headline,description,fields_json,consent_text,thank_you,redirect_url,active,created_at FROM marketing_forms WHERE id=? AND tenant_id=?`, x.FormID, tenant).Scan(&f.ID, &f.TenantID, &f.Name, &f.Slug, &f.Headline, &f.Description, &f.FieldsJSON, &f.ConsentText, &f.ThankYou, &f.RedirectURL, &active, &f.CreatedAt) == nil && active == 1 {
			v.FormHTML = fmt.Sprintf(`<h2>%s</h2><p>%s</p><form method="post" action="/f/%d/%s"><input name="name" placeholder="Nombre completo" required><input name="phone" placeholder="WhatsApp o teléfono"><input name="email" type="email" placeholder="Correo electrónico"><input name="city" placeholder="Ciudad"><textarea name="interest" rows="4" placeholder="Cuéntanos qué necesitas"></textarea><label><input style="width:auto" name="consent" type="checkbox" required> %s</label><button class="btn">%s</button></form>`, template.HTMLEscapeString(f.Headline), template.HTMLEscapeString(f.Description), tenant, f.Slug, template.HTMLEscapeString(f.ConsentText), template.HTMLEscapeString(x.PrimaryCTA))
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=60")
	_ = landingTpl.Execute(w, v)
}
