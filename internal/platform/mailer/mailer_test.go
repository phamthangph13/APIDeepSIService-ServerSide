package mailer

import (
	"strings"
	"testing"

	"github.com/deepsi/agent-service/internal/config"
)

func TestBuildMessageEncodesHeadersAndUsesValidatedAddresses(t *testing.T) {
	message := buildMessage(config.SMTPConfig{
		FromEmail: "mailer@example.com",
		FromName:  "DeepSI\r\nBcc: attacker@example.com",
	}, Message{
		To:      "user@example.com",
		Subject: "Xác nhận email",
		Text:    "Confirm",
		HTML:    "<p>Confirm</p>",
	})
	text := string(message)
	if strings.Contains(text, "\r\nBcc:") {
		t.Fatalf("message contains injected header: %s", text)
	}
	if !strings.Contains(text, "From: ") || !strings.Contains(text, "To: <user@example.com>") {
		t.Fatalf("message headers were not safely formatted: %s", text)
	}
}

func TestParseAddressRejectsHeaderInjection(t *testing.T) {
	if _, err := parseAddress("user@example.com\r\nBcc: attacker@example.com"); err == nil {
		t.Fatal("parseAddress accepted a header injection")
	}
}
