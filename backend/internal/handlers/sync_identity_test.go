package handlers

// sync_identity_test.go
//
// Regression tests for sync-event identifier consistency.
//
// Contract:
//  - event_id (e.g. "sync_19d6a14e") is the sole canonical public identifier.
//  - Internal MongoDB _id is NOT exposed as an ambiguous "id" field in the public JSON API.
//  - GET /api/v1/sync/{event_id} retrieves by event_id.
//  - POST /api/v1/sync/{event_id}/cancel cancels by event_id.
//  - GET /api/v1/sync/current exposes event_id.
//  - MongoDB ObjectID hexes are NOT accepted as event_id lookups (returns 404).

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/eva-bharat/media-sequencer/backend/internal/config"
	"github.com/eva-bharat/media-sequencer/backend/internal/models"
	"github.com/eva-bharat/media-sequencer/backend/internal/repository"
	"github.com/eva-bharat/media-sequencer/backend/internal/service"
	"github.com/eva-bharat/media-sequencer/backend/internal/utils"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type syncIdentityFixture struct {
	router      http.Handler
	syncRepo    *repository.MockSyncRepository
	mediaRepo   *repository.MockMediaRepository
	coordinator service.SyncCoordinator
}

func setupSyncIdentityFixture(t *testing.T) *syncIdentityFixture {
	t.Helper()
	ctx := context.Background()

	cfg := &config.Config{
		Port:               "8080",
		Environment:        "test",
		CORSAllowedOrigins: []string{"http://localhost:3000"},
	}

	mediaRepo := repository.NewMockMediaRepository()
	windowRepo := repository.NewMockWindowRepository()
	playlistRepo := repository.NewMockPlaylistRepository()
	syncRepo := repository.NewMockSyncRepository()

	// Seed Media M2 for synchronization
	if err := mediaRepo.Create(ctx, &models.Media{
		MediaKey:        "M2",
		Name:            "Emergency Broadcast",
		Type:            models.MediaTypeVideo,
		URL:             "https://cdn.example.com/m2.mp4",
		DurationSeconds: 30,
	}); err != nil {
		t.Fatalf("seed M2: %v", err)
	}

	// Seed Window 1
	wID := bson.NewObjectID()
	if err := windowRepo.Create(ctx, &models.Window{
		ID:                   wID,
		WindowNumber:         1,
		Name:                 "Window 1",
		CycleDurationSeconds: 18000,
		CycleStartTime:       time.Now().UTC(),
		IsActive:             true,
	}); err != nil {
		t.Fatalf("seed window 1: %v", err)
	}

	playlistSvc := service.NewPlaylistService(playlistRepo, mediaRepo, windowRepo)
	coordinator := service.NewSyncCoordinator(syncRepo, mediaRepo, windowRepo, playlistRepo, playlistSvc, nil)

	deps := Dependencies{
		Config:          cfg,
		Pinger:          &mockPinger{},
		WindowService:   service.NewWindowService(windowRepo),
		MediaService:    service.NewMediaService(mediaRepo),
		PlaylistService: playlistSvc,
		SyncService:     coordinator,
	}

	return &syncIdentityFixture{
		router:      NewRouter(deps),
		syncRepo:    syncRepo,
		mediaRepo:   mediaRepo,
		coordinator: coordinator,
	}
}

