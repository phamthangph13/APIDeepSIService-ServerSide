package domain

import (
	"context"
	"time"
)

type UserRepository interface {
	Create(ctx context.Context, user *User) error
	FindByID(ctx context.Context, id string) (*User, error)
	FindByEmail(ctx context.Context, email string) (*User, error)
	FindByProvider(ctx context.Context, provider, providerID string) (*User, error)
	Update(ctx context.Context, user *User) error
	UpdateProfile(ctx context.Context, id, name, avatarColor string) error
	SetPasswordHash(ctx context.Context, id, hash string) error
	MarkEmailVerified(ctx context.Context, id string) error
	SetRole(ctx context.Context, id, role string) error
	SetPlan(ctx context.Context, id string, plan PlanID, startedAt, expiresAt *time.Time, cancelAtPeriodEnd bool) error
	SetCancelAtPeriodEnd(ctx context.Context, id string, cancel bool) error
	TouchLogin(ctx context.Context, id string, at time.Time) error
	List(ctx context.Context, filter UserFilter) ([]*User, int64, error)
	Count(ctx context.Context) (int64, error)
	CountByPlan(ctx context.Context) (map[PlanID]int64, error)
	ExpirePlans(ctx context.Context, now time.Time) (int64, error)
}

type TokenRepository interface {
	CreateRefresh(ctx context.Context, token *RefreshToken) error
	FindRefreshByHash(ctx context.Context, hash string) (*RefreshToken, error)
	RevokeRefresh(ctx context.Context, hash string, at time.Time) error
	RevokeAllRefreshForUser(ctx context.Context, userID string, at time.Time) error
	CreateOneTime(ctx context.Context, token *OneTimeToken) error
	ConsumeOneTime(ctx context.Context, purpose, hash string, at time.Time) (*OneTimeToken, error)
}

type APIKeyRepository interface {
	Create(ctx context.Context, key *APIKey) error
	ListByUser(ctx context.Context, userID string) ([]*APIKey, error)
	FindByID(ctx context.Context, userID, id string) (*APIKey, error)
	FindByHash(ctx context.Context, hash string) (*APIKey, error)
	Revoke(ctx context.Context, userID, id string, at time.Time) error
	TouchLastUsed(ctx context.Context, id string, at time.Time) error
	CountActive(ctx context.Context, userID string) (int64, error)
	CountAllActive(ctx context.Context) (int64, error)
}

type UsageRepository interface {
	Insert(ctx context.Context, log *UsageLog) error
	Summary(ctx context.Context, userID string, from, to time.Time, timezone string) (*UsageSummary, error)
	Activity(ctx context.Context, userID string, limit int64) ([]ActivityRow, error)
	Sum(ctx context.Context, userID string, from, to time.Time) (UsageAggregate, error)
	AdminTotals(ctx context.Context, from, to time.Time) (UsageAggregate, error)
}

type PaymentRepository interface {
	CreateOrder(ctx context.Context, order *PaymentOrder) error
	FindOrderByID(ctx context.Context, userID, id string) (*PaymentOrder, error)
	FindOrderByPaymentCode(ctx context.Context, code string) (*PaymentOrder, error)
	ListOrdersByUser(ctx context.Context, userID string, limit int64) ([]*PaymentOrder, error)
	ListInvoicesByUser(ctx context.Context, userID string, limit int64) ([]Invoice, error)
	MarkOrderPaid(ctx context.Context, id, providerTransaction string, at time.Time) (bool, error)
	UpdateOrderStatus(ctx context.Context, id, status string, at time.Time) error
	AdminListOrders(ctx context.Context, status string, limit, offset int64) ([]*PaymentOrder, int64, error)
	Revenue(ctx context.Context, from, to time.Time) (int64, error)
}

type WebhookRepository interface {
	CreateEvent(ctx context.Context, event *WebhookEvent) (bool, error)
	FindEvent(ctx context.Context, sepayID int64) (*WebhookEvent, error)
	MarkEventProcessed(ctx context.Context, sepayID int64, status, errMessage string, at time.Time) error
}
