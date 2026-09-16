package repository

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/eva-bharat/media-sequencer/backend/internal/models"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// MockMediaRepository provides an in-memory thread-safe implementation for unit testing.
type MockMediaRepository struct {
	mu    sync.RWMutex
	media map[string]*models.Media
}

func NewMockMediaRepository() *MockMediaRepository {
	return &MockMediaRepository{
		media: make(map[string]*models.Media),
	}
}

func (m *MockMediaRepository) Create(ctx context.Context, item *models.Media) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.media[item.MediaKey]; exists {
		return ErrDuplicateKey
	}
	if item.ID.IsZero() {
		item.ID = bson.NewObjectID()
	}
	copyItem := *item
	m.media[item.MediaKey] = &copyItem
	return nil
}

func (m *MockMediaRepository) FindByKey(ctx context.Context, mediaKey string) (*models.Media, error) {
	return m.FindByIdOrKey(ctx, mediaKey)
}

func (m *MockMediaRepository) FindByIdOrKey(ctx context.Context, idOrKey string) (*models.Media, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, item := range m.media {
		if item.MediaKey == idOrKey || item.ID.Hex() == idOrKey {
			copyItem := *item
			return &copyItem, nil
		}
	}
	return nil, ErrNotFound
}

func (m *MockMediaRepository) ListAll(ctx context.Context) ([]models.Media, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var list []models.Media
	for _, item := range m.media {
		list = append(list, *item)
	}
	return list, nil
}

func (m *MockMediaRepository) Upsert(ctx context.Context, item *models.Media) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if item.ID.IsZero() {
		item.ID = bson.NewObjectID()
	}
	copyItem := *item
	m.media[item.MediaKey] = &copyItem
	return nil
}

// MockWindowRepository provides an in-memory thread-safe implementation for unit testing.
type MockWindowRepository struct {
	mu      sync.RWMutex
	windows map[int]*models.Window
}

func NewMockWindowRepository() *MockWindowRepository {
	return &MockWindowRepository{
		windows: make(map[int]*models.Window),
	}
}

func (m *MockWindowRepository) Create(ctx context.Context, w *models.Window) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.windows[w.WindowNumber]; exists {
		return ErrDuplicateKey
	}
	if w.ID.IsZero() {
		w.ID = bson.NewObjectID()
	}
	copyW := *w
	m.windows[w.WindowNumber] = &copyW
	return nil
}

func (m *MockWindowRepository) FindByNumber(ctx context.Context, windowNumber int) (*models.Window, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	w, exists := m.windows[windowNumber]
	if !exists {
		return nil, ErrNotFound
	}
	copyW := *w
	return &copyW, nil
}

func (m *MockWindowRepository) FindByIdOrNumber(ctx context.Context, idOrNumber string) (*models.Window, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, w := range m.windows {
		if strconv.Itoa(w.WindowNumber) == idOrNumber || w.ID.Hex() == idOrNumber {
			copyW := *w
			return &copyW, nil
		}
	}
	return nil, ErrNotFound
}

func (m *MockWindowRepository) ListAll(ctx context.Context) ([]models.Window, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var list []models.Window
	for _, w := range m.windows {
		list = append(list, *w)
	}
	return list, nil
}

func (m *MockWindowRepository) Upsert(ctx context.Context, w *models.Window) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if w.ID.IsZero() {
		w.ID = bson.NewObjectID()
	}
	copyW := *w
	m.windows[w.WindowNumber] = &copyW
	return nil
}

// MockPlaylistRepository provides an in-memory thread-safe implementation for unit testing.
type MockPlaylistRepository struct {
	mu        sync.RWMutex
	playlists map[int]*models.Playlist
}

func NewMockPlaylistRepository() *MockPlaylistRepository {
	return &MockPlaylistRepository{
		playlists: make(map[int]*models.Playlist),
	}
}

func (m *MockPlaylistRepository) FindByWindowNumber(ctx context.Context, windowNumber int) (*models.Playlist, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, exists := m.playlists[windowNumber]
	if !exists {
		return nil, ErrNotFound
	}
	copyP := *p
	copyP.Items = append([]models.PlaylistItem(nil), p.Items...)
	return &copyP, nil
}

