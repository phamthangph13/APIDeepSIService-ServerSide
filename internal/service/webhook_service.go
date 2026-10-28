package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/deepsi/agent-service/internal/config"
	"github.com/deepsi/agent-service/internal/domain"
)

type WebhookHeaders struct {
	Authorization string
	Timestamp     string
	Signature     string
}

type WebhookService struct {
	cfg      *config.Config
	webhooks domain.WebhookRepository
	billing  *BillingService
	sepay    *SePayService
	log      *slog.Logger
}

func NewWebhookService(cfg *config.Config, webhooks domain.WebhookRepository, billing *BillingService, sepay *SePayService, log *slog.Logger) *WebhookService {
	return &WebhookService{cfg: cfg, webhooks: webhooks, billing: billing, sepay: sepay, log: log}
}

func (s *WebhookService) Verify(headers WebhookHeaders, body []byte) bool {
	switch s.cfg.SePay.WebhookAuthMode {
	case "api_key":
		return s.sepay.VerifyAPIKey(headers.Authorization)
	case "hmac":
		return s.sepay.VerifyHMAC(headers.Timestamp, headers.Signature, body)
	case "none":
		return !s.cfg.IsProduction()
	default:
		return false
	}
}

func (s *WebhookService) Process(ctx context.Context, body []byte) error {
	var payload domain.SePayWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return domain.Invalid("invalid_webhook_payload", "Payload webhook không hợp lệ.")
	}
	if payload.ID == 0 {
		return domain.Invalid("invalid_webhook_payload", "Payload webhook thiếu mã giao dịch.")
	}

	now := time.Now()
	event := &domain.WebhookEvent{
		SePayID:    payload.ID,
		Status:     "received",
		Payload:    string(body),
		ReceivedAt: now,
	}
	first, err := s.webhooks.CreateEvent(ctx, event)
	if err != nil {
		return err
	}
	if !first {
		// SePay retries failed deliveries. If a previous attempt failed before
		// the order was activated, let the retry run; otherwise acknowledge
		// without applying the payment twice.
		existing, findErr := s.webhooks.FindEvent(ctx, payload.ID)
		if findErr != nil {
			return findErr
		}
		if existing.Status != "failed" {
			return nil
		}
	}

	if err := s.billing.HandleSePayWebhook(ctx, payload); err != nil {
		_ = s.webhooks.MarkEventProcessed(ctx, payload.ID, "failed", err.Error(), time.Now())
		return err
	}
	if err := s.webhooks.MarkEventProcessed(ctx, payload.ID, "processed", "", time.Now()); err != nil {
		s.log.Warn("mark webhook processed failed", "sepay_id", payload.ID, "error", err)
	}
	return nil
}
