package main

// Worktic Studio V27.19: optional native text/image -> video through the
// Luma Agents API. All jobs and assets are scoped to the authenticated tenant.
// The API key is never sent to the browser.
import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func initStudioVideoV2719Schema(db *DB) error {
	for _, sql := range []string{
		`CREATE TABLE IF NOT EXISTS studio_video_jobs_v2719 (id TEXT PRIMARY KEY, tenant_id INTEGER NOT NULL, user_id INTEGER NOT NULL, prompt TEXT NOT NULL, source_kind TEXT NOT NULL, duration INTEGER NOT NULL, state TEXT NOT NULL, asset_url TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS idx_studio_video_jobs_tenant ON studio_video_jobs_v2719(tenant_id,user_id,created_at)`,
	} {
		if _, err := db.Exec(sql); err != nil {
			return err
		}
	}
	return nil
}
func studioVideoLimit(plan string) int {
	switch strings.ToLower(plan) {
	case "personal":
		return 1
	case "negocio", "business":
		return 2
	case "empresa", "enterprise":
		return 5
	default:
		return 0
	}
}
func studioVideoKey() string { return strings.TrimSpace(os.Getenv("LUMA_AGENTS_API_KEY")) }
func studioProviderRequest(ctx context.Context, method, path string, body []byte) (map[string]json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, method, "https://agents.lumalabs.ai/v1/generations"+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+studioVideoKey())
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := &http.Client{Timeout: 35 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 100<<10))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("proveedor de video: estado %d", resp.StatusCode)
	}
	out := map[string]json.RawMessage{}
	if err = json.Unmarshal(data, &out); err != nil {
		return nil, errors.New("respuesta de video inválida")
	}
	return out, nil
}
func studioVideoUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
		} else if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return true
}
func studioVideoString(row map[string]json.RawMessage, k string) string {
	var s string
	_ = json.Unmarshal(row[k], &s)
	return s
}
func (a *App) studioVideoV2719Handler(w http.ResponseWriter, r *http.Request) {
	tid, u, err := a.tenantFor(r)
	if err != nil {
		http.Error(w, "Sesión requerida", 401)
		return
	}
	if !socialPermissionsFor(u).CreateDraft {
		http.Error(w, "Sin permiso para crear videos", 403)
		return
	}
	if r.Method == http.MethodGet && r.URL.Query().Get("status") == "config" {
		writeJSON(w, map[string]any{"available": studioVideoKey() != "", "provider": "Luma Agents", "model": "ray-3.2"})
		return
	}
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", 405)
		return
	}
	if studioVideoKey() == "" {
		http.Error(w, "Para generar video IA real, el administrador debe configurar LUMA_AGENTS_API_KEY en Render", 503)
		return
	}
	if r.Method == http.MethodPost {
		plan, _, err := a.activePlan(a.billingAccountUserID(u))
		if err != nil {
			http.Error(w, "No se pudo comprobar el plan", 503)
			return
		}
		lim := studioVideoLimit(plan.Code)
		if lim <= 0 {
			http.Error(w, "Tu plan no tiene generaciones de video IA habilitadas", 403)
			return
		}
		var used int
		start := time.Now().UTC().Format("2006-01-02")
		if err = a.db.QueryRow(`SELECT COUNT(*) FROM studio_video_jobs_v2719 WHERE tenant_id=? AND user_id=? AND created_at>=?`, tid, u.ID, start).Scan(&used); err != nil {
			http.Error(w, "No se pudo calcular el límite", 500)
			return
		}
		if used >= lim {
			http.Error(w, "Límite diario de videos IA alcanzado para este plan", 429)
			return
		}
		var in struct {
			Prompt   string `json:"prompt"`
			Ratio    string `json:"ratio"`
			ImageURL string `json:"image_url"`
			Duration int    `json:"duration"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, 8192)
		if json.NewDecoder(r.Body).Decode(&in) != nil {
			http.Error(w, "Datos inválidos", 400)
			return
		}
		in.Prompt = strings.TrimSpace(in.Prompt)
		if len([]rune(in.Prompt)) < 10 || len([]rune(in.Prompt)) > 1800 {
			http.Error(w, "Describe el video entre 10 y 1800 caracteres", 400)
			return
		}
		switch in.Ratio {
		case "9:16", "16:9", "1:1", "4:5":
		default:
			in.Ratio = "9:16"
		}
		if in.Duration != 5 && in.Duration != 10 {
			in.Duration = 5
		}
		kind := "text"
		settings := map[string]any{"duration": fmt.Sprintf("%ds", in.Duration), "resolution": "720p"}
		if in.ImageURL != "" {
			// Never download an arbitrary client URL. The image MUST be in the
			// current tenant's existing multimedia library and persistent disk.
			var mediaType, urlOnDB string
			if err = a.db.QueryRow(`SELECT media_type,url FROM social_media_assets WHERE tenant_id=? AND url=?`, tid, in.ImageURL).Scan(&mediaType, &urlOnDB); err != nil || mediaType != "image" {
				http.Error(w, "La imagen no pertenece a tu galería", 403)
				return
			}
			expected := "/uploads/social/" + strconv.FormatInt(tid, 10) + "/"
			if !strings.HasPrefix(urlOnDB, expected) {
				http.Error(w, "Ruta de imagen no válida", 400)
				return
			}
			base := strings.TrimPrefix(urlOnDB, expected)
			if base == "" || base != filepath.Base(base) || strings.Contains(base, "\\") {
				http.Error(w, "Archivo de imagen no válido", 400)
				return
			}
			f, err := os.Open(filepath.Join(a.cfg.DataDir, "social_uploads", strconv.FormatInt(tid, 10), base))
			if err != nil {
				http.Error(w, "No se pudo leer la imagen", 400)
				return
			}
			b, err := io.ReadAll(io.LimitReader(f, (10<<20)+1))
			f.Close()
			if err != nil || len(b) == 0 || len(b) > 10<<20 {
				http.Error(w, "La imagen debe ser menor de 10 MB", 400)
				return
			}
			mime := http.DetectContentType(b)
			if mime != "image/png" && mime != "image/jpeg" && mime != "image/webp" {
				http.Error(w, "Usa imágenes PNG, JPG o WebP", 400)
				return
			}
			settings["start_frame"] = map[string]any{"data": base64.StdEncoding.EncodeToString(b), "media_type": mime}
			kind = "image"
		}
		payload, _ := json.Marshal(map[string]any{"type": "video", "model": "ray-3.2", "prompt": in.Prompt, "aspect_ratio": in.Ratio, "video": settings, "user_id": fmt.Sprintf("t%d_u%d", tid, u.ID)})
		ctx, cancel := context.WithTimeout(r.Context(), 42*time.Second)
		defer cancel()
		result, err := studioProviderRequest(ctx, http.MethodPost, "", payload)
		if err != nil {
			http.Error(w, "No se pudo solicitar el video: "+err.Error(), 502)
			return
		}
		id := studioVideoString(result, "id")
		if !studioVideoUUID(id) {
			http.Error(w, "El proveedor no devolvió un identificador válido", 502)
			return
		}
		state := studioVideoString(result, "state")
		if state == "" {
			state = "queued"
		}
		now := time.Now().UTC().Format(time.RFC3339)
		if _, err = a.db.Exec(`INSERT INTO studio_video_jobs_v2719(id,tenant_id,user_id,prompt,source_kind,duration,state,asset_url,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, id, tid, u.ID, in.Prompt, kind, in.Duration, state, "", now, now); err != nil {
			http.Error(w, "Solicitud enviada, pero no se pudo guardar el seguimiento; contacta al administrador", 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "id": id, "state": state, "duration": in.Duration, "kind": kind})
		return
	}
	id := r.URL.Query().Get("id")
	if !studioVideoUUID(id) {
		http.Error(w, "ID inválido", 400)
		return
	}
	var state, assetURL string
	var duration int
	if err = a.db.QueryRow(`SELECT state,asset_url,duration FROM studio_video_jobs_v2719 WHERE id=? AND tenant_id=? AND user_id=?`, id, tid, u.ID).Scan(&state, &assetURL, &duration); err != nil {
		http.Error(w, "Video no encontrado", 404)
		return
	}
	if assetURL != "" {
		writeJSON(w, map[string]any{"ok": true, "id": id, "state": "completed", "url": assetURL, "duration": duration})
		return
	}
	result, err := studioProviderRequest(r.Context(), http.MethodGet, "/"+id, nil)
	if err != nil {
		http.Error(w, "No se pudo consultar el video", 502)
		return
	}
	state = studioVideoString(result, "state")
	switch state {
	case "queued", "processing", "completed", "failed":
	default:
		state = "processing"
	}
	if state == "completed" {
		var outputs []struct {
			Type string `json:"type"`
			URL  string `json:"url"`
		}
		_ = json.Unmarshal(result["output"], &outputs)
		remote := ""
		for _, v := range outputs {
			if v.Type == "video" && v.URL != "" {
				remote = v.URL
				break
			}
		}
		if remote == "" {
			http.Error(w, "El proveedor no entregó un MP4", 502)
			return
		}
		video, err := studioDownloadVideoV2719(r.Context(), remote)
		if err != nil {
			http.Error(w, "Error al guardar video generado: "+err.Error(), 502)
			return
		}
		assetURL, err = a.studioWriteAsset(tid, u.ID, "Video IA "+id[:8]+".mp4", "video/mp4", video, "studio_video_ia")
		if err != nil {
			http.Error(w, "No se pudo guardar el MP4 en tu Galería", 500)
			return
		}
	}
	_, _ = a.db.Exec(`UPDATE studio_video_jobs_v2719 SET state=?,asset_url=?,updated_at=? WHERE id=? AND tenant_id=? AND user_id=?`, state, assetURL, time.Now().UTC().Format(time.RFC3339), id, tid, u.ID)
	response := map[string]any{"ok": true, "id": id, "state": state, "duration": duration}
	if assetURL != "" {
		response["url"] = assetURL
	}
	if state == "failed" {
		response["error"] = "El proveedor no pudo generar este clip"
	}
	writeJSON(w, response)
}

