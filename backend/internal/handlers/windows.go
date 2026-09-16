package handlers

import (
	"errors"
	"net/http"

	"github.com/eva-bharat/media-sequencer/backend/internal/service"
	"github.com/eva-bharat/media-sequencer/backend/internal/utils"
)

type WindowHandler struct {
	windowService service.WindowService
}

func NewWindowHandler(windowService service.WindowService) *WindowHandler {
	return &WindowHandler{
		windowService: windowService,
	}
}

func (h *WindowHandler) ListWindows(w http.ResponseWriter, r *http.Request) {
	windows, err := h.windowService.ListWindows(r.Context())
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve windows")
		return
	}
	utils.WriteJSON(w, http.StatusOK, windows)
}

func (h *WindowHandler) GetWindow(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	if idStr == "" {
		idStr = r.PathValue("windowId")
	}
	if idStr == "" {
		utils.WriteError(w, http.StatusBadRequest, "INVALID_WINDOW_ID", "Window ID or number is required")
		return
	}

	window, err := h.windowService.GetWindow(r.Context(), idStr)
	if err != nil {
		if errors.Is(err, service.ErrWindowNotFound) {
			utils.WriteError(w, http.StatusNotFound, "NOT_FOUND", "Window not found")
			return
		}
		utils.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve window")
		return
	}

	utils.WriteJSON(w, http.StatusOK, window)
}
