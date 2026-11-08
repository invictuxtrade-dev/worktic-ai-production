package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// This is an ephemeral, redacted diagnostic. It never stores or displays token,
// numeric account IDs, business IDs, user IDs or untrusted Graph error bodies.
type b9SUATSignals struct {
	TokenType         string
	Valid             string
	AppMatch          string
	BasicScope        string
	GranularIGTargets string
	Subject           string
}

func b9SafeTokenType(s string) string {
	switch strings.ToUpper(s) {
	case "SYSTEM_USER":
		return "SYSTEM_USER"
	case "USER":
		return "USER"
	case "PAGE":
		return "PAGE"
	case "APP":
		return "APP"
	default:
		return "OTHER"
	}
}

func b9SignalsString(s b9SUATSignals) string {
	return "[B9_SUAT_DIAG;type=" + s.TokenType + ";valid=" + s.Valid + ";app=" + s.AppMatch + ";basic=" + s.BasicScope + ";ig_targets=" + s.GranularIGTargets + ";subject=" + s.Subject + "]"
}

func (a *App) instagramB9SUATDiagnostic(parent context.Context, token string) string {
	s := b9SUATSignals{"UNKNOWN", "UNKNOWN", "UNKNOWN", "UNKNOWN", "UNKNOWN", "UNKNOWN"}
	if token == "" || a.cfg.MetaAppID == "" || a.cfg.MetaAppSecret == "" {
		return b9SignalsString(s)
	}
	ctx, cancel := context.WithTimeout(parent, 6*time.Second)
	defer cancel()
	// Use POST body for input_token so it never appears in URL/query access logs.
	endpoint := "https://graph.facebook.com/" + a.cfg.MetaGraphVersion + "/debug_token"
	form := url.Values{"input_token": {token}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return b9SignalsString(s)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+a.cfg.MetaAppID+"|"+a.cfg.MetaAppSecret)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return b9SignalsString(s)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return b9SignalsString(s)
	}
	var out struct {
		Data struct {
			Type           string   `json:"type"`
			IsValid        bool     `json:"is_valid"`
			AppID          string   `json:"app_id"`
			UserID         string   `json:"user_id"`
			Scopes         []string `json:"scopes"`
			GranularScopes []struct {
				Scope     string   `json:"scope"`
				TargetIDs []string `json:"target_ids"`
			} `json:"granular_scopes"`
		} `json:"data"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 1048576)).Decode(&out) != nil {
		return b9SignalsString(s)
	}
	s.TokenType = b9SafeTokenType(out.Data.Type)
	if out.Data.IsValid {
		s.Valid = "YES"
	} else {
		s.Valid = "NO"
	}
	if out.Data.AppID == a.cfg.MetaAppID {
		s.AppMatch = "YES"
	} else {
		s.AppMatch = "NO"
	}
	if out.Data.UserID != "" {
		s.Subject = "PRESENT"
	} else {
		s.Subject = "MISSING"
	}
	s.BasicScope = "NO"
	for _, scope := range out.Data.Scopes {
		if scope == "instagram_basic" {
			s.BasicScope = "YES"
		}
	}
	s.GranularIGTargets = "NONE_REPORTED"
	for _, scope := range out.Data.GranularScopes {
		if scope.Scope == "instagram_basic" {
			if len(scope.TargetIDs) > 0 {
				s.GranularIGTargets = "PRESENT"
			} else {
				s.GranularIGTargets = "EMPTY"
			}
			break
		}
	}
	return b9SignalsString(s)
}
