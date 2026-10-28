package mongo

import (
	"context"
	"errors"
	"time"

	"github.com/deepsi/agent-service/internal/domain"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type TokenRepository struct {
	refresh *mongo.Collection
	oneTime *mongo.Collection
}

func NewTokenRepository(db *mongo.Database) *TokenRepository {
	return &TokenRepository{
		refresh: db.Collection("refresh_tokens"),
		oneTime: db.Collection("one_time_tokens"),
	}
}

func (r *TokenRepository) CreateRefresh(ctx context.Context, token *domain.RefreshToken) error {
	_, err := r.refresh.InsertOne(ctx, token)
	return err
}

func (r *TokenRepository) FindRefreshByHash(ctx context.Context, hash string) (*domain.RefreshToken, error) {
	var token domain.RefreshToken
	err := r.refresh.FindOne(ctx, bson.M{"token_hash": hash}).Decode(&token)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &token, nil
}

func (r *TokenRepository) RevokeRefresh(ctx context.Context, hash string, at time.Time) error {
	_, err := r.refresh.UpdateOne(ctx, bson.M{"token_hash": hash, "revoked_at": nil}, bson.M{"$set": bson.M{"revoked_at": at}})
	return err
}

func (r *TokenRepository) RevokeAllRefreshForUser(ctx context.Context, userID string, at time.Time) error {
	_, err := r.refresh.UpdateMany(ctx, bson.M{"user_id": userID, "revoked_at": nil}, bson.M{"$set": bson.M{"revoked_at": at}})
	return err
}

func (r *TokenRepository) CreateOneTime(ctx context.Context, token *domain.OneTimeToken) error {
	_, err := r.oneTime.InsertOne(ctx, token)
	return err
}

func (r *TokenRepository) ConsumeOneTime(ctx context.Context, purpose, hash string, at time.Time) (*domain.OneTimeToken, error) {
	var token domain.OneTimeToken
	err := r.oneTime.FindOneAndUpdate(ctx,
		bson.M{
			"token_hash": hash,
			"purpose":    purpose,
			"used_at":    nil,
			"expires_at": bson.M{"$gt": at},
		},
		bson.M{"$set": bson.M{"used_at": at}},
	).Decode(&token)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &token, nil
}
