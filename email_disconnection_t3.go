package main

import "time"

// t3SoftRemoveEmailAccount retira UNA cuenta de la lista operativa, sin borrar
// el registro de cuenta ni los mensajes que referencian su ID histórico.
// Los tokens se invalidan localmente y la recepción multiempresa vuelve a OFF,
// incluso si posteriormente se reconecta la misma cuenta por OAuth.
func t3SoftRemoveEmailAccount(db *DB, tenantID, accountID int64) (bool, error) {
	if tenantID <= 0 || accountID <= 0 {
		return false, nil
	}
	if err := emailP1D3Schema(db); err != nil {
		return false, err
	}
	tx, err := db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := tx.Exec(`UPDATE email_p1_accounts
SET encrypted_token='', token_expiry='', status='removed', last_error='', updated_at=?
WHERE tenant_id=? AND id=? AND status<>'removed'`, now, tenantID, accountID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	if n != 1 {
		return false, nil
	}
	_, err = tx.Exec(`UPDATE email_p1d3_accounts
SET sync_enabled=0, enabled_at='', updated_at=?
WHERE tenant_id=? AND account_id=?`, now, tenantID, accountID)
	if err != nil {
		return false, err
	}
	if err = tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}
