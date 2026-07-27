package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type AgendaProfessional struct {
	ID       int64  `json:"id"`
	TenantID int64  `json:"tenant_id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Active   bool   `json:"active"`
}
type AgendaService struct {
	ID       int64  `json:"id"`
	TenantID int64  `json:"tenant_id"`
	Name     string `json:"name"`
	Duration int    `json:"duration_minutes"`
	Buffer   int    `json:"buffer_minutes"`
	Active   bool   `json:"active"`
}
type AgendaHours struct {
	ID             int64  `json:"id"`
	TenantID       int64  `json:"tenant_id"`
	ProfessionalID int64  `json:"professional_id"`
	Weekday        int    `json:"weekday"`
	Start          string `json:"start_time"`
	End            string `json:"end_time"`
	Active         bool   `json:"active"`
}

func initAgendaPremiumSchema(db *sql.DB) error {
	stmts := []string{
		`ALTER TABLE crm_appointments ADD COLUMN tenant_id INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE crm_appointments ADD COLUMN professional_id INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE crm_appointments ADD COLUMN service_id INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE crm_appointments ADD COLUMN timezone TEXT NOT NULL DEFAULT 'America/Bogota'`,
	}
	for _, s := range stmts {
		_, _ = db.Exec(s)
	}
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS agenda_settings(tenant_id INTEGER PRIMARY KEY,timezone TEXT NOT NULL DEFAULT 'America/Bogota',slot_interval INTEGER NOT NULL DEFAULT 30,min_notice_hours INTEGER NOT NULL DEFAULT 2,max_advance_days INTEGER NOT NULL DEFAULT 60,allow_weekends INTEGER NOT NULL DEFAULT 0,updated_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS agenda_professionals(id INTEGER PRIMARY KEY AUTOINCREMENT,tenant_id INTEGER NOT NULL,name TEXT NOT NULL,email TEXT NOT NULL DEFAULT '',phone TEXT NOT NULL DEFAULT '',active INTEGER NOT NULL DEFAULT 1,created_at TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS idx_agenda_prof_tenant ON agenda_professionals(tenant_id,active);
CREATE TABLE IF NOT EXISTS agenda_services(id INTEGER PRIMARY KEY AUTOINCREMENT,tenant_id INTEGER NOT NULL,name TEXT NOT NULL,duration_minutes INTEGER NOT NULL DEFAULT 30,buffer_minutes INTEGER NOT NULL DEFAULT 0,active INTEGER NOT NULL DEFAULT 1,created_at TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS idx_agenda_service_tenant ON agenda_services(tenant_id,active);
CREATE TABLE IF NOT EXISTS agenda_hours(id INTEGER PRIMARY KEY AUTOINCREMENT,tenant_id INTEGER NOT NULL,professional_id INTEGER NOT NULL DEFAULT 0,weekday INTEGER NOT NULL,start_time TEXT NOT NULL,end_time TEXT NOT NULL,active INTEGER NOT NULL DEFAULT 1);
CREATE UNIQUE INDEX IF NOT EXISTS idx_agenda_hours_unique ON agenda_hours(tenant_id,professional_id,weekday,start_time,end_time);
CREATE TABLE IF NOT EXISTS agenda_blocks(id INTEGER PRIMARY KEY AUTOINCREMENT,tenant_id INTEGER NOT NULL,professional_id INTEGER NOT NULL DEFAULT 0,starts_at TEXT NOT NULL,ends_at TEXT NOT NULL,reason TEXT NOT NULL DEFAULT 'Bloqueo',created_at TEXT NOT NULL);
`)
	return err
}

func (a *App) agendaSettingsHandler(w http.ResponseWriter, r *http.Request) {
	tid, u, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if r.Method == http.MethodGet {
		_, _ = a.db.Exec(`INSERT OR IGNORE INTO agenda_settings(tenant_id,updated_at) VALUES(?,?)`, tid, now)
		var tz string
		var interval, notice, advance, weekends int
		_ = a.db.QueryRow(`SELECT timezone,slot_interval,min_notice_hours,max_advance_days,allow_weekends FROM agenda_settings WHERE tenant_id=?`, tid).Scan(&tz, &interval, &notice, &advance, &weekends)
		writeJSON(w, map[string]any{"timezone": tz, "slot_interval": interval, "min_notice_hours": notice, "max_advance_days": advance, "allow_weekends": weekends == 1})
		return
	}
	if r.Method == http.MethodPut {
		if u.Role != "owner" && u.Role != "admin" && u.Role != "superadmin" {
			writeError(w, errors.New("sin permiso para configurar la agenda"), 403)
			return
		}
		var q struct {
			Timezone       string `json:"timezone"`
			SlotInterval   int    `json:"slot_interval"`
			MinNoticeHours int    `json:"min_notice_hours"`
			MaxAdvanceDays int    `json:"max_advance_days"`
			AllowWeekends  bool   `json:"allow_weekends"`
		}
		_ = json.NewDecoder(r.Body).Decode(&q)
		if q.Timezone == "" {
			q.Timezone = "America/Bogota"
		}
		if q.SlotInterval < 5 {
			q.SlotInterval = 30
		}
		if q.MaxAdvanceDays < 1 {
			q.MaxAdvanceDays = 60
		}
		wk := 0
		if q.AllowWeekends {
			wk = 1
		}
		_, err = a.db.Exec(`INSERT INTO agenda_settings(tenant_id,timezone,slot_interval,min_notice_hours,max_advance_days,allow_weekends,updated_at) VALUES(?,?,?,?,?,?,?) ON CONFLICT(tenant_id) DO UPDATE SET timezone=excluded.timezone,slot_interval=excluded.slot_interval,min_notice_hours=excluded.min_notice_hours,max_advance_days=excluded.max_advance_days,allow_weekends=excluded.allow_weekends,updated_at=excluded.updated_at`, tid, q.Timezone, q.SlotInterval, q.MinNoticeHours, q.MaxAdvanceDays, wk, now)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
		return
	}
	http.Error(w, "Método no permitido", 405)
}

