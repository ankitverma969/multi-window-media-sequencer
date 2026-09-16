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

func setupTestRouter(pingErr error) (
	http.Handler,
	*repository.MockMediaRepository,
	*repository.MockWindowRepository,
	*repository.MockPlaylistRepository,
	*repository.MockSyncRepository,
) {
	cfg := &config.Config{
		Port:               "8080",
		Environment:        "test",
		CORSAllowedOrigins: []string{"http://localhost:3000"},
	}

	mediaRepo := repository.NewMockMediaRepository()
	windowRepo := repository.NewMockWindowRepository()
	playlistRepo := repository.NewMockPlaylistRepository()
	syncRepo := repository.NewMockSyncRepository()

	deps := Dependencies{
		Config:          cfg,
		Pinger:          &mockPinger{err: pingErr},
		WindowService:   service.NewWindowService(windowRepo),
		MediaService:    service.NewMediaService(mediaRepo),
		PlaylistService: service.NewPlaylistService(playlistRepo, mediaRepo, windowRepo),
		SyncService:     service.NewSyncService(syncRepo, mediaRepo),
	}

	return NewRouter(deps), mediaRepo, windowRepo, playlistRepo, syncRepo
}

func TestHealthCheckSuccess(t *testing.T) {
	router, _, _, _, _ := setupTestRouter(nil)

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
	router, _, _, _, _ := setupTestRouter(errors.New("db connection lost"))

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
	router, _, _, _, _ := setupTestRouter(nil)

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

func TestWindowEndpoints(t *testing.T) {
	ctx := context.Background()
	router, _, windowRepo, _, _ := setupTestRouter(nil)

	_ = windowRepo.Create(ctx, &models.Window{
		WindowNumber:         1,
		Name:                 "Display 1",
		CycleDurationSeconds: 18000,
		CycleStartTime:       time.Now().UTC(),
		IsActive:             true,
	})

	// 1. GET /api/v1/windows
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/windows", nil)
	recList := httptest.NewRecorder()
	router.ServeHTTP(recList, reqList)

	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 on list windows, got %d", recList.Code)
	}

	// 2. GET /api/v1/windows/1
	reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/windows/1", nil)
	recGet := httptest.NewRecorder()
	router.ServeHTTP(recGet, reqGet)

	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200 on get window 1, got %d", recGet.Code)
	}

	// 3. GET /api/v1/windows/999 (Not Found)
	reqNotFound := httptest.NewRequest(http.MethodGet, "/api/v1/windows/999", nil)
	recNotFound := httptest.NewRecorder()
	router.ServeHTTP(recNotFound, reqNotFound)

	if recNotFound.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown window, got %d", recNotFound.Code)
	}
}

func TestMediaEndpoints(t *testing.T) {
	ctx := context.Background()
	router, mediaRepo, _, _, _ := setupTestRouter(nil)

	_ = mediaRepo.Create(ctx, &models.Media{
		MediaKey:        "M1",
		Name:            "Nature 4K",
		Type:            models.MediaTypeVideo,
		URL:             "https://example.com/m1.mp4",
		DurationSeconds: 30,
	})

	// 1. GET /api/v1/media
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/media", nil)
	recList := httptest.NewRecorder()
	router.ServeHTTP(recList, reqList)

	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 on list media, got %d", recList.Code)
	}

	// 2. GET /api/v1/media/M1
	reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/media/M1", nil)
	recGet := httptest.NewRecorder()
	router.ServeHTTP(recGet, reqGet)

	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200 on get media M1, got %d", recGet.Code)
	}

	// 3. POST /api/v1/media (Create Media)
	createBody := `{"media_key":"M2","name":"Promo","type":"image","url":"https://example.com/m2.jpg","duration_seconds":15}`
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v1/media", bytes.NewBufferString(createBody))
	recCreate := httptest.NewRecorder()
	router.ServeHTTP(recCreate, reqCreate)

	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 on create media, got %d: %s", recCreate.Code, recCreate.Body.String())
	}
}

