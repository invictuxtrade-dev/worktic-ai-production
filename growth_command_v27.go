package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
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

	var contacts, openOpps, unread, activeAgents, activeAutomations, connectedChannels, waCampaigns int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM crm_contacts WHERE tenant_id=? AND COALESCE(deleted_at,'')=''`, tid).Scan(&contacts)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM crm_opportunities WHERE tenant_id=? AND COALESCE(deleted_at,'')='' AND stage NOT IN ('Ganado','Perdido')`, tid).Scan(&openOpps)
	_ = a.db.QueryRow(`SELECT COALESCE(SUM(unread),0) FROM worktic_contacts WHERE tenant_id=?`, tid).Scan(&unread)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM ai_agents WHERE tenant_id=? AND status='active'`, tid).Scan(&activeAgents)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM automation_workflows WHERE tenant_id=? AND status='active'`, tid).Scan(&activeAutomations)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM channel_connections WHERE tenant_id=? AND status='connected'`, tid).Scan(&connectedChannels)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM whatsapp_marketing_campaigns_v27 WHERE tenant_id=? AND status IN ('running','scheduled')`, tid).Scan(&waCampaigns)

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

	activity := []map[string]any{}
	if rows, err := a.db.Query(`SELECT event_type,channel,source,value,occurred_at FROM analytics_attribution_events_v24 WHERE tenant_id=? ORDER BY occurred_at DESC,id DESC LIMIT 12`, tid); err == nil {
		defer rows.Close()
		for rows.Next() {
			var eventType, channel, source, occurred string
			var value float64
			if rows.Scan(&eventType, &channel, &source, &value, &occurred) == nil {
				activity = append(activity, map[string]any{"event_type": eventType, "channel": channel, "source": source, "value": value, "occurred_at": occurred})
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
		"operations": map[string]any{"connected_channels": connectedChannels, "active_agents": activeAgents, "active_automations": activeAutomations, "active_whatsapp_campaigns": waCampaigns},
		"series":     series, "sources": sources, "insights": insights, "activity": activity,
	})
}

