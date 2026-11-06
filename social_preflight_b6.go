package main

import (
	"fmt"
	"strings"
)

// socialB6PublishPrerequisites validates only publish and schedule operations.
// Drafts remain available even when the official connection is incomplete.
func (a *App) socialB6PublishPrerequisites(tenantID int64, platforms []string) error {
	visited := map[string]bool{}
	for _, raw := range platforms {
		platform := strings.ToLower(strings.TrimSpace(raw))
		if !validSocialPlatform(platform) {
			return fmt.Errorf("red social no compatible: %s", platform)
		}
		if visited[platform] {
			continue
		}
		visited[platform] = true
		var mode, scopes string
		err := a.db.QueryRow(`SELECT provider_mode, scopes FROM social_connections WHERE tenant_id=? AND platform=? AND status='connected' ORDER BY id LIMIT 1`, tenantID, platform).Scan(&mode, &scopes)
		if err != nil {
			return fmt.Errorf("%s: conecta una cuenta activa en Redes Sociales > Cuentas antes de publicar o programar", platform)
		}
		if mode == "sandbox" {
			continue
		}
		if platform == "facebook" && !socialB6HasScope(scopes, "pages_manage_posts") {
			return fmt.Errorf("Facebook: falta el permiso pages_manage_posts para publicar en la página. Puedes guardar el borrador")
		}
		if platform == "instagram" && !socialB6HasScope(scopes, "instagram_content_publish") {
			return fmt.Errorf("Instagram: falta el permiso instagram_content_publish. Puedes guardar el borrador")
		}
	}
	return nil
}

func socialB6HasScope(scopes, wanted string) bool {
	return strings.Contains(","+strings.ToLower(strings.ReplaceAll(scopes, " ", ""))+",", ","+strings.ToLower(wanted)+",")
}
