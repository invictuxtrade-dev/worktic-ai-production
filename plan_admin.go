package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

func (a *App) adminPlansHandler(w http.ResponseWriter, r *http.Request) {
	u := a.currentUser(r)
	if u == nil || u.Role != "superadmin" {
		writeError(w, errors.New("solo superadministrador"), 403)
		return
	}
	for _, stmt := range []string{`ALTER TABLE billing_plans ADD COLUMN features_json TEXT NOT NULL DEFAULT '{}'`, `ALTER TABLE billing_plans ADD COLUMN max_whatsapp INTEGER NOT NULL DEFAULT 1`, `ALTER TABLE billing_plans ADD COLUMN max_telegram INTEGER NOT NULL DEFAULT 1`, `ALTER TABLE billing_plans ADD COLUMN max_messenger INTEGER NOT NULL DEFAULT 0`, `ALTER TABLE billing_plans ADD COLUMN max_agents INTEGER NOT NULL DEFAULT 1`} {
		_, _ = a.db.Exec(stmt)
	}
	if r.Method == http.MethodGet {
		rows, e := a.db.Query(`SELECT id,code,name,description,price_usdt,billing_days,max_users,max_channels,max_contacts,max_ai_responses,max_products,max_rules,max_whatsapp,max_telegram,max_messenger,max_agents,features_json,active FROM billing_plans ORDER BY price_usdt,id`)
		if e != nil {
			writeError(w, e, 500)
			return
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var id int64
			var code, name, desc, features string
			var price float64
			var days, users, channels, contacts, ai, products, rules, wa, tg, msg, agents, active int
			_ = rows.Scan(&id, &code, &name, &desc, &price, &days, &users, &channels, &contacts, &ai, &products, &rules, &wa, &tg, &msg, &agents, &features, &active)
			out = append(out, map[string]any{"id": id, "code": code, "name": name, "description": desc, "price_usdt": price, "billing_days": days, "max_users": users, "max_channels": channels, "max_contacts": contacts, "max_ai_responses": ai, "max_products": products, "max_rules": rules, "max_whatsapp": wa, "max_telegram": tg, "max_messenger": msg, "max_agents": agents, "features_json": features, "active": active == 1})
		}
		writeJSON(w, out)
		return
	}
	var q struct {
		ID             int64   `json:"id"`
		Code           string  `json:"code"`
		Name           string  `json:"name"`
		Description    string  `json:"description"`
		FeaturesJSON   string  `json:"features_json"`
		PriceUSDT      float64 `json:"price_usdt"`
		BillingDays    int     `json:"billing_days"`
		MaxUsers       int     `json:"max_users"`
		MaxChannels    int     `json:"max_channels"`
		MaxContacts    int     `json:"max_contacts"`
		MaxAIResponses int     `json:"max_ai_responses"`
		MaxProducts    int     `json:"max_products"`
		MaxRules       int     `json:"max_rules"`
		MaxWhatsapp    int     `json:"max_whatsapp"`
		MaxTelegram    int     `json:"max_telegram"`
		MaxMessenger   int     `json:"max_messenger"`
		MaxAgents      int     `json:"max_agents"`
		Active         bool    `json:"active"`
	}
	_ = json.NewDecoder(r.Body).Decode(&q)
	q.Code = strings.ToLower(strings.TrimSpace(q.Code))
	if q.Code == "" || q.Name == "" {
		writeError(w, errors.New("código y nombre obligatorios"), 400)
		return
	}
	if q.BillingDays < 1 {
		q.BillingDays = 30
	}
	if q.FeaturesJSON == "" {
		q.FeaturesJSON = "{}"
	}
	active := 0
	if q.Active {
		active = 1
	}
	if r.Method == http.MethodPost {
		_, e := a.db.Exec(`INSERT INTO billing_plans(code,name,description,price_usdt,billing_days,max_users,max_channels,max_contacts,max_ai_responses,max_products,max_rules,max_whatsapp,max_telegram,max_messenger,max_agents,features_json,active) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, q.Code, q.Name, q.Description, q.PriceUSDT, q.BillingDays, q.MaxUsers, q.MaxChannels, q.MaxContacts, q.MaxAIResponses, q.MaxProducts, q.MaxRules, q.MaxWhatsapp, q.MaxTelegram, q.MaxMessenger, q.MaxAgents, q.FeaturesJSON, active)
		if e != nil {
			writeError(w, e, 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
		return
	}
	if r.Method == http.MethodPut {
		_, e := a.db.Exec(`UPDATE billing_plans SET code=?,name=?,description=?,price_usdt=?,billing_days=?,max_users=?,max_channels=?,max_contacts=?,max_ai_responses=?,max_products=?,max_rules=?,max_whatsapp=?,max_telegram=?,max_messenger=?,max_agents=?,features_json=?,active=? WHERE id=?`, q.Code, q.Name, q.Description, q.PriceUSDT, q.BillingDays, q.MaxUsers, q.MaxChannels, q.MaxContacts, q.MaxAIResponses, q.MaxProducts, q.MaxRules, q.MaxWhatsapp, q.MaxTelegram, q.MaxMessenger, q.MaxAgents, q.FeaturesJSON, active, q.ID)
		if e != nil {
			writeError(w, e, 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
		return
	}
	if r.Method == http.MethodDelete {
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		_, e := a.db.Exec(`UPDATE billing_plans SET active=0 WHERE id=?`, id)
		if e != nil {
			writeError(w, e, 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
		return
	}
	http.Error(w, "Método no permitido", 405)
}
