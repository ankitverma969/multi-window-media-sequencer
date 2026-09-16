package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/eva-bharat/media-sequencer/backend/internal/models"
	"github.com/eva-bharat/media-sequencer/backend/internal/repository"
	"github.com/eva-bharat/media-sequencer/backend/internal/timeline"
	"github.com/google/uuid"
)

type defaultPlaylistService struct {
	playlistRepo   repository.PlaylistRepository
	mediaRepo      repository.MediaRepository
	windowRepo     repository.WindowRepository
	timelineEngine *timeline.Engine
	mu             sync.RWMutex
	onUpdate       func(windowNumber int, playlist *models.Playlist)
}

func NewPlaylistService(
	playlistRepo repository.PlaylistRepository,
	mediaRepo repository.MediaRepository,
	windowRepo repository.WindowRepository,
) PlaylistService {
	return &defaultPlaylistService{
		playlistRepo:   playlistRepo,
		mediaRepo:      mediaRepo,
		windowRepo:     windowRepo,
		timelineEngine: timeline.NewEngine(),
	}
}

func (s *defaultPlaylistService) SetUpdateListener(fn func(windowNumber int, playlist *models.Playlist)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onUpdate = fn
}

func (s *defaultPlaylistService) notifyUpdate(windowNumber int, playlist *models.Playlist) {
	s.mu.RLock()
	fn := s.onUpdate
	s.mu.RUnlock()
	if fn != nil {
		go fn(windowNumber, playlist)
	}
}

func (s *defaultPlaylistService) GetPlaylist(ctx context.Context, idOrNumber string) (*models.Playlist, error) {
	p, err := s.playlistRepo.FindByWindowIdOrNumber(ctx, idOrNumber)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrPlaylistNotFound
		}
		return nil, fmt.Errorf("failed to retrieve playlist for window %s: %w", idOrNumber, err)
	}
	return p, nil
}

func (s *defaultPlaylistService) AddPlaylistItem(ctx context.Context, idOrNumber string, mediaKey string, customDuration int) (*models.Playlist, error) {
	// Verify window exists
	window, err := s.windowRepo.FindByIdOrNumber(ctx, idOrNumber)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrWindowNotFound
		}
		return nil, fmt.Errorf("failed to verify window %s: %w", idOrNumber, err)
	}

	// Verify media asset exists
	media, err := s.mediaRepo.FindByIdOrKey(ctx, mediaKey)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrMediaNotFound
		}
		return nil, fmt.Errorf("failed to verify media %s: %w", mediaKey, err)
	}

	if customDuration < 0 {
		return nil, fmt.Errorf("%w: custom duration cannot be negative", ErrInvalidInput)
	}

	duration := media.DurationSeconds
	if customDuration > 0 {
		duration = customDuration
	}

	if duration <= 0 {
		return nil, fmt.Errorf("%w: item duration must be strictly positive", ErrInvalidInput)
	}

	item := models.PlaylistItem{
		ItemID:          uuid.New().String(),
		MediaKey:        media.MediaKey,
		Type:            media.Type,
		URL:             media.URL,
		DurationSeconds: duration,
	}

	updatedPlaylist, err := s.playlistRepo.AppendItem(ctx, window.WindowNumber, item)
	if err != nil {
		if errors.Is(err, repository.ErrConflict) {
			return nil, ErrConflict
		}
		return nil, fmt.Errorf("failed to append item to window %d: %w", window.WindowNumber, err)
	}

	s.notifyUpdate(window.WindowNumber, updatedPlaylist)
	return updatedPlaylist, nil
}

func (s *defaultPlaylistService) RemovePlaylistItem(ctx context.Context, idOrNumber string, itemID string) (*models.Playlist, error) {
	window, err := s.windowRepo.FindByIdOrNumber(ctx, idOrNumber)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrWindowNotFound
		}
		return nil, fmt.Errorf("failed to verify window %s: %w", idOrNumber, err)
	}

	updatedPlaylist, err := s.playlistRepo.RemoveItem(ctx, window.WindowNumber, itemID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrPlaylistNotFound
		}
		return nil, fmt.Errorf("failed to remove item %s from window %d: %w", itemID, window.WindowNumber, err)
	}

	s.notifyUpdate(window.WindowNumber, updatedPlaylist)
	return updatedPlaylist, nil
}

func (s *defaultPlaylistService) UpdatePlaylist(ctx context.Context, idOrNumber string, items []models.PlaylistItem) (*models.Playlist, error) {
	window, err := s.windowRepo.FindByIdOrNumber(ctx, idOrNumber)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrWindowNotFound
		}
		return nil, fmt.Errorf("failed to verify window %s: %w", idOrNumber, err)
	}

	// Validate all items in updated playlist
	for i, it := range items {
		if it.DurationSeconds <= 0 && it.DurationMs <= 0 {
			return nil, fmt.Errorf("%w: item at index %d has non-positive duration", ErrInvalidInput, i)
		}
		if it.MediaKey == "" {
			return nil, fmt.Errorf("%w: item at index %d has empty media_key", ErrInvalidInput, i)
		}
		if it.ItemID == "" {
			items[i].ItemID = uuid.New().String()
		}
	}

	updatedPlaylist, err := s.playlistRepo.UpdateItems(ctx, window.WindowNumber, items)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrPlaylistNotFound
		}
		return nil, fmt.Errorf("failed to update playlist for window %d: %w", window.WindowNumber, err)
	}

	s.notifyUpdate(window.WindowNumber, updatedPlaylist)
	return updatedPlaylist, nil
}

func (s *defaultPlaylistService) GetPlaybackState(ctx context.Context, idOrNumber string, queryTime time.Time) (*timeline.PlaybackState, error) {
	window, err := s.windowRepo.FindByIdOrNumber(ctx, idOrNumber)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrWindowNotFound
		}
		return nil, fmt.Errorf("failed to retrieve window %s: %w", idOrNumber, err)
	}

	playlist, err := s.playlistRepo.FindByWindowNumber(ctx, window.WindowNumber)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrPlaylistNotFound
		}
		return nil, fmt.Errorf("failed to retrieve playlist for window %d: %w", window.WindowNumber, err)
	}

	return s.timelineEngine.Calculate(window, playlist, queryTime)
}
