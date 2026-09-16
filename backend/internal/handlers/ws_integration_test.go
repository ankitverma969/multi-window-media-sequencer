package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/eva-bharat/media-sequencer/backend/internal/config"
	"github.com/eva-bharat/media-sequencer/backend/internal/models"
	"github.com/eva-bharat/media-sequencer/backend/internal/repository"
	"github.com/eva-bharat/media-sequencer/backend/internal/service"
	ws "github.com/eva-bharat/media-sequencer/backend/internal/websocket"
	gorilla "github.com/gorilla/websocket"
)

func setupFullIntegrationServer(t *testing.T) (
	*httptest.Server,
	*ws.Hub,
	service.SyncCoordinator,
	repository.PlaylistRepository,
) {
	ctx := context.Background()

	mediaRepo := repository.NewMockMediaRepository()
	windowRepo := repository.NewMockWindowRepository()
	playlistRepo := repository.NewMockPlaylistRepository()
	syncRepo := repository.NewMockSyncRepository()

	// Seed 4 windows
	for i := 1; i <= 4; i++ {
		_ = windowRepo.Create(ctx, &models.Window{
			WindowNumber:         i,
			Name:                 fmt.Sprintf("Window %d", i),
			CycleDurationSeconds: 18000,
			CycleStartTime:       time.Now().UTC().Add(-1 * time.Hour),
			IsActive:             true,
		})

		_ = playlistRepo.Upsert(ctx, &models.Playlist{
			WindowNumber: i,
			Items: []models.PlaylistItem{
				{
					ItemID:          fmt.Sprintf("item-w%d-1", i),
					MediaKey:        fmt.Sprintf("M%d", i),
					Type:            models.MediaTypeVideo,
					URL:             fmt.Sprintf("https://example.com/m%d.mp4", i),
					DurationSeconds: 30,
					Order:           1,
				},
			},
			TotalSequenceDurationSeconds: 30,
			Version:                      1,
		})
	}

	// Seed media assets M1..M4
	for i := 1; i <= 4; i++ {
		_ = mediaRepo.Create(ctx, &models.Media{
			MediaKey:        fmt.Sprintf("M%d", i),
			Name:            fmt.Sprintf("Media %d", i),
			Type:            models.MediaTypeVideo,
			URL:             fmt.Sprintf("https://example.com/m%d.mp4", i),
			DurationSeconds: 30,
		})
	}

	wsHub := ws.NewHub()
	mediaService := service.NewMediaService(mediaRepo)
	windowService := service.NewWindowService(windowRepo)
	playlistService := service.NewPlaylistService(playlistRepo, mediaRepo, windowRepo)
	syncCoordinator := service.NewSyncCoordinator(syncRepo, mediaRepo, windowRepo, playlistRepo, playlistService, wsHub)

	cfg := &config.Config{
		Environment:        "test",
		Port:               "8080",
		CORSAllowedOrigins: []string{"*"},
	}

	router := NewRouter(Dependencies{
		Config:          cfg,
		Pinger:          &mockPinger{},
		WindowService:   windowService,
		MediaService:    mediaService,
		PlaylistService: playlistService,
		SyncService:     syncCoordinator,
		WSHub:           wsHub,
	})

	server := httptest.NewServer(router)
	return server, wsHub, syncCoordinator, playlistRepo
}

func TestWebSocket_InvalidWindowConnectionRejected(t *testing.T) {
	server, hub, _, _ := setupFullIntegrationServer(t)
	defer server.Close()
	defer hub.Shutdown()

	u, _ := url.Parse(server.URL)
	u.Scheme = "ws"
	u.Path = "/ws"
	u.RawQuery = "window_id=999" // non-existent window

	_, resp, err := gorilla.DefaultDialer.Dial(u.String(), nil)
	if err == nil {
		t.Fatal("expected connection to be rejected for invalid window")
	}
	if resp != nil && resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status 400 Bad Request, got %d", resp.StatusCode)
	}
}

