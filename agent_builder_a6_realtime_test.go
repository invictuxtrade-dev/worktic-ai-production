package main

import (
 "context"
 "encoding/json"
 "io"
 "mime"
 "net/http"
 "net/http/httptest"
 "os"
 "strings"
 "testing"
 "time"
)

func TestAgentBuilderA6Config(t *testing.T){
 t.Setenv("WORKTIC_AGENT_REALTIME_MODEL","")
 data,err:=agentBuilderA6Config();if err!=nil{t.Fatal(err)}
 var doc map[string]any
 if err=json.Unmarshal(data,&doc);err!=nil{t.Fatal(err)}
 if doc["type"]!="realtime" {t.Fatal(doc)}
 audio:=doc["audio"].(map[string]any)
 input:=audio["input"].(map[string]any)
 vad:=input["turn_detection"].(map[string]any)
 if vad["type"]!="semantic_vad" || vad["interrupt_response"]!=true || vad["create_response"]!=true {t.Fatal(vad)}
 t.Setenv("WORKTIC_AGENT_REALTIME_MODEL","unsafe-custom")
 if _,err=agentBuilderA6Config();err==nil{t.Fatal("model should be allowlisted")}
}
func TestAgentBuilderA6PilotOptIn(t *testing.T){
 t.Setenv("WORKTIC_AGENT_REALTIME_PILOT_TENANTS","")
 if agentBuilderA6PilotAllowsTenant(1,"admin") {t.Fatal("default must deny")}
 t.Setenv("WORKTIC_AGENT_REALTIME_PILOT_TENANTS","2, 8")
 if !agentBuilderA6PilotAllowsTenant(8,"admin") || agentBuilderA6PilotAllowsTenant(9,"admin") {t.Fatal("tenant isolation")}
 t.Setenv("WORKTIC_AGENT_REALTIME_SUPERADMIN_PILOT", "1")
 if !agentBuilderA6PilotAllowsTenant(9,"superadmin") || agentBuilderA6PilotAllowsTenant(9,"owner") {t.Fatal("superadmin pilot isolation")}
 t.Setenv("WORKTIC_AGENT_REALTIME_ENABLED", "")
 if agentBuilderA6Enabled(){t.Fatal("must opt in")}
}
func TestAgentBuilderA6RateLimit(t *testing.T){
 tenant:=int64(987654321)
 now:=time.Now()
 for i:=0;i<4;i++ {if !agentBuilderA6Admit(tenant,now){t.Fatal("unexpected block",i)}}
 if agentBuilderA6Admit(tenant,now){t.Fatal("allowed fifth session")}
 if !agentBuilderA6Admit(tenant,now.Add(time.Hour+time.Second)){t.Fatal("rate limit did not expire")}
}
func TestAgentBuilderA6Exchange(t *testing.T){
 provider:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  if r.Method!="POST" || r.Header.Get("Authorization")!="Bearer fake-key" {t.Error("auth failed")}
  if !strings.HasPrefix(r.Header.Get("OpenAI-Safety-Identifier"),"worktic") {t.Error("safety id missing")}
  med,params,err:=mime.ParseMediaType(r.Header.Get("Content-Type"));if err!=nil||med!="multipart/form-data" {t.Error(err);return};_ = params
  if err=r.ParseMultipartForm(1<<20);err!=nil{t.Error(err);return}
  if !strings.HasPrefix(r.FormValue("sdp"),"v=0") || !strings.Contains(r.FormValue("session"),"realtime") {t.Error("invalid upstream request")}
  w.Header().Set("Content-Type","application/sdp")
  w.WriteHeader(201)
  _,_=io.WriteString(w,"v=0\r\no=- 111 1 IN IP4 127.0.0.1\r\ns=-\r\nt=0 0\r\n")
 }))
 defer provider.Close()
 answer,err:=agentBuilderA6Exchange(context.Background(),provider.Client(),provider.URL,"fake-key","worktic-test",[]byte("v=0\r\no=- 123 1 IN IP4 127.0.0.1\r\ns=-\r\nt=0 0\r\n"),[]byte(`{"type":"realtime"}`))
 if err!=nil{t.Fatal(err)}
 if !strings.HasPrefix(string(answer),"v=0"){t.Fatal("bad SDP")}
 _=os.ErrNotExist
}
