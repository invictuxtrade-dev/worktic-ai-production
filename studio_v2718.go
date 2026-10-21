package main

// V27.18 — tenant-scoped voice narration. All vendor credentials remain on the server.
import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

func (a *App) studioNarrationV2718Handler(w http.ResponseWriter, r *http.Request) {
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
		http.Error(w, "Sin permiso de edición", http.StatusForbidden)
		return
	}
	if a.openAIKey() == "" {
		http.Error(w, "Proveedor de narración no configurado", http.StatusServiceUnavailable)
		return
	}
	plan, _, err := a.activePlan(a.billingAccountUserID(u))
	if err != nil {
		http.Error(w, "No se pudo verificar el plan", http.StatusServiceUnavailable)
		return
	}
	limit := studioAILimit(plan.Code)
	if limit <= 0 {
		http.Error(w, "La narración IA requiere un plan de pago", http.StatusForbidden)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 12<<10)
	var q struct {
		Text  string  `json:"text"`
		Voice string  `json:"voice"`
		Speed float64 `json:"speed"`
	}
	if json.NewDecoder(r.Body).Decode(&q) != nil {
		http.Error(w, "Solicitud inválida", 400)
		return
	}
	q.Text = strings.TrimSpace(q.Text)
	if len([]rune(q.Text)) < 3 || len(q.Text) > 3500 {
		http.Error(w, "Usa entre 3 y 3500 caracteres de narración", 400)
		return
	}
	allowed := map[string]bool{"alloy": true, "ash": true, "ballad": true, "coral": true, "echo": true, "fable": true, "onyx": true, "nova": true, "sage": true, "shimmer": true, "verse": true, "marin": true, "cedar": true}
	if !allowed[q.Voice] {
		q.Voice = "coral"
	}
	if q.Speed < 0.5 || q.Speed > 2 {
		q.Speed = 1
	}
	day := time.Now().UTC().Format("2006-01-02")
	var used int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM studio_ai_usage_v2711 WHERE tenant_id=? AND created_at>=?`, tid, day).Scan(&used); err != nil {
		http.Error(w, "Error verificando créditos", 500)
		return
	}
	if used >= limit {
		http.Error(w, "Límite diario de generación IA alcanzado", 429)
		return
	}
	body, _ := json.Marshal(map[string]any{"model": env("WORKTIC_STUDIO_TTS_MODEL", "gpt-4o-mini-tts"), "input": q.Text, "voice": q.Voice, "speed": q.Speed, "response_format": "mp3"})
	ctxRequest, err := http.NewRequestWithContext(r.Context(), http.MethodPost, "https://api.openai.com/v1/audio/speech", bytes.NewReader(body))
	if err != nil {
		http.Error(w, "Error preparando la narración", 500)
		return
	}
	ctxRequest.Header.Set("Authorization", "Bearer "+a.openAIKey())
	ctxRequest.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 48 * time.Second}
	resp, err := client.Do(ctxRequest)
	if err != nil {
		http.Error(w, "Proveedor de voz no disponible", 502)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
		http.Error(w, "Proveedor de voz rechazó la solicitud", 502)
		return
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, (15<<20)+1))
	if err != nil || len(data) == 0 || len(data) > 15<<20 {
		http.Error(w, "Audio recibido inválido", 502)
		return
	}
	url, err := a.studioWriteAsset(tid, u.ID, "Narración IA.mp3", "audio/mpeg", data, "studio_tts")
	if err != nil {
		http.Error(w, "No se pudo guardar la narración", 500)
		return
	}
	if _, err := a.db.Exec(`INSERT INTO studio_ai_usage_v2711(tenant_id,user_id,action,created_at) VALUES(?,?,?,?)`, tid, u.ID, "narration", time.Now().UTC().Format(time.RFC3339)); err != nil { /* Audio already persisted. Do not misreport generation. */
	}
	writeJSON(w, map[string]any{"ok": true, "url": url, "name": "Narración IA.mp3", "voice": q.Voice, "remaining": max(0, limit-used-1)})
}
