package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/eva-bharat/media-sequencer/backend/internal/config"
	"github.com/eva-bharat/media-sequencer/backend/internal/database"
	"github.com/eva-bharat/media-sequencer/backend/internal/handlers"
	"github.com/eva-bharat/media-sequencer/backend/internal/repository"
	"github.com/eva-bharat/media-sequencer/backend/internal/service"
	"github.com/eva-bharat/media-sequencer/backend/seeds"
)

func main() {
	// 1. Initialize structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("starting Media Sequencer Backend service...")

	// 2. Load and validate configuration
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	// 3. Connect to persistent MongoDB storage
	initCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	db, err := database.Connect(initCtx, cfg)
	if err != nil {
		slog.Error("CRITICAL: persistent storage initialization failed", "error", err)
		os.Exit(1)
	}

	// 4. Ensure database indexes
	if err := database.EnsureIndexes(initCtx, db.Database); err != nil {
		slog.Error("CRITICAL: failed to ensure database indexes", "error", err)
		_ = db.Close(context.Background())
		os.Exit(1)
	}

	// 5. Initialize Repositories
	mediaRepo := repository.NewMongoMediaRepository(db.Database)
	windowRepo := repository.NewMongoWindowRepository(db.Database)
	playlistRepo := repository.NewMongoPlaylistRepository(db.Database)
	syncRepo := repository.NewMongoSyncRepository(db.Database)

	// 6. Seed initial development data if configured
	if cfg.SeedOnStartup {
		if err := seeds.SeedInitialData(initCtx, mediaRepo, windowRepo, playlistRepo); err != nil {
			slog.Warn("warning: seed data population encountered error", "error", err)
		}
	}

	// 7. Initialize Services
	mediaService := service.NewMediaService(mediaRepo)
	windowService := service.NewWindowService(windowRepo)
	playlistService := service.NewPlaylistService(playlistRepo, mediaRepo, windowRepo)
	syncService := service.NewSyncService(syncRepo, mediaRepo)

	// 8. Build HTTP Router & Middleware Stack
	router := handlers.NewRouter(handlers.Dependencies{
		Config:          cfg,
		Pinger:          db,
		WindowService:   windowService,
		MediaService:    mediaService,
		PlaylistService: playlistService,
		SyncService:     syncService,
	})

	// 9. Configure HTTP Server
	serverAddr := fmt.Sprintf(":%s", cfg.Port)
	server := &http.Server{
		Addr:         serverAddr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 10. Start HTTP server in a goroutine
	serverErrors := make(chan error, 1)
	go func() {
		slog.Info("HTTP server listening", "addr", serverAddr, "env", cfg.Environment)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	// 11. Listen for termination signals (Graceful Shutdown)
	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	select {
	case err := <-serverErrors:
		slog.Error("server failed to start", "error", err)
		_ = db.Close(context.Background())
		os.Exit(1)

	case sig := <-shutdownSignal:
		slog.Info("shutdown signal received", "signal", sig.String())

		// Graceful shutdown context with 10s deadline
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		// 1. Stop accepting new requests & drain active connections
		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("HTTP server graceful shutdown error", "error", err)
			_ = server.Close()
		} else {
			slog.Info("HTTP server stopped cleanly")
		}

		// 2. Disconnect MongoDB client pool
		if err := db.Close(shutdownCtx); err != nil {
			slog.Error("MongoDB disconnect error", "error", err)
		} else {
			slog.Info("MongoDB connection closed cleanly")
		}

		slog.Info("service shutdown complete")
	}
}
