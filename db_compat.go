package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync/atomic"

	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

type DB struct {
	raw     *sql.DB
	dialect string
}

type Tx struct {
	raw     *sql.Tx
	dialect string
}

type compatResult struct {
	id   int64
	rows int64
}

func (r compatResult) LastInsertId() (int64, error) { return r.id, nil }
func (r compatResult) RowsAffected() (int64, error) { return r.rows, nil }

var postgresSerialTables = map[string]bool{
	"ads_campaigns_v25": true, "ads_connections_v25": true, "ads_daily_metrics_v25": true, "ads_entities_v25": true,
	"ads_oauth_states_v25": true, "ads_sync_log_v25": true, "agenda_blocks": true, "agenda_hours": true,
	"agenda_professionals": true, "agenda_services": true, "ai_agent_audit": true, "ai_agent_knowledge_v22": true,
	"ai_agent_memory_v22": true, "ai_agent_permissions": true, "ai_agent_routes": true, "ai_agent_tool_audit_v22": true,
	"ai_agent_usage": true, "ai_agents": true, "analytics_attribution_events_v24": true, "app_users": true,
	"automation_events": true, "automation_execution_steps": true, "automation_executions": true, "automation_workflows": true,
	"billing_payments": true, "billing_plans": true, "billing_promo_codes": true, "billing_subscriptions": true,
	"channel_audit": true, "channel_connections": true, "channel_events": true, "crm_appointments": true,
	"crm_contacts": true, "crm_opportunities": true, "crm_products": true, "group_activity": true,
	"group_prospects": true, "inbox_conversations": true, "inbox_departments": true, "inbox_notes": true,
	"lead_automation_outbox": true, "managed_groups": true, "marketing_campaigns": true, "marketing_content": true,
	"marketing_creatives": true, "marketing_forms": true, "marketing_landings": true, "marketing_leads": true,
	"marketing_meta_connections": true, "marketing_metrics": true, "messenger_outbox": true, "meta_lead_events": true,
	"meta_lead_forms": true, "meta_lead_profiles": true, "security_audit_v26": true, "social_approvals": true,
	"social_comments_v23": true, "social_connections": true, "social_governance_audit_v231": true, "social_media_assets": true,
	"social_metrics": true, "social_oauth_states": true, "social_post_groups": true, "social_posts": true,
	"social_publish_attempts": true, "social_repurpose_jobs_v23": true, "team_invitations": true, "tenants": true,
	"whatsapp_business_events": true, "whatsapp_template_drafts": true, "whatsapp_marketing_campaigns_v27": true, "whatsapp_marketing_recipients_v27": true, "worktic_auto_rules": true, "worktic_messages": true,
}

var insertTableRx = regexp.MustCompile(`(?is)^\s*INSERT\s+INTO\s+([a-zA-Z0-9_]+)\b`)

func openAppDB(driver, dsn string) (*DB, error) {
	driver = strings.ToLower(strings.TrimSpace(driver))
	if driver == "" {
		if strings.HasPrefix(strings.ToLower(dsn), "postgres://") || strings.HasPrefix(strings.ToLower(dsn), "postgresql://") {
			driver = "postgres"
		} else {
			driver = "sqlite"
		}
	}
	sqlDriver := driver
	if driver == "postgresql" {
		sqlDriver = "postgres"
	}
	raw, err := sql.Open(sqlDriver, dsn)
	if err != nil {
		return nil, err
	}
	db := &DB{raw: raw, dialect: driver}
	if driver == "postgres" || driver == "postgresql" {
		raw.SetMaxOpenConns(30)
		raw.SetMaxIdleConns(10)
	} else {
		raw.SetMaxOpenConns(1)
		raw.SetMaxIdleConns(1)
	}
	return db, nil
}

func (d *DB) Raw() *sql.DB                          { return d.raw }
func (d *DB) Dialect() string                       { return d.dialect }
func (d *DB) IsPostgres() bool                      { return d.dialect == "postgres" || d.dialect == "postgresql" }
func (d *DB) Close() error                          { return d.raw.Close() }
func (d *DB) PingContext(ctx context.Context) error { return d.raw.PingContext(ctx) }
func (d *DB) Begin() (*Tx, error) {
	tx, err := d.raw.Begin()
	if err != nil {
		return nil, err
	}
	return &Tx{raw: tx, dialect: d.dialect}, nil
}

func (d *DB) Exec(query string, args ...any) (sql.Result, error) {
	q := adaptSQL(query, d.dialect)
	if d.IsPostgres() && len(args) == 0 && strings.Contains(q, ";") {
		var total int64
		for _, stmt := range splitSQLStatements(q) {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			res, err := d.execOne(stmt)
			if err != nil {
				return nil, err
			}
			if n, e := res.RowsAffected(); e == nil {
				total += n
			}
		}
		return compatResult{rows: total}, nil
	}
	return d.execArgs(q, args...)
}

func (d *DB) execOne(q string) (sql.Result, error) { return d.execArgs(q) }
func (d *DB) execArgs(q string, args ...any) (sql.Result, error) {
	if d.IsPostgres() {
		if m := insertTableRx.FindStringSubmatch(q); len(m) == 2 && postgresSerialTables[strings.ToLower(m[1])] && !strings.Contains(strings.ToUpper(q), " RETURNING ") {
			q = strings.TrimSuffix(strings.TrimSpace(q), ";") + " RETURNING id"
			var id int64
			err := d.raw.QueryRow(q, args...).Scan(&id)
			if errors.Is(err, sql.ErrNoRows) {
				return compatResult{}, nil
			}
			if err != nil {
				return nil, err
			}
			return compatResult{id: id, rows: 1}, nil
		}
	}
	return d.raw.Exec(q, args...)
}

