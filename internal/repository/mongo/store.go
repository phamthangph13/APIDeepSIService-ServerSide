package mongo

import (
	"context"

	"github.com/deepsi/agent-service/internal/domain"
	platformmongo "github.com/deepsi/agent-service/internal/platform/mongo"
	"go.mongodb.org/mongo-driver/mongo"
)

type Store struct {
	Users    *UserRepository
	Tokens   *TokenRepository
	APIKeys  *APIKeyRepository
	Usage    *UsageRepository
	Payments *PaymentRepository
	Webhooks *WebhookRepository
}

func NewStore(db *mongo.Database) *Store {
	return &Store{
		Users:    NewUserRepository(db),
		Tokens:   NewTokenRepository(db),
		APIKeys:  NewAPIKeyRepository(db),
		Usage:    NewUsageRepository(db),
		Payments: NewPaymentRepository(db),
		Webhooks: NewWebhookRepository(db),
	}
}

func (s *Store) EnsureIndexes(ctx context.Context, db *mongo.Database) error {
	return platformmongo.EnsureIndexes(ctx, db)
}

var (
	_ domain.UserRepository    = (*UserRepository)(nil)
	_ domain.TokenRepository   = (*TokenRepository)(nil)
	_ domain.APIKeyRepository  = (*APIKeyRepository)(nil)
	_ domain.UsageRepository   = (*UsageRepository)(nil)
	_ domain.PaymentRepository = (*PaymentRepository)(nil)
	_ domain.WebhookRepository = (*WebhookRepository)(nil)
)
