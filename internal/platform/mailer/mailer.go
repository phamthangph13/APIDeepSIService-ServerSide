package mailer

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"log/slog"
	"mime"
	"net"
	"net/smtp"
	"strings"
	"time"

	"github.com/deepsi/agent-service/internal/config"
)

type Message struct {
	To      string
	Subject string
	Text    string
	HTML    string
}

type Mailer interface {
	Send(ctx context.Context, message Message) error
	Configured() bool
}

type SMTPMailer struct {
	cfg config.SMTPConfig
	log *slog.Logger
}

func NewSMTPMailer(cfg config.SMTPConfig, log *slog.Logger) Mailer {
	if strings.TrimSpace(cfg.Host) == "" {
		return &LogMailer{log: log}
	}
	return &SMTPMailer{cfg: cfg, log: log}
}

func (m *SMTPMailer) Configured() bool { return true }

func (m *SMTPMailer) Send(ctx context.Context, message Message) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	from := m.cfg.FromEmail
	if from == "" {
		from = m.cfg.Username
	}
	addr := net.JoinHostPort(m.cfg.Host, fmt.Sprintf("%d", m.cfg.Port))

	dialer := &net.Dialer{Timeout: 10 * time.Second}
	var conn net.Conn
	var err error
	if strings.EqualFold(m.cfg.TLSMode, "tls") {
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{ServerName: m.cfg.Host, MinVersion: tls.VersionTLS12})
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("smtp dial: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, m.cfg.Host)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	defer client.Close()

	if strings.EqualFold(m.cfg.TLSMode, "starttls") {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{ServerName: m.cfg.Host, MinVersion: tls.VersionTLS12}); err != nil {
				return fmt.Errorf("smtp starttls: %w", err)
			}
		}
	}

	if m.cfg.Username != "" {
		auth := smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, m.cfg.Host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	if err := client.Rcpt(message.To); err != nil {
		return fmt.Errorf("smtp rcpt: %w", err)
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := writer.Write(buildMessage(m.cfg, message)); err != nil {
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("smtp close data: %w", err)
	}
	return client.Quit()
}

func buildMessage(cfg config.SMTPConfig, message Message) []byte {
	boundary := "deepsi-" + fmt.Sprintf("%d", time.Now().UnixNano())
	fromName := mime.QEncoding.Encode("utf-8", cfg.FromName)
	subject := mime.QEncoding.Encode("utf-8", message.Subject)

	var b bytes.Buffer
	fmt.Fprintf(&b, "From: %s <%s>\r\n", fromName, cfg.FromEmail)
	fmt.Fprintf(&b, "To: <%s>\r\n", message.To)
	fmt.Fprintf(&b, "Subject: %s\r\n", subject)
	fmt.Fprintf(&b, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&b, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	fmt.Fprintf(&b, "Content-Type: multipart/alternative; boundary=\"%s\"\r\n", boundary)
	fmt.Fprintf(&b, "\r\n--%s\r\n", boundary)
	fmt.Fprintf(&b, "Content-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: base64\r\n\r\n")
	b.WriteString(wrapBase64(message.Text))
	fmt.Fprintf(&b, "\r\n--%s\r\n", boundary)
	fmt.Fprintf(&b, "Content-Type: text/html; charset=UTF-8\r\nContent-Transfer-Encoding: base64\r\n\r\n")
	b.WriteString(wrapBase64(message.HTML))
	fmt.Fprintf(&b, "\r\n--%s--\r\n", boundary)
	return b.Bytes()
}

func wrapBase64(value string) string {
	encoded := base64.StdEncoding.EncodeToString([]byte(value))
	var b strings.Builder
	for len(encoded) > 76 {
		b.WriteString(encoded[:76])
		b.WriteString("\r\n")
		encoded = encoded[76:]
	}
	b.WriteString(encoded)
	return b.String()
}

// LogMailer is used when SMTP is not configured. It never fails a user flow;
// it logs the message so local development can still copy reset links.
type LogMailer struct {
	log *slog.Logger
}

func (m *LogMailer) Configured() bool { return false }

func (m *LogMailer) Send(ctx context.Context, message Message) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.log.Warn("SMTP is not configured; email was not sent",
		"to", message.To,
		"subject", message.Subject,
		"text", message.Text,
	)
	return nil
}
