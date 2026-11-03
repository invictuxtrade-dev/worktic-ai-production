package main

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

const workticLegalVersionV273600 = "2026-11-03-v1"

func workticLegalRegistrationValidV273600(accepted, authorized bool, privacyVersion, termsVersion string) error {
	if !accepted || !authorized {
		return errors.New("para crear la cuenta debes aceptar los Términos y autorizar el tratamiento informado de tus datos")
	}
	if strings.TrimSpace(privacyVersion) != workticLegalVersionV273600 || strings.TrimSpace(termsVersion) != workticLegalVersionV273600 {
		return errors.New("la política de privacidad o los términos han cambiado; actualiza la página para continuar")
	}
	return nil
}

func (a *App) ensureWorkticLegalSchemaV273600() error {
	_, err := a.db.Exec(`CREATE TABLE IF NOT EXISTS worktic_legal_acceptances_v273600 (
      user_id BIGINT PRIMARY KEY,
      tenant_id BIGINT NOT NULL,
      terms_version VARCHAR(40) NOT NULL,
      privacy_version VARCHAR(40) NOT NULL,
      accepted_at VARCHAR(40) NOT NULL,
      privacy_authorized BOOLEAN NOT NULL,
      marketing_opt_in BOOLEAN NOT NULL,
      source VARCHAR(64) NOT NULL
    )`)
	return err
}

type workticLegalExecerV273600 interface {
	Exec(string, ...any) (sql.Result, error)
}

func workticLegalInsertAcceptanceV273600(tx workticLegalExecerV273600, userID, tenantID int64, marketingOptIn bool, at time.Time) error {
	_, err := tx.Exec(`INSERT INTO worktic_legal_acceptances_v273600
      (user_id, tenant_id, terms_version, privacy_version, accepted_at, privacy_authorized, marketing_opt_in, source)
      VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, userID, tenantID, workticLegalVersionV273600,
		workticLegalVersionV273600, at.UTC().Format(time.RFC3339), true, marketingOptIn, "web_register")
	return err
}
