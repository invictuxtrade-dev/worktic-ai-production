package main

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// V27.35.49: a bounded number of IA nodes per workflow prevents unexpected
// repeated inference in business workflows. This is NOT a second billing system.
func automationAIMaxNodesV273549() int {
	const defaultLimit = 5
	raw := os.Getenv("WORKTIC_AUTOMATION_MAX_AI_NODES")
	if raw == "" {
		return defaultLimit
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > 50 {
		return defaultLimit
	}
	return n
}

func automationValidateAINodeTypesV273549(types []string, maxAllowed int) error {
	if maxAllowed < 1 || maxAllowed > 50 {
		return fmt.Errorf("límite IA inválido")
	}
	used := 0
	for _, t := range types {
		if t == "ai_task" {
			used++
		}
	}
	if used > maxAllowed {
		return fmt.Errorf("el flujo contiene %d bloques IA; máximo permitido %d", used, maxAllowed)
	}
	return nil
}

func automationAIMonthBoundsV273549(now time.Time) (time.Time, time.Time) {
	utc := now.UTC()
	start := time.Date(utc.Year(), utc.Month(), 1, 0, 0, 0, 0, time.UTC)
	return start, start.AddDate(0, 1, 0)
}
