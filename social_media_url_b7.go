package main

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// socialB7CanonicalMediaURL converts a Worktic upload path into a public HTTPS URL.
// Only /uploads/ relative paths are eligible. Remote URLs are not fetched here.
func socialB7CanonicalMediaURL(baseURL, raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", fmt.Errorf("la URL del recurso multimedia esta vacia")
	}
	if strings.ContainsAny(s, "\r\n\t") {
		return "", fmt.Errorf("la URL multimedia contiene caracteres de control")
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", fmt.Errorf("URL multimedia no valida: %w", err)
	}
	if u.Scheme == "" && u.Host == "" {
		// Worktic uploads are mounted at /uploads/ for the public web client.
		// Do not accept relative URLs to arbitrary app endpoints.
		if !strings.HasPrefix(u.Path, "/uploads/") && !strings.HasPrefix(u.Path, "uploads/") {
			return "", fmt.Errorf("ruta multimedia no publica: se requiere https://... o /uploads/...")
		}
		if strings.Contains(u.EscapedPath(), "..") {
			return "", fmt.Errorf("ruta multimedia insegura")
		}
		base, err := url.Parse(strings.TrimSpace(baseURL))
		if err != nil || base.Scheme != "https" || base.Hostname() == "" {
			return "", fmt.Errorf("BaseURL debe ser una URL HTTPS publica para multimedia")
		}
		u.Path = "/" + strings.TrimPrefix(u.Path, "/")
		u = base.ResolveReference(u)
	}
	if u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
		return "", fmt.Errorf("la URL multimedia debe ser HTTPS publica")
	}
	host := strings.ToLower(u.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") {
		return "", fmt.Errorf("la URL multimedia no puede apuntar a un servidor local")
	}
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() || ip.IsUnspecified() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return "", fmt.Errorf("la URL multimedia debe ser accesible en Internet")
		}
	}
	u.Fragment = "" // A URL fragment is never sent to the media server.
	return u.String(), nil
}

func socialB7IsPermanentMediaURLFailure(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "url should represent a valid url") ||
		strings.Contains(msg, "recurso multimedia invalido") ||
		strings.Contains(msg, "media url is not valid")
}
