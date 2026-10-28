package service

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/deepsi/agent-service/internal/config"
	"github.com/deepsi/agent-service/internal/domain"
	"github.com/deepsi/agent-service/internal/platform/security"
	"github.com/google/uuid"
)

type BillingService struct {
	cfg      *config.Config
	users    domain.UserRepository
	payments domain.PaymentRepository
	usage    *UsageService
	email    *EmailService
	sepay    *SePayService
	log      *slog.Logger
}

func NewBillingService(
	cfg *config.Config,
	users domain.UserRepository,
	payments domain.PaymentRepository,
	usage *UsageService,
	email *EmailService,
	sepay *SePayService,
	log *slog.Logger,
) *BillingService {
	return &BillingService{cfg: cfg, users: users, payments: payments, usage: usage, email: email, sepay: sepay, log: log}
}

func (s *BillingService) Overview(ctx context.Context, user *domain.User) (*domain.BillingOverview, error) {
	plan := s.cfg.Plans.MustGet(user.EffectivePlan(time.Now()))
	summary, err := s.usage.Summary(ctx, user, 30)
	if err != nil {
		return nil, err
	}
	invoices, err := s.payments.ListInvoicesByUser(ctx, user.ID, 50)
	if err != nil {
		return nil, err
	}
	for i := range invoices {
		invoices[i].PlanName = s.cfg.Plans.MustGet(invoices[i].PlanID).Name
	}
	orders, err := s.payments.ListOrdersByUser(ctx, user.ID, 10)
	if err != nil {
		return nil, err
	}
	var pending *domain.PaymentOrder
	for _, order := range orders {
		if order.Status == domain.PaymentStatusPending && order.ExpiresAt.After(time.Now()) {
			pending = order
			break
		}
	}

	overview := &domain.BillingOverview{
		Plan:              plan,
		PlanStartedAt:     user.PlanStartedAt,
		PlanExpiresAt:     user.PlanExpiresAt,
		CancelAtPeriodEnd: user.CancelAtPeriodEnd,
		Usage:             *summary,
		Invoices:          invoices,
		PendingOrder:      pending,
		PaymentConfigured: s.sepay.PaymentConfigured(),
		PaymentMode:       s.paymentMode(),
	}
	if s.sepay.BankTransferConfigured() {
		overview.BankTransfer = s.bankTransferInfo(plan.PriceVND, "")
	}
	return overview, nil
}

func (s *BillingService) Checkout(ctx context.Context, user *domain.User, planID domain.PlanID) (*domain.PaymentCheckout, error) {
	plan, ok := s.cfg.Plans.Get(planID)
	if !ok || plan.ID == domain.PlanFree {
		return nil, domain.Invalid("invalid_plan", "Gói dịch vụ không hợp lệ.")
	}
	if plan.PriceVND <= 0 {
		return nil, domain.Invalid("invalid_plan", "Gói này không cần thanh toán.")
	}
	if user.EffectivePlan(time.Now()) == plan.ID {
		return nil, domain.Conflict("already_on_plan", "Bạn đang dùng gói "+plan.Name+".")
	}

	// Reuse a pending order for the same plan so refreshing the page does not
	// create duplicate bank-transfer codes.
	if orders, err := s.payments.ListOrdersByUser(ctx, user.ID, 10); err == nil {
		for _, order := range orders {
			if order.PlanID == plan.ID && order.Status == domain.PaymentStatusPending && order.ExpiresAt.After(time.Now()) {
				return s.buildCheckout(order, plan)
			}
		}
	}

	now := time.Now()
	order := &domain.PaymentOrder{
		ID:            "ord_" + uuid.NewString(),
		UserID:        user.ID,
		PlanID:        plan.ID,
		AmountVND:     plan.PriceVND,
		Currency:      "VND",
		InvoiceNumber: s.invoiceNumber(now),
		PaymentCode:   s.paymentCode(),
		Status:        domain.PaymentStatusPending,
		Provider:      domain.PaymentProviderSePay,
		Description:   fmt.Sprintf("DeepSI Agent - %s", plan.Name),
		CreatedAt:     now,
		UpdatedAt:     now,
		ExpiresAt:     now.Add(s.cfg.SePay.OrderTTL),
	}
	if err := s.payments.CreateOrder(ctx, order); err != nil {
		return nil, err
	}
	return s.buildCheckout(order, plan)
}

