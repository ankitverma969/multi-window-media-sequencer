package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/eva-bharat/media-sequencer/backend/internal/models"
	"github.com/eva-bharat/media-sequencer/backend/internal/repository"
)

type defaultWindowService struct {
	windowRepo repository.WindowRepository
}

func NewWindowService(windowRepo repository.WindowRepository) WindowService {
	return &defaultWindowService{
		windowRepo: windowRepo,
	}
}

func (s *defaultWindowService) ListWindows(ctx context.Context) ([]models.Window, error) {
	return s.windowRepo.ListAll(ctx)
}

func (s *defaultWindowService) GetWindow(ctx context.Context, idOrNumber string) (*models.Window, error) {
	w, err := s.windowRepo.FindByIdOrNumber(ctx, idOrNumber)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrWindowNotFound
		}
		return nil, fmt.Errorf("failed to retrieve window %s: %w", idOrNumber, err)
	}
	return w, nil
}

func (s *defaultWindowService) EnsureWindow(ctx context.Context, windowNumber int, name string) (*models.Window, error) {
	existing, err := s.windowRepo.FindByNumber(ctx, windowNumber)
	if err == nil && existing != nil {
		return existing, nil
	}

	w := &models.Window{
		WindowNumber:         windowNumber,
		Name:                 name,
		CycleDurationSeconds: 18000,                                     // 5 hours in seconds
		CycleStartTime:       time.Now().UTC().Truncate(24 * time.Hour), // Canonical daily midnight origin
		IsActive:             true,
	}

	if err := w.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidInput, err.Error())
	}

	if err := s.windowRepo.Create(ctx, w); err != nil {
		if errors.Is(err, repository.ErrDuplicateKey) {
			return s.windowRepo.FindByNumber(ctx, windowNumber)
		}
		return nil, fmt.Errorf("failed to create window %d: %w", windowNumber, err)
	}

	return w, nil
}
