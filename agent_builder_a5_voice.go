package main

import (
    "bytes"
    "context"
    "encoding/json"
    "errors"
    "io"
    "mime/multipart"
    "net/http"
    "net/url"
    "os"
    "path/filepath"
    "strings"
    "sync"
    "sync/atomic"
    "time"
)

// A5 audio-to-text does not save media or change agents. The existing A1 handler
// authenticates the user, verifies the tenant and authorizes agent administration
// BEFORE calling this method. Keeping it in the same handler avoids a public route.
const agentBuilderA5MaxAudioBytes = 5 << 20

var agentBuilderA5ActiveTranscriptions sync.Map // tenant -> *atomic.Bool

func agentBuilderA5AcceptedExt(ext string) bool {
    switch strings.ToLower(ext) {
    case ".webm", ".mp4", ".m4a", ".ogg", ".wav", ".mp3":
        return true
    default:
        return false
    }
}

func agentBuilderA5OwnOrigin(r *http.Request) bool {
    origin := strings.TrimSpace(r.Header.Get("Origin"))
    if origin == "" || origin == "null" { return false }
    parsed, err := url.Parse(origin)
    if err != nil || !strings.EqualFold(parsed.Host, r.Host) || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" { return false }
    if parsed.Path != "" && parsed.Path != "/" { return false }
    // Requests in production must originate from the HTTPS web app.
    if parsed.Scheme == "https" { return true }
    return parsed.Scheme == "http" && (strings.HasPrefix(r.Host,"localhost:") || strings.HasPrefix(r.Host,"127.0.0.1:"))
}

func agentBuilderA5Model() string {
    model := strings.TrimSpace(os.Getenv("WORKTIC_AGENT_VOICE_TRANSCRIBE_MODEL"))
    switch model {
    case "gpt-transcribe", "gpt-4o-mini-transcribe", "gpt-4o-transcribe": return model
    default: return "gpt-transcribe"
    }
}

// agentBuilderA5SendAudio is a private seam to test the upstream protocol
// without external network access, tenant credentials or a live microphone.
func agentBuilderA5SendAudio(ctx context.Context, client *http.Client, endpoint, apiKey, model, extension string, audio []byte) (string, error) {
    if apiKey == "" || !agentBuilderA5AcceptedExt(extension) || len(audio) < 800 || len(audio) > agentBuilderA5MaxAudioBytes {
        return "", errors.New("invalid transcription request")
    }
    body := new(bytes.Buffer)
    writer := multipart.NewWriter(body)
    if err := writer.WriteField("model", model); err != nil { return "", err }
    if err := writer.WriteField("response_format", "json"); err != nil { return "", err }
    part, err := writer.CreateFormFile("file", "mensaje"+extension)
    if err != nil { return "", err }
    if _, err = part.Write(audio); err != nil { return "", err }
    if err = writer.Close(); err != nil { return "", err }
    request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, body)
    if err != nil { return "", err }
    request.Header.Set("Authorization", "Bearer "+apiKey)
    request.Header.Set("Content-Type", writer.FormDataContentType())
    response, err := client.Do(request)
    if err != nil { return "", err }
    defer response.Body.Close()
    if response.StatusCode < 200 || response.StatusCode >= 300 {
        // Never forward upstream errors that could contain private details.
        return "", errors.New("transcription upstream not available")
    }
    raw, err := io.ReadAll(io.LimitReader(response.Body, 32<<10))
    if err != nil { return "", err }
    var parsed struct {Text string `json:"text"`}
    if json.Unmarshal(raw, &parsed) != nil { return "", errors.New("invalid transcription response") }
    text := strings.TrimSpace(parsed.Text)
    if len([]rune(text)) > 1200 {text = string([]rune(text)[:1200])}
    if text == "" {return "", errors.New("transcription empty")}
    return text, nil
}

func (a *App) agentBuilderA5Transcribe(w http.ResponseWriter, r *http.Request, tenant int64) {
    w.Header().Set("Cache-Control", "no-store")
    w.Header().Set("X-Content-Type-Options", "nosniff")
    if !agentBuilderA5OwnOrigin(r) {
        writeError(w, errors.New("origen de audio no autorizado"), http.StatusForbidden)
        return
    }
    lockAny, _ := agentBuilderA5ActiveTranscriptions.LoadOrStore(tenant, &atomic.Bool{})
    lock := lockAny.(*atomic.Bool)
    if !lock.CompareAndSwap(false, true) {
        writeError(w, errors.New("Ya se está procesando una grabación de esta empresa"), http.StatusTooManyRequests)
        return
    }
    defer lock.Store(false)

    r.Body = http.MaxBytesReader(w, r.Body, (agentBuilderA5MaxAudioBytes + (1 << 19)))
    if err := r.ParseMultipartForm(1 << 20); err != nil {
        writeError(w, errors.New("archivo demasiado grande o inválido"), http.StatusRequestEntityTooLarge)
        return
    }
    if r.MultipartForm != nil { defer r.MultipartForm.RemoveAll() }
    if r.FormValue("mode") != "transcribe" {
        writeError(w, errors.New("modo de voz no válido"), http.StatusBadRequest)
        return
    }
    file, hdr, err := r.FormFile("audio")
    if err != nil {
        writeError(w, errors.New("falta una grabación de audio"), http.StatusBadRequest)
        return
    }
    defer file.Close()
    ext := strings.ToLower(filepath.Ext(hdr.Filename))
    if !agentBuilderA5AcceptedExt(ext) {
        writeError(w, errors.New("formato de audio no admitido"), http.StatusUnsupportedMediaType)
        return
    }
    if hdr.Size > agentBuilderA5MaxAudioBytes {
        writeError(w, errors.New("grabación demasiado grande"), http.StatusRequestEntityTooLarge)
        return
    }
    audio, err := io.ReadAll(io.LimitReader(file, agentBuilderA5MaxAudioBytes+1))
    if err != nil || len(audio) > agentBuilderA5MaxAudioBytes || len(audio) < 800 {
        writeError(w, errors.New("grabación inválida, breve o demasiado grande"), http.StatusBadRequest)
        return
    }
    ctx, cancel := context.WithTimeout(r.Context(), 40*time.Second)
    defer cancel()
    text, err := agentBuilderA5SendAudio(ctx, &http.Client{Timeout:40*time.Second},
        "https://api.openai.com/v1/audio/transcriptions", a.openAIKey(), agentBuilderA5Model(), ext, audio)
    if err != nil {
        writeError(w, errors.New("No se pudo transcribir el audio. Reintenta o utiliza el chat de texto"), http.StatusBadGateway)
        return
    }
    writeJSON(w, map[string]any{"text":text})
}