func (s *BillingService) buildCheckout(order *domain.PaymentOrder, plan domain.Plan) (*domain.PaymentCheckout, error) {
	checkout := &domain.PaymentCheckout{Order: order}
	switch {
	case s.sepay.GatewayConfigured():
		fields := s.sepay.BuildGatewayFields(order, plan)
		checkout.Mode = "gateway"
		checkout.Action = s.cfg.SePay.CheckoutURL
		checkout.Method = "POST"
		checkout.Fields = fields
	case s.sepay.BankTransferConfigured():
		checkout.Mode = "bank_transfer"
		checkout.BankTransfer = s.bankTransferInfo(order.AmountVND, order.PaymentCode)
	case s.cfg.SePay.MockEnabled:
		checkout.Mode = "mock"
		checkout.Mock = true
	default:
		return nil, domain.NewError(503, "payment_not_configured", "Cổng thanh toán chưa được cấu hình. Vui lòng liên hệ hỗ trợ.", nil)
	}
	return checkout, nil
}

func (s *BillingService) OrderStatus(ctx context.Context, user *domain.User, orderID string) (*domain.PaymentOrder, error) {
	return s.payments.FindOrderByID(ctx, user.ID, orderID)
}

func (s *BillingService) CancelRenewal(ctx context.Context, user *domain.User) error {
	if !user.IsPaid() {
		return domain.Invalid("not_paid_plan", "Tài khoản đang dùng gói Free.")
	}
	return s.users.SetCancelAtPeriodEnd(ctx, user.ID, true)
}

func (s *BillingService) Downgrade(ctx context.Context, user *domain.User) error {
	if err := s.users.SetPlan(ctx, user.ID, domain.PlanFree, nil, nil, false); err != nil {
		return err
	}
	if orders, err := s.payments.ListOrdersByUser(ctx, user.ID, 20); err == nil {
		for _, order := range orders {
			if order.Status == domain.PaymentStatusPending {
				_ = s.payments.UpdateOrderStatus(ctx, order.ID, domain.PaymentStatusCancelled, time.Now())
			}
		}
	}
	return nil
}

// CompleteMockPayment is only enabled with PAYMENT_MOCK_ENABLED=true. It lets
// local development exercise the full billing flow without SePay credentials.
func (s *BillingService) CompleteMockPayment(ctx context.Context, user *domain.User, orderID string) (*domain.PaymentOrder, error) {
	if !s.cfg.SePay.MockEnabled {
		return nil, domain.NotFound("mock_payment_disabled", "Thanh toán mô phỏng đang tắt.")
	}
	order, err := s.payments.FindOrderByID(ctx, user.ID, orderID)
	if err != nil {
		return nil, err
	}
	if order.Status != domain.PaymentStatusPending {
		return order, nil
	}
	if order.ExpiresAt.Before(time.Now()) {
		return nil, domain.NewError(410, "order_expired", "Đơn thanh toán đã hết hạn.", nil)
	}
	ok, err := s.payments.MarkOrderPaid(ctx, order.ID, "MOCK-"+order.PaymentCode, time.Now())
	if err != nil {
		return nil, err
	}
	if ok {
		if err := s.activatePlan(ctx, order); err != nil {
			_ = s.payments.UpdateOrderStatus(ctx, order.ID, domain.PaymentStatusPending, time.Now())
			return nil, err
		}
	}
	return s.payments.FindOrderByID(ctx, user.ID, order.ID)
}