// Guard remote provider MP4s without permitting SSRF into private Render services.
func studioDownloadVideoV2719(ctx context.Context, rawURL string) ([]byte, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil {
		return nil, errors.New("URL de proveedor inválida")
	}
	if parsed.Port() != "" && parsed.Port() != "443" {
		return nil, errors.New("Puerto no autorizado")
	}
	validateHost := func(host string) (string, error) {
		if net.ParseIP(host) != nil {
			return "", errors.New("No se aceptan IP directas")
		}
		ips, e := net.DefaultResolver.LookupIPAddr(ctx, host)
		if e != nil || len(ips) == 0 {
			return "", errors.New("DNS inválido")
		}
		for _, ip := range ips {
			v := ip.IP
			if !v.IsGlobalUnicast() || v.IsPrivate() || v.IsLoopback() || v.IsLinkLocalUnicast() {
				return "", errors.New("Destino no público")
			}
		}
		return ips[0].IP.String(), nil
	}
	transport := &http.Transport{Proxy: nil, DialContext: func(c context.Context, network, addr string) (net.Conn, error) {
		host, port, e := net.SplitHostPort(addr)
		if e != nil {
			return nil, e
		}
		ip, e := validateHost(host)
		if e != nil {
			return nil, e
		}
		return (&net.Dialer{Timeout: 12 * time.Second}).DialContext(c, network, net.JoinHostPort(ip, port))
	}}
	client := &http.Client{Timeout: 75 * time.Second, Transport: transport, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > 3 || req.URL.Scheme != "https" || req.URL.User != nil {
			return errors.New("Redirección no autorizada")
		}
		return nil
	}}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, errors.New("Archivo del proveedor no disponible")
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, (50<<20)+1))
	if err != nil || len(b) > 50<<20 || len(b) < 12 {
		return nil, errors.New("Video vacío o mayor a 50 MB")
	}
	if string(b[4:8]) != "ftyp" {
		return nil, errors.New("Respuesta no es MP4")
	}
	return b, nil
}
