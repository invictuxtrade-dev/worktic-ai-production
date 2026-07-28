package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

var adminFullSchemaOnce sync.Once
var adminFullSchemaErr error

func (a *App) ensureAdminFullSchema() error {
	adminFullSchemaOnce.Do(func() {
		stmts := []string{
			`ALTER TABLE app_users ADD COLUMN deleted_at TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE app_users ADD COLUMN blocked_reason TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE app_users ADD COLUMN updated_at TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE billing_plans ADD COLUMN deleted_at TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE billing_subscriptions ADD COLUMN source TEXT NOT NULL DEFAULT 'system'`,
			`ALTER TABLE billing_subscriptions ADD COLUMN admin_note TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE billing_subscriptions ADD COLUMN granted_by INTEGER NOT NULL DEFAULT 0`,
		}
		for _, stmt := range stmts {
			_, _ = a.db.Exec(stmt)
		}
		_, adminFullSchemaErr = a.db.Exec(`CREATE INDEX IF NOT EXISTS idx_app_users_admin_full ON app_users(active,role,tenant_id,created_at)`)
		if adminFullSchemaErr == nil {
			_, adminFullSchemaErr = a.db.Exec(`CREATE INDEX IF NOT EXISTS idx_billing_subscriptions_admin_full ON billing_subscriptions(user_id,status,ends_at,source)`)
		}
		if adminFullSchemaErr == nil {
			_, adminFullSchemaErr = a.db.Exec(`CREATE INDEX IF NOT EXISTS idx_billing_payments_admin_full ON billing_payments(status,network,created_at,user_id)`)
		}
	})
	return adminFullSchemaErr
}

func adminPageValues(r *http.Request) (int, int, int) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if page < 1 {
		page = 1
	}
	if perPage < 10 {
		perPage = 10
	}
	if perPage > 100 {
		perPage = 100
	}
	return page, perPage, (page - 1) * perPage
}

func adminLike(v string) string { return "%" + strings.ToLower(strings.TrimSpace(v)) + "%" }

func paginationMeta(page, perPage, total int) map[string]any {
	pages := 0
	if total > 0 {
		pages = int(math.Ceil(float64(total) / float64(perPage)))
	}
	return map[string]any{"page": page, "per_page": perPage, "total": total, "pages": pages}
}

func (a *App) adminFullOverviewHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireSuperadmin(w, r); !ok {
		return
	}
	if err := a.ensureAdminFullSchema(); err != nil {
		writeError(w, err, 500)
		return
	}
	var users, activeUsers, blockedUsers, deletedUsers, activeSubs, courtesySubs, pendingPayments, plans int
	var revenue float64
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM app_users WHERE COALESCE(deleted_at,'')=''`).Scan(&users)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM app_users WHERE active=1 AND COALESCE(deleted_at,'')=''`).Scan(&activeUsers)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM app_users WHERE active=0 AND COALESCE(deleted_at,'')=''`).Scan(&blockedUsers)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM app_users WHERE COALESCE(deleted_at,'')<>''`).Scan(&deletedUsers)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM billing_subscriptions WHERE status='active' AND ends_at>?`, time.Now().UTC().Format(time.RFC3339)).Scan(&activeSubs)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM billing_subscriptions WHERE source='courtesy'`).Scan(&courtesySubs)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM billing_payments WHERE status='pending'`).Scan(&pendingPayments)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM billing_plans WHERE COALESCE(deleted_at,'')=''`).Scan(&plans)
	_ = a.db.QueryRow(`SELECT COALESCE(SUM(amount_usdt),0) FROM billing_payments WHERE status='approved'`).Scan(&revenue)
	writeJSON(w, map[string]any{
		"users": users, "active_users": activeUsers, "blocked_users": blockedUsers, "deleted_users": deletedUsers,
		"active_subscriptions": activeSubs, "courtesy_licenses": courtesySubs, "pending_payments": pendingPayments,
		"plans": plans, "revenue_usdt": revenue,
	})
}

