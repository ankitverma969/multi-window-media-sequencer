package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/eva-bharat/media-sequencer/backend/internal/models"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type MongoSyncRepository struct {
	collection *mongo.Collection
}

func NewMongoSyncRepository(db *mongo.Database) *MongoSyncRepository {
	return &MongoSyncRepository{
		collection: db.Collection("sync_events"),
	}
}

func (r *MongoSyncRepository) Create(ctx context.Context, s *models.SyncEvent) error {
	now := time.Now().UTC()
	s.CreatedAt = now
	if s.ID.IsZero() {
		s.ID = bson.NewObjectID()
	}

	_, err := r.collection.InsertOne(ctx, s)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return ErrDuplicateKey
		}
		return fmt.Errorf("failed to insert sync event: %w", err)
	}
	return nil
}

func (r *MongoSyncRepository) FindActive(ctx context.Context, now time.Time) (*models.SyncEvent, error) {
	// Active or scheduled: status in [SCHEDULED, ACTIVE] and end_time > now
	filter := bson.M{
		"status":   bson.M{"$in": []models.SyncStatus{models.SyncStatusScheduled, models.SyncStatusActive}},
		"end_time": bson.M{"$gt": now},
	}
	opts := options.FindOne().SetSort(bson.D{{Key: "start_time", Value: -1}})

	var event models.SyncEvent
	err := r.collection.FindOne(ctx, filter, opts).Decode(&event)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil // No active sync event currently
		}
		return nil, fmt.Errorf("failed to find active sync event: %w", err)
	}

	return &event, nil
}

func (r *MongoSyncRepository) FindByID(ctx context.Context, eventID string) (*models.SyncEvent, error) {
	filter := bson.M{"event_id": eventID}
	var event models.SyncEvent
	err := r.collection.FindOne(ctx, filter).Decode(&event)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to find sync event %s: %w", eventID, err)
	}
	return &event, nil
}

func (r *MongoSyncRepository) UpdateStatus(ctx context.Context, eventID string, status models.SyncStatus) error {
	filter := bson.M{"event_id": eventID}
	update := bson.M{"$set": bson.M{"status": status}}

	res, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update sync event status for %s: %w", eventID, err)
	}
	if res.MatchedCount == 0 {
		return ErrNotFound
	}
	return nil
}
