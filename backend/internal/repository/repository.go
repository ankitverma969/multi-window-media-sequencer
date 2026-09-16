package repository

import (
	"context"
	"errors"
	"time"

	"github.com/eva-bharat/media-sequencer/backend/internal/models"
	"go.mongodb.org/mongo-driver/v2/bson"
)

var (
	// ErrNotFound indicates that the requested document does not exist.
	ErrNotFound = errors.New("resource not found")

	// ErrDuplicateKey indicates an index violation (e.g. duplicate key or number).
	ErrDuplicateKey = errors.New("duplicate resource key")

	// ErrConflict indicates a concurrent modification conflict.
	ErrConflict = errors.New("concurrent update conflict")
)

// MediaRepository defines persistence operations for media assets.
type MediaRepository interface {
	Create(ctx context.Context, m *models.Media) error
	FindByKey(ctx context.Context, mediaKey string) (*models.Media, error)
	FindByIdOrKey(ctx context.Context, idOrKey string) (*models.Media, error)
	ListAll(ctx context.Context) ([]models.Media, error)
	Upsert(ctx context.Context, m *models.Media) error
}

// WindowRepository defines persistence operations for display windows.
type WindowRepository interface {
	Create(ctx context.Context, w *models.Window) error
	FindByNumber(ctx context.Context, windowNumber int) (*models.Window, error)
	FindByIdOrNumber(ctx context.Context, idOrNumber string) (*models.Window, error)
	ListAll(ctx context.Context) ([]models.Window, error)
	Upsert(ctx context.Context, w *models.Window) error
}

// PlaylistRepository defines persistence operations for window playlists.
type PlaylistRepository interface {
	FindByWindowNumber(ctx context.Context, windowNumber int) (*models.Playlist, error)
	FindByWindowIdOrNumber(ctx context.Context, idOrNumber string) (*models.Playlist, error)
	AppendItem(ctx context.Context, windowNumber int, windowID bson.ObjectID, item models.PlaylistItem) (*models.Playlist, error)
	UpdateItems(ctx context.Context, windowNumber int, windowID bson.ObjectID, items []models.PlaylistItem) (*models.Playlist, error)
	RemoveItem(ctx context.Context, windowNumber int, windowID bson.ObjectID, itemID string) (*models.Playlist, error)
	Upsert(ctx context.Context, p *models.Playlist) error
}

// SyncRepository defines persistence operations for sync override events.
type SyncRepository interface {
	Create(ctx context.Context, s *models.SyncEvent) error
	FindActive(ctx context.Context, now time.Time) (*models.SyncEvent, error)
	FindByID(ctx context.Context, eventID string) (*models.SyncEvent, error)
	UpdateStatus(ctx context.Context, eventID string, status models.SyncStatus) error
}
