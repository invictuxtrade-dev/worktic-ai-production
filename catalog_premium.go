package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type CatalogProduct struct {
	ID               int64    `json:"id"`
	TenantID         int64    `json:"tenant_id"`
	Type             string   `json:"type"`
	Name             string   `json:"name"`
	SKU              string   `json:"sku"`
	Category         string   `json:"category"`
	Description      string   `json:"description"`
	Price            float64  `json:"price"`
	PromotionalPrice float64  `json:"promotional_price"`
	PromotionStarts  string   `json:"promotion_starts"`
	PromotionEnds    string   `json:"promotion_ends"`
	Currency         string   `json:"currency"`
	Stock            int      `json:"stock"`
	UnlimitedStock   bool     `json:"unlimited_stock"`
	ImageURL         string   `json:"image_url"`
	Gallery          []string `json:"gallery"`
	PaymentURL       string   `json:"payment_url"`
	DeliveryInfo     string   `json:"delivery_info"`
	FAQ              string   `json:"faq"`
	Variants         string   `json:"variants"`
	Locations        string   `json:"locations"`
	Featured         bool     `json:"featured"`
	Active           bool     `json:"active"`
	UpdatedAt        string   `json:"updated_at"`
}

func initCatalogPremiumSchema(db *DB) error {
	alters := []string{
		`ALTER TABLE crm_products ADD COLUMN tenant_id INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE crm_products ADD COLUMN type TEXT NOT NULL DEFAULT 'product'`,
		`ALTER TABLE crm_products ADD COLUMN sku TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE crm_products ADD COLUMN category TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE crm_products ADD COLUMN promotional_price REAL NOT NULL DEFAULT 0`,
		`ALTER TABLE crm_products ADD COLUMN promotion_starts TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE crm_products ADD COLUMN promotion_ends TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE crm_products ADD COLUMN unlimited_stock INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE crm_products ADD COLUMN image_url TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE crm_products ADD COLUMN gallery_json TEXT NOT NULL DEFAULT '[]'`,
		`ALTER TABLE crm_products ADD COLUMN payment_url TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE crm_products ADD COLUMN delivery_info TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE crm_products ADD COLUMN faq TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE crm_products ADD COLUMN variants TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE crm_products ADD COLUMN locations TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE crm_products ADD COLUMN featured INTEGER NOT NULL DEFAULT 0`,
	}
	for _, q := range alters {
		_, _ = db.Exec(q)
	}
	_, _ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_crm_products_tenant ON crm_products(tenant_id,active,updated_at)`)
	var firstTenant int64
	_ = db.QueryRow(`SELECT id FROM tenants ORDER BY id LIMIT 1`).Scan(&firstTenant)
	if firstTenant > 0 {
		_, _ = db.Exec(`UPDATE crm_products SET tenant_id=? WHERE tenant_id=0`, firstTenant)
	}
	return nil
}

func canManageCatalog(role string) bool {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "superadmin", "owner", "admin", "supervisor":
		return true
	default:
		return false
	}
}

func normalizeCatalogProduct(x *CatalogProduct) error {
	x.Name = strings.TrimSpace(x.Name)
	if len([]rune(x.Name)) > 160 || len([]rune(x.Description)) > 4000 {
		return errors.New("nombre o descripción demasiado largos")
	}
	if x.Name == "" {
		return errors.New("el nombre es obligatorio")
	}
	x.Type = strings.ToLower(strings.TrimSpace(x.Type))
	if x.Type != "service" {
		x.Type = "product"
	}
	x.Currency = strings.ToUpper(strings.TrimSpace(x.Currency))
	if x.Currency == "" {
		x.Currency = "COP"
	}
	if math.IsNaN(x.Price) || math.IsInf(x.Price, 0) || math.IsNaN(x.PromotionalPrice) || math.IsInf(x.PromotionalPrice, 0) || x.Price < 0 || x.PromotionalPrice < 0 || x.Stock < 0 {
		return errors.New("precio y stock no pueden ser negativos")
	}
	if x.PromotionalPrice > 0 && x.Price > 0 && x.PromotionalPrice >= x.Price {
		return errors.New("el precio promocional debe ser menor al precio normal")
	}
	from, ef := catalogDateV273516(x.PromotionStarts, false)
	to, et := catalogDateV273516(x.PromotionEnds, true)
	if ef != nil || et != nil || (!from.IsZero() && !to.IsZero() && to.Before(from)) {
		return errors.New("revisa las fechas de promoción")
	}
	if x.PaymentURL != "" {
		u, err := url.Parse(x.PaymentURL)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
			return errors.New("el enlace de pago debe ser HTTPS válido")
		}
	}
	if x.TenantID > 0 {
		if x.ImageURL != "" && !catalogMediaAllowedV273516(x.ImageURL, x.TenantID) {
			return errors.New("imagen: URL HTTPS o carga de esta empresa")
		}
		for _, v := range x.Gallery {
			if v != "" && !catalogMediaAllowedV273516(v, x.TenantID) {
				return errors.New("galería: URL inválida o de otra empresa")
			}
		}
	}
	if len(x.Gallery) > 8 {
		x.Gallery = x.Gallery[:8]
	}
	clean := make([]string, 0, len(x.Gallery))
	seen := map[string]bool{}
	for _, u := range x.Gallery {
		u = strings.TrimSpace(u)
		if u != "" && !seen[u] {
			seen[u] = true
			clean = append(clean, u)
		}
	}
	x.Gallery = clean
	return nil
}

func (a *App) catalogProductsHandler(w http.ResponseWriter, r *http.Request) {
	tid, u, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	switch r.Method {
	case http.MethodGet:
		out, e := catalogReadV273516(a.db, tid, false)
		if e != nil {
			writeError(w, e, 500)
			return
		}
		writeJSON(w, out)
	case http.MethodPost:
		if !canManageCatalog(u.Role) {
			writeError(w, errors.New("sin permiso para administrar el catálogo"), 403)
			return
		}
		if err := a.quotaAllowed(r, "products"); err != nil {
			writeError(w, err, 403)
			return
		}
		var x CatalogProduct
		if json.NewDecoder(r.Body).Decode(&x) != nil {
			writeError(w, errors.New("datos inválidos"), 400)
			return
		}
		x.TenantID = tid
		if err := normalizeCatalogProduct(&x); err != nil {
			writeError(w, err, 400)
			return
		}
		x.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		gallery, _ := json.Marshal(x.Gallery)
		query := `INSERT INTO crm_products(tenant_id,type,name,sku,category,description,price,promotional_price,promotion_starts,promotion_ends,currency,stock,unlimited_stock,image_url,gallery_json,payment_url,delivery_info,faq,variants,locations,featured,active,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`
		args := []any{tid, x.Type, x.Name, x.SKU, x.Category, x.Description, x.Price, x.PromotionalPrice, x.PromotionStarts, x.PromotionEnds, x.Currency, x.Stock, catalogBoolInt(x.UnlimitedStock), x.ImageURL, string(gallery), x.PaymentURL, x.DeliveryInfo, x.FAQ, x.Variants, x.Locations, catalogBoolInt(x.Featured), catalogBoolInt(x.Active), x.UpdatedAt}
		var res sql.Result
		var e error
		if resourceGuardV2724Enabled() && u.Role != "superadmin" {
			var cap int64
			cap, e = a.resourcePlanCapV2724(tid, "products")
			if e == nil {
				res, e = a.resourceInsertV2724(tid, "products", cap, query, args...)
			}
		} else {
			res, e = a.db.Exec(query, args...)
		}
		if e != nil {
			writeError(w, e, 500)
			return
		}
		x.ID, _ = res.LastInsertId()
		_ = a.catalogAuditV273516(tid, u.ID, "create", "product", x.ID, fmt.Sprintf("name=%s price=%.2f", x.Name, x.Price))
		writeJSON(w, x)
	case http.MethodPut:
		if !canManageCatalog(u.Role) {
			writeError(w, errors.New("sin permiso para administrar el catálogo"), 403)
			return
		}
		var x CatalogProduct
		if json.NewDecoder(r.Body).Decode(&x) != nil || x.ID <= 0 {
			writeError(w, errors.New("producto inválido"), 400)
			return
		}
		x.TenantID = tid
		if err := normalizeCatalogProduct(&x); err != nil {
			writeError(w, err, 400)
			return
		}
		x.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		gallery, _ := json.Marshal(x.Gallery)
		res, e := a.db.Exec(`UPDATE crm_products SET type=?,name=?,sku=?,category=?,description=?,price=?,promotional_price=?,promotion_starts=?,promotion_ends=?,currency=?,stock=?,unlimited_stock=?,image_url=?,gallery_json=?,payment_url=?,delivery_info=?,faq=?,variants=?,locations=?,featured=?,active=?,updated_at=? WHERE id=? AND tenant_id=?`, x.Type, x.Name, x.SKU, x.Category, x.Description, x.Price, x.PromotionalPrice, x.PromotionStarts, x.PromotionEnds, x.Currency, x.Stock, catalogBoolInt(x.UnlimitedStock), x.ImageURL, string(gallery), x.PaymentURL, x.DeliveryInfo, x.FAQ, x.Variants, x.Locations, catalogBoolInt(x.Featured), catalogBoolInt(x.Active), x.UpdatedAt, x.ID, tid)
		if e != nil {
			writeError(w, e, 500)
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			writeError(w, errors.New("producto no encontrado"), 404)
			return
		}
		_ = a.catalogAuditV273516(tid, u.ID, "update", "product", x.ID, fmt.Sprintf("name=%s price=%.2f stock=%d", x.Name, x.Price, x.Stock))
		writeJSON(w, map[string]any{"ok": true})
	case http.MethodDelete:
		if !canManageCatalog(u.Role) {
			writeError(w, errors.New("sin permiso para administrar el catálogo"), 403)
			return
		}
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		if id <= 0 {
			writeError(w, errors.New("id inválido"), 400)
			return
		}
		res, e := a.db.Exec(`UPDATE crm_products SET active=0,updated_at=? WHERE id=? AND tenant_id=?`, time.Now().UTC().Format(time.RFC3339), id, tid)
		if e != nil {
			writeError(w, e, 500)
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			writeError(w, errors.New("producto no encontrado"), 404)
			return
		}
		_ = a.catalogAuditV273516(tid, u.ID, "archive", "product", id, "producto desactivado")
		writeJSON(w, map[string]any{"ok": true})
	default:
		http.Error(w, "Método no permitido", 405)
	}
}

func catalogBoolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func validCatalogImage(header *multipart.FileHeader) (string, error) {
	ext := strings.ToLower(filepath.Ext(header.Filename))
	allowed := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true}
	if !allowed[ext] {
		return "", errors.New("formato no permitido; usa JPG, PNG o WEBP")
	}
	return ext, nil
}

func (a *App) catalogImageUploadHandler(w http.ResponseWriter, r *http.Request) {
	tid, u, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, 401)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	if !canManageCatalog(u.Role) {
		writeError(w, errors.New("sin permiso para subir imágenes"), 403)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 6<<20)
	if err := r.ParseMultipartForm(6 << 20); err != nil {
		writeError(w, errors.New("imagen demasiado grande; máximo 5 MB"), 400)
		return
	}
	file, header, err := r.FormFile("image")
	if err != nil {
		writeError(w, errors.New("selecciona una imagen"), 400)
		return
	}
	defer file.Close()
	if header.Size > 5<<20 {
		writeError(w, errors.New("imagen demasiado grande; máximo 5 MB"), 400)
		return
	}
	ext, err := validCatalogImage(header)
	if err != nil {
		writeError(w, err, 400)
		return
	}
	data, err := io.ReadAll(io.LimitReader(file, (5<<20)+1))
	if err != nil {
		writeError(w, errors.New("no fue posible leer la imagen"), 400)
		return
	}
	if len(data) > 5<<20 {
		writeError(w, errors.New("imagen demasiado grande; máximo 5 MB"), 400)
		return
	}
	info, err := inspectUploadedImage(data, ext)
	if err != nil {
		writeError(w, err, 400)
		return
	}
	if err := validateCatalogImageStandard(info); err != nil {
		writeError(w, err, 400)
		return
	}
	dir := filepath.Join(a.cfg.DataDir, "catalog_uploads", strconv.FormatInt(tid, 10))
	if err = os.MkdirAll(dir, 0755); err != nil {
		writeError(w, err, 500)
		return
	}
	name := fmt.Sprintf("%d_%d%s", time.Now().UnixNano(), tid, ext)
	target := filepath.Join(dir, name)
	if err = os.WriteFile(target, data, 0644); err != nil {
		writeError(w, err, 500)
		return
	}
	writeJSON(w, map[string]any{
		"url":                "/uploads/catalog/" + strconv.FormatInt(tid, 10) + "/" + name,
		"width":              info.Width,
		"height":             info.Height,
		"format":             info.Format,
		"recommended_width":  1200,
		"recommended_height": 1200,
		"ratio":              "1:1",
	})
}

func (a *App) catalogUploadFileHandler(w http.ResponseWriter, r *http.Request) {
	rel := strings.TrimPrefix(r.URL.Path, "/uploads/catalog/")
	rel = filepath.Clean(rel)
	if rel == "." || strings.Contains(rel, "..") {
		http.NotFound(w, r)
		return
	}
	full := filepath.Join(a.cfg.DataDir, "catalog_uploads", rel)
	root, _ := filepath.Abs(filepath.Join(a.cfg.DataDir, "catalog_uploads"))
	abs, _ := filepath.Abs(full)
	if !strings.HasPrefix(abs, root+string(os.PathSeparator)) {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, abs)
}
