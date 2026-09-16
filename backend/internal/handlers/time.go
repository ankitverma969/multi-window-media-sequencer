package handlers

import (
	"net/http"
	"time"

	"github.com/eva-bharat/media-sequencer/backend/internal/utils"
)

type TimeHandler struct{}

func NewTimeHandler() *TimeHandler {
	return &TimeHandler{}
}

type TimeResponse struct {
	ServerTimeUTC string `json:"server_time_utc"`
	ServerTimeMs  int64  `json:"server_time_ms"`
}

func (h *TimeHandler) GetServerTime(w http.ResponseWriter, r *http.Request) {
	now := time.Now().UTC()
	resp := TimeResponse{
		ServerTimeUTC: now.Format(time.RFC3339Nano),
		ServerTimeMs:  now.UnixMilli(),
	}
	utils.WriteJSON(w, http.StatusOK, resp)
}
