package main

import "testing"

func TestAttractionP1Score(t *testing.T) {
	x := AttractionCandidateP1{Name: "Clínica Ejemplo", Kind: "business", City: "Bogotá", Country: "Colombia", Niche: "salud", Website: "https://example.org", Signal: "Busco proveedor de sistema"}
	if attractionP1Score(x) != 63 {
		t.Fatalf("unexpected score %d", attractionP1Score(x))
	}
}
func TestAttractionP1Identity(t *testing.T) {
	a := AttractionCandidateP1{Name: "Empresa", Kind: "business", City: "Cali", Country: "Colombia"}
	b := a
	b.City = "Bogotá"
	if attractionP1SourceKey(a) == attractionP1SourceKey(b) {
		t.Fatal("identity collision")
	}
}
func TestAttractionP1UnsafeURL(t *testing.T) {
	for _, s := range []string{"javascript:alert(1)", "https://user:pass@example.com", "file:///etc/passwd"} {
		if _, err := attractionP1SourceURL(s); err == nil {
			t.Fatalf("unsafe URL allowed: %s", s)
		}
	}
}
