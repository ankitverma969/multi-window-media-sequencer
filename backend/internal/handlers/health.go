package handlers

import (
	"context"
	"net/http"

	"github.com/eva-bharat/media-sequencer/backend/internal/utils"
)

// Pinger is an interface for verifying database liveness.
type Pinger interface {
	Ping(ctx context.Context) error
}

type HealthHandler struct {
	pinger      Pinger
	environment string
}

func NewHealthHandler(pinger Pinger, environment string) *HealthHandler {
	return &HealthHandler{
		pinger:      pinger,
		environment: environment,
	}
}

type HealthResponse struct {
	Status      string `json:"status"`
	Database    string `json:"database"`
	Environment string `json:"environment"`
}

func (h *HealthHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	dbStatus := "connected"
	if h.pinger != nil {
		if err := h.pinger.Ping(r.Context()); err != nil {
			dbStatus = "disconnected"
			utils.WriteError(w, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "Persistent storage is currently unreachable")
			return
		}
	}

	resp := HealthResponse{
		Status:      "ok",
		Database:    dbStatus,
		Environment: h.environment,
	}

	utils.WriteJSON(w, http.StatusOK, resp)
}
