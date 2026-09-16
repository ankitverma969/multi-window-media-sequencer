package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/eva-bharat/media-sequencer/backend/internal/config"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// DB encapsulates the active MongoDB client and target database.
type DB struct {
	Client   *mongo.Client
	Database *mongo.Database
}

// Connect initializes a persistent connection to MongoDB using the official driver,
// validates connectivity with a ping, and returns a managed DB instance.
func Connect(ctx context.Context, cfg *config.Config) (*DB, error) {
	slog.Info("connecting to MongoDB...", "uri_configured", cfg.MongoURI != "", "database", cfg.MongoDBName)

	clientOpts := options.Client().
		ApplyURI(cfg.MongoURI).
		SetConnectTimeout(10 * time.Second).
		SetServerSelectionTimeout(5 * time.Second)

	client, err := mongo.Connect(clientOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize MongoDB client: %w", err)
	}

	// Verify connectivity with a ping healthcheck
	pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
	defer pingCancel()

	if err := client.Ping(pingCtx, nil); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, fmt.Errorf("failed to ping MongoDB at %s: %w", cfg.MongoURI, err)
	}

	slog.Info("successfully connected to MongoDB", "database", cfg.MongoDBName)
	return &DB{
		Client:   client,
		Database: client.Database(cfg.MongoDBName),
	}, nil
}

// Ping checks if the MongoDB connection is alive.
func (db *DB) Ping(ctx context.Context) error {
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return db.Client.Ping(pingCtx, nil)
}

// Close gracefully closes the MongoDB connection pool.
func (db *DB) Close(ctx context.Context) error {
	if db.Client == nil {
		return nil
	}
	slog.Info("closing MongoDB client connection...")
	return db.Client.Disconnect(ctx)
}
