package main

import (
	"errors"
	"testing"
)

func TestAutomationResumedTerminalV273552(t *testing.T) {
	gotWait, gotExec, gotMessage := automationResumedOutcomeV273552(nil)
	if gotWait != "completed" || gotExec != "success" || gotMessage != "" {
		t.Fatalf("éxito incorrecto: %s %s %q", gotWait, gotExec, gotMessage)
	}
	gotWait, gotExec, gotMessage = automationResumedOutcomeV273552(errors.New("error de prueba"))
	if gotWait != "failed" || gotExec != "failed" || gotMessage == "" {
		t.Fatalf("fallo incorrecto: %s %s %q", gotWait, gotExec, gotMessage)
	}
	a := &App{}
	if err := a.finishResumedAutomationWaitV273552(automationWaitItemV273538{}, nil); err == nil {
		t.Fatal("aceptó referencias inválidas")
	}
}
