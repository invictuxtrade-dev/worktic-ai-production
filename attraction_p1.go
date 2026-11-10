package main

// Worktic AI — Atracción de Clientes P1A
// Almacena hallazgos revisados por una persona. No extrae miembros de grupos,
// no ejecuta scraping ni realiza envíos comerciales automáticos.

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type AttractionCandidateP1 struct {
	ID               int64  `json:"id"`
	Name             string `json:"name"`
	Kind             string `json:"kind"`
	City             string `json:"city"`
	Country          string `json:"country"`
	Niche            string `json:"niche"`
	Website          string `json:"website"`
	Email            string `json:"email"`
	Phone            string `json:"phone"`
	SourceURL        string `json:"source_url"`
	Signal           string `json:"signal"`
	ConsentConfirmed bool   `json:"consent_confirmed,omitempty"`
	Score            int    `json:"score"`
	Status           string `json:"status"`
	ContactID        int64  `json:"crm_contact_id"`
	OpportunityID    int64  `json:"crm_opportunity_id"`
	CreatedAt        string `json:"created_at"`
}

func attractionP1Enabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("WORKTIC_ATTRACTION_P1_ENABLED")), "true")
}

func attractionP1Clean(s string, limit int) string {
	s = strings.TrimSpace(s)
	if len([]rune(s)) > limit {
		s = string([]rune(s)[:limit])
	}
	return s
}

func attractionP1SourceURL(s string) (string, error) {
	s = attractionP1Clean(s, 1024)
	if s == "" {
		return "", nil
	}
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" || u.User != nil {
		return "", errors.New("la URL de origen debe ser http(s) válida y pública")
	}
	u.Fragment = ""
	return u.String(), nil
}

// Puntuación determinista, explicable. No presenta conjeturas como hechos.
func attractionP1Score(x AttractionCandidateP1) int {
	score := 15
	if x.Website != "" {
		score += 12
	}
	if x.City != "" && x.Country != "" {
		score += 8
	}
	if x.Niche != "" {
		score += 8
	}
	if x.Email != "" {
		score += 9
	}
	if x.Phone != "" {
		score += 8
	}
	if x.SourceURL != "" {
		score += 10
	}
	signal := strings.ToLower(x.Signal)
	for _, w := range []string{"necesit", "busco", "requier", "contratar", "cotiz", "presupuesto", "interesad", "solicit", "proveedor", "recomend"} {
		if strings.Contains(signal, w) {
			score += 20
			break
		}
	}
	if score > 100 {
		return 100
	}
	return score
}

func attractionP1SourceKey(x AttractionCandidateP1) string {
	anchor := strings.ToLower(strings.TrimSpace(x.SourceURL))
	if anchor == "" {
		anchor = strings.ToLower(strings.Join([]string{x.Kind, x.Name, x.City, x.Country}, "|"))
	}
	sum := sha256.Sum256([]byte(anchor))
	return hex.EncodeToString(sum[:])
}

