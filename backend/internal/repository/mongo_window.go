package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/eva-bharat/media-sequencer/backend/internal/models"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type MongoWindowRepository struct {
	collection *mongo.Collection
}

func NewMongoWindowRepository(db *mongo.Database) *MongoWindowRepository {
	return &MongoWindowRepository{
		collection: db.Collection("windows"),
	}
}

func (r *MongoWindowRepository) Create(ctx context.Context, w *models.Window) error {
	now := time.Now().UTC()
	w.CreatedAt = now
	w.UpdatedAt = now
	if w.ID.IsZero() {
		w.ID = bson.NewObjectID()
	}

	_, err := r.collection.InsertOne(ctx, w)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return ErrDuplicateKey
		}
		return fmt.Errorf("failed to insert window: %w", err)
	}
	return nil
}

func (r *MongoWindowRepository) FindByNumber(ctx context.Context, windowNumber int) (*models.Window, error) {
	var w models.Window
	err := r.collection.FindOne(ctx, bson.M{"window_number": windowNumber}).Decode(&w)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to find window %d: %w", windowNumber, err)
	}
	return &w, nil
}

func (r *MongoWindowRepository) FindByIdOrNumber(ctx context.Context, idOrNumber string) (*models.Window, error) {
	var filter bson.M
	if num, err := strconv.Atoi(idOrNumber); err == nil && num > 0 {
		filter = bson.M{"window_number": num}
	} else if oid, err := bson.ObjectIDFromHex(idOrNumber); err == nil {
		filter = bson.M{"_id": oid}
	} else {
		return nil, ErrNotFound
	}

	var w models.Window
	err := r.collection.FindOne(ctx, filter).Decode(&w)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to find window %s: %w", idOrNumber, err)
	}
	return &w, nil
}

func (r *MongoWindowRepository) ListAll(ctx context.Context) ([]models.Window, error) {
	cursor, err := r.collection.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "window_number", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("failed to list windows: %w", err)
	}
	defer cursor.Close(ctx)

	var list []models.Window
	if err := cursor.All(ctx, &list); err != nil {
		return nil, fmt.Errorf("failed to decode windows list: %w", err)
	}
	if list == nil {
		list = []models.Window{}
	}
	return list, nil
}

func (r *MongoWindowRepository) Upsert(ctx context.Context, w *models.Window) error {
	now := time.Now().UTC()
	w.UpdatedAt = now
	if w.CreatedAt.IsZero() {
		w.CreatedAt = now
	}
	if w.ID.IsZero() {
		w.ID = bson.NewObjectID()
	}

	filter := bson.M{"window_number": w.WindowNumber}
	update := bson.M{
		"$set": bson.M{
			"name":                   w.Name,
			"cycle_duration_seconds": w.CycleDurationSeconds,
			"cycle_start_time":       w.CycleStartTime,
			"is_active":              w.IsActive,
			"updated_at":             w.UpdatedAt,
		},
		"$setOnInsert": bson.M{
			"_id":           w.ID,
			"window_number": w.WindowNumber,
			"created_at":    w.CreatedAt,
		},
	}
	opts := options.UpdateOne().SetUpsert(true)

	_, err := r.collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to upsert window %d: %w", w.WindowNumber, err)
	}
	return nil
}
