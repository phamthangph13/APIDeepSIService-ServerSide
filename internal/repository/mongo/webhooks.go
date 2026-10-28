package mongo

import (
	"context"
	"errors"
	"time"

	"github.com/deepsi/agent-service/internal/domain"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type WebhookRepository struct {
	col *mongo.Collection
}

func NewWebhookRepository(db *mongo.Database) *WebhookRepository {
	return &WebhookRepository{col: db.Collection("webhook_events")}
}

// CreateEvent returns false when the SePay transaction was already received.
// The unique index on sepay_id is the final guard against concurrent retries.
func (r *WebhookRepository) CreateEvent(ctx context.Context, event *domain.WebhookEvent) (bool, error) {
	_, err := r.col.InsertOne(ctx, event)
	if mongo.IsDuplicateKeyError(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *WebhookRepository) FindEvent(ctx context.Context, sepayID int64) (*domain.WebhookEvent, error) {
	var event domain.WebhookEvent
	err := r.col.FindOne(ctx, bson.M{"sepay_id": sepayID}).Decode(&event)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &event, nil
}

func (r *WebhookRepository) MarkEventProcessed(ctx context.Context, sepayID int64, status, errMessage string, at time.Time) error {
	_, err := r.col.UpdateOne(ctx,
		bson.M{"sepay_id": sepayID},
		bson.M{"$set": bson.M{
			"status":       status,
			"error":        errMessage,
			"processed_at": at,
		}},
	)
	return err
}