func TestPlaylistEndpoints(t *testing.T) {
	ctx := context.Background()
	router, mediaRepo, windowRepo, playlistRepo, _ := setupTestRouter(nil)

	// Seed window 1
	_ = windowRepo.Create(ctx, &models.Window{
		WindowNumber:         1,
		Name:                 "Window 1",
		CycleDurationSeconds: 18000,
		CycleStartTime:       time.Now().UTC(),
		IsActive:             true,
	})

	// Seed media M1 & M2
	_ = mediaRepo.Create(ctx, &models.Media{
		MediaKey:        "M1",
		Name:            "Intro Video",
		Type:            models.MediaTypeVideo,
		URL:             "https://example.com/m1.mp4",
		DurationSeconds: 20,
	})
	_ = mediaRepo.Create(ctx, &models.Media{
		MediaKey:        "M2",
		Name:            "Feature Image",
		Type:            models.MediaTypeImage,
		URL:             "https://example.com/m2.jpg",
		DurationSeconds: 10,
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

	// Extract item_id for deletion test
	firstItem := items[0].(map[string]any)
	itemID := firstItem["item_id"].(string)

	// 3. Query playback state via GET /api/v1/windows/1/playback
	reqState := httptest.NewRequest(http.MethodGet, "/api/v1/windows/1/playback", nil)
	recState := httptest.NewRecorder()
	router.ServeHTTP(recState, reqState)

	if recState.Code != http.StatusOK {
		t.Fatalf("expected status 200 on playback state, got %d: %s", recState.Code, recState.Body.String())
	}

	var stateResp utils.APIResponse
	_ = json.NewDecoder(recState.Body).Decode(&stateResp)
	stateMap := stateResp.Data.(map[string]any)
	if stateMap["media_key"] != "M1" {
		t.Errorf("expected active media_key M1, got %v", stateMap["media_key"])
	}

	// 4. Update playlist batch via PUT /api/v1/windows/1/playlist
	putBody := `{"items":[{"media_key":"M2","type":"image","url":"https://example.com/m2.jpg","duration_seconds":15}]}`
	reqPut := httptest.NewRequest(http.MethodPut, "/api/v1/windows/1/playlist", bytes.NewBufferString(putBody))
	recPut := httptest.NewRecorder()
	router.ServeHTTP(recPut, reqPut)

	if recPut.Code != http.StatusOK {
		t.Fatalf("expected status 200 on PUT playlist, got %d: %s", recPut.Code, recPut.Body.String())
	}

	// 5. Remove item from playlist via DELETE /api/v1/windows/1/playlist/{itemId}
	reqDel := httptest.NewRequest(http.MethodDelete, "/api/v1/windows/1/playlist/"+itemID, nil)
	recDel := httptest.NewRecorder()
	router.ServeHTTP(recDel, reqDel)

	// Note: itemID was replaced by PUT, so deleting old itemID should return 404
	if recDel.Code != http.StatusNotFound {
		t.Errorf("expected status 404 on deleting non-existent itemID, got %d", recDel.Code)
	}
}

func TestSyncEndpoints(t *testing.T) {
	ctx := context.Background()
	router, mediaRepo, _, _, _ := setupTestRouter(nil)

	_ = mediaRepo.Create(ctx, &models.Media{
		MediaKey:        "M2",
		Name:            "Sync Banner",
		Type:            models.MediaTypeImage,
		URL:             "https://example.com/m2.jpg",
		DurationSeconds: 15,
	})

	// 1. POST /api/v1/sync (Trigger Sync)
	syncBody := `{"media_key":"M2","duration_seconds":15,"lead_time_ms":1000}`
	reqSync := httptest.NewRequest(http.MethodPost, "/api/v1/sync", bytes.NewBufferString(syncBody))
	recSync := httptest.NewRecorder()
	router.ServeHTTP(recSync, reqSync)

	if recSync.Code != http.StatusCreated {
		t.Fatalf("expected 201 on trigger sync, got %d: %s", recSync.Code, recSync.Body.String())
	}

	var syncResp utils.APIResponse
	_ = json.NewDecoder(recSync.Body).Decode(&syncResp)
	eventMap := syncResp.Data.(map[string]any)
	eventID := eventMap["event_id"].(string)

	// 2. GET /api/v1/sync/current
	reqCurrent := httptest.NewRequest(http.MethodGet, "/api/v1/sync/current", nil)
	recCurrent := httptest.NewRecorder()
	router.ServeHTTP(recCurrent, reqCurrent)

	if recCurrent.Code != http.StatusOK {
		t.Fatalf("expected 200 on get active sync, got %d", recCurrent.Code)
	}

	// 3. GET /api/v1/sync/{id}
	reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/sync/"+eventID, nil)
	recGet := httptest.NewRecorder()
	router.ServeHTTP(recGet, reqGet)

	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200 on get sync by id, got %d", recGet.Code)
	}

	// 4. POST /api/v1/sync/{id}/cancel
	reqCancel := httptest.NewRequest(http.MethodPost, "/api/v1/sync/"+eventID+"/cancel", nil)
	recCancel := httptest.NewRecorder()
	router.ServeHTTP(recCancel, reqCancel)

	if recCancel.Code != http.StatusOK {
		t.Fatalf("expected 200 on cancel sync, got %d", recCancel.Code)
	}
}
