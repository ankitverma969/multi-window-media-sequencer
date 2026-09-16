package handlers

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/eva-bharat/media-sequencer/backend/internal/service"
	ws "github.com/eva-bharat/media-sequencer/backend/internal/websocket"
	gorilla "github.com/gorilla/websocket"
)

// WSHandler manages WebSocket upgrade and client lifecycle connections.
type WSHandler struct {
	hub           ws.HubInterface
	windowService service.WindowService
	upgrader      gorilla.Upgrader
}

// NewWSHandler constructs an HTTP handler for WebSocket connections.
func NewWSHandler(hub ws.HubInterface, windowService service.WindowService, allowedOrigins []string) *WSHandler {
	originsMap := make(map[string]bool)
	allowAll := false
	for _, o := range allowedOrigins {
		oTrim := strings.TrimSpace(o)
		if oTrim == "*" {
			allowAll = true
			break
		}
		if oTrim != "" {
			originsMap[oTrim] = true
		}
	}

	return &WSHandler{
		hub:           hub,
		windowService: windowService,
		upgrader: gorilla.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				if allowAll {
					return true
				}
				origin := r.Header.Get("Origin")
				if origin == "" {
					return true
				}
				return originsMap[origin]
			},
		},
	}
}

// HandleWS upgrades an incoming HTTP request to a WebSocket connection.
func (h *WSHandler) HandleWS(w http.ResponseWriter, r *http.Request) {
	// 1. Check for optional window_id or window parameter in URL query
	query := r.URL.Query()
	windowParam := query.Get("window_id")
	if windowParam == "" {
		windowParam = query.Get("window")
	}

	var windowNumber int
	if windowParam != "" {
		// Validate that the requested window exists in the database/service
		window, err := h.windowService.GetWindow(r.Context(), windowParam)
		if err != nil {
			slog.Warn("rejecting websocket connection: invalid window",
				"window_param", windowParam,
				"error", err)
			http.Error(w, "invalid or non-existent window", http.StatusBadRequest)
			return
		}
		windowNumber = window.WindowNumber
	}

	// 2. Perform WebSocket handshake
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("websocket upgrade failed", "error", err)
		return
	}

	// 3. Instantiate client and register with the Hub
	client := ws.NewClient(h.hub.(*ws.Hub), conn, windowNumber)
	h.hub.Register(client)

	// 4. Start concurrent read and write message pumps
	go client.WritePump()
	go client.ReadPump()
}

// ParseWindowNumber safely parses an integer string.
func parseWindowNumber(val string) (int, error) {
	return strconv.Atoi(strings.TrimSpace(val))
}
