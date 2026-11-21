package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"mime"
	"net/http"
	"net/textproto"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// A6: Opt-in pilot for natural speech-to-speech conversations.
// All requests pass A1's existing session, tenant and role checks first.
// No API key, SDP, or voice media is stored in Worktic's database.
var agentBuilderA6RateMu sync.Mutex
var agentBuilderA6Starts = map[int64][]time.Time{}

func agentBuilderA6Enabled() bool { return os.Getenv("WORKTIC_AGENT_REALTIME_ENABLED") == "1" }

func agentBuilderA6PilotAllowsTenant(tenant int64, role string) bool {
	if role == "superadmin" && os.Getenv("WORKTIC_AGENT_REALTIME_SUPERADMIN_PILOT") == "1" {
		return true
	}
	allowed := strings.TrimSpace(os.Getenv("WORKTIC_AGENT_REALTIME_PILOT_TENANTS"))
	if allowed == "" {
		return false
	} // deny by default even if enabled, until explicitly allowlisted
	for _, item := range strings.Split(allowed, ",") {
		n, err := strconv.ParseInt(strings.TrimSpace(item), 10, 64)
		if err == nil && n == tenant {
			return true
		}
	}
	return false
}

func agentBuilderA6Admit(tenant int64, now time.Time) bool {
	agentBuilderA6RateMu.Lock()
	defer agentBuilderA6RateMu.Unlock()
	prev := agentBuilderA6Starts[tenant]
	kept := make([]time.Time, 0, len(prev)+1)
	for _, t := range prev {
		if now.Sub(t) < time.Hour {
			kept = append(kept, t)
		}
	}
	if len(kept) >= 4 {
		agentBuilderA6Starts[tenant] = kept
		return false
	}
	agentBuilderA6Starts[tenant] = append(kept, now)
	return true
}

func agentBuilderA6Release(tenant int64, started time.Time) {
	agentBuilderA6RateMu.Lock()
	defer agentBuilderA6RateMu.Unlock()
	values := agentBuilderA6Starts[tenant]
	for i := len(values)-1; i>=0; i-- {
		if values[i].Equal(started) {
			agentBuilderA6Starts[tenant] = append(values[:i], values[i+1:]...)
			return
		}
	}
}

func agentBuilderA6Config() ([]byte, error) {
	model := strings.TrimSpace(os.Getenv("WORKTIC_AGENT_REALTIME_MODEL"))
	if model == "" {
		model = "gpt-realtime-2.1"
	}
	if model != "gpt-realtime-2.1" && model != "gpt-realtime-2" && model != "gpt-realtime" {
		return nil, errors.New("modelo de voz no permitido")
	}
	instruction := `Eres el asistente de voz de Worktic AI para ayudar a diseñar agentes IA. Hablas en español rioplatense natural con voz femenina cálida, joven-adulta y expresiva, acento argentino suave y comprensible para toda Latinoamérica. Usa voseo con moderación cuando sea natural, como «contame» o «querés», sin exagerar muletillas, imitar a ninguna persona ni parecer una locución robótica. Habla con ritmo conversacional, pequeñas pausas naturales y entonación amable; evita recitar listas y repetir frases prefabricadas. Escucha al usuario, conserva sus preferencias y construye las ideas gradualmente. Haz como máximo una pregunta breve por respuesta cuando sea útil. Responde con frases cortas por voz y evita enumerar listas largas. En el primer turno de voz saluda TÚ primero de manera espontánea, preséntate brevemente como la asistente de Worktic AI y pregunta solamente qué agente desea crear. No esperes a que el usuario hable para iniciar el saludo; después de saludar, escucha su respuesta. Si ya conversaron, retoma el contexto que te aporte la persona. Puedes explicar capacidades existentes sin inventar integraciones, precios, permisos ni datos del negocio. No puedes crear, guardar, activar ni publicar agentes ni realizar acciones sobre cuentas. Al acabar, dile que use «Usar en el formulario» para revisar lo conversado y que solo el usuario puede pulsar Crear agente. Atiende interrupciones con naturalidad. Nunca solicites contraseñas o secretos.`
	// Keep voice selection on the trusted server; a prompt cannot change
	// the underlying voice timbre. Only approved built-in voices are allowed.
	voice := strings.ToLower(strings.TrimSpace(os.Getenv("WORKTIC_AGENT_REALTIME_VOICE")))
	if voice == "" { voice = "coral" }
	if voice != "coral" && voice != "shimmer" && voice != "marin" {
		return nil, errors.New("voz de Realtime no permitida")
	}
	return json.Marshal(map[string]any{
		"type": "realtime", "model": model,
		"instructions":      instruction,
		"output_modalities": []string{"audio"},
		"audio": map[string]any{
			"input": map[string]any{
				"transcription":  map[string]any{"model": "gpt-transcribe", "language": "es"},
				"turn_detection": map[string]any{"type": "semantic_vad", "eagerness": "medium", "create_response": true, "interrupt_response": true},
			},
			"output": map[string]any{"voice": voice},
		},
	})
}

