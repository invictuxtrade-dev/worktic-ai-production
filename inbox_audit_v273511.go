package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

// V27.35.11: reversible, tenant-scoped removal. Messages and contacts are not
// destroyed; the ledger and tombstones remain available for an authorized review.
func initInboxAuditV273511Schema(db *DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS inbox_removed_v273511 (
 tenant_id INTEGER NOT NULL, chat_jid TEXT NOT NULL,
 deleted_by INTEGER NOT NULL, deleted_by_name TEXT NOT NULL DEFAULT '',
 reason TEXT NOT NULL, deleted_at TEXT NOT NULL,
 PRIMARY KEY (tenant_id,chat_jid)
);
CREATE INDEX IF NOT EXISTS idx_inbox_removed_v273511_date ON inbox_removed_v273511(tenant_id,deleted_at DESC);
CREATE TABLE IF NOT EXISTS inbox_audit_v273511 (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 tenant_id INTEGER NOT NULL, chat_jid TEXT NOT NULL,
 actor_id INTEGER NOT NULL, actor_name TEXT NOT NULL DEFAULT '',
 action TEXT NOT NULL, before_json TEXT NOT NULL DEFAULT '{}',
 after_json TEXT NOT NULL DEFAULT '{}', reason TEXT NOT NULL DEFAULT '',
 created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_inbox_audit_v273511_tenant_chat ON inbox_audit_v273511(tenant_id,chat_jid,id DESC);
`)
	return err
}

func inboxManager(u *User) bool {
	return u != nil && (u.Role == "superadmin" || u.Role == "owner" || u.Role == "admin")
}

func (a *App) inboxIsRemoved(tid int64, chat string) (bool, error) {
	var n int
	err := a.db.QueryRow(`SELECT COUNT(*) FROM inbox_removed_v273511 WHERE tenant_id=? AND chat_jid=?`, tid, chat).Scan(&n)
	return n > 0, err
}

func (a *App) inboxVisible(tid int64, chat string) bool {
	if tid <= 0 || strings.TrimSpace(chat) == "" {
		return false
	}
	var n int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM worktic_contacts c WHERE c.tenant_id=? AND c.chat_jid=? AND NOT EXISTS (SELECT 1 FROM inbox_removed_v273511 d WHERE d.tenant_id=c.tenant_id AND d.chat_jid=c.chat_jid)`, tid, chat).Scan(&n); err != nil {
		return false
	}
	return n > 0
}

func (a *App) inboxAuditTx(tx *Tx, tid int64, chat string, u *User, action string, before, after any, reason string) error {
	b, e := json.Marshal(before)
	if e != nil {
		return e
	}
	n, e := json.Marshal(after)
	if e != nil {
		return e
	}
	_, e = tx.Exec(`INSERT INTO inbox_audit_v273511(tenant_id,chat_jid,actor_id,actor_name,action,before_json,after_json,reason,created_at) VALUES(?,?,?,?,?,?,?,?,?)`,
		tid, chat, u.ID, u.Name, action, string(b), string(n), reason, time.Now().UTC().Format(time.RFC3339Nano))
	return e
}