func (a *App) copilotContextV27(tid int64) map[string]any {
	ctx := map[string]any{}
	var connected, agents, workflows, products, contacts, leads, openOpps, waMarketing, pendingPayments int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM channel_connections WHERE tenant_id=? AND status='connected'`, tid).Scan(&connected)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM ai_agents WHERE tenant_id=? AND status='active'`, tid).Scan(&agents)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM automation_workflows WHERE tenant_id=? AND status='active'`, tid).Scan(&workflows)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM crm_products WHERE tenant_id=? AND active=1`, tid).Scan(&products)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM crm_contacts WHERE tenant_id=? AND COALESCE(deleted_at,'')=''`, tid).Scan(&contacts)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM marketing_leads WHERE tenant_id=?`, tid).Scan(&leads)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM crm_opportunities WHERE tenant_id=? AND COALESCE(deleted_at,'')='' AND stage NOT IN ('Ganado','Perdido')`, tid).Scan(&openOpps)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM whatsapp_marketing_campaigns_v27 WHERE tenant_id=? AND status IN ('running','scheduled')`, tid).Scan(&waMarketing)
	var ownerID int64
	_ = a.db.QueryRow(`SELECT owner_user_id FROM tenants WHERE id=?`, tid).Scan(&ownerID)
	if ownerID > 0 {
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM billing_payments WHERE user_id=? AND status='pending'`, ownerID).Scan(&pendingPayments)
	}
	ctx["connected_channels"] = connected
	ctx["active_agents"] = agents
	ctx["active_workflows"] = workflows
	ctx["products"] = products
	ctx["contacts"] = contacts
	ctx["leads"] = leads
	ctx["open_opportunities"] = openOpps
	ctx["active_whatsapp_marketing"] = waMarketing
	ctx["pending_payments"] = pendingPayments
	return ctx
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
		add("channels", "Revisar conexión")
	case strings.Contains(t, "automat"):
		add("rules", "Abrir Automatizaciones")
	case strings.Contains(t, "agente") || strings.Contains(t, "ia"):
		add("agents", "Abrir Agentes IA")
	case strings.Contains(t, "lead") || strings.Contains(t, "contact") || strings.Contains(t, "crm"):
		add("contacts", "Abrir CRM")
		add("pipeline", "Abrir oportunidades")
	case strings.Contains(t, "anuncio") || strings.Contains(t, "ads") || strings.Contains(t, "roas"):
		add("adscenter", "Abrir Ads Center")
		add("analytics", "Abrir Analytics")
	case strings.Contains(t, "instagram") || strings.Contains(t, "facebook") || strings.Contains(t, "tiktok") || strings.Contains(t, "social"):
		add("socialhub", "Abrir Social Hub")
	case strings.Contains(t, "cita") || strings.Contains(t, "agenda"):
		add("appointments", "Abrir Agenda")
	default:
		add("dashboard", "Ir al resumen")
	}
	return links
}

func (a *App) callCopilotOpenAIV27(prompt string) (string, error) {
	key := a.openAIKey()
	if key == "" {
		return "", errors.New("OPENAI_API_KEY no está configurada")
	}
	payload := map[string]any{"model": a.cfg.OpenAIModel, "input": prompt, "store": false}
	b, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, "https://api.openai.com/v1/responses", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("OpenAI HTTP %d", resp.StatusCode)
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
		return "", errors.New("respuesta de IA inválida")
	}
	for _, o := range x.Output {
		for _, c := range o.Content {
			if c.Type == "output_text" && strings.TrimSpace(c.Text) != "" {
				return strings.TrimSpace(c.Text), nil
			}
		}
	}
	return "", errors.New("la IA no devolvió respuesta")
}

func (a *App) copilotV27Handler(w http.ResponseWriter, r *http.Request) {
	tid, u, err := a.tenantFor(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	if r.Method == http.MethodGet {
		writeJSON(w, map[string]any{"ok": true, "context": a.copilotContextV27(tid), "suggestions": []string{"Ayúdame a configurar WhatsApp", "¿Qué me falta para automatizar ventas?", "Revisa mi configuración", "¿Cómo creo una campaña?"}})
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	var q struct {
		Message string `json:"message"`
		View    string `json:"view"`
		History []struct {
			Role string `json:"role"`
			Text string `json:"text"`
		} `json:"history"`
	}
	if json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&q) != nil || strings.TrimSpace(q.Message) == "" {
		writeError(w, errors.New("escribe una pregunta"), 400)
		return
	}
	if len(q.History) > 8 {
		q.History = q.History[len(q.History)-8:]
	}
	ctxRaw, _ := json.Marshal(a.copilotContextV27(tid))
	hist := []string{}
	for _, h := range q.History {
		text := strings.TrimSpace(h.Text)
		if len(text) > 700 {
			text = text[:700]
		}
		hist = append(hist, h.Role+": "+text)
	}
	prompt := `Eres Worktic Copilot, el asistente interno experto de WorkticAI. Ayudas al usuario a configurar y operar la plataforma de manera práctica, segura y orientada a resultados comerciales.

Conoces estos módulos reales de WorkticAI V27: Dashboard Intelligence, CRM Contactos, Oportunidades, Inbox Omnicanal, Catálogo, Agenda, Agentes IA, Canales, WhatsApp Business Cloud, WhatsApp Marketing, Automatizaciones visuales, Social Hub, Growth y formularios, Landing Pages, Analytics/Attribution, Ads Center, Equipos, Planes y Administración.

Reglas: no inventes que una integración está conectada si el contexto dice lo contrario; no inventes métricas; para WhatsApp Marketing exige consentimiento y plantillas aprobadas; para acciones externas sensibles indica el paso exacto y que el usuario confirme en la interfaz; nunca pidas ni muestres secretos, tokens o API keys. Da instrucciones breves, numeradas solo cuando realmente ayuden. Si detectas una configuración faltante, dilo claramente y nombra el módulo exacto donde se corrige.

Usuario: ` + u.Name + `
Empresa: ` + u.Company + `
Vista actual: ` + firstNonEmpty(q.View, "dashboard") + `
Diagnóstico actual (solo conteos, sin PII): ` + string(ctxRaw) + `
Historial reciente:
` + strings.Join(hist, "\n") + `

Pregunta actual: ` + strings.TrimSpace(q.Message)
	answer, err := a.callCopilotOpenAIV27(prompt)
	if err != nil {
		writeError(w, err, 502)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "answer": answer, "actions": copilotDeepLinksV27(q.Message), "context": a.copilotContextV27(tid)})
}
