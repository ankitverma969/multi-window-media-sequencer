package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// EnsureIndexes creates all required indexes across collections if they don't already exist.
func EnsureIndexes(ctx context.Context, db *mongo.Database) error {
	idxCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// 1. Media collection indexes:
	// - media_key: unique (for fast lookup and preventing duplicate key names)
	// - type: regular index (for filtering media by type)
	mediaIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "media_key", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("idx_media_key_unique"),
		},
		{
			Keys:    bson.D{{Key: "type", Value: 1}},
			Options: options.Index().SetName("idx_media_type"),
		},
	}
	if _, err := db.Collection("media").Indexes().CreateMany(idxCtx, mediaIndexes); err != nil {
		return fmt.Errorf("failed to create media indexes: %w", err)
	}

	// 2. Windows collection indexes:
	// - window_number: unique (ensures display windows 1, 2, 3 have stable identity)
	windowIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "window_number", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("idx_window_number_unique"),
		},
	}
	if _, err := db.Collection("windows").Indexes().CreateMany(idxCtx, windowIndexes); err != nil {
		return fmt.Errorf("failed to create windows indexes: %w", err)
	}

	// 3. Playlists collection indexes:
	// - window_number: unique (ensures one authoritative playlist document per window)
	// - window_id: unique
	playlistIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "window_number", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("idx_playlist_window_number_unique"),
		},
		{
			Keys:    bson.D{{Key: "window_id", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("idx_playlist_window_id_unique"),
		},
	}
	if _, err := db.Collection("playlists").Indexes().CreateMany(idxCtx, playlistIndexes); err != nil {
		return fmt.Errorf("failed to create playlists indexes: %w", err)
	}

	// 4. Sync events collection indexes:
	// - event_id: unique
	// - status + end_time: compound index (for fast retrieval of active/scheduled syncs)
	// - start_time: descending index (for historical querying)
	syncIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "event_id", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("idx_sync_event_id_unique"),
		},
		{
			Keys:    bson.D{{Key: "status", Value: 1}, {Key: "end_time", Value: -1}},
			Options: options.Index().SetName("idx_sync_status_end_time"),
		},
		{
			Keys:    bson.D{{Key: "start_time", Value: -1}},
			Options: options.Index().SetName("idx_sync_start_time"),
		},
	}
	if _, err := db.Collection("sync_events").Indexes().CreateMany(idxCtx, syncIndexes); err != nil {
		return fmt.Errorf("failed to create sync_events indexes: %w", err)
	}

	slog.Info("MongoDB indexes ensured successfully across all collections")
	return nil
}
