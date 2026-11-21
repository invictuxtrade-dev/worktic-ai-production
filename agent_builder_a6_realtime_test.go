package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestAgentBuilderA6Config(t *testing.T) {
	t.Setenv("WORKTIC_AGENT_REALTIME_MODEL", "")
	data, err := agentBuilderA6Config()
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err = json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["type"] != "realtime" {
		t.Fatal(doc)
	}
	if doc["model"] != "gpt-realtime-2.1" {
		t.Fatalf("unexpected model: %v", doc["model"])
	}
	instructions, _ := doc["instructions"].(string)
	if !strings.Contains(instructions, "acento argentino suave") || !strings.Contains(instructions, "saluda TÚ primero") {
		t.Fatalf("missing voice/greeting configuration: %s", instructions)
	}
	audio := doc["audio"].(map[string]any)
	output := audio["output"].(map[string]any)
	if output["voice"] != "coral" {
		t.Fatalf("unexpected voice: %v", output["voice"])
	}
	input := audio["input"].(map[string]any)
	vad := input["turn_detection"].(map[string]any)
	if vad["type"] != "semantic_vad" || vad["interrupt_response"] != true || vad["create_response"] != true {
		t.Fatal(vad)
	}
	transcription := input["transcription"].(map[string]any)
	if transcription["model"] != "gpt-transcribe" || transcription["language"] != "es" {
		t.Fatalf("unexpected transcription config: %v", transcription)
	}
	t.Setenv("WORKTIC_AGENT_REALTIME_VOICE", "shimmer")
	if data, err = agentBuilderA6Config(); err != nil { t.Fatal(err) }
	if err = json.Unmarshal(data, &doc); err != nil { t.Fatal(err) }
	if doc["audio"].(map[string]any)["output"].(map[string]any)["voice"] != "shimmer" { t.Fatal("approved voice not selected") }
	t.Setenv("WORKTIC_AGENT_REALTIME_VOICE", "unsafe-voice")
	if _, err = agentBuilderA6Config(); err == nil {t.Fatal("voice should be allowlisted")}
	t.Setenv("WORKTIC_AGENT_REALTIME_VOICE", "coral")
	t.Setenv("WORKTIC_AGENT_REALTIME_MODEL", "unsafe-custom")
	if _, err = agentBuilderA6Config(); err == nil {
		t.Fatal("model should be allowlisted")
	}
}
func TestAgentBuilderA6PilotOptIn(t *testing.T) {
	t.Setenv("WORKTIC_AGENT_REALTIME_PILOT_TENANTS", "")
	if agentBuilderA6PilotAllowsTenant(1, "admin") {
		t.Fatal("default must deny")
	}
	t.Setenv("WORKTIC_AGENT_REALTIME_PILOT_TENANTS", "2, 8")
	if !agentBuilderA6PilotAllowsTenant(8, "admin") || agentBuilderA6PilotAllowsTenant(9, "admin") {
		t.Fatal("tenant isolation")
	}
	t.Setenv("WORKTIC_AGENT_REALTIME_SUPERADMIN_PILOT", "1")
	if !agentBuilderA6PilotAllowsTenant(9, "superadmin") || agentBuilderA6PilotAllowsTenant(9, "owner") {
		t.Fatal("superadmin pilot isolation")
	}
	t.Setenv("WORKTIC_AGENT_REALTIME_ENABLED", "")
	if agentBuilderA6Enabled() {
		t.Fatal("must opt in")
	}
}
func TestAgentBuilderA6RateLimit(t *testing.T) {
	tenant := int64(987654321)
	now := time.Now()
	for i := 0; i < 4; i++ {
		if !agentBuilderA6Admit(tenant, now) {
			t.Fatal("unexpected block", i)
		}
	}
	if agentBuilderA6Admit(tenant, now) {
		t.Fatal("allowed fifth session")
	}
	if !agentBuilderA6Admit(tenant, now.Add(time.Hour+time.Second)) {
		t.Fatal("rate limit did not expire")
	}
}
func TestAgentBuilderA6Exchange(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.Header.Get("Authorization") != "Bearer fake-key" {
			t.Error("auth failed")
		}
		if !strings.HasPrefix(r.Header.Get("OpenAI-Safety-Identifier"), "worktic") {
			t.Error("safety id missing")
		}
		med, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || med != "multipart/form-data" {
			t.Error(err)
			return
		}
		_ = params
		if err = r.ParseMultipartForm(1 << 20); err != nil {
			t.Error(err)
			return
		}
		if !strings.HasPrefix(r.FormValue("sdp"), "v=0") || !strings.Contains(r.FormValue("session"), "realtime") {
			t.Error("invalid upstream request")
		}
		w.Header().Set("Content-Type", "application/sdp")
		w.WriteHeader(201)
		_, _ = io.WriteString(w, "v=0\r\no=- 111 1 IN IP4 127.0.0.1\r\ns=-\r\nt=0 0\r\n")
	}))
	defer provider.Close()
	answer, err := agentBuilderA6Exchange(context.Background(), provider.Client(), provider.URL, "fake-key", "worktic-test", []byte("v=0\r\no=- 123 1 IN IP4 127.0.0.1\r\ns=-\r\nt=0 0\r\n"), []byte(`{"type":"realtime"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(answer), "v=0") {
		t.Fatal("bad SDP")
	}
	_ = os.ErrNotExist
}

func TestAgentBuilderA6ProviderErrorSanitization(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(429)
		_,_ = io.WriteString(w, `{"error":{"code":"insufficient_quota","type":"rate_limit_error","message":"SECRET NEVER LOG"}}`)
	}))
	defer provider.Close()
	_,err:=agentBuilderA6Exchange(context.Background(),provider.Client(),provider.URL,"fake-key","worktic-test",[]byte("v=0\r\no=- 123 1 IN IP4 127.0.0.1\r\ns=-\r\nt=0 0\r\n"),[]byte(`{"type":"realtime"}`))
	var pe *agentBuilderA6ProviderError
	if !errors.As(err,&pe) || pe.Status!=429 || pe.Code!="insufficient_quota" || strings.Contains(pe.Error(),"SECRET") {t.Fatalf("bad provider error: %v",err)}
	if agentBuilderA6SafeCode("bad <thing>")!="unknown" {t.Fatal("unsafe code accepted")}
}
func TestAgentBuilderA6RetryDoesNotExhaustPilot(t *testing.T) {
	tenant:=int64(998877665)
	for i:=0;i<6;i++ {
		start:=time.Now().Add(time.Duration(i)*time.Microsecond)
		if !agentBuilderA6Admit(tenant,start){t.Fatalf("failed retry %d blocked",i)}
		agentBuilderA6Release(tenant,start)
	}
}

func TestAgentBuilderA6TypedMultipartHeaders(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mr, err := r.MultipartReader()
		if err != nil { t.Error(err); return }
		want := []struct{ name, media string }{{"sdp", "application/sdp"}, {"session", "application/json"}}
		for _, item := range want {
			p, err := mr.NextPart()
			if err != nil { t.Error(err); return }
			if p.FormName() != item.name || p.Header.Get("Content-Type") != item.media {
				t.Errorf("multipart metadata %s %s", p.FormName(), p.Header.Get("Content-Type"))
			}
			data, _ := io.ReadAll(io.LimitReader(p, 64<<10))
			if len(data) == 0 { t.Error("empty multipart field") }
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, "v=0\r\no=- 111 1 IN IP4 127.0.0.1\r\ns=-\r\nt=0 0\r\n")
	}))
	defer provider.Close()
	answer, err := agentBuilderA6Exchange(context.Background(), provider.Client(), provider.URL, "fake-key", "worktic-test", []byte("v=0\r\no=- 123 1 IN IP4 127.0.0.1\r\ns=-\r\nt=0 0\r\n"), []byte(`{"type":"realtime"}`))
	if err != nil { t.Fatal(err) }
	if !strings.HasPrefix(string(answer), "v=0") { t.Fatal("did not return SDP answer") }
}

func TestAgentBuilderA6InvalidSuccessfulResponseIsSanitized(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"error":{"message":"DO NOT LOG THIS PRIVATE PROVIDER DATA"}}`)
	}))
	defer provider.Close()
	_, err := agentBuilderA6Exchange(context.Background(), provider.Client(), provider.URL, "fake-key", "worktic-test", []byte("v=0\r\no=- 123 1 IN IP4 127.0.0.1\r\ns=-\r\nt=0 0\r\n"), []byte(`{"type":"realtime"}`))
	if err == nil || !strings.Contains(err.Error(), "upstream_http=201") || !strings.Contains(err.Error(), "upstream_media=application/json") || strings.Contains(err.Error(), "DO NOT LOG") {
		t.Fatalf("bad diagnostic: %v", err)
	}
}
