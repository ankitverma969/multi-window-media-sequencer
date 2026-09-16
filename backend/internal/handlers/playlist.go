package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/eva-bharat/media-sequencer/backend/internal/models"
	"github.com/eva-bharat/media-sequencer/backend/internal/service"
	"github.com/eva-bharat/media-sequencer/backend/internal/utils"
)

type PlaylistHandler struct {
	playlistService service.PlaylistService
}

func NewPlaylistHandler(playlistService service.PlaylistService) *PlaylistHandler {
	return &PlaylistHandler{
		playlistService: playlistService,
	}
}

type AddItemRequest struct {
	MediaKey              string `json:"media_key"`
	MediaID               string `json:"media_id"`
	CustomDurationSeconds int    `json:"custom_duration_seconds"`
	Duration              int    `json:"duration"`
}

type UpdatePlaylistRequest struct {
	Items []models.PlaylistItem `json:"items"`
}

func getWindowID(r *http.Request) string {
	id := r.PathValue("id")
	if id == "" {
		id = r.PathValue("windowId")
	}
	return strings.TrimSpace(id)
}

func (h *PlaylistHandler) GetPlaylist(w http.ResponseWriter, r *http.Request) {
	idStr := getWindowID(r)
	if idStr == "" {
		utils.WriteError(w, http.StatusBadRequest, "INVALID_WINDOW_ID", "Window ID or number is required")
		return
	}

	p, err := h.playlistService.GetPlaylist(r.Context(), idStr)
	if err != nil {
		if errors.Is(err, service.ErrWindowNotFound) {
			utils.WriteError(w, http.StatusNotFound, "WINDOW_NOT_FOUND", "Window not found")
			return
		}
		if errors.Is(err, service.ErrPlaylistNotFound) {
			utils.WriteError(w, http.StatusNotFound, "NOT_FOUND", "Playlist not found for window")
			return
		}
		utils.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve playlist")
		return
	}

	utils.WriteJSON(w, http.StatusOK, p)
}

func (h *PlaylistHandler) AddPlaylistItem(w http.ResponseWriter, r *http.Request) {
	idStr := getWindowID(r)
	if idStr == "" {
		utils.WriteError(w, http.StatusBadRequest, "INVALID_WINDOW_ID", "Window ID or number is required")
		return
	}

	var req AddItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON payload")
		return
	}

	mediaKey := strings.TrimSpace(req.MediaKey)
	if mediaKey == "" {
		mediaKey = strings.TrimSpace(req.MediaID)
	}
	if mediaKey == "" {
		utils.WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "media_key or media_id is required")
		return
	}

	// Use CustomDurationSeconds if provided; fall back to the alias field Duration.
	// Negative values are intentionally passed through to the service for validation.
	customDuration := req.CustomDurationSeconds
	if customDuration == 0 && req.Duration > 0 {
		customDuration = req.Duration
	}

	updated, err := h.playlistService.AddPlaylistItem(r.Context(), idStr, mediaKey, customDuration)
	if err != nil {
		if errors.Is(err, service.ErrWindowNotFound) {
			utils.WriteError(w, http.StatusNotFound, "WINDOW_NOT_FOUND", "Specified window does not exist")
			return
		}
		if errors.Is(err, service.ErrMediaNotFound) {
			utils.WriteError(w, http.StatusNotFound, "MEDIA_NOT_FOUND", "Specified media key does not exist")
			return
		}
		if errors.Is(err, service.ErrInvalidInput) {
			utils.WriteError(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
			return
		}
		if errors.Is(err, service.ErrConflict) {
			utils.WriteError(w, http.StatusConflict, "CONCURRENT_CONFLICT", "Concurrent update conflict. Please retry.")
			return
		}
		utils.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to add playlist item")
		return
	}

	utils.WriteJSON(w, http.StatusCreated, updated)
}

