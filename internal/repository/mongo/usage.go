package mongo

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/deepsi/agent-service/internal/domain"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type UsageRepository struct {
	col *mongo.Collection
}

func NewUsageRepository(db *mongo.Database) *UsageRepository {
	return &UsageRepository{col: db.Collection("usage_logs")}
}

func (r *UsageRepository) Insert(ctx context.Context, log *domain.UsageLog) error {
	_, err := r.col.InsertOne(ctx, log)
	return err
}

func (r *UsageRepository) Summary(ctx context.Context, userID string, from, to time.Time, timezone string) (*domain.UsageSummary, error) {
	match := bson.M{"created_at": bson.M{"$gte": from, "$lt": to}}
	if userID != "" {
		match["user_id"] = userID
	}
	if timezone == "" {
		timezone = "Asia/Ho_Chi_Minh"
	}

	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: match}},
		{{Key: "$facet", Value: bson.M{
			"daily": bson.A{
				bson.M{"$group": bson.M{
					"_id": bson.M{"$dateToString": bson.M{
						"format":   "%Y-%m-%d",
						"date":     "$created_at",
						"timezone": timezone,
					}},
					"tokens":   bson.M{"$sum": "$total_tokens"},
					"requests": bson.M{"$sum": 1},
					"input":    bson.M{"$sum": "$prompt_tokens"},
					"output":   bson.M{"$sum": "$completion_tokens"},
				}},
				bson.M{"$sort": bson.M{"_id": 1}},
			},
			"models": bson.A{
				bson.M{"$group": bson.M{
					"_id":      "$model",
					"tokens":   bson.M{"$sum": "$total_tokens"},
					"requests": bson.M{"$sum": 1},
				}},
				bson.M{"$sort": bson.M{"tokens": -1}},
			},
			"totals": bson.A{
				bson.M{"$group": bson.M{
					"_id":         nil,
					"tokens":      bson.M{"$sum": "$total_tokens"},
					"requests":    bson.M{"$sum": 1},
					"input":       bson.M{"$sum": "$prompt_tokens"},
					"output":      bson.M{"$sum": "$completion_tokens"},
					"avg_latency": bson.M{"$avg": "$latency_ms"},
					"errors": bson.M{"$sum": bson.M{"$cond": bson.A{
						bson.M{"$gte": bson.A{"$status", 400}}, 1, 0,
					}}},
				}},
			},
		}}},
	}

	cursor, err := r.col.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var result struct {
		Daily []struct {
			Date     string `bson:"_id"`
			Tokens   int64  `bson:"tokens"`
			Requests int64  `bson:"requests"`
			Input    int64  `bson:"input"`
			Output   int64  `bson:"output"`
		} `bson:"daily"`
		Models []struct {
			ID       string `bson:"_id"`
			Tokens   int64  `bson:"tokens"`
			Requests int64  `bson:"requests"`
		} `bson:"models"`
		Totals []struct {
			Tokens     int64   `bson:"tokens"`
			Requests   int64   `bson:"requests"`
			Input      int64   `bson:"input"`
			Output     int64   `bson:"output"`
			AvgLatency float64 `bson:"avg_latency"`
			Errors     int64   `bson:"errors"`
		} `bson:"totals"`
	}
	if cursor.Next(ctx) {
		if err := cursor.Decode(&result); err != nil {
			return nil, err
		}
	}

	summary := &domain.UsageSummary{
		Days:   make([]domain.DayUsage, 0, len(result.Daily)),
		Models: make([]domain.ModelUsage, 0, len(result.Models)),
	}
	for _, day := range result.Daily {
		summary.Days = append(summary.Days, domain.DayUsage{
			DateKey:  day.Date,
			Tokens:   day.Tokens,
			Requests: day.Requests,
			Input:    day.Input,
			Output:   day.Output,
		})
	}
	for _, model := range result.Models {
		summary.Models = append(summary.Models, domain.ModelUsage{
			ID:       model.ID,
			Name:     model.ID,
			Tokens:   model.Tokens,
			Requests: model.Requests,
		})
	}
	if len(result.Totals) > 0 {
		total := result.Totals[0]
		summary.TotalTokens = total.Tokens
		summary.TotalRequests = total.Requests
		summary.InputTokens = total.Input
		summary.OutputTokens = total.Output
		summary.AvgLatency = int64(total.AvgLatency)
		if total.Requests > 0 {
			summary.ErrorRate = float64(total.Errors) / float64(total.Requests) * 100
		}
	}
	if p95, err := r.percentileLatency(ctx, match, 0.95); err == nil {
		summary.P95Latency = p95
	}
	if summary.P95Latency == 0 {
		summary.P95Latency = summary.AvgLatency
	}
	return summary, nil
}

