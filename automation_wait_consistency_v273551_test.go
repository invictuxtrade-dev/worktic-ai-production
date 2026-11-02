package main

import (
	"errors"
	"strings"
	"testing"
)

func TestAutomationWaitReviewRejectsInvalidReferencesV273551(t *testing.T) {
	invalid := []automationWaitItemV273538{
		{},
		{id: 0, tid: 1, wid: 1, eid: 1},
		{id: 1, tid: 0, wid: 1, eid: 1},
		{id: 1, tid: 1, wid: 0, eid: 1},
		{id: 1, tid: 1, wid: 1, eid: 0},
	}
	for _, x := range invalid {
		// Invalid identifiers must stop before any transaction or DB access.
		err := (&App{}).markAutomationWaitFailureForReviewV273551(x, errors.New("test"))
		if err == nil || !strings.Contains(err.Error(), "referencias") {
			t.Fatalf("identificadores inválidos aceptados %+v: %v", x, err)
		}
	}
}
