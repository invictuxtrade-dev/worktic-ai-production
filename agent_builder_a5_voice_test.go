package main

import (
 "context"
 "fmt"
 "io"
 "net/http"
 "net/http/httptest"
 "strings"
 "testing"
)

func TestAgentBuilderA5OriginMustMatchHost(t *testing.T) {
 req := httptest.NewRequest("POST", "https://workticai.com/api/agents/builder-a1", nil)
 req.Header.Set("Origin", "https://www.workticai.com")
 if agentBuilderA5OwnOrigin(req) { t.Fatal("www should not match apex host") }
 req.Header.Set("Origin", "https://workticai.com")
 if !agentBuilderA5OwnOrigin(req) { t.Fatal("same origin should pass") }
 req.Header.Set("Origin", "https://evil.example")
 if agentBuilderA5OwnOrigin(req) { t.Fatal("foreign origin accepted") }
}
func TestAgentBuilderA5AcceptedAudioAndBounds(t *testing.T) {
 for _, ext := range []string{".webm", ".ogg", ".mp4", ".wav"}{if !agentBuilderA5AcceptedExt(ext) {t.Fatal(ext)}}
 if agentBuilderA5AcceptedExt(".exe") {t.Fatal("unexpected format accepted")}
 _, err:=agentBuilderA5SendAudio(context.Background(),http.DefaultClient,"http://localhost/","token","gpt-transcribe",".webm",[]byte("x"))
 if err==nil {t.Fatal("undersized audio accepted")}
}
func TestAgentBuilderA5MultipartTranscription(t *testing.T){
 server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  if r.Header.Get("Authorization")!="Bearer secret" {t.Error("missing authentication header")}
  if r.ParseMultipartForm(1<<20)!=nil {t.Error("invalid multipart");return}
  if r.FormValue("model")!="gpt-transcribe" {t.Error("unexpected transcription model")}
  f,h,e:=r.FormFile("file");if e!=nil{t.Error(e);return};defer f.Close()
  if !strings.HasSuffix(h.Filename,".webm") {t.Error("wrong file name")}
  data,_:=io.ReadAll(f);if len(data)!=1800{t.Errorf("unexpected byte count %d",len(data))}
  fmt.Fprint(w,`{"text":" Quiero un agente que atienda clientes por WhatsApp. "}`)
 }))
 defer server.Close()
 text,err:=agentBuilderA5SendAudio(context.Background(),server.Client(),server.URL,"secret","gpt-transcribe",".webm",make([]byte,1800))
 if err!=nil {t.Fatal(err)}
 if text!="Quiero un agente que atienda clientes por WhatsApp." {t.Fatalf("unexpected text %q",text)}
}
