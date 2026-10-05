package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

// ============================================================
// BILLING PLANS - COMPATIBILIDAD SQLITE / POSTGRESQL V26
// ============================================================
//
// Esta función asegura que instalaciones antiguas tengan todas
// las columnas requeridas actualmente por el módulo de planes.
//
// Los errores de "columna ya existe" se ignoran intencionalmente,
// porque debe poder ejecutarse más de una vez sin afectar producción.
func (a *App) ensureBillingPlansSchema() error {
	stmts := []string{
		`ALTER TABLE billing_plans ADD COLUMN features_json TEXT NOT NULL DEFAULT '{}'`,
		`ALTER TABLE billing_plans ADD COLUMN max_whatsapp INTEGER NOT NULL DEFAULT 1`,
		`ALTER TABLE billing_plans ADD COLUMN max_telegram INTEGER NOT NULL DEFAULT 1`,
		`ALTER TABLE billing_plans ADD COLUMN max_messenger INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE billing_plans ADD COLUMN max_agents INTEGER NOT NULL DEFAULT 1`,
	}

	for _, stmt := range stmts {
		_, _ = a.db.Exec(stmt)
	}

	return nil
}

// ============================================================
// ADMINISTRACIÓN DE PLANES
// ============================================================

func (a *App) adminPlansHandler(w http.ResponseWriter, r *http.Request) {
	u := a.currentUser(r)

	if u == nil || u.Role != "superadmin" {
		writeError(w, errors.New("solo superadministrador"), http.StatusForbidden)
		return
	}

	// Asegurar esquema antes de utilizar billing_plans.
	if err := a.ensureBillingPlansSchema(); err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}

	switch r.Method {

	// ========================================================
	// GET - LISTAR PLANES
	// ========================================================
	case http.MethodGet:
		rows, err := a.db.Query(`
			SELECT
				id,
				code,
				name,
				description,
				price_usdt,
				billing_days,
				max_users,
				max_channels,
				max_contacts,
				max_ai_responses,
				max_products,
				max_rules,
				max_whatsapp,
				max_telegram,
				max_messenger,
				max_agents,
				features_json,
				active
			FROM billing_plans
			ORDER BY price_usdt, id
		`)
		if err != nil {
			writeError(w, err, http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		out := []map[string]any{}

		for rows.Next() {
			var (
				id       int64
				code     string
				name     string
				desc     string
				features string
				price    float64

				days     int
				users    int
				channels int
				contacts int
				ai       int
				products int
				rules    int
				wa       int
				tg       int
				msg      int
				agents   int
				active   int
			)

			if err := rows.Scan(
				&id,
				&code,
				&name,
				&desc,
				&price,
				&days,
				&users,
				&channels,
				&contacts,
				&ai,
				&products,
				&rules,
				&wa,
				&tg,
				&msg,
				&agents,
				&features,
				&active,
			); err != nil {
				continue
			}

			out = append(out, map[string]any{
				"id":               id,
				"code":             code,
				"name":             name,
				"description":      desc,
				"price_usdt":       price,
				"billing_days":     days,
				"max_users":        users,
				"max_channels":     channels,
				"max_contacts":     contacts,
				"max_ai_responses": ai,
				"max_products":     products,
				"max_rules":        rules,
				"max_whatsapp":     wa,
				"max_telegram":     tg,
				"max_messenger":    msg,
				"max_agents":       agents,
				"features_json":    features,
				"active":           active == 1,
			})
		}

		if err := rows.Err(); err != nil {
			writeError(w, err, http.StatusInternalServerError)
			return
		}

		writeJSON(w, out)
		return

	// ========================================================
	// POST / PUT
	// ========================================================
	case http.MethodPost, http.MethodPut:

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

		if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
			writeError(w, errors.New("datos inválidos"), http.StatusBadRequest)
			return
		}

		q.Code = strings.ToLower(strings.TrimSpace(q.Code))
		q.Name = strings.TrimSpace(q.Name)
		q.Description = strings.TrimSpace(q.Description)
		q.FeaturesJSON = strings.TrimSpace(q.FeaturesJSON)

		if q.Code == "" || q.Name == "" {
			writeError(
				w,
				errors.New("código y nombre obligatorios"),
				http.StatusBadRequest,
			)
			return
		}

		if q.BillingDays < 1 {
			q.BillingDays = 30
		}

		// Evitar números negativos.
		if q.MaxUsers < 0 {
			q.MaxUsers = 0
		}
		if q.MaxChannels < 0 {
			q.MaxChannels = 0
		}
		if q.MaxContacts < 0 {
			q.MaxContacts = 0
		}
		if q.MaxAIResponses < 0 {
			q.MaxAIResponses = 0
		}
		if q.MaxProducts < 0 {
			q.MaxProducts = 0
		}
		if q.MaxRules < 0 {
			q.MaxRules = 0
		}
		if q.MaxWhatsapp < 0 {
			q.MaxWhatsapp = 0
		}
		if q.MaxTelegram < 0 {
			q.MaxTelegram = 0
		}
		if q.MaxMessenger < 0 {
			q.MaxMessenger = 0
		}
		if q.MaxAgents < 0 {
			q.MaxAgents = 0
		}

		if q.PriceUSDT < 0 {
			q.PriceUSDT = 0
		}

		if q.FeaturesJSON == "" {
			q.FeaturesJSON = "{}"
		}

		// features_json debe contener JSON válido.
		if !json.Valid([]byte(q.FeaturesJSON)) {
			writeError(
				w,
				errors.New("features_json debe contener JSON válido"),
				http.StatusBadRequest,
			)
			return
		}

		active := 0
		if q.Active {
			active = 1
		}

		// ====================================================
		// CREAR PLAN
		// ====================================================
		if r.Method == http.MethodPost {
			_, err := a.db.Exec(`
				INSERT INTO billing_plans(
					code,
					name,
					description,
					price_usdt,
					billing_days,
					max_users,
					max_channels,
					max_contacts,
					max_ai_responses,
					max_products,
					max_rules,
					max_whatsapp,
					max_telegram,
					max_messenger,
					max_agents,
					features_json,
					active
				)
				VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
			`,
				q.Code,
				q.Name,
				q.Description,
				q.PriceUSDT,
				q.BillingDays,
				q.MaxUsers,
				q.MaxChannels,
				q.MaxContacts,
				q.MaxAIResponses,
				q.MaxProducts,
				q.MaxRules,
				q.MaxWhatsapp,
				q.MaxTelegram,
				q.MaxMessenger,
				q.MaxAgents,
				q.FeaturesJSON,
				active,
			)

			if err != nil {
				writeError(w, err, http.StatusInternalServerError)
				return
			}

			writeJSON(w, map[string]any{
				"ok": true,
			})
			return
		}

		// ====================================================
		// ACTUALIZAR PLAN
		// ====================================================
		if q.ID <= 0 {
			writeError(
				w,
				errors.New("id del plan obligatorio"),
				http.StatusBadRequest,
			)
			return
		}

		_, err := a.db.Exec(`
			UPDATE billing_plans
			SET
				code=?,
				name=?,
				description=?,
				price_usdt=?,
				billing_days=?,
				max_users=?,
				max_channels=?,
				max_contacts=?,
				max_ai_responses=?,
				max_products=?,
				max_rules=?,
				max_whatsapp=?,
				max_telegram=?,
				max_messenger=?,
				max_agents=?,
				features_json=?,
				active=?
			WHERE id=?
		`,
			q.Code,
			q.Name,
			q.Description,
			q.PriceUSDT,
			q.BillingDays,
			q.MaxUsers,
			q.MaxChannels,
			q.MaxContacts,
			q.MaxAIResponses,
			q.MaxProducts,
			q.MaxRules,
			q.MaxWhatsapp,
			q.MaxTelegram,
			q.MaxMessenger,
			q.MaxAgents,
			q.FeaturesJSON,
			active,
			q.ID,
		)

		if err != nil {
			writeError(w, err, http.StatusInternalServerError)
			return
		}

		writeJSON(w, map[string]any{
			"ok": true,
		})
		return

	// ========================================================
	// DELETE - DESACTIVAR PLAN
	// ========================================================
	case http.MethodDelete:

		id, err := strconv.ParseInt(
			strings.TrimSpace(r.URL.Query().Get("id")),
			10,
			64,
		)

		if err != nil || id <= 0 {
			writeError(
				w,
				errors.New("id del plan inválido"),
				http.StatusBadRequest,
			)
			return
		}

		var code string

		if err := a.db.QueryRow(
			`SELECT code FROM billing_plans WHERE id=?`,
			id,
		).Scan(&code); err != nil {
			writeError(
				w,
				errors.New("plan no encontrado"),
				http.StatusNotFound,
			)
			return
		}

		// Protección del plan Free.
		if strings.EqualFold(code, "free") {
			writeError(
				w,
				errors.New("el plan Free no puede desactivarse"),
				http.StatusBadRequest,
			)
			return
		}

		_, err = a.db.Exec(
			`UPDATE billing_plans SET active=0 WHERE id=?`,
			id,
		)

		if err != nil {
			writeError(w, err, http.StatusInternalServerError)
			return
		}

		writeJSON(w, map[string]any{
			"ok": true,
		})
		return

	default:
		http.Error(
			w,
			"Método no permitido",
			http.StatusMethodNotAllowed,
		)
	}
}
