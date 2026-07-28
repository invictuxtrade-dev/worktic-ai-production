package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type landingStat struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type landingPremiumConfig struct {
	BrandName            string        `json:"brand_name"`
	MediaType            string        `json:"media_type"`
	VideoURL             string        `json:"video_url"`
	VideoPlacement       string        `json:"video_placement"`
	HeroAlt              string        `json:"hero_alt"`
	HeroFit              string        `json:"hero_fit"`
	HeroLayout           string        `json:"hero_layout"`
	ContactTitle         string        `json:"contact_title"`
	ContactSubtitle      string        `json:"contact_subtitle"`
	ContactMessage       string        `json:"contact_message"`
	ChannelKeys          []string      `json:"channel_keys"`
	Stats                []landingStat `json:"stats"`
	TrustText            string        `json:"trust_text"`
	FooterText           string        `json:"footer_text"`
	ShowContactPanel     bool          `json:"show_contact_panel"`
	ShowFloatingChannels bool          `json:"show_floating_channels"`
	ShowStickyCTA        bool          `json:"show_sticky_cta"`
}

type landingContactOption struct {
	Key   string `json:"key"`
	Type  string `json:"type"`
	Name  string `json:"name"`
	URL   string `json:"url"`
	Kind  string `json:"kind"`
	Label string `json:"label"`
}

func defaultLandingPremium() landingPremiumConfig {
	return landingPremiumConfig{
		MediaType:            "image",
		VideoPlacement:       "section",
		HeroFit:              "cover",
		HeroLayout:           "split",
		ContactTitle:         "Hablemos de tu proyecto",
		ContactSubtitle:      "Elige el canal que prefieras y recibe atención directa.",
		TrustText:            "Atención rápida, segura y personalizada",
		ShowContactPanel:     true,
		ShowFloatingChannels: true,
		ShowStickyCTA:        true,
	}
}

func parseLandingPremium(raw string) landingPremiumConfig {
	cfg := defaultLandingPremium()
	if strings.TrimSpace(raw) != "" {
		_ = json.Unmarshal([]byte(raw), &cfg)
	}
	cfg.normalize()
	return cfg
}

func normalizeLandingPremiumJSON(raw string) (string, error) {
	cfg := defaultLandingPremium()
	if strings.TrimSpace(raw) != "" {
		if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
			return "", errors.New("la configuración premium de la landing contiene datos inválidos")
		}
	}
	cfg.normalize()
	b, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (c *landingPremiumConfig) normalize() {
	c.BrandName = strings.TrimSpace(c.BrandName)
	c.VideoURL = strings.TrimSpace(c.VideoURL)
	c.HeroAlt = strings.TrimSpace(c.HeroAlt)
	c.ContactTitle = strings.TrimSpace(c.ContactTitle)
	c.ContactSubtitle = strings.TrimSpace(c.ContactSubtitle)
	c.ContactMessage = strings.TrimSpace(c.ContactMessage)
	c.TrustText = strings.TrimSpace(c.TrustText)
	c.FooterText = strings.TrimSpace(c.FooterText)
	if c.MediaType != "video" {
		c.MediaType = "image"
	}
	if c.VideoPlacement != "hero" {
		c.VideoPlacement = "section"
	}
	if c.HeroFit != "contain" {
		c.HeroFit = "cover"
	}
	if c.HeroLayout != "center" {
		c.HeroLayout = "split"
	}
	if c.ContactTitle == "" {
		c.ContactTitle = "Hablemos de tu proyecto"
	}
	if c.ContactSubtitle == "" {
		c.ContactSubtitle = "Elige el canal que prefieras y recibe atención directa."
	}
	if c.TrustText == "" {
		c.TrustText = "Atención rápida, segura y personalizada"
	}
	cleanKeys := make([]string, 0, len(c.ChannelKeys))
	seen := map[string]bool{}
	for _, k := range c.ChannelKeys {
		k = strings.TrimSpace(k)
		if k != "" && !seen[k] {
			seen[k] = true
			cleanKeys = append(cleanKeys, k)
		}
	}
	c.ChannelKeys = cleanKeys
	cleanStats := make([]landingStat, 0, len(c.Stats))
	for _, s := range c.Stats {
		s.Value = strings.TrimSpace(s.Value)
		s.Label = strings.TrimSpace(s.Label)
		if s.Value != "" || s.Label != "" {
			cleanStats = append(cleanStats, s)
		}
		if len(cleanStats) == 4 {
			break
		}
	}
	c.Stats = cleanStats
}

