package seeds

import (
	"context"
	"testing"

	"github.com/eva-bharat/media-sequencer/backend/internal/repository"
)

func TestSeedInitialData(t *testing.T) {
	ctx := context.Background()

	mediaRepo := repository.NewMockMediaRepository()
	windowRepo := repository.NewMockWindowRepository()
	playlistRepo := repository.NewMockPlaylistRepository()

	err := SeedInitialData(ctx, mediaRepo, windowRepo, playlistRepo)
	if err != nil {
		t.Fatalf("unexpected error during seed execution: %v", err)
	}

	// Verify media catalog populated
	mediaList, err := mediaRepo.ListAll(ctx)
	if err != nil {
		t.Fatalf("failed to list media: %v", err)
	}
	if len(mediaList) != len(initialMediaCatalog) {
		t.Errorf("expected %d media items, got %d", len(initialMediaCatalog), len(mediaList))
	}

	// Verify windows populated
	windows, err := windowRepo.ListAll(ctx)
	if err != nil {
		t.Fatalf("failed to list windows: %v", err)
	}
	if len(windows) != 4 {
		t.Errorf("expected 4 windows, got %d", len(windows))
	}

	// Verify window 1 playlist
	p1, err := playlistRepo.FindByWindowNumber(ctx, 1)
	if err != nil {
		t.Fatalf("failed to find playlist 1: %v", err)
	}
	if len(p1.Items) != 3 {
		t.Errorf("expected 3 items in window 1, got %d", len(p1.Items))
	}
	if p1.Items[0].MediaKey != "M1" || p1.Items[1].MediaKey != "M2" || p1.Items[2].MediaKey != "M3" {
		t.Errorf("unexpected items in window 1: %+v", p1.Items)
	}

	// Verify window 2 has configured blank state
	p2, err := playlistRepo.FindByWindowNumber(ctx, 2)
	if err != nil {
		t.Fatalf("failed to find playlist 2: %v", err)
	}
	if p2.Items[2].MediaKey != "BLANK_10" {
		t.Errorf("expected BLANK_10 item in window 2, got %s", p2.Items[2].MediaKey)
	}

	// Verify window 4 playlist
	p4, err := playlistRepo.FindByWindowNumber(ctx, 4)
	if err != nil {
		t.Fatalf("failed to find playlist 4: %v", err)
	}
	if len(p4.Items) != 3 {
		t.Errorf("expected 3 items in window 4, got %d", len(p4.Items))
	}
	if p4.Items[0].MediaKey != "M9" || p4.Items[1].MediaKey != "M10" || p4.Items[2].MediaKey != "M1" {
		t.Errorf("unexpected items in window 4: %+v", p4.Items)
	}
}
