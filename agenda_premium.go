package main

import (
	"database/sql"
	"encoding/json"
	"errors"
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

func canManageAgenda(role string) bool {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "superadmin", "owner", "admin", "supervisor":
		return true
	default:
		return false
	}
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
		if !canManageAgenda(u.Role) {
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
	if !canManageAgenda(u.Role) {
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
	if !canManageAgenda(u.Role) {
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
			if e = rows.Scan(&x.ID, &x.TenantID, &x.ProfessionalID, &x.Weekday, &x.Start, &x.End, &active); e != nil {
				writeError(w, e, 500)
				return
			}
			x.Active = active == 1
			out = append(out, x)
		}
		writeJSON(w, out)
		return
	}
	if !canManageAgenda(u.Role) {
		writeError(w, errors.New("sin permiso para gestionar la disponibilidad"), 403)
		return
	}
	if r.Method == http.MethodDelete {
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		if id <= 0 {
			writeError(w, errors.New("horario inválido"), 400)
			return
		}
		res, e := a.db.Exec(`DELETE FROM agenda_hours WHERE id=? AND tenant_id=?`, id, tid)
		if e != nil {
			writeError(w, e, 500)
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			writeError(w, errors.New("horario no encontrado"), 404)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
		return
	}
	var q AgendaHours
	if e := json.NewDecoder(r.Body).Decode(&q); e != nil {
		writeError(w, errors.New("datos inválidos"), 400)
		return
	}
	q.Start = strings.TrimSpace(q.Start)
	q.End = strings.TrimSpace(q.End)
	if q.Weekday < 0 || q.Weekday > 6 || q.Start == "" || q.End == "" || q.Start >= q.End {
		writeError(w, errors.New("horario inválido"), 400)
		return
	}
	if q.ProfessionalID > 0 {
		var n int
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM agenda_professionals WHERE id=? AND tenant_id=?`, q.ProfessionalID, tid).Scan(&n)
		if n == 0 {
			writeError(w, errors.New("profesional no encontrado"), 400)
			return
		}
	}
	active := 0
	if q.Active {
		active = 1
	}
	switch r.Method {
	case http.MethodPost:
		res, e := a.db.Exec(`INSERT INTO agenda_hours(tenant_id,professional_id,weekday,start_time,end_time,active) VALUES(?,?,?,?,?,?)`, tid, q.ProfessionalID, q.Weekday, q.Start, q.End, active)
		if e != nil {
			if strings.Contains(strings.ToLower(e.Error()), "unique") {
				writeError(w, errors.New("ese bloque horario ya existe"), 409)
				return
			}
			writeError(w, e, 500)
			return
		}
		q.ID, _ = res.LastInsertId()
		q.TenantID = tid
		writeJSON(w, q)
	case http.MethodPut:
		if q.ID <= 0 {
			writeError(w, errors.New("horario inválido"), 400)
			return
		}
		res, e := a.db.Exec(`UPDATE agenda_hours SET professional_id=?,weekday=?,start_time=?,end_time=?,active=? WHERE id=? AND tenant_id=?`, q.ProfessionalID, q.Weekday, q.Start, q.End, active, q.ID, tid)
		if e != nil {
			if strings.Contains(strings.ToLower(e.Error()), "unique") {
				writeError(w, errors.New("ese bloque horario ya existe"), 409)
				return
			}
			writeError(w, e, 500)
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			writeError(w, errors.New("horario no encontrado"), 404)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	default:
		http.Error(w, "Método no permitido", 405)
	}
}

func (a *App) agendaAvailabilityHandler(w http.ResponseWriter, r *http.Request) {
	tid, _, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	date := strings.TrimSpace(r.URL.Query().Get("date"))
	pid, _ := strconv.ParseInt(r.URL.Query().Get("professional_id"), 10, 64)
	sid, _ := strconv.ParseInt(r.URL.Query().Get("service_id"), 10, 64)
	currentID, _ := strconv.ParseInt(r.URL.Query().Get("current_appointment_id"), 10, 64)
	d, e := time.Parse("2006-01-02", date)
	if e != nil {
		writeError(w, errors.New("fecha inválida"), 400)
		return
	}

	var timezone string
	var interval, notice, advance, allowWeekends int
	_ = a.db.QueryRow(`SELECT timezone,slot_interval,min_notice_hours,max_advance_days,allow_weekends FROM agenda_settings WHERE tenant_id=?`, tid).Scan(&timezone, &interval, &notice, &advance, &allowWeekends)
	if timezone == "" {
		timezone = "America/Bogota"
	}
	if interval < 5 {
		interval = 30
	}
	if advance < 1 {
		advance = 60
	}
	loc, e := time.LoadLocation(timezone)
	if e != nil {
		loc, _ = time.LoadLocation("America/Bogota")
	}
	now := time.Now().In(loc)
	dayStart := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, loc)
	if dayStart.Before(time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)) || dayStart.After(now.AddDate(0, 0, advance)) {
		writeJSON(w, map[string]any{"date": date, "professional_id": pid, "service_id": sid, "slots": []string{}, "timezone": timezone})
		return
	}
	if allowWeekends == 0 && (d.Weekday() == time.Saturday || d.Weekday() == time.Sunday) {
		writeJSON(w, map[string]any{"date": date, "professional_id": pid, "service_id": sid, "slots": []string{}, "timezone": timezone})
		return
	}
	var duration, buffer int
	if sid > 0 {
		if e = a.db.QueryRow(`SELECT duration_minutes,buffer_minutes FROM agenda_services WHERE id=? AND tenant_id=? AND active=1`, sid, tid).Scan(&duration, &buffer); e != nil {
			writeError(w, errors.New("servicio no encontrado o inactivo"), 400)
			return
		}
	} else {
		duration = 30
	}

	weekday := int(d.Weekday())
	// Si el profesional tiene horario propio para el día, se usa solo ese. De lo contrario se usa el horario general.
	hourPID := int64(0)
	if pid > 0 {
		var n int
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM agenda_hours WHERE tenant_id=? AND professional_id=? AND weekday=? AND active=1`, tid, pid, weekday).Scan(&n)
		if n > 0 {
			hourPID = pid
		}
	}
	rows, e := a.db.Query(`SELECT start_time,end_time FROM agenda_hours WHERE tenant_id=? AND professional_id=? AND weekday=? AND active=1 ORDER BY start_time`, tid, hourPID, weekday)
	if e != nil {
		writeError(w, e, 500)
		return
	}
	defer rows.Close()
	type rng struct{ s, e string }
	rs := []rng{}
	for rows.Next() {
		var x rng
		if e = rows.Scan(&x.s, &x.e); e != nil {
			writeError(w, e, 500)
			return
		}
		rs = append(rs, x)
	}

	slots := []string{}
	seen := map[string]bool{}
	for _, x := range rs {
		stClock, es := time.Parse("15:04", x.s)
		enClock, ee := time.Parse("15:04", x.e)
		if es != nil || ee != nil {
			continue
		}
		start := time.Date(d.Year(), d.Month(), d.Day(), stClock.Hour(), stClock.Minute(), 0, 0, loc)
		end := time.Date(d.Year(), d.Month(), d.Day(), enClock.Hour(), enClock.Minute(), 0, 0, loc)
		for cur := start; !cur.Add(time.Duration(duration) * time.Minute).After(end); cur = cur.Add(time.Duration(interval) * time.Minute) {
			if cur.Before(now.Add(time.Duration(notice) * time.Hour)) {
				continue
			}
			ts := cur.Format("2006-01-02T15:04:05")
			endTS := cur.Add(time.Duration(duration+buffer) * time.Minute).Format("2006-01-02T15:04:05")
			var blocked int
			_ = a.db.QueryRow(`SELECT COUNT(*) FROM agenda_blocks WHERE tenant_id=? AND professional_id IN (0,?) AND starts_at < ? AND ends_at > ?`, tid, pid, endTS, ts).Scan(&blocked)
			if blocked > 0 {
				continue
			}
			var occupied int
			_ = a.db.QueryRow(`SELECT COUNT(*) FROM crm_appointments WHERE tenant_id=? AND professional_id=? AND id<>? AND status NOT IN ('Cancelada','No asistió') AND starts_at < ? AND datetime(starts_at, '+' || (duration_minutes + ?) || ' minutes') > ?`, tid, pid, currentID, endTS, buffer, ts).Scan(&occupied)
			if occupied == 0 && !seen[ts] {
				seen[ts] = true
				slots = append(slots, ts)
			}
		}
	}
	writeJSON(w, map[string]any{"date": date, "professional_id": pid, "service_id": sid, "slots": slots, "timezone": timezone})
}