func landingVideoEmbed(raw string) template.URL {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return ""
	}
	host := strings.ToLower(strings.TrimPrefix(u.Host, "www."))
	var id string
	switch host {
	case "youtube.com", "m.youtube.com", "youtube-nocookie.com":
		if strings.HasPrefix(u.Path, "/watch") {
			id = u.Query().Get("v")
		} else if strings.HasPrefix(u.Path, "/embed/") || strings.HasPrefix(u.Path, "/shorts/") {
			id = strings.Trim(strings.SplitN(strings.TrimPrefix(strings.TrimPrefix(u.Path, "/embed/"), "/shorts/"), "/", 2)[0], " ")
		}
		if id != "" {
			return template.URL("https://www.youtube-nocookie.com/embed/" + url.PathEscape(id) + "?rel=0&modestbranding=1")
		}
	case "youtu.be":
		id = strings.Trim(strings.SplitN(strings.TrimPrefix(u.Path, "/"), "/", 2)[0], " ")
		if id != "" {
			return template.URL("https://www.youtube-nocookie.com/embed/" + url.PathEscape(id) + "?rel=0&modestbranding=1")
		}
	case "vimeo.com", "player.vimeo.com":
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		for i := len(parts) - 1; i >= 0; i-- {
			if parts[i] != "" && parts[i] != "video" {
				id = parts[i]
				break
			}
		}
		if id != "" {
			return template.URL("https://player.vimeo.com/video/" + url.PathEscape(id) + "?title=0&byline=0&portrait=0")
		}
	}
	return ""
}

func normalizePublicURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "#") || strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "mailto:") || strings.HasPrefix(raw, "tel:") {
		return raw
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ""
	}
	return u.String()
}

