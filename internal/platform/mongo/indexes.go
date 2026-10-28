package mongo

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// EnsureIndexes is idempotent and safe to run on every boot.
func EnsureIndexes(ctx context.Context, db *mongo.Database) error {
	indexes := []struct {
		collection string
		models     []mongo.IndexModel
	}{
		{
			collection: "users",
			models: []mongo.IndexModel{
				{Keys: bson.D{{Key: "public_id", Value: 1}}, Options: options.Index().SetUnique(true)},
				{Keys: bson.D{{Key: "email", Value: 1}}, Options: options.Index().SetUnique(true)},
				{
					Keys: bson.D{{Key: "provider", Value: 1}, {Key: "provider_id", Value: 1}},
					Options: options.Index().
						SetUnique(true).
						SetPartialFilterExpression(bson.M{"provider_id": bson.M{"$type": "string", "$gt": ""}}),
				},
				{Keys: bson.D{{Key: "role", Value: 1}}},
				{Keys: bson.D{{Key: "plan", Value: 1}}},
				{Keys: bson.D{{Key: "created_at", Value: -1}}},
			},
		},
		{
			collection: "refresh_tokens",
			models: []mongo.IndexModel{
				{Keys: bson.D{{Key: "token_hash", Value: 1}}, Options: options.Index().SetUnique(true)},
				{Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "expires_at", Value: 1}}},
				{Keys: bson.D{{Key: "expires_at", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)},
			},
		},
		{
			collection: "one_time_tokens",
			models: []mongo.IndexModel{
				{Keys: bson.D{{Key: "token_hash", Value: 1}}, Options: options.Index().SetUnique(true)},
				{Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "purpose", Value: 1}}},
				{Keys: bson.D{{Key: "expires_at", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)},
			},
		},
		{
			collection: "api_keys",
			models: []mongo.IndexModel{
				{Keys: bson.D{{Key: "public_id", Value: 1}}, Options: options.Index().SetUnique(true)},
				{Keys: bson.D{{Key: "key_hash", Value: 1}}, Options: options.Index().SetUnique(true)},
				{Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "status", Value: 1}}},
				{Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "created_at", Value: -1}}},
			},
		},
		{
			collection: "usage_logs",
			models: []mongo.IndexModel{
				{Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "created_at", Value: -1}}},
				{Keys: bson.D{{Key: "api_key_id", Value: 1}, {Key: "created_at", Value: -1}}},
				{Keys: bson.D{{Key: "expires_at", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)},
			},
		},
		{
			collection: "payment_orders",
			models: []mongo.IndexModel{
				{Keys: bson.D{{Key: "public_id", Value: 1}}, Options: options.Index().SetUnique(true)},
				{Keys: bson.D{{Key: "invoice_number", Value: 1}}, Options: options.Index().SetUnique(true)},
				{Keys: bson.D{{Key: "payment_code", Value: 1}}, Options: options.Index().SetUnique(true)},
				{Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "created_at", Value: -1}}},
				{Keys: bson.D{{Key: "status", Value: 1}, {Key: "created_at", Value: -1}}},
				{Keys: bson.D{{Key: "expires_at", Value: 1}}},
			},
		},
		{
			collection: "webhook_events",
			models: []mongo.IndexModel{
				{Keys: bson.D{{Key: "sepay_id", Value: 1}}, Options: options.Index().SetUnique(true)},
				{Keys: bson.D{{Key: "received_at", Value: -1}}},
			},
		},
	}

	for _, entry := range indexes {
		if len(entry.models) == 0 {
			continue
		}
		if _, err := db.Collection(entry.collection).Indexes().CreateMany(ctx, entry.models); err != nil {
			return fmt.Errorf("create indexes for %s: %w", entry.collection, err)
		}
	}
	return nil
}
