package main

// Worktic Studio V27.11 — additive, tenant-isolated multimedia projects.
// Does not change existing Social Hub tables, payments or publication flows.
import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func initStudioV2711Schema(db *DB) error {
	for _, statement := range []string{
		`CREATE TABLE IF NOT EXISTS studio_projects_v2711 (id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL, name TEXT NOT NULL DEFAULT '', kind TEXT NOT NULL DEFAULT 'image', data_json TEXT NOT NULL DEFAULT '{}', created_by INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS idx_studio_projects_tenant ON studio_projects_v2711(tenant_id,updated_at)`,
		`CREATE TABLE IF NOT EXISTS studio_ai_usage_v2711 (id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL, user_id INTEGER NOT NULL, action TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS idx_studio_ai_usage_tenant_date ON studio_ai_usage_v2711(tenant_id,created_at)`,
	} {
		if _, err := db.Exec(statement); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) studioProjectsV2711Handler(w http.ResponseWriter, r *http.Request) {
	tid, u, err := a.tenantFor(r)
	if err != nil {
		http.Error(w, "Sesión requerida", 401)
		return
	}
	perms := socialPermissionsFor(u)
	plan, _, planErr := a.activePlan(a.billingAccountUserID(u))
	if planErr != nil {
		http.Error(w, "No se pudo verificar el plan", 503)
		return
	}
	commercialRule, ruleErr := a.contractStudioRuleV2729(tid, plan.Code)
	if ruleErr != nil || !commercialRule.Editor {
		http.Error(w, "Editor no incluido en tu plan", 403)
		return
	}
	if !perms.View {
		http.Error(w, "Sin permiso para usar Worktic Studio", 403)
		return
	}
	id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	switch r.Method {
	case http.MethodGet:
		if id > 0 {
			var name, kind, data, created, updated string
			var owner int64
			if err := a.db.QueryRow(`SELECT name,kind,data_json,created_at,updated_at,created_by FROM studio_projects_v2711 WHERE id=? AND tenant_id=?`, id, tid).Scan(&name, &kind, &data, &created, &updated, &owner); err != nil {
				http.Error(w, "Proyecto no encontrado", 404)
				return
			}
			writeJSON(w, map[string]any{"id": id, "name": name, "kind": kind, "data": json.RawMessage(data), "created_at": created, "updated_at": updated, "created_by": owner})
			return
		}
		rows, err := a.db.Query(`SELECT id,name,kind,updated_at FROM studio_projects_v2711 WHERE tenant_id=? ORDER BY updated_at DESC LIMIT 120`, tid)
		if err != nil {
			http.Error(w, "Error de lectura", 500)
			return
		}
		defer rows.Close()
		list := []map[string]any{}
		for rows.Next() {
			var pid int64
			var name, kind, updated string
			if rows.Scan(&pid, &name, &kind, &updated) == nil {
				list = append(list, map[string]any{"id": pid, "name": name, "kind": kind, "updated_at": updated})
			}
		}
		writeJSON(w, list)
	case http.MethodPost:
		if !perms.CreateDraft {
			http.Error(w, "Sin permiso para editar", 403)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1024*1024+4096)
		var q struct {
			ID   int64           `json:"id"`
			Name string          `json:"name"`
			Kind string          `json:"kind"`
			Data json.RawMessage `json:"data"`
		}
		if json.NewDecoder(r.Body).Decode(&q) != nil || len(q.Data) == 0 || len(q.Data) > 1024*1024 || !json.Valid(q.Data) {
			http.Error(w, "Proyecto inválido o demasiado grande", 400)
			return
		}
		if q.Kind != "image" && q.Kind != "video" {
			http.Error(w, "Tipo de proyecto inválido", 400)
			return
		}
		q.Name = strings.TrimSpace(q.Name)
		if len(q.Name) > 120 {
			http.Error(w, "Nombre demasiado largo", 400)
			return
		}
		if q.Name == "" {
			q.Name = "Sin título"
		}
		var v map[string]any
		if json.Unmarshal(q.Data, &v) != nil || v == nil {
			http.Error(w, "Documento de proyecto inválido", 400)
			return
		}
		now := time.Now().UTC().Format(time.RFC3339)
		if q.ID > 0 {
			res, err := a.db.Exec(`UPDATE studio_projects_v2711 SET name=?,kind=?,data_json=?,updated_at=? WHERE id=? AND tenant_id=?`, q.Name, q.Kind, string(q.Data), now, q.ID, tid)
			if err != nil {
				http.Error(w, "Error al guardar", 500)
				return
			}
			n, _ := res.RowsAffected()
			if n == 0 {
				http.Error(w, "Proyecto no encontrado", 404)
				return
			}
			writeJSON(w, map[string]any{"ok": true, "id": q.ID, "updated_at": now})
			return
		}
		tx, err := a.db.Begin()
		if err != nil {
			http.Error(w, "Error al iniciar el proyecto", 503)
			return
		}
		defer tx.Rollback()
		if err = commercialLockTenantTx(tx, tid); err != nil {
			http.Error(w, "No se pudo bloquear proyectos", 503)
			return
		}
		var existing int64
		if err = tx.QueryRow(`SELECT COUNT(*) FROM studio_projects_v2711 WHERE tenant_id=?`, tid).Scan(&existing); err != nil {
			http.Error(w, "No se pudieron consultar proyectos", 503)
			return
		}
		if existing >= commercialRule.MaxProjects {
			http.Error(w, "Has alcanzado el máximo de proyectos del plan", 429)
			return
		}
		res, err := tx.Exec(`INSERT INTO studio_projects_v2711(tenant_id,name,kind,data_json,created_by,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`, tid, q.Name, q.Kind, string(q.Data), u.ID, now, now)
		if err != nil {
			http.Error(w, "Error guardando proyecto", 500)
			return
		}
		newID, _ := res.LastInsertId()
		if err = tx.Commit(); err != nil {
			http.Error(w, "Error confirmando proyecto", 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "id": newID, "updated_at": now})
	case http.MethodDelete:
		if !perms.DeleteContent {
			http.Error(w, "Sin permiso para eliminar", 403)
			return
		}
		if id <= 0 {
			http.Error(w, "ID requerido", 400)
			return
		}
		_, err := a.db.Exec(`DELETE FROM studio_projects_v2711 WHERE id=? AND tenant_id=?`, id, tid)
		if err != nil {
			http.Error(w, "Error al eliminar", 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	default:
		http.Error(w, "Método no permitido", 405)
	}
}

func studioAILimit(plan string) int {
	switch strings.ToLower(plan) {
	case "enterprise", "empresa":
		return 30
	case "business", "negocio":
		return 10
	case "personal":
		return 3
	default:
		return 0
	}
}

func (a *App) studioAIV2711Handler(w http.ResponseWriter, r *http.Request) {
	tid, u, err := a.tenantFor(r)
	if err != nil {
		http.Error(w, "Sesión requerida", 401)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	if !socialPermissionsFor(u).CreateDraft {
		http.Error(w, "Sin permiso para crear", 403)
		return
	}
	if a.openAIKey() == "" {
		http.Error(w, "OPENAI_API_KEY no configurada", 503)
		return
	}
	plan, _, err := a.activePlan(a.billingAccountUserID(u))
	if err != nil {
		http.Error(w, "No se pudo determinar el plan", 503)
		return
	}
	limit := studioAILimit(plan.Code)
	if limit == 0 {
		http.Error(w, "Generación IA disponible para planes de pago", 403)
		return
	}
	start := time.Now().UTC().Format("2006-01-02")
	var used int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM studio_ai_usage_v2711 WHERE tenant_id=? AND created_at>=?`, tid, start).Scan(&used); err != nil {
		http.Error(w, "Error de límites", 500)
		return
	}
	if !studioCreditsEnabled() && used >= limit {
		http.Error(w, "Límite diario de IA alcanzado en tu plan", 429)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	var q struct {
		Action string `json:"action"`
		Prompt string `json:"prompt"`
		Ratio  string `json:"ratio"`
	}
	if json.NewDecoder(r.Body).Decode(&q) != nil {
		http.Error(w, "Solicitud inválida", 400)
		return
	}
	q.Prompt = strings.TrimSpace(q.Prompt)
	if len(q.Prompt) < 8 || len(q.Prompt) > 2000 {
		http.Error(w, "Describe la creación entre 8 y 2000 caracteres", 400)
		return
	}
	if q.Action != "image" && q.Action != "storyboard" && q.Action != "copy" {
		http.Error(w, "Acción inválida", 400)
		return
	}
	provider := "openai_text"
	if q.Action == "image" {
		provider = "openai_image"
	}
	hold, allowed := a.creativeReserveForStudio(w, tid, u.ID, plan.Code, q.Action, provider, creativeQuote(q.Action, 0, len([]rune(q.Prompt))), 0, len([]rune(q.Prompt)))
	if !allowed {
		return
	}
	providerAttempted := false
	defer func() {
		if err := a.creativeSettle(hold, providerAttempted); err != nil {
			fmt.Printf("creative guard settlement error: %v\n", err)
		}
	}()
	var output any
	if q.Action == "image" {
		size := "1024x1024"
		if q.Ratio == "16:9" {
			size = "1536x1024"
		}
		if q.Ratio == "9:16" {
			size = "1024x1536"
		}
		// Generation is server-side: the key never reaches the tenant browser.
		payload, _ := json.Marshal(map[string]any{"model": env("WORKTIC_STUDIO_IMAGE_MODEL", "gpt-image-1"), "quality": "medium", "prompt": q.Prompt, "size": size, "n": 1})
		client := &http.Client{Timeout: 52 * time.Second}
		request, _ := http.NewRequestWithContext(r.Context(), http.MethodPost, "https://api.openai.com/v1/images/generations", bytes.NewReader(payload))
		request.Header.Set("Authorization", "Bearer "+a.openAIKey())
		request.Header.Set("Content-Type", "application/json")
		providerAttempted = true
		response, err := client.Do(request)
		if err != nil {
			http.Error(w, "Servicio de imágenes IA no disponible", 502)
			return
		}
		defer response.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(response.Body, 18<<20))
		model := env("WORKTIC_STUDIO_IMAGE_MODEL", "gpt-image-1")
		status := "completed"
		if response.StatusCode >= 300 {
			status = "vendor_error"
		}
		ev := meterEventV2726{Key: meterKeyV2726("studio_image", hold), HoldID: meterHoldV2726(hold),
			TenantID: tid, Provider: "openai", Operation: "studio_image", Model: model, Status: status,
			Units: 1, UnitKind: size}
		if status == "completed" {
			ev.EstimatedMicroUSD = meterFlatPriceV2726("studio_image", model, size, 1)
		}
		a.meterRecordV2726(ev)
		if response.StatusCode >= 300 {
			http.Error(w, fmt.Sprintf("Servicio de imágenes IA: %s", studioAIError(body)), 502)
			return
		}
		var out struct {
			Data []struct {
				B64JSON string `json:"b64_json"`
			} `json:"data"`
		}
		if json.Unmarshal(body, &out) != nil || len(out.Data) == 0 {
			http.Error(w, "Respuesta de imágenes IA incompleta", 502)
			return
		}
		b, err := base64.StdEncoding.DecodeString(out.Data[0].B64JSON)
		if err != nil || len(b) > 12<<20 || len(b) == 0 {
			http.Error(w, "Imagen IA inválida", 502)
			return
		}
		imageURL, err := a.studioWriteAsset(tid, u.ID, "worktic_ia.png", "image/png", b, "studio_ia")
		if err != nil {
			http.Error(w, "Error guardando imagen IA", 500)
			return
		}
		output = map[string]any{"url": imageURL, "kind": "image"}
	} else {
		system := "Eres un director creativo de marketing. No inventes resultados, precios ni testimonios. Responde en español y devuelve solo JSON válido."
		if q.Action == "storyboard" {
			system += ` Formato obligatorio: {"title":"...","scenes":[{"text":"título de pantalla breve","duration":4,"visual":"idea visual"}]} entre 3 y 6 escenas, duración 3-6 segundos por escena.`
		} else {
			system += ` Formato obligatorio: {"headline":"máximo 60 caracteres","subheadline":"máximo 120 caracteres","cta":"máximo 35 caracteres"}.`
		}
		providerAttempted = true
		// Negative tenant marker: Studio has ALREADY reserved its own creative credits.
		// Keep tenant attribution for metering without double debit of Core AI.
		txt, err := a.callOpenAI(system, q.Prompt, -tid)
		if err != nil {
			http.Error(w, "No se pudo generar el texto IA", 502)
			return
		}
		txt = strings.TrimSpace(txt)
		i := strings.Index(txt, "{")
		j := strings.LastIndex(txt, "}")
		if i < 0 || j <= i {
			http.Error(w, "La IA devolvió un formato inválido", 502)
			return
		}
		var result map[string]any
		if json.Unmarshal([]byte(txt[i:j+1]), &result) != nil {
			http.Error(w, "La IA devolvió un JSON inválido", 502)
			return
		}
		output = result
	}
	if _, err := a.db.Exec(`INSERT INTO studio_ai_usage_v2711(tenant_id,user_id,action,created_at) VALUES(?,?,?,?)`, tid, u.ID, q.Action, time.Now().UTC().Format(time.RFC3339)); err != nil { // Creation succeeded; log metering issue without undoing a legitimate asset.
		fmt.Printf("studio ai meter error: %v\n", err)
	}
	writeJSON(w, map[string]any{"ok": true, "result": output, "remaining": max(0, limit-used-1)})
}
func studioAIError(b []byte) string {
	var q struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(b, &q) == nil && q.Error.Message != "" {
		return q.Error.Message
	}
	return "error del proveedor"
}

func (a *App) studioWriteAsset(tid, uid int64, name, mime string, b []byte, source string) (string, error) {
	if len(b) == 0 || len(b) > 50<<20 {
		return "", errors.New("archivo demasiado grande")
	}
	ext := ".png"
	if mime == "image/jpeg" {
		ext = ".jpg"
	}
	if mime == "image/webp" {
		ext = ".webp"
	}
	if mime == "video/webm" {
		ext = ".webm"
	}
	if mime == "video/mp4" {
		ext = ".mp4"
	}
	if mime == "audio/mpeg" {
		ext = ".mp3"
	}
	if mime == "audio/wav" {
		ext = ".wav"
	}
	if mime == "audio/ogg" {
		ext = ".ogg"
	}
	if mime == "audio/mp4" {
		ext = ".m4a"
	}
	tx, err := a.commercialAssetWriteTx(tid, int64(len(b)))
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	dir := filepath.Join(a.cfg.DataDir, "social_uploads", strconv.FormatInt(tid, 10))
	if err := os.MkdirAll(dir, 0750); err != nil {
		return "", err
	}
	filename := fmt.Sprintf("studio_%d_%d%s", time.Now().UnixNano(), uid, ext)
	if err := os.WriteFile(filepath.Join(dir, filename), b, 0640); err != nil {
		return "", err
	}
	url := "/uploads/social/" + strconv.FormatInt(tid, 10) + "/" + filename
	typ := "image"
	if strings.HasPrefix(mime, "video/") {
		typ = "video"
	}
	if strings.HasPrefix(mime, "audio/") {
		typ = "audio"
	}
	_, err = tx.Exec(`INSERT INTO social_media_assets(tenant_id,name,url,media_type,mime,size_bytes,source,created_by,created_at) VALUES(?,?,?,?,?,?,?,?,?)`, tid, name, url, typ, mime, len(b), source, uid, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		_ = os.Remove(filepath.Join(dir, filename))
		return "", err
	}
	if err = tx.Commit(); err != nil {
		_ = os.Remove(filepath.Join(dir, filename))
		return "", err
	}
	return url, nil
}

// studioAudioUploadV2717Handler adds audio without relaxing the original
// Social Hub media upload validations or altering its existing endpoints.
func (a *App) studioAudioUploadV2717Handler(w http.ResponseWriter, r *http.Request) {
	tid, u, err := a.tenantFor(r)
	if err != nil {
		http.Error(w, "Sesión requerida", http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}
	if !socialPermissionsFor(u).CreateDraft {
		http.Error(w, "Sin permiso para cargar audio", http.StatusForbidden)
		return
	}
	const maxAudioSize = 20 << 20
	r.Body = http.MaxBytesReader(w, r.Body, maxAudioSize+(1<<20))
	if err := r.ParseMultipartForm(maxAudioSize); err != nil {
		http.Error(w, "Audio demasiado grande", http.StatusBadRequest)
		return
	}
	f, hdr, err := r.FormFile("media")
	if err != nil {
		http.Error(w, "Selecciona un audio", http.StatusBadRequest)
		return
	}
	defer f.Close()
	valid := map[string]string{".mp3": "audio/mpeg", ".wav": "audio/wav", ".ogg": "audio/ogg", ".m4a": "audio/mp4"}
	ext := strings.ToLower(path.Ext(hdr.Filename))
	mime, ok := valid[ext]
	if !ok || hdr.Size > maxAudioSize {
		http.Error(w, "Usa MP3, WAV, OGG o M4A de máximo 20 MB", http.StatusBadRequest)
		return
	}
	if ct := hdr.Header.Get("Content-Type"); ct != "" && ct != "application/octet-stream" && !strings.HasPrefix(ct, "audio/") {
		http.Error(w, "El archivo no es audio", http.StatusBadRequest)
		return
	}
	data, err := io.ReadAll(io.LimitReader(f, maxAudioSize+1))
	if err != nil || len(data) == 0 || len(data) > maxAudioSize {
		http.Error(w, "Audio inválido o demasiado grande", http.StatusBadRequest)
		return
	}
	// Reject common non-audio uploads even if renamed to an audio extension.
	detected := http.DetectContentType(data[:min(len(data), 512)])
	if strings.HasPrefix(detected, "text/") || strings.HasPrefix(detected, "image/") || detected == "application/pdf" || strings.HasPrefix(detected, "video/") {
		http.Error(w, "Contenido de audio inválido", http.StatusBadRequest)
		return
	}
	url, err := a.studioWriteAsset(tid, u.ID, path.Base(hdr.Filename), mime, data, "studio_audio")
	if err != nil {
		http.Error(w, "No se pudo guardar audio", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"url": url, "name": path.Base(hdr.Filename), "mime": mime, "type": "audio", "size": len(data)})
}