func (a *App) inboxRemoveHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	tid, u, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	if !inboxManager(u) {
		writeError(w, errors.New("solo administradores pueden eliminar conversaciones"), 403)
		return
	}
	var q struct {
		Chat   string `json:"chat"`
		Reason string `json:"reason"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&q) != nil {
		writeError(w, errors.New("solicitud inválida"), 400)
		return
	}
	q.Chat = strings.TrimSpace(q.Chat)
	q.Reason = strings.TrimSpace(q.Reason)
	if q.Chat == "" || len([]rune(q.Reason)) < 5 || len([]rune(q.Reason)) > 300 {
		writeError(w, errors.New("indica la conversación y un motivo de 5 a 300 caracteres"), 400)
		return
	}
	tx, err := a.db.Begin()
	if err != nil {
		writeError(w, err, 500)
		return
	}
	defer tx.Rollback()
	var name, channel string
	var messageCount int
	err = tx.QueryRow(`SELECT c.name,c.channel,(SELECT COUNT(*) FROM worktic_messages m WHERE m.tenant_id=c.tenant_id AND m.chat_jid=c.chat_jid) FROM worktic_contacts c WHERE c.tenant_id=? AND c.chat_jid=?`, tid, q.Chat).Scan(&name, &channel, &messageCount)
	if err == sql.ErrNoRows {
		writeError(w, errors.New("conversación no encontrada en tu empresa"), 404)
		return
	}
	if err != nil {
		writeError(w, err, 500)
		return
	}
	var n int
	if err = tx.QueryRow(`SELECT COUNT(*) FROM inbox_removed_v273511 WHERE tenant_id=? AND chat_jid=?`, tid, q.Chat).Scan(&n); err != nil {
		writeError(w, err, 500)
		return
	}
	if n != 0 {
		writeError(w, errors.New("conversación ya eliminada"), 409)
		return
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err = tx.Exec(`INSERT INTO inbox_removed_v273511(tenant_id,chat_jid,deleted_by,deleted_by_name,reason,deleted_at) VALUES(?,?,?,?,?,?)`, tid, q.Chat, u.ID, u.Name, q.Reason, now); err != nil {
		writeError(w, err, 500)
		return
	}
	if err = a.inboxAuditTx(tx, tid, q.Chat, u, "conversation.removed", map[string]any{"name": name, "channel": channel, "messages_preserved": messageCount}, map[string]any{"hidden_from_inbox": true}, q.Reason); err != nil {
		writeError(w, err, 500)
		return
	}
	if err = tx.Commit(); err != nil {
		writeError(w, err, 500)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "messages_preserved": messageCount, "message": "Conversación retirada de la bandeja. Evidencia conservada."})
}

func (a *App) inboxRestoreHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	tid, u, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	if !inboxManager(u) {
		writeError(w, errors.New("solo administradores pueden restaurar conversaciones"), 403)
		return
	}
	var q struct {
		Chat   string `json:"chat"`
		Reason string `json:"reason"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&q) != nil {
		writeError(w, errors.New("solicitud inválida"), 400)
		return
	}
	q.Chat = strings.TrimSpace(q.Chat)
	q.Reason = strings.TrimSpace(q.Reason)
	if q.Chat == "" || len([]rune(q.Reason)) < 5 || len([]rune(q.Reason)) > 300 {
		writeError(w, errors.New("motivo de restauración de 5 a 300 caracteres requerido"), 400)
		return
	}
	tx, err := a.db.Begin()
	if err != nil {
		writeError(w, err, 500)
		return
	}
	defer tx.Rollback()
	var who, when string
	if err = tx.QueryRow(`SELECT deleted_by_name,deleted_at FROM inbox_removed_v273511 WHERE tenant_id=? AND chat_jid=?`, tid, q.Chat).Scan(&who, &when); err == sql.ErrNoRows {
		writeError(w, errors.New("conversación eliminada no encontrada"), 404)
		return
	} else if err != nil {
		writeError(w, err, 500)
		return
	}
	if _, err = tx.Exec(`DELETE FROM inbox_removed_v273511 WHERE tenant_id=? AND chat_jid=?`, tid, q.Chat); err != nil {
		writeError(w, err, 500)
		return
	}
	if err = a.inboxAuditTx(tx, tid, q.Chat, u, "conversation.restored", map[string]any{"hidden_from_inbox": true, "deleted_by": who, "deleted_at": when}, map[string]any{"hidden_from_inbox": false}, q.Reason); err != nil {
		writeError(w, err, 500)
		return
	}
	if err = tx.Commit(); err != nil {
		writeError(w, err, 500)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (a *App) inboxRemovedListHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", 405)
		return
	}
	tid, u, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	if !inboxManager(u) {
		writeError(w, errors.New("acceso reservado a administradores"), 403)
		return
	}
	rows, err := a.db.Query(`SELECT d.chat_jid,COALESCE(c.name,''),d.deleted_by_name,d.reason,d.deleted_at FROM inbox_removed_v273511 d LEFT JOIN worktic_contacts c ON c.tenant_id=d.tenant_id AND c.chat_jid=d.chat_jid WHERE d.tenant_id=? ORDER BY d.deleted_at DESC LIMIT 100`, tid)
	if err != nil {
		writeError(w, err, 500)
		return
	}
	defer rows.Close()
	result := []map[string]string{}
	for rows.Next() {
		var chat, name, by, reason, at string
		if err = rows.Scan(&chat, &name, &by, &reason, &at); err != nil {
			writeError(w, err, 500)
			return
		}
		result = append(result, map[string]string{"chat": chat, "name": name, "deleted_by": by, "reason": reason, "deleted_at": at})
	}
	if err = rows.Err(); err != nil {
		writeError(w, err, 500)
		return
	}
	writeJSON(w, result)
}

func (a *App) inboxAuditHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", 405)
		return
	}
	tid, u, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	if !inboxManager(u) {
		writeError(w, errors.New("acceso reservado a administradores"), 403)
		return
	}
	chat := strings.TrimSpace(r.URL.Query().Get("chat"))
	if chat == "" {
		writeError(w, errors.New("chat obligatorio"), 400)
		return
	}
	// Any historic audit entry can be viewed, even after the chat was removed.
	var exists int
	if err = a.db.QueryRow(`SELECT (SELECT COUNT(*) FROM worktic_contacts WHERE tenant_id=? AND chat_jid=?) + (SELECT COUNT(*) FROM inbox_audit_v273511 WHERE tenant_id=? AND chat_jid=?)`, tid, chat, tid, chat).Scan(&exists); err != nil {
		writeError(w, err, 500)
		return
	}
	if exists == 0 {
		writeError(w, errors.New("conversación no encontrada"), 404)
		return
	}
	rows, err := a.db.Query(`SELECT action,actor_name,reason,before_json,after_json,created_at FROM inbox_audit_v273511 WHERE tenant_id=? AND chat_jid=? ORDER BY id DESC LIMIT 200`, tid, chat)
	if err != nil {
		writeError(w, err, 500)
		return
	}
	defer rows.Close()
	result := []map[string]string{}
	for rows.Next() {
		var action, actor, reason, before, after, at string
		if err = rows.Scan(&action, &actor, &reason, &before, &after, &at); err != nil {
			writeError(w, err, 500)
			return
		}
		result = append(result, map[string]string{"action": action, "actor": actor, "reason": reason, "before": before, "after": after, "created_at": at})
	}
	if err = rows.Err(); err != nil {
		writeError(w, err, 500)
		return
	}
	writeJSON(w, result)
}
