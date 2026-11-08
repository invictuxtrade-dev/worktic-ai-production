package main

import "testing"

func TestB103BGraphVersion(t *testing.T) {
	for _, x := range []struct {
		s  string
		ok bool
	}{
		{"v26.0", true}, {"v27.0", true}, {"v0.0", true}, {"http://bad", false}, {"v26.0/abc", false}, {"v26", false}, {"v2.3.4", false},
	} {
		if got := validB103BGraphVersion(x.s); got != x.ok {
			t.Fatalf("%q: %v, want %v", x.s, got, x.ok)
		}
	}
}
func TestB103BSubscriptionParse(t *testing.T) {
	fields, subscribed, valid := parseB103BSubscription([]byte(`{"data":[{"subscribed_fields":["messages","messaging_postbacks","private_secret"]}]}`))
	if !valid || !subscribed || !containsB103B(fields, "messages") || containsB103B(fields, "private_secret") {
		t.Fatal("incorrect filtered subscription")
	}
	_, subscribed, valid = parseB103BSubscription([]byte(`{"data":[]}`))
	if !valid || subscribed {
		t.Fatal("empty subscription must be reported")
	}
	_, _, valid = parseB103BSubscription([]byte(`not-json`))
	if valid {
		t.Fatal("malformed response accepted")
	}
}
