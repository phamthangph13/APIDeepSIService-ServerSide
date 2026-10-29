package config

import (
	"fmt"
	"log/slog"
	"net/mail"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/deepsi/agent-service/internal/domain"
	"github.com/joho/godotenv"
)

type Config struct {
	App   AppConfig
	HTTP  HTTPConfig
	Mongo MongoConfig
	Auth  AuthConfig
	SMTP  SMTPConfig
	OAuth OAuthConfig
	SePay SePayConfig
	LLM   LLMConfig
	CORS  CORSConfig
	Seed  SeedConfig
	Plans *domain.PlanCatalog
}

type AppConfig struct {
	Env          string
	Name         string
	Version      string
	FrontendURL  string
	PublicAPIURL string
	LogLevel     slog.Level
	Timezone     *time.Location
}

type HTTPConfig struct {
	Addr            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
	MaxBodyBytes    int64
}

type MongoConfig struct {
	URI            string
	Database       string
	ConnectTimeout time.Duration
}

type AuthConfig struct {
	JWTSecret                string
	Issuer                   string
	AccessTTL                time.Duration
	RefreshTTL               time.Duration
	ResetPasswordTTL         time.Duration
	VerifyEmailTTL           time.Duration
	OAuthCodeTTL             time.Duration
	BcryptCost               int
	RequireEmailVerification bool
	ExposeDebugTokens        bool
	CookieDomain             string
	CookieSecure             bool
	CookieSameSite           string
	RateLimitPerMinute       int
	APIKeyPepper             string
}

type SMTPConfig struct {
	Host      string
	Port      int
	Username  string
	Password  string
	FromEmail string
	FromName  string
	TLSMode   string // starttls | tls | none
}

type OAuthProviderConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	AuthURL      string
	TokenURL     string
	UserInfoURL  string
	EmailURL     string
	Scopes       []string
}

type OAuthConfig struct {
	Google OAuthProviderConfig
	GitHub OAuthProviderConfig
}

type SePayConfig struct {
	WebhookAuthMode string // api_key | hmac | none
	WebhookAPIKey   string
	WebhookSecret   string
	MerchantID      string
	CheckoutSecret  string
	CheckoutURL     string
	PaymentMethod   string
	SuccessURL      string
	ErrorURL        string
	CancelURL       string
	BankCode        string
	BankName        string
	BankAccount     string
	BankHolder      string
	QRBaseURL       string
	MockEnabled     bool
	OrderTTL        time.Duration
}

type LLMConfig struct {
	Enabled bool
	BaseURL string
	APIKey  string
	Timeout time.Duration
	Models  []string
}

type CORSConfig struct {
	AllowedOrigins []string
}

