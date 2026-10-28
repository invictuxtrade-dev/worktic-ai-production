package main

import (
	"errors"
	"fmt"
)

// securityTenantAgentActive only accepts an active agent owned by this tenant.
// A missing/foreign/draft agent MUST fail closed; never use a global fallback.
func (a *App) securityTenantAgentActive(tenant, agent int64) error {
	if tenant <= 0 || agent <= 0 {
		return errors.New("empresa y agente requeridos")
	}
	var status string
	if err := a.db.QueryRow(`SELECT status FROM ai_agents WHERE id=? AND tenant_id=?`, agent, tenant).Scan(&status); err != nil || status != "active" {
		return errors.New("agente no pertenece a la empresa o no está activo")
	}
	return nil
}

// securityTenantResolveAgent never follows a route to a foreign/draft agent.
func (a *App) securityTenantResolveAgent(tenant, assigned int64, channel, text string) (int64, error) {
	if tenant <= 0 {
		return 0, errors.New("empresa requerida")
	}
	if assigned > 0 {
		if err := a.securityTenantAgentActive(tenant, assigned); err != nil {
			return 0, err
		}
		return assigned, nil
	}
	agent, err := a.resolveAgent(tenant, channel, "", text, 0, 0, 0)
	if err != nil || agent <= 0 {
		return 0, errors.New("configura un agente activo de tu empresa")
	}
	if err := a.securityTenantAgentActive(tenant, agent); err != nil {
		return 0, err
	}
	return agent, nil
}

// No silent fallback to old global worktic_agent under multi-tenancy.
func (a *App) securityLegacyAgentSingleTenant() error {
	var total int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM tenants`).Scan(&total); err != nil || total != 1 {
		return fmt.Errorf("el agente legado global está deshabilitado para %d empresas", total)
	}
	return nil
}
