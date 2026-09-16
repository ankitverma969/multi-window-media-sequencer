package service

import (
	"context"
	"testing"
	"time"

	"github.com/eva-bharat/media-sequencer/backend/internal/models"
	"github.com/eva-bharat/media-sequencer/backend/internal/repository"
	"github.com/eva-bharat/media-sequencer/backend/internal/websocket"
)

// mockHub implements websocket.HubInterface for testing coordinator events.
type mockHub struct {
	broadcastEvents       []websocket.Envelope
	windowBroadcastEvents map[int][]websocket.Envelope
	onWindowRegister      func(client *websocket.Client, windowNumber int)
}

func newMockHub() *mockHub {
	return &mockHub{
		broadcastEvents:       make([]websocket.Envelope, 0),
		windowBroadcastEvents: make(map[int][]websocket.Envelope),
	}
}

func (m *mockHub) Register(client *websocket.Client)                         {}
func (m *mockHub) Unregister(client *websocket.Client)                       {}
func (m *mockHub) RegisterWindow(client *websocket.Client, windowNumber int) {}
func (m *mockHub) Broadcast(env websocket.Envelope) {
	m.broadcastEvents = append(m.broadcastEvents, env)
}
func (m *mockHub) BroadcastToWindow(windowNumber int, env websocket.Envelope) {
	m.windowBroadcastEvents[windowNumber] = append(m.windowBroadcastEvents[windowNumber], env)
}
func (m *mockHub) ClientCount() int                       { return 0 }
func (m *mockHub) WindowClientCount(windowNumber int) int { return 0 }
func (m *mockHub) SetWindowRegisterHandler(fn func(client *websocket.Client, windowNumber int)) {
	m.onWindowRegister = fn
}
func (m *mockHub) Shutdown() {}

func setupTestCoordinator(t *testing.T) (
	SyncCoordinator,
	*mockHub,
	repository.SyncRepository,
	repository.MediaRepository,
	repository.WindowRepository,
	repository.PlaylistRepository,
) {
	ctx := context.Background()
	syncRepo := repository.NewMockSyncRepository()
	mediaRepo := repository.NewMockMediaRepository()
	windowRepo := repository.NewMockWindowRepository()
	playlistRepo := repository.NewMockPlaylistRepository()
	hub := newMockHub()

	// Seed Windows 1 and 2
	w1 := &models.Window{
		WindowNumber:         1,
		Name:                 "Window 1",
		CycleDurationSeconds: 18000,
		CycleStartTime:       time.Now().UTC().Add(-1 * time.Hour),
		IsActive:             true,
	}
	_ = windowRepo.Create(ctx, w1)

	w2 := &models.Window{
		WindowNumber:         2,
		Name:                 "Window 2",
		CycleDurationSeconds: 18000,
		CycleStartTime:       time.Now().UTC().Add(-1 * time.Hour),
		IsActive:             true,
	}
	_ = windowRepo.Create(ctx, w2)

	// Seed Media items
	m1 := &models.Media{
		MediaKey:        "M1",
		Name:            "Video 1",
		Type:            models.MediaTypeVideo,
		URL:             "https://example.com/m1.mp4",
		DurationSeconds: 30,
	}
	_ = mediaRepo.Create(ctx, m1)

	m2 := &models.Media{
		MediaKey:        "M2",
		Name:            "Sync Video 2",
		Type:            models.MediaTypeVideo,
		URL:             "https://example.com/m2.mp4",
		DurationSeconds: 15,
	}
	_ = mediaRepo.Create(ctx, m2)

	// Seed Playlist for Window 1
	p1 := &models.Playlist{
		WindowNumber: 1,
		Items: []models.PlaylistItem{
			{ItemID: "item-1", MediaKey: "M1", Type: models.MediaTypeVideo, URL: "https://example.com/m1.mp4", DurationSeconds: 30, Order: 1},
		},
		TotalSequenceDurationSeconds: 30,
	}
	_ = playlistRepo.Upsert(ctx, p1)

	// Seed Playlist for Window 2
	p2 := &models.Playlist{
		WindowNumber: 2,
		Items: []models.PlaylistItem{
			{ItemID: "item-2", MediaKey: "M2", Type: models.MediaTypeVideo, URL: "https://example.com/m2.mp4", DurationSeconds: 15, Order: 1},
		},
		TotalSequenceDurationSeconds: 15,
	}
	_ = playlistRepo.Upsert(ctx, p2)

	playlistService := NewPlaylistService(playlistRepo, mediaRepo, windowRepo)
	coordinator := NewSyncCoordinator(syncRepo, mediaRepo, windowRepo, playlistRepo, playlistService, hub)

	return coordinator, hub, syncRepo, mediaRepo, windowRepo, playlistRepo
}