func digitsOnly(v string) string {
	if i := strings.Index(v, ":"); i >= 0 {
		v = v[:i]
	}
	if i := strings.Index(v, "@"); i >= 0 {
		v = v[:i]
	}
	var b strings.Builder
	for _, r := range v {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func channelPublicURL(kind, externalID, accountName, message string) string {
	ext := strings.TrimSpace(externalID)
	name := strings.TrimSpace(strings.TrimPrefix(accountName, "@"))
	switch kind {
	case "whatsapp", "whatsapp_qr":
		phone := digitsOnly(ext)
		if phone == "" {
			return ""
		}
		link := "https://wa.me/" + phone
		if strings.TrimSpace(message) != "" {
			link += "?text=" + url.QueryEscape(message)
		}
		return link
	case "telegram":
		username := strings.TrimPrefix(ext, "@")
		if username == "" {
			username = name
		}
		if username == "" || strings.Contains(username, " ") {
			return ""
		}
		return "https://t.me/" + url.PathEscape(username)
	case "messenger":
		if ext == "" {
			return ""
		}
		return "https://m.me/" + url.PathEscape(ext)
	case "instagram":
		username := name
		if username == "" {
			username = strings.TrimPrefix(ext, "@")
		}
		if username == "" {
			return ""
		}
		return "https://instagram.com/" + url.PathEscape(username)
	case "facebook":
		id := ext
		if id == "" {
			id = name
		}
		if id == "" {
			return ""
		}
		return "https://facebook.com/" + url.PathEscape(id)
	case "linkedin":
		if strings.HasPrefix(ext, "http://") || strings.HasPrefix(ext, "https://") {
			return normalizePublicURL(ext)
		}
		if name != "" {
			return "https://linkedin.com/in/" + url.PathEscape(name)
		}
	case "tiktok":
		if name != "" {
			return "https://tiktok.com/@" + url.PathEscape(name)
		}
	case "youtube":
		if strings.HasPrefix(ext, "http://") || strings.HasPrefix(ext, "https://") {
			return normalizePublicURL(ext)
		}
		if ext != "" {
			return "https://youtube.com/channel/" + url.PathEscape(ext)
		}
	}
	return ""
}

func contactLabel(kind string) string {
	switch kind {
	case "whatsapp", "whatsapp_qr":
		return "WhatsApp"
	case "telegram":
		return "Telegram"
	case "messenger":
		return "Messenger"
	case "instagram":
		return "Instagram"
	case "facebook":
		return "Facebook"
	case "linkedin":
		return "LinkedIn"
	case "tiktok":
		return "TikTok"
	case "youtube":
		return "YouTube"
	}
	return strings.Title(kind) //nolint:staticcheck
}

func landingChannelIcon(kind string) template.HTML {
	kind = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(kind), "_qr"))
	icons := map[string]string{
		"whatsapp":  `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 2a9.5 9.5 0 0 0-8.3 14.1L2.4 21.6l5.7-1.3A9.5 9.5 0 1 0 12 2Zm0 17.2a7.7 7.7 0 0 1-3.9-1.1l-.4-.2-3.3.8.8-3.2-.2-.4A7.7 7.7 0 1 1 12 19.2Zm4.2-5.8c-.2-.1-1.3-.7-1.6-.7-.2-.1-.4-.1-.6.2l-.7.9c-.1.2-.3.2-.5.1-1.4-.7-2.4-1.3-3.3-2.9-.2-.3.2-.4.6-.9.1-.2.1-.3.2-.5 0-.2 0-.3-.1-.5l-.7-1.7c-.2-.4-.4-.4-.6-.4h-.5c-.2 0-.5.1-.7.3-.2.3-1 1-1 2.4s1 2.8 1.2 3c.1.2 2 3.1 4.9 4.3.7.3 1.2.5 1.7.6.7.2 1.3.2 1.8.1.5-.1 1.6-.7 1.9-1.3.2-.6.2-1.1.2-1.3-.1-.1-.2-.2-.4-.3Z"/></svg>`,
		"telegram":  `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M21.6 3.3 18.4 20c-.2 1.2-.9 1.5-1.9.9l-4.9-3.6-2.4 2.3c-.3.3-.5.5-1 .5l.4-5 9.1-8.2c.4-.4-.1-.6-.6-.2L5.9 13.8 1 12.3c-1.1-.3-1.1-1.1.2-1.6L20.3 3c.9-.3 1.6.2 1.3.3Z"/></svg>`,
		"messenger": `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 2C6.4 2 2 6.1 2 11.4c0 3 1.4 5.6 3.7 7.3V22l3.4-1.9c.9.3 1.9.5 2.9.5 5.6 0 10-4.1 10-9.4S17.6 2 12 2Zm1 12.6-2.5-2.7-4.8 2.7 5.3-5.6 2.5 2.7 4.8-2.7-5.3 5.6Z"/></svg>`,
		"instagram": `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M7.2 2h9.6A5.2 5.2 0 0 1 22 7.2v9.6a5.2 5.2 0 0 1-5.2 5.2H7.2A5.2 5.2 0 0 1 2 16.8V7.2A5.2 5.2 0 0 1 7.2 2Zm-.2 2A3 3 0 0 0 4 7v10a3 3 0 0 0 3 3h10a3 3 0 0 0 3-3V7a3 3 0 0 0-3-3H7Zm10.3 1.5a1.2 1.2 0 1 1 0 2.4 1.2 1.2 0 0 1 0-2.4ZM12 7a5 5 0 1 1 0 10 5 5 0 0 1 0-10Zm0 2a3 3 0 1 0 0 6 3 3 0 0 0 0-6Z"/></svg>`,
		"facebook":  `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M13.8 22v-8h2.7l.4-3.1h-3.1v-2c0-.9.3-1.5 1.6-1.5H17V4.6c-.3 0-1.3-.1-2.4-.1-2.4 0-4 1.4-4 4.1v2.3H8V14h2.6v8h3.2Z"/></svg>`,
		"linkedin":  `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M5.3 7.8A2.3 2.3 0 1 1 5.3 3a2.3 2.3 0 0 1 0 4.8ZM3.3 21V9.2h4V21h-4Zm6.4 0V9.2h3.8v1.6h.1c.5-1 1.8-2.1 3.7-2.1 4 0 4.7 2.6 4.7 6V21h-4v-5.6c0-1.3 0-3.1-1.9-3.1s-2.2 1.5-2.2 3V21h-4.2Z"/></svg>`,
		"tiktok":    `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M15.8 2c.3 2 1.5 3.4 3.5 3.8v3.4a9 9 0 0 1-3.5-.9v6.9a6.2 6.2 0 1 1-5.4-6.1v3.5a2.8 2.8 0 1 0 2 2.7V2h3.4Z"/></svg>`,
		"youtube":   `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M21.6 7.1a2.8 2.8 0 0 0-2-2C17.9 4.6 12 4.6 12 4.6s-5.9 0-7.6.5a2.8 2.8 0 0 0-2 2A29 29 0 0 0 2 12a29 29 0 0 0 .4 4.9 2.8 2.8 0 0 0 2 2c1.7.5 7.6.5 7.6.5s5.9 0 7.6-.5a2.8 2.8 0 0 0 2-2A29 29 0 0 0 22 12a29 29 0 0 0-.4-4.9ZM10 15.3V8.7l5.7 3.3-5.7 3.3Z"/></svg>`,
		"link":      `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M10.6 13.4a1.5 1.5 0 0 0 2.1 0l3.5-3.5a3 3 0 0 0-4.2-4.2l-2 2-1.4-1.4 2-2a5 5 0 0 1 7.1 7.1l-3.5 3.5a3.5 3.5 0 0 1-5 0l1.4-1.5Zm2.8-2.8a1.5 1.5 0 0 0-2.1 0l-3.5 3.5a3 3 0 1 0 4.2 4.2l2-2 1.4 1.4-2 2a5 5 0 0 1-7.1-7.1l3.5-3.5a3.5 3.5 0 0 1 5 0l-1.4 1.5Z"/></svg>`,
	}
	if icon, ok := icons[kind]; ok {
		return template.HTML(icon) // #nosec G203 -- fixed internal SVG catalog, no user input.
	}
	return template.HTML(icons["link"]) // #nosec G203 -- fixed internal SVG catalog.
}

