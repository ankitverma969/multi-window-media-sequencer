package service

import (
	"context"
	"errors"
	"time"

	"github.com/eva-bharat/media-sequencer/backend/internal/models"
	"github.com/eva-bharat/media-sequencer/backend/internal/timeline"
)

var (
	ErrInvalidInput     = errors.New("invalid input data")
	ErrWindowNotFound   = errors.New("window not found")
	ErrMediaNotFound    = errors.New("media not found")
	ErrPlaylistNotFound = errors.New("playlist not found")
)

type WindowService interface {
	ListWindows(ctx context.Context) ([]models.Window, error)
	GetWindow(ctx context.Context, windowNumber int) (*models.Window, error)
	EnsureWindow(ctx context.Context, windowNumber int, name string) (*models.Window, error)
}

type MediaService interface {
	ListMedia(ctx context.Context) ([]models.Media, error)
	GetMedia(ctx context.Context, mediaKey string) (*models.Media, error)
	CreateMedia(ctx context.Context, m *models.Media) error
}

type PlaylistService interface {
	GetPlaylist(ctx context.Context, windowNumber int) (*models.Playlist, error)
	AddPlaylistItem(ctx context.Context, windowNumber int, mediaKey string, customDuration int) (*models.Playlist, error)
	RemovePlaylistItem(ctx context.Context, windowNumber int, itemID string) (*models.Playlist, error)
	UpdatePlaylist(ctx context.Context, windowNumber int, items []models.PlaylistItem) (*models.Playlist, error)
	GetPlaybackState(ctx context.Context, windowNumber int, queryTime time.Time) (*timeline.PlaybackState, error)
}

type SyncService interface {
	GetActiveSync(ctx context.Context) (*models.SyncEvent, error)
}
