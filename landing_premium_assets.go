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
	dir := filepath.Join(a.cfg.DataDir, "landing_uploads", strconv.FormatInt(tenant, 10))
	if err := os.MkdirAll(dir, 0755); err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	name := fmt.Sprintf("hero_%d%s", time.Now().UnixNano(), ext)
	target := filepath.Join(dir, name)
	dst, err := os.Create(target)
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	defer dst.Close()
	if _, err := io.Copy(dst, file); err != nil {
		_ = os.Remove(target)
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "url": "/uploads/landings/" + strconv.FormatInt(tenant, 10) + "/" + name})
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
