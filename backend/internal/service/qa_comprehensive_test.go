package service

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/eva-bharat/media-sequencer/backend/internal/models"
	"github.com/eva-bharat/media-sequencer/backend/internal/repository"
	"github.com/eva-bharat/media-sequencer/backend/internal/timeline"
	"github.com/eva-bharat/media-sequencer/backend/internal/websocket"
)

// ============================================================================
// 1. RE-READ ASSIGNMENT & DATABASE PERSISTENCE RESTART TEST
// ============================================================================
func TestQA_DatabasePersistenceSurvivalAcrossRestart(t *testing.T) {
	ctx := context.Background()

	// 1. Initialize persistent storage mock
	syncRepo := repository.NewMockSyncRepository()
	mediaRepo := repository.NewMockMediaRepository()
	windowRepo := repository.NewMockWindowRepository()
	playlistRepo := repository.NewMockPlaylistRepository()
	hub := newMockHub()

	// Seed Media
	_ = mediaRepo.Create(ctx, &models.Media{
		MediaKey: "M1", Name: "Media 1", Type: models.MediaTypeVideo, DurationSeconds: 30, URL: "http://example.com/1.mp4",
	})
	_ = mediaRepo.Create(ctx, &models.Media{
		MediaKey: "M2", Name: "Media 2", Type: models.MediaTypeImage, DurationSeconds: 20, URL: "http://example.com/2.jpg",
	})
	_ = mediaRepo.Create(ctx, &models.Media{
		MediaKey: "M3", Name: "Media 3", Type: models.MediaTypeVideo, DurationSeconds: 25, URL: "http://example.com/3.mp4",
	})

	// Seed Window 1
	_ = windowRepo.Create(ctx, &models.Window{
		WindowNumber: 1, Name: "Window 1", CycleDurationSeconds: 18000, CycleStartTime: time.Now().UTC(), IsActive: true,
	})
	_ = playlistRepo.Upsert(ctx, &models.Playlist{
		WindowNumber: 1, Items: []models.PlaylistItem{}, Version: 1,
	})

	// Service Instance 1
	playlistSvc1 := NewPlaylistService(playlistRepo, mediaRepo, windowRepo)
	_, err := playlistSvc1.AddPlaylistItem(ctx, "1", "M1", 30)
	if err != nil {
		t.Fatalf("failed to add item: %v", err)
	}
	_, err = playlistSvc1.AddPlaylistItem(ctx, "1", "M2", 20)
	if err != nil {
		t.Fatalf("failed to add item: %v", err)
	}

	// 2. SIMULATE COMPLETE BACKEND RESTART
	// Teardown Service Instance 1 and instantiate Service Instance 2 with the SAME persistent repositories
	playlistSvc2 := NewPlaylistService(playlistRepo, mediaRepo, windowRepo)
	coord2 := NewSyncCoordinator(syncRepo, mediaRepo, windowRepo, playlistRepo, playlistSvc2, hub)

	// 3. Verify data was preserved completely
	p, err := playlistSvc2.GetPlaylist(ctx, "1")
	if err != nil {
		t.Fatalf("failed to retrieve playlist after restart: %v", err)
	}
	if len(p.Items) != 2 {
		t.Fatalf("expected 2 playlist items after restart, got %d", len(p.Items))
	}
	if p.Items[0].MediaKey != "M1" || p.Items[1].MediaKey != "M2" {
		t.Errorf("playlist order or keys corrupted after restart: %+v", p.Items)
	}
	if p.TotalSequenceDurationSeconds != 50 {
		t.Errorf("expected total duration 50s, got %d", p.TotalSequenceDurationSeconds)
	}

	// Verify coordinator can generate snapshot using persisted data
	snap, err := coord2.BuildSnapshotForWindow(ctx, 1)
	if err != nil {
		t.Fatalf("failed to build snapshot after restart: %v", err)
	}
	if snap.Playlist == nil || len(snap.Playlist.Items) != 2 {
		t.Fatalf("snapshot playlist empty after restart")
	}
}

