package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

type dashboardSeriesPointV27 struct {
	Date          string  `json:"date"`
	Leads         int     `json:"leads"`
	Sales         int     `json:"sales"`
	Revenue       float64 `json:"revenue"`
	Conversations int     `json:"conversations"`
}

func pctChangeV27(current, previous float64) float64 {
	if previous == 0 {
		if current > 0 {
			return 100
		}
		return 0
	}
	return (current - previous) * 100 / previous
}

func (a *App) dashboardV27Handler(w http.ResponseWriter, r *http.Request) {
	tid, u, err := a.tenantFor(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	now := time.Now().UTC()
	from := now.AddDate(0, 0, -29).Format("2006-01-02")
	prevFrom := now.AddDate(0, 0, -59).Format("2006-01-02")
	prevTo := now.AddDate(0, 0, -30).Format("2006-01-02")

	var leads, prevLeads, sales, prevSales, conversations, prevConversations, appointments, prevAppointments int
	var revenue, prevRevenue, pipeline float64
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM marketing_leads WHERE tenant_id=? AND substr(created_at,1,10)>=?`, tid, from).Scan(&leads)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM marketing_leads WHERE tenant_id=? AND substr(created_at,1,10)>=? AND substr(created_at,1,10)<=?`, tid, prevFrom, prevTo).Scan(&prevLeads)
	_ = a.db.QueryRow(`SELECT COUNT(*),COALESCE(SUM(value),0) FROM crm_opportunities WHERE tenant_id=? AND stage='Ganado' AND COALESCE(deleted_at,'')='' AND substr(updated_at,1,10)>=?`, tid, from).Scan(&sales, &revenue)
	_ = a.db.QueryRow(`SELECT COUNT(*),COALESCE(SUM(value),0) FROM crm_opportunities WHERE tenant_id=? AND stage='Ganado' AND COALESCE(deleted_at,'')='' AND substr(updated_at,1,10)>=? AND substr(updated_at,1,10)<=?`, tid, prevFrom, prevTo).Scan(&prevSales, &prevRevenue)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM worktic_messages WHERE tenant_id=? AND direction='in' AND substr(timestamp,1,10)>=?`, tid, from).Scan(&conversations)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM worktic_messages WHERE tenant_id=? AND direction='in' AND substr(timestamp,1,10)>=? AND substr(timestamp,1,10)<=?`, tid, prevFrom, prevTo).Scan(&prevConversations)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM crm_appointments WHERE tenant_id=? AND substr(created_at,1,10)>=?`, tid, from).Scan(&appointments)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM crm_appointments WHERE tenant_id=? AND substr(created_at,1,10)>=? AND substr(created_at,1,10)<=?`, tid, prevFrom, prevTo).Scan(&prevAppointments)
	_ = a.db.QueryRow(`SELECT COALESCE(SUM(value),0) FROM crm_opportunities WHERE tenant_id=? AND COALESCE(deleted_at,'')='' AND stage NOT IN ('Ganado','Perdido')`, tid).Scan(&pipeline)

	var contacts, openOpps, unread, activeAgents, activeAutomations, connectedChannels int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM crm_contacts WHERE tenant_id=? AND COALESCE(deleted_at,'')=''`, tid).Scan(&contacts)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM crm_opportunities WHERE tenant_id=? AND COALESCE(deleted_at,'')='' AND stage NOT IN ('Ganado','Perdido')`, tid).Scan(&openOpps)
	_ = a.db.QueryRow(`SELECT COALESCE(SUM(unread),0) FROM worktic_contacts WHERE tenant_id=?`, tid).Scan(&unread)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM ai_agents WHERE tenant_id=? AND status='active'`, tid).Scan(&activeAgents)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM automation_workflows WHERE tenant_id=? AND status='active'`, tid).Scan(&activeAutomations)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM channel_connections WHERE tenant_id=? AND status='connected'`, tid).Scan(&connectedChannels)

	seriesMap := map[string]*dashboardSeriesPointV27{}
	for i := 13; i >= 0; i-- {
		d := now.AddDate(0, 0, -i).Format("2006-01-02")
		seriesMap[d] = &dashboardSeriesPointV27{Date: d}
	}
	mergeCounts := func(q string, set func(*dashboardSeriesPointV27, int, float64)) {
		rows, err := a.db.Query(q, tid, now.AddDate(0, 0, -13).Format("2006-01-02"))
		if err != nil {
			return
		}
		defer rows.Close()
		for rows.Next() {
			var d string
			var n int
			var value float64
			if strings.Contains(q, "SUM(value)") {
				if rows.Scan(&d, &n, &value) != nil {
					continue
				}
			} else if rows.Scan(&d, &n) != nil {
				continue
			}
			if p := seriesMap[d]; p != nil {
				set(p, n, value)
			}
		}
	}
	mergeCounts(`SELECT substr(created_at,1,10),COUNT(*) FROM marketing_leads WHERE tenant_id=? AND substr(created_at,1,10)>=? GROUP BY substr(created_at,1,10)`, func(p *dashboardSeriesPointV27, n int, _ float64) { p.Leads = n })
	mergeCounts(`SELECT substr(updated_at,1,10),COUNT(*),COALESCE(SUM(value),0) FROM crm_opportunities WHERE tenant_id=? AND stage='Ganado' AND COALESCE(deleted_at,'')='' AND substr(updated_at,1,10)>=? GROUP BY substr(updated_at,1,10)`, func(p *dashboardSeriesPointV27, n int, v float64) { p.Sales, p.Revenue = n, v })
	mergeCounts(`SELECT substr(timestamp,1,10),COUNT(*) FROM worktic_messages WHERE tenant_id=? AND direction='in' AND substr(timestamp,1,10)>=? GROUP BY substr(timestamp,1,10)`, func(p *dashboardSeriesPointV27, n int, _ float64) { p.Conversations = n })
	series := make([]dashboardSeriesPointV27, 0, len(seriesMap))
	for _, p := range seriesMap {
		series = append(series, *p)
	}
	sort.Slice(series, func(i, j int) bool { return series[i].Date < series[j].Date })

	type sourceRow struct {
		Source string `json:"source"`
		Leads  int    `json:"leads"`
	}
	sources := []sourceRow{}
	if rows, err := a.db.Query(`SELECT CASE WHEN source='' THEN 'direct' ELSE source END,COUNT(*) FROM marketing_leads WHERE tenant_id=? AND substr(created_at,1,10)>=? GROUP BY CASE WHEN source='' THEN 'direct' ELSE source END ORDER BY COUNT(*) DESC LIMIT 6`, tid, from); err == nil {
		defer rows.Close()
		for rows.Next() {
			var x sourceRow
			if rows.Scan(&x.Source, &x.Leads) == nil {
				sources = append(sources, x)
			}
		}
	}

	insights := []map[string]string{}
	if leads > 0 && sales == 0 {
		insights = append(insights, map[string]string{"type": "warning", "title": "Leads sin cierres", "text": fmt.Sprintf("Tienes %d leads nuevos en 30 días y todavía no hay ventas ganadas registradas. Revisa seguimiento y pipeline.", leads)})
	}
	if openOpps > 10 {
		insights = append(insights, map[string]string{"type": "opportunity", "title": "Pipeline activo", "text": fmt.Sprintf("Hay %d oportunidades abiertas por un valor aproximado de %.0f. Prioriza las de mayor valor o antigüedad.", openOpps, pipeline)})
	}
	if unread > 0 {
		insights = append(insights, map[string]string{"type": "warning", "title": "Conversaciones pendientes", "text": fmt.Sprintf("Hay %d mensajes sin leer. El Inbox debe mantenerse al día para reducir fugas comerciales.", unread)})
	}
	if connectedChannels == 0 {
		insights = append(insights, map[string]string{"type": "setup", "title": "Conecta tus canales", "text": "No hay canales conectados. Conecta al menos WhatsApp Cloud o un canal social para activar el flujo omnicanal."})
	}
	if activeAutomations == 0 {
		insights = append(insights, map[string]string{"type": "setup", "title": "Automatización disponible", "text": "Aún no tienes workflows activos. Puedes automatizar seguimiento, tareas y cambios de pipeline."})
	}
	if len(insights) == 0 {
		insights = append(insights, map[string]string{"type": "good", "title": "Operación saludable", "text": "Tu operación tiene actividad y no se detectaron alertas básicas. Revisa Analytics para profundizar en atribución y ROAS."})
	}

	writeJSON(w, map[string]any{
		"period": map[string]any{"from": from, "to": now.Format("2006-01-02"), "days": 30},
		"user":   map[string]any{"name": u.Name, "company": u.Company},
		"kpis": map[string]any{
			"revenue": revenue, "revenue_change": pctChangeV27(revenue, prevRevenue),
			"sales": sales, "sales_change": pctChangeV27(float64(sales), float64(prevSales)),
			"leads": leads, "leads_change": pctChangeV27(float64(leads), float64(prevLeads)),
			"conversations": conversations, "conversations_change": pctChangeV27(float64(conversations), float64(prevConversations)),
			"appointments": appointments, "appointments_change": pctChangeV27(float64(appointments), float64(prevAppointments)),
			"contacts": contacts, "open_opportunities": openOpps, "pipeline_value": pipeline, "unread": unread,
			"conversion_rate": func() float64 {
				if leads == 0 {
					return 0
				}
				return float64(sales) * 100 / float64(leads)
			}(),
		},
		"operations": map[string]any{"connected_channels": connectedChannels, "active_agents": activeAgents, "active_automations": activeAutomations, "unread_messages": unread},
		"series":     series, "sources": sources, "insights": insights,
	})
}

func (a *App) dashboardActivityV27Handler(w http.ResponseWriter, r *http.Request) {
	tid, _, err := a.tenantFor(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	page, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("page")))
	perPage, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("per_page")))
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 5
	}
	if perPage > 20 {
		perPage = 20
	}
	search := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("search")))
	where := `tenant_id=?`
	args := []any{tid}
	if search != "" {
		where += ` AND (lower(COALESCE(event_type,'')) LIKE ? OR lower(COALESCE(channel,'')) LIKE ? OR lower(COALESCE(source,'')) LIKE ?)`
		like := "%" + search + "%"
		args = append(args, like, like, like)
	}

	var total int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM analytics_attribution_events_v24 WHERE `+where, args...).Scan(&total); err != nil {
		writeError(w, err, 500)
		return
	}

	offset := (page - 1) * perPage
	queryArgs := append(append([]any{}, args...), perPage, offset)
	rows, err := a.db.Query(`SELECT event_type,channel,source,value,occurred_at FROM analytics_attribution_events_v24 WHERE `+where+` ORDER BY occurred_at DESC,id DESC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		writeError(w, err, 500)
		return
	}
	defer rows.Close()

	items := []map[string]any{}
	for rows.Next() {
		var eventType, channel, source, occurred string
		var value float64
		if rows.Scan(&eventType, &channel, &source, &value, &occurred) == nil {
			items = append(items, map[string]any{
				"event_type":  eventType,
				"channel":     channel,
				"source":      source,
				"value":       value,
				"occurred_at": occurred,
			})
		}
	}
	pages := 0
	if total > 0 {
		pages = (total + perPage - 1) / perPage
	}
	writeJSON(w, map[string]any{
		"items": items,
		"pagination": map[string]any{
			"page": page, "per_page": perPage, "total": total, "pages": pages,
		},
	})
}

func (a *App) copilotContextV27(tid int64) map[string]any {
	ctx := map[string]any{}
	var connected, socialConnected, adsConnected, agents, workflows, products, contacts, leads, openOpps int
	var forms, landings, growthCampaigns, appointments, unread, waMarketing, pendingPayments int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM channel_connections WHERE tenant_id=? AND status='connected'`, tid).Scan(&connected)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM social_connections WHERE tenant_id=? AND status='connected'`, tid).Scan(&socialConnected)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM ads_connections_v25 WHERE tenant_id=? AND status='connected'`, tid).Scan(&adsConnected)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM ai_agents WHERE tenant_id=? AND status='active'`, tid).Scan(&agents)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM automation_workflows WHERE tenant_id=? AND status='active'`, tid).Scan(&workflows)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM crm_products WHERE tenant_id=? AND active=1`, tid).Scan(&products)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM crm_contacts WHERE tenant_id=? AND COALESCE(deleted_at,'')=''`, tid).Scan(&contacts)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM marketing_leads WHERE tenant_id=?`, tid).Scan(&leads)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM crm_opportunities WHERE tenant_id=? AND COALESCE(deleted_at,'')='' AND stage NOT IN ('Ganado','Perdido')`, tid).Scan(&openOpps)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM marketing_forms WHERE tenant_id=? AND active=1`, tid).Scan(&forms)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM marketing_landings WHERE tenant_id=? AND published=1`, tid).Scan(&landings)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM marketing_campaigns WHERE tenant_id=? AND status NOT IN ('archived','cancelled')`, tid).Scan(&growthCampaigns)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM crm_appointments WHERE tenant_id=?`, tid).Scan(&appointments)
	_ = a.db.QueryRow(`SELECT COALESCE(SUM(unread),0) FROM worktic_contacts WHERE tenant_id=?`, tid).Scan(&unread)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM whatsapp_marketing_campaigns_v27 WHERE tenant_id=? AND status IN ('running','scheduled')`, tid).Scan(&waMarketing)
	var ownerID int64
	_ = a.db.QueryRow(`SELECT owner_user_id FROM tenants WHERE id=?`, tid).Scan(&ownerID)
	if ownerID > 0 {
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM billing_payments WHERE user_id=? AND status='pending'`, ownerID).Scan(&pendingPayments)
	}
	ctx["connected_channels"] = connected
	ctx["connected_social_accounts"] = socialConnected
	ctx["connected_ads_accounts"] = adsConnected
	ctx["active_agents"] = agents
	ctx["active_workflows"] = workflows
	ctx["products"] = products
	ctx["contacts"] = contacts
	ctx["leads"] = leads
	ctx["open_opportunities"] = openOpps
	ctx["active_forms"] = forms
	ctx["published_landings"] = landings
	ctx["growth_campaigns"] = growthCampaigns
	ctx["appointments"] = appointments
	ctx["unread_messages"] = unread
	ctx["active_whatsapp_marketing"] = waMarketing
	ctx["pending_payments"] = pendingPayments
	return ctx
}