func TestWebSocket_MultiWindowSyncAndPlaylistFlow(t *testing.T) {
	server, hub, _, playlistRepo := setupFullIntegrationServer(t)
	defer server.Close()
	defer hub.Shutdown()

	u, _ := url.Parse(server.URL)
	u.Scheme = "ws"
	u.Path = "/ws"

	conns := make([]*gorilla.Conn, 4)

	// 1. Connect 4 client windows and verify initial STATE_SNAPSHOT
	for i := 1; i <= 4; i++ {
		q := url.Values{}
		q.Set("window_id", fmt.Sprintf("%d", i))
		wsURL := fmt.Sprintf("%s://%s/ws?%s", u.Scheme, u.Host, q.Encode())

		conn, resp, err := gorilla.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("failed to connect window %d client: %v", i, err)
		}
		if resp.StatusCode != http.StatusSwitchingProtocols {
			t.Fatalf("expected switching protocols, got %d", resp.StatusCode)
		}
		defer conn.Close()
		conns[i-1] = conn

		// Read initial STATE_SNAPSHOT
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, msg, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("window %d failed to read state snapshot: %v", i, err)
		}

		var env ws.Envelope
		if err := json.Unmarshal(msg, &env); err != nil {
			t.Fatalf("window %d unmarshal error: %v", i, err)
		}
		if env.Type != ws.EventStateSnapshot {
			t.Fatalf("window %d expected %s, got %s", i, ws.EventStateSnapshot, env.Type)
		}

		// Verify snapshot payload matches window
		payloadBytes, _ := json.Marshal(env.Payload)
		var snap ws.StateSnapshotPayload
		_ = json.Unmarshal(payloadBytes, &snap)

		if snap.WindowNumber != i {
			t.Fatalf("expected window %d in snapshot, got %d", i, snap.WindowNumber)
		}
		if snap.Playlist == nil || len(snap.Playlist.Items) == 0 {
			t.Fatalf("window %d expected non-empty playlist in snapshot", i)
		}
		if snap.ActiveSync != nil {
			t.Fatalf("window %d expected nil active sync initially", i)
		}
	}

	// 2. Trigger sync for M2 via REST POST /api/v1/sync
	syncReqBody := []byte(`{
		"media_id": "M2",
		"duration_seconds": 2,
		"lead_time_ms": 500
	}`)
	resp, err := http.Post(server.URL+"/api/v1/sync", "application/json", bytes.NewReader(syncReqBody))
	if err != nil {
		t.Fatalf("failed to post sync request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200/201 from sync trigger, got %d", resp.StatusCode)
	}

	// 3. Verify all 4 connected windows receive identical SYNC_STARTED event
	var syncEventID string
	for i := 1; i <= 4; i++ {
		conn := conns[i-1]
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, msg, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("window %d failed to receive SYNC_STARTED: %v", i, err)
		}

		var env ws.Envelope
		if err := json.Unmarshal(msg, &env); err != nil {
			t.Fatalf("window %d unmarshal error: %v", i, err)
		}
		if env.Type != ws.EventSyncStarted {
			t.Fatalf("window %d expected %s, got %s", i, ws.EventSyncStarted, env.Type)
		}

		payloadBytes, _ := json.Marshal(env.Payload)
		var syncStarted ws.SyncStartedPayload
		_ = json.Unmarshal(payloadBytes, &syncStarted)

		if syncStarted.MediaKey != "M2" {
			t.Fatalf("window %d expected sync media M2, got %s", i, syncStarted.MediaKey)
		}
		if syncEventID == "" {
			syncEventID = syncStarted.EventID
		} else if syncStarted.EventID != syncEventID {
			t.Fatalf("window %d received mismatched syncEventID: %s != %s", i, syncStarted.EventID, syncEventID)
		}
	}

	// 4. Verify that stored playlists in MongoDB/repo were NEVER mutated
	for i := 1; i <= 4; i++ {
		p, err := playlistRepo.FindByWindowNumber(context.Background(), i)
		if err != nil {
			t.Fatalf("failed to retrieve playlist for window %d: %v", i, err)
		}
		expectedKey := fmt.Sprintf("M%d", i)
		if len(p.Items) != 1 || p.Items[0].MediaKey != expectedKey {
			t.Fatalf("window %d playlist was mutated during sync! Expected %s, got %+v", i, expectedKey, p.Items)
		}
	}

	// 5. Connect a NEW client during sync (Mid-sync joining test)
	// Connecting to Window 1 while sync is active
	lateURL := fmt.Sprintf("%s://%s/ws?window_id=1", u.Scheme, u.Host)
	lateConn, _, err := gorilla.DefaultDialer.Dial(lateURL, nil)
	if err != nil {
		t.Fatalf("failed to connect late joining client: %v", err)
	}
	defer lateConn.Close()

	_ = lateConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, lateMsg, err := lateConn.ReadMessage()
	if err != nil {
		t.Fatalf("late client failed to read snapshot: %v", err)
	}

	var lateEnv ws.Envelope
	_ = json.Unmarshal(lateMsg, &lateEnv)
	if lateEnv.Type != ws.EventStateSnapshot {
		t.Fatalf("expected STATE_SNAPSHOT for late joiner, got %s", lateEnv.Type)
	}

	lateSnapBytes, _ := json.Marshal(lateEnv.Payload)
	var lateSnap ws.StateSnapshotPayload
	_ = json.Unmarshal(lateSnapBytes, &lateSnap)

	if lateSnap.ActiveSync == nil {
		t.Fatal("late joining client expected active_sync in snapshot!")
	}
	if lateSnap.ActiveSync.MediaKey != "M2" {
		t.Fatalf("expected late joiner active sync to be M2, got %s", lateSnap.ActiveSync.MediaKey)
	}

	// 6. Test targeted PLAYLIST_UPDATED event:
	// Update Window 1's playlist via REST POST /api/v1/windows/1/playlist
	addReqBody := []byte(`{
		"media_id": "M3",
		"duration": 25
	}`)
	addResp, err := http.Post(server.URL+"/api/v1/windows/1/playlist", "application/json", bytes.NewReader(addReqBody))
	if err != nil {
		t.Fatalf("failed to add playlist item: %v", err)
	}
	defer addResp.Body.Close()
	if addResp.StatusCode != http.StatusOK && addResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 200/201 from add playlist, got %d", addResp.StatusCode)
	}

	// Window 1 client should receive PLAYLIST_UPDATED
	w1Conn := conns[0]
	_ = w1Conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, w1Msg, err := w1Conn.ReadMessage()
	if err != nil {
		t.Fatalf("window 1 failed to receive playlist update: %v", err)
	}

	var w1Env ws.Envelope
	_ = json.Unmarshal(w1Msg, &w1Env)
	if w1Env.Type != ws.EventPlaylistUpdated {
		t.Fatalf("window 1 expected %s, got %s", ws.EventPlaylistUpdated, w1Env.Type)
	}

	// 7. Cancel the active sync via REST POST /api/v1/sync/{id}/cancel
	cancelURL := fmt.Sprintf("%s/api/v1/sync/%s/cancel", server.URL, syncEventID)
	cancelResp, err := http.Post(cancelURL, "application/json", nil)
	if err != nil {
		t.Fatalf("failed to cancel sync: %v", err)
	}
	defer cancelResp.Body.Close()
	if cancelResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from cancel sync, got %d", cancelResp.StatusCode)
	}

	// All 4 windows should receive SYNC_ENDED with reason CANCELLED
	for i := 1; i <= 4; i++ {
		conn := conns[i-1]
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, msg, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("window %d failed to receive SYNC_ENDED: %v", i, err)
		}

		var env ws.Envelope
		_ = json.Unmarshal(msg, &env)
		if env.Type != ws.EventSyncEnded {
			t.Fatalf("window %d expected %s, got %s", i, ws.EventSyncEnded, env.Type)
		}

		endedBytes, _ := json.Marshal(env.Payload)
		var endedPayload ws.SyncEndedPayload
		_ = json.Unmarshal(endedBytes, &endedPayload)

		if endedPayload.EventID != syncEventID {
			t.Fatalf("window %d expected ended event %s, got %s", i, syncEventID, endedPayload.EventID)
		}
	}
}
