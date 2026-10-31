package workticactivation

import (
	"encoding/base64"
	"testing"
)

func TestNewTokenUniqueAndValid(t *testing.T) {
	a, hashA, err := newToken()
	if err != nil {
		t.Fatal(err)
	}
	b, hashB, err := newToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != 43 || len(b) != 43 || a == b {
		t.Fatal("token not unique or incorrect length")
	}
	if len(hashA) != 32 || len(hashB) != 32 {
		t.Fatal("digest length")
	}
	if _, err := base64.RawURLEncoding.DecodeString(a); err != nil {
		t.Fatal(err)
	}
}
func TestSecureIndexInRange(t *testing.T) {
	for _, n := range []int{1, 2, 3, 6, 16, 255} {
		for i := 0; i < 50; i++ {
			x, err := secureIndex(n)
			if err != nil || x < 0 || x >= n {
				t.Fatalf("bad index: %d %v", x, err)
			}
		}
	}
}
func TestSecureIndexRejectsBounds(t *testing.T) {
	for _, n := range []int{-1, 0, 257} {
		if _, err := secureIndex(n); err == nil {
			t.Fatalf("expected error for %d", n)
		}
	}
}
