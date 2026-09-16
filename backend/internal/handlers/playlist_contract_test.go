package handlers

// playlist_contract_test.go
//
// Comprehensive contract tests for all playlist-related endpoints.
//
// Key finding: POST /api/v1/windows/{id}/playlist returns the complete updated
// *models.Playlist inside data{}. The newly created item's item_id is found at
// data.items[].item_id — NOT at data.item_id.
// This is CORRECT REST semantics (the resource is the playlist, not the item).
// The test script that expected data.item_id was wrong, not the API.
//
// These tests verify the actual contract and perform full cleanup so no
// test-created data persists in the database.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/eva-bharat/media-sequencer/backend/internal/models"
	"github.com/eva-bharat/media-sequencer/backend/internal/utils"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

// setupPlaylistContractServer seeds Window 1 with media M1–M4 and an empty
// playlist, then returns the test router.
func setupPlaylistContractServer(t *testing.T) http.Handler {
	t.Helper()
	ctx := context.Background()
	router, mediaRepo, windowRepo, playlistRepo, _ := setupTestRouter(nil)

	// Window 1
	if err := windowRepo.Create(ctx, &models.Window{
		WindowNumber:         1,
		Name:                 "Display 1",
		CycleDurationSeconds: 18000,
		CycleStartTime:       time.Now().UTC(),
		IsActive:             true,
	}); err != nil {
		t.Fatalf("seed window: %v", err)
	}

	// Media catalog
	medias := []models.Media{
		{MediaKey: "M1", Name: "Nature 4K", Type: models.MediaTypeVideo, URL: "https://cdn.example.com/m1.mp4", DurationSeconds: 30},
		{MediaKey: "M2", Name: "Promo Banner", Type: models.MediaTypeImage, URL: "https://cdn.example.com/m2.jpg", DurationSeconds: 15},
		{MediaKey: "M3", Name: "Blank Slate", Type: models.MediaTypeBlank, URL: "", DurationSeconds: 5},
		{MediaKey: "M4", Name: "City Timelapse", Type: models.MediaTypeVideo, URL: "https://cdn.example.com/m4.mp4", DurationSeconds: 45},
	}
	for _, m := range medias {
		mc := m
		if err := mediaRepo.Create(ctx, &mc); err != nil {
			t.Fatalf("seed media %s: %v", m.MediaKey, err)
		}
	}

	// Empty playlist for window 1
	if err := playlistRepo.Upsert(ctx, &models.Playlist{
		WindowNumber: 1,
		Items:        []models.PlaylistItem{},
	}); err != nil {
		t.Fatalf("seed playlist: %v", err)
	}

	return router
}

// postJSON sends a POST request and returns the response recorder.
func postJSON(router http.Handler, path, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	return rec
}

// putJSON sends a PUT request and returns the response recorder.
func putJSON(router http.Handler, path, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	return rec
}

// deleteReq sends a DELETE request and returns the response recorder.
func deleteReq(router http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, path, nil)
	router.ServeHTTP(rec, req)
	return rec
}

// getReq sends a GET request and returns the response recorder.
func getReq(router http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	router.ServeHTTP(rec, req)
	return rec
}

// decodeAPIResponse decodes the standard envelope from a recorder.
func decodeAPIResponse(t *testing.T, rec *httptest.ResponseRecorder) utils.APIResponse {
	t.Helper()
	var resp utils.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode APIResponse: %v — body: %s", err, rec.Body.String())
	}
	return resp
}

// extractPlaylistFromData casts resp.Data into the map structure that JSON
// deserialises *models.Playlist into when using any.
func extractPlaylistFromData(t *testing.T, resp utils.APIResponse) map[string]any {
	t.Helper()
	pMap, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected data to be map[string]any, got %T: %+v", resp.Data, resp.Data)
	}
	return pMap
}

// extractItems returns data.items[] from a playlist response.
// When all items are removed the underlying slice may be nil (JSON null),
// which is treated as an empty list.
func extractItems(t *testing.T, pMap map[string]any) []any {
	t.Helper()
	raw, ok := pMap["items"]
	if !ok {
		t.Fatal("data.items field missing from playlist response")
	}
	// nil (JSON null) is valid when the playlist is empty after all items removed
	if raw == nil {
		return []any{}
	}
	items, ok := raw.([]any)
	if !ok {
		t.Fatalf("data.items is not []any, got %T", raw)
	}
	return items
}

// ─── Task 2: Correct POST playlist contract test ──────────────────────────────

