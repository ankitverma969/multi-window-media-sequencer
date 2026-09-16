package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

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
	CustomDurationSeconds int    `json:"custom_duration_seconds"`
}

func (h *PlaylistHandler) GetPlaylist(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	windowNumber, err := strconv.Atoi(idStr)
	if err != nil || windowNumber <= 0 {
		utils.WriteError(w, http.StatusBadRequest, "INVALID_WINDOW_ID", "Window ID must be a positive integer")
		return
	}

	p, err := h.playlistService.GetPlaylist(r.Context(), windowNumber)
	if err != nil {
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
	idStr := r.PathValue("id")
	windowNumber, err := strconv.Atoi(idStr)
	if err != nil || windowNumber <= 0 {
		utils.WriteError(w, http.StatusBadRequest, "INVALID_WINDOW_ID", "Window ID must be a positive integer")
		return
	}

	var req AddItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON payload")
		return
	}

	if strings.TrimSpace(req.MediaKey) == "" {
		utils.WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "media_key is required")
		return
	}

	updated, err := h.playlistService.AddPlaylistItem(r.Context(), windowNumber, req.MediaKey, req.CustomDurationSeconds)
	if err != nil {
		if errors.Is(err, service.ErrWindowNotFound) {
			utils.WriteError(w, http.StatusNotFound, "WINDOW_NOT_FOUND", "Specified window does not exist")
			return
		}
		if errors.Is(err, service.ErrMediaNotFound) {
			utils.WriteError(w, http.StatusNotFound, "MEDIA_NOT_FOUND", "Specified media key does not exist")
			return
		}
		utils.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to add playlist item")
		return
	}

	utils.WriteJSON(w, http.StatusCreated, updated)
}

func (h *PlaylistHandler) RemovePlaylistItem(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	windowNumber, err := strconv.Atoi(idStr)
	if err != nil || windowNumber <= 0 {
		utils.WriteError(w, http.StatusBadRequest, "INVALID_WINDOW_ID", "Window ID must be a positive integer")
		return
	}

	itemID := r.PathValue("itemId")
	if strings.TrimSpace(itemID) == "" {
		utils.WriteError(w, http.StatusBadRequest, "INVALID_ITEM_ID", "Item ID is required")
		return
	}

	updated, err := h.playlistService.RemovePlaylistItem(r.Context(), windowNumber, itemID)
	if err != nil {
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
	idStr := r.PathValue("id")
	windowNumber, err := strconv.Atoi(idStr)
	if err != nil || windowNumber <= 0 {
		utils.WriteError(w, http.StatusBadRequest, "INVALID_WINDOW_ID", "Window ID must be a positive integer")
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

	state, err := h.playlistService.GetPlaybackState(r.Context(), windowNumber, queryTime)
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
