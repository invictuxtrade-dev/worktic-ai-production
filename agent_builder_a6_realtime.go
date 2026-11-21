package main

import (
  "bytes"
  "context"
  "crypto/sha256"
  "encoding/hex"
  "encoding/json"
  "errors"
  "io"
  "mime/multipart"
  "net/http"
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
  if role=="superadmin" && os.Getenv("WORKTIC_AGENT_REALTIME_SUPERADMIN_PILOT")=="1" {return true}
  allowed := strings.TrimSpace(os.Getenv("WORKTIC_AGENT_REALTIME_PILOT_TENANTS"))
  if allowed == "" { return false } // deny by default even if enabled, until explicitly allowlisted
  for _, item := range strings.Split(allowed, ",") {
    n,err:=strconv.ParseInt(strings.TrimSpace(item),10,64)
    if err==nil && n==tenant { return true }
  }
  return false
}

func agentBuilderA6Admit(tenant int64, now time.Time) bool {
  agentBuilderA6RateMu.Lock()
  defer agentBuilderA6RateMu.Unlock()
  prev:=agentBuilderA6Starts[tenant]
  kept:=make([]time.Time,0,len(prev)+1)
  for _, t:=range prev {if now.Sub(t)<time.Hour {kept=append(kept,t)}}
  if len(kept)>=4 {agentBuilderA6Starts[tenant]=kept;return false}
  agentBuilderA6Starts[tenant]=append(kept,now)
  return true
}

func agentBuilderA6Config() ([]byte,error) {
  model := strings.TrimSpace(os.Getenv("WORKTIC_AGENT_REALTIME_MODEL"))
  if model=="" { model="gpt-realtime-2.1" }
  if model!="gpt-realtime-2.1" && model!="gpt-realtime-2" && model!="gpt-realtime" { return nil,errors.New("modelo de voz no permitido") }
  instruction := `Eres el asistente de voz de Worktic AI para ayudar a diseñar agentes IA. Hablas español latinoamericano de forma muy natural, cálida y clara; tu voz debe sentirse conversacional, no como cuestionario. Escucha al usuario, conserva sus preferencias y construye las ideas gradualmente. Haz como máximo una pregunta breve por respuesta cuando sea útil. Responde con frases cortas por voz y evita enumerar listas largas. Al comenzar, saluda y pregunta qué agente quiere diseñar o qué necesita que haga. Si ya conversaron, retoma el contexto que te aporte la persona. Puedes explicar capacidades existentes sin inventar integraciones, precios, permisos ni datos del negocio. No puedes crear, guardar, activar ni publicar agentes ni realizar acciones sobre cuentas. Al acabar, dile que use «Usar en el formulario» para revisar lo conversado y que solo el usuario puede pulsar Crear agente. Atiende interrupciones con naturalidad. Nunca solicites contraseñas o secretos.`
  return json.Marshal(map[string]any{
    "type":"realtime", "model":model,
    "instructions":instruction,
    "output_modalities":[]string{"audio"},
    "audio":map[string]any{
       "input":map[string]any{
          "transcription":map[string]any{"model":"gpt-transcribe"},
          "turn_detection":map[string]any{"type":"semantic_vad","eagerness":"medium","create_response":true,"interrupt_response":true},
       },
       "output":map[string]any{"voice":"marin"},
    },
  })
}

// Exchange seam permits a complete integration test without external API calls.
func agentBuilderA6Exchange(ctx context.Context, client *http.Client, endpoint, key, safety string, sdp []byte, session []byte) ([]byte,error) {
  if key=="" || len(sdp)<24 || len(sdp)>64<<10 {return nil,errors.New("invalid offer")}
  var payload bytes.Buffer
  mw:=multipart.NewWriter(&payload)
  if err:=mw.WriteField("sdp",string(sdp));err!=nil{return nil,err}
  if err:=mw.WriteField("session",string(session));err!=nil{return nil,err}
  if err:=mw.Close();err!=nil{return nil,err}
  req,err:=http.NewRequestWithContext(ctx,http.MethodPost,endpoint,&payload)
  if err!=nil{return nil,err}
  req.Header.Set("Authorization","Bearer "+key)
  req.Header.Set("Content-Type",mw.FormDataContentType())
  req.Header.Set("OpenAI-Safety-Identifier",safety)
  resp,err:=client.Do(req)
  if err!=nil{return nil,err}
  defer resp.Body.Close()
  if resp.StatusCode < 200 || resp.StatusCode >= 300 {return nil,errors.New("realtime provider unavailable")}
  if !strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")),"application/sdp") {return nil,errors.New("unexpected realtime response")}
  answer,err:=io.ReadAll(io.LimitReader(resp.Body,65<<10))
  if err!=nil || len(answer)<24 || len(answer)>64<<10 {return nil,errors.New("invalid realtime answer")}
  return answer,nil
}

func (a *App) agentBuilderA6Realtime(w http.ResponseWriter, r *http.Request, tenant int64, role string) {
  w.Header().Set("Cache-Control","no-store")
  w.Header().Set("X-Content-Type-Options","nosniff")
  if !agentBuilderA6Enabled() || !agentBuilderA6PilotAllowsTenant(tenant, role) {
    writeError(w,errors.New("La voz en vivo todavía no está habilitada para esta empresa"),http.StatusServiceUnavailable)
    return
  }
  if !agentBuilderA5OwnOrigin(r) {
    writeError(w,errors.New("origen no autorizado"),http.StatusForbidden)
    return
  }
  if r.Header.Get("Content-Type")!="application/sdp" {
    writeError(w,errors.New("formato no permitido"),http.StatusUnsupportedMediaType)
    return
  }
  r.Body=http.MaxBytesReader(w,r.Body,64<<10)
  sdp,err:=io.ReadAll(r.Body)
  if err!=nil || len(sdp)<24 || len(sdp)>64<<10 || !bytes.HasPrefix(bytes.TrimSpace(sdp),[]byte("v=0")) {
    writeError(w,errors.New("solicitud de voz inválida"),http.StatusBadRequest)
    return
  }
  config,err:=agentBuilderA6Config()
  if err!=nil {writeError(w,errors.New("configuración no disponible"),http.StatusServiceUnavailable);return}
  if !agentBuilderA6Admit(tenant,time.Now()) {
    writeError(w,errors.New("Límite piloto de sesiones por empresa alcanzado"),http.StatusTooManyRequests)
    return
  }
  // Tenant-level stable pseudonymous safety identifier: do not expose real ids externally.
  secret:=a.openAIKey()
  fingerprint:=sha256.Sum256([]byte(strconv.FormatInt(tenant,10)+":"+secret))
  safety:="worktic-tenant-"+hex.EncodeToString(fingerprint[:12])
  ctx,cancel:=context.WithTimeout(r.Context(),20*time.Second)
  defer cancel()
  answer,err:=agentBuilderA6Exchange(ctx,&http.Client{Timeout:20*time.Second},"https://api.openai.com/v1/realtime/calls",secret,safety,sdp,config)
  if err!=nil {writeError(w,errors.New("No se pudo abrir la conversación en vivo. Puedes usar el micrófono de dictado"),http.StatusBadGateway);return}
  w.Header().Set("Content-Type","application/sdp")
  w.WriteHeader(http.StatusCreated)
  _,_=w.Write(answer)
}
