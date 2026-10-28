package mongo

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/deepsi/agent-service/internal/domain"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type UserRepository struct {
	col *mongo.Collection
}

func NewUserRepository(db *mongo.Database) *UserRepository {
	return &UserRepository{col: db.Collection("users")}
}

func (r *UserRepository) Create(ctx context.Context, user *domain.User) error {
	now := time.Now()
	user.CreatedAt = now
	user.UpdatedAt = now
	if _, err := r.col.InsertOne(ctx, user); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return domain.ErrDuplicate
		}
		return err
	}
	return nil
}

func (r *UserRepository) FindByID(ctx context.Context, id string) (*domain.User, error) {
	return r.findOne(ctx, bson.M{"public_id": id})
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	return r.findOne(ctx, bson.M{"email": strings.ToLower(strings.TrimSpace(email))})
}

func (r *UserRepository) FindByProvider(ctx context.Context, provider, providerID string) (*domain.User, error) {
	return r.findOne(ctx, bson.M{"provider": provider, "provider_id": providerID})
}

func (r *UserRepository) findOne(ctx context.Context, filter bson.M) (*domain.User, error) {
	var user domain.User
	err := r.col.FindOne(ctx, filter).Decode(&user)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) Update(ctx context.Context, user *domain.User) error {
	user.UpdatedAt = time.Now()
	result, err := r.col.ReplaceOne(ctx, bson.M{"public_id": user.ID}, user)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *UserRepository) UpdateProfile(ctx context.Context, id, name, avatarColor string) error {
	update := bson.M{"updated_at": time.Now()}
	if name != "" {
		update["name"] = name
	}
	if avatarColor != "" {
		update["avatar_color"] = avatarColor
	}
	return r.updateOne(ctx, bson.M{"public_id": id}, bson.M{"$set": update})
}

func (r *UserRepository) SetPasswordHash(ctx context.Context, id, hash string) error {
	return r.updateOne(ctx, bson.M{"public_id": id}, bson.M{"$set": bson.M{
		"password_hash": hash,
		"updated_at":    time.Now(),
	}})
}

func (r *UserRepository) MarkEmailVerified(ctx context.Context, id string) error {
	return r.updateOne(ctx, bson.M{"public_id": id}, bson.M{"$set": bson.M{
		"email_verified": true,
		"updated_at":     time.Now(),
	}})
}

func (r *UserRepository) SetRole(ctx context.Context, id, role string) error {
	return r.updateOne(ctx, bson.M{"public_id": id}, bson.M{"$set": bson.M{
		"role":       role,
		"updated_at": time.Now(),
	}})
}

func (r *UserRepository) SetPlan(ctx context.Context, id string, plan domain.PlanID, startedAt, expiresAt *time.Time, cancelAtPeriodEnd bool) error {
	set := bson.M{
		"plan":                 plan,
		"cancel_at_period_end": cancelAtPeriodEnd,
		"updated_at":           time.Now(),
	}
	if startedAt != nil {
		set["plan_started_at"] = *startedAt
	}
	if expiresAt != nil {
		set["plan_expires_at"] = *expiresAt
	} else {
		set["plan_expires_at"] = nil
	}
	return r.updateOne(ctx, bson.M{"public_id": id}, bson.M{"$set": set})
}

func (r *UserRepository) SetCancelAtPeriodEnd(ctx context.Context, id string, cancel bool) error {
	return r.updateOne(ctx, bson.M{"public_id": id}, bson.M{"$set": bson.M{
		"cancel_at_period_end": cancel,
		"updated_at":           time.Now(),
	}})
}

func (r *UserRepository) TouchLogin(ctx context.Context, id string, at time.Time) error {
	return r.updateOne(ctx, bson.M{"public_id": id}, bson.M{"$set": bson.M{
		"last_login_at": at,
		"updated_at":    at,
	}})
}

func (r *UserRepository) updateOne(ctx context.Context, filter, update bson.M) error {
	result, err := r.col.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *UserRepository) List(ctx context.Context, filter domain.UserFilter) ([]*domain.User, int64, error) {
	query := bson.M{}
	if filter.Query != "" {
		safe := regexp.QuoteMeta(strings.TrimSpace(filter.Query))
		query["$or"] = bson.A{
			bson.M{"name": bson.M{"$regex": safe, "$options": "i"}},
			bson.M{"email": bson.M{"$regex": safe, "$options": "i"}},
			bson.M{"public_id": safe},
		}
	}
	if filter.Role != "" {
		query["role"] = filter.Role
	}
	if filter.Plan != "" {
		query["plan"] = filter.Plan
	}
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	total, err := r.col.CountDocuments(ctx, query)
	if err != nil {
		return nil, 0, err
	}
	cursor, err := r.col.Find(ctx, query, options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(limit).
		SetSkip(offset))
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	users := make([]*domain.User, 0, limit)
	for cursor.Next(ctx) {
		var user domain.User
		if err := cursor.Decode(&user); err != nil {
			return nil, 0, err
		}
		users = append(users, &user)
	}
	return users, total, cursor.Err()
}

func (r *UserRepository) Count(ctx context.Context) (int64, error) {
	return r.col.CountDocuments(ctx, bson.M{})
}

func (r *UserRepository) CountByPlan(ctx context.Context) (map[domain.PlanID]int64, error) {
	cursor, err := r.col.Aggregate(ctx, mongo.Pipeline{
		{{Key: "$group", Value: bson.M{"_id": "$plan", "count": bson.M{"$sum": 1}}}},
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	out := map[domain.PlanID]int64{
		domain.PlanFree:   0,
		domain.PlanPlus:   0,
		domain.PlanUltral: 0,
	}
	for cursor.Next(ctx) {
		var row struct {
			ID    domain.PlanID `bson:"_id"`
			Count int64         `bson:"count"`
		}
		if err := cursor.Decode(&row); err != nil {
			return nil, err
		}
		out[row.ID] = row.Count
	}
	return out, cursor.Err()
}

func (r *UserRepository) ExpirePlans(ctx context.Context, now time.Time) (int64, error) {
	result, err := r.col.UpdateMany(ctx,
		bson.M{
			"plan":            bson.M{"$in": []domain.PlanID{domain.PlanPlus, domain.PlanUltral}},
			"plan_expires_at": bson.M{"$lt": now},
		},
		bson.M{"$set": bson.M{
			"plan":                 domain.PlanFree,
			"plan_expires_at":      nil,
			"cancel_at_period_end": false,
			"updated_at":           now,
		}},
	)
	if err != nil {
		return 0, err
	}
	return result.ModifiedCount, nil
}
