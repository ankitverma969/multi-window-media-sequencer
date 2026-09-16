package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/eva-bharat/media-sequencer/backend/internal/utils"
)

func TestCORSMiddleware(t *testing.T) {
	allowedOrigins := []string{"http://localhost:5173", "http://example.com"}
	corsHandler := CORS(allowedOrigins)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Test allowed origin
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	rec := httptest.NewRecorder()

	corsHandler.ServeHTTP(rec, req)

	if rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Errorf("expected Access-Control-Allow-Origin to match allowed origin")
	}

	// Test disallowed origin
	reqDisallowed := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	reqDisallowed.Header.Set("Origin", "http://malicious.com")
	recDisallowed := httptest.NewRecorder()

	corsHandler.ServeHTTP(recDisallowed, reqDisallowed)

	if recDisallowed.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("expected no Access-Control-Allow-Origin for disallowed origin")
	}

	// Test preflight OPTIONS request
	reqOptions := httptest.NewRequest(http.MethodOptions, "/api/v1/health", nil)
	reqOptions.Header.Set("Origin", "http://localhost:5173")
	recOptions := httptest.NewRecorder()

	corsHandler.ServeHTTP(recOptions, reqOptions)

	if recOptions.Code != http.StatusNoContent {
		t.Errorf("expected status 204 for preflight OPTIONS, got %d", recOptions.Code)
	}
}

func TestRecoveryMiddleware(t *testing.T) {
	panickingHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("simulated critical crash")
	})

	recoveryHandler := Recovery(panickingHandler)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/panic", nil)
	rec := httptest.NewRecorder()

	recoveryHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500 on panic recovery, got %d", rec.Code)
	}

	var resp utils.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if resp.Success {
		t.Errorf("expected success false")
	}
	if resp.Error == nil || resp.Error.Code != "INTERNAL_ERROR" {
		t.Errorf("expected INTERNAL_ERROR code, got %+v", resp.Error)
	}
}
