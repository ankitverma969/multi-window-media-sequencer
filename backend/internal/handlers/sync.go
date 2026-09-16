package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/eva-bharat/media-sequencer/backend/internal/service"
	"github.com/eva-bharat/media-sequencer/backend/internal/utils"
)

type SyncHandler struct {
	syncService service.SyncService
}

func NewSyncHandler(syncService service.SyncService) *SyncHandler {
	return &SyncHandler{
		syncService: syncService,
	}
}

type TriggerSyncRequest struct {
	MediaKey        string `json:"media_key"`
	MediaID         string `json:"media_id"`
	DurationSeconds int    `json:"duration_seconds"`
	Duration        int    `json:"duration"`
	LeadTimeMs      int    `json:"lead_time_ms"`
	TriggeredBy     string `json:"triggered_by"`
}

func (h *SyncHandler) TriggerSync(w http.ResponseWriter, r *http.Request) {
	var req TriggerSyncRequest
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

	duration := req.DurationSeconds
	if duration <= 0 {
		duration = req.Duration
	}
	if duration <= 0 {
		utils.WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "duration_seconds must be positive")
		return
	}

	event, err := h.syncService.TriggerSync(
		r.Context(),
		mediaKey,
		duration,
		req.LeadTimeMs,
		req.TriggeredBy,
	)
	if err != nil {
		if errors.Is(err, service.ErrMediaNotFound) {
			utils.WriteError(w, http.StatusNotFound, "MEDIA_NOT_FOUND", "Specified media item not found")
			return
		}
		if errors.Is(err, service.ErrInvalidInput) {
			utils.WriteError(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
			return
		}
		utils.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to trigger synchronization")
		return
	}

	utils.WriteJSON(w, http.StatusCreated, event)
}

func (h *SyncHandler) GetActiveSync(w http.ResponseWriter, r *http.Request) {
	active, err := h.syncService.GetActiveSync(r.Context())
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve active sync state")
		return
	}
	utils.WriteJSON(w, http.StatusOK, active)
}

func (h *SyncHandler) GetSyncEvent(w http.ResponseWriter, r *http.Request) {
	eventID := r.PathValue("event_id")
	if strings.TrimSpace(eventID) == "" {
		eventID = r.PathValue("id")
	}
	if strings.TrimSpace(eventID) == "" {
		utils.WriteError(w, http.StatusBadRequest, "INVALID_EVENT_ID", "event id is required")
		return
	}

	event, err := h.syncService.GetSyncEvent(r.Context(), eventID)
	if err != nil {
		if errors.Is(err, service.ErrSyncEventNotFound) {
			utils.WriteError(w, http.StatusNotFound, "NOT_FOUND", "Sync event not found")
			return
		}
		utils.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve sync event")
		return
	}

	utils.WriteJSON(w, http.StatusOK, event)
}

func (h *SyncHandler) CancelSync(w http.ResponseWriter, r *http.Request) {
	eventID := r.PathValue("event_id")
	if strings.TrimSpace(eventID) == "" {
		eventID = r.PathValue("id")
	}
	if strings.TrimSpace(eventID) == "" {
		utils.WriteError(w, http.StatusBadRequest, "INVALID_EVENT_ID", "event id is required")
		return
	}

	err := h.syncService.CancelSync(r.Context(), eventID)
	if err != nil {
		if errors.Is(err, service.ErrSyncEventNotFound) {
			utils.WriteError(w, http.StatusNotFound, "NOT_FOUND", "Sync event not found")
			return
		}
		utils.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to cancel sync event")
		return
	}

	utils.WriteJSON(w, http.StatusOK, map[string]string{
		"event_id": eventID,
		"status":   "CANCELLED",
	})
}
