package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/eva-bharat/media-sequencer/backend/internal/models"
	"github.com/eva-bharat/media-sequencer/backend/internal/repository"
)

type defaultMediaService struct {
	mediaRepo repository.MediaRepository
}

func NewMediaService(mediaRepo repository.MediaRepository) MediaService {
	return &defaultMediaService{
		mediaRepo: mediaRepo,
	}
}

func (s *defaultMediaService) ListMedia(ctx context.Context) ([]models.Media, error) {
	return s.mediaRepo.ListAll(ctx)
}

func (s *defaultMediaService) GetMedia(ctx context.Context, idOrKey string) (*models.Media, error) {
	m, err := s.mediaRepo.FindByIdOrKey(ctx, idOrKey)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrMediaNotFound
		}
		return nil, fmt.Errorf("failed to retrieve media %s: %w", idOrKey, err)
	}
	return m, nil
}

func (s *defaultMediaService) CreateMedia(ctx context.Context, m *models.Media) error {
	if err := m.Validate(); err != nil {
		return fmt.Errorf("%w: %s", ErrInvalidInput, err.Error())
	}

	if err := s.mediaRepo.Create(ctx, m); err != nil {
		if errors.Is(err, repository.ErrDuplicateKey) {
			return fmt.Errorf("%w: media key '%s' already exists", ErrInvalidInput, m.MediaKey)
		}
		return fmt.Errorf("failed to create media: %w", err)
	}
	return nil
}
