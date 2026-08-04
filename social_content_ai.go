package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type socialGeneratedContent struct {
	Name    string   `json:"name"`
	Content string   `json:"content"`
	CTA     string   `json:"cta"`
	Tags    []string `json:"tags"`
}

func (a *App) socialGenerateContentHandler(w http.ResponseWriter, r *http.Request) {
	_, u, err := a.tenantFor(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}
	if !socialPermissionsFor(u).CreateDraft {
		http.Error(w, "No tienes permiso para crear contenido", http.StatusForbidden)
		return
	}
	if a.openAIKey() == "" {
		http.Error(w, "OpenAI no está configurado. Agrega OPENAI_API_KEY en la configuración del servidor.", http.StatusServiceUnavailable)
		return
	}

	var q struct {
		Prompt, Objective, Format, Tone, Language, Audience, Product, LinkURL string
		Platforms                                                             []string
	}
	if json.NewDecoder(r.Body).Decode(&q) != nil {
		http.Error(w, "datos inválidos", 400)
		return
	}
	q.Prompt = strings.TrimSpace(q.Prompt)
	if q.Prompt == "" {
		http.Error(w, "Escribe las instrucciones para la IA", 400)
		return
	}
	if q.Tone == "" {
		q.Tone = "profesional, persuasivo, cercano y confiable"
	}
	if q.Language == "" {
		q.Language = "español"
	}
	platforms := make([]string, 0, len(q.Platforms))
	for _, p := range q.Platforms {
		p = strings.ToLower(strings.TrimSpace(p))
		if validSocialPlatform(p) {
			platforms = append(platforms, p)
		}
	}

	system := `Eres un estratega senior de contenido para redes sociales. Crea textos profesionales, claros, persuasivos y naturales. No inventes testimonios, cifras, urgencia, premios ni resultados. Evita afirmaciones engañosas. Devuelve exclusivamente JSON válido, sin markdown, con esta estructura exacta: {"name":"nombre interno breve","content":"texto principal listo para publicar","cta":"llamado a la acción breve","tags":["#Etiqueta1","#Etiqueta2"]}. El contenido debe tener un gancho fuerte, beneficio central, explicación concreta y cierre. Los tags deben ser relevantes, buscables, sin espacios y entre 5 y 10.`
	user := fmt.Sprintf("Instrucciones del usuario: %s\nObjetivo: %s\nProducto o tema: %s\nPúblico: %s\nFormato: %s\nRedes: %s\nTono: %s\nIdioma: %s\nEnlace: %s", q.Prompt, q.Objective, q.Product, q.Audience, q.Format, strings.Join(platforms, ", "), q.Tone, q.Language, q.LinkURL)
	raw, err := a.callOpenAI(system, user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	raw = strings.TrimSpace(raw)
	if i := strings.Index(raw, "{"); i >= 0 {
		raw = raw[i:]
	}
	if i := strings.LastIndex(raw, "}"); i >= 0 {
		raw = raw[:i+1]
	}
	var out socialGeneratedContent
	if json.Unmarshal([]byte(raw), &out) != nil || strings.TrimSpace(out.Content) == "" {
		out.Content = strings.TrimSpace(raw)
		out.Name = "Contenido generado con IA"
	}
	cleanTags := make([]string, 0, len(out.Tags))
	for _, tag := range out.Tags {
		tag = strings.TrimSpace(strings.ReplaceAll(tag, " ", ""))
		if tag == "" {
			continue
		}
		if !strings.HasPrefix(tag, "#") {
			tag = "#" + tag
		}
		cleanTags = append(cleanTags, tag)
		if len(cleanTags) == 10 {
			break
		}
	}
	out.Tags = cleanTags
	writeJSON(w, map[string]any{"generated": true, "name": strings.TrimSpace(out.Name), "content": strings.TrimSpace(out.Content), "cta": strings.TrimSpace(out.CTA), "tags": out.Tags})
}

func socialMediaExtension(filename, contentType string) (string, string, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	allowed := map[string]string{".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png", ".webp": "image/webp", ".gif": "image/gif", ".mp4": "video/mp4", ".mov": "video/quicktime", ".webm": "video/webm"}
	mime, ok := allowed[ext]
	if !ok {
		return "", "", errors.New("formato no permitido; usa JPG, PNG, WEBP, GIF, MP4, MOV o WEBM")
	}
	if contentType != "" && !strings.HasPrefix(contentType, "image/") && !strings.HasPrefix(contentType, "video/") && contentType != "application/octet-stream" {
		return "", "", errors.New("el archivo no es una imagen o video válido")
	}
	return ext, mime, nil
}

func (a *App) socialMediaUploadHandler(w http.ResponseWriter, r *http.Request) {
	tid, u, err := a.tenantFor(r)
	if err != nil {
		http.Error(w, err.Error(), 401)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", 405)
		return
	}
	if !socialPermissionsFor(u).CreateDraft {
		http.Error(w, "No tienes permiso para cargar contenido", 403)
		return
	}
	const maxSize = 50 << 20
	r.Body = http.MaxBytesReader(w, r.Body, maxSize+1024)
	if err := r.ParseMultipartForm(maxSize); err != nil {
		http.Error(w, "archivo demasiado grande; máximo 50 MB", 400)
		return
	}
	file, header, err := r.FormFile("media")
	if err != nil {
		http.Error(w, "selecciona una imagen o video", 400)
		return
	}
	defer file.Close()
	if header.Size > maxSize {
		http.Error(w, "archivo demasiado grande; máximo 50 MB", 400)
		return
	}
	ext, mime, err := socialMediaExtension(header.Filename, header.Header.Get("Content-Type"))
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	data, err := io.ReadAll(io.LimitReader(file, maxSize+1))
	if err != nil || len(data) > maxSize {
		http.Error(w, "no fue posible leer el archivo o supera 50 MB", 400)
		return
	}
	dir := filepath.Join(a.cfg.DataDir, "social_uploads", strconv.FormatInt(tid, 10))
	if err := os.MkdirAll(dir, 0755); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	name := fmt.Sprintf("%d_%d%s", time.Now().UnixNano(), tid, ext)
	if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, map[string]any{"url": "/uploads/social/" + strconv.FormatInt(tid, 10) + "/" + name, "type": strings.Split(mime, "/")[0], "mime": mime, "size": len(data), "name": header.Filename})
}

func (a *App) socialMediaFileHandler(w http.ResponseWriter, r *http.Request) {
	rel := filepath.Clean(strings.TrimPrefix(r.URL.Path, "/uploads/social/"))
	if rel == "." || strings.Contains(rel, "..") {
		http.NotFound(w, r)
		return
	}
	root, _ := filepath.Abs(filepath.Join(a.cfg.DataDir, "social_uploads"))
	target, _ := filepath.Abs(filepath.Join(root, rel))
	if !strings.HasPrefix(target, root+string(os.PathSeparator)) {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, target)
}
