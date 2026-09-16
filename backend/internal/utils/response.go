package utils

import (
	"encoding/json"
	"net/http"
	"time"
)

// APIError represents the standardized error payload returned by all API endpoints.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// APIResponse is the unified envelope returned for both success and error responses.
type APIResponse struct {
	Success    bool      `json:"success"`
	Data       any       `json:"data"`
	Error      *APIError `json:"error"`
	ServerTime string    `json:"server_time"`
}

// WriteJSON sends a formatted JSON response with the current UTC server time.
func WriteJSON(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	resp := APIResponse{
		Success:    statusCode >= 200 && statusCode < 300,
		Data:       data,
		Error:      nil,
		ServerTime: time.Now().UTC().Format(time.RFC3339Nano),
	}

	_ = json.NewEncoder(w).Encode(resp)
}

// WriteError sends a standardized error response with the appropriate HTTP status code.
func WriteError(w http.ResponseWriter, statusCode int, errorCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	resp := APIResponse{
		Success: false,
		Data:    nil,
		Error: &APIError{
			Code:    errorCode,
			Message: message,
		},
		ServerTime: time.Now().UTC().Format(time.RFC3339Nano),
	}

	_ = json.NewEncoder(w).Encode(resp)
}