func (h *PlaylistHandler) UpdatePlaylist(w http.ResponseWriter, r *http.Request) {
	idStr := getWindowID(r)
	if idStr == "" {
		utils.WriteError(w, http.StatusBadRequest, "INVALID_WINDOW_ID", "Window ID or number is required")
		return
	}

	// Support both {"items": [...]} envelope and raw [...] array
	var req UpdatePlaylistRequest
	decoder := json.NewDecoder(r.Body)

	// Try reading as raw slice first
	var rawItems []models.PlaylistItem
	var err error
	var bodyBytes []byte
	var bodyMap map[string]json.RawMessage

	if err = decoder.Decode(&bodyMap); err == nil {
		if itemsRaw, ok := bodyMap["items"]; ok {
			_ = json.Unmarshal(itemsRaw, &rawItems)
		}
	}

	if rawItems == nil {
		utils.WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON payload: 'items' array required")
		return
	}
	req.Items = rawItems

	updated, err := h.playlistService.UpdatePlaylist(r.Context(), idStr, req.Items)
	if err != nil {
		if errors.Is(err, service.ErrWindowNotFound) {
			utils.WriteError(w, http.StatusNotFound, "WINDOW_NOT_FOUND", "Window not found")
			return
		}
		if errors.Is(err, service.ErrPlaylistNotFound) {
			utils.WriteError(w, http.StatusNotFound, "NOT_FOUND", "Playlist not found for window")
			return
		}
		if errors.Is(err, service.ErrInvalidInput) {
			utils.WriteError(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
			return
		}
		utils.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update playlist")
		return
	}

	utils.WriteJSON(w, http.StatusOK, updated)
	_ = bodyBytes
}

func (h *PlaylistHandler) RemovePlaylistItem(w http.ResponseWriter, r *http.Request) {
	idStr := getWindowID(r)
	if idStr == "" {
		utils.WriteError(w, http.StatusBadRequest, "INVALID_WINDOW_ID", "Window ID or number is required")
		return
	}

	itemID := r.PathValue("itemId")
	if strings.TrimSpace(itemID) == "" {
		utils.WriteError(w, http.StatusBadRequest, "INVALID_ITEM_ID", "Item ID is required")
		return
	}

	updated, err := h.playlistService.RemovePlaylistItem(r.Context(), idStr, itemID)
	if err != nil {
		if errors.Is(err, service.ErrWindowNotFound) {
			utils.WriteError(w, http.StatusNotFound, "WINDOW_NOT_FOUND", "Window not found")
			return
		}
		if errors.Is(err, service.ErrPlaylistNotFound) {
			utils.WriteError(w, http.StatusNotFound, "NOT_FOUND", "Item or playlist not found")
			return
		}
		utils.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to remove playlist item")
		return
	}

	utils.WriteJSON(w, http.StatusOK, updated)
}

func (h *PlaylistHandler) GetPlaybackState(w http.ResponseWriter, r *http.Request) {
	idStr := getWindowID(r)
	if idStr == "" {
		utils.WriteError(w, http.StatusBadRequest, "INVALID_WINDOW_ID", "Window ID or number is required")
		return
	}

	queryTime := time.Now().UTC()
	if timeStr := r.URL.Query().Get("time"); timeStr != "" {
		parsedTime, parseErr := time.Parse(time.RFC3339, timeStr)
		if parseErr != nil {
			utils.WriteError(w, http.StatusBadRequest, "INVALID_TIMESTAMP", "Timestamp must be in RFC3339 format")
			return
		}
		queryTime = parsedTime.UTC()
	}

	state, err := h.playlistService.GetPlaybackState(r.Context(), idStr, queryTime)
	if err != nil {
		if errors.Is(err, service.ErrWindowNotFound) {
			utils.WriteError(w, http.StatusNotFound, "NOT_FOUND", "Window not found")
			return
		}
		if errors.Is(err, service.ErrPlaylistNotFound) {
			utils.WriteError(w, http.StatusNotFound, "NOT_FOUND", "Playlist not found for window")
			return
		}
		utils.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to calculate playback state")
		return
	}

	utils.WriteJSON(w, http.StatusOK, state)
}