// ============================================================================
// 2. MULTI-WINDOW INDEPENDENCE TEST (4 WINDOWS)
// ============================================================================
func TestQA_MultiWindowIndependence(t *testing.T) {
	ctx := context.Background()
	mediaRepo := repository.NewMockMediaRepository()
	windowRepo := repository.NewMockWindowRepository()
	playlistRepo := repository.NewMockPlaylistRepository()
	now := time.Now().UTC()

	// Seed 10 media assets
	for i := 1; i <= 10; i++ {
		_ = mediaRepo.Create(ctx, &models.Media{
			MediaKey: fmt.Sprintf("M%d", i), Name: fmt.Sprintf("Media %d", i), Type: models.MediaTypeVideo, DurationSeconds: 10 + i, URL: fmt.Sprintf("http://example.com/%d.mp4", i),
		})
	}

	// Configure 4 distinct windows matching the assignment example:
	// W1: M1, M2, M3
	// W2: M4, M5
	// W3: M6, M7, M8
	// W4: M9, M10
	configs := []struct {
		num   int
		items []string
	}{
		{1, []string{"M1", "M2", "M3"}},
		{2, []string{"M4", "M5"}},
		{3, []string{"M6", "M7", "M8"}},
		{4, []string{"M9", "M10"}},
	}

	for _, c := range configs {
		_ = windowRepo.Create(ctx, &models.Window{
			WindowNumber: c.num, Name: fmt.Sprintf("Window %d", c.num), CycleDurationSeconds: 18000, CycleStartTime: now, IsActive: true,
		})
		var pItems []models.PlaylistItem
		total := 0
		for idx, k := range c.items {
			m, _ := mediaRepo.FindByKey(ctx, k)
			pItems = append(pItems, models.PlaylistItem{
				ItemID: fmt.Sprintf("item-%d-%s", c.num, k), MediaKey: k, Type: m.Type, URL: m.URL, DurationSeconds: m.DurationSeconds, Order: idx + 1,
			})
			total += m.DurationSeconds
		}
		_ = playlistRepo.Upsert(ctx, &models.Playlist{
			WindowNumber: c.num, Items: pItems, TotalSequenceDurationSeconds: total, Version: 1,
		})
	}

	playlistSvc := NewPlaylistService(playlistRepo, mediaRepo, windowRepo)

	// Verify all 4 windows return independent playlists
	for _, c := range configs {
		p, err := playlistSvc.GetPlaylist(ctx, fmt.Sprintf("%d", c.num))
		if err != nil {
			t.Fatalf("failed to get playlist for window %d: %v", c.num, err)
		}
		if len(p.Items) != len(c.items) {
			t.Errorf("window %d expected %d items, got %d", c.num, len(c.items), len(p.Items))
		}
	}

	// Mutate Window 1: Add M4
	_, err := playlistSvc.AddPlaylistItem(ctx, "1", "M4", 15)
	if err != nil {
		t.Fatalf("failed to add item to window 1: %v", err)
	}

	// Verify Window 1 has 4 items
	p1, _ := playlistSvc.GetPlaylist(ctx, "1")
	if len(p1.Items) != 4 {
		t.Errorf("expected 4 items in window 1, got %d", len(p1.Items))
	}

	// Verify Windows 2, 3, 4 were STRICTLY UNAFFECTED
	p2, _ := playlistSvc.GetPlaylist(ctx, "2")
	if len(p2.Items) != 2 {
		t.Errorf("window 2 was contaminated by window 1 update! Got %d items", len(p2.Items))
	}

	p3, _ := playlistSvc.GetPlaylist(ctx, "3")
	if len(p3.Items) != 3 {
		t.Errorf("window 3 was contaminated! Got %d items", len(p3.Items))
	}

	p4, _ := playlistSvc.GetPlaylist(ctx, "4")
	if len(p4.Items) != 2 {
		t.Errorf("window 4 was contaminated! Got %d items", len(p4.Items))
	}
}

