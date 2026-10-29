package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const catalogEvidenceMaxV2735161 = 4 << 20 // 4 MiB

// catalogSaleEvidenceV2735161 stores payment proof privately in the tenant-scoped DB.
// It does not verify the payment, and is not reachable by a public catalog token.
func (a *App) catalogSaleEvidenceV2735161(w http.ResponseWriter, r *http.Request) {
	tenant, actor, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, http.StatusUnauthorized)
		return
	}
	if !canManageCatalog(actor.Role) {
		writeError(w, errors.New("sin permiso de comprobantes"), http.StatusForbidden)
		return
	}
	switch r.Method {
	case http.MethodGet:
		id, _ := strconv.ParseInt(r.URL.Query().Get("sale_id"), 10, 64)
		if id <= 0 {
			writeError(w, errors.New("pedido inválido"), 400)
			return
		}
		var data []byte
		var typ string
		err = a.db.QueryRow(`SELECT e.content,e.media_type FROM catalog_sale_evidence_v2735161 e JOIN catalog_sales_v273516 s ON s.tenant_id=e.tenant_id AND s.id=e.sale_id WHERE e.tenant_id=? AND e.sale_id=?`, tenant, id).Scan(&data, &typ)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", typ)
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="comprobante-pedido-%d%s"`, id, catalogEvidenceExtV2735161(typ)))
		w.Header().Set("Cache-Control", "no-store, private")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		_, _ = w.Write(data)
	case http.MethodPost:
		r.Body = http.MaxBytesReader(w, r.Body, catalogEvidenceMaxV2735161+(1<<20))
		if err = r.ParseMultipartForm(catalogEvidenceMaxV2735161 + (1 << 20)); err != nil {
			writeError(w, errors.New("archivo demasiado grande o formulario inválido"), 413)
			return
		}
		saleID, _ := strconv.ParseInt(strings.TrimSpace(r.FormValue("sale_id")), 10, 64)
		if saleID <= 0 {
			writeError(w, errors.New("pedido inválido"), 400)
			return
		}
		var status string
		if err = a.db.QueryRow(`SELECT status FROM catalog_sales_v273516 WHERE tenant_id=? AND id=?`, tenant, saleID).Scan(&status); err != nil {
			http.NotFound(w, r)
			return
		}
		if status != "pending_review" {
			writeError(w, errors.New("solo se pueden adjuntar comprobantes a ventas pendientes"), 409)
			return
		}
		file, _, err := r.FormFile("proof")
		if err != nil {
			writeError(w, errors.New("adjunta un comprobante"), 400)
			return
		}
		defer file.Close()
		bytes, err := io.ReadAll(io.LimitReader(file, catalogEvidenceMaxV2735161+1))
		if err != nil || len(bytes) == 0 || len(bytes) > catalogEvidenceMaxV2735161 {
			writeError(w, errors.New("comprobante inválido o demasiado grande (máximo 4 MB)"), 400)
			return
		}
		mediaType := http.DetectContentType(bytes)
		if mediaType != "image/jpeg" && mediaType != "image/png" && mediaType != "image/webp" && mediaType != "application/pdf" {
			writeError(w, errors.New("solo JPG, PNG, WEBP o PDF"), 400)
			return
		}
		digest := sha256.Sum256(bytes)
		tx, err := a.db.Begin()
		if err != nil {
			writeError(w, err, 500)
			return
		}
		defer tx.Rollback()
		// Recheck the sale status inside the write transaction.
		if err = tx.QueryRow(`SELECT status FROM catalog_sales_v273516 WHERE tenant_id=? AND id=?`, tenant, saleID).Scan(&status); err != nil || status != "pending_review" {
			writeError(w, errors.New("el pedido ya no está pendiente"), 409)
			return
		}
		now := time.Now().UTC().Format(time.RFC3339)
		_, err = tx.Exec(`INSERT INTO catalog_sale_evidence_v2735161(tenant_id,sale_id,media_type,content,sha256,uploaded_by,uploaded_at) VALUES(?,?,?,?,?,?,?) ON CONFLICT(tenant_id,sale_id) DO UPDATE SET media_type=excluded.media_type,content=excluded.content,sha256=excluded.sha256,uploaded_by=excluded.uploaded_by,uploaded_at=excluded.uploaded_at`, tenant, saleID, mediaType, bytes, hex.EncodeToString(digest[:]), actor.ID, now)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		_, err = tx.Exec(`INSERT INTO catalog_audit_v273516(tenant_id,actor_id,action,entity_type,entity_id,details,created_at) VALUES(?,?,?,?,?,?,?)`, tenant, actor.ID, "sale_evidence_uploaded", "sale", saleID, "Archivo privado SHA256="+hex.EncodeToString(digest[:]), now)
		if err != nil {
			writeError(w, err, 500)
			return
		}
		if err = tx.Commit(); err != nil {
			writeError(w, err, 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "sale_id": saleID, "has_evidence": true, "payment_verified": false})
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Método no permitido", 405)
	}
}

func catalogEvidenceExtV2735161(mediaType string) string {
	switch mediaType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "application/pdf":
		return ".pdf"
	default:
		return ".bin"
	}
}
