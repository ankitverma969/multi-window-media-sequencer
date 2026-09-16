package seeds

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/eva-bharat/media-sequencer/backend/internal/models"
	"github.com/eva-bharat/media-sequencer/backend/internal/repository"
	"github.com/google/uuid"
)

// NOTE ON SEED DATA:
// The assignment specification mentions "Seed data matching the example windows and media lists"
// and references "for example M2". It does not provide full media URLs or file assets in the text.
// Therefore, the following dataset represents CANDIDATE-CREATED DEVELOPMENT SEED DATA.
// It uses stable public domain test assets (W3C video streams & Unsplash test imagery)
// and explicitly demonstrates multi-window independent playback, images, videos, and a configured blank state.

var initialMediaCatalog = []models.Media{
	{
		MediaKey:        "M1",
		Name:            "Big Buck Bunny Sample (M1)",
		Type:            models.MediaTypeVideo,
		URL:             "https://commondatastorage.googleapis.com/gtv-videos-bucket/sample/BigBuckBunny.mp4",
		DurationSeconds: 15,
	},
	{
		MediaKey:        "M2",
		Name:            "Synchronized Promo Banner (M2)",
		Type:            models.MediaTypeImage,
		URL:             "https://images.unsplash.com/photo-1579546929518-9e396f3cc809?w=1200",
		DurationSeconds: 10,
	},
	{
		MediaKey:        "M3",
		Name:            "Elephants Dream Sample (M3)",
		Type:            models.MediaTypeVideo,
		URL:             "https://commondatastorage.googleapis.com/gtv-videos-bucket/sample/ElephantsDream.mp4",
		DurationSeconds: 20,
	},
	{
		MediaKey:        "M4",
		Name:            "For Bigger Blazes (M4)",
		Type:            models.MediaTypeVideo,
		URL:             "https://commondatastorage.googleapis.com/gtv-videos-bucket/sample/ForBiggerBlazes.mp4",
		DurationSeconds: 12,
	},
	{
		MediaKey:        "M5",
		Name:            "Product Showcase Display (M5)",
		Type:            models.MediaTypeImage,
		URL:             "https://images.unsplash.com/photo-1507525428034-b723cf961d3e?w=1200",
		DurationSeconds: 8,
	},
	{
		MediaKey:        "M6",
		Name:            "City Night Horizon (M6)",
		Type:            models.MediaTypeImage,
		URL:             "https://images.unsplash.com/photo-1519501025264-65ba15a82390?w=1200",
		DurationSeconds: 14,
	},
	{
		MediaKey:        "M7",
		Name:            "For Bigger Escapes (M7)",
		Type:            models.MediaTypeVideo,
		URL:             "https://commondatastorage.googleapis.com/gtv-videos-bucket/sample/ForBiggerEscapes.mp4",
		DurationSeconds: 18,
	},
	{
		MediaKey:        "BLANK_10",
		Name:            "Configured 10s Intermission Blank",
		Type:            models.MediaTypeBlank,
		URL:             "",
		DurationSeconds: 10,
	},
}

// SeedInitialData populates MongoDB with candidate-created baseline data if empty.
func SeedInitialData(
	ctx context.Context,
	mediaRepo repository.MediaRepository,
	windowRepo repository.WindowRepository,
	playlistRepo repository.PlaylistRepository,
) error {
	slog.Info("checking database seed state...")

	// 1. Seed Media Catalog
	for _, m := range initialMediaCatalog {
		if err := mediaRepo.Upsert(ctx, &m); err != nil {
			return fmt.Errorf("failed to upsert seed media %s: %w", m.MediaKey, err)
		}
	}

	// 2. Seed 3 Windows
	baseMidnight := time.Now().UTC().Truncate(24 * time.Hour)
	windowConfigs := []struct {
		number int
		name   string
		items  []string // media keys
	}{
		{
			number: 1,
			name:   "Display Window 1 (Front)",
			items:  []string{"M1", "M2", "M3"},
		},
		{
			number: 2,
			name:   "Display Window 2 (Side)",
			items:  []string{"M4", "M5", "BLANK_10"},
		},
		{
			number: 3,
			name:   "Display Window 3 (Lobby)",
			items:  []string{"M1", "M6", "M7"},
		},
	}

	for _, wc := range windowConfigs {
		window := &models.Window{
			WindowNumber:         wc.number,
			Name:                 wc.name,
			CycleDurationSeconds: 18000, // 5 hours in seconds
			CycleStartTime:       baseMidnight,
			IsActive:             true,
		}
		if err := windowRepo.Upsert(ctx, window); err != nil {
			return fmt.Errorf("failed to upsert seed window %d: %w", wc.number, err)
		}

		// Build playlist items
		var playlistItems []models.PlaylistItem
		for _, key := range wc.items {
			media, err := mediaRepo.FindByKey(ctx, key)
			if err != nil {
				return fmt.Errorf("media %s missing during playlist seed: %w", key, err)
			}

			playlistItems = append(playlistItems, models.PlaylistItem{
				ItemID:          uuid.New().String(),
				MediaKey:        media.MediaKey,
				Type:            media.Type,
				URL:             media.URL,
				DurationSeconds: media.DurationSeconds,
			})
		}

		playlist := &models.Playlist{
			WindowID:     window.ID,
			WindowNumber: window.WindowNumber,
			Items:        playlistItems,
			Version:      1,
		}
		playlist.Recalculate()

		if err := playlistRepo.Upsert(ctx, playlist); err != nil {
			return fmt.Errorf("failed to upsert seed playlist for window %d: %w", wc.number, err)
		}
	}

	slog.Info("development seed data successfully populated",
		"media_count", len(initialMediaCatalog),
		"window_count", len(windowConfigs),
	)
	return nil
}
