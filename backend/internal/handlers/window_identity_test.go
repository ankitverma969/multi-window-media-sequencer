package handlers

// window_identity_test.go
//
// Regression tests for the window_id / window_number consistency bug.
//
// Root cause: playlist documents could store a stale (or zero-value) window_id
// that did not match the canonical Window MongoDB ObjectID. GET /playlist was
// the only operation that searched by window_id directly; all mutations went
// via window_number. After a DELETE the next GET by Window ID failed with 404.
//
// Fix applied:
//  1. GetPlaylist now resolves the Window first (via windowRepo) and then looks
//     up by window_number — consistent with every mutating method.
//  2. AppendItem / RemoveItem / UpdateItems now write the canonical window_id to
//     MongoDB in every $set operation.
//  3. Mock implementations updated to accept and store windowID.
//  4. Seed logic reads back the window after Upsert to obtain its actual _id.
//
// These tests exercise all 6 scenarios from the bug report.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/eva-bharat/media-sequencer/backend/internal/config"
	"github.com/eva-bharat/media-sequencer/backend/internal/models"
	"github.com/eva-bharat/media-sequencer/backend/internal/repository"
	"github.com/eva-bharat/media-sequencer/backend/internal/service"
	"github.com/eva-bharat/media-sequencer/backend/internal/utils"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// ─── test server setup ────────────────────────────────────────────────────────

type windowIdentityFixture struct {
	router       http.Handler
	mediaRepo    *repository.MockMediaRepository
	windowRepo   *repository.MockWindowRepository
	playlistRepo *repository.MockPlaylistRepository
	syncRepo     *repository.MockSyncRepository
	// The canonical Window 1 ObjectID as stored by the repository
	canonicalWindowID bson.ObjectID
}

// setupWindowIdentityFixture seeds Window 1 with a known canonical ObjectID,
// seeds M4 media, and seeds an empty playlist whose window_id is set to the
// canonical Window ID.
func setupWindowIdentityFixture(t *testing.T) *windowIdentityFixture {
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

	// Seed Window 1 with an explicit canonical ObjectID
	canonicalID := bson.NewObjectID()
	if err := windowRepo.Create(ctx, &models.Window{
		ID:                   canonicalID,
		WindowNumber:         1,
		Name:                 "Display 1",
		CycleDurationSeconds: 18000,
		CycleStartTime:       time.Now().UTC(),
		IsActive:             true,
	}); err != nil {
		t.Fatalf("seed window: %v", err)
	}

	// Seed M4 media
	if err := mediaRepo.Create(ctx, &models.Media{
		MediaKey:        "M4",
		Name:            "City Timelapse",
		Type:            models.MediaTypeVideo,
		URL:             "https://cdn.example.com/m4.mp4",
		DurationSeconds: 12,
	}); err != nil {
		t.Fatalf("seed M4: %v", err)
	}

	// Seed an empty playlist with the CORRECT canonical window_id
	if err := playlistRepo.Upsert(ctx, &models.Playlist{
		WindowID:     canonicalID, // correct canonical ID
		WindowNumber: 1,
		Items:        []models.PlaylistItem{},
	}); err != nil {
		t.Fatalf("seed playlist: %v", err)
	}

	deps := Dependencies{
		Config:          cfg,
		Pinger:          &mockPinger{},
		WindowService:   service.NewWindowService(windowRepo),
		MediaService:    service.NewMediaService(mediaRepo),
		PlaylistService: service.NewPlaylistService(playlistRepo, mediaRepo, windowRepo),
		SyncService:     service.NewSyncService(syncRepo, mediaRepo),
	}

	return &windowIdentityFixture{
		router:            NewRouter(deps),
		mediaRepo:         mediaRepo,
		windowRepo:        windowRepo,
		playlistRepo:      playlistRepo,
		syncRepo:          syncRepo,
		canonicalWindowID: canonicalID,
	}
}

// ─── Test 1 — GET playlist by Window ObjectID ──────────────────────────────

func TestWindowIdentity_GetPlaylistByWindowID(t *testing.T) {
	f := setupWindowIdentityFixture(t)

	path := fmt.Sprintf("/api/v1/windows/%s/playlist", f.canonicalWindowID.Hex())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	f.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET playlist by Window ObjectID: expected 200, got %d — %s", rec.Code, rec.Body.String())
	}

	var resp utils.APIResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if !resp.Success {
		t.Fatal("expected success=true")
	}
}

// ─── Test 2 — GET playlist by window number ────────────────────────────────

func TestWindowIdentity_GetPlaylistByWindowNumber(t *testing.T) {
	f := setupWindowIdentityFixture(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/windows/1/playlist", nil)
	f.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET playlist by window number: expected 200, got %d — %s", rec.Code, rec.Body.String())
	}

	var resp utils.APIResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if !resp.Success {
		t.Fatal("expected success=true")
	}
}

