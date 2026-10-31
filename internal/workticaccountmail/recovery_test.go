package workticaccountmail

import (
	"strings"
	"testing"
)

func TestToken(t *testing.T) {
	tok, hash, e := randomToken()
	if e != nil || len(tok) != 43 || len(hash) != 32 {
		t.Fatal("token generation")
	}
	d, ok := digestToken(tok)
	if !ok || string(d) != string(hash) {
		t.Fatal("digest mismatch")
	}
	if _, ok := digestToken("bad"); ok {
		t.Fatal("accepted invalid token")
	}
}
func TestEmailTemplate(t *testing.T) {
	_, html, _ := WelcomeEmail("Ana <script>", "https://workticai.com", "es")
	if strings.Contains(html, "<script>") {
		t.Fatal("template injection")
	}
	if !strings.Contains(html, "https://workticai.com/login.html") {
		t.Fatal("missing link")
	}
}
func TestSenderValidation(t *testing.T) {
	_, e := (MailConfig{Provider: "smtp", From: "test@workticai.com"}).Sender()
	if e == nil {
		t.Fatal("missing credentials accepted")
	}
}
