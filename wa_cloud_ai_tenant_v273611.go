package main

// B1 V27.36.11: controlled customer rollout of WhatsApp Cloud AI.
// Fail closed: each tenant's opt-in is bound to its persisted Meta WABA and phone ID.
// The original Worktic pilot allowlist continues to work unchanged.

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func initWACloudAITenantV273611Schema(db *DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS wa_cloud_ai_tenant_optins_v273611 (
		tenant_id BIGINT NOT NULL,
		connection_id BIGINT NOT NULL,
		waba_id TEXT NOT NULL,
		phone_number_id TEXT NOT NULL,
		enabled INTEGER NOT NULL DEFAULT 0,
		updated_by BIGINT NOT NULL,
		updated_at TEXT NOT NULL,
		PRIMARY KEY(tenant_id,connection_id)
	)`)
	return err
}

func waCloudAICustomerRolloutV273611() bool {
	return strings.EqualFold(strings.TrimSpace(env("WORKTIC_WA_CLOUD_AI_CUSTOMER_ROLLOUT", "false")), "true")
}

// These values come from the tenant-scoped channel row, never from inbound payloads.
func waCloudAIChannelIdentityV273611(c ChannelConnection) (string, string, bool) {
	if c.ID <= 0 || c.TenantID <= 0 || c.Type != "whatsapp_cloud" || c.Status != "connected" || c.AssignedAgentID <= 0 {
		return "", "", false
	}
	var cfg whatsappCloudConfig
	if json.Unmarshal([]byte(c.ConfigJSON), &cfg) != nil {
		return "", "", false
	}
	waba := strings.TrimSpace(cfg.WABAID)
	phone := strings.TrimSpace(cfg.PhoneNumberID)
	if !waCloudAIAllDigitsV273610(waba) || !waCloudAIAllDigitsV273610(phone) || phone != strings.TrimSpace(c.ExternalAccountID) ||
		waCloudAINormalizePhoneV273610(cfg.DisplayPhone) == "" {
		return "", "", false
	}
	return waba, phone, true
}

func (a *App) waCloudAICustomerOptedInV273611(c ChannelConnection) bool {
	waba, phone, ok := waCloudAIChannelIdentityV273611(c)
	if !ok {
		return false
	}
	var enabled int
	err := a.db.QueryRow(`SELECT enabled FROM wa_cloud_ai_tenant_optins_v273611
		WHERE tenant_id=? AND connection_id=? AND waba_id=? AND phone_number_id=?`,
		c.TenantID, c.ID, waba, phone).Scan(&enabled)
	return err == nil && enabled == 1
}

func (a *App) waCloudAIAllowedConnectionV273611(c ChannelConnection) bool {
	if !waCloudAIEnabledV273609() {
		return false
	}
	if waCloudAIAllowedConnectionV273610(c) { // Never weaken existing pilot.
		return true
	}
	return waCloudAICustomerRolloutV273611() && a.waCloudAICustomerOptedInV273611(c)
}

func (a *App) whatsappCloudAIPermissionV273611Handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		writeError(w, errors.New("método no permitido"), http.StatusMethodNotAllowed)
		return
	}
	tid, u, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, http.StatusUnauthorized)
		return
	}
	var input struct {
		ConnectionID int64 `json:"connection_id"`
		Enabled      bool  `json:"enabled"`
	}
	if r.Method == http.MethodGet {
		input.ConnectionID, err = strconv.ParseInt(r.URL.Query().Get("connection_id"), 10, 64)
		if err != nil {
			writeError(w, errors.New("connection_id inválido"), 400)
			return
		}
	} else {
		if u.Role != "owner" && u.Role != "admin" && u.Role != "superadmin" {
			writeError(w, errors.New("solo el administrador puede cambiar la atención IA"), 403)
			return
		}
		err = json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&input)
		if err != nil {
			writeError(w, errors.New("solicitud inválida"), 400)
			return
		}
	}
	if input.ConnectionID <= 0 {
		writeError(w, errors.New("connection_id obligatorio"), 400)
		return
	}
	var c ChannelConnection
	err = a.db.QueryRow(`SELECT id,tenant_id,public_id,type,name,status,external_account_id,assigned_agent_id,config_json,encrypted_credentials,last_connected_at,last_disconnected_at,last_message_at,last_error,created_at,updated_at FROM channel_connections WHERE id=? AND tenant_id=? AND type='whatsapp_cloud'`, input.ConnectionID, tid).
		Scan(&c.ID, &c.TenantID, &c.PublicID, &c.Type, &c.Name, &c.Status, &c.ExternalAccountID, &c.AssignedAgentID, &c.ConfigJSON, &c.EncryptedCredentials, &c.LastConnectedAt, &c.LastDisconnectedAt, &c.LastMessageAt, &c.LastError, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		writeError(w, errors.New("canal WhatsApp de esta empresa no encontrado"), 404)
		return
	}
	pilot := waCloudAIAllowedConnectionV273610(c)
	waba, phone, eligible := waCloudAIChannelIdentityV273611(c)
	if r.Method == http.MethodPost {
		if pilot {
			writeError(w, errors.New("el canal piloto se controla únicamente desde la configuración central"), 409)
			return
		}
		if input.Enabled {
			if !waCloudAICustomerRolloutV273611() || !waCloudAIEnabledV273609() {
				writeError(w, errors.New("las respuestas IA para clientes aún no están habilitadas por Worktic"), 409)
				return
			}
			if !eligible || !a.cfg.BackgroundWorkersEnabled || a.openAIKey() == "" {
				writeError(w, errors.New("el canal, el motor IA o el agente no están listos"), 409)
				return
			}
			if err = a.securityTenantAgentActive(tid, c.AssignedAgentID); err != nil {
				writeError(w, errors.New("asigna un agente IA activo de esta empresa"), 409)
				return
			}
		}
		// Disabling the permission is always allowed, even after a failed connection.
		if !eligible && input.Enabled {
			writeError(w, errors.New("número oficial no verificado"), 409)
			return
		}
		stamp := time.Now().UTC().Format(time.RFC3339)
		value := 0
		if input.Enabled {
			value = 1
		}
		_, err = a.db.Exec(`INSERT INTO wa_cloud_ai_tenant_optins_v273611
			(tenant_id,connection_id,waba_id,phone_number_id,enabled,updated_by,updated_at)
			VALUES(?,?,?,?,?,?,?) ON CONFLICT(tenant_id,connection_id)
			DO UPDATE SET waba_id=excluded.waba_id,phone_number_id=excluded.phone_number_id,enabled=excluded.enabled,updated_by=excluded.updated_by,updated_at=excluded.updated_at`,
			tid, c.ID, waba, phone, value, u.ID, stamp)
		if err != nil {
			writeError(w, errors.New("no se pudo actualizar la atención IA"), 500)
			return
		}
		activity := "wa_cloud_customer_ai_disabled"
		if input.Enabled {
			activity = "wa_cloud_customer_ai_enabled"
		}
		_, _ = a.db.Exec(`INSERT INTO channel_audit(tenant_id,connection_id,user_id,action,detail,created_at)
			VALUES(?,?,?,?,?,?)`, tid, c.ID, u.ID, activity, "tenant_scoped", stamp)
	}
	writeJSON(w, map[string]any{
		"ok":              true,
		"connection_id":   c.ID,
		"pilot":           pilot,
		"rollout_enabled": waCloudAICustomerRolloutV273611(),
		"engine_enabled":  waCloudAIEnabledV273609(),
		"eligible":        eligible,
		"enabled":         a.waCloudAIAllowedConnectionV273611(c),
		"opted_in":        a.waCloudAICustomerOptedInV273611(c),
		"has_agent":       c.AssignedAgentID > 0,
	})
}