func attractionP1Schema(db *DB) error {
	ddl := `CREATE TABLE IF NOT EXISTS attraction_candidates_p1 (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        tenant_id INTEGER NOT NULL,
        source_key TEXT NOT NULL,
        name TEXT NOT NULL,
        kind TEXT NOT NULL,
        city TEXT NOT NULL DEFAULT '',
        country TEXT NOT NULL DEFAULT '',
        niche TEXT NOT NULL DEFAULT '',
        website TEXT NOT NULL DEFAULT '',
        email TEXT NOT NULL DEFAULT '',
        phone TEXT NOT NULL DEFAULT '',
        source_url TEXT NOT NULL DEFAULT '',
        signal TEXT NOT NULL DEFAULT '',
        score INTEGER NOT NULL DEFAULT 0,
        status TEXT NOT NULL DEFAULT 'review',
        crm_contact_id INTEGER NOT NULL DEFAULT 0,
        crm_opportunity_id INTEGER NOT NULL DEFAULT 0,
        created_at TEXT NOT NULL,
        updated_at TEXT NOT NULL,
        UNIQUE(tenant_id,source_key)
    )`
	if db.IsPostgres() {
		ddl = strings.Replace(ddl, "id INTEGER PRIMARY KEY AUTOINCREMENT", "id BIGSERIAL PRIMARY KEY", 1)
	}
	_, err := db.Exec(ddl)
	if err != nil {
		return err
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_attraction_p1_tenant ON attraction_candidates_p1(tenant_id,created_at DESC)`)
	return err
}

func (a *App) attractionP1CandidatesHandler(w http.ResponseWriter, r *http.Request) {
	tenant, _, err := a.tenantFor(r)
	if err != nil {
		writeError(w, err, http.StatusUnauthorized)
		return
	}
	if !attractionP1Enabled() {
		writeError(w, errors.New("Atracción de Clientes está desactivada para este despliegue"), http.StatusServiceUnavailable)
		return
	}
	if err := attractionP1Schema(a.db); err != nil {
		writeError(w, err, 500)
		return
	}
	switch r.Method {
	case http.MethodGet:
		rows, err := a.db.Query(`SELECT id,name,kind,city,country,niche,website,email,phone,source_url,signal,score,status,crm_contact_id,crm_opportunity_id,created_at FROM attraction_candidates_p1 WHERE tenant_id=? ORDER BY id DESC LIMIT 250`, tenant)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		defer rows.Close()
		out := []AttractionCandidateP1{}
		for rows.Next() {
			var x AttractionCandidateP1
			if err := rows.Scan(&x.ID, &x.Name, &x.Kind, &x.City, &x.Country, &x.Niche, &x.Website, &x.Email, &x.Phone, &x.SourceURL, &x.Signal, &x.Score, &x.Status, &x.ContactID, &x.OpportunityID, &x.CreatedAt); err != nil {
				writeError(w, err, 500)
				return
			}
			out = append(out, x)
		}
		if err := rows.Err(); err != nil {
			writeError(w, err, 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "items": out})
	case http.MethodPost:
		if attractionP1BAIEnabled() {writeError(w,errors.New("La carga manual está desactivada en Autopilot IA"),403);return}
		r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
		var x AttractionCandidateP1
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&x); err != nil {
			writeError(w, errors.New("datos de oportunidad inválidos"), 400)
			return
		}
		x.Name = attractionP1Clean(x.Name, 180)
		x.Kind = attractionP1Clean(x.Kind, 30)
		x.City = attractionP1Clean(x.City, 100)
		x.Country = attractionP1Clean(x.Country, 100)
		x.Niche = attractionP1Clean(x.Niche, 140)
		x.Email = attractionP1Clean(x.Email, 180)
		x.Phone = attractionP1Clean(x.Phone, 80)
		x.Signal = attractionP1Clean(x.Signal, 1200)
		if x.Kind != "business" && x.Kind != "group_signal" {
			writeError(w, errors.New("origen no permitido"), 400)
			return
		}
		if x.Name == "" {
			writeError(w, errors.New("nombre obligatorio"), 400)
			return
		}
		x.Website, err = attractionP1SourceURL(x.Website)
		if err != nil {
			writeError(w, err, 400)
			return
		}
		x.SourceURL, err = attractionP1SourceURL(x.SourceURL)
		if err != nil {
			writeError(w, err, 400)
			return
		}
		if x.Kind == "group_signal" && (x.Email != "" || x.Phone != "") && !x.ConsentConfirmed {
			writeError(w, errors.New("confirma la autorización para registrar datos de contacto de comunidades"), 400)
			return
		}
		if x.Kind == "group_signal" && (x.SourceURL == "" || x.Signal == "") {
			writeError(w, errors.New("indica el enlace público y la señal comercial observada"), 400)
			return
		}
		x.Score = attractionP1Score(x)
		now := time.Now().UTC().Format(time.RFC3339)
		key := attractionP1SourceKey(x)
		_, err = a.db.Exec(`INSERT INTO attraction_candidates_p1(tenant_id,source_key,name,kind,city,country,niche,website,email,phone,source_url,signal,score,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(tenant_id,source_key) DO NOTHING`, tenant, key, x.Name, x.Kind, x.City, x.Country, x.Niche, x.Website, x.Email, x.Phone, x.SourceURL, x.Signal, x.Score, now, now)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		var id int64
		if err := a.db.QueryRow(`SELECT id FROM attraction_candidates_p1 WHERE tenant_id=? AND source_key=?`, tenant, key).Scan(&id); err != nil {
			writeError(w, err, 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "id": id, "score": x.Score})
	default:
		http.Error(w, "Método no permitido", 405)
	}
}

func (a *App) attractionP1PromoteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	tenant, _, err := a.tenantFor(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	if !attractionP1Enabled() {
		writeError(w, errors.New("módulo desactivado"), 503)
		return
	}
	if err := attractionP1Schema(a.db); err != nil {
		writeError(w, err, 500)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 2048)
	var q struct {
		ID int64 `json:"id"`
	}
	if json.NewDecoder(r.Body).Decode(&q) != nil || q.ID <= 0 {
		writeError(w, errors.New("id inválido"), 400)
		return
	}
	var x AttractionCandidateP1
	err = a.db.QueryRow(`SELECT name,kind,city,country,niche,website,email,phone,source_url,signal,score,status,crm_contact_id,crm_opportunity_id FROM attraction_candidates_p1 WHERE id=? AND tenant_id=?`, q.ID, tenant).Scan(&x.Name, &x.Kind, &x.City, &x.Country, &x.Niche, &x.Website, &x.Email, &x.Phone, &x.SourceURL, &x.Signal, &x.Score, &x.Status, &x.ContactID, &x.OpportunityID)
	if err == sql.ErrNoRows {
		writeError(w, errors.New("oportunidad no encontrada"), 404)
		return
	}
	if err != nil {
		writeError(w, err, 500)
		return
	}
	if x.Status == "promoted" && x.ContactID > 0 && x.OpportunityID > 0 {
		writeJSON(w, map[string]any{"ok": true, "contact_id": x.ContactID, "opportunity_id": x.OpportunityID, "already": true})
		return
	}
	// Un registro de grupo requiere identificar una empresa/contacto autorizado, no un perfil personal de un grupo.
	if x.Kind == "group_signal" && x.Email == "" && x.Phone == "" {
		writeError(w, errors.New("para enviar al CRM, registra un contacto empresarial autorizado (correo o teléfono)"), 409)
		return
	}
	ext := fmt.Sprintf("attraction:p1:%d", q.ID)
	if err := a.syncCRMContact(tenant, x.Name, x.Phone, x.Email, "manual", "attraction", ext); err != nil {
		writeError(w, err, 500)
		return
	}
	contactID, err := a.contactIDForOpportunity(tenant, 0, x.Phone, x.Email, ext)
	if err != nil || contactID <= 0 {
		writeError(w, errors.New("no se pudo validar el contacto del CRM"), 500)
		return
	}
	opportunityID, _, err := a.syncAutomaticOpportunity(tenant, contactID, "attraction", ext, "manual", x.Niche, "Nuevo", x.Score, "")
	if err != nil || opportunityID <= 0 {
		writeError(w, errors.New("no se pudo generar la oportunidad del CRM"), 500)
		return
	}
	_, err = a.db.Exec(`UPDATE attraction_candidates_p1 SET status='promoted',crm_contact_id=?,crm_opportunity_id=?,updated_at=? WHERE id=? AND tenant_id=?`, contactID, opportunityID, time.Now().UTC().Format(time.RFC3339), q.ID, tenant)
	if err != nil {
		writeError(w, err, 500)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "contact_id": contactID, "opportunity_id": opportunityID})
}