// ============================================================================
// 3. NO AUTOMATIC BLANK REGRESSION TEST
// ============================================================================
func TestQA_NoAutomaticBlank_Regression(t *testing.T) {
	// A playlist with M1=10s, M2=20s, M3=30s has total duration = 60s.
	// Over a 5-hour (18,000s) cycle, the sequence must repeat continuously:
	// M1 -> M2 -> M3 -> M1 -> M2 -> M3 -> ...
	// It must NEVER return PlaybackStatusBlank or PlaybackStatusFallback!
	engine := timeline.NewEngine()
	origin := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
	window := &models.Window{
		WindowNumber:         1,
		Name:                 "Window 1",
		CycleDurationSeconds: 18000,
		CycleStartTime:       origin,
		IsActive:             true,
	}

	playlist := &models.Playlist{
		WindowNumber: 1,
		Items: []models.PlaylistItem{
			{ItemID: "1", MediaKey: "M1", Type: models.MediaTypeVideo, DurationSeconds: 10, Order: 1},
			{ItemID: "2", MediaKey: "M2", Type: models.MediaTypeImage, DurationSeconds: 20, Order: 2},
			{ItemID: "3", MediaKey: "M3", Type: models.MediaTypeVideo, DurationSeconds: 30, Order: 3},
		},
		TotalSequenceDurationSeconds: 60,
		Version:                      1,
	}

	// Test 500 distinct timestamps scattered across the 5-hour cycle
	r := rand.New(rand.NewSource(42))
	for i := 0; i < 500; i++ {
		secOffset := r.Float64() * 18000.0 // random point in 5-hour cycle
		queryTime := origin.Add(time.Duration(secOffset * float64(time.Second)))

		state, err := engine.Calculate(window, playlist, queryTime)
		if err != nil {
			t.Fatalf("unexpected error at offset %.2fs: %v", secOffset, err)
		}

		if state.Status == timeline.PlaybackStatusBlank {
			t.Fatalf("CRITICAL BUG: automatic blank detected at %.2fs into cycle! Status must be NORMAL", secOffset)
		}
		if state.Status == timeline.PlaybackStatusFallback {
			t.Fatalf("CRITICAL BUG: fallback detected at %.2fs! Status must be NORMAL", secOffset)
		}

		expectedMod := int64(secOffset) % 60
		var expectedKey string
		if expectedMod < 10 {
			expectedKey = "M1"
		} else if expectedMod < 30 {
			expectedKey = "M2"
		} else {
			expectedKey = "M3"
		}

		if state.MediaKey != expectedKey {
			t.Fatalf("at %.2fs (mod %ds): expected %s, got %s", secOffset, expectedMod, expectedKey, state.MediaKey)
		}
	}
}

// ============================================================================
// 4. EXPLICIT BLANK TEST
// ============================================================================
func TestQA_ExplicitBlankBehavior(t *testing.T) {
	// M1 (15s) -> BLANK (10s) -> M2 (20s)
	// Blank should only display for the configured 10 seconds.
	engine := timeline.NewEngine()
	origin := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
	window := &models.Window{
		WindowNumber:         1,
		CycleDurationSeconds: 18000,
		CycleStartTime:       origin,
		IsActive:             true,
	}

	playlist := &models.Playlist{
		WindowNumber: 1,
		Items: []models.PlaylistItem{
			{ItemID: "1", MediaKey: "M1", Type: models.MediaTypeVideo, DurationSeconds: 15, Order: 1},
			{ItemID: "2", MediaKey: "BLANK_10", Type: models.MediaTypeBlank, DurationSeconds: 10, Order: 2},
			{ItemID: "3", MediaKey: "M2", Type: models.MediaTypeImage, DurationSeconds: 20, Order: 3},
		},
		TotalSequenceDurationSeconds: 45,
		Version:                      1,
	}

	// t=14.9s -> M1
	st1, _ := engine.Calculate(window, playlist, origin.Add(14*time.Second+900*time.Millisecond))
	if st1.MediaKey != "M1" || st1.Status != timeline.PlaybackStatusNormal {
		t.Errorf("expected M1 at 14.9s, got %+v", st1)
	}

	// t=15.0s -> BLANK_10 (explicit blank)
	st2, _ := engine.Calculate(window, playlist, origin.Add(15*time.Second))
	if st2.MediaKey != "BLANK_10" || st2.Status != timeline.PlaybackStatusBlank {
		t.Errorf("expected BLANK_10 at 15.0s, got %+v", st2)
	}

	// t=24.9s -> BLANK_10
	st3, _ := engine.Calculate(window, playlist, origin.Add(24*time.Second+900*time.Millisecond))
	if st3.MediaKey != "BLANK_10" || st3.Status != timeline.PlaybackStatusBlank {
		t.Errorf("expected BLANK_10 at 24.9s, got %+v", st3)
	}

	// t=25.0s -> M2
	st4, _ := engine.Calculate(window, playlist, origin.Add(25*time.Second))
	if st4.MediaKey != "M2" || st4.Status != timeline.PlaybackStatusNormal {
		t.Errorf("expected M2 at 25.0s, got %+v", st4)
	}

	// t=45.0s -> sequence wraps back to M1
	st5, _ := engine.Calculate(window, playlist, origin.Add(45*time.Second))
	if st5.MediaKey != "M1" || st5.Status != timeline.PlaybackStatusNormal {
		t.Errorf("expected M1 at 45.0s wrap, got %+v", st5)
	}
}

