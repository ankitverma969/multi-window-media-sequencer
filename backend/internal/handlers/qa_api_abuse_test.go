package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/eva-bharat/media-sequencer/backend/internal/config"
	"github.com/eva-bharat/media-sequencer/backend/internal/models"
	"github.com/eva-bharat/media-sequencer/backend/internal/repository"
	"github.com/eva-bharat/media-sequencer/backend/internal/service"
	ws "github.com/eva-bharat/media-sequencer/backend/internal/websocket"
)

func setupAbuseTestServer(t *testing.T) (*httptest.Server, *ws.Hub) {
	ctx := context.Background()

	mediaRepo := repository.NewMockMediaRepository()
	windowRepo := repository.NewMockWindowRepository()
	playlistRepo := repository.NewMockPlaylistRepository()
	syncRepo := repository.NewMockSyncRepository()

	_ = mediaRepo.Create(ctx, &models.Media{
		MediaKey: "M1", Name: "Media 1", Type: models.MediaTypeVideo, DurationSeconds: 30, URL: "http://example.com/1.mp4",
	})
	_ = windowRepo.Create(ctx, &models.Window{
		WindowNumber: 1, Name: "Window 1", CycleDurationSeconds: 18000, CycleStartTime: time.Now().UTC(), IsActive: true,
	})
	_ = playlistRepo.Upsert(ctx, &models.Playlist{
		WindowNumber: 1,
		Items: []models.PlaylistItem{
			{ItemID: "item-1", MediaKey: "M1", Type: models.MediaTypeVideo, DurationSeconds: 30, Order: 1},
		},
		TotalSequenceDurationSeconds: 30,
		Version:                      1,
	})

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
	return server, wsHub
}

func TestQA_APIAbuse_MalformedJSON(t *testing.T) {
	server, hub := setupAbuseTestServer(t)
	defer server.Close()
	defer hub.Shutdown()

	malformedPayloads := []struct {
		method string
		path   string
		body   string
	}{
		{"POST", "/api/v1/sync", `{"media_id": "M1",`},
		{"POST", "/api/v1/sync", `not-json`},
		{"POST", "/api/v1/windows/1/playlist", `{"media_key": `},
		{"PUT", "/api/v1/windows/1/playlist", `[{"duration": }`},
		{"POST", "/api/v1/media", `{"name": "test"`},
	}

	for _, tc := range malformedPayloads {
		req, _ := http.NewRequest(tc.method, server.URL+tc.path, bytes.NewBufferString(tc.body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("unexpected client error on %s %s: %v", tc.method, tc.path, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s %s expected 400 Bad Request for malformed JSON, got %d", tc.method, tc.path, resp.StatusCode)
		}

		// Verify response is safe structured JSON
		var errResp map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
			t.Errorf("expected valid JSON error response: %v", err)
		}
		if errResp["success"] != false {
			t.Errorf("expected success: false in error response")
		}
	}
}

func TestQA_APIAbuse_MissingAndExtremeFields(t *testing.T) {
	server, hub := setupAbuseTestServer(t)
	defer server.Close()
	defer hub.Shutdown()

	testCases := []struct {
		name           string
		method         string
		path           string
		body           string
		expectedStatus int
	}{
		// Missing fields
		{"sync empty body", "POST", "/api/v1/sync", `{}`, http.StatusBadRequest},
		{"playlist item empty body", "POST", "/api/v1/windows/1/playlist", `{}`, http.StatusBadRequest},

		// Extreme/negative durations in sync
		{"sync negative duration", "POST", "/api/v1/sync", `{"media_id": "M1", "duration_seconds": -5}`, http.StatusBadRequest},
		{"sync zero duration", "POST", "/api/v1/sync", `{"media_id": "M1", "duration_seconds": 0}`, http.StatusBadRequest},
		{"sync excessive duration", "POST", "/api/v1/sync", `{"media_id": "M1", "duration_seconds": 99999}`, http.StatusBadRequest},

		// Nonexistent resources
		{"get nonexistent window", "GET", "/api/v1/windows/9999", ``, http.StatusNotFound},
		{"get nonexistent media", "GET", "/api/v1/media/NON_EXISTENT", ``, http.StatusNotFound},
		{"sync nonexistent media", "POST", "/api/v1/sync", `{"media_id": "NON_EXISTENT", "duration_seconds": 15}`, http.StatusNotFound},
		{"cancel nonexistent sync", "POST", "/api/v1/sync/sync_nonexistent/cancel", ``, http.StatusNotFound},
		{"add item to nonexistent window", "POST", "/api/v1/windows/9999/playlist", `{"media_key": "M1"}`, http.StatusNotFound},
		{"remove item from nonexistent window", "DELETE", "/api/v1/windows/9999/playlist/item-1", ``, http.StatusNotFound},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest(tc.method, server.URL+tc.path, bytes.NewBufferString(tc.body))
			req.Header.Set("Content-Type", "application/json")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tc.expectedStatus {
				t.Errorf("expected status %d, got %d", tc.expectedStatus, resp.StatusCode)
			}
		})
	}
}