func TestSyncCoordinator_TriggerSync_ValidationAndAuthoritativeTimestamps(t *testing.T) {
	coordinator, hub, _, _, _, _ := setupTestCoordinator(t)
	ctx := context.Background()

	// 1. Invalid duration (< 1)
	_, err := coordinator.TriggerSync(ctx, "M2", 0, 1000, "admin")
	if err == nil {
		t.Fatal("expected error for 0 duration")
	}

	// 2. Invalid duration (> 3600)
	_, err = coordinator.TriggerSync(ctx, "M2", 3601, 1000, "admin")
	if err == nil {
		t.Fatal("expected error for duration > 3600")
	}

	// 3. Non-existent media
	_, err = coordinator.TriggerSync(ctx, "NON_EXISTENT", 20, 1000, "admin")
	if err == nil {
		t.Fatal("expected error for non-existent media")
	}

	// 4. Valid trigger
	before := time.Now().UTC()
	event, err := coordinator.TriggerSync(ctx, "M2", 20, 1000, "admin")
	if err != nil {
		t.Fatalf("unexpected error triggering sync: %v", err)
	}

	if event.MediaKey != "M2" {
		t.Errorf("expected media key M2, got %s", event.MediaKey)
	}
	if event.DurationSeconds != 20 {
		t.Errorf("expected duration 20, got %d", event.DurationSeconds)
	}

	// Verify server-authoritative timestamps
	expectedStartMin := before.Add(900 * time.Millisecond)
	expectedStartMax := time.Now().UTC().Add(1200 * time.Millisecond)
	if event.StartTime.Before(expectedStartMin) || event.StartTime.After(expectedStartMax) {
		t.Errorf("start time out of expected range: %v", event.StartTime)
	}

	expectedEnd := event.StartTime.Add(20 * time.Second)
	if !event.EndTime.Equal(expectedEnd) {
		t.Errorf("expected end time %v, got %v", expectedEnd, event.EndTime)
	}

	// Verify SYNC_STARTED broadcast occurred
	if len(hub.broadcastEvents) == 0 {
		t.Fatal("expected broadcast event to be dispatched")
	}
	lastEvent := hub.broadcastEvents[len(hub.broadcastEvents)-1]
	if lastEvent.Type != websocket.EventSyncStarted {
		t.Errorf("expected EventSyncStarted, got %s", lastEvent.Type)
	}
}

func TestSyncCoordinator_ActiveSyncConflictReplacement(t *testing.T) {
	coordinator, hub, syncRepo, _, _, _ := setupTestCoordinator(t)
	ctx := context.Background()

	// Trigger first sync
	sync1, err := coordinator.TriggerSync(ctx, "M2", 30, 1000, "admin")
	if err != nil {
		t.Fatalf("failed to trigger sync 1: %v", err)
	}

	// Trigger second sync while sync 1 is active (conflict replacement)
	sync2, err := coordinator.TriggerSync(ctx, "M1", 45, 1000, "operator")
	if err != nil {
		t.Fatalf("failed to trigger sync 2: %v", err)
	}

	if sync2.EventID == sync1.EventID {
		t.Fatal("expected different event IDs")
	}

	// Check active sync is now sync2
	active, err := coordinator.GetActiveSync(ctx)
	if err != nil {
		t.Fatalf("failed to get active sync: %v", err)
	}
	if active.EventID != sync2.EventID {
		t.Errorf("expected active sync to be %s, got %s", sync2.EventID, active.EventID)
	}

	// Allow brief moment for background update of superseded sync in repo
	time.Sleep(50 * time.Millisecond)

	// Verify sync1 is marked CANCELLED in repository
	oldEvent, err := syncRepo.FindByID(ctx, sync1.EventID)
	if err != nil {
		t.Fatalf("failed to find old event: %v", err)
	}
	if oldEvent.Status != models.SyncStatusCancelled {
		t.Errorf("expected old sync to be CANCELLED, got %s", oldEvent.Status)
	}

	// Verify both SYNC_STARTED broadcasts occurred
	if len(hub.broadcastEvents) < 2 {
		t.Fatalf("expected at least 2 broadcast events, got %d", len(hub.broadcastEvents))
	}
}

