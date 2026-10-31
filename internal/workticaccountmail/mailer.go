package workticaccountmail

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"time"
)

type MailConfig struct {
	Provider, From, APIKey                     string
	SMTPHost, SMTPPort, SMTPUser, SMTPPassword string
}

func MailConfigFromEnv() MailConfig {
	return MailConfig{
		Provider: strings.ToLower(os.Getenv("WORKTIC_MAIL_PROVIDER")), From: os.Getenv("WORKTIC_MAIL_FROM"), APIKey: os.Getenv("WORKTIC_RESEND_API_KEY"),
		SMTPHost: os.Getenv("WORKTIC_SMTP_HOST"), SMTPPort: os.Getenv("WORKTIC_SMTP_PORT"), SMTPUser: os.Getenv("WORKTIC_SMTP_USER"), SMTPPassword: os.Getenv("WORKTIC_SMTP_PASSWORD"),
	}
}
func (c MailConfig) Validate() error {
	from, err := mail.ParseAddress(c.From)
	if err != nil || from.Address == "" || strings.ContainsAny(c.From, "\r\n") {
		return errors.New("WORKTIC_MAIL_FROM must contain a valid verified sender")
	}
	switch c.Provider {
	case "resend":
		if c.APIKey == "" {
			return errors.New("WORKTIC_RESEND_API_KEY is required")
		}
	case "smtp":
		if c.SMTPHost == "" || c.SMTPUser == "" || c.SMTPPassword == "" {
			return errors.New("SMTP host/user/password required")
		}
		port, err := strconv.Atoi(c.SMTPPort)
		if err != nil || port < 1 || port > 65535 {
			return errors.New("invalid SMTP port")
		}
	default:
		return errors.New("WORKTIC_MAIL_PROVIDER must be resend or smtp")
	}
	return nil
}
func (c MailConfig) Sender() (Sender, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &configuredMailer{c: c, client: &http.Client{Timeout: 12 * time.Second}}, nil
}

type configuredMailer struct {
	c      MailConfig
	client *http.Client
}

func (m *configuredMailer) Send(ctx context.Context, to, subject, html, plain string) error {
	dest, err := mail.ParseAddress(to)
	if err != nil || dest.Address != to {
		return errors.New("invalid recipient")
	}
	if strings.ContainsAny(subject, "\r\n") {
		return errors.New("invalid subject")
	}
	if m.c.Provider == "resend" {
		payload, err := json.Marshal(map[string]any{"from": m.c.From, "to": []string{to}, "subject": subject, "html": html, "text": plain})
		if err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+m.c.APIKey)
		req.Header.Set("Content-Type", "application/json")
		res, err := m.client.Do(req)
		if err != nil {
			return err
		}
		defer res.Body.Close()
		io.Copy(io.Discard, io.LimitReader(res.Body, 2048))
		if res.StatusCode < 200 || res.StatusCode > 299 {
			return fmt.Errorf("email provider returned %d", res.StatusCode)
		}
		return nil
	}
	return m.smtpSend(ctx, to, subject, html, plain)
}
func (m *configuredMailer) smtpSend(ctx context.Context, to, subject, html, plain string) error {
	c := m.c
	from, err := mail.ParseAddress(c.From)
	if err != nil {
		return err
	}
	hostport := net.JoinHostPort(c.SMTPHost, c.SMTPPort)
	var conn net.Conn
	d := net.Dialer{Timeout: 10 * time.Second}
	if c.SMTPPort == "465" {
		conn, err = tls.DialWithDialer(&d, "tcp", hostport, &tls.Config{ServerName: c.SMTPHost, MinVersion: tls.VersionTLS12})
	} else {
		conn, err = d.DialContext(ctx, "tcp", hostport)
	}
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(20 * time.Second))
	client, err := smtp.NewClient(conn, c.SMTPHost)
	if err != nil {
		return err
	}
	defer client.Close()
	if c.SMTPPort != "465" {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return errors.New("smtp server does not support STARTTLS")
		}
		if err = client.StartTLS(&tls.Config{ServerName: c.SMTPHost, MinVersion: tls.VersionTLS12}); err != nil {
			return err
		}
	}
	if err = client.Auth(smtp.PlainAuth("", c.SMTPUser, c.SMTPPassword, c.SMTPHost)); err != nil {
		return err
	}
	if err = client.Mail(from.Address); err != nil {
		return err
	}
	if err = client.Rcpt(to); err != nil {
		return err
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	boundary := "WorkticAI-Mixed-Email"
	raw := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: multipart/alternative; boundary=%q\r\n\r\n--%s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s\r\n--%s\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s\r\n--%s--\r\n", c.From, to, mime.QEncoding.Encode("utf-8", subject), boundary, boundary, plain, boundary, html, boundary)
	_, err = io.WriteString(writer, raw)
	if cerr := writer.Close(); err == nil {
		err = cerr
	}
	return err
}