// Helper: post JSON and return recorder
func postSyncJSON(router http.Handler, path, jsonBody string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// Helper: get request and return recorder
func getSyncReq(router http.Handler, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 1: Creation — Verify HTTP 201, event_id returned, no ambiguous id
// ─────────────────────────────────────────────────────────────────────────────
func TestSyncIdentity_Creation(t *testing.T) {
	f := setupSyncIdentityFixture(t)

	body := `{"media_key":"M2","duration_seconds":30,"lead_time_ms":500,"triggered_by":"operator"}`
	rec := postSyncJSON(f.router, "/api/v1/sync", body)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected HTTP 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode JSON response: %v", err)
	}

	if resp["success"] != true {
		t.Fatalf("expected success=true, got %v", resp["success"])
	}

	data, ok := resp["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data object in response, got %T", resp["data"])
	}

	// 1. event_id must be present and well-formed
	eventID, ok := data["event_id"].(string)
	if !ok || !strings.HasPrefix(eventID, "sync_") {
		t.Errorf("expected valid event_id starting with 'sync_', got: %v", data["event_id"])
	}

	// 2. MongoDB internal _id must NOT be exposed as 'id' in the public JSON
	if val, exists := data["id"]; exists {
		t.Errorf("expected 'id' field to NOT exist in public sync JSON response, but found: %v", val)
	}

	// 3. Verify public identifier works with subsequent endpoint
	getRec := getSyncReq(f.router, "/api/v1/sync/"+eventID)
	if getRec.Code != http.StatusOK {
		t.Errorf("expected subsequent GET with returned event_id to return 200, got %d", getRec.Code)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 2: Retrieval — GET /api/v1/sync/{event_id} returns HTTP 200 and same event_id
// ─────────────────────────────────────────────────────────────────────────────
func TestSyncIdentity_Retrieval(t *testing.T) {
	f := setupSyncIdentityFixture(t)

	// Create sync event
	body := `{"media_key":"M2","duration_seconds":30,"lead_time_ms":100}`
	createRec := postSyncJSON(f.router, "/api/v1/sync", body)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("failed to create sync event: %d", createRec.Code)
	}

	var createResp utils.APIResponse
	_ = json.NewDecoder(createRec.Body).Decode(&createResp)
	createData := createResp.Data.(map[string]any)
	eventID := createData["event_id"].(string)

	// Retrieve by event_id
	getRec := getSyncReq(f.router, "/api/v1/sync/"+eventID)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", getRec.Code, getRec.Body.String())
	}

	var getResp utils.APIResponse
	_ = json.NewDecoder(getRec.Body).Decode(&getResp)
	getData := getResp.Data.(map[string]any)

	if getData["event_id"] != eventID {
		t.Errorf("expected event_id %s, got %v", eventID, getData["event_id"])
	}

	// Ensure internal MongoDB _id is not in retrieval JSON either
	if val, exists := getData["id"]; exists {
		t.Errorf("expected 'id' to NOT be in retrieval response, got: %v", val)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 3: Cancellation — POST /api/v1/sync/{event_id}/cancel
// ─────────────────────────────────────────────────────────────────────────────
func TestSyncIdentity_Cancellation(t *testing.T) {
	f := setupSyncIdentityFixture(t)

	// Create a long sync event (300 seconds)
	body := `{"media_key":"M2","duration_seconds":300,"lead_time_ms":0}`
	createRec := postSyncJSON(f.router, "/api/v1/sync", body)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create sync failed: %d", createRec.Code)
	}

	var createResp utils.APIResponse
	_ = json.NewDecoder(createRec.Body).Decode(&createResp)
	eventID := createResp.Data.(map[string]any)["event_id"].(string)

	// Verify active sync is currently running
	currRec := getSyncReq(f.router, "/api/v1/sync/current")
	if currRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on /sync/current, got %d", currRec.Code)
	}
	var currResp utils.APIResponse
	_ = json.NewDecoder(currRec.Body).Decode(&currResp)
	if currResp.Data == nil {
		t.Fatal("expected active sync event, got null")
	}

	// Cancel the sync event
	cancelRec := postSyncJSON(f.router, "/api/v1/sync/"+eventID+"/cancel", "")
	if cancelRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on cancel, got %d: %s", cancelRec.Code, cancelRec.Body.String())
	}

	var cancelResp utils.APIResponse
	_ = json.NewDecoder(cancelRec.Body).Decode(&cancelResp)
	cancelData := cancelResp.Data.(map[string]any)

	if cancelData["status"] != "CANCELLED" {
		t.Errorf("expected status 'CANCELLED', got %v", cancelData["status"])
	}
	if cancelData["event_id"] != eventID {
		t.Errorf("expected event_id %s, got %v", eventID, cancelData["event_id"])
	}

	// Verify /api/v1/sync/current no longer returns the cancelled event
	afterCancelRec := getSyncReq(f.router, "/api/v1/sync/current")
	if afterCancelRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on /sync/current after cancel, got %d", afterCancelRec.Code)
	}
	var afterResp utils.APIResponse
	_ = json.NewDecoder(afterCancelRec.Body).Decode(&afterResp)
	if afterResp.Data != nil {
		t.Errorf("expected active sync to be null after cancellation, got: %+v", afterResp.Data)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 4: Invalid Identifier — Nonexistent event ID returns HTTP 404
// ─────────────────────────────────────────────────────────────────────────────
func TestSyncIdentity_InvalidIdentifier(t *testing.T) {
	f := setupSyncIdentityFixture(t)

	// 1. GET with nonexistent event_id
	getRec := getSyncReq(f.router, "/api/v1/sync/sync_nonexistent9999")
	if getRec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for nonexistent GET, got %d: %s", getRec.Code, getRec.Body.String())
	}

	// 2. POST cancel with nonexistent event_id
	cancelRec := postSyncJSON(f.router, "/api/v1/sync/sync_nonexistent9999/cancel", "")
	if cancelRec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for nonexistent CANCEL, got %d: %s", cancelRec.Code, cancelRec.Body.String())
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 5: MongoDB Identity — MongoDB _id is not accepted as public identifier
// ─────────────────────────────────────────────────────────────────────────────
func TestSyncIdentity_MongoDBIdentityNotAccepted(t *testing.T) {
	f := setupSyncIdentityFixture(t)

	// Create an event
	body := `{"media_key":"M2","duration_seconds":60,"lead_time_ms":100}`
	createRec := postSyncJSON(f.router, "/api/v1/sync", body)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create failed: %d", createRec.Code)
	}

	var createResp utils.APIResponse
	_ = json.NewDecoder(createRec.Body).Decode(&createResp)
	eventID := createResp.Data.(map[string]any)["event_id"].(string)

	// Fetch event directly from mock repo to inspect its internal MongoDB _id
	storedEvent, err := f.syncRepo.FindByID(context.Background(), eventID)
	if err != nil {
		t.Fatalf("failed to retrieve stored event from repo: %v", err)
	}
	mongoHex := storedEvent.ID.Hex()

	// Ensure the internal MongoDB _id is different from the public event_id
	if mongoHex == eventID {
		t.Fatalf("internal MongoDB ID should not match public event_id")
	}

	// 1. Attempting GET using MongoDB _id hex must return 404 NOT_FOUND
	getMongoRec := getSyncReq(f.router, "/api/v1/sync/"+mongoHex)
	if getMongoRec.Code != http.StatusNotFound {
		t.Errorf("expected 404 when querying by internal MongoDB _id hex, got %d: %s",
			getMongoRec.Code, getMongoRec.Body.String())
	}

	// 2. Attempting CANCEL using MongoDB _id hex must return 404 NOT_FOUND
	cancelMongoRec := postSyncJSON(f.router, "/api/v1/sync/"+mongoHex+"/cancel", "")
	if cancelMongoRec.Code != http.StatusNotFound {
		t.Errorf("expected 404 when cancelling by internal MongoDB _id hex, got %d: %s",
			cancelMongoRec.Code, cancelMongoRec.Body.String())
	}

	// 3. Verifying repository FindByID with mongoHex directly returns ErrNotFound
	_, repoErr := f.syncRepo.FindByID(context.Background(), mongoHex)
	if repoErr == nil {
		t.Errorf("expected repository FindByID with mongoHex to fail, but it succeeded")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Test 6: Lifecycle — CREATE → GET → CURRENT → CANCEL → CURRENT
// ─────────────────────────────────────────────────────────────────────────────
func TestSyncIdentity_FullLifecycle(t *testing.T) {
	f := setupSyncIdentityFixture(t)

	// 1. CREATE
	createRec := postSyncJSON(f.router, "/api/v1/sync", `{"media_key":"M2","duration_seconds":120,"lead_time_ms":0}`)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("Step 1 (CREATE) failed: %d — %s", createRec.Code, createRec.Body.String())
	}
	var createResp utils.APIResponse
	_ = json.NewDecoder(createRec.Body).Decode(&createResp)
	createData := createResp.Data.(map[string]any)
	canonicalEventID := createData["event_id"].(string)

	if !strings.HasPrefix(canonicalEventID, "sync_") {
		t.Fatalf("Step 1: invalid event_id %s", canonicalEventID)
	}

	// 2. GET by event_id
	getRec := getSyncReq(f.router, fmt.Sprintf("/api/v1/sync/%s", canonicalEventID))
	if getRec.Code != http.StatusOK {
		t.Fatalf("Step 2 (GET) failed: %d — %s", getRec.Code, getRec.Body.String())
	}
	var getResp utils.APIResponse
	_ = json.NewDecoder(getRec.Body).Decode(&getResp)
	getData := getResp.Data.(map[string]any)
	if getData["event_id"] != canonicalEventID {
		t.Fatalf("Step 2: event_id mismatch: expected %s, got %v", canonicalEventID, getData["event_id"])
	}

	// 3. CURRENT
	currRec := getSyncReq(f.router, "/api/v1/sync/current")
	if currRec.Code != http.StatusOK {
		t.Fatalf("Step 3 (CURRENT) failed: %d — %s", currRec.Code, currRec.Body.String())
	}
	var currResp utils.APIResponse
	_ = json.NewDecoder(currRec.Body).Decode(&currResp)
	currData := currResp.Data.(map[string]any)
	if currData["event_id"] != canonicalEventID {
		t.Fatalf("Step 3: active sync event_id mismatch: expected %s, got %v", canonicalEventID, currData["event_id"])
	}

	// 4. CANCEL
	cancelRec := postSyncJSON(f.router, fmt.Sprintf("/api/v1/sync/%s/cancel", canonicalEventID), "")
	if cancelRec.Code != http.StatusOK {
		t.Fatalf("Step 4 (CANCEL) failed: %d — %s", cancelRec.Code, cancelRec.Body.String())
	}
	var cancelResp utils.APIResponse
	_ = json.NewDecoder(cancelRec.Body).Decode(&cancelResp)
	cancelData := cancelResp.Data.(map[string]any)
	if cancelData["event_id"] != canonicalEventID {
		t.Fatalf("Step 4: cancel event_id mismatch: expected %s, got %v", canonicalEventID, cancelData["event_id"])
	}
	if cancelData["status"] != "CANCELLED" {
		t.Fatalf("Step 4: expected status CANCELLED, got %v", cancelData["status"])
	}

	// 5. CURRENT (after cancellation)
	afterCurrRec := getSyncReq(f.router, "/api/v1/sync/current")
	if afterCurrRec.Code != http.StatusOK {
		t.Fatalf("Step 5 (CURRENT after cancel) failed: %d", afterCurrRec.Code)
	}
	var afterCurrResp utils.APIResponse
	_ = json.NewDecoder(afterCurrRec.Body).Decode(&afterCurrResp)
	if afterCurrResp.Data != nil {
		t.Fatalf("Step 5: expected active sync to be nil after cancel, got: %+v", afterCurrResp.Data)
	}
}
