package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strconv"
	"testing"
	"time"

	"github.com/deepsi/agent-service/internal/config"
	"github.com/deepsi/agent-service/internal/domain"
)

func sepayTestConfig() *config.Config {
	return &config.Config{SePay: config.SePayConfig{
		WebhookAuthMode: "api_key",
		WebhookAPIKey:   "webhook-secret",
		WebhookSecret:   "hmac-secret",
		MerchantID:      "merchant-1",
		CheckoutSecret:  "checkout-secret",
		BankAccount:     "0123456789",
	}}
}

func TestVerifyAPIKey(t *testing.T) {
	service := NewSePayService(sepayTestConfig())
	if !service.VerifyAPIKey("Apikey webhook-secret") {
		t.Fatal("valid API key was rejected")
	}
	if !service.VerifyAPIKey("apikey webhook-secret") {
		t.Fatal("API key comparison should be case-insensitive for the scheme")
	}
	if service.VerifyAPIKey("Apikey wrong") {
		t.Fatal("invalid API key was accepted")
	}
	if service.VerifyAPIKey("Bearer webhook-secret") {
		t.Fatal("wrong scheme was accepted")
	}
	if service.VerifyAPIKey("") {
		t.Fatal("empty authorization was accepted")
	}
}

func TestVerifyHMAC(t *testing.T) {
	service := NewSePayService(sepayTestConfig())
	body := []byte(`{"id":1,"transferType":"in"}`)
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte("hmac-secret"))
	mac.Write([]byte(timestamp + "."))
	mac.Write(body)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	if !service.VerifyHMAC(timestamp, signature, body) {
		t.Fatal("valid signature was rejected")
	}
	if service.VerifyHMAC(timestamp, signature, []byte(`{"id":2}`)) {
		t.Fatal("signature for another body was accepted")
	}
	old := strconv.FormatInt(time.Now().Add(-10*time.Minute).Unix(), 10)
	if service.VerifyHMAC(old, signature, body) {
		t.Fatal("stale timestamp was accepted")
	}
}

func TestSignCheckout(t *testing.T) {
	service := NewSePayService(sepayTestConfig())
	fields := map[string]string{
		"order_amount":         "1499000",
		"merchant":             "merchant-1",
		"currency":             "VND",
		"operation":            "PURCHASE",
		"order_description":    "DeepSI Agent - Plus",
		"order_invoice_number": "DSI-20261028-AB12CD",
		"customer_id":          "usr_1",
		"success_url":          "https://example.com/success",
		"error_url":            "https://example.com/error",
		"cancel_url":           "https://example.com/cancel",
	}
	canonical := "order_amount=1499000,merchant=merchant-1,currency=VND,operation=PURCHASE," +
		"order_description=DeepSI Agent - Plus,order_invoice_number=DSI-20261028-AB12CD," +
		"customer_id=usr_1,success_url=https://example.com/success," +
		"error_url=https://example.com/error,cancel_url=https://example.com/cancel"
	mac := hmac.New(sha256.New, []byte("checkout-secret"))
	mac.Write([]byte(canonical))
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	if got := service.SignCheckout(fields); got != expected {
		t.Fatalf("SignCheckout() = %q, want %q", got, expected)
	}
}

func TestExtractPaymentCode(t *testing.T) {
	service := NewSePayService(sepayTestConfig())
	code := "dsi1a2b3c4d"
	if got := service.ExtractPaymentCode(domain.SePayWebhookPayload{Code: &code}); got != "DSI1A2B3C4D" {
		t.Fatalf("ExtractPaymentCode() = %q", got)
	}
	if got := service.ExtractPaymentCode(domain.SePayWebhookPayload{
		Content: "Chuyen tien DSI1A2B3C4D cam on",
	}); got != "DSI1A2B3C4D" {
		t.Fatalf("ExtractPaymentCode() from content = %q", got)
	}
	if got := service.ExtractPaymentCode(domain.SePayWebhookPayload{Content: "no code"}); got != "" {
		t.Fatalf("ExtractPaymentCode() = %q, want empty", got)
	}
}

func TestPaymentConfiguration(t *testing.T) {
	service := NewSePayService(sepayTestConfig())
	if !service.GatewayConfigured() {
		t.Fatal("gateway should be configured")
	}
	if !service.BankTransferConfigured() {
		t.Fatal("bank transfer should be configured")
	}
	if !service.PaymentConfigured() {
		t.Fatal("payment should be configured")
	}
	empty := NewSePayService(&config.Config{})
	if empty.PaymentConfigured() {
		t.Fatal("empty config should not be configured")
	}
}
