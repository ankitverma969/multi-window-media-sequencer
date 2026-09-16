package service

import (
	"context"
	"errors"
	"time"

	"github.com/eva-bharat/media-sequencer/backend/internal/models"
	"github.com/eva-bharat/media-sequencer/backend/internal/timeline"
)

var (
	ErrInvalidInput      = errors.New("invalid input data")
	ErrWindowNotFound    = errors.New("window not found")
	ErrMediaNotFound     = errors.New("media not found")
	ErrPlaylistNotFound  = errors.New("playlist not found")
	ErrSyncEventNotFound = errors.New("sync event not found")
	ErrConflict          = errors.New("conflict during operation")
)

type WindowService interface {
	ListWindows(ctx context.Context) ([]models.Window, error)
	GetWindow(ctx context.Context, idOrNumber string) (*models.Window, error)
	EnsureWindow(ctx context.Context, windowNumber int, name string) (*models.Window, error)
}

type MediaService interface {
	ListMedia(ctx context.Context) ([]models.Media, error)
	GetMedia(ctx context.Context, idOrKey string) (*models.Media, error)
	CreateMedia(ctx context.Context, m *models.Media) error
}

type PlaylistService interface {
	GetPlaylist(ctx context.Context, idOrNumber string) (*models.Playlist, error)
	AddPlaylistItem(ctx context.Context, idOrNumber string, mediaKey string, customDuration int) (*models.Playlist, error)
	RemovePlaylistItem(ctx context.Context, idOrNumber string, itemID string) (*models.Playlist, error)
	UpdatePlaylist(ctx context.Context, idOrNumber string, items []models.PlaylistItem) (*models.Playlist, error)
	GetPlaybackState(ctx context.Context, idOrNumber string, queryTime time.Time) (*timeline.PlaybackState, error)
	SetUpdateListener(fn func(windowNumber int, playlist *models.Playlist))
}

type SyncService interface {
	TriggerSync(ctx context.Context, mediaKey string, durationSeconds int, leadTimeMs int, triggeredBy string) (*models.SyncEvent, error)
	GetActiveSync(ctx context.Context) (*models.SyncEvent, error)
	GetSyncEvent(ctx context.Context, eventID string) (*models.SyncEvent, error)
	CancelSync(ctx context.Context, eventID string) error
}
