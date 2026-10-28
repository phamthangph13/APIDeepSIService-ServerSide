package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/deepsi/agent-service/internal/config"
	"github.com/deepsi/agent-service/internal/domain"
)

var paymentCodePattern = regexp.MustCompile(`DSI[A-Z0-9]{8}`)

type SePayService struct {
	cfg *config.Config
}

func NewSePayService(cfg *config.Config) *SePayService {
	return &SePayService{cfg: cfg}
}

func (s *SePayService) VerifyAPIKey(authorization string) bool {
	expected := s.cfg.SePay.WebhookAPIKey
	if expected == "" {
		return false
	}
	value := strings.TrimSpace(authorization)
	if len(value) < 7 || !strings.EqualFold(value[:7], "Apikey ") {
		return false
	}
	return constantTimeEqual(value[7:], expected)
}

func (s *SePayService) VerifyHMAC(timestamp, signature string, body []byte) bool {
	secret := s.cfg.SePay.WebhookSecret
	if secret == "" {
		return false
	}
	seconds, err := strconv.ParseInt(strings.TrimSpace(timestamp), 10, 64)
	if err != nil {
		return false
	}
	if delta := time.Since(time.Unix(seconds, 0)); delta > 5*time.Minute || delta < -5*time.Minute {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strings.TrimSpace(timestamp)))
	mac.Write([]byte("."))
	mac.Write(body)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	return constantTimeEqual(strings.TrimSpace(signature), expected)
}

// SignCheckout implements SePay Payment Gateway's HMAC-SHA256 signature. Field
// order is fixed by SePay and must not be changed.
func (s *SePayService) SignCheckout(fields map[string]string) string {
	allowed := []string{
		"order_amount", "merchant", "currency", "operation",
		"order_description", "order_invoice_number", "customer_id",
		"payment_method", "success_url", "error_url", "cancel_url",
	}
	parts := make([]string, 0, len(allowed))
	for _, key := range allowed {
		if value, ok := fields[key]; ok && value != "" {
			parts = append(parts, key+"="+value)
		}
	}
	mac := hmac.New(sha256.New, []byte(s.cfg.SePay.CheckoutSecret))
	mac.Write([]byte(strings.Join(parts, ",")))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func (s *SePayService) ExtractPaymentCode(payload domain.SePayWebhookPayload) string {
	if payload.Code != nil {
		code := strings.ToUpper(strings.TrimSpace(*payload.Code))
		if code != "" {
			return code
		}
	}
	for _, value := range []string{payload.Content, payload.Description} {
		if match := paymentCodePattern.FindString(strings.ToUpper(value)); match != "" {
			return match
		}
	}
	return ""
}

func (s *SePayService) GatewayConfigured() bool {
	return s.cfg.SePay.MerchantID != "" && s.cfg.SePay.CheckoutSecret != ""
}

func (s *SePayService) BankTransferConfigured() bool {
	return s.cfg.SePay.BankAccount != ""
}

func (s *SePayService) PaymentConfigured() bool {
	return s.GatewayConfigured() || s.BankTransferConfigured() || s.cfg.SePay.MockEnabled
}

func (s *SePayService) BuildGatewayFields(order *domain.PaymentOrder, plan domain.Plan) map[string]string {
	successURL := appendOrderID(s.cfg.SePay.SuccessURL, order.ID)
	errorURL := appendOrderID(s.cfg.SePay.ErrorURL, order.ID)
	cancelURL := appendOrderID(s.cfg.SePay.CancelURL, order.ID)
	fields := map[string]string{
		"order_amount":         strconv.FormatInt(order.AmountVND, 10),
		"merchant":             s.cfg.SePay.MerchantID,
		"currency":             "VND",
		"operation":            "PURCHASE",
		"order_description":    fmt.Sprintf("DeepSI Agent - %s - %s", plan.Name, order.InvoiceNumber),
		"order_invoice_number": order.InvoiceNumber,
		"customer_id":          order.UserID,
		"success_url":          successURL,
		"error_url":            errorURL,
		"cancel_url":           cancelURL,
	}
	if s.cfg.SePay.PaymentMethod != "" {
		fields["payment_method"] = s.cfg.SePay.PaymentMethod
	}
	fields["signature"] = s.SignCheckout(fields)
	return fields
}

func appendOrderID(base, orderID string) string {
	if base == "" {
		return ""
	}
	separator := "?"
	if strings.Contains(base, "?") {
		separator = "&"
	}
	return base + separator + "order_id=" + orderID
}

func constantTimeEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := 0; i < len(a); i++ {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}
