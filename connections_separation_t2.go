package main

import "time"

// t2SoftRemoveSocialConnection revoca las credenciales locales de UNA empresa,
// oculta la cuenta de la lista operativa y pausa las publicaciones pendientes.
// No borra registros de publicaciones, contactos ni conversaciones.
// Se conserva (tenant, platform, external_account_id) para que OAuth pueda
// reutilizar el ID histórico al autorizar de nuevo la misma cuenta.
func t2SoftRemoveSocialConnection(db *DB, tenantID, connectionID int64) (bool, error) {
	if tenantID <= 0 || connectionID <= 0 {
		return false, nil
	}
	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`UPDATE social_connections
SET status='removed', encrypted_credentials='', config_json='{}', scopes='',
    token_expires_at='', refresh_status='', last_error='', updated_at=?
WHERE tenant_id=? AND id=? AND status<>'removed'`, now, tenantID, connectionID)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	if affected == 0 {
		return false, nil
	}
	// El scheduler solo procesa queued/scheduled/retrying; se bloquean sin
	// modificar el histórico published ni los borradores de otras cuentas.
	_, err = tx.Exec(`UPDATE social_posts
SET status='paused_connection',error_message='Conexión desvinculada por el administrador',updated_at=?
WHERE tenant_id=? AND connection_id=? AND status IN ('scheduled','queued','retrying','draft')`, now, tenantID, connectionID)
	if err != nil {
		return false, err
	}
	if err = tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}
