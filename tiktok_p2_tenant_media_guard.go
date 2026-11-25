package main

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// tiktokP2TenantMediaPath rejects cross-tenant paths and arbitrary external URLs.
// TikTok P2 only supports videos uploaded to this tenant's Social Hub library.
// The asset's PUBLIC URL is for provider retrieval; database ownership is still enforced.
func tiktokP2TenantMediaPath(tenantID int64, rawURL, configuredPrefix string) (assetURL, filename string, err error) {
	invalid := errors.New("El video de TikTok debe ser un MP4 propio de esta empresa, cargado en Multimedia")
	if tenantID <= 0 || rawURL == "" {
		return "", "", invalid
	}
	u, parseErr := url.Parse(strings.TrimSpace(rawURL))
	if parseErr != nil || u == nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
		return "", "", invalid
	}
	// TikTok requires a nonredirecting HTTPS URL. Worktic's canonical public origin is www.
	// An explicitly configured verified media prefix can override the canonical host.
	validHost := "www.workticai.com"
	if configuredPrefix != "" {
		p, e := url.Parse(strings.TrimSpace(configuredPrefix))
		if e != nil || p == nil || p.Scheme != "https" || p.Host == "" || p.User != nil || p.RawQuery != "" || p.Fragment != "" {
			return "", "", errors.New("El prefijo de medios TikTok no esta configurado correctamente")
		}
		validHost = p.Host
	}
	if !strings.EqualFold(u.Host, validHost) || u.RawPath != "" || strings.Contains(u.EscapedPath(), "%") {
		return "", "", invalid
	}
	pathPrefix := "/uploads/social/" + strconv.FormatInt(tenantID, 10) + "/"
	if !strings.HasPrefix(u.Path, pathPrefix) {
		return "", "", invalid
	}
	filename = strings.TrimPrefix(u.Path, pathPrefix)
	if filename == "" || strings.HasPrefix(filename, ".") || filename == "." || filename == ".." || strings.ContainsAny(filename, "/\\") || filepath.Base(filename) != filename || !strings.EqualFold(filepath.Ext(filename), ".mp4") {
		return "", "", invalid
	}
	// No backtracking or odd path syntax, even when not percent-encoded.
	for _, char := range filename {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '.' || char == '_' || char == '-') {
			return "", "", invalid
		}
	}
	return u.Path, filename, nil
}

// tiktokP2OwnedVideoCheck makes the posting boundary fail closed. The URL's
// filename and tenant ID must match a registered uploaded video AND a real file.
func (a *App) tiktokP2OwnedVideoCheck(tenantID int64, videoURL string) (int64, error) {
	path, filename, err := tiktokP2TenantMediaPath(tenantID, videoURL, os.Getenv("WORKTIC_TIKTOK_VERIFIED_MEDIA_PREFIX"))
	if err != nil {
		return 0, err
	}
	var count int
	err = a.db.QueryRow(`SELECT COUNT(*) FROM social_media_assets WHERE tenant_id=? AND url=? AND media_type='video' AND source='upload'`, tenantID, path).Scan(&count)
	if err != nil || count != 1 {
		return 0, errors.New("No se encontro un video propio registrado en Multimedia para esta empresa")
	}
	filePath := filepath.Join(a.cfg.DataDir, "social_uploads", strconv.FormatInt(tenantID, 10), filename)
	info, err := os.Stat(filePath)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 {
		return 0, fmt.Errorf("El video seleccionado no esta disponible en el almacenamiento de esta empresa")
	}
	return info.Size(), nil
}