func copilotSolutionsV271() []map[string]string {
	return []map[string]string{
		{"id": "setup", "label": "Configuración inicial", "view": "dashboard", "prompt": "Haz un diagnóstico de mi configuración inicial y dime qué me falta para dejar WorkticAI listo para operar."},
		{"id": "diagnostic", "label": "Diagnóstico general", "view": "dashboard", "prompt": "Revisa mi configuración actual y detecta bloqueos, módulos incompletos y próximos pasos prioritarios."},
		{"id": "whatsapp", "label": "WhatsApp Business", "view": "whatsappbusiness", "prompt": "Guíame para configurar y diagnosticar WhatsApp Business Cloud, webhook, plantillas y conexión."},
		{"id": "wamarketing", "label": "WhatsApp Marketing", "view": "whatsappmarketing", "prompt": "Ayúdame a crear una campaña de WhatsApp Marketing con audiencia, consentimiento, plantilla, programación y métricas."},
		{"id": "inbox", "label": "Inbox omnicanal", "view": "inbox", "prompt": "Ayúdame a organizar y operar el Inbox omnicanal, asignaciones, prioridades y handoff entre IA y humano."},
		{"id": "crm", "label": "CRM y oportunidades", "view": "contacts", "prompt": "Ayúdame a organizar contactos, oportunidades, pipeline, seguimiento y cierre comercial."},
		{"id": "agents", "label": "Agentes IA", "view": "agents", "prompt": "Ayúdame a configurar, entrenar, enrutar y probar mis agentes IA según las funciones reales de WorkticAI."},
		{"id": "automation", "label": "Automatizaciones", "view": "rules", "prompt": "Ayúdame a diseñar o diagnosticar automatizaciones de seguimiento, CRM, agenda, mensajes y ventas."},
		{"id": "social", "label": "Social Hub", "view": "socialhub", "prompt": "Ayúdame a conectar y operar Social Hub, publicaciones, calendario, gobernanza y métricas."},
		{"id": "growth", "label": "Growth y formularios", "view": "marketing", "prompt": "Ayúdame con campañas Growth, formularios, leads, scoring y seguimiento comercial."},
		{"id": "landings", "label": "Landing Pages", "view": "landings", "prompt": "Ayúdame a crear, publicar y conectar Landing Pages con formularios, campañas y CRM."},
		{"id": "ads", "label": "Ads Center", "view": "adscenter", "prompt": "Ayúdame a conectar, sincronizar y analizar Meta Ads, Google Ads o TikTok Ads dentro de Ads Center."},
		{"id": "analytics", "label": "Analytics y atribución", "view": "analytics", "prompt": "Explícame y ayúdame a interpretar Analytics, atribución, revenue, CPL, CAC y ROAS con mis datos reales."},
		{"id": "agenda", "label": "Agenda", "view": "appointments", "prompt": "Ayúdame a configurar agenda, servicios, profesionales, disponibilidad y citas."},
		{"id": "catalog", "label": "Catálogo", "view": "products", "prompt": "Ayúdame a organizar productos y servicios para que agentes, campañas y CRM los utilicen correctamente."},
		{"id": "team", "label": "Equipo y roles", "view": "users", "prompt": "Ayúdame a configurar usuarios, roles, permisos y operación del equipo."},
		{"id": "billing", "label": "Plan y facturación", "view": "billing", "prompt": "Ayúdame a revisar plan, límites, pagos y facturación sin solicitar ni exponer datos sensibles."},
		{"id": "integrations", "label": "Integraciones y webhooks", "view": "channels", "prompt": "Ayúdame a diagnosticar integraciones, canales, webhooks, callbacks y permisos externos."},
		{"id": "troubleshoot", "label": "Resolver un error", "view": "setup", "prompt": "Tengo un problema técnico. Guíame para diagnosticarlo paso a paso usando la configuración y módulos reales de WorkticAI."},
	}
}

