package main

// LinkedIn L2: incrementally authorize Share on LinkedIn for an existing
// connected member. It never swaps credentials or author identity on failure.
import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const linkedinL2Scope = "openid profile w_member_social"
const linkedinL2StoredScopes = "openid,profile,w_member_social"

var linkedinL2PersonIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

func linkedinL2VerifyReturnedScopes(scope string) error {
	if !linkedinL1HasScope(scope, "openid") || !linkedinL1HasScope(scope, "profile") || !linkedinL1HasScope(scope, "w_member_social") {
		// Fail closed: never claim a publish grant from an omitted/unverified response.
		return errors.New("LinkedIn no confirmo el permiso w_member_social en el token; el perfil anterior permanece conectado")
	}
	return nil
}

func linkedinL2PersonURN(id string) (string, error) {
	id = strings.TrimSpace(id)
	if !linkedinL2PersonIDPattern.MatchString(id) {
		return "", errors.New("LinkedIn devolvio un identificador de autor invalido")
	}
	return "urn:li:person:" + id, nil
}

// L2 is intentionally text/link-only. LinkedIn images/videos require an actual
// media upload workflow; silently dropping attachments would mislead users.
func linkedinL2ValidatePublishRequest(tokenScopes, connectionScopes, urn, format, mediaJSON string) error {
	if !linkedinL1HasScope(connectionScopes, "w_member_social") || !linkedinL1HasScope(tokenScopes, "w_member_social") {
		return errors.New("LinkedIn: el token no tiene permiso de publicacion. Autoriza Share on LinkedIn")
	}
	if !strings.HasPrefix(urn, "urn:li:person:") {
		return errors.New("LinkedIn: falta identificador de autor verificado; vuelve a autorizar publicaciones")
	}
	if _, err := linkedinL2PersonURN(strings.TrimPrefix(urn, "urn:li:person:")); err != nil {
		return err
	}
	f := strings.ToLower(strings.TrimSpace(format))
	if f == "video" || f == "reel" || f == "carousel" || f == "story" {
		return errors.New("LinkedIn L2 solo admite texto y enlaces; los videos requieren la siguiente fase")
	}
	var attachments []map[string]any
	if strings.TrimSpace(mediaJSON) != "" {
		if err := json.Unmarshal([]byte(mediaJSON), &attachments); err != nil {
			return errors.New("LinkedIn: medios adjuntos invalidos")
		}
		for _, item := range attachments {
			if url, ok := item["url"].(string); ok && strings.TrimSpace(url) != "" {
				return errors.New("LinkedIn L2: no se publicara un contenido multimedia sin subirlo realmente; por ahora usa texto o enlace")
			}
		}
	}
	return nil
}

func (a *App) linkedinL2EnsureSchema() error {
	_, err := a.db.Exec(`CREATE TABLE IF NOT EXISTS social_linkedin_publish_grants (
        state TEXT PRIMARY KEY,
        tenant_id INTEGER NOT NULL,
        connection_id INTEGER NOT NULL,
        expected_account_id TEXT NOT NULL,
        expires_at TEXT NOT NULL
    )`)
	return err
}

func (a *App) linkedinL2ValidateTarget(tid, connID int64) (string, error) {
	if err := a.linkedinL2EnsureSchema(); err != nil {
		return "", err
	}
	var externalID string
	err := a.db.QueryRow(`SELECT external_account_id FROM social_connections
        WHERE id=? AND tenant_id=? AND platform='linkedin' AND status='connected' AND provider_mode='official'`, connID, tid).Scan(&externalID)
	if err != nil || strings.TrimSpace(externalID) == "" {
		return "", errors.New("LinkedIn: no se encontro el perfil conectado de esta empresa")
	}
	return externalID, nil
}

func (a *App) linkedinL2StoreIntent(state string, tid, connID int64, expected string) error {
	if err := a.linkedinL2EnsureSchema(); err != nil {
		return err
	}
	_, _ = a.db.Exec(`DELETE FROM social_linkedin_publish_grants WHERE expires_at<?`, time.Now().UTC().Format(time.RFC3339))
	_, err := a.db.Exec(`INSERT INTO social_linkedin_publish_grants(state,tenant_id,connection_id,expected_account_id,expires_at)
        VALUES(?,?,?,?,?)`, state, tid, connID, expected, time.Now().UTC().Add(15*time.Minute).Format(time.RFC3339))
	return err
}

func (a *App) linkedinL2FinishGrant(ctx context.Context, state string, tid int64, tok socialToken) error {
	if err := a.linkedinL2EnsureSchema(); err != nil {
		return err
	}
	var storedTenant, connID int64
	var expectedID string
	// Consumed once; a failed grant never updates or disconnects the old account.
	err := a.db.QueryRow(`DELETE FROM social_linkedin_publish_grants WHERE state=? AND expires_at>?
        RETURNING tenant_id,connection_id,expected_account_id`, state, time.Now().UTC().Format(time.RFC3339)).Scan(&storedTenant, &connID, &expectedID)
	if errors.Is(err, sql.ErrNoRows) {
		return errors.New("LinkedIn: autorizacion L2 vencida o utilizada; vuelve a intentarlo")
	}
	if err != nil {
		return err
	}
	if storedTenant != tid {
		return errors.New("LinkedIn: autorizacion no pertenece a esta empresa")
	}
	if err := linkedinL2VerifyReturnedScopes(tok.Scope); err != nil {
		return err
	}
	id, name, err := linkedinL1DiscoverProfile(ctx, tok.AccessToken)
	if err != nil {
		return err
	}
	if id != expectedID {
		return errors.New("LinkedIn: se autorizo otro perfil; la conexion anterior no fue modificada")
	}
	author, err := linkedinL2PersonURN(id)
	if err != nil {
		return err
	}
	cfg, err := json.Marshal(socialProviderConfig{AuthorURN: author})
	if err != nil {
		return err
	}
	raw, err := json.Marshal(tok)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	expiry := ""
	if tok.ExpiresIn > 0 {
		expiry = now.Add(time.Duration(tok.ExpiresIn) * time.Second).Format(time.RFC3339)
	}
	res, err := a.db.Exec(`UPDATE social_connections SET account_name=?,scopes=?,config_json=?,encrypted_credentials=?,
        token_expires_at=?,last_sync_at=?,last_error='',updated_at=?
        WHERE id=? AND tenant_id=? AND platform='linkedin' AND provider_mode='official' AND status='connected' AND external_account_id=?`,
		name, linkedinL2StoredScopes, string(cfg), encryptLocal(string(raw), a.cfg.ChannelEncryptionKey), expiry, now.Format(time.RFC3339), now.Format(time.RFC3339), connID, tid, id)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil || affected != 1 {
		return fmt.Errorf("LinkedIn: el perfil conectado cambio durante OAuth; no se actualizo: %v", err)
	}
	return nil
}
