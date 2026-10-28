package service

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"time"

	"github.com/deepsi/agent-service/internal/config"
	"github.com/deepsi/agent-service/internal/domain"
	"github.com/deepsi/agent-service/internal/platform/mailer"
)

type EmailService struct {
	cfg    *config.Config
	mailer mailer.Mailer
	log    *slog.Logger
}

func NewEmailService(cfg *config.Config, m mailer.Mailer, log *slog.Logger) *EmailService {
	return &EmailService{cfg: cfg, mailer: m, log: log}
}

func (s *EmailService) Configured() bool { return s.mailer.Configured() }

func (s *EmailService) SendResetPassword(ctx context.Context, user *domain.User, token string) error {
	link := s.frontendURL("/reset-password?token=" + url.QueryEscape(token))
	return s.send(ctx, mailer.ResetPasswordMessage(mailer.TemplateData{
		AppName: s.cfg.App.Name,
		Name:    user.Name,
		Email:   user.Email,
		URL:     link,
		Expires: formatDuration(s.cfg.Auth.ResetPasswordTTL),
	}))
}

func (s *EmailService) SendVerifyEmail(ctx context.Context, user *domain.User, token string) error {
	link := s.frontendURL("/verify-email?token=" + url.QueryEscape(token))
	return s.send(ctx, mailer.VerifyEmailMessage(mailer.TemplateData{
		AppName: s.cfg.App.Name,
		Name:    user.Name,
		Email:   user.Email,
		URL:     link,
		Expires: formatDuration(s.cfg.Auth.VerifyEmailTTL),
	}))
}

func (s *EmailService) SendWelcome(ctx context.Context, user *domain.User) error {
	return s.send(ctx, mailer.WelcomeMessage(mailer.TemplateData{
		AppName: s.cfg.App.Name,
		Name:    user.Name,
		Email:   user.Email,
		URL:     s.frontendURL("/dashboard"),
	}))
}

func (s *EmailService) SendPaymentReceipt(ctx context.Context, user *domain.User, plan domain.Plan, order *domain.PaymentOrder) error {
	return s.send(ctx, mailer.PaymentReceiptMessage(mailer.TemplateData{
		AppName:     s.cfg.App.Name,
		Name:        user.Name,
		Email:       user.Email,
		URL:         s.frontendURL("/dashboard/billing"),
		PlanName:    plan.Name,
		AmountLabel: formatVND(order.AmountVND),
		Invoice:     order.InvoiceNumber,
	}))
}

func (s *EmailService) send(ctx context.Context, message mailer.Message) error {
	if err := s.mailer.Send(ctx, message); err != nil {
		s.log.Error("send email failed", "to", message.To, "subject", message.Subject, "error", err)
		return err
	}
	return nil
}

func (s *EmailService) frontendURL(path string) string {
	return s.cfg.App.FrontendURL + path
}

func formatDuration(d time.Duration) string {
	if d >= 24*time.Hour {
		days := int(d.Hours() / 24)
		if days == 1 {
			return "24 giờ"
		}
		return fmt.Sprintf("%d ngày", days)
	}
	minutes := int(d.Minutes())
	if minutes >= 60 {
		hours := minutes / 60
		if hours == 1 {
			return "1 giờ"
		}
		return fmt.Sprintf("%d giờ", hours)
	}
	return fmt.Sprintf("%d phút", minutes)
}

func formatVND(value int64) string {
	if value == 0 {
		return "0đ"
	}
	raw := fmt.Sprintf("%d", value)
	out := make([]byte, 0, len(raw)+len(raw)/3+1)
	for i, r := range raw {
		if i > 0 && (len(raw)-i)%3 == 0 {
			out = append(out, '.')
		}
		out = append(out, byte(r))
	}
	return string(out) + "đ"
}