func copilotDeepLinksV27(message string) []map[string]string {
	t := strings.ToLower(message)
	links := []map[string]string{}
	add := func(view, label string) { links = append(links, map[string]string{"view": view, "label": label}) }
	switch {
	case strings.Contains(t, "whatsapp") && (strings.Contains(t, "campaña") || strings.Contains(t, "marketing") || strings.Contains(t, "masiv")):
		add("whatsappmarketing", "Abrir WhatsApp Marketing")
		add("whatsappbusiness", "Revisar WhatsApp Business")
	case strings.Contains(t, "whatsapp"):
		add("whatsappbusiness", "Abrir WhatsApp Business")
		add("channels", "Revisar canales")
	case strings.Contains(t, "inbox") || strings.Contains(t, "convers") || strings.Contains(t, "mensaje"):
		add("inbox", "Abrir Inbox")
	case strings.Contains(t, "automat") || strings.Contains(t, "workflow") || strings.Contains(t, "flujo"):
		add("rules", "Abrir Automatizaciones")
	case strings.Contains(t, "agente") || strings.Contains(t, "ia"):
		add("agents", "Abrir Agentes IA")
	case strings.Contains(t, "lead") || strings.Contains(t, "contact") || strings.Contains(t, "crm") || strings.Contains(t, "oportun") || strings.Contains(t, "pipeline"):
		add("contacts", "Abrir CRM")
		add("pipeline", "Abrir oportunidades")
	case strings.Contains(t, "anuncio") || strings.Contains(t, "ads") || strings.Contains(t, "roas") || strings.Contains(t, "google ads") || strings.Contains(t, "meta ads"):
		add("adscenter", "Abrir Ads Center")
		add("analytics", "Abrir Analytics")
	case strings.Contains(t, "instagram") || strings.Contains(t, "facebook") || strings.Contains(t, "tiktok") || strings.Contains(t, "linkedin") || strings.Contains(t, "youtube") || strings.Contains(t, "social"):
		add("socialhub", "Abrir Social Hub")
	case strings.Contains(t, "landing"):
		add("landings", "Abrir Landing Pages")
		add("marketing", "Abrir Growth")
	case strings.Contains(t, "formulario") || strings.Contains(t, "growth"):
		add("marketing", "Abrir Growth & Campañas")
	case strings.Contains(t, "cita") || strings.Contains(t, "agenda") || strings.Contains(t, "disponibilidad"):
		add("appointments", "Abrir Agenda")
	case strings.Contains(t, "producto") || strings.Contains(t, "catálogo") || strings.Contains(t, "catalogo") || strings.Contains(t, "servicio"):
		add("products", "Abrir Catálogo")
	case strings.Contains(t, "equipo") || strings.Contains(t, "usuario") || strings.Contains(t, "rol") || strings.Contains(t, "permiso"):
		add("users", "Abrir Equipo")
	case strings.Contains(t, "plan") || strings.Contains(t, "factur") || strings.Contains(t, "pago") || strings.Contains(t, "membres"):
		add("billing", "Abrir Plan y membresía")
	case strings.Contains(t, "webhook") || strings.Contains(t, "callback") || strings.Contains(t, "integración") || strings.Contains(t, "integracion") || strings.Contains(t, "api"):
		add("channels", "Abrir Canales")
		add("setup", "Abrir Guías")
	case strings.Contains(t, "analytics") || strings.Contains(t, "atribuci") || strings.Contains(t, "métrica") || strings.Contains(t, "metrica"):
		add("analytics", "Abrir Analytics")
	case strings.Contains(t, "grupo") || strings.Contains(t, "comunidad"):
		add("groups", "Abrir Grupos & Comunidades")
	default:
		add("dashboard", "Ir al resumen")
		add("setup", "Abrir Guías")
	}
	return links
}