func (m *MockPlaylistRepository) FindByWindowIdOrNumber(ctx context.Context, idOrNumber string) (*models.Playlist, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, p := range m.playlists {
		if strconv.Itoa(p.WindowNumber) == idOrNumber || p.WindowID.Hex() == idOrNumber || p.ID.Hex() == idOrNumber {
			copyP := *p
			copyP.Items = append([]models.PlaylistItem(nil), p.Items...)
			return &copyP, nil
		}
	}
	return nil, ErrNotFound
}

func (m *MockPlaylistRepository) AppendItem(ctx context.Context, windowNumber int, windowID bson.ObjectID, item models.PlaylistItem) (*models.Playlist, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, exists := m.playlists[windowNumber]
	if !exists {
		return nil, ErrNotFound
	}
	item.Order = len(p.Items) + 1
	p.Items = append(p.Items, item)
	p.WindowID = windowID // stamp canonical window ObjectID
	p.Recalculate()
	copyP := *p
	copyP.Items = append([]models.PlaylistItem(nil), p.Items...)
	return &copyP, nil
}

func (m *MockPlaylistRepository) UpdateItems(ctx context.Context, windowNumber int, windowID bson.ObjectID, items []models.PlaylistItem) (*models.Playlist, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, exists := m.playlists[windowNumber]
	if !exists {
		return nil, ErrNotFound
	}
	p.Items = items
	p.WindowID = windowID // stamp canonical window ObjectID
	p.Recalculate()
	copyP := *p
	copyP.Items = append([]models.PlaylistItem(nil), p.Items...)
	return &copyP, nil
}

func (m *MockPlaylistRepository) RemoveItem(ctx context.Context, windowNumber int, windowID bson.ObjectID, itemID string) (*models.Playlist, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, exists := m.playlists[windowNumber]
	if !exists {
		return nil, ErrNotFound
	}
	found := false
	var updated []models.PlaylistItem
	for _, it := range p.Items {
		if it.ItemID == itemID {
			found = true
			continue
		}
		updated = append(updated, it)
	}
	if !found {
		return nil, ErrNotFound
	}
	p.Items = updated
	p.WindowID = windowID // stamp canonical window ObjectID
	p.Recalculate()
	copyP := *p
	copyP.Items = append([]models.PlaylistItem(nil), p.Items...)
	return &copyP, nil
}

func (m *MockPlaylistRepository) Upsert(ctx context.Context, p *models.Playlist) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if p.ID.IsZero() {
		p.ID = bson.NewObjectID()
	}
	p.Recalculate()
	copyP := *p
	copyP.Items = append([]models.PlaylistItem(nil), p.Items...)
	m.playlists[p.WindowNumber] = &copyP
	return nil
}

// MockSyncRepository provides an in-memory thread-safe implementation for unit testing.
type MockSyncRepository struct {
	mu     sync.RWMutex
	events []*models.SyncEvent
}

func NewMockSyncRepository() *MockSyncRepository {
	return &MockSyncRepository{
		events: []*models.SyncEvent{},
	}
}

func (m *MockSyncRepository) Create(ctx context.Context, s *models.SyncEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s.ID.IsZero() {
		s.ID = bson.NewObjectID()
	}
	copyS := *s
	m.events = append(m.events, &copyS)
	return nil
}

func (m *MockSyncRepository) FindActive(ctx context.Context, now time.Time) (*models.SyncEvent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for i := len(m.events) - 1; i >= 0; i-- {
		ev := m.events[i]
		if (ev.Status == models.SyncStatusScheduled || ev.Status == models.SyncStatusActive) && ev.EndTime.After(now) {
			copyEv := *ev
			return &copyEv, nil
		}
	}
	return nil, nil
}

func (m *MockSyncRepository) FindByID(ctx context.Context, eventID string) (*models.SyncEvent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, ev := range m.events {
		if ev.EventID == eventID || ev.ID.Hex() == eventID {
			copyEv := *ev
			return &copyEv, nil
		}
	}
	return nil, ErrNotFound
}

func (m *MockSyncRepository) UpdateStatus(ctx context.Context, eventID string, status models.SyncStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, ev := range m.events {
		if ev.EventID == eventID {
			ev.Status = status
			return nil
		}
	}
	return ErrNotFound
}