func (a *App) adminFullUsersHandler(w http.ResponseWriter, r *http.Request) {
	admin, ok := a.requireSuperadmin(w, r)
	if !ok {
		return
	}
	if err := a.ensureAdminFullSchema(); err != nil {
		writeError(w, err, 500)
		return
	}
	switch r.Method {
	case http.MethodGet:
		a.adminFullUsersList(w, r)
	case http.MethodPost:
		var q struct {
			Name        string `json:"name"`
			Email       string `json:"email"`
			Password    string `json:"password"`
			Role        string `json:"role"`
			Company     string `json:"company"`
			AccountType string `json:"account_type"`
		}
		if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
			writeError(w, errors.New("datos inválidos"), 400)
			return
		}
		q.Name = strings.TrimSpace(q.Name)
		q.Email = strings.ToLower(strings.TrimSpace(q.Email))
		q.Company = strings.TrimSpace(q.Company)
		if len(q.Name) < 2 || !strings.Contains(q.Email, "@") || len(q.Password) < 8 {
			writeError(w, errors.New("nombre, correo válido y contraseña de mínimo 8 caracteres son obligatorios"), 400)
			return
		}
		if q.Role != "superadmin" {
			q.Role = "owner"
		}
		if q.Role == "superadmin" {
			q.AccountType = "business"
		}
		if q.AccountType != "personal" && q.AccountType != "business" {
			q.AccountType = "business"
		}
		if q.Company == "" {
			q.Company = q.Name
		}
		now := time.Now().UTC()
		tx, err := a.db.Begin()
		if err != nil {
			writeError(w, err, 500)
			return
		}
		defer tx.Rollback()
		res, err := tx.Exec(`INSERT INTO app_users(name,email,password_hash,role,company,active,created_at,tenant_id,updated_at) VALUES(?,?,?,?,?,1,?,0,?)`, q.Name, q.Email, hashPassword(q.Password, a.adminSalt()), q.Role, q.Company, now.Format(time.RFC3339), now.Format(time.RFC3339))
		if err != nil {
			writeError(w, errors.New("el correo ya está registrado"), 400)
			return
		}
		uid, _ := res.LastInsertId()
		if q.Role != "superadmin" {
			tenantRes, err := tx.Exec(`INSERT INTO tenants(name,account_type,owner_user_id,status,created_at,updated_at) VALUES(?,?,?,?,?,?)`, q.Company, q.AccountType, uid, "active", now.Format(time.RFC3339), now.Format(time.RFC3339))
			if err != nil {
				writeError(w, err, 500)
				return
			}
			tenantID, _ := tenantRes.LastInsertId()
			if _, err = tx.Exec(`UPDATE app_users SET tenant_id=? WHERE id=?`, tenantID, uid); err != nil {
				writeError(w, err, 500)
				return
			}
			if _, err = tx.Exec(`INSERT INTO tenant_users(tenant_id,user_id,role,created_at) VALUES(?,?,?,?)`, tenantID, uid, q.Role, now.Format(time.RFC3339)); err != nil {
				writeError(w, err, 500)
				return
			}
		}
		if _, err = tx.Exec(`INSERT INTO billing_subscriptions(user_id,plan_code,status,starts_at,ends_at,created_at,source,admin_note,granted_by) VALUES(?,'free','active',?,?,?,?,?,?)`, uid, now.Format(time.RFC3339), now.AddDate(10, 0, 0).Format(time.RFC3339), now.Format(time.RFC3339), "admin", "Cuenta creada por superadministración", admin.ID); err != nil {
			writeError(w, err, 500)
			return
		}
		if err = tx.Commit(); err != nil {
			writeError(w, err, 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "id": uid})
	case http.MethodPut:
		var q struct {
			ID          int64  `json:"id"`
			Action      string `json:"action"`
			Name        string `json:"name"`
			Email       string `json:"email"`
			Password    string `json:"password"`
			Role        string `json:"role"`
			Company     string `json:"company"`
			AccountType string `json:"account_type"`
			Reason      string `json:"reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&q); err != nil || q.ID == 0 {
			writeError(w, errors.New("usuario inválido"), 400)
			return
		}
		var role string
		var tenantID int64
		var deletedAt string
		if err := a.db.QueryRow(`SELECT role,tenant_id,COALESCE(deleted_at,'') FROM app_users WHERE id=?`, q.ID).Scan(&role, &tenantID, &deletedAt); err != nil {
			writeError(w, errors.New("usuario no encontrado"), 404)
			return
		}
		if q.ID == admin.ID && (q.Action == "block" || q.Action == "delete") {
			writeError(w, errors.New("no puedes bloquear o eliminar tu propio usuario"), 400)
			return
		}
		if role == "superadmin" && q.ID != admin.ID && (q.Action == "block" || q.Action == "delete") {
			writeError(w, errors.New("otro superadministrador no puede bloquearse o eliminarse desde este panel"), 400)
			return
		}
		now := time.Now().UTC().Format(time.RFC3339)
		switch q.Action {
		case "block":
			_, err := a.db.Exec(`UPDATE app_users SET active=0,blocked_reason=?,updated_at=? WHERE id=?`, strings.TrimSpace(q.Reason), now, q.ID)
			if err != nil {
				writeError(w, err, 500)
				return
			}
			_, _ = a.db.Exec(`DELETE FROM app_sessions WHERE user_id=?`, q.ID)
		case "unblock":
			if deletedAt != "" {
				writeError(w, errors.New("restaura primero el usuario eliminado"), 400)
				return
			}
			_, err := a.db.Exec(`UPDATE app_users SET active=1,blocked_reason='',updated_at=? WHERE id=?`, now, q.ID)
			if err != nil {
				writeError(w, err, 500)
				return
			}
		case "delete":
			_, err := a.db.Exec(`UPDATE app_users SET active=0,deleted_at=?,blocked_reason=?,updated_at=? WHERE id=?`, now, firstNonEmpty(strings.TrimSpace(q.Reason), "Eliminado por superadministración"), now, q.ID)
			if err != nil {
				writeError(w, err, 500)
				return
			}
			_, _ = a.db.Exec(`DELETE FROM app_sessions WHERE user_id=?`, q.ID)
			if tenantID > 0 {
				var ownerID int64
				if a.db.QueryRow(`SELECT owner_user_id FROM tenants WHERE id=?`, tenantID).Scan(&ownerID) == nil && ownerID == q.ID {
					_, _ = a.db.Exec(`UPDATE tenants SET status='suspended',updated_at=? WHERE id=?`, now, tenantID)
				}
			}
		case "restore":
			_, err := a.db.Exec(`UPDATE app_users SET active=1,deleted_at='',blocked_reason='',updated_at=? WHERE id=?`, now, q.ID)
			if err != nil {
				writeError(w, err, 500)
				return
			}
			if tenantID > 0 {
				var ownerID int64
				if a.db.QueryRow(`SELECT owner_user_id FROM tenants WHERE id=?`, tenantID).Scan(&ownerID) == nil && ownerID == q.ID {
					_, _ = a.db.Exec(`UPDATE tenants SET status='active',updated_at=? WHERE id=?`, now, tenantID)
				}
			}
		case "update", "":
			q.Name = strings.TrimSpace(q.Name)
			q.Email = strings.ToLower(strings.TrimSpace(q.Email))
			q.Company = strings.TrimSpace(q.Company)
			allowedRoles := map[string]bool{"owner": true, "admin": true, "supervisor": true, "agent": true, "superadmin": true}
			if q.Name == "" || !strings.Contains(q.Email, "@") || !allowedRoles[q.Role] {
				writeError(w, errors.New("nombre, correo y rol válidos son obligatorios"), 400)
				return
			}
			if role == "superadmin" && q.Role != "superadmin" {
				writeError(w, errors.New("el rol del superadministrador principal no puede degradarse"), 400)
				return
			}
			if tenantID > 0 {
				var ownerID int64
				if a.db.QueryRow(`SELECT owner_user_id FROM tenants WHERE id=?`, tenantID).Scan(&ownerID) == nil && ownerID == q.ID && q.Role != "owner" {
					writeError(w, errors.New("el propietario de la empresa debe conservar el rol Propietario; transfiere primero la propiedad"), 400)
					return
				}
			}
			tx, err := a.db.Begin()
			if err != nil {
				writeError(w, err, 500)
				return
			}
			defer tx.Rollback()
			if _, err = tx.Exec(`UPDATE app_users SET name=?,email=?,role=?,company=?,updated_at=? WHERE id=?`, q.Name, q.Email, q.Role, q.Company, now, q.ID); err != nil {
				writeError(w, errors.New("no se pudo actualizar; verifica que el correo no esté en uso"), 400)
				return
			}
			if strings.TrimSpace(q.Password) != "" {
				if len(q.Password) < 8 {
					writeError(w, errors.New("la nueva contraseña debe tener mínimo 8 caracteres"), 400)
					return
				}
				if _, err = tx.Exec(`UPDATE app_users SET password_hash=? WHERE id=?`, hashPassword(q.Password, a.adminSalt()), q.ID); err != nil {
					writeError(w, err, 500)
					return
				}
				_, _ = tx.Exec(`DELETE FROM app_sessions WHERE user_id=?`, q.ID)
			}
			if tenantID > 0 {
				_, _ = tx.Exec(`UPDATE tenant_users SET role=? WHERE tenant_id=? AND user_id=?`, q.Role, tenantID, q.ID)
				var ownerID int64
				if tx.QueryRow(`SELECT owner_user_id FROM tenants WHERE id=?`, tenantID).Scan(&ownerID) == nil && ownerID == q.ID {
					if q.AccountType != "personal" && q.AccountType != "business" {
						q.AccountType = "business"
					}
					_, _ = tx.Exec(`UPDATE tenants SET name=?,account_type=?,updated_at=? WHERE id=?`, firstNonEmpty(q.Company, q.Name), q.AccountType, now, tenantID)
				}
			}
			if err = tx.Commit(); err != nil {
				writeError(w, err, 500)
				return
			}
		default:
			writeError(w, errors.New("acción no permitida"), 400)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	default:
		http.Error(w, "Método no permitido", 405)
	}
}

func (a *App) adminFullUsersList(w http.ResponseWriter, r *http.Request) {
	page, perPage, offset := adminPageValues(r)
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	role := strings.TrimSpace(r.URL.Query().Get("role"))
	plan := strings.TrimSpace(r.URL.Query().Get("plan"))
	clauses := []string{"1=1"}
	args := []any{}
	if search != "" {
		clauses = append(clauses, `(lower(u.name) LIKE ? OR lower(u.email) LIKE ? OR lower(u.company) LIKE ? OR lower(COALESCE(t.name,'')) LIKE ?)`)
		like := adminLike(search)
		args = append(args, like, like, like, like)
	}
	if role != "" && role != "all" {
		clauses = append(clauses, `u.role=?`)
		args = append(args, role)
	}
	switch status {
	case "active":
		clauses = append(clauses, `u.active=1 AND COALESCE(u.deleted_at,'')=''`)
	case "blocked":
		clauses = append(clauses, `u.active=0 AND COALESCE(u.deleted_at,'')=''`)
	case "deleted":
		clauses = append(clauses, `COALESCE(u.deleted_at,'')<>''`)
	default:
		clauses = append(clauses, `COALESCE(u.deleted_at,'')=''`)
	}
	if plan != "" && plan != "all" {
		clauses = append(clauses, `COALESCE((SELECT s.plan_code FROM billing_subscriptions s WHERE s.user_id=COALESCE(t.owner_user_id,u.id) AND s.status='active' AND s.ends_at>? ORDER BY s.ends_at DESC LIMIT 1),'free')=?`)
		args = append(args, time.Now().UTC().Format(time.RFC3339), plan)
	}
	where := strings.Join(clauses, " AND ")
	var total int
	countQuery := `SELECT COUNT(*) FROM app_users u LEFT JOIN tenants t ON t.id=u.tenant_id WHERE ` + where
	if err := a.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		writeError(w, err, 500)
		return
	}
	query := `SELECT u.id,u.name,u.email,u.role,u.company,u.active,u.created_at,u.tenant_id,COALESCE(u.deleted_at,''),COALESCE(u.blocked_reason,''),
COALESCE(t.name,u.company,''),COALESCE(t.account_type,'personal'),COALESCE(t.status,'active'),COALESCE(t.owner_user_id,u.id),
COALESCE((SELECT s.plan_code FROM billing_subscriptions s WHERE s.user_id=COALESCE(t.owner_user_id,u.id) AND s.status='active' AND s.ends_at>? ORDER BY s.ends_at DESC LIMIT 1),'free'),
COALESCE((SELECT p.name FROM billing_plans p WHERE p.code=COALESCE((SELECT s2.plan_code FROM billing_subscriptions s2 WHERE s2.user_id=COALESCE(t.owner_user_id,u.id) AND s2.status='active' AND s2.ends_at>? ORDER BY s2.ends_at DESC LIMIT 1),'free') LIMIT 1),'Free'),
COALESCE((SELECT s.ends_at FROM billing_subscriptions s WHERE s.user_id=COALESCE(t.owner_user_id,u.id) AND s.status='active' AND s.ends_at>? ORDER BY s.ends_at DESC LIMIT 1),'')
FROM app_users u LEFT JOIN tenants t ON t.id=u.tenant_id WHERE ` + where + ` ORDER BY u.id DESC LIMIT ? OFFSET ?`
	queryArgs := []any{time.Now().UTC().Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339)}
	queryArgs = append(queryArgs, args...)
	queryArgs = append(queryArgs, perPage, offset)
	rows, err := a.db.Query(query, queryArgs...)
	if err != nil {
		writeError(w, err, 500)
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, tenantID, ownerID int64
		var name, email, userRole, company, createdAt, deletedAt, blockedReason, tenantName, accountType, tenantStatus, planCode, planName, endsAt string
		var active int
		if err := rows.Scan(&id, &name, &email, &userRole, &company, &active, &createdAt, &tenantID, &deletedAt, &blockedReason, &tenantName, &accountType, &tenantStatus, &ownerID, &planCode, &planName, &endsAt); err != nil {
			continue
		}
		items = append(items, map[string]any{"id": id, "name": name, "email": email, "role": userRole, "company": company, "active": active == 1, "created_at": createdAt, "tenant_id": tenantID, "deleted_at": deletedAt, "blocked_reason": blockedReason, "tenant_name": tenantName, "account_type": accountType, "tenant_status": tenantStatus, "owner_user_id": ownerID, "is_owner": ownerID == id, "plan_code": planCode, "plan_name": planName, "plan_ends_at": endsAt})
	}
	writeJSON(w, map[string]any{"items": items, "pagination": paginationMeta(page, perPage, total)})
}

func (a *App) adminFullPlansHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireSuperadmin(w, r); !ok {
		return
	}
	if err := a.ensureAdminFullSchema(); err != nil {
		writeError(w, err, 500)
		return
	}
	if r.Method == http.MethodDelete {
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		var code string
		if id == 0 || a.db.QueryRow(`SELECT code FROM billing_plans WHERE id=?`, id).Scan(&code) != nil {
			writeError(w, errors.New("plan no encontrado"), 404)
			return
		}
		if code == "free" {
			writeError(w, errors.New("el plan Free es el plan de respaldo del sistema y no puede eliminarse"), 400)
			return
		}
		now := time.Now().UTC().Format(time.RFC3339)
		_, err := a.db.Exec(`UPDATE billing_plans SET active=0,deleted_at=? WHERE id=?`, now, id)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
		return
	}
	if r.Method == http.MethodPut && r.URL.Query().Get("action") == "restore" {
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		_, err := a.db.Exec(`UPDATE billing_plans SET deleted_at='',active=1 WHERE id=?`, id)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
		return
	}
	if r.Method != http.MethodGet {
		// Reutiliza la validación y escritura ya estable del administrador de planes.
		a.adminPlansHandler(w, r)
		return
	}
	page, perPage, offset := adminPageValues(r)
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	clauses := []string{"1=1"}
	args := []any{}
	if search != "" {
		clauses = append(clauses, `(lower(name) LIKE ? OR lower(code) LIKE ? OR lower(description) LIKE ?)`)
		like := adminLike(search)
		args = append(args, like, like, like)
	}
	switch status {
	case "active":
		clauses = append(clauses, `active=1 AND COALESCE(deleted_at,'')=''`)
	case "inactive":
		clauses = append(clauses, `active=0 AND COALESCE(deleted_at,'')=''`)
	case "deleted":
		clauses = append(clauses, `COALESCE(deleted_at,'')<>''`)
	default:
		clauses = append(clauses, `COALESCE(deleted_at,'')=''`)
	}
	where := strings.Join(clauses, " AND ")
	var total int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM billing_plans WHERE `+where, args...).Scan(&total); err != nil {
		writeError(w, err, 500)
		return
	}
	query := `SELECT id,code,name,description,price_usdt,billing_days,max_users,max_channels,max_contacts,max_ai_responses,max_products,max_rules,max_whatsapp,max_telegram,max_messenger,max_agents,features_json,active,COALESCE(deleted_at,''),
(SELECT COUNT(*) FROM billing_subscriptions s WHERE s.plan_code=billing_plans.code),
(SELECT COUNT(*) FROM billing_subscriptions s WHERE s.plan_code=billing_plans.code AND s.status='active' AND s.ends_at>?)
FROM billing_plans WHERE ` + where + ` ORDER BY price_usdt,id LIMIT ? OFFSET ?`
	queryArgs := []any{time.Now().UTC().Format(time.RFC3339)}
	queryArgs = append(queryArgs, args...)
	queryArgs = append(queryArgs, perPage, offset)
	rows, err := a.db.Query(query, queryArgs...)
	if err != nil {
		writeError(w, err, 500)
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id int64
		var code, name, desc, features, deletedAt string
		var price float64
		var days, users, channels, contacts, ai, products, rules, wa, tg, msg, agents, active, totalSubs, activeSubs int
		if rows.Scan(&id, &code, &name, &desc, &price, &days, &users, &channels, &contacts, &ai, &products, &rules, &wa, &tg, &msg, &agents, &features, &active, &deletedAt, &totalSubs, &activeSubs) != nil {
			continue
		}
		items = append(items, map[string]any{"id": id, "code": code, "name": name, "description": desc, "price_usdt": price, "billing_days": days, "max_users": users, "max_channels": channels, "max_contacts": contacts, "max_ai_responses": ai, "max_products": products, "max_rules": rules, "max_whatsapp": wa, "max_telegram": tg, "max_messenger": msg, "max_agents": agents, "features_json": features, "active": active == 1, "deleted_at": deletedAt, "subscriptions": totalSubs, "active_subscriptions": activeSubs})
	}
	writeJSON(w, map[string]any{"items": items, "pagination": paginationMeta(page, perPage, total)})
}