// Expose only a sanitized upstream status and error identifier; never expose
// credentials, session SDP, request content, or raw provider error messages.
type agentBuilderA6ProviderError struct {
	Status int
	Code string
	Kind string
}
func (e *agentBuilderA6ProviderError) Error() string {
	return fmt.Sprintf("proveedor de voz HTTP %d code=%s type=%s", e.Status, e.Code, e.Kind)
}
func agentBuilderA6SafeCode(raw string) string {
	if len(raw) > 60 { return "unknown" }
	if raw == "" { return "unspecified" }
	for _, r := range raw {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.') { return "unknown" }
	}
	return raw
}
// Exchange seam permits a complete integration test without external API calls.
func agentBuilderA6Exchange(ctx context.Context, client *http.Client, endpoint, key, safety string, sdp []byte, session []byte) ([]byte, error) {
	if key == "" || len(sdp) < 24 || len(sdp) > 64<<10 {
		return nil, errors.New("invalid offer")
	}
	var payload bytes.Buffer
	mw := multipart.NewWriter(&payload)
	// Realtime's multipart API requires a media type on each field.
	// WriteField omits it, which can cause upstream compatibility failures.
	writeTypedPart := func(field, mediaType string, value []byte) error {
		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", `form-data; name="`+field+`"`)
		header.Set("Content-Type", mediaType)
		part, err := mw.CreatePart(header)
		if err != nil { return err }
		_, err = part.Write(value)
		return err
	}
	if err := writeTypedPart("sdp", "application/sdp", sdp); err != nil {
		return nil, err
	}
	if err := writeTypedPart("session", "application/json", session); err != nil {
		return nil, err
	}
	if err := mw.Close(); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &payload)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("OpenAI-Safety-Identifier", safety)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Parse only structured error codes. Do not propagate error.message,
		// which can contain sensitive or provider-controlled text.
		var payload struct { Error struct { Code string `json:"code"`; Type string `json:"type"` } `json:"error"` }
		_ = json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&payload)
		return nil, &agentBuilderA6ProviderError{
			Status: resp.StatusCode,
			Code: agentBuilderA6SafeCode(payload.Error.Code),
			Kind: agentBuilderA6SafeCode(payload.Error.Type),
		}
	}
	// The call endpoint returns a raw SDP answer. Some upstream transports
	// label that text/plain rather than application/sdp. Validate the actual
	// answer instead of rejecting a successful call based on MIME alone.
	// Never log or return the raw SDP: it includes ICE/network metadata.
	answer, err := io.ReadAll(io.LimitReader(resp.Body, (64<<10)+1))
	if err != nil { return nil, errors.New("could not read realtime answer") }
	mediaType, _, parseErr := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if parseErr != nil { mediaType = "unrecognized" }
	switch mediaType {
	case "", "application/sdp", "text/plain", "application/json", "text/html", "application/octet-stream":
	default:
		mediaType = "other"
	}
	trimmed := bytes.TrimSpace(answer)
	if len(answer) < 24 || len(answer) > 64<<10 || !bytes.HasPrefix(trimmed, []byte("v=0")) {
		// Diagnostics contain only HTTP status and allowlisted media type.
		// Neither audio, SDP nor provider response text enters the log.
		return nil, fmt.Errorf("invalid realtime answer: upstream_http=%d upstream_media=%s sdp_valid=false", resp.StatusCode, mediaType)
	}
	return answer, nil
}

func (a *App) agentBuilderA6Realtime(w http.ResponseWriter, r *http.Request, tenant int64, role string) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if !agentBuilderA6Enabled() || !agentBuilderA6PilotAllowsTenant(tenant, role) {
		writeError(w, errors.New("La voz en vivo todavía no está habilitada para esta empresa"), http.StatusServiceUnavailable)
		return
	}
	if !agentBuilderA5OwnOrigin(r) {
		writeError(w, errors.New("origen no autorizado"), http.StatusForbidden)
		return
	}
	if r.Header.Get("Content-Type") != "application/sdp" {
		writeError(w, errors.New("formato no permitido"), http.StatusUnsupportedMediaType)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	sdp, err := io.ReadAll(r.Body)
	if err != nil || len(sdp) < 24 || len(sdp) > 64<<10 || !bytes.HasPrefix(bytes.TrimSpace(sdp), []byte("v=0")) {
		writeError(w, errors.New("solicitud de voz inválida"), http.StatusBadRequest)
		return
	}
	config, err := agentBuilderA6Config()
	if err != nil {
		writeError(w, errors.New("configuración no disponible"), http.StatusServiceUnavailable)
		return
	}
	started := time.Now()
	if !agentBuilderA6Admit(tenant, started) {
		writeError(w, errors.New("Límite piloto de sesiones por empresa alcanzado"), http.StatusTooManyRequests)
		return
	}
	// Tenant-level stable pseudonymous safety identifier: do not expose real ids externally.
	secret := a.openAIKey()
	fingerprint := sha256.Sum256([]byte(strconv.FormatInt(tenant, 10) + ":" + secret))
	safety := "worktic-tenant-" + hex.EncodeToString(fingerprint[:12])
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	answer, err := agentBuilderA6Exchange(ctx, &http.Client{Timeout: 20 * time.Second}, "https://api.openai.com/v1/realtime/calls", secret, safety, sdp, config)
	if err != nil {
		// A failed upstream request is not a successful voice session; allow
		// the pilot to retry without consuming all four hourly openings.
		agentBuilderA6Release(tenant, started)
		log.Printf("agent-builder A6 realtime provider: %v", err)
		var provider *agentBuilderA6ProviderError
		if errors.As(err, &provider) {
			msg := fmt.Sprintf("Realtime HTTP %d (%s). Verifica el registro de voz en Render.", provider.Status, provider.Code)
			writeError(w, errors.New(msg), http.StatusBadGateway)
		} else {
			writeError(w, errors.New("No se pudo abrir la conversación de voz. Revisa los registros de Render."), http.StatusBadGateway)
		}
		return
	}
	w.Header().Set("Content-Type", "application/sdp")
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write(answer)
}