func (s *BillingService) HandleSePayWebhook(ctx context.Context, payload domain.SePayWebhookPayload) error {
	if payload.TransferType != "in" {
		return nil
	}
	code := s.sepay.ExtractPaymentCode(payload)
	if code == "" {
		return nil
	}
	order, err := s.payments.FindOrderByPaymentCode(ctx, code)
	if err != nil {
		s.log.Warn("sepay webhook: order not found", "code", code, "sepay_id", payload.ID)
		return nil
	}
	if order.Status == domain.PaymentStatusPaid {
		return nil
	}
	if order.Status != domain.PaymentStatusPending {
		s.log.Warn("sepay webhook: order is not pending", "code", code, "status", order.Status)
		return nil
	}
	if order.ExpiresAt.Before(time.Now()) {
		_ = s.payments.UpdateOrderStatus(ctx, order.ID, domain.PaymentStatusExpired, time.Now())
		s.log.Warn("sepay webhook: order expired", "code", code, "order_id", order.ID)
		return nil
	}
	if payload.TransferAmount < order.AmountVND {
		s.log.Warn("sepay webhook: underpaid", "code", code, "order_id", order.ID, "amount", payload.TransferAmount, "expected", order.AmountVND)
		return nil
	}
	transaction := fmt.Sprintf("%d", payload.ID)
	ok, err := s.payments.MarkOrderPaid(ctx, order.ID, transaction, time.Now())
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	if err := s.activatePlan(ctx, order); err != nil {
		// Keep the order retryable. SePay will redeliver the webhook and the
		// event is marked failed, so the next attempt can finish activation.
		_ = s.payments.UpdateOrderStatus(ctx, order.ID, domain.PaymentStatusPending, time.Now())
		return err
	}
	return nil
}

func (s *BillingService) activatePlan(ctx context.Context, order *domain.PaymentOrder) error {
	user, err := s.users.FindByID(ctx, order.UserID)
	if err != nil {
		return err
	}
	now := time.Now()
	start := now
	base := now
	if user.Plan == order.PlanID && user.PlanExpiresAt != nil && user.PlanExpiresAt.After(now) {
		base = *user.PlanExpiresAt
	}
	expiry := base.AddDate(0, 1, 0)
	if err := s.users.SetPlan(ctx, user.ID, order.PlanID, &start, &expiry, false); err != nil {
		return err
	}
	plan := s.cfg.Plans.MustGet(order.PlanID)
	user.Plan = order.PlanID
	user.PlanStartedAt = &start
	user.PlanExpiresAt = &expiry
	user.CancelAtPeriodEnd = false
	go func() {
		emailCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err := s.email.SendPaymentReceipt(emailCtx, user, plan, order); err != nil {
			s.log.Warn("send payment receipt failed", "user_id", user.ID, "error", err)
		}
	}()
	return nil
}

func (s *BillingService) ExpirePlans(ctx context.Context) (int64, error) {
	return s.users.ExpirePlans(ctx, time.Now())
}

func (s *BillingService) bankTransferInfo(amount int64, code string) *domain.BankTransfer {
	content := code
	if content == "" {
		content = "DSI"
	}
	qr := ""
	if s.cfg.SePay.QRBaseURL != "" {
		values := url.Values{}
		values.Set("acc", s.cfg.SePay.BankAccount)
		if s.cfg.SePay.BankCode != "" {
			values.Set("bank", s.cfg.SePay.BankCode)
		}
		values.Set("amount", fmt.Sprintf("%d", amount))
		values.Set("des", content)
		qr = strings.TrimRight(s.cfg.SePay.QRBaseURL, "?") + "?" + values.Encode()
	}
	return &domain.BankTransfer{
		BankCode:      s.cfg.SePay.BankCode,
		BankName:      s.cfg.SePay.BankName,
		AccountNumber: s.cfg.SePay.BankAccount,
		AccountHolder: s.cfg.SePay.BankHolder,
		AmountVND:     amount,
		Content:       content,
		QRURL:         qr,
	}
}

func (s *BillingService) paymentMode() string {
	switch {
	case s.sepay.GatewayConfigured():
		return "gateway"
	case s.sepay.BankTransferConfigured():
		return "bank_transfer"
	case s.cfg.SePay.MockEnabled:
		return "mock"
	default:
		return "none"
	}
}

// PaymentMode reports how checkout payments are currently collected.
func (s *BillingService) PaymentMode() string { return s.paymentMode() }

func (s *BillingService) invoiceNumber(now time.Time) string {
	random, _ := security.RandomHex(3)
	return fmt.Sprintf("DSI-%s-%s", now.Format("20060102"), strings.ToUpper(random))
}

func (s *BillingService) paymentCode() string {
	random, _ := security.RandomHex(4)
	return "DSI" + strings.ToUpper(random)
}
