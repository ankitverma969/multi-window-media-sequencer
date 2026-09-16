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

type MongoMediaRepository struct {
	collection *mongo.Collection
}

func NewMongoMediaRepository(db *mongo.Database) *MongoMediaRepository {
	return &MongoMediaRepository{
		collection: db.Collection("media"),
	}
}

func (r *MongoMediaRepository) Create(ctx context.Context, m *models.Media) error {
	now := time.Now().UTC()
	m.CreatedAt = now
	m.UpdatedAt = now
	if m.ID.IsZero() {
		m.ID = bson.NewObjectID()
	}

	_, err := r.collection.InsertOne(ctx, m)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return ErrDuplicateKey
		}
		return fmt.Errorf("failed to insert media: %w", err)
	}
	return nil
}

func (r *MongoMediaRepository) FindByKey(ctx context.Context, mediaKey string) (*models.Media, error) {
	return r.FindByIdOrKey(ctx, mediaKey)
}

func (r *MongoMediaRepository) FindByIdOrKey(ctx context.Context, idOrKey string) (*models.Media, error) {
	filter := bson.M{"media_key": idOrKey}
	if oid, err := bson.ObjectIDFromHex(idOrKey); err == nil {
		filter = bson.M{
			"$or": []bson.M{
				{"_id": oid},
				{"media_key": idOrKey},
			},
		}
	}

	var m models.Media
	err := r.collection.FindOne(ctx, filter).Decode(&m)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to find media %s: %w", idOrKey, err)
	}
	return &m, nil
}

func (r *MongoMediaRepository) ListAll(ctx context.Context) ([]models.Media, error) {
	cursor, err := r.collection.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "media_key", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("failed to list media: %w", err)
	}
	defer cursor.Close(ctx)

	var list []models.Media
	if err := cursor.All(ctx, &list); err != nil {
		return nil, fmt.Errorf("failed to decode media list: %w", err)
	}
	if list == nil {
		list = []models.Media{}
	}
	return list, nil
}

func (r *MongoMediaRepository) Upsert(ctx context.Context, m *models.Media) error {
	now := time.Now().UTC()
	m.UpdatedAt = now
	if m.CreatedAt.IsZero() {
		m.CreatedAt = now
	}
	if m.ID.IsZero() {
		m.ID = bson.NewObjectID()
	}

	filter := bson.M{"media_key": m.MediaKey}
	update := bson.M{"$set": m}
	opts := options.UpdateOne().SetUpsert(true)

	_, err := r.collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to upsert media %s: %w", m.MediaKey, err)
	}
	return nil
}
