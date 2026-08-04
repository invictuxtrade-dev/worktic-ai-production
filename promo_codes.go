package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type PromoCode struct {
	ID              int64   `json:"id"`
	Code            string  `json:"code"`
	DiscountPercent float64 `json:"discount_percent"`
	StartsAt        string  `json:"starts_at"`
	ExpiresAt       string  `json:"expires_at"`
	Active          bool    `json:"active"`
	Uses            int     `json:"uses"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
}

func normalizePromoCode(v string) string { return strings.ToUpper(strings.TrimSpace(v)) }

func (a *App) promoCodeForCheckout(code string, plan Plan) (*PromoCode, float64, error) {
	code = normalizePromoCode(code)
	if code == "" {
		return nil, plan.PriceUSDT, nil
	}
	var p PromoCode
	var active int
	err := a.db.QueryRow(`SELECT id,code,discount_percent,starts_at,expires_at,active,uses,created_at,updated_at FROM billing_promo_codes WHERE code=?`, code).Scan(&p.ID, &p.Code, &p.DiscountPercent, &p.StartsAt, &p.ExpiresAt, &active, &p.Uses, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, 0, errors.New("código promocional inválido")
	}
	if err != nil {
		return nil, 0, err
	}
	p.Active = active == 1
	now := time.Now().UTC()
	if !p.Active {
		return nil, 0, errors.New("este código promocional está inactivo")
	}
	if p.StartsAt != "" {
		if t, e := time.Parse(time.RFC3339, p.StartsAt); e == nil && now.Before(t) {
			return nil, 0, errors.New("este código promocional todavía no está vigente")
		}
	}
	if p.ExpiresAt != "" {
		if t, e := time.Parse(time.RFC3339, p.ExpiresAt); e == nil && now.After(t) {
			return nil, 0, errors.New("este código promocional ya venció")
		}
	}
	if p.DiscountPercent <= 0 || p.DiscountPercent > 100 {
		return nil, 0, errors.New("el descuento configurado no es válido")
	}
	final := plan.PriceUSDT * (1 - p.DiscountPercent/100)
	if final < 0 {
		final = 0
	}
	final = float64(int64(final*100+0.5)) / 100
	return &p, final, nil
}

func (a *App) promoValidateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	if a.currentUser(r) == nil {
		writeError(w, errors.New("sesión requerida"), 401)
		return
	}
	var q struct {
		Code     string `json:"code"`
		PlanCode string `json:"plan_code"`
	}
	if json.NewDecoder(r.Body).Decode(&q) != nil {
		writeError(w, errors.New("datos inválidos"), 400)
		return
	}
	var plan Plan
	var active int
	if a.db.QueryRow(`SELECT id,code,name,description,price_usdt,billing_days,max_users,max_channels,max_contacts,max_ai_responses,max_products,max_rules,active FROM billing_plans WHERE code=? AND active=1`, strings.ToLower(strings.TrimSpace(q.PlanCode))).Scan(&plan.ID, &plan.Code, &plan.Name, &plan.Description, &plan.PriceUSDT, &plan.BillingDays, &plan.MaxUsers, &plan.MaxChannels, &plan.MaxContacts, &plan.MaxAIResponses, &plan.MaxProducts, &plan.MaxRules, &active) != nil {
		writeError(w, errors.New("plan inválido"), 400)
		return
	}
	promo, final, err := a.promoCodeForCheckout(q.Code, plan)
	if err != nil {
		writeError(w, err, 400)
		return
	}
	if promo == nil {
		writeError(w, errors.New("ingresa un código promocional"), 400)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "code": promo.Code, "discount_percent": promo.DiscountPercent, "original_amount": plan.PriceUSDT, "final_amount": final, "expires_at": promo.ExpiresAt})
}

func (a *App) adminPromoCodesHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireSuperadmin(w, r); !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		rows, err := a.db.Query(`SELECT id,code,discount_percent,starts_at,expires_at,active,uses,created_at,updated_at FROM billing_promo_codes ORDER BY id DESC`)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		defer rows.Close()
		out := []PromoCode{}
		for rows.Next() {
			var p PromoCode
			var active int
			if rows.Scan(&p.ID, &p.Code, &p.DiscountPercent, &p.StartsAt, &p.ExpiresAt, &active, &p.Uses, &p.CreatedAt, &p.UpdatedAt) == nil {
				p.Active = active == 1
				out = append(out, p)
			}
		}
		writeJSON(w, out)
	case http.MethodPost, http.MethodPut:
		var q PromoCode
		if json.NewDecoder(r.Body).Decode(&q) != nil {
			writeError(w, errors.New("datos inválidos"), 400)
			return
		}
		q.Code = normalizePromoCode(q.Code)
		if len(q.Code) < 3 || strings.ContainsAny(q.Code, " \t\n") {
			writeError(w, errors.New("el código debe tener mínimo 3 caracteres y no contener espacios"), 400)
			return
		}
		if q.DiscountPercent <= 0 || q.DiscountPercent > 100 {
			writeError(w, errors.New("el descuento debe estar entre 0.01% y 100%"), 400)
			return
		}
		if q.ExpiresAt == "" {
			writeError(w, errors.New("la fecha de vencimiento es obligatoria"), 400)
			return
		}
		if _, err := time.Parse(time.RFC3339, q.ExpiresAt); err != nil {
			writeError(w, errors.New("fecha de vencimiento inválida"), 400)
			return
		}
		if q.StartsAt == "" {
			q.StartsAt = time.Now().UTC().Format(time.RFC3339)
		}
		active := 0
		if q.Active {
			active = 1
		}
		now := time.Now().UTC().Format(time.RFC3339)
		var err error
		if r.Method == http.MethodPost {
			_, err = a.db.Exec(`INSERT INTO billing_promo_codes(code,discount_percent,starts_at,expires_at,active,uses,created_at,updated_at) VALUES(?,?,?,?,?,0,?,?)`, q.Code, q.DiscountPercent, q.StartsAt, q.ExpiresAt, active, now, now)
		} else {
			if q.ID == 0 {
				writeError(w, errors.New("id obligatorio"), 400)
				return
			}
			_, err = a.db.Exec(`UPDATE billing_promo_codes SET code=?,discount_percent=?,starts_at=?,expires_at=?,active=?,updated_at=? WHERE id=?`, q.Code, q.DiscountPercent, q.StartsAt, q.ExpiresAt, active, now, q.ID)
		}
		if err != nil {
			writeError(w, fmt.Errorf("no fue posible guardar el código; verifica que no esté repetido"), 400)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	case http.MethodDelete:
		id := strings.TrimSpace(r.URL.Query().Get("id"))
		if id == "" {
			writeError(w, errors.New("id obligatorio"), 400)
			return
		}
		if _, err := a.db.Exec(`DELETE FROM billing_promo_codes WHERE id=?`, id); err != nil {
			writeError(w, err, 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	default:
		http.Error(w, "Método no permitido", 405)
	}
}
