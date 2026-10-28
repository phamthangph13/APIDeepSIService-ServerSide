package mongo

import (
	"context"
	"errors"
	"time"

	"github.com/deepsi/agent-service/internal/domain"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type APIKeyRepository struct {
	col *mongo.Collection
}

func NewAPIKeyRepository(db *mongo.Database) *APIKeyRepository {
	return &APIKeyRepository{col: db.Collection("api_keys")}
}

func (r *APIKeyRepository) Create(ctx context.Context, key *domain.APIKey) error {
	_, err := r.col.InsertOne(ctx, key)
	if mongo.IsDuplicateKeyError(err) {
		return domain.ErrDuplicate
	}
	return err
}

func (r *APIKeyRepository) ListByUser(ctx context.Context, userID string) ([]*domain.APIKey, error) {
	cursor, err := r.col.Find(ctx, bson.M{"user_id": userID}, options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	keys := make([]*domain.APIKey, 0)
	for cursor.Next(ctx) {
		var key domain.APIKey
		if err := cursor.Decode(&key); err != nil {
			return nil, err
		}
		keys = append(keys, &key)
	}
	return keys, cursor.Err()
}

func (r *APIKeyRepository) FindByID(ctx context.Context, userID, id string) (*domain.APIKey, error) {
	var key domain.APIKey
	err := r.col.FindOne(ctx, bson.M{"public_id": id, "user_id": userID}).Decode(&key)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &key, nil
}

func (r *APIKeyRepository) FindByHash(ctx context.Context, hash string) (*domain.APIKey, error) {
	var key domain.APIKey
	err := r.col.FindOne(ctx, bson.M{"key_hash": hash}).Decode(&key)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &key, nil
}

func (r *APIKeyRepository) Revoke(ctx context.Context, userID, id string, at time.Time) error {
	result, err := r.col.UpdateOne(ctx,
		bson.M{"public_id": id, "user_id": userID, "status": domain.KeyStatusActive},
		bson.M{"$set": bson.M{"status": domain.KeyStatusRevoked, "revoked_at": at}},
	)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *APIKeyRepository) TouchLastUsed(ctx context.Context, id string, at time.Time) error {
	_, err := r.col.UpdateOne(ctx, bson.M{"public_id": id}, bson.M{"$set": bson.M{"last_used_at": at}})
	return err
}

func (r *APIKeyRepository) CountActive(ctx context.Context, userID string) (int64, error) {
	return r.col.CountDocuments(ctx, bson.M{"user_id": userID, "status": domain.KeyStatusActive})
}

func (r *APIKeyRepository) CountAllActive(ctx context.Context) (int64, error) {
	return r.col.CountDocuments(ctx, bson.M{"status": domain.KeyStatusActive})
}