type SeedConfig struct {
	AdminEmail    string
	AdminPassword string
	AdminName     string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	loc, err := time.LoadLocation(env("APP_TIMEZONE", "Asia/Ho_Chi_Minh"))
	if err != nil {
		return nil, fmt.Errorf("invalid APP_TIMEZONE: %w", err)
	}

	cfg := &Config{
		App: AppConfig{
			Env:          env("APP_ENV", "development"),
			Name:         env("APP_NAME", "DeepSI Agent"),
			Version:      env("APP_VERSION", "1.0.0"),
			FrontendURL:  strings.TrimRight(env("FRONTEND_URL", "http://localhost:5173"), "/"),
			PublicAPIURL: strings.TrimRight(env("PUBLIC_API_URL", "http://localhost:8080"), "/"),
			LogLevel:     parseLogLevel(env("LOG_LEVEL", "info")),
			Timezone:     loc,
		},
		HTTP: HTTPConfig{
			Addr:            ":" + env("PORT", "8080"),
			ReadTimeout:     envDuration("HTTP_READ_TIMEOUT", 15*time.Second),
			WriteTimeout:    envDuration("HTTP_WRITE_TIMEOUT", 120*time.Second),
			IdleTimeout:     envDuration("HTTP_IDLE_TIMEOUT", 60*time.Second),
			ShutdownTimeout: envDuration("HTTP_SHUTDOWN_TIMEOUT", 15*time.Second),
			MaxBodyBytes:    envInt64("HTTP_MAX_BODY_BYTES", 10<<20),
		},
		Mongo: MongoConfig{
			URI:            env("MONGO_URI", "mongodb://localhost:27017"),
			Database:       env("MONGO_DATABASE", "deepsi_agent"),
			ConnectTimeout: envDuration("MONGO_CONNECT_TIMEOUT", 10*time.Second),
		},
		Auth: AuthConfig{
			JWTSecret:                env("JWT_SECRET", ""),
			Issuer:                   env("JWT_ISSUER", "deepsi-agent"),
			AccessTTL:                envDuration("ACCESS_TOKEN_TTL", 15*time.Minute),
			RefreshTTL:               envDuration("REFRESH_TOKEN_TTL", 30*24*time.Hour),
			ResetPasswordTTL:         envDuration("RESET_PASSWORD_TTL", 30*time.Minute),
			VerifyEmailTTL:           envDuration("VERIFY_EMAIL_TTL", 24*time.Hour),
			OAuthCodeTTL:             envDuration("OAUTH_CODE_TTL", 5*time.Minute),
			BcryptCost:               envInt("BCRYPT_COST", 12),
			RequireEmailVerification: envBool("REQUIRE_EMAIL_VERIFICATION", false),
			ExposeDebugTokens:        envBool("AUTH_EXPOSE_DEBUG_TOKENS", false),
			CookieDomain:             env("COOKIE_DOMAIN", ""),
			CookieSecure:             envBool("COOKIE_SECURE", false),
			CookieSameSite:           env("COOKIE_SAME_SITE", "lax"),
			RateLimitPerMinute:       envInt("AUTH_RATE_LIMIT_PER_MINUTE", 30),
			APIKeyPepper:             env("API_KEY_PEPPER", ""),
		},
		SMTP: SMTPConfig{
			Host:      env("SMTP_HOST", ""),
			Port:      envInt("SMTP_PORT", 587),
			Username:  env("SMTP_USERNAME", ""),
			Password:  env("SMTP_PASSWORD", ""),
			FromEmail: env("SMTP_FROM_EMAIL", "no-reply@deepsi.space"),
			FromName:  env("SMTP_FROM_NAME", "DeepSI Agent"),
			TLSMode:   strings.ToLower(env("SMTP_TLS_MODE", "starttls")),
		},
		OAuth: OAuthConfig{
			Google: OAuthProviderConfig{
				ClientID:     env("GOOGLE_CLIENT_ID", ""),
				ClientSecret: env("GOOGLE_CLIENT_SECRET", ""),
				RedirectURL:  env("GOOGLE_REDIRECT_URL", ""),
				AuthURL:      env("GOOGLE_AUTH_URL", "https://accounts.google.com/o/oauth2/v2/auth"),
				TokenURL:     env("GOOGLE_TOKEN_URL", "https://oauth2.googleapis.com/token"),
				UserInfoURL:  env("GOOGLE_USERINFO_URL", "https://openidconnect.googleapis.com/v1/userinfo"),
				Scopes:       splitCSV(env("GOOGLE_SCOPES", "openid,email,profile")),
			},
			GitHub: OAuthProviderConfig{
				ClientID:     env("GITHUB_CLIENT_ID", ""),
				ClientSecret: env("GITHUB_CLIENT_SECRET", ""),
				RedirectURL:  env("GITHUB_REDIRECT_URL", ""),
				AuthURL:      env("GITHUB_AUTH_URL", "https://github.com/login/oauth/authorize"),
				TokenURL:     env("GITHUB_TOKEN_URL", "https://github.com/login/oauth/access_token"),
				UserInfoURL:  env("GITHUB_USERINFO_URL", "https://api.github.com/user"),
				EmailURL:     env("GITHUB_EMAIL_URL", "https://api.github.com/user/emails"),
				Scopes:       splitCSV(env("GITHUB_SCOPES", "read:user,user:email")),
			},
		},
		SePay: SePayConfig{
			WebhookAuthMode: strings.ToLower(env("SEPAY_WEBHOOK_AUTH_MODE", "api_key")),
			WebhookAPIKey:   env("SEPAY_WEBHOOK_API_KEY", ""),
			WebhookSecret:   env("SEPAY_WEBHOOK_SECRET", ""),
			MerchantID:      env("SEPAY_MERCHANT_ID", ""),
			CheckoutSecret:  env("SEPAY_CHECKOUT_SECRET", ""),
			CheckoutURL:     env("SEPAY_CHECKOUT_URL", "https://pay.sepay.vn/v1/checkout/init"),
			PaymentMethod:   env("SEPAY_PAYMENT_METHOD", ""),
			SuccessURL:      env("SEPAY_SUCCESS_URL", ""),
			ErrorURL:        env("SEPAY_ERROR_URL", ""),
			CancelURL:       env("SEPAY_CANCEL_URL", ""),
			BankCode:        env("SEPAY_BANK_CODE", ""),
			BankName:        env("SEPAY_BANK_NAME", ""),
			BankAccount:     env("SEPAY_BANK_ACCOUNT", ""),
			BankHolder:      env("SEPAY_BANK_HOLDER", ""),
			QRBaseURL:       env("SEPAY_QR_BASE_URL", "https://qr.sepay.vn/img"),
			MockEnabled:     envBool("PAYMENT_MOCK_ENABLED", false),
			OrderTTL:        envDuration("PAYMENT_ORDER_TTL", 24*time.Hour),
		},
		LLM: LLMConfig{
			Enabled: envBool("LLM_ENABLED", false),
			BaseURL: strings.TrimRight(env("LLM_BASE_URL", ""), "/"),
			APIKey:  env("LLM_API_KEY", ""),
			Timeout: envDuration("LLM_TIMEOUT", 120*time.Second),
			Models:  splitCSV(env("LLM_MODELS", "deepsi-s1,deepsi-m1,deepsi-v1,deepsi-e1")),
		},
		CORS: CORSConfig{
			AllowedOrigins: splitCSV(env("CORS_ALLOWED_ORIGINS", "http://localhost:5173")),
		},
		Seed: SeedConfig{
			AdminEmail:    strings.ToLower(env("ADMIN_EMAIL", "")),
			AdminPassword: env("ADMIN_PASSWORD", ""),
			AdminName:     env("ADMIN_NAME", "DeepSI Admin"),
		},
	}

	if cfg.App.FrontendURL == "" {
		return nil, fmt.Errorf("FRONTEND_URL is required")
	}
	if cfg.Auth.JWTSecret == "" {
		if cfg.App.Env == "development" {
			cfg.Auth.JWTSecret = "dev-only-insecure-jwt-secret-change-me-please"
		} else {
			return nil, fmt.Errorf("JWT_SECRET is required in production")
		}
	}
	if len(cfg.Auth.JWTSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}
	if cfg.Auth.APIKeyPepper == "" {
		if cfg.App.Env == "development" {
			cfg.Auth.APIKeyPepper = "dev-only-api-key-pepper-change-me"
		} else {
			return nil, fmt.Errorf("API_KEY_PEPPER is required in production")
		}
	}
	if cfg.Auth.BcryptCost < 10 || cfg.Auth.BcryptCost > 15 {
		return nil, fmt.Errorf("BCRYPT_COST must be between 10 and 15")
	}
	if err := cfg.validateSMTP(); err != nil {
		return nil, err
	}
	if cfg.SePay.WebhookAuthMode != "api_key" && cfg.SePay.WebhookAuthMode != "hmac" && cfg.SePay.WebhookAuthMode != "none" {
		return nil, fmt.Errorf("SEPAY_WEBHOOK_AUTH_MODE must be api_key, hmac or none")
	}
	if cfg.App.Env == "production" && cfg.SePay.WebhookAuthMode == "none" {
		return nil, fmt.Errorf("SEPAY_WEBHOOK_AUTH_MODE=none is not allowed in production")
	}
	if cfg.SePay.WebhookAuthMode == "api_key" && cfg.SePay.WebhookAPIKey == "" {
		slog.Warn("SEPAY_WEBHOOK_API_KEY is empty; SePay webhook will reject all requests until configured")
	}
	if cfg.SePay.WebhookAuthMode == "hmac" && cfg.SePay.WebhookSecret == "" {
		slog.Warn("SEPAY_WEBHOOK_SECRET is empty; SePay webhook will reject all requests until configured")
	}

	cfg.Plans = LoadPlans()
	return cfg, nil
}