func (a *App) callCopilotOpenAIV27(prompt string) (string, error) {
	// Reuse the same OpenAI path already proven by agents, automations and channels.
	// This avoids maintaining a second parser/client just for Copilot.
	return a.callOpenAI(
		"Eres Worktic Copilot, el asistente interno experto de WorkticAI. Responde en español claro, práctico y profesional. No inventes conexiones, métricas ni configuraciones. Nunca solicites secretos, tokens ni API keys.",
		prompt,
	)
}

func copilotFallbackV274(message string, ctx map[string]any) string {
	t := strings.ToLower(strings.TrimSpace(message))
	connected := fmt.Sprint(ctx["connected_channels"])
	agents := fmt.Sprint(ctx["active_agents"])
	workflows := fmt.Sprint(ctx["active_workflows"])
	unread := fmt.Sprint(ctx["unread_messages"])
	switch {
	case strings.Contains(t, "whatsapp"):
		return "Puedo seguir guiándote aunque la IA externa esté temporalmente ocupada. Revisa primero WhatsApp Business → conexión oficial, número, webhook y plantillas aprobadas. En tu espacio aparecen " + connected + " canales conectados. Para campañas usa WhatsApp Marketing y confirma consentimiento antes del envío."
	case strings.Contains(t, "automat") || strings.Contains(t, "workflow"):
		return "Tu espacio registra " + workflows + " automatizaciones activas. Abre Automatizaciones y valida disparador, condiciones, acción y estado activo. Si me indicas qué proceso quieres automatizar, puedo ayudarte a estructurarlo paso a paso."
	case strings.Contains(t, "agente") || strings.Contains(t, " ia"):
		return "Actualmente se detectan " + agents + " agentes IA activos. Revisa Agentes IA → instrucciones, permisos, enrutamiento y simulador. Después prueba una conversación real desde el canal correspondiente."
	case strings.Contains(t, "mensaje") || strings.Contains(t, "inbox") || strings.Contains(t, "convers"):
		return "El Inbox muestra " + unread + " mensajes pendientes. Revisa Conversaciones para atenderlos, validar asignación y confirmar si la respuesta debe quedar en IA o pasar a un asesor humano."
	default:
		return "Puedo ayudarte con toda la plataforma. El servicio de IA tardó más de lo esperado, pero el diagnóstico interno sigue disponible. Indícame el módulo o problema concreto —por ejemplo CRM, WhatsApp, Social Hub, Automatizaciones, Ads, Analytics o Agenda— y te guío con la configuración real de WorkticAI."
	}
}

