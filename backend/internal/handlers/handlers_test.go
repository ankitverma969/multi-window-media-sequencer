package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/eva-bharat/media-sequencer/backend/internal/config"
	"github.com/eva-bharat/media-sequencer/backend/internal/models"
	"github.com/eva-bharat/media-sequencer/backend/internal/repository"
	"github.com/eva-bharat/media-sequencer/backend/internal/service"
	"github.com/eva-bharat/media-sequencer/backend/internal/utils"
)

type mockPinger struct {
	err error
}

func (m *mockPinger) Ping(ctx context.Context) error {
	return m.err
}

func setupTestRouter(pingErr error) (http.Handler, *repository.MockMediaRepository, *repository.MockWindowRepository, *repository.MockPlaylistRepository) {
	cfg := &config.Config{
		Port:               "8080",
		Environment:        "test",
		CORSAllowedOrigins: []string{"http://localhost:3000"},
	}

	mediaRepo := repository.NewMockMediaRepository()
	windowRepo := repository.NewMockWindowRepository()
	playlistRepo := repository.NewMockPlaylistRepository()

	deps := Dependencies{
		Config:          cfg,
		Pinger:          &mockPinger{err: pingErr},
		WindowService:   service.NewWindowService(windowRepo),
		MediaService:    service.NewMediaService(mediaRepo),
		PlaylistService: service.NewPlaylistService(playlistRepo, mediaRepo, windowRepo),
	}

	return NewRouter(deps), mediaRepo, windowRepo, playlistRepo
}

func TestHealthCheckSuccess(t *testing.T) {
	router, _, _, _ := setupTestRouter(nil)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp utils.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success true")
	}
}

func TestHealthCheckDatabaseUnavailable(t *testing.T) {
	router, _, _, _ := setupTestRouter(errors.New("db connection lost"))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", rec.Code)
	}

	var resp utils.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Success {
		t.Errorf("expected success false on DB error")
	}
	if resp.Error == nil || resp.Error.Code != "DATABASE_UNAVAILABLE" {
		t.Errorf("expected DATABASE_UNAVAILABLE code, got %+v", resp.Error)
	}
}

func TestTimeEndpoint(t *testing.T) {
	router, _, _, _ := setupTestRouter(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/time", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp utils.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	dataMap, ok := resp.Data.(map[string]any)
	if !ok || dataMap["server_time_ms"] == nil {
		t.Errorf("expected server_time_ms in response data: %+v", resp.Data)
	}
}

func TestPlaylistEndpoints(t *testing.T) {
	ctx := context.Background()
	router, mediaRepo, windowRepo, playlistRepo := setupTestRouter(nil)

	// Seed window 1
	_ = windowRepo.Create(ctx, &models.Window{
		WindowNumber:         1,
		Name:                 "Window 1",
		CycleDurationSeconds: 18000,
		CycleStartTime:       time.Now().UTC(),
		IsActive:             true,
	})

	// Seed media M1
	_ = mediaRepo.Create(ctx, &models.Media{
		MediaKey:        "M1",
		Name:            "Intro Video",
		Type:            models.MediaTypeVideo,
		URL:             "https://example.com/m1.mp4",
		DurationSeconds: 20,
	})

	// Seed empty playlist
	_ = playlistRepo.Upsert(ctx, &models.Playlist{
		WindowNumber: 1,
		Items:        []models.PlaylistItem{},
	})

	// 1. Add item to playlist via POST /api/v1/windows/1/playlist/items
	addBody := `{"media_key":"M1","custom_duration_seconds":20}`
	reqAdd := httptest.NewRequest(http.MethodPost, "/api/v1/windows/1/playlist/items", bytes.NewBufferString(addBody))
	recAdd := httptest.NewRecorder()

	router.ServeHTTP(recAdd, reqAdd)

	if recAdd.Code != http.StatusCreated {
		t.Fatalf("expected status 201 on add item, got %d: %s", recAdd.Code, recAdd.Body.String())
	}

	// 2. Fetch playlist via GET /api/v1/windows/1/playlist
	reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/windows/1/playlist", nil)
	recGet := httptest.NewRecorder()

	router.ServeHTTP(recGet, reqGet)

	if recGet.Code != http.StatusOK {
		t.Fatalf("expected status 200 on get playlist, got %d", recGet.Code)
	}

	var getResp utils.APIResponse
	_ = json.NewDecoder(recGet.Body).Decode(&getResp)
	pMap := getResp.Data.(map[string]any)
	items := pMap["items"].([]any)
	if len(items) != 1 {
		t.Errorf("expected 1 item in playlist, got %d", len(items))
	}
}