func (a *App) landingContactOptions(tenant int64, message string) []landingContactOption {
	out := []landingContactOption{}
	seen := map[string]bool{}
	rows, err := a.db.Query(`SELECT id,type,name,external_account_id FROM channel_connections WHERE tenant_id=? AND status='connected' ORDER BY type,name`, tenant)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id int64
			var typ, name, ext string
			_ = rows.Scan(&id, &typ, &name, &ext)
			publicType := strings.TrimSuffix(typ, "_qr")
			link := channelPublicURL(typ, ext, name, message)
			if link == "" || seen[link] {
				continue
			}
			seen[link] = true
			out = append(out, landingContactOption{Key: fmt.Sprintf("channel:%d", id), Type: publicType, Name: firstNonEmpty(name, contactLabel(typ)), URL: link, Kind: "channel", Label: contactLabel(typ)})
		}
	}
	rows2, err := a.db.Query(`SELECT id,platform,account_name,external_account_id FROM social_connections WHERE tenant_id=? AND status='connected' ORDER BY platform,account_name`, tenant)
	if err == nil {
		defer rows2.Close()
		for rows2.Next() {
			var id int64
			var platform, name, ext string
			_ = rows2.Scan(&id, &platform, &name, &ext)
			link := channelPublicURL(platform, ext, name, message)
			if link == "" || seen[link] {
				continue
			}
			seen[link] = true
			out = append(out, landingContactOption{Key: fmt.Sprintf("social:%d", id), Type: platform, Name: firstNonEmpty(name, contactLabel(platform)), URL: link, Kind: "social", Label: contactLabel(platform)})
		}
	}
	return out
}