// ─── Test 3 — POST item via Window ObjectID preserves canonical identity ───

func TestWindowIdentity_AddItemPreservesWindowID(t *testing.T) {
	f := setupWindowIdentityFixture(t)

	path := fmt.Sprintf("/api/v1/windows/%s/playlist/items", f.canonicalWindowID.Hex())
	body := `{"media_key":"M4","custom_duration_seconds":12}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	f.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST item by Window ObjectID: expected 201, got %d — %s", rec.Code, rec.Body.String())
	}

	var resp utils.APIResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if !resp.Success {
		t.Fatalf("expected success=true, got false — %+v", resp.Error)
	}

	pMap, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected data to be map, got %T", resp.Data)
	}

	// window_number must be 1
	wn, _ := pMap["window_number"].(float64)
	if int(wn) != 1 {
		t.Errorf("expected window_number=1, got %v", pMap["window_number"])
	}

	// window_id in response must equal canonical Window ObjectID
	respWindowID, _ := pMap["window_id"].(string)
	if respWindowID != f.canonicalWindowID.Hex() {
		t.Errorf("response window_id=%s does not match canonical window ID=%s",
			respWindowID, f.canonicalWindowID.Hex())
	}

	// M4 must be present in items
	items, ok := pMap["items"].([]any)
	if !ok || len(items) == 0 {
		t.Fatal("expected at least 1 item in playlist after POST")
	}
	firstItem := items[0].(map[string]any)
	if firstItem["media_key"] != "M4" {
		t.Errorf("expected media_key=M4, got %v", firstItem["media_key"])
	}
}

// ─── Test 4 — DELETE item then GET by Window ObjectID succeeds ────────────

func TestWindowIdentity_DeleteItemThenGetByWindowIDSucceeds(t *testing.T) {
	f := setupWindowIdentityFixture(t)

	// Step 1: Add M4 via Window ObjectID
	addPath := fmt.Sprintf("/api/v1/windows/%s/playlist/items", f.canonicalWindowID.Hex())
	addRec := httptest.NewRecorder()
	addReq := httptest.NewRequest(http.MethodPost, addPath,
		bytes.NewBufferString(`{"media_key":"M4","custom_duration_seconds":12}`))
	addReq.Header.Set("Content-Type", "application/json")
	f.router.ServeHTTP(addRec, addReq)

	if addRec.Code != http.StatusCreated {
		t.Fatalf("add item: expected 201, got %d — %s", addRec.Code, addRec.Body.String())
	}

	// Extract item_id from data.items[]
	var addResp utils.APIResponse
	_ = json.NewDecoder(addRec.Body).Decode(&addResp)
	pMap := addResp.Data.(map[string]any)
	items := pMap["items"].([]any)
	firstItem := items[0].(map[string]any)
	itemID := firstItem["item_id"].(string)

	// Step 2: DELETE via Window ObjectID
	delPath := fmt.Sprintf("/api/v1/windows/%s/playlist/%s", f.canonicalWindowID.Hex(), itemID)
	delRec := httptest.NewRecorder()
	delReq := httptest.NewRequest(http.MethodDelete, delPath, nil)
	f.router.ServeHTTP(delRec, delReq)

	if delRec.Code != http.StatusOK {
		t.Fatalf("DELETE item: expected 200, got %d — %s", delRec.Code, delRec.Body.String())
	}

	// Step 3: GET playlist by Window ObjectID — MUST succeed (this is the bug scenario)
	getRec := httptest.NewRecorder()
	getReq := httptest.NewRequest(http.MethodGet,
		fmt.Sprintf("/api/v1/windows/%s/playlist", f.canonicalWindowID.Hex()), nil)
	f.router.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("GET after DELETE by Window ObjectID: expected 200, got %d — %s (THIS IS THE BUG SCENARIO)",
			getRec.Code, getRec.Body.String())
	}

	var getResp utils.APIResponse
	_ = json.NewDecoder(getRec.Body).Decode(&getResp)
	if !getResp.Success {
		t.Fatal("GET after DELETE: expected success=true")
	}

	// M4 must no longer appear in the playlist
	afterMap := getResp.Data.(map[string]any)
	afterItems := afterMap["items"]
	if afterItems != nil {
		afterList := afterItems.([]any)
		if len(afterList) != 0 {
			t.Errorf("expected 0 items after DELETE, got %d", len(afterList))
		}
	}
}

// ─── Test 5 — Stale window_id in legacy playlist (robustness) ────────────

// TestWindowIdentity_StaleWindowIDHandledRobustly seeds a playlist with an
// intentionally wrong window_id but a correct window_number, then verifies
// that all API operations still succeed via the window-number–first lookup.
func TestWindowIdentity_StaleWindowIDHandledRobustly(t *testing.T) {
	ctx := context.Background()

	cfg := &config.Config{
		Port: "8080", Environment: "test",
		CORSAllowedOrigins: []string{"http://localhost:3000"},
	}

	mediaRepo := repository.NewMockMediaRepository()
	windowRepo := repository.NewMockWindowRepository()
	playlistRepo := repository.NewMockPlaylistRepository()
	syncRepo := repository.NewMockSyncRepository()

	// Create window 1 with a known canonical ID
	canonicalID := bson.NewObjectID()
	_ = windowRepo.Create(ctx, &models.Window{
		ID:                   canonicalID,
		WindowNumber:         1,
		Name:                 "Display 1",
		CycleDurationSeconds: 18000,
		CycleStartTime:       time.Now().UTC(),
		IsActive:             true,
	})

	// Seed M4
	_ = mediaRepo.Create(ctx, &models.Media{
		MediaKey: "M4", Name: "City", Type: models.MediaTypeVideo,
		URL: "https://cdn.example.com/m4.mp4", DurationSeconds: 12,
	})

	// Deliberately seed playlist with a WRONG window_id (stale from before the fix)
	staleID := bson.NewObjectID() // this is NOT the canonical window ID
	_ = playlistRepo.Upsert(ctx, &models.Playlist{
		WindowID:     staleID, // stale / wrong ID
		WindowNumber: 1,       // but correct window_number
		Items:        []models.PlaylistItem{},
	})

	deps := Dependencies{
		Config:          cfg,
		Pinger:          &mockPinger{},
		WindowService:   service.NewWindowService(windowRepo),
		MediaService:    service.NewMediaService(mediaRepo),
		PlaylistService: service.NewPlaylistService(playlistRepo, mediaRepo, windowRepo),
		SyncService:     service.NewSyncService(syncRepo, mediaRepo),
	}
	router := NewRouter(deps)

	// GET by window number — must succeed even with stale window_id
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/windows/1/playlist", nil)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET by number with stale window_id: expected 200, got %d — %s", rec.Code, rec.Body.String())
	}

	// GET by canonical Window ObjectID — must also succeed (service resolves
	// via window_number, so stale window_id in playlist is irrelevant)
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet,
		fmt.Sprintf("/api/v1/windows/%s/playlist", canonicalID.Hex()), nil)
	router.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("GET by canonical Window ObjectID with stale playlist window_id: expected 200, got %d — %s",
			rec2.Code, rec2.Body.String())
	}

	// Add item via canonical Window ObjectID — must succeed and heal window_id
	rec3 := httptest.NewRecorder()
	req3 := httptest.NewRequest(http.MethodPost,
		fmt.Sprintf("/api/v1/windows/%s/playlist/items", canonicalID.Hex()),
		bytes.NewBufferString(`{"media_key":"M4","custom_duration_seconds":12}`))
	req3.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusCreated {
		t.Fatalf("POST item with stale playlist window_id: expected 201, got %d — %s", rec3.Code, rec3.Body.String())
	}

	// Response must carry the canonical window_id (healed by the service layer)
	var addResp utils.APIResponse
	_ = json.NewDecoder(rec3.Body).Decode(&addResp)
	pMap := addResp.Data.(map[string]any)
	respWindowID, _ := pMap["window_id"].(string)
	if respWindowID != canonicalID.Hex() {
		t.Errorf("expected response window_id=%s (canonical), got %s (stale or zero)",
			canonicalID.Hex(), respWindowID)
	}
}

// ─── Test 6 — Invalid window returns 404 ──────────────────────────────────

func TestWindowIdentity_InvalidWindowIDReturns404(t *testing.T) {
	f := setupWindowIdentityFixture(t)

	nonExistentID := bson.NewObjectID().Hex() // valid ObjectID but no such window

	// GET by non-existent Window ObjectID
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet,
		fmt.Sprintf("/api/v1/windows/%s/playlist", nonExistentID), nil)
	f.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("GET non-existent window by ObjectID: expected 404, got %d — %s", rec.Code, rec.Body.String())
	}

	var resp utils.APIResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Success {
		t.Error("expected success=false for non-existent window")
	}
	if resp.Error == nil {
		t.Error("expected error field in response")
	}

	// GET by non-existent window number
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/windows/9999/playlist", nil)
	f.router.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusNotFound {
		t.Errorf("GET non-existent window by number: expected 404, got %d — %s", rec2.Code, rec2.Body.String())
	}
}
