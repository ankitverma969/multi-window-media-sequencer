package handlers

import (
	"net/http"

	"github.com/eva-bharat/media-sequencer/backend/internal/config"
	"github.com/eva-bharat/media-sequencer/backend/internal/middleware"
	"github.com/eva-bharat/media-sequencer/backend/internal/service"
	ws "github.com/eva-bharat/media-sequencer/backend/internal/websocket"
)

type Dependencies struct {
	Config          *config.Config
	Pinger          Pinger
	WindowService   service.WindowService
	MediaService    service.MediaService
	PlaylistService service.PlaylistService
	SyncService     service.SyncService
	WSHub           ws.HubInterface
}

// NewRouter constructs and configures the HTTP multiplexer and middleware chain.
func NewRouter(deps Dependencies) http.Handler {
	mux := http.NewServeMux()

	healthHandler := NewHealthHandler(deps.Pinger, deps.Config.Environment)
	timeHandler := NewTimeHandler()
	mediaHandler := NewMediaHandler(deps.MediaService)
	windowHandler := NewWindowHandler(deps.WindowService)
	playlistHandler := NewPlaylistHandler(deps.PlaylistService)
	syncHandler := NewSyncHandler(deps.SyncService)

	// WebSocket endpoint
	if deps.WSHub != nil {
		wsHandler := NewWSHandler(deps.WSHub, deps.WindowService, deps.Config.CORSAllowedOrigins)
		mux.HandleFunc("GET /ws", wsHandler.HandleWS)
		mux.HandleFunc("GET /api/v1/ws", wsHandler.HandleWS)
	}

	// Health check endpoints
	mux.HandleFunc("GET /health", healthHandler.HealthCheck)
	mux.HandleFunc("GET /api/v1/health", healthHandler.HealthCheck)

	// System time endpoint
	mux.HandleFunc("GET /api/v1/time", timeHandler.GetServerTime)

	// Media catalog endpoints
	mux.HandleFunc("GET /api/v1/media", mediaHandler.ListMedia)
	mux.HandleFunc("POST /api/v1/media", mediaHandler.CreateMedia)
	mux.HandleFunc("GET /api/v1/media/{id}", mediaHandler.GetMedia)

	// Window endpoints
	mux.HandleFunc("GET /api/v1/windows", windowHandler.ListWindows)
	mux.HandleFunc("GET /api/v1/windows/{id}", windowHandler.GetWindow)

	// Playlist endpoints (supporting both /playlist and /playlist/items routes)
	mux.HandleFunc("GET /api/v1/windows/{id}/playlist", playlistHandler.GetPlaylist)
	mux.HandleFunc("POST /api/v1/windows/{id}/playlist", playlistHandler.AddPlaylistItem)
	mux.HandleFunc("POST /api/v1/windows/{id}/playlist/items", playlistHandler.AddPlaylistItem)
	mux.HandleFunc("PUT /api/v1/windows/{id}/playlist", playlistHandler.UpdatePlaylist)
	mux.HandleFunc("DELETE /api/v1/windows/{id}/playlist/{itemId}", playlistHandler.RemovePlaylistItem)
	mux.HandleFunc("DELETE /api/v1/windows/{id}/playlist/items/{itemId}", playlistHandler.RemovePlaylistItem)

	// Playback state endpoints (derived from 5-hour cycle timeline engine)
	mux.HandleFunc("GET /api/v1/windows/{id}/playback", playlistHandler.GetPlaybackState)
	mux.HandleFunc("GET /api/v1/windows/{id}/playback-state", playlistHandler.GetPlaybackState)

	// Synchronization endpoints
	if deps.SyncService != nil {
		mux.HandleFunc("POST /api/v1/sync", syncHandler.TriggerSync)
		mux.HandleFunc("GET /api/v1/sync/current", syncHandler.GetActiveSync)
		mux.HandleFunc("GET /api/v1/sync/{id}", syncHandler.GetSyncEvent)
		mux.HandleFunc("POST /api/v1/sync/{id}/cancel", syncHandler.CancelSync)
	}

	// Middleware chain: Recovery -> Logger -> CORS -> Mux
	corsMiddleware := middleware.CORS(deps.Config.CORSAllowedOrigins)
	return middleware.Recovery(middleware.Logger(corsMiddleware(mux)))
}
