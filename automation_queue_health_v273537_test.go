package main

import "testing"

func TestAutomationQueueCountsV273537(t *testing.T) {
	h := automationQueueHealthV273537{}
	for _, item := range []struct {
		state string
		n     int64
	}{
		{"pending", 3}, {"processing", 2}, {"failed", 4}, {"processed", 5},
		{"pending", 1}, {"unexpected", 100}, {"failed", -20},
	} {
		applyAutomationQueueCountV273537(&h, item.state, item.n)
	}
	if h.Pending != 4 || h.Processing != 2 || h.Failed != 4 || h.Processed != 5 {
		t.Fatalf("unexpected queue aggregation: %+v", h)
	}
}
func TestAutomationQueueCountsNilV273537(t *testing.T) {
	applyAutomationQueueCountV273537(nil, "pending", 1)
}
