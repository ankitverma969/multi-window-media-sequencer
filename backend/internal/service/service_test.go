package service

import (
	"context"
	"testing"
	"time"

	"github.com/eva-bharat/media-sequencer/backend/internal/models"
	"github.com/eva-bharat/media-sequencer/backend/internal/repository"
)

func TestPlaylistServiceAddPlaylistItem(t *testing.T) {
	ctx := context.Background()

	mediaRepo := repository.NewMockMediaRepository()
	windowRepo := repository.NewMockWindowRepository()
	playlistRepo := repository.NewMockPlaylistRepository()

	// Seed window 1
	w := &models.Window{
		WindowNumber:         1,
		Name:                 "Window 1",
		CycleDurationSeconds: 18000,
		CycleStartTime:       time.Now().UTC(),
		IsActive:             true,
	}
	_ = windowRepo.Create(ctx, w)

	// Seed playlist for window 1
	p := &models.Playlist{
		WindowNumber: 1,
		Items:        []models.PlaylistItem{},
	}
	_ = playlistRepo.Upsert(ctx, p)

	// Seed media M1
	m1 := &models.Media{
		MediaKey:        "M1",
		Name:            "Test Video",
		Type:            models.MediaTypeVideo,
		URL:             "https://example.com/m1.mp4",
		DurationSeconds: 30,
	}
	_ = mediaRepo.Create(ctx, m1)

	svc := NewPlaylistService(playlistRepo, mediaRepo, windowRepo)

	// 1. Test adding existing media
	updated, err := svc.AddPlaylistItem(ctx, "1", "M1", 0)
	if err != nil {
		t.Fatalf("unexpected error adding item: %v", err)
	}

	if len(updated.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(updated.Items))
	}
	if updated.Items[0].MediaKey != "M1" {
		t.Errorf("expected media key M1, got %s", updated.Items[0].MediaKey)
	}
	if updated.Items[0].DurationSeconds != 30 {
		t.Errorf("expected duration 30, got %d", updated.Items[0].DurationSeconds)
	}
	if updated.TotalSequenceDurationSeconds != 30 {
		t.Errorf("expected total duration 30, got %d", updated.TotalSequenceDurationSeconds)
	}

	// 2. Test adding non-existent media
	_, err = svc.AddPlaylistItem(ctx, "1", "NON_EXISTENT", 0)
	if err != ErrMediaNotFound {
		t.Errorf("expected ErrMediaNotFound, got %v", err)
	}

	// 3. Test adding to non-existent window
	_, err = svc.AddPlaylistItem(ctx, "99", "M1", 0)
	if err != ErrWindowNotFound {
		t.Errorf("expected ErrWindowNotFound, got %v", err)
	}
}

func TestMediaServiceValidation(t *testing.T) {
	ctx := context.Background()
	mediaRepo := repository.NewMockMediaRepository()
	svc := NewMediaService(mediaRepo)

	invalidMedia := &models.Media{
		MediaKey:        "",
		Name:            "Invalid",
		Type:            models.MediaTypeVideo,
		DurationSeconds: 10,
	}

	err := svc.CreateMedia(ctx, invalidMedia)
	if err == nil {
		t.Errorf("expected validation error for empty media key")
	}
}

func TestSyncService(t *testing.T) {
	ctx := context.Background()
	mediaRepo := repository.NewMockMediaRepository()
	syncRepo := repository.NewMockSyncRepository()

	// Seed media M2
	_ = mediaRepo.Create(ctx, &models.Media{
		MediaKey:        "M2",
		Name:            "Sync Promo",
		Type:            models.MediaTypeImage,
		URL:             "https://example.com/m2.jpg",
		DurationSeconds: 15,
	})

	syncSvc := NewSyncService(syncRepo, mediaRepo)

	// 1. Trigger valid sync
	event, err := syncSvc.TriggerSync(ctx, "M2", 15, 1000, "admin")
	if err != nil {
		t.Fatalf("unexpected error triggering sync: %v", err)
	}
	if event.MediaKey != "M2" || event.DurationSeconds != 15 {
		t.Errorf("unexpected event: %+v", event)
	}

	// 2. Query active sync
	active, err := syncSvc.GetActiveSync(ctx)
	if err != nil {
		t.Fatalf("unexpected error getting active sync: %v", err)
	}
	if active == nil || active.EventID != event.EventID {
		t.Errorf("expected active event %s, got %+v", event.EventID, active)
	}

	// 3. Trigger invalid duration
	_, err = syncSvc.TriggerSync(ctx, "M2", 0, 1000, "admin")
	if err == nil {
		t.Errorf("expected error for 0 duration")
	}

	// 4. Trigger non-existent media
	_, err = syncSvc.TriggerSync(ctx, "NON_EXISTENT", 15, 1000, "admin")
	if err != ErrMediaNotFound {
		t.Errorf("expected ErrMediaNotFound, got %v", err)
	}

	// 5. Cancel sync
	if err := syncSvc.CancelSync(ctx, event.EventID); err != nil {
		t.Fatalf("failed to cancel sync: %v", err)
	}
}