// percentileLatency avoids the $percentile operator so the service also runs on
// MongoDB 6.x. It is only used for the small dashboard query.
func (r *UsageRepository) percentileLatency(ctx context.Context, match bson.M, percentile float64) (int64, error) {
	total, err := r.col.CountDocuments(ctx, match)
	if err != nil || total == 0 {
		return 0, err
	}
	skip := int64(math.Floor(float64(total) * percentile))
	if skip >= total {
		skip = total - 1
	}
	cursor, err := r.col.Find(ctx, match, options.Find().
		SetSort(bson.D{{Key: "latency_ms", Value: 1}}).
		SetSkip(skip).
		SetLimit(1).
		SetProjection(bson.M{"latency_ms": 1}),
	)
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)
	var row struct {
		LatencyMS int64 `bson:"latency_ms"`
	}
	if cursor.Next(ctx) {
		if err := cursor.Decode(&row); err != nil {
			return 0, err
		}
	}
	return row.LatencyMS, cursor.Err()
}

func (r *UsageRepository) Activity(ctx context.Context, userID string, limit int64) ([]domain.ActivityRow, error) {
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	cursor, err := r.col.Find(ctx,
		bson.M{"user_id": userID},
		options.Find().
			SetSort(bson.D{{Key: "created_at", Value: -1}}).
			SetLimit(limit),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	rows := make([]domain.ActivityRow, 0, limit)
	for cursor.Next(ctx) {
		var log domain.UsageLog
		if err := cursor.Decode(&log); err != nil {
			return nil, err
		}
		rows = append(rows, domain.ActivityRow{
			ID:        log.ID,
			Time:      log.CreatedAt,
			Model:     log.Model,
			Tokens:    log.TotalTokens,
			LatencyMS: log.LatencyMS,
			Status:    log.Status,
			Endpoint:  log.Endpoint,
		})
	}
	return rows, cursor.Err()
}

func (r *UsageRepository) Sum(ctx context.Context, userID string, from, to time.Time) (domain.UsageAggregate, error) {
	match := bson.M{"created_at": bson.M{"$gte": from, "$lt": to}}
	if userID != "" {
		match["user_id"] = userID
	}
	return r.sum(ctx, match)
}

func (r *UsageRepository) AdminTotals(ctx context.Context, from, to time.Time) (domain.UsageAggregate, error) {
	return r.Sum(ctx, "", from, to)
}

func (r *UsageRepository) sum(ctx context.Context, match bson.M) (domain.UsageAggregate, error) {
	cursor, err := r.col.Aggregate(ctx, mongo.Pipeline{
		{{Key: "$match", Value: match}},
		{{Key: "$group", Value: bson.M{
			"_id":      nil,
			"tokens":   bson.M{"$sum": "$total_tokens"},
			"requests": bson.M{"$sum": 1},
			"input":    bson.M{"$sum": "$prompt_tokens"},
			"output":   bson.M{"$sum": "$completion_tokens"},
		}}},
	})
	if err != nil {
		return domain.UsageAggregate{}, fmt.Errorf("aggregate usage: %w", err)
	}
	defer cursor.Close(ctx)

	var row struct {
		Tokens   int64 `bson:"tokens"`
		Requests int64 `bson:"requests"`
		Input    int64 `bson:"input"`
		Output   int64 `bson:"output"`
	}
	if cursor.Next(ctx) {
		if err := cursor.Decode(&row); err != nil {
			return domain.UsageAggregate{}, err
		}
	}
	return domain.UsageAggregate{
		Tokens:   row.Tokens,
		Requests: row.Requests,
		Input:    row.Input,
		Output:   row.Output,
	}, nil
}