func (a *App) agendaProfessionalsHandler(w http.ResponseWriter, r *http.Request) {
	tid, u, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	if r.Method == http.MethodGet {
		rows, e := a.db.Query(`SELECT id,tenant_id,name,email,phone,active FROM agenda_professionals WHERE tenant_id=? ORDER BY active DESC,name`, tid)
		if e != nil {
			writeError(w, e, 500)
			return
		}
		defer rows.Close()
		out := []AgendaProfessional{}
		for rows.Next() {
			var x AgendaProfessional
			var active int
			_ = rows.Scan(&x.ID, &x.TenantID, &x.Name, &x.Email, &x.Phone, &active)
			x.Active = active == 1
			out = append(out, x)
		}
		writeJSON(w, out)
		return
	}
	if u.Role != "owner" && u.Role != "admin" && u.Role != "superadmin" {
		writeError(w, errors.New("sin permiso"), 403)
		return
	}
	var q AgendaProfessional
	_ = json.NewDecoder(r.Body).Decode(&q)
	if strings.TrimSpace(q.Name) == "" {
		writeError(w, errors.New("nombre obligatorio"), 400)
		return
	}
	active := 0
	if q.Active {
		active = 1
	}
	if r.Method == http.MethodPost {
		res, e := a.db.Exec(`INSERT INTO agenda_professionals(tenant_id,name,email,phone,active,created_at) VALUES(?,?,?,?,?,?)`, tid, q.Name, q.Email, q.Phone, active, time.Now().UTC().Format(time.RFC3339))
		if e != nil {
			writeError(w, e, 500)
			return
		}
		q.ID, _ = res.LastInsertId()
		writeJSON(w, q)
		return
	}
	if r.Method == http.MethodPut {
		_, e := a.db.Exec(`UPDATE agenda_professionals SET name=?,email=?,phone=?,active=? WHERE id=? AND tenant_id=?`, q.Name, q.Email, q.Phone, active, q.ID, tid)
		if e != nil {
			writeError(w, e, 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
		return
	}
	if r.Method == http.MethodDelete {
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		_, e := a.db.Exec(`DELETE FROM agenda_professionals WHERE id=? AND tenant_id=?`, id, tid)
		if e != nil {
			writeError(w, e, 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
		return
	}
	http.Error(w, "Método no permitido", 405)
}

func (a *App) agendaServicesHandler(w http.ResponseWriter, r *http.Request) {
	tid, u, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	if r.Method == http.MethodGet {
		rows, e := a.db.Query(`SELECT id,tenant_id,name,duration_minutes,buffer_minutes,active FROM agenda_services WHERE tenant_id=? ORDER BY active DESC,name`, tid)
		if e != nil {
			writeError(w, e, 500)
			return
		}
		defer rows.Close()
		out := []AgendaService{}
		for rows.Next() {
			var x AgendaService
			var active int
			_ = rows.Scan(&x.ID, &x.TenantID, &x.Name, &x.Duration, &x.Buffer, &active)
			x.Active = active == 1
			out = append(out, x)
		}
		writeJSON(w, out)
		return
	}
	if u.Role != "owner" && u.Role != "admin" && u.Role != "superadmin" {
		writeError(w, errors.New("sin permiso"), 403)
		return
	}
	var q AgendaService
	_ = json.NewDecoder(r.Body).Decode(&q)
	if q.Name == "" {
		writeError(w, errors.New("nombre obligatorio"), 400)
		return
	}
	if q.Duration < 5 {
		q.Duration = 30
	}
	active := 0
	if q.Active {
		active = 1
	}
	if r.Method == http.MethodPost {
		res, e := a.db.Exec(`INSERT INTO agenda_services(tenant_id,name,duration_minutes,buffer_minutes,active,created_at) VALUES(?,?,?,?,?,?)`, tid, q.Name, q.Duration, q.Buffer, active, time.Now().UTC().Format(time.RFC3339))
		if e != nil {
			writeError(w, e, 500)
			return
		}
		q.ID, _ = res.LastInsertId()
		writeJSON(w, q)
		return
	}
	if r.Method == http.MethodPut {
		_, e := a.db.Exec(`UPDATE agenda_services SET name=?,duration_minutes=?,buffer_minutes=?,active=? WHERE id=? AND tenant_id=?`, q.Name, q.Duration, q.Buffer, active, q.ID, tid)
		if e != nil {
			writeError(w, e, 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
		return
	}
	if r.Method == http.MethodDelete {
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		_, e := a.db.Exec(`DELETE FROM agenda_services WHERE id=? AND tenant_id=?`, id, tid)
		if e != nil {
			writeError(w, e, 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
		return
	}
	http.Error(w, "Método no permitido", 405)
}

func (a *App) agendaHoursHandler(w http.ResponseWriter, r *http.Request) {
	tid, u, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	if r.Method == http.MethodGet {
		rows, e := a.db.Query(`SELECT id,tenant_id,professional_id,weekday,start_time,end_time,active FROM agenda_hours WHERE tenant_id=? ORDER BY professional_id,weekday,start_time`, tid)
		if e != nil {
			writeError(w, e, 500)
			return
		}
		defer rows.Close()
		out := []AgendaHours{}
		for rows.Next() {
			var x AgendaHours
			var active int
			_ = rows.Scan(&x.ID, &x.TenantID, &x.ProfessionalID, &x.Weekday, &x.Start, &x.End, &active)
			x.Active = active == 1
			out = append(out, x)
		}
		writeJSON(w, out)
		return
	}
	if u.Role != "owner" && u.Role != "admin" && u.Role != "superadmin" {
		writeError(w, errors.New("sin permiso"), 403)
		return
	}
	var q struct {
		ProfessionalID int64  `json:"professional_id"`
		Weekday        int    `json:"weekday"`
		Start          string `json:"start_time"`
		End            string `json:"end_time"`
		Active         bool   `json:"active"`
	}
	_ = json.NewDecoder(r.Body).Decode(&q)
	if q.Weekday < 0 || q.Weekday > 6 || q.Start == "" || q.End == "" || q.Start >= q.End {
		writeError(w, errors.New("horario inválido"), 400)
		return
	}
	active := 0
	if q.Active {
		active = 1
	}
	_, e := a.db.Exec(`INSERT OR REPLACE INTO agenda_hours(tenant_id,professional_id,weekday,start_time,end_time,active) VALUES(?,?,?,?,?,?)`, tid, q.ProfessionalID, q.Weekday, q.Start, q.End, active)
	if e != nil {
		writeError(w, e, 500)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (a *App) agendaAvailabilityHandler(w http.ResponseWriter, r *http.Request) {
	tid, _, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	date := r.URL.Query().Get("date")
	pid, _ := strconv.ParseInt(r.URL.Query().Get("professional_id"), 10, 64)
	sid, _ := strconv.ParseInt(r.URL.Query().Get("service_id"), 10, 64)
	d, e := time.Parse("2006-01-02", date)
	if e != nil {
		writeError(w, errors.New("fecha inválida"), 400)
		return
	}
	var duration = 30
	_ = a.db.QueryRow(`SELECT duration_minutes FROM agenda_services WHERE id=? AND tenant_id=?`, sid, tid).Scan(&duration)
	weekday := int(d.Weekday())
	rows, e := a.db.Query(`SELECT start_time,end_time FROM agenda_hours WHERE tenant_id=? AND professional_id IN (0,?) AND weekday=? AND active=1 ORDER BY professional_id DESC,start_time`, tid, pid, weekday)
	if e != nil {
		writeError(w, e, 500)
		return
	}
	defer rows.Close()
	type rng struct{ s, e string }
	rs := []rng{}
	for rows.Next() {
		var x rng
		_ = rows.Scan(&x.s, &x.e)
		rs = append(rs, x)
	}
	slots := []string{}
	for _, x := range rs {
		st, _ := time.Parse("15:04", x.s)
		en, _ := time.Parse("15:04", x.e)
		for cur := st; !cur.Add(time.Duration(duration) * time.Minute).After(en); cur = cur.Add(30 * time.Minute) {
			ts := fmt.Sprintf("%sT%s:00", date, cur.Format("15:04"))
			var n int
			_ = a.db.QueryRow(`SELECT COUNT(*) FROM crm_appointments WHERE tenant_id=? AND professional_id=? AND status NOT IN ('Cancelada','No asistió') AND starts_at < datetime(?, '+' || ? || ' minutes') AND datetime(starts_at, '+' || duration_minutes || ' minutes') > ?`, tid, pid, ts, duration, ts).Scan(&n)
			if n == 0 {
				slots = append(slots, ts)
			}
		}
	}
	writeJSON(w, map[string]any{"date": date, "professional_id": pid, "service_id": sid, "slots": slots})
}
