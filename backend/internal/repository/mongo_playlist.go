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

type MongoPlaylistRepository struct {
	collection *mongo.Collection
}

func NewMongoPlaylistRepository(db *mongo.Database) *MongoPlaylistRepository {
	return &MongoPlaylistRepository{
		collection: db.Collection("playlists"),
	}
}

func (r *MongoPlaylistRepository) FindByWindowNumber(ctx context.Context, windowNumber int) (*models.Playlist, error) {
	var p models.Playlist
	err := r.collection.FindOne(ctx, bson.M{"window_number": windowNumber}).Decode(&p)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to find playlist for window %d: %w", windowNumber, err)
	}
	return &p, nil
}

func (r *MongoPlaylistRepository) AppendItem(ctx context.Context, windowNumber int, item models.PlaylistItem) (*models.Playlist, error) {
	// Retrieve existing playlist to determine sequence order and new total
	p, err := r.FindByWindowNumber(ctx, windowNumber)
	if err != nil {
		return nil, err
	}

	item.Order = len(p.Items) + 1
	p.Items = append(p.Items, item)
	p.Recalculate()

	filter := bson.M{"window_number": windowNumber}
	update := bson.M{
		"$set": bson.M{
			"items":                           p.Items,
			"total_sequence_duration_seconds": p.TotalSequenceDurationSeconds,
			"version":                         p.Version,
			"updated_at":                      p.UpdatedAt,
		},
	}

	_, err = r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return nil, fmt.Errorf("failed to append item to playlist for window %d: %w", windowNumber, err)
	}

	return p, nil
}

func (r *MongoPlaylistRepository) UpdateItems(ctx context.Context, windowNumber int, items []models.PlaylistItem) (*models.Playlist, error) {
	p, err := r.FindByWindowNumber(ctx, windowNumber)
	if err != nil {
		return nil, err
	}

	p.Items = items
	p.Recalculate()

	filter := bson.M{"window_number": windowNumber}
	update := bson.M{
		"$set": bson.M{
			"items":                           p.Items,
			"total_sequence_duration_seconds": p.TotalSequenceDurationSeconds,
			"version":                         p.Version,
			"updated_at":                      p.UpdatedAt,
		},
	}

	_, err = r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return nil, fmt.Errorf("failed to update items for window %d: %w", windowNumber, err)
	}

	return p, nil
}

func (r *MongoPlaylistRepository) RemoveItem(ctx context.Context, windowNumber int, itemID string) (*models.Playlist, error) {
	p, err := r.FindByWindowNumber(ctx, windowNumber)
	if err != nil {
		return nil, err
	}

	found := false
	var updatedItems []models.PlaylistItem
	for _, it := range p.Items {
		if it.ItemID == itemID {
			found = true
			continue
		}
		updatedItems = append(updatedItems, it)
	}

	if !found {
		return nil, ErrNotFound
	}

	p.Items = updatedItems
	p.Recalculate()

	filter := bson.M{"window_number": windowNumber}
	update := bson.M{
		"$set": bson.M{
			"items":                           p.Items,
			"total_sequence_duration_seconds": p.TotalSequenceDurationSeconds,
			"version":                         p.Version,
			"updated_at":                      p.UpdatedAt,
		},
	}

	_, err = r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return nil, fmt.Errorf("failed to remove item from playlist for window %d: %w", windowNumber, err)
	}

	return p, nil
}

func (r *MongoPlaylistRepository) Upsert(ctx context.Context, p *models.Playlist) error {
	p.UpdatedAt = time.Now().UTC()
	if p.ID.IsZero() {
		p.ID = bson.NewObjectID()
	}
	p.Recalculate()

	filter := bson.M{"window_number": p.WindowNumber}
	update := bson.M{"$set": p}
	opts := options.UpdateOne().SetUpsert(true)

	_, err := r.collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to upsert playlist for window %d: %w", p.WindowNumber, err)
	}
	return nil
}
