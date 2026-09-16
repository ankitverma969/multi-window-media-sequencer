package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/eva-bharat/media-sequencer/backend/internal/models"
	"github.com/eva-bharat/media-sequencer/backend/internal/repository"
	"github.com/google/uuid"
)

type defaultSyncService struct {
	syncRepo  repository.SyncRepository
	mediaRepo repository.MediaRepository
}

func NewSyncService(syncRepo repository.SyncRepository, mediaRepo repository.MediaRepository) SyncService {
	return &defaultSyncService{
		syncRepo:  syncRepo,
		mediaRepo: mediaRepo,
	}
}

func (s *defaultSyncService) TriggerSync(
	ctx context.Context,
	mediaKey string,
	durationSeconds int,
	leadTimeMs int,
	triggeredBy string,
) (*models.SyncEvent, error) {
	if durationSeconds <= 0 {
		return nil, fmt.Errorf("%w: duration_seconds must be positive", ErrInvalidInput)
	}

	media, err := s.mediaRepo.FindByIdOrKey(ctx, mediaKey)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrMediaNotFound
		}
		return nil, fmt.Errorf("failed to verify sync media %s: %w", mediaKey, err)
	}

	if leadTimeMs < 0 {
		leadTimeMs = 1000
	}

	now := time.Now().UTC()
	startTime := now.Add(time.Duration(leadTimeMs) * time.Millisecond)
	endTime := startTime.Add(time.Duration(durationSeconds) * time.Second)

	if triggeredBy == "" {
		triggeredBy = "api"
	}

	event := &models.SyncEvent{
		EventID:  fmt.Sprintf("sync_%s", uuid.New().String()[:8]),
		MediaKey: media.MediaKey,
		MediaSnapshot: models.MediaSnapshot{
			MediaKey: media.MediaKey,
			Name:     media.Name,
			Type:     media.Type,
			URL:      media.URL,
		},
		DurationSeconds: durationSeconds,
		StartTime:       startTime,
		EndTime:         endTime,
		Status:          models.SyncStatusScheduled,
		TriggeredBy:     triggeredBy,
		CreatedAt:       now,
	}

	if err := event.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidInput, err.Error())
	}

	if err := s.syncRepo.Create(ctx, event); err != nil {
		return nil, fmt.Errorf("failed to persist sync event: %w", err)
	}

	return event, nil
}

func (s *defaultSyncService) GetActiveSync(ctx context.Context) (*models.SyncEvent, error) {
	return s.syncRepo.FindActive(ctx, time.Now().UTC())
}

func (s *defaultSyncService) GetSyncEvent(ctx context.Context, eventID string) (*models.SyncEvent, error) {
	event, err := s.syncRepo.FindByID(ctx, eventID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrSyncEventNotFound
		}
		return nil, fmt.Errorf("failed to retrieve sync event %s: %w", eventID, err)
	}
	return event, nil
}

func (s *defaultSyncService) CancelSync(ctx context.Context, eventID string) error {
	err := s.syncRepo.UpdateStatus(ctx, eventID, models.SyncStatusCancelled)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrSyncEventNotFound
		}
		return fmt.Errorf("failed to cancel sync event %s: %w", eventID, err)
	}
	return nil
}