func (a *App) adminFullPaymentsHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireSuperadmin(w, r); !ok {
		return
	}
	if err := a.ensureAdminFullSchema(); err != nil {
		writeError(w, err, 500)
		return
	}
	if r.Method != http.MethodGet {
		a.adminPaymentsHandler(w, r)
		return
	}
	page, perPage, offset := adminPageValues(r)
	q := r.URL.Query()
	clauses := []string{"1=1"}
	args := []any{}
	if s := strings.TrimSpace(q.Get("search")); s != "" {
		like := adminLike(s)
		clauses = append(clauses, `(lower(u.name) LIKE ? OR lower(u.email) LIKE ? OR lower(bp.tx_hash) LIKE ? OR lower(bp.wallet) LIKE ?)`)
		args = append(args, like, like, like, like)
	}
	for _, filter := range []struct{ key, col string }{{"status", "bp.status"}, {"network", "bp.network"}, {"plan", "bp.plan_code"}} {
		if v := strings.TrimSpace(q.Get(filter.key)); v != "" && v != "all" {
			clauses = append(clauses, filter.col+"=?")
			args = append(args, v)
		}
	}
	if v := strings.TrimSpace(q.Get("from")); v != "" {
		clauses = append(clauses, `substr(bp.created_at,1,10)>=?`)
		args = append(args, v)
	}
	if v := strings.TrimSpace(q.Get("to")); v != "" {
		clauses = append(clauses, `substr(bp.created_at,1,10)<=?`)
		args = append(args, v)
	}
	where := strings.Join(clauses, " AND ")
	var total int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM billing_payments bp JOIN app_users u ON u.id=bp.user_id WHERE `+where, args...).Scan(&total); err != nil {
		writeError(w, err, 500)
		return
	}
	query := `SELECT bp.id,bp.user_id,u.name,u.email,bp.plan_code,COALESCE(p.name,bp.plan_code),bp.network,bp.wallet,bp.amount_usdt,bp.tx_hash,bp.status,bp.admin_note,bp.created_at,bp.reviewed_at FROM billing_payments bp JOIN app_users u ON u.id=bp.user_id LEFT JOIN billing_plans p ON p.code=bp.plan_code WHERE ` + where + ` ORDER BY CASE bp.status WHEN 'pending' THEN 0 ELSE 1 END,bp.id DESC LIMIT ? OFFSET ?`
	queryArgs := append(append([]any{}, args...), perPage, offset)
	rows, err := a.db.Query(query, queryArgs...)
	if err != nil {
		writeError(w, err, 500)
		return
	}
	defer rows.Close()
	items := []CryptoPayment{}
	for rows.Next() {
		var x CryptoPayment
		if rows.Scan(&x.ID, &x.UserID, &x.UserName, &x.UserEmail, &x.PlanCode, &x.PlanName, &x.Network, &x.Wallet, &x.AmountUSDT, &x.TxHash, &x.Status, &x.AdminNote, &x.CreatedAt, &x.ReviewedAt) == nil {
			items = append(items, x)
		}
	}
	var pending, approved, rejected int
	var revenue float64
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM billing_payments WHERE status='pending'`).Scan(&pending)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM billing_payments WHERE status='approved'`).Scan(&approved)
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM billing_payments WHERE status='rejected'`).Scan(&rejected)
	_ = a.db.QueryRow(`SELECT COALESCE(SUM(amount_usdt),0) FROM billing_payments WHERE status='approved'`).Scan(&revenue)
	writeJSON(w, map[string]any{"items": items, "pagination": paginationMeta(page, perPage, total), "summary": map[string]any{"pending": pending, "approved": approved, "rejected": rejected, "revenue_usdt": revenue}})
}