func TestSyncCoordinator_CancelSync(t *testing.T) {
	coordinator, hub, syncRepo, _, _, _ := setupTestCoordinator(t)
	ctx := context.Background()

	event, err := coordinator.TriggerSync(ctx, "M2", 30, 1000, "admin")
	if err != nil {
		t.Fatalf("failed to trigger sync: %v", err)
	}

	// Cancel the active sync
	err = coordinator.CancelSync(ctx, event.EventID)
	if err != nil {
		t.Fatalf("failed to cancel sync: %v", err)
	}

	// Verify in repo
	inRepo, err := syncRepo.FindByID(ctx, event.EventID)
	if err != nil {
		t.Fatalf("failed to find event in repo: %v", err)
	}
	if inRepo.Status != models.SyncStatusCancelled {
		t.Errorf("expected status CANCELLED, got %s", inRepo.Status)
	}

	// Verify SYNC_ENDED broadcast with reason CANCELLED
	lastEvent := hub.broadcastEvents[len(hub.broadcastEvents)-1]
	if lastEvent.Type != websocket.EventSyncEnded {
		t.Fatalf("expected EventSyncEnded, got %s", lastEvent.Type)
	}
	payload := lastEvent.Payload.(websocket.SyncEndedPayload)
	if payload.Reason != "CANCELLED" {
		t.Errorf("expected reason CANCELLED, got %s", payload.Reason)
	}
}

func TestSyncCoordinator_BuildSnapshotAndPlaylistPreservation(t *testing.T) {
	coordinator, _, _, _, _, playlistRepo := setupTestCoordinator(t)
	ctx := context.Background()

	// 1. Initial snapshot for window 1 without active sync
	snap1, err := coordinator.BuildSnapshotForWindow(ctx, 1)
	if err != nil {
		t.Fatalf("failed to build snapshot for window 1: %v", err)
	}
	if snap1.WindowNumber != 1 {
		t.Errorf("expected window number 1, got %d", snap1.WindowNumber)
	}
	if snap1.Playlist == nil || len(snap1.Playlist.Items) != 1 {
		t.Fatalf("expected playlist with 1 item")
	}
	if snap1.ActiveSync != nil {
		t.Fatalf("expected no active sync initially")
	}
	if snap1.NormalPlayback == nil {
		t.Fatalf("expected normal playback calculation")
	}

	// 2. Trigger sync for M2
	_, err = coordinator.TriggerSync(ctx, "M2", 30, 1000, "admin")
	if err != nil {
		t.Fatalf("failed to trigger sync: %v", err)
	}

	// Snapshot during scheduled sync (lead time not elapsed yet)
	snapDuring, err := coordinator.BuildSnapshotForWindow(ctx, 1)
	if err != nil {
		t.Fatalf("failed to build snapshot during sync: %v", err)
	}
	if snapDuring.ActiveSync == nil {
		t.Fatal("expected active sync in snapshot")
	}
	if snapDuring.ActiveSync.MediaKey != "M2" {
		t.Errorf("expected sync media key M2, got %s", snapDuring.ActiveSync.MediaKey)
	}

	// CRITICAL TEST: Verify Window 1 playlist in repository is STRICTLY UNCHANGED!
	p1After, err := playlistRepo.FindByWindowNumber(ctx, 1)
	if err != nil {
		t.Fatalf("failed to fetch playlist 1 from repo: %v", err)
	}
	if len(p1After.Items) != 1 || p1After.Items[0].MediaKey != "M1" {
		t.Fatalf("PLAYLIST CORRUPTED! Expected item M1, got %+v", p1After.Items)
	}
}

func TestSyncCoordinator_StartupRecovery(t *testing.T) {
	coordinator, _, syncRepo, _, _, _ := setupTestCoordinator(t)
	ctx := context.Background()

	// Manually insert an active sync that ends 10 seconds in future
	now := time.Now().UTC()
	activeEvent := &models.SyncEvent{
		EventID:         "sync_startup_active",
		MediaKey:        "M2",
		DurationSeconds: 15,
		StartTime:       now.Add(-5 * time.Second),
		EndTime:         now.Add(10 * time.Second),
		Status:          models.SyncStatusActive,
		CreatedAt:       now.Add(-5 * time.Second),
	}
	_ = syncRepo.Create(ctx, activeEvent)

	// Run recovery
	coordinator.RecoverActiveSyncOnStartup(ctx)

	// Check active sync in coordinator
	active, err := coordinator.GetActiveSync(ctx)
	if err != nil {
		t.Fatalf("failed to get active sync after recovery: %v", err)
	}
	if active == nil || active.EventID != "sync_startup_active" {
		t.Fatalf("expected recovered sync to be active, got %+v", active)
	}
}