// TestPlaylistPOST_AddItem_FullLifecycle verifies the full add→inspect→delete
// lifecycle and ensures:
//  1. HTTP 201 and success:true on POST.
//  2. The response is the complete updated playlist (not just the item).
//  3. The newly added item's item_id lives at data.items[].item_id.
//  4. The item can be looked up in a subsequent GET.
//  5. The item can be deleted with DELETE using that exact item_id.
//  6. The playlist is restored to its original empty state after deletion.
//
// No test data is left in the database.
func TestPlaylistPOST_AddItem_FullLifecycle(t *testing.T) {
	router := setupPlaylistContractServer(t)

	// ── Step 1: POST a valid item ─────────────────────────────────────────
	addRec := postJSON(router, "/api/v1/windows/1/playlist/items",
		`{"media_key":"M2","custom_duration_seconds":15}`)

	if addRec.Code != http.StatusCreated {
		t.Fatalf("POST playlist item: expected 201 Created, got %d — %s", addRec.Code, addRec.Body.String())
	}

	// ── Step 2: Decode response envelope ─────────────────────────────────
	addResp := decodeAPIResponse(t, addRec)
	if !addResp.Success {
		t.Fatalf("POST playlist item: expected success=true, got false — error: %+v", addResp.Error)
	}
	if addResp.Data == nil {
		t.Fatal("POST playlist item: expected non-nil data field")
	}

	// ── Step 3: Verify response is a Playlist (not just the added item) ──
	// The API correctly returns the full updated Playlist, consistent with
	// standard REST semantics (resource = playlist, not the individual item).
	// The newly added item's item_id is inside data.items[].item_id.
	pMap := extractPlaylistFromData(t, addResp)

	windowNum, _ := pMap["window_number"].(float64)
	if int(windowNum) != 1 {
		t.Errorf("expected window_number=1 in playlist response, got %v", pMap["window_number"])
	}

	items := extractItems(t, pMap)
	if len(items) != 1 {
		t.Fatalf("expected exactly 1 item in playlist after add, got %d", len(items))
	}

	// ── Step 4: Extract item_id from data.items[0].item_id ───────────────
	firstItem, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("items[0] is not a map, got %T", items[0])
	}

	itemID, ok := firstItem["item_id"].(string)
	if !ok || itemID == "" {
		t.Fatalf("expected non-empty item_id in data.items[0], got %v", firstItem["item_id"])
	}

	// Verify media_key matches what we sent
	if firstItem["media_key"] != "M2" {
		t.Errorf("expected media_key=M2 in added item, got %v", firstItem["media_key"])
	}

	// Verify duration_seconds is what we supplied
	dur, _ := firstItem["duration_seconds"].(float64)
	if int(dur) != 15 {
		t.Errorf("expected duration_seconds=15, got %v", firstItem["duration_seconds"])
	}

	// ── Step 5: Confirm via GET that the item persists ────────────────────
	getRec := getReq(router, "/api/v1/windows/1/playlist")
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET playlist: expected 200, got %d", getRec.Code)
	}
	getResp := decodeAPIResponse(t, getRec)
	getMap := extractPlaylistFromData(t, getResp)
	getItems := extractItems(t, getMap)
	if len(getItems) != 1 {
		t.Errorf("GET playlist: expected 1 item, got %d", len(getItems))
	}

	// ── Step 6: DELETE the item using the extracted item_id ───────────────
	delRec := deleteReq(router, "/api/v1/windows/1/playlist/"+itemID)
	if delRec.Code != http.StatusOK {
		t.Fatalf("DELETE playlist item: expected 200 OK, got %d — %s", delRec.Code, delRec.Body.String())
	}

	delResp := decodeAPIResponse(t, delRec)
	if !delResp.Success {
		t.Fatalf("DELETE playlist item: expected success=true, got false — %+v", delResp.Error)
	}

	// ── Step 7: Verify original (empty) playlist state is restored ────────
	afterRec := getReq(router, "/api/v1/windows/1/playlist")
	afterResp := decodeAPIResponse(t, afterRec)
	afterMap := extractPlaylistFromData(t, afterResp)
	afterItems := extractItems(t, afterMap)
	if len(afterItems) != 0 {
		t.Errorf("expected empty playlist after deletion, got %d items", len(afterItems))
	}
}