func (a *App) adminFullSubscriptionsHandler(w http.ResponseWriter, r *http.Request) {
	admin, ok := a.requireSuperadmin(w, r)
	if !ok {
		return
	}
	if err := a.ensureAdminFullSchema(); err != nil {
		writeError(w, err, 500)
		return
	}
	switch r.Method {
	case http.MethodGet:
		a.adminFullSubscriptionsList(w, r)
	case http.MethodPost:
		var q struct {
			UserID   int64  `json:"user_id"`
			PlanCode string `json:"plan_code"`
			Days     int    `json:"days"`
			EndsAt   string `json:"ends_at"`
			Note     string `json:"note"`
		}
		if err := json.NewDecoder(r.Body).Decode(&q); err != nil || q.UserID == 0 || strings.TrimSpace(q.PlanCode) == "" {
			writeError(w, errors.New("usuario y plan son obligatorios"), 400)
			return
		}
		var tenantID int64
		var deletedAt string
		if err := a.db.QueryRow(`SELECT tenant_id,COALESCE(deleted_at,'') FROM app_users WHERE id=?`, q.UserID).Scan(&tenantID, &deletedAt); err != nil || deletedAt != "" {
			writeError(w, errors.New("usuario no disponible"), 404)
			return
		}
		billingUserID := q.UserID
		if tenantID > 0 {
			_ = a.db.QueryRow(`SELECT owner_user_id FROM tenants WHERE id=?`, tenantID).Scan(&billingUserID)
		}
		var planExists int
		if a.db.QueryRow(`SELECT COUNT(*) FROM billing_plans WHERE code=? AND COALESCE(deleted_at,'')=''`, q.PlanCode).Scan(&planExists) != nil || planExists == 0 {
			writeError(w, errors.New("plan inválido"), 400)
			return
		}
		now := time.Now().UTC()
		end := time.Time{}
		if strings.TrimSpace(q.EndsAt) != "" {
			var err error
			end, err = time.Parse("2006-01-02", q.EndsAt)
			if err != nil {
				writeError(w, errors.New("fecha final inválida"), 400)
				return
			}
			end = end.Add(23*time.Hour + 59*time.Minute + 59*time.Second).UTC()
		} else {
			if q.Days < 1 {
				q.Days = 30
			}
			if q.Days > 3650 {
				q.Days = 3650
			}
			end = now.AddDate(0, 0, q.Days)
		}
		if !end.After(now) {
			writeError(w, errors.New("la fecha final debe ser posterior a hoy"), 400)
			return
		}
		tx, err := a.db.Begin()
		if err != nil {
			writeError(w, err, 500)
			return
		}
		defer tx.Rollback()
		if _, err = tx.Exec(`UPDATE billing_subscriptions SET status='replaced' WHERE user_id=? AND status='active'`, billingUserID); err != nil {
			writeError(w, err, 500)
			return
		}
		res, err := tx.Exec(`INSERT INTO billing_subscriptions(user_id,plan_code,status,starts_at,ends_at,created_at,source,admin_note,granted_by) VALUES(?,?, 'active',?,?,?,?,?,?)`, billingUserID, q.PlanCode, now.Format(time.RFC3339), end.Format(time.RFC3339), now.Format(time.RFC3339), "courtesy", strings.TrimSpace(q.Note), admin.ID)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		id, _ := res.LastInsertId()
		if err = tx.Commit(); err != nil {
			writeError(w, err, 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "id": id, "billing_user_id": billingUserID, "ends_at": end.Format(time.RFC3339)})
	case http.MethodPut:
		var q struct {
			ID     int64  `json:"id"`
			Action string `json:"action"`
			Days   int    `json:"days"`
			Note   string `json:"note"`
		}
		if json.NewDecoder(r.Body).Decode(&q) != nil || q.ID == 0 {
			writeError(w, errors.New("suscripción inválida"), 400)
			return
		}
		switch q.Action {
		case "cancel":
			_, err := a.db.Exec(`UPDATE billing_subscriptions SET status='cancelled',admin_note=CASE WHEN ?<>'' THEN ? ELSE admin_note END WHERE id=?`, strings.TrimSpace(q.Note), strings.TrimSpace(q.Note), q.ID)
			if err != nil {
				writeError(w, err, 500)
				return
			}
		case "extend":
			if q.Days < 1 {
				writeError(w, errors.New("define los días adicionales"), 400)
				return
			}
			var userID int64
			var endsAt string
			if a.db.QueryRow(`SELECT user_id,ends_at FROM billing_subscriptions WHERE id=?`, q.ID).Scan(&userID, &endsAt) != nil {
				writeError(w, errors.New("suscripción no encontrada"), 404)
				return
			}
			end, err := time.Parse(time.RFC3339, endsAt)
			if err != nil {
				writeError(w, err, 500)
				return
			}
			base := end
			if base.Before(time.Now().UTC()) {
				base = time.Now().UTC()
			}
			tx, err := a.db.Begin()
			if err != nil {
				writeError(w, err, 500)
				return
			}
			defer tx.Rollback()
			if _, err = tx.Exec(`UPDATE billing_subscriptions SET status='replaced' WHERE user_id=? AND status='active' AND id<>?`, userID, q.ID); err != nil {
				writeError(w, err, 500)
				return
			}
			if _, err = tx.Exec(`UPDATE billing_subscriptions SET ends_at=?,status='active',admin_note=CASE WHEN ?<>'' THEN ? ELSE admin_note END WHERE id=?`, base.AddDate(0, 0, q.Days).Format(time.RFC3339), strings.TrimSpace(q.Note), strings.TrimSpace(q.Note), q.ID); err != nil {
				writeError(w, err, 500)
				return
			}
			if err = tx.Commit(); err != nil {
				writeError(w, err, 500)
				return
			}
		default:
			writeError(w, errors.New("acción no permitida"), 400)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	default:
		http.Error(w, "Método no permitido", 405)
	}
}

func (a *App) adminFullSubscriptionsList(w http.ResponseWriter, r *http.Request) {
	page, perPage, offset := adminPageValues(r)
	q := r.URL.Query()
	clauses := []string{"1=1"}
	args := []any{}
	if s := strings.TrimSpace(q.Get("search")); s != "" {
		like := adminLike(s)
		clauses = append(clauses, `(lower(u.name) LIKE ? OR lower(u.email) LIKE ? OR lower(u.company) LIKE ?)`)
		args = append(args, like, like, like)
	}
	for _, filter := range []struct{ key, col string }{{"status", "s.status"}, {"source", "s.source"}, {"plan", "s.plan_code"}} {
		if v := strings.TrimSpace(q.Get(filter.key)); v != "" && v != "all" {
			clauses = append(clauses, filter.col+"=?")
			args = append(args, v)
		}
	}
	where := strings.Join(clauses, " AND ")
	var total int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM billing_subscriptions s JOIN app_users u ON u.id=s.user_id WHERE `+where, args...).Scan(&total); err != nil {
		writeError(w, err, 500)
		return
	}
	query := `SELECT s.id,s.user_id,u.name,u.email,u.company,s.plan_code,COALESCE(p.name,s.plan_code),s.status,s.starts_at,s.ends_at,s.created_at,COALESCE(s.source,'system'),COALESCE(s.admin_note,''),COALESCE(s.granted_by,0),COALESCE(g.name,'') FROM billing_subscriptions s JOIN app_users u ON u.id=s.user_id LEFT JOIN billing_plans p ON p.code=s.plan_code LEFT JOIN app_users g ON g.id=s.granted_by WHERE ` + where + ` ORDER BY s.id DESC LIMIT ? OFFSET ?`
	queryArgs := append(append([]any{}, args...), perPage, offset)
	rows, err := a.db.Query(query, queryArgs...)
	if err != nil {
		writeError(w, err, 500)
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, userID, grantedBy int64
		var name, email, company, planCode, planName, status, startsAt, endsAt, createdAt, source, note, grantedByName string
		if rows.Scan(&id, &userID, &name, &email, &company, &planCode, &planName, &status, &startsAt, &endsAt, &createdAt, &source, &note, &grantedBy, &grantedByName) != nil {
			continue
		}
		items = append(items, map[string]any{"id": id, "user_id": userID, "user_name": name, "user_email": email, "company": company, "plan_code": planCode, "plan_name": planName, "status": status, "starts_at": startsAt, "ends_at": endsAt, "created_at": createdAt, "source": source, "admin_note": note, "granted_by": grantedBy, "granted_by_name": grantedByName})
	}
	writeJSON(w, map[string]any{"items": items, "pagination": paginationMeta(page, perPage, total)})
}

// Asegura que los datos enviados por filtros no se conviertan en SQL dinámico.
func adminAllowedSort(value string, allowed map[string]string, fallback string) string {
	if v, ok := allowed[value]; ok {
		return v
	}
	return fallback
}

// Evita que la importación database/sql sea eliminada por herramientas que compilan
// solo este archivo durante las verificaciones reducidas.
var _ = sql.ErrNoRows
var _ = fmt.Sprintf
