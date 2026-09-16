package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/eva-bharat/media-sequencer/backend/internal/config"
	"github.com/eva-bharat/media-sequencer/backend/internal/database"
	"github.com/eva-bharat/media-sequencer/backend/internal/repository"
	"github.com/eva-bharat/media-sequencer/backend/seeds"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("running standalone database seeder for EVA Bharat Media Sequencer...")

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	db, err := database.Connect(ctx, cfg)
	if err != nil {
		slog.Error("failed to connect to MongoDB", "error", err)
		os.Exit(1)
	}
	defer func() {
		_ = db.Close(context.Background())
	}()

	if err := database.EnsureIndexes(ctx, db.Database); err != nil {
		slog.Error("failed to ensure indexes", "error", err)
		os.Exit(1)
	}

	mediaRepo := repository.NewMongoMediaRepository(db.Database)
	windowRepo := repository.NewMongoWindowRepository(db.Database)
	playlistRepo := repository.NewMongoPlaylistRepository(db.Database)

	if err := seeds.SeedInitialData(ctx, mediaRepo, windowRepo, playlistRepo); err != nil {
		slog.Error("failed to seed database", "error", err)
		os.Exit(1)
	}

	slog.Info("database successfully seeded with development example windows, media, and playlists")
}
