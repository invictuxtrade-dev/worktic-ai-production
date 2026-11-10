package main

import (
	"encoding/json"
	"testing"
)

func TestAttractionP1BSourceGate(t *testing.T) {
	resp := attractionSearchResponseP1B{Status: "completed"}
	raw := []byte(`{"status":"completed","output":[{"type":"web_search_call","action":{"sources":[{"url":"https://clinic.example/salud"}]}},{"type":"message","content":[{"type":"output_text","text":"{\"candidates\":[{\"name\":\"Clinica Ejemplo\",\"kind\":\"business\",\"website\":\"https://clinic.example\",\"source_url\":\"https://clinic.example/salud\",\"signal\":\"Empresa en el sector\"},{\"name\":\"Inventada\",\"kind\":\"business\",\"website\":\"https://wrong.example\",\"source_url\":\"https://wrong.example\",\"signal\":\"No existe\"}]}"}]}]}`)
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatal(err)
	}
	xs, err := attractionP1BVerifiedCandidates(resp)
	if err != nil {
		t.Fatal(err)
	}
	if len(xs) != 1 || xs[0].Name != "Clinica Ejemplo" {
		t.Fatalf("unexpected candidates: %+v", xs)
	}
	// The same payload without a search call must fail closed.
	resp.Output = resp.Output[1:]
	if _, err = attractionP1BVerifiedCandidates(resp); err == nil {
		t.Fatal("accepted ungrounded output")
	}
}

func TestAttractionP1BDedupe(t *testing.T) {
	a := attractionWebCandidateP1B{Name: "Clinica Uno", Website: "https://clinic.example/a"}
	b := a
	b.Website = "https://clinic.example/b"
	if attractionP1BDedupeKey(a, "Bogota", "Colombia") != attractionP1BDedupeKey(b, "Bogota", "Colombia") {
		t.Fatal("website identity changed with path")
	}
	if attractionP1BDedupeKey(a, "Bogota", "Colombia") == attractionP1BDedupeKey(a, "Cali", "Colombia") {
		t.Fatal("cross-city identity")
	}
}

func TestAttractionP1BInputs(t *testing.T) {
	if attractionP1BCleanMode("other") != "" {
		t.Fatal("invalid campaign mode")
	}
	for _, s := range []string{"javascript:alert(1)", "file:///etc/passwd", "http://localhost/"} {
		if attractionP1BCanonicalURL(s) != "" {
			t.Fatalf("unsafe URL accepted: %s", s)
		}
	}
}
