package service

import (
	"context"
	"errors"
	"fmt"
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

func (s *defaultPlaylistService) GetPlaylist(ctx context.Context, windowNumber int) (*models.Playlist, error) {
	p, err := s.playlistRepo.FindByWindowNumber(ctx, windowNumber)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrPlaylistNotFound
		}
		return nil, fmt.Errorf("failed to retrieve playlist for window %d: %w", windowNumber, err)
	}
	return p, nil
}

func (s *defaultPlaylistService) AddPlaylistItem(ctx context.Context, windowNumber int, mediaKey string, customDuration int) (*models.Playlist, error) {
	// Verify window exists
	_, err := s.windowRepo.FindByNumber(ctx, windowNumber)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrWindowNotFound
		}
		return nil, fmt.Errorf("failed to verify window %d: %w", windowNumber, err)
	}

	// Verify media asset exists
	media, err := s.mediaRepo.FindByKey(ctx, mediaKey)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrMediaNotFound
		}
		return nil, fmt.Errorf("failed to verify media %s: %w", mediaKey, err)
	}

	duration := media.DurationSeconds
	if customDuration > 0 {
		duration = customDuration
	}

	item := models.PlaylistItem{
		ItemID:          uuid.New().String(),
		MediaKey:        media.MediaKey,
		Type:            media.Type,
		URL:             media.URL,
		DurationSeconds: duration,
	}

	updatedPlaylist, err := s.playlistRepo.AppendItem(ctx, windowNumber, item)
	if err != nil {
		return nil, fmt.Errorf("failed to append item to window %d: %w", windowNumber, err)
	}

	return updatedPlaylist, nil
}

func (s *defaultPlaylistService) RemovePlaylistItem(ctx context.Context, windowNumber int, itemID string) (*models.Playlist, error) {
	updatedPlaylist, err := s.playlistRepo.RemoveItem(ctx, windowNumber, itemID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrPlaylistNotFound
		}
		return nil, fmt.Errorf("failed to remove item %s from window %d: %w", itemID, windowNumber, err)
	}
	return updatedPlaylist, nil
}

func (s *defaultPlaylistService) UpdatePlaylist(ctx context.Context, windowNumber int, items []models.PlaylistItem) (*models.Playlist, error) {
	updatedPlaylist, err := s.playlistRepo.UpdateItems(ctx, windowNumber, items)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrPlaylistNotFound
		}
		return nil, fmt.Errorf("failed to update playlist for window %d: %w", windowNumber, err)
	}
	return updatedPlaylist, nil
}

func (s *defaultPlaylistService) GetPlaybackState(ctx context.Context, windowNumber int, queryTime time.Time) (*timeline.PlaybackState, error) {
	window, err := s.windowRepo.FindByNumber(ctx, windowNumber)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrWindowNotFound
		}
		return nil, fmt.Errorf("failed to retrieve window %d: %w", windowNumber, err)
	}

	playlist, err := s.playlistRepo.FindByWindowNumber(ctx, windowNumber)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrPlaylistNotFound
		}
		return nil, fmt.Errorf("failed to retrieve playlist for window %d: %w", windowNumber, err)
	}

	return s.timelineEngine.Calculate(window, playlist, queryTime)
}
