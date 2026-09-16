package repository

import (
	"context"
	"testing"

	"github.com/eva-bharat/media-sequencer/backend/internal/models"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestMockMediaRepository(t *testing.T) {
	ctx := context.Background()
	repo := NewMockMediaRepository()

	m1 := &models.Media{
		MediaKey:        "M1",
		Name:            "Nature 4K",
		Type:            models.MediaTypeVideo,
		URL:             "https://example.com/m1.mp4",
		DurationSeconds: 30,
	}

	if err := repo.Create(ctx, m1); err != nil {
		t.Fatalf("failed to create media: %v", err)
	}

	// Test duplicate key
	if err := repo.Create(ctx, m1); err != ErrDuplicateKey {
		t.Errorf("expected ErrDuplicateKey, got %v", err)
	}

	// Test FindByKey
	found, err := repo.FindByKey(ctx, "M1")
	if err != nil {
		t.Fatalf("failed to find media: %v", err)
	}
	if found.Name != "Nature 4K" {
		t.Errorf("expected 'Nature 4K', got '%s'", found.Name)
	}

	// Test FindByKey not found
	_, err = repo.FindByKey(ctx, "UNKNOWN")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestMockPlaylistRepository(t *testing.T) {
	ctx := context.Background()
	repo := NewMockPlaylistRepository()

	p := &models.Playlist{
		WindowNumber: 1,
		Items: []models.PlaylistItem{
			{ItemID: "it-1", MediaKey: "M1", Type: models.MediaTypeVideo, DurationSeconds: 30},
		},
	}
	if err := repo.Upsert(ctx, p); err != nil {
		t.Fatalf("failed to upsert playlist: %v", err)
	}

	// Append item
	newItem := models.PlaylistItem{
		ItemID:          "it-2",
		MediaKey:        "M2",
		Type:            models.MediaTypeImage,
		DurationSeconds: 15,
	}
	updated, err := repo.AppendItem(ctx, 1, bson.ObjectID{}, newItem)
	if err != nil {
		t.Fatalf("failed to append item: %v", err)
	}
	if len(updated.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(updated.Items))
	}
	if updated.TotalSequenceDurationSeconds != 45 {
		t.Errorf("expected total duration 45, got %d", updated.TotalSequenceDurationSeconds)
	}

	// Remove item
	removed, err := repo.RemoveItem(ctx, 1, bson.ObjectID{}, "it-1")
	if err != nil {
		t.Fatalf("failed to remove item: %v", err)
	}
	if len(removed.Items) != 1 {
		t.Errorf("expected 1 item remaining, got %d", len(removed.Items))
	}
	if removed.Items[0].ItemID != "it-2" {
		t.Errorf("expected remaining item it-2, got %s", removed.Items[0].ItemID)
	}
}