func filterLandingContactOptions(all []landingContactOption, selected []string) []landingContactOption {
	if len(selected) == 0 {
		out := []landingContactOption{}
		for _, item := range all {
			if item.Type == "whatsapp" || item.Type == "telegram" || item.Type == "messenger" || item.Type == "instagram" {
				out = append(out, item)
			}
		}
		return out
	}
	wanted := map[string]bool{}
	for _, key := range selected {
		wanted[key] = true
	}
	out := []landingContactOption{}
	for _, item := range all {
		if wanted[item.Key] {
			out = append(out, item)
		}
	}
	return out
}

func (a *App) landingChannelsHandler(w http.ResponseWriter, r *http.Request) {
	tenant, _, err := a.tenantFor(r)
	if err != nil {
		writeError(w, err, http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, errors.New("método no permitido"), http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, map[string]any{"channels": a.landingContactOptions(tenant, "")})
}

func (a *App) landingImageUploadHandler(w http.ResponseWriter, r *http.Request) {
	tenant, user, err := a.tenantForRequest(r)
	if err != nil {
		writeError(w, err, http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, errors.New("método no permitido"), http.StatusMethodNotAllowed)
		return
	}
	if user.Role != "owner" && user.Role != "admin" && user.Role != "superadmin" && user.Role != "supervisor" {
		writeError(w, errors.New("sin permiso para subir imágenes"), http.StatusForbidden)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 9<<20)
	if err := r.ParseMultipartForm(9 << 20); err != nil {
		writeError(w, errors.New("imagen demasiado grande; máximo 8 MB"), http.StatusBadRequest)
		return
	}
	file, header, err := r.FormFile("image")
	if err != nil {
		writeError(w, errors.New("selecciona una imagen"), http.StatusBadRequest)
		return
	}
	defer file.Close()
	if header.Size > 8<<20 {
		writeError(w, errors.New("imagen demasiado grande; máximo 8 MB"), http.StatusBadRequest)
		return
	}
	ext, err := validCatalogImage(header)
	if err != nil {
		writeError(w, err, http.StatusBadRequest)
		return
	}
	data, err := io.ReadAll(io.LimitReader(file, (8<<20)+1))
	if err != nil {
		writeError(w, errors.New("no fue posible leer la imagen"), http.StatusBadRequest)
		return
	}
	if len(data) > 8<<20 {
		writeError(w, errors.New("imagen demasiado grande; máximo 8 MB"), http.StatusBadRequest)
		return
	}
	info, err := inspectUploadedImage(data, ext)
	if err != nil {
		writeError(w, err, http.StatusBadRequest)
		return
	}
	if err := validateLandingHeroStandard(info); err != nil {
		writeError(w, err, http.StatusBadRequest)
		return
	}
	dir := filepath.Join(a.cfg.DataDir, "landing_uploads", strconv.FormatInt(tenant, 10))
	if err := os.MkdirAll(dir, 0755); err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	name := fmt.Sprintf("hero_%d%s", time.Now().UnixNano(), ext)
	target := filepath.Join(dir, name)
	if err := os.WriteFile(target, data, 0644); err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{
		"ok":                 true,
		"url":                "/uploads/landings/" + strconv.FormatInt(tenant, 10) + "/" + name,
		"width":              info.Width,
		"height":             info.Height,
		"format":             info.Format,
		"recommended_width":  1600,
		"recommended_height": 1200,
		"ratio":              "4:3",
	})
}

func (a *App) landingUploadFileHandler(w http.ResponseWriter, r *http.Request) {
	rel := filepath.Clean(strings.TrimPrefix(r.URL.Path, "/uploads/landings/"))
	if rel == "." || strings.Contains(rel, "..") {
		http.NotFound(w, r)
		return
	}
	root, _ := filepath.Abs(filepath.Join(a.cfg.DataDir, "landing_uploads"))
	full, _ := filepath.Abs(filepath.Join(root, rel))
	if !strings.HasPrefix(full, root+string(os.PathSeparator)) {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, full)
}
