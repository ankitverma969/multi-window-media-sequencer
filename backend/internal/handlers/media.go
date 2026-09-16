package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/eva-bharat/media-sequencer/backend/internal/models"
	"github.com/eva-bharat/media-sequencer/backend/internal/service"
	"github.com/eva-bharat/media-sequencer/backend/internal/utils"
)

type MediaHandler struct {
	mediaService service.MediaService
}

func NewMediaHandler(mediaService service.MediaService) *MediaHandler {
	return &MediaHandler{
		mediaService: mediaService,
	}
}

func (h *MediaHandler) ListMedia(w http.ResponseWriter, r *http.Request) {
	list, err := h.mediaService.ListMedia(r.Context())
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve media list")
		return
	}
	utils.WriteJSON(w, http.StatusOK, list)
}

func (h *MediaHandler) CreateMedia(w http.ResponseWriter, r *http.Request) {
	var req models.Media
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON payload")
		return
	}

	if err := h.mediaService.CreateMedia(r.Context(), &req); err != nil {
		if errors.Is(err, service.ErrInvalidInput) {
			utils.WriteError(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
			return
		}
		utils.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create media item")
		return
	}

	utils.WriteJSON(w, http.StatusCreated, req)
}
