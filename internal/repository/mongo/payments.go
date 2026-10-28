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

type PaymentRepository struct {
	col *mongo.Collection
}

func NewPaymentRepository(db *mongo.Database) *PaymentRepository {
	return &PaymentRepository{col: db.Collection("payment_orders")}
}

func (r *PaymentRepository) CreateOrder(ctx context.Context, order *domain.PaymentOrder) error {
	_, err := r.col.InsertOne(ctx, order)
	if mongo.IsDuplicateKeyError(err) {
		return domain.ErrDuplicate
	}
	return err
}

func (r *PaymentRepository) FindOrderByID(ctx context.Context, userID, id string) (*domain.PaymentOrder, error) {
	filter := bson.M{"public_id": id}
	if userID != "" {
		filter["user_id"] = userID
	}
	return r.findOneOrder(ctx, filter)
}

func (r *PaymentRepository) FindOrderByPaymentCode(ctx context.Context, code string) (*domain.PaymentOrder, error) {
	return r.findOneOrder(ctx, bson.M{"payment_code": code})
}

func (r *PaymentRepository) findOneOrder(ctx context.Context, filter bson.M) (*domain.PaymentOrder, error) {
	var order domain.PaymentOrder
	err := r.col.FindOne(ctx, filter).Decode(&order)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func (r *PaymentRepository) ListOrdersByUser(ctx context.Context, userID string, limit int64) ([]*domain.PaymentOrder, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
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

	orders := make([]*domain.PaymentOrder, 0, limit)
	for cursor.Next(ctx) {
		var order domain.PaymentOrder
		if err := cursor.Decode(&order); err != nil {
			return nil, err
		}
		orders = append(orders, &order)
	}
	return orders, cursor.Err()
}

func (r *PaymentRepository) ListInvoicesByUser(ctx context.Context, userID string, limit int64) ([]domain.Invoice, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	cursor, err := r.col.Find(ctx,
		bson.M{"user_id": userID, "status": domain.PaymentStatusPaid},
		options.Find().
			SetSort(bson.D{{Key: "paid_at", Value: -1}}).
			SetLimit(limit),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	invoices := make([]domain.Invoice, 0, limit)
	for cursor.Next(ctx) {
		var order domain.PaymentOrder
		if err := cursor.Decode(&order); err != nil {
			return nil, err
		}
		invoices = append(invoices, domain.Invoice{
			ID:            order.ID,
			InvoiceNumber: order.InvoiceNumber,
			PlanID:        order.PlanID,
			AmountVND:     order.AmountVND,
			Status:        order.Status,
			CreatedAt:     order.CreatedAt,
			PaidAt:        order.PaidAt,
		})
	}
	return invoices, cursor.Err()
}

func (r *PaymentRepository) MarkOrderPaid(ctx context.Context, id, providerTransaction string, at time.Time) (bool, error) {
	result, err := r.col.UpdateOne(ctx,
		bson.M{"public_id": id, "status": domain.PaymentStatusPending},
		bson.M{"$set": bson.M{
			"status":               domain.PaymentStatusPaid,
			"provider_transaction": providerTransaction,
			"paid_at":              at,
			"updated_at":           at,
		}},
	)
	if err != nil {
		return false, err
	}
	return result.ModifiedCount > 0, nil
}

func (r *PaymentRepository) UpdateOrderStatus(ctx context.Context, id, status string, at time.Time) error {
	_, err := r.col.UpdateOne(ctx, bson.M{"public_id": id}, bson.M{"$set": bson.M{
		"status":     status,
		"updated_at": at,
	}})
	return err
}

func (r *PaymentRepository) AdminListOrders(ctx context.Context, status string, limit, offset int64) ([]*domain.PaymentOrder, int64, error) {
	filter := bson.M{}
	if status != "" {
		filter["status"] = status
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	total, err := r.col.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	cursor, err := r.col.Find(ctx, filter, options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(limit).
		SetSkip(offset))
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	orders := make([]*domain.PaymentOrder, 0, limit)
	for cursor.Next(ctx) {
		var order domain.PaymentOrder
		if err := cursor.Decode(&order); err != nil {
			return nil, 0, err
		}
		orders = append(orders, &order)
	}
	return orders, total, cursor.Err()
}

func (r *PaymentRepository) Revenue(ctx context.Context, from, to time.Time) (int64, error) {
	cursor, err := r.col.Aggregate(ctx, mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"status":  domain.PaymentStatusPaid,
			"paid_at": bson.M{"$gte": from, "$lt": to},
		}}},
		{{Key: "$group", Value: bson.M{"_id": nil, "total": bson.M{"$sum": "$amount_vnd"}}}},
	})
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)
	var row struct {
		Total int64 `bson:"total"`
	}
	if cursor.Next(ctx) {
		if err := cursor.Decode(&row); err != nil {
			return 0, err
		}
	}
	return row.Total, nil
}