type copilotQueryV275 struct {
	Message string `json:"message"`
	View    string `json:"view"`
	History []struct {
		Role string `json:"role"`
		Text string `json:"text"`
	} `json:"history"`
}

func decodeCopilotQueryV275(r *http.Request) (copilotQueryV275, error) {
	var q copilotQueryV275
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&q); err != nil {
		return q, errors.New("solicitud inválida")
	}
	q.Message = strings.TrimSpace(q.Message)
	if q.Message == "" {
		return q, errors.New("escribe una pregunta")
	}
	if len(q.History) > 8 {
		q.History = q.History[len(q.History)-8:]
	}
	return q, nil
}

func (a *App) copilotPromptV275(tid int64, u *User, q copilotQueryV275) (string, map[string]any) {
	ctx := a.copilotContextV27(tid)
	ctxRaw, _ := json.Marshal(ctx)
	hist := []string{}
	for _, h := range q.History {
		text := strings.TrimSpace(h.Text)
		if len(text) > 700 {
			text = text[:700]
		}
		hist = append(hist, h.Role+": "+text)
	}
	prompt := `Eres Worktic Copilot, el asistente interno experto de WorkticAI. Ayudas al usuario a configurar y operar la plataforma de manera práctica, segura y orientada a resultados comerciales.

Conoces en detalle estos módulos reales de WorkticAI V27: Dashboard Intelligence; Inbox Omnicanal; CRM Contactos y Oportunidades; Catálogo; Agenda; Agentes IA y enrutamiento; Canales; WhatsApp Business Cloud, Embedded Signup, plantillas y webhooks; WhatsApp Marketing con consentimiento; Automatizaciones visuales; Social Hub, calendario y gobernanza; Growth, campañas, formularios, leads y scoring; Landing Pages; Grupos y Comunidades; Analytics y Attribution; Ads Center para Meta/TikTok/Google; Equipo y roles; Perfil; Planes, membresías, pagos y Administración Worktic. También conoces los flujos de integración entre estos módulos.

Tu función cubre onboarding, soporte, diagnóstico, explicación, configuración, optimización y guía paso a paso de toda la plataforma. Cuando el usuario pregunte algo ambiguo, relaciona la respuesta con el módulo correcto y termina con el siguiente paso concreto. Reglas: no inventes que una integración está conectada si el contexto dice lo contrario; no inventes métricas; para WhatsApp Marketing exige consentimiento y plantillas aprobadas; para acciones externas sensibles indica el paso exacto y que el usuario confirme en la interfaz; nunca pidas ni muestres secretos, tokens o API keys. Responde con párrafos breves y pasos concretos. Evita introducciones largas.

Usuario: ` + u.Name + `
Empresa: ` + u.Company + `
Vista actual: ` + firstNonEmpty(q.View, "dashboard") + `
Diagnóstico actual (solo conteos, sin PII): ` + string(ctxRaw) + `
Historial reciente:
` + strings.Join(hist, "\n") + `

Pregunta actual: ` + q.Message
	return prompt, ctx
}

