package handlers

import (
	"net/http"

	"github.com/eva-bharat/media-sequencer/backend/internal/config"
	"github.com/eva-bharat/media-sequencer/backend/internal/middleware"
	"github.com/eva-bharat/media-sequencer/backend/internal/service"
)

type Dependencies struct {
	Config          *config.Config
	Pinger          Pinger
	WindowService   service.WindowService
	MediaService    service.MediaService
	PlaylistService service.PlaylistService
}

// NewRouter constructs and configures the HTTP multiplexer and middleware chain.
func NewRouter(deps Dependencies) http.Handler {
	mux := http.NewServeMux()

	healthHandler := NewHealthHandler(deps.Pinger, deps.Config.Environment)
	timeHandler := NewTimeHandler()
	mediaHandler := NewMediaHandler(deps.MediaService)
	windowHandler := NewWindowHandler(deps.WindowService)
	playlistHandler := NewPlaylistHandler(deps.PlaylistService)

	// Health check endpoints
	mux.HandleFunc("GET /health", healthHandler.HealthCheck)
	mux.HandleFunc("GET /api/v1/health", healthHandler.HealthCheck)

	// System time endpoint
	mux.HandleFunc("GET /api/v1/time", timeHandler.GetServerTime)

	// Media catalog endpoints
	mux.HandleFunc("GET /api/v1/media", mediaHandler.ListMedia)
	mux.HandleFunc("POST /api/v1/media", mediaHandler.CreateMedia)

	// Window endpoints
	mux.HandleFunc("GET /api/v1/windows", windowHandler.ListWindows)
	mux.HandleFunc("GET /api/v1/windows/{id}", windowHandler.GetWindow)

	// Playlist endpoints
	mux.HandleFunc("GET /api/v1/windows/{id}/playlist", playlistHandler.GetPlaylist)
	mux.HandleFunc("POST /api/v1/windows/{id}/playlist/items", playlistHandler.AddPlaylistItem)
	mux.HandleFunc("DELETE /api/v1/windows/{id}/playlist/items/{itemId}", playlistHandler.RemovePlaylistItem)
	mux.HandleFunc("GET /api/v1/windows/{id}/playback-state", playlistHandler.GetPlaybackState)

	// Middleware chain: Recovery -> Logger -> CORS -> Mux
	corsMiddleware := middleware.CORS(deps.Config.CORSAllowedOrigins)
	return middleware.Recovery(middleware.Logger(corsMiddleware(mux)))
}
