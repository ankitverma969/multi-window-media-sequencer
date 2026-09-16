package utils

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	payload := map[string]string{"key": "value"}

	WriteJSON(rec, http.StatusOK, payload)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected application/json content type")
	}

	var resp APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success true")
	}
	if resp.Error != nil {
		t.Errorf("expected error to be nil")
	}
	if resp.ServerTime == "" {
		t.Errorf("expected server_time to be populated")
	}
}

func TestWriteError(t *testing.T) {
	rec := httptest.NewRecorder()

	WriteError(rec, http.StatusBadRequest, "INVALID_INPUT", "Input validation failed")

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}

	var resp APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Success {
		t.Errorf("expected success false")
	}
	if resp.Error == nil {
		t.Fatalf("expected error object")
	}
	if resp.Error.Code != "INVALID_INPUT" {
		t.Errorf("expected code INVALID_INPUT, got %s", resp.Error.Code)
	}
	if resp.Error.Message != "Input validation failed" {
		t.Errorf("expected message 'Input validation failed', got %s", resp.Error.Message)
	}
}