func (d *DB) Query(query string, args ...any) (*sql.Rows, error) {
	return d.raw.Query(adaptSQL(query, d.dialect), args...)
}
func (d *DB) QueryRow(query string, args ...any) *sql.Row {
	return d.raw.QueryRow(adaptSQL(query, d.dialect), args...)
}

func (t *Tx) Exec(query string, args ...any) (sql.Result, error) {
	q := adaptSQL(query, t.dialect)
	if t.dialect == "postgres" || t.dialect == "postgresql" {
		if m := insertTableRx.FindStringSubmatch(q); len(m) == 2 && postgresSerialTables[strings.ToLower(m[1])] && !strings.Contains(strings.ToUpper(q), " RETURNING ") {
			q = strings.TrimSuffix(strings.TrimSpace(q), ";") + " RETURNING id"
			var id int64
			err := t.raw.QueryRow(q, args...).Scan(&id)
			if errors.Is(err, sql.ErrNoRows) {
				return compatResult{}, nil
			}
			if err != nil {
				return nil, err
			}
			return compatResult{id: id, rows: 1}, nil
		}
	}
	return t.raw.Exec(q, args...)
}
func (t *Tx) Query(query string, args ...any) (*sql.Rows, error) {
	return t.raw.Query(adaptSQL(query, t.dialect), args...)
}
func (t *Tx) QueryRow(query string, args ...any) *sql.Row {
	return t.raw.QueryRow(adaptSQL(query, t.dialect), args...)
}
func (t *Tx) Commit() error   { return t.raw.Commit() }
func (t *Tx) Rollback() error { return t.raw.Rollback() }

func adaptSQL(query, dialect string) string {
	if dialect != "postgres" && dialect != "postgresql" {
		return query
	}
	q := strings.ReplaceAll(query, "INTEGER PRIMARY KEY AUTOINCREMENT", "BIGSERIAL PRIMARY KEY")
	q = strings.ReplaceAll(q, "integer primary key autoincrement", "BIGSERIAL PRIMARY KEY")
	// Make additive migrations idempotent on PostgreSQL.
	alterRx := regexp.MustCompile(`(?i)ALTER\s+TABLE\s+([a-zA-Z0-9_]+)\s+ADD\s+COLUMN\s+`)
	q = alterRx.ReplaceAllString(q, "ALTER TABLE $1 ADD COLUMN IF NOT EXISTS ")
	// SQLite datetime arithmetic used by Agenda -> PostgreSQL interval arithmetic.
	q = strings.ReplaceAll(q, "starts_at < datetime(?, '+' || ? || ' minutes') AND datetime(starts_at, '+' || duration_minutes || ' minutes') > ?", "starts_at::timestamptz < (?::timestamptz + (? || ' minutes')::interval) AND (starts_at::timestamptz + (duration_minutes || ' minutes')::interval) > ?::timestamptz")
	q = strings.ReplaceAll(q, "starts_at < ? AND datetime(starts_at, '+' || (duration_minutes + ?) || ' minutes') > ?", "starts_at::timestamptz < ?::timestamptz AND (starts_at::timestamptz + ((duration_minutes + ?) || ' minutes')::interval) > ?::timestamptz")
	q = strings.ReplaceAll(q, "instr(','||tags||',', ','||?||',')>0", "position(','||?||',' in ','||tags||',')>0")
	stmts := splitSQLStatements(q)
	for i, stmt := range stmts {
		s := strings.TrimSpace(stmt)
		upper := strings.ToUpper(s)
		if strings.HasPrefix(upper, "INSERT OR IGNORE INTO ") {
			s = "INSERT INTO " + strings.TrimSpace(s[len("INSERT OR IGNORE INTO "):])
			if !strings.Contains(strings.ToUpper(s), "ON CONFLICT") {
				s += " ON CONFLICT DO NOTHING"
			}
		}
		stmts[i] = s
	}
	q = strings.Join(stmts, ";")
	return rebindPostgres(q)
}

func rebindPostgres(q string) string {
	var b strings.Builder
	b.Grow(len(q) + 16)
	n := int64(0)
	inSingle, inDouble := false, false
	for i := 0; i < len(q); i++ {
		c := q[i]
		if c == '\'' && !inDouble {
			b.WriteByte(c)
			if inSingle && i+1 < len(q) && q[i+1] == '\'' {
				b.WriteByte(q[i+1])
				i++
				continue
			}
			inSingle = !inSingle
			continue
		}
		if c == '"' && !inSingle {
			inDouble = !inDouble
			b.WriteByte(c)
			continue
		}
		if c == '?' && !inSingle && !inDouble {
			idx := atomic.AddInt64(&n, 1)
			b.WriteString(fmt.Sprintf("$%d", idx))
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

func splitSQLStatements(q string) []string {
	var out []string
	var b strings.Builder
	inSingle, inDouble := false, false
	for i := 0; i < len(q); i++ {
		c := q[i]
		if c == '\'' && !inDouble {
			b.WriteByte(c)
			if inSingle && i+1 < len(q) && q[i+1] == '\'' {
				b.WriteByte(q[i+1])
				i++
				continue
			}
			inSingle = !inSingle
			continue
		}
		if c == '"' && !inSingle {
			inDouble = !inDouble
			b.WriteByte(c)
			continue
		}
		if c == ';' && !inSingle && !inDouble {
			out = append(out, b.String())
			b.Reset()
			continue
		}
		b.WriteByte(c)
	}
	if strings.TrimSpace(b.String()) != "" {
		out = append(out, b.String())
	}
	return out
}
