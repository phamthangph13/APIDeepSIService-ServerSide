package domain

import "time"

const (
	PaymentStatusPending   = "pending"
	PaymentStatusPaid      = "paid"
	PaymentStatusExpired   = "expired"
	PaymentStatusCancelled = "cancelled"
	PaymentStatusFailed    = "failed"

	PaymentProviderSePay = "sepay"
)

type PaymentOrder struct {
	ID                  string     `bson:"public_id" json:"id"`
	UserID              string     `bson:"user_id" json:"-"`
	PlanID              PlanID     `bson:"plan_id" json:"plan_id"`
	AmountVND           int64      `bson:"amount_vnd" json:"amount_vnd"`
	Currency            string     `bson:"currency" json:"currency"`
	InvoiceNumber       string     `bson:"invoice_number" json:"invoice_number"`
	PaymentCode         string     `bson:"payment_code" json:"payment_code"`
	Status              string     `bson:"status" json:"status"`
	Provider            string     `bson:"provider" json:"provider"`
	ProviderTransaction string     `bson:"provider_transaction,omitempty" json:"provider_transaction,omitempty"`
	Description         string     `bson:"description" json:"description"`
	CreatedAt           time.Time  `bson:"created_at" json:"created_at"`
	UpdatedAt           time.Time  `bson:"updated_at" json:"updated_at"`
	ExpiresAt           time.Time  `bson:"expires_at" json:"expires_at"`
	PaidAt              *time.Time `bson:"paid_at,omitempty" json:"paid_at,omitempty"`
}

type PaymentCheckout struct {
	Order        *PaymentOrder     `json:"order"`
	Mode         string            `json:"mode"` // gateway | bank_transfer | mock
	Action       string            `json:"action,omitempty"`
	Method       string            `json:"method,omitempty"`
	Fields       map[string]string `json:"fields,omitempty"`
	BankTransfer *BankTransfer     `json:"bank_transfer,omitempty"`
	Mock         bool              `json:"mock,omitempty"`
}

type BankTransfer struct {
	BankCode      string `json:"bank_code"`
	BankName      string `json:"bank_name,omitempty"`
	AccountNumber string `json:"account_number"`
	AccountHolder string `json:"account_holder"`
	AmountVND     int64  `json:"amount_vnd"`
	Content       string `json:"content"`
	QRURL         string `json:"qr_url"`
}

type Invoice struct {
	ID            string     `json:"id"`
	InvoiceNumber string     `json:"invoice_number"`
	PlanID        PlanID     `json:"plan_id"`
	PlanName      string     `json:"plan_name"`
	AmountVND     int64      `json:"amount_vnd"`
	Status        string     `json:"status"`
	CreatedAt     time.Time  `json:"created_at"`
	PaidAt        *time.Time `json:"paid_at,omitempty"`
}

type BillingOverview struct {
	Plan              Plan          `json:"plan"`
	PlanStartedAt     *time.Time    `json:"plan_started_at,omitempty"`
	PlanExpiresAt     *time.Time    `json:"plan_expires_at,omitempty"`
	CancelAtPeriodEnd bool          `json:"cancel_at_period_end"`
	Usage             UsageSummary  `json:"usage"`
	Invoices          []Invoice     `json:"invoices"`
	PendingOrder      *PaymentOrder `json:"pending_order,omitempty"`
	PaymentConfigured bool          `json:"payment_configured"`
	PaymentMode       string        `json:"payment_mode"`
	BankTransfer      *BankTransfer `json:"bank_transfer,omitempty"`
}

type SePayWebhookPayload struct {
	ID              int64   `json:"id"`
	Gateway         string  `json:"gateway"`
	TransactionDate string  `json:"transactionDate"`
	AccountNumber   string  `json:"accountNumber"`
	SubAccount      string  `json:"subAccount"`
	Code            *string `json:"code"`
	Content         string  `json:"content"`
	TransferType    string  `json:"transferType"`
	Description     string  `json:"description"`
	TransferAmount  int64   `json:"transferAmount"`
	Accumulated     int64   `json:"accumulated"`
	ReferenceCode   string  `json:"referenceCode"`
}

type WebhookEvent struct {
	SePayID     int64      `bson:"sepay_id"`
	Status      string     `bson:"status"`
	Payload     string     `bson:"payload"`
	Error       string     `bson:"error,omitempty"`
	ReceivedAt  time.Time  `bson:"received_at"`
	ProcessedAt *time.Time `bson:"processed_at,omitempty"`
}