func (a *App) streamCopilotOpenAIV275(ctx context.Context, prompt string, emit func(string) error) error {
	payload := map[string]any{
		"model":  a.cfg.OpenAIModel,
		"input":  "Eres Worktic Copilot, el asistente interno experto de WorkticAI. Responde en español claro, práctico y profesional. No inventes conexiones, métricas ni configuraciones. Nunca solicites secretos, tokens ni API keys.\n\nMensaje del usuario: " + prompt,
		"store":  false,
		"stream": true,
	}
	b, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/responses", bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+a.openAIKey())
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	resp, err := (&http.Client{Timeout: 65 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return fmt.Errorf("OpenAI: %s", strings.TrimSpace(string(raw)))
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
	emitted := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		var ev struct {
			Type  string `json:"type"`
			Delta string `json:"delta"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal([]byte(data), &ev) != nil {
			continue
		}
		switch ev.Type {
		case "response.output_text.delta":
			if ev.Delta != "" {
				emitted = true
				if err := emit(ev.Delta); err != nil {
					return err
				}
			}
		case "error", "response.failed":
			if ev.Error != nil && ev.Error.Message != "" {
				return errors.New(ev.Error.Message)
			}
			return errors.New("OpenAI no pudo completar la respuesta")
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if !emitted {
		return errors.New("OpenAI no devolvió texto en streaming")
	}
	return nil
}

func (a *App) copilotV27Handler(w http.ResponseWriter, r *http.Request) {
	tid, u, err := a.tenantFor(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	if r.Method == http.MethodGet {
		writeJSON(w, map[string]any{"ok": true, "context": a.copilotContextV27(tid), "suggestions": []string{"Revisa mi configuración", "¿Qué me falta para automatizar ventas?", "Ayúdame a resolver un error", "Optimiza mi operación"}, "solutions": copilotSolutionsV271()})
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	q, err := decodeCopilotQueryV275(r)
	if err != nil {
		writeError(w, err, 400)
		return
	}
	prompt, ctx := a.copilotPromptV275(tid, u, q)
	answer, aiErr := a.callCopilotOpenAIV27(prompt)
	degraded := false
	if aiErr != nil || strings.TrimSpace(answer) == "" {
		degraded = true
		answer = copilotFallbackV274(q.Message, ctx)
	}
	writeJSON(w, map[string]any{"ok": true, "answer": answer, "actions": copilotDeepLinksV27(q.Message), "context": ctx, "degraded": degraded})
}

func (a *App) copilotV275StreamHandler(w http.ResponseWriter, r *http.Request) {
	tid, u, err := a.tenantFor(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	q, err := decodeCopilotQueryV275(r)
	if err != nil {
		writeError(w, err, 400)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, errors.New("streaming no disponible"), 500)
		return
	}
	w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	writeEvent := func(v any) error {
		b, _ := json.Marshal(v)
		if _, err := w.Write(append(b, '\n')); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}

	prompt, ctx := a.copilotPromptV275(tid, u, q)
	_ = writeEvent(map[string]any{"type": "start", "context": ctx})
	parts := strings.Builder{}
	streamErr := a.streamCopilotOpenAIV275(r.Context(), prompt, func(delta string) error {
		parts.WriteString(delta)
		return writeEvent(map[string]any{"type": "delta", "delta": delta})
	})
	degraded := false
	if streamErr != nil && parts.Len() == 0 {
		degraded = true
		fallback := copilotFallbackV274(q.Message, ctx)
		parts.WriteString(fallback)
		_ = writeEvent(map[string]any{"type": "delta", "delta": fallback})
	}
	_ = writeEvent(map[string]any{
		"type":     "done",
		"answer":   strings.TrimSpace(parts.String()),
		"actions":  copilotDeepLinksV27(q.Message),
		"context":  ctx,
		"degraded": degraded,
	})
}