func env(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return strings.TrimSpace(value)
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	value, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return parsed
}

func envInt(key string, fallback int) int {
	value, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return parsed
}

func envInt64(key string, fallback int64) int64 {
	value, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return parsed
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func parseLogLevel(value string) slog.Level {
	switch strings.ToLower(value) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func (c *Config) IsProduction() bool { return c.App.Env == "production" }

func (c *Config) validateSMTP() error {
	smtp := c.SMTP
	if strings.TrimSpace(smtp.Host) == "" {
		if c.IsProduction() && c.Auth.RequireEmailVerification {
			return fmt.Errorf("SMTP_HOST is required when REQUIRE_EMAIL_VERIFICATION is enabled in production")
		}
		return nil
	}
	if smtp.Port < 1 || smtp.Port > 65535 {
		return fmt.Errorf("SMTP_PORT must be between 1 and 65535")
	}
	if smtp.TLSMode != "starttls" && smtp.TLSMode != "tls" && smtp.TLSMode != "none" {
		return fmt.Errorf("SMTP_TLS_MODE must be starttls, tls or none")
	}
	if (smtp.Username == "") != (smtp.Password == "") {
		return fmt.Errorf("SMTP_USERNAME and SMTP_PASSWORD must be configured together")
	}
	if c.IsProduction() && smtp.TLSMode == "none" {
		return fmt.Errorf("SMTP_TLS_MODE=none is not allowed in production")
	}
	if smtp.Username != "" && smtp.TLSMode == "none" {
		return fmt.Errorf("SMTP authentication requires TLS; use SMTP_TLS_MODE=starttls or tls")
	}
	from := strings.TrimSpace(smtp.FromEmail)
	if from == "" {
		from = strings.TrimSpace(smtp.Username)
	}
	if from == "" {
		return fmt.Errorf("SMTP_FROM_EMAIL is required when SMTP_HOST is configured")
	}
	address, err := mail.ParseAddress(from)
	if err != nil || address.Address == "" || strings.ContainsAny(address.Address, "\r\n") {
		return fmt.Errorf("SMTP_FROM_EMAIL must be a valid email address")
	}
	return nil
}