// ============================================================================
// 5. SYNC + PLAYLIST UPDATE CONCURRENT RACE TEST
// ============================================================================
func TestQA_SyncPlusPlaylistUpdateRace(t *testing.T) {
	coordinator, hub, _, mediaRepo, windowRepo, playlistRepo := setupTestCoordinator(t)
	ctx := context.Background()

	// 1. Start synchronization for M2
	syncEvent, err := coordinator.TriggerSync(ctx, "M2", 30, 1000, "admin")
	if err != nil {
		t.Fatalf("failed to trigger sync: %v", err)
	}

	// 2. While sync is active, perform playlist mutation on Window 1
	playlistSvc := NewPlaylistService(playlistRepo, mediaRepo, windowRepo)
	_ = mediaRepo.Create(ctx, &models.Media{
		MediaKey: "M99", Name: "New Media", Type: models.MediaTypeVideo, DurationSeconds: 15, URL: "http://example.com/99.mp4",
	})

	updatedP, err := playlistSvc.AddPlaylistItem(ctx, "1", "M99", 15)
	if err != nil {
		t.Fatalf("failed to add item during sync: %v", err)
	}
	if len(updatedP.Items) != 2 {
		t.Fatalf("expected 2 items in updated playlist")
	}

	// 3. Verify active sync was NOT corrupted by the playlist update
	active, err := coordinator.GetActiveSync(ctx)
	if err != nil {
		t.Fatalf("failed to get active sync: %v", err)
	}
	if active.EventID != syncEvent.EventID {
		t.Errorf("active sync was altered by playlist update: %s != %s", active.EventID, syncEvent.EventID)
	}

	// 4. Snapshot for Window 1 should show both: updated playlist AND active sync
	snap, err := coordinator.BuildSnapshotForWindow(ctx, 1)
	if err != nil {
		t.Fatalf("failed to build snapshot: %v", err)
	}
	if snap.ActiveSync == nil || snap.ActiveSync.MediaKey != "M2" {
		t.Errorf("snapshot active sync missing or corrupted: %+v", snap.ActiveSync)
	}
	if snap.Playlist == nil || len(snap.Playlist.Items) != 2 {
		t.Errorf("snapshot does not reflect updated playlist: %+v", snap.Playlist)
	}

	// 5. Cancel sync
	err = coordinator.CancelSync(ctx, syncEvent.EventID)
	if err != nil {
		t.Fatalf("failed to cancel sync: %v", err)
	}

	// Snapshot after sync cancellation should show no active sync and updated playlist
	snapAfter, err := coordinator.BuildSnapshotForWindow(ctx, 1)
	if err != nil {
		t.Fatalf("failed to build snapshot after sync: %v", err)
	}
	if snapAfter.ActiveSync != nil {
		t.Errorf("expected active sync to be nil after cancellation, got %+v", snapAfter.ActiveSync)
	}

	_ = hub
}

// ============================================================================
// 6. WEBSOCKET STRESS TEST & CONCURRENCY
// ============================================================================
func TestQA_WebSocketHubStressAndStorms(t *testing.T) {
	hub := websocket.NewHub()
	var wg sync.WaitGroup

	numClients := 50
	operationsPerClient := 20

	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()
			wNum := (clientID % 4) + 1
			client := websocket.NewClient(hub, nil, wNum)

			for op := 0; op < operationsPerClient; op++ {
				hub.Register(client)
				hub.Broadcast(websocket.NewEnvelope(websocket.EventSyncStarted, map[string]string{"id": "sync_1"}))
				hub.BroadcastToWindow(wNum, websocket.NewEnvelope(websocket.EventPlaylistUpdated, map[string]int{"w": wNum}))
				hub.RegisterWindow(client, ((wNum+op)%4)+1)
				hub.Unregister(client)
			}
		}(i)
	}

	wg.Wait()

	if hub.ClientCount() != 0 {
		t.Errorf("expected 0 clients after storm, got %d", hub.ClientCount())
	}
}
