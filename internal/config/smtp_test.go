package config

import "testing"

func TestValidateSMTPRequiresDeliveryForProductionVerification(t *testing.T) {
	cfg := &Config{App: AppConfig{Env: "production"}, Auth: AuthConfig{RequireEmailVerification: true}}
	if err := cfg.validateSMTP(); err == nil {
		t.Fatal("validateSMTP accepted required email verification without SMTP")
	}
}

func TestValidateSMTPRejectsUnsafeProductionTransport(t *testing.T) {
	cfg := &Config{
		App:  AppConfig{Env: "production"},
		SMTP: SMTPConfig{Host: "smtp.example.com", Port: 587, FromEmail: "mail@example.com", TLSMode: "none"},
	}
	if err := cfg.validateSMTP(); err == nil {
		t.Fatal("validateSMTP accepted plaintext SMTP in production")
	}
}

func TestValidateSMTPAcceptsAuthenticatedSTARTTLS(t *testing.T) {
	cfg := &Config{
		App:  AppConfig{Env: "production"},
		SMTP: SMTPConfig{Host: "smtp.example.com", Port: 587, Username: "mailer", Password: "secret", FromEmail: "mail@example.com", TLSMode: "starttls"},
	}
	if err := cfg.validateSMTP(); err != nil {
		t.Fatalf("validateSMTP rejected valid STARTTLS configuration: %v", err)
	}
}