// TestPlaylistPOST_AlternatePath verifies both POST routes are equivalent.
func TestPlaylistPOST_AlternatePath(t *testing.T) {
	router := setupPlaylistContractServer(t)

	// POST via /playlist (no /items suffix — both are registered in routes.go)
	rec := postJSON(router, "/api/v1/windows/1/playlist",
		`{"media_key":"M1","custom_duration_seconds":30}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /playlist (short path): expected 201, got %d — %s", rec.Code, rec.Body.String())
	}
	resp := decodeAPIResponse(t, rec)
	if !resp.Success {
		t.Fatalf("expected success=true")
	}
	pMap := extractPlaylistFromData(t, resp)
	items := extractItems(t, pMap)
	if len(items) == 0 {
		t.Error("expected at least one item in playlist after POST via /playlist path")
	}
}

// ─── Task 3: Comprehensive error / edge-case contract tests ──────────────────

func TestPlaylistPOST_InvalidMediaKey(t *testing.T) {
	router := setupPlaylistContractServer(t)

	rec := postJSON(router, "/api/v1/windows/1/playlist/items",
		`{"media_key":"NON_EXISTENT","custom_duration_seconds":10}`)

	if rec.Code != http.StatusNotFound {
		t.Errorf("invalid media key: expected 404, got %d — %s", rec.Code, rec.Body.String())
	}
	resp := decodeAPIResponse(t, rec)
	if resp.Success {
		t.Error("expected success=false for invalid media key")
	}
	if resp.Error == nil || resp.Error.Code != "MEDIA_NOT_FOUND" {
		t.Errorf("expected MEDIA_NOT_FOUND error code, got %+v", resp.Error)
	}
}

func TestPlaylistPOST_InvalidWindow(t *testing.T) {
	router := setupPlaylistContractServer(t)

	rec := postJSON(router, "/api/v1/windows/9999/playlist/items",
		`{"media_key":"M1","custom_duration_seconds":10}`)

	if rec.Code != http.StatusNotFound {
		t.Errorf("invalid window: expected 404, got %d — %s", rec.Code, rec.Body.String())
	}
	resp := decodeAPIResponse(t, rec)
	if resp.Success {
		t.Error("expected success=false for invalid window")
	}
	if resp.Error == nil || resp.Error.Code != "WINDOW_NOT_FOUND" {
		t.Errorf("expected WINDOW_NOT_FOUND, got %+v", resp.Error)
	}
}

func TestPlaylistPOST_MissingMediaKey(t *testing.T) {
	router := setupPlaylistContractServer(t)

	rec := postJSON(router, "/api/v1/windows/1/playlist/items", `{}`)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("missing media_key: expected 400, got %d — %s", rec.Code, rec.Body.String())
	}
	resp := decodeAPIResponse(t, rec)
	if resp.Success {
		t.Error("expected success=false for missing media_key")
	}
	if resp.Error == nil {
		t.Error("expected non-nil error field")
	}
}

func TestPlaylistPOST_MalformedJSON(t *testing.T) {
	router := setupPlaylistContractServer(t)

	malformed := []string{
		`{`,
		`{"media_key":`,
		`not json at all`,
		`[1,2,3]`, // wrong root type
	}

	for _, body := range malformed {
		rec := postJSON(router, "/api/v1/windows/1/playlist/items", body)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("malformed JSON %q: expected 400, got %d — %s", body, rec.Code, rec.Body.String())
		}
		resp := decodeAPIResponse(t, rec)
		if resp.Success {
			t.Errorf("malformed JSON %q: expected success=false", body)
		}
	}
}

func TestPlaylistPOST_NegativeCustomDuration(t *testing.T) {
	router := setupPlaylistContractServer(t)

	rec := postJSON(router, "/api/v1/windows/1/playlist/items",
		`{"media_key":"M1","custom_duration_seconds":-5}`)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("negative duration: expected 400, got %d — %s", rec.Code, rec.Body.String())
	}
}

func TestPlaylistDELETE_NonExistentItemID(t *testing.T) {
	router := setupPlaylistContractServer(t)

	rec := deleteReq(router, "/api/v1/windows/1/playlist/item-does-not-exist")
	if rec.Code != http.StatusNotFound {
		t.Errorf("delete non-existent item: expected 404, got %d — %s", rec.Code, rec.Body.String())
	}
	resp := decodeAPIResponse(t, rec)
	if resp.Success {
		t.Error("expected success=false for non-existent item deletion")
	}
}

func TestPlaylistDELETE_NonExistentWindow(t *testing.T) {
	router := setupPlaylistContractServer(t)

	rec := deleteReq(router, "/api/v1/windows/9999/playlist/some-item-id")
	if rec.Code != http.StatusNotFound {
		t.Errorf("delete from non-existent window: expected 404, got %d — %s", rec.Code, rec.Body.String())
	}
}

func TestPlaylistGET_ResponseStructure(t *testing.T) {
	router := setupPlaylistContractServer(t)

	rec := getReq(router, "/api/v1/windows/1/playlist")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET playlist: expected 200, got %d", rec.Code)
	}

	resp := decodeAPIResponse(t, rec)
	if !resp.Success {
		t.Fatal("expected success=true")
	}
	if resp.ServerTime == "" {
		t.Error("expected non-empty server_time in response envelope")
	}

	pMap := extractPlaylistFromData(t, resp)

	// Required fields in a Playlist response
	requiredFields := []string{"window_number", "items", "total_sequence_duration_seconds", "version"}
	for _, field := range requiredFields {
		if _, ok := pMap[field]; !ok {
			t.Errorf("playlist response missing required field: %s", field)
		}
	}
}

// ─── Task 4: PUT playlist update tests ────────────────────────────────────────

func TestPlaylistPUT_ValidUpdate(t *testing.T) {
	router := setupPlaylistContractServer(t)

	// First seed an item so PUT has something to replace.
	addRec := postJSON(router, "/api/v1/windows/1/playlist/items",
		`{"media_key":"M1","custom_duration_seconds":30}`)
	if addRec.Code != http.StatusCreated {
		t.Fatalf("seed item for PUT test: got %d — %s", addRec.Code, addRec.Body.String())
	}

	// Build PUT body using the actual UpdatePlaylistRequest schema:
	// {"items": [ PlaylistItem objects ]}
	// PlaylistItem requires: media_key, type, url, duration_seconds.
	// item_id is optional (auto-generated if empty).
	putBody := `{
		"items": [
			{
				"media_key": "M2",
				"type": "image",
				"url": "https://cdn.example.com/m2.jpg",
				"duration_seconds": 20
			},
			{
				"media_key": "M4",
				"type": "video",
				"url": "https://cdn.example.com/m4.mp4",
				"duration_seconds": 45
			}
		]
	}`

	putRec := putJSON(router, "/api/v1/windows/1/playlist", putBody)
	if putRec.Code != http.StatusOK {
		t.Fatalf("PUT playlist: expected 200, got %d — %s", putRec.Code, putRec.Body.String())
	}

	putResp := decodeAPIResponse(t, putRec)
	if !putResp.Success {
		t.Fatalf("PUT playlist: expected success=true, got false — %+v", putResp.Error)
	}

	pMap := extractPlaylistFromData(t, putResp)
	items := extractItems(t, pMap)

	// ── Verify count ──────────────────────────────────────────────────────
	if len(items) != 2 {
		t.Fatalf("PUT playlist: expected 2 items, got %d", len(items))
	}

	// ── Verify ordering (order field must be sequential) ──────────────────
	for i, raw := range items {
		item := raw.(map[string]any)
		order, _ := item["order"].(float64)
		if int(order) != i+1 {
			t.Errorf("item[%d]: expected order=%d, got %v", i, i+1, item["order"])
		}
	}

	// ── Verify total duration is recalculated (20 + 45 = 65) ─────────────
	totalDur, _ := pMap["total_sequence_duration_seconds"].(float64)
	if int(totalDur) != 65 {
		t.Errorf("expected total_sequence_duration_seconds=65, got %v", pMap["total_sequence_duration_seconds"])
	}

	// ── Verify version incremented (was 0 or 1 from seed; must be > 0) ────
	version, _ := pMap["version"].(float64)
	if int(version) < 1 {
		t.Errorf("expected version >= 1 after PUT, got %v", pMap["version"])
	}

	// ── Verify item_ids were auto-assigned ────────────────────────────────
	for i, raw := range items {
		item := raw.(map[string]any)
		id, _ := item["item_id"].(string)
		if id == "" {
			t.Errorf("item[%d] missing item_id after PUT", i)
		}
	}
}

func TestPlaylistPUT_VersionIncrements(t *testing.T) {
	router := setupPlaylistContractServer(t)

	putBody := func(dur int) string {
		return fmt.Sprintf(`{"items":[{"media_key":"M1","type":"video","url":"https://cdn.example.com/m1.mp4","duration_seconds":%d}]}`, dur)
	}

	// First PUT
	rec1 := putJSON(router, "/api/v1/windows/1/playlist", putBody(30))
	if rec1.Code != http.StatusOK {
		t.Fatalf("PUT 1: expected 200, got %d — %s", rec1.Code, rec1.Body.String())
	}
	r1 := decodeAPIResponse(t, rec1)
	m1 := extractPlaylistFromData(t, r1)
	v1, _ := m1["version"].(float64)

	// Second PUT — version must increment
	rec2 := putJSON(router, "/api/v1/windows/1/playlist", putBody(60))
	if rec2.Code != http.StatusOK {
		t.Fatalf("PUT 2: expected 200, got %d — %s", rec2.Code, rec2.Body.String())
	}
	r2 := decodeAPIResponse(t, rec2)
	m2 := extractPlaylistFromData(t, r2)
	v2, _ := m2["version"].(float64)

	if v2 <= v1 {
		t.Errorf("expected version to increment: v1=%v v2=%v", v1, v2)
	}
}

func TestPlaylistPUT_TotalDurationRecalculated(t *testing.T) {
	router := setupPlaylistContractServer(t)

	putBody := `{
		"items": [
			{"media_key":"M1","type":"video","url":"https://cdn.example.com/m1.mp4","duration_seconds":30},
			{"media_key":"M2","type":"image","url":"https://cdn.example.com/m2.jpg","duration_seconds":15},
			{"media_key":"M4","type":"video","url":"https://cdn.example.com/m4.mp4","duration_seconds":45}
		]
	}`

	rec := putJSON(router, "/api/v1/windows/1/playlist", putBody)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — %s", rec.Code, rec.Body.String())
	}
	resp := decodeAPIResponse(t, rec)
	pMap := extractPlaylistFromData(t, resp)
	total, _ := pMap["total_sequence_duration_seconds"].(float64)
	if int(total) != 90 { // 30+15+45
		t.Errorf("expected total=90, got %v", pMap["total_sequence_duration_seconds"])
	}
}

func TestPlaylistPUT_MissingItemsField(t *testing.T) {
	router := setupPlaylistContractServer(t)

	// Missing "items" key at all
	rec := putJSON(router, "/api/v1/windows/1/playlist", `{"other_field":"value"}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("missing items field: expected 400, got %d — %s", rec.Code, rec.Body.String())
	}
}

func TestPlaylistPUT_EmptyItemsArray(t *testing.T) {
	router := setupPlaylistContractServer(t)

	// Empty items array clears the playlist — this IS valid (allowed by the service)
	rec := putJSON(router, "/api/v1/windows/1/playlist", `{"items":[]}`)
	// The service currently does NOT reject empty items — it simply sets the
	// playlist to empty and returns 200. Verify that behavior.
	if rec.Code != http.StatusOK {
		t.Errorf("empty items array: expected 200 (clear playlist), got %d — %s", rec.Code, rec.Body.String())
	}
}

func TestPlaylistPUT_InvalidWindow(t *testing.T) {
	router := setupPlaylistContractServer(t)

	rec := putJSON(router, "/api/v1/windows/9999/playlist",
		`{"items":[{"media_key":"M1","type":"video","url":"https://cdn.example.com/m1.mp4","duration_seconds":30}]}`)
	if rec.Code != http.StatusNotFound {
		t.Errorf("PUT invalid window: expected 404, got %d — %s", rec.Code, rec.Body.String())
	}
}

func TestPlaylistPUT_MalformedJSON(t *testing.T) {
	router := setupPlaylistContractServer(t)

	malformed := []string{
		`{`,
		`{"items": [}`,
		`not json`,
	}

	for _, body := range malformed {
		rec := putJSON(router, "/api/v1/windows/1/playlist", body)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("PUT malformed %q: expected 400, got %d", body, rec.Code)
		}
	}
}

func TestPlaylistPUT_ZeroDurationItem(t *testing.T) {
	router := setupPlaylistContractServer(t)

	rec := putJSON(router, "/api/v1/windows/1/playlist",
		`{"items":[{"media_key":"M1","type":"video","url":"https://cdn.example.com/m1.mp4","duration_seconds":0}]}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("zero duration item: expected 400, got %d — %s", rec.Code, rec.Body.String())
	}
}

func TestPlaylistPUT_DatabaseStateAfterUpdate(t *testing.T) {
	router := setupPlaylistContractServer(t)

	// PUT with M4 only
	putRec := putJSON(router, "/api/v1/windows/1/playlist",
		`{"items":[{"media_key":"M4","type":"video","url":"https://cdn.example.com/m4.mp4","duration_seconds":45}]}`)
	if putRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — %s", putRec.Code, putRec.Body.String())
	}

	// Independently GET the playlist and confirm state
	getRec := getReq(router, "/api/v1/windows/1/playlist")
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET after PUT: expected 200, got %d", getRec.Code)
	}

	getResp := decodeAPIResponse(t, getRec)
	pMap := extractPlaylistFromData(t, getResp)
	items := extractItems(t, pMap)

	if len(items) != 1 {
		t.Fatalf("expected 1 item after PUT, got %d", len(items))
	}
	item := items[0].(map[string]any)
	if item["media_key"] != "M4" {
		t.Errorf("expected media_key=M4 after PUT, got %v", item["media_key"])
	}
	total, _ := pMap["total_sequence_duration_seconds"].(float64)
	if int(total) != 45 {
		t.Errorf("expected total=45, got %v", total)
	}
}

// ─── Task 5: Playback calculation tests ───────────────────────────────────────

// setupPlaybackTestRouter creates a router with Window 1 seeded with a 3-item
// playlist and a known start time for deterministic calculations.
func setupPlaybackTestRouter(t *testing.T, cycleStart time.Time) http.Handler {
	t.Helper()
	ctx := context.Background()
	router, mediaRepo, windowRepo, playlistRepo, _ := setupTestRouter(nil)

	if err := windowRepo.Create(ctx, &models.Window{
		WindowNumber:         1,
		Name:                 "Display 1",
		CycleDurationSeconds: 18000,
		CycleStartTime:       cycleStart,
		IsActive:             true,
	}); err != nil {
		t.Fatalf("seed window: %v", err)
	}

	medias := []models.Media{
		{MediaKey: "M1", Name: "Intro", Type: models.MediaTypeVideo, URL: "https://cdn.example.com/m1.mp4", DurationSeconds: 30},
		{MediaKey: "M2", Name: "Feature", Type: models.MediaTypeImage, URL: "https://cdn.example.com/m2.jpg", DurationSeconds: 20},
		{MediaKey: "M3", Name: "Blank", Type: models.MediaTypeBlank, URL: "", DurationSeconds: 10},
	}
	for _, m := range medias {
		mc := m
		_ = mediaRepo.Create(ctx, &mc)
	}

	_ = playlistRepo.Upsert(ctx, &models.Playlist{
		WindowNumber: 1,
		Items: []models.PlaylistItem{
			{ItemID: "item-a", MediaKey: "M1", Type: models.MediaTypeVideo, URL: "https://cdn.example.com/m1.mp4", DurationSeconds: 30, Order: 1},
			{ItemID: "item-b", MediaKey: "M2", Type: models.MediaTypeImage, URL: "https://cdn.example.com/m2.jpg", DurationSeconds: 20, Order: 2},
			{ItemID: "item-c", MediaKey: "M3", Type: models.MediaTypeBlank, URL: "", DurationSeconds: 10, Order: 3},
		},
		TotalSequenceDurationSeconds: 60,
		Version:                      1,
	})

	return router
}

func TestPlayback_ResponseStructure(t *testing.T) {
	router := setupPlaybackTestRouter(t, time.Now().UTC().Add(-1*time.Minute))

	rec := getReq(router, "/api/v1/windows/1/playback")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET playback: expected 200, got %d — %s", rec.Code, rec.Body.String())
	}

	resp := decodeAPIResponse(t, rec)
	if !resp.Success {
		t.Fatal("expected success=true")
	}

	stateMap, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected data to be a map, got %T", resp.Data)
	}

	// Verify all required playback state fields are present
	requiredFields := []string{
		"window_number",
		"status",
		"media_key",
		"media_type",
		"item_duration",
		"playback_position",
		"time_remaining",
		"cycle_duration",
		"cycle_position",
		"cycle_number",
		"sequence_iteration",
		"sequence_duration",
		"evaluated_at",
	}
	for _, field := range requiredFields {
		if _, ok := stateMap[field]; !ok {
			t.Errorf("playback response missing required field: %s", field)
		}
	}
}

func TestPlayback_CurrentItemAtCycleStart(t *testing.T) {
	// At cycle start + 5s the first item (M1, 30s duration) must be active.
	// We use a fixed past anchor so the query time is unambiguous.
	origin := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	router := setupPlaybackTestRouter(t, origin)

	// Query at cycle start + 5s — well within M1 (0–30s)
	queryTime := origin.Add(5 * time.Second).Format(time.RFC3339)
	rec := getReq(router, "/api/v1/windows/1/playback?time="+queryTime)
	if rec.Code != http.StatusOK {
		t.Fatalf("playback at cycle start+5s: expected 200, got %d — %s", rec.Code, rec.Body.String())
	}

	resp := decodeAPIResponse(t, rec)
	stateMap := resp.Data.(map[string]any)

	if stateMap["media_key"] != "M1" {
		t.Errorf("at cycle start+5s expected media_key=M1, got %v", stateMap["media_key"])
	}
	if stateMap["status"] != "NORMAL" {
		t.Errorf("expected status=NORMAL for M1 (video), got %v", stateMap["status"])
	}
}

func TestPlayback_SecondItemAtOffset30s(t *testing.T) {
	// After 30 seconds the second item (M2) must be active.
	origin := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC) // Fixed anchor
	router := setupPlaybackTestRouter(t, origin)

	// Query 35s after start → should be into M2 (M1=30s, M2 starts at 30s)
	queryTime := origin.Add(35 * time.Second).Format(time.RFC3339)
	rec := getReq(router, "/api/v1/windows/1/playback?time="+queryTime)
	if rec.Code != http.StatusOK {
		t.Fatalf("playback at 35s offset: expected 200, got %d — %s", rec.Code, rec.Body.String())
	}

	resp := decodeAPIResponse(t, rec)
	stateMap := resp.Data.(map[string]any)

	if stateMap["media_key"] != "M2" {
		t.Errorf("at 35s expected media_key=M2, got %v", stateMap["media_key"])
	}
}

func TestPlayback_BlankItemStatus(t *testing.T) {
	// At offset 55s, M3 (blank) should be active → status=BLANK.
	origin := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC) // Fixed anchor
	router := setupPlaybackTestRouter(t, origin)

	// M1=30s, M2=20s, M3 starts at 50s → query at 55s
	queryTime := origin.Add(55 * time.Second).Format(time.RFC3339)
	rec := getReq(router, "/api/v1/windows/1/playback?time="+queryTime)
	if rec.Code != http.StatusOK {
		t.Fatalf("playback at blank item: expected 200, got %d — %s", rec.Code, rec.Body.String())
	}

	resp := decodeAPIResponse(t, rec)
	stateMap := resp.Data.(map[string]any)

	if stateMap["media_key"] != "M3" {
		t.Errorf("at 55s expected media_key=M3, got %v", stateMap["media_key"])
	}
	if stateMap["status"] != "BLANK" {
		t.Errorf("expected status=BLANK for blank media type, got %v", stateMap["status"])
	}
}

func TestPlayback_SequenceWrapsAround(t *testing.T) {
	// After 60s the sequence wraps back to M1.
	origin := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC) // Fixed anchor
	router := setupPlaybackTestRouter(t, origin)

	// Sequence duration = 60s; at 62s we should be 2s into M1 again.
	queryTime := origin.Add(62 * time.Second).Format(time.RFC3339)
	rec := getReq(router, "/api/v1/windows/1/playback?time="+queryTime)
	if rec.Code != http.StatusOK {
		t.Fatalf("playback wrap: expected 200, got %d — %s", rec.Code, rec.Body.String())
	}

	resp := decodeAPIResponse(t, rec)
	stateMap := resp.Data.(map[string]any)

	if stateMap["media_key"] != "M1" {
		t.Errorf("at 62s (60s+2s) expected media_key=M1 (wrap), got %v", stateMap["media_key"])
	}

	iter, _ := stateMap["sequence_iteration"].(float64)
	if int(iter) != 1 {
		t.Errorf("expected sequence_iteration=1 at 62s, got %v", iter)
	}
}

func TestPlayback_CyclePosition(t *testing.T) {
	// Use fixed timestamps so there's no wall-clock drift between setup and query.
	origin := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	router := setupPlaybackTestRouter(t, origin)

	// Query exactly 90s after origin using RFC3339 (seconds precision).
	// RFC3339 truncates to the second, so cycle_position = exactly 90s.
	queryTime := origin.Add(90 * time.Second).Format(time.RFC3339)
	rec := getReq(router, "/api/v1/windows/1/playback?time="+queryTime)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	resp := decodeAPIResponse(t, rec)
	stateMap := resp.Data.(map[string]any)

	cyclePos, _ := stateMap["cycle_position"].(float64)
	// cycle_position is a time.Duration (int64 nanoseconds) serialised as JSON number.
	// 90s = 90_000_000_000 ns
	expectedNs := float64(90 * time.Second)
	if cyclePos != expectedNs {
		t.Errorf("expected cycle_position=%v ns, got %v", expectedNs, cyclePos)
	}

	cycleNum, _ := stateMap["cycle_number"].(float64)
	if int(cycleNum) != 0 {
		t.Errorf("expected cycle_number=0 at 90s, got %v", cycleNum)
	}
}

func TestPlayback_EmptyPlaylist_FallbackStatus(t *testing.T) {
	ctx := context.Background()
	router, _, windowRepo, playlistRepo, _ := setupTestRouter(nil)

	_ = windowRepo.Create(ctx, &models.Window{
		WindowNumber:         1,
		Name:                 "Empty Window",
		CycleDurationSeconds: 18000,
		CycleStartTime:       time.Now().UTC(),
		IsActive:             true,
	})
	_ = playlistRepo.Upsert(ctx, &models.Playlist{
		WindowNumber: 1,
		Items:        []models.PlaylistItem{},
	})

	rec := getReq(router, "/api/v1/windows/1/playback")
	if rec.Code != http.StatusOK {
		t.Fatalf("empty playlist playback: expected 200, got %d — %s", rec.Code, rec.Body.String())
	}

	resp := decodeAPIResponse(t, rec)
	stateMap := resp.Data.(map[string]any)

	if stateMap["status"] != "FALLBACK" {
		t.Errorf("empty playlist: expected status=FALLBACK, got %v", stateMap["status"])
	}
	if stateMap["media_key"] != "FALLBACK" {
		t.Errorf("empty playlist: expected media_key=FALLBACK, got %v", stateMap["media_key"])
	}
}

func TestPlayback_InvalidWindow(t *testing.T) {
	router := setupPlaylistContractServer(t)

	rec := getReq(router, "/api/v1/windows/9999/playback")
	if rec.Code != http.StatusNotFound {
		t.Errorf("invalid window playback: expected 404, got %d — %s", rec.Code, rec.Body.String())
	}
}

func TestPlayback_InvalidTimestamp(t *testing.T) {
	router := setupPlaylistContractServer(t)

	rec := getReq(router, "/api/v1/windows/1/playback?time=not-a-timestamp")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("invalid timestamp: expected 400, got %d — %s", rec.Code, rec.Body.String())
	}
	resp := decodeAPIResponse(t, rec)
	if resp.Error == nil || resp.Error.Code != "INVALID_TIMESTAMP" {
		t.Errorf("expected INVALID_TIMESTAMP error code, got %+v", resp.Error)
	}
}

func TestPlayback_NextItemField(t *testing.T) {
	origin := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC) // Fixed anchor
	router := setupPlaybackTestRouter(t, origin)

	// Query at cycle start — M1 is active, next item should be M2.
	queryTime := origin.Add(5 * time.Second).Format(time.RFC3339)
	rec := getReq(router, "/api/v1/windows/1/playback?time="+queryTime)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	resp := decodeAPIResponse(t, rec)
	stateMap := resp.Data.(map[string]any)

	nextItem, ok := stateMap["next_item"]
	if !ok {
		t.Fatal("playback response missing next_item field")
	}
	if nextItem == nil {
		t.Fatal("next_item should not be nil when playlist has multiple items")
	}
	nextMap := nextItem.(map[string]any)
	if nextMap["media_key"] != "M2" {
		t.Errorf("expected next_item.media_key=M2, got %v", nextMap["media_key"])
	}
}

func TestPlayback_SequenceDurationField(t *testing.T) {
	origin := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC) // Fixed anchor
	router := setupPlaybackTestRouter(t, origin)

	rec := getReq(router, "/api/v1/windows/1/playback")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	resp := decodeAPIResponse(t, rec)
	stateMap := resp.Data.(map[string]any)

	seqDur, _ := stateMap["sequence_duration"].(float64)
	// 30+20+10=60s = 60_000_000_000 ns
	expected := float64(60 * time.Second)
	if seqDur != expected {
		t.Errorf("expected sequence_duration=%v ns, got %v", expected, seqDur)
	}
}

// ─── Task 6: Sync contract tests ──────────────────────────────────────────────

func setupSyncContractServer(t *testing.T) http.Handler {
	t.Helper()
	ctx := context.Background()
	router, mediaRepo, windowRepo, playlistRepo, _ := setupTestRouter(nil)

	_ = windowRepo.Create(ctx, &models.Window{
		WindowNumber: 1, Name: "Sync Display", CycleDurationSeconds: 18000,
		CycleStartTime: time.Now().UTC(), IsActive: true,
	})
	_ = mediaRepo.Create(ctx, &models.Media{
		MediaKey: "M2", Name: "Sync Banner", Type: models.MediaTypeImage,
		URL: "https://cdn.example.com/m2.jpg", DurationSeconds: 15,
	})
	_ = playlistRepo.Upsert(ctx, &models.Playlist{
		WindowNumber: 1, Items: []models.PlaylistItem{},
	})
	return router
}

func TestSync_TriggerAndResponseStructure(t *testing.T) {
	router := setupSyncContractServer(t)

	syncBody := `{"media_key":"M2","duration_seconds":15,"lead_time_ms":500}`
	rec := postJSON(router, "/api/v1/sync", syncBody)

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST sync: expected 201, got %d — %s", rec.Code, rec.Body.String())
	}

	resp := decodeAPIResponse(t, rec)
	if !resp.Success {
		t.Fatalf("POST sync: expected success=true, got false — %+v", resp.Error)
	}

	eventMap, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("sync response data is not a map, got %T", resp.Data)
	}

	requiredFields := []string{"event_id", "media_key", "duration_seconds", "status", "start_time", "end_time"}
	for _, field := range requiredFields {
		if _, ok := eventMap[field]; !ok {
			t.Errorf("sync response missing required field: %s", field)
		}
	}

	if eventMap["media_key"] != "M2" {
		t.Errorf("expected media_key=M2, got %v", eventMap["media_key"])
	}
	if eventMap["status"] != "SCHEDULED" && eventMap["status"] != "ACTIVE" {
		t.Errorf("expected status=SCHEDULED or ACTIVE, got %v", eventMap["status"])
	}
}

func TestSync_GetActiveSync(t *testing.T) {
	router := setupSyncContractServer(t)

	// Trigger sync first
	postJSON(router, "/api/v1/sync", `{"media_key":"M2","duration_seconds":15,"lead_time_ms":500}`)

	rec := getReq(router, "/api/v1/sync/current")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET sync/current: expected 200, got %d — %s", rec.Code, rec.Body.String())
	}

	resp := decodeAPIResponse(t, rec)
	if !resp.Success {
		t.Fatal("GET sync/current: expected success=true")
	}
}

func TestSync_GetByID(t *testing.T) {
	router := setupSyncContractServer(t)

	createRec := postJSON(router, "/api/v1/sync", `{"media_key":"M2","duration_seconds":15,"lead_time_ms":100}`)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create sync: got %d", createRec.Code)
	}
	createResp := decodeAPIResponse(t, createRec)
	eventMap := createResp.Data.(map[string]any)
	eventID := eventMap["event_id"].(string)

	getRec := getReq(router, "/api/v1/sync/"+eventID)
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET sync/{id}: expected 200, got %d — %s", getRec.Code, getRec.Body.String())
	}

	getResp := decodeAPIResponse(t, getRec)
	getMap := getResp.Data.(map[string]any)
	if getMap["event_id"] != eventID {
		t.Errorf("expected event_id=%s, got %v", eventID, getMap["event_id"])
	}
}

func TestSync_CancelSync(t *testing.T) {
	router := setupSyncContractServer(t)

	createRec := postJSON(router, "/api/v1/sync", `{"media_key":"M2","duration_seconds":15,"lead_time_ms":100}`)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create sync: got %d", createRec.Code)
	}
	createResp := decodeAPIResponse(t, createRec)
	eventMap := createResp.Data.(map[string]any)
	eventID := eventMap["event_id"].(string)

	cancelRec := postJSON(router, "/api/v1/sync/"+eventID+"/cancel", "")
	if cancelRec.Code != http.StatusOK {
		t.Fatalf("cancel sync: expected 200, got %d — %s", cancelRec.Code, cancelRec.Body.String())
	}

	cancelResp := decodeAPIResponse(t, cancelRec)
	if !cancelResp.Success {
		t.Fatal("cancel sync: expected success=true")
	}
}

func TestSync_GetNonExistentID(t *testing.T) {
	router := setupSyncContractServer(t)

	rec := getReq(router, "/api/v1/sync/sync-does-not-exist-abc123")
	if rec.Code != http.StatusNotFound {
		t.Errorf("non-existent sync id: expected 404, got %d — %s", rec.Code, rec.Body.String())
	}
}

func TestSync_CancelNonExistentID(t *testing.T) {
	router := setupSyncContractServer(t)

	rec := postJSON(router, "/api/v1/sync/sync-does-not-exist-abc123/cancel", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("cancel non-existent sync: expected 404, got %d — %s", rec.Code, rec.Body.String())
	}
}

func TestSync_MissingMediaKey(t *testing.T) {
	router := setupSyncContractServer(t)

	rec := postJSON(router, "/api/v1/sync", `{"duration_seconds":15}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("sync missing media_key: expected 400, got %d — %s", rec.Code, rec.Body.String())
	}
}

func TestSync_NonExistentMediaKey(t *testing.T) {
	router := setupSyncContractServer(t)

	rec := postJSON(router, "/api/v1/sync", `{"media_key":"GHOST","duration_seconds":15}`)
	if rec.Code != http.StatusNotFound {
		t.Errorf("sync non-existent media: expected 404, got %d — %s", rec.Code, rec.Body.String())
	}
}

func TestSync_ZeroDuration(t *testing.T) {
	router := setupSyncContractServer(t)

	rec := postJSON(router, "/api/v1/sync", `{"media_key":"M2","duration_seconds":0}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("sync zero duration: expected 400, got %d — %s", rec.Code, rec.Body.String())
	}
}

func TestSync_NegativeDuration(t *testing.T) {
	router := setupSyncContractServer(t)

	rec := postJSON(router, "/api/v1/sync", `{"media_key":"M2","duration_seconds":-10}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("sync negative duration: expected 400, got %d — %s", rec.Code, rec.Body.String())
	}
}

func TestSync_MalformedJSON(t *testing.T) {
	router := setupSyncContractServer(t)

	rec := postJSON(router, "/api/v1/sync", `{"media_key":`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("sync malformed JSON: expected 400, got %d — %s", rec.Code, rec.Body.String())
	}
}

func TestSync_ResponseEnvelopeAlwaysPresent(t *testing.T) {
	router := setupSyncContractServer(t)

	// Even error responses must include the standard envelope fields
	rec := postJSON(router, "/api/v1/sync", `{}`)

	resp := decodeAPIResponse(t, rec)
	if resp.Success {
		t.Error("empty sync body: expected success=false")
	}
	if resp.Error == nil {
		t.Error("expected error field in error response")
	}
	if resp.ServerTime == "" {
		t.Error("expected server_time in error response")
	}
}
