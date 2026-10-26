package main

// V27.34.6: read-only creative-credit movements. All queries are tenant scoped;
// no prompts, personal messages, billing addresses, or API secrets are returned.
import (
    "net/http"
    "strings"
)

type studioWalletMovementV27346 struct {
    ID string `json:"id"`
    Kind string `json:"kind"`
    Operation string `json:"operation"`
    Status string `json:"status"`
    Credits int64 `json:"credits"`
    At string `json:"at"`
}

func studioWalletLabelV27346(operation string) string {
    switch strings.ToLower(operation) {
    case "image": return "Imagen IA"
    case "video": return "Video IA"
    case "narration": return "Narración IA"
    case "copy": return "Texto IA"
    case "storyboard": return "Guion IA"
    default: return "Generación IA"
    }
}

func (a *App) commercialMovementsV27346Handler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet { http.Error(w,"Método no permitido",http.StatusMethodNotAllowed);return }
    tenantID, _, err := a.tenantFor(r)
    if err != nil || tenantID <= 0 { http.Error(w,"Inicia sesión",http.StatusUnauthorized);return }
    result := make([]studioWalletMovementV27346,0,80)
    holds, err := a.db.Query(`SELECT id, operation, plan_credits+bought_credits, status, created_at FROM commercial_holds_v2721 WHERE tenant_id=? ORDER BY created_at DESC LIMIT 50`,tenantID)
    if err != nil { http.Error(w,"No se pudieron consultar los movimientos",http.StatusServiceUnavailable);return }
    for holds.Next() {
        var m studioWalletMovementV27346
        if err=holds.Scan(&m.ID,&m.Operation,&m.Credits,&m.Status,&m.At);err != nil { break }
        m.Kind="generation"
        m.Operation=studioWalletLabelV27346(m.Operation)
        result=append(result,m)
    }
    if err == nil { err=holds.Err() }
    holds.Close()
    if err != nil { http.Error(w,"No se pudieron consultar los movimientos",http.StatusServiceUnavailable);return }
    topups,err:=a.db.Query(`SELECT id, credits, status, created_at FROM commercial_topups_v2721 WHERE tenant_id=? ORDER BY created_at DESC LIMIT 50`,tenantID)
    if err != nil { http.Error(w,"No se pudieron consultar las recargas",http.StatusServiceUnavailable);return }
    for topups.Next() {
        var m studioWalletMovementV27346
        if err=topups.Scan(&m.ID,&m.Credits,&m.Status,&m.At);err != nil { break }
        m.Kind="topup"
        m.Operation="Recarga de créditos"
        result=append(result,m)
    }
    if err == nil { err=topups.Err() }
    topups.Close()
    if err != nil { http.Error(w,"No se pudieron consultar las recargas",http.StatusServiceUnavailable);return }
    commercialSetJSON(w,http.StatusOK,map[string]any{"movements":result})
}
