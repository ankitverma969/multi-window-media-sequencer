package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/eva-bharat/media-sequencer/backend/internal/models"
	"github.com/eva-bharat/media-sequencer/backend/internal/repository"
	"github.com/eva-bharat/media-sequencer/backend/internal/timeline"
	"github.com/eva-bharat/media-sequencer/backend/internal/websocket"
	"github.com/google/uuid"
)

const (
	// MinSyncDurationSeconds defines the minimum allowable sync duration.
	MinSyncDurationSeconds = 1
	// MaxSyncDurationSeconds defines the maximum allowable sync duration (1 hour).
	MaxSyncDurationSeconds = 3600
	// DefaultLeadTimeMs provides the default preload delay for clients.
	DefaultLeadTimeMs = 1000
	// MaxLeadTimeMs prevents excessive future scheduling delays.
	MaxLeadTimeMs = 10000
)

// SyncCoordinator coordinates authoritative sync events, client broadcasts, and playback snapshots.
type SyncCoordinator interface {
	SyncService
	BuildSnapshotForWindow(ctx context.Context, windowNumber int) (*websocket.StateSnapshotPayload, error)
	RecoverActiveSyncOnStartup(ctx context.Context)
	SetHub(hub websocket.HubInterface)
}

type defaultSyncCoordinator struct {
	syncRepo        repository.SyncRepository
	mediaRepo       repository.MediaRepository
	windowRepo      repository.WindowRepository
	playlistRepo    repository.PlaylistRepository
	playlistService PlaylistService
	hub             websocket.HubInterface
	timelineEngine  *timeline.Engine

	mu              sync.RWMutex
	activeSync      *models.SyncEvent
	expirationTimer *time.Timer
}

// NewSyncCoordinator creates a new authoritative synchronization coordinator.
func NewSyncCoordinator(
	syncRepo repository.SyncRepository,
	mediaRepo repository.MediaRepository,
	windowRepo repository.WindowRepository,
	playlistRepo repository.PlaylistRepository,
	playlistService PlaylistService,
	hub websocket.HubInterface,
) SyncCoordinator {
	c := &defaultSyncCoordinator{
		syncRepo:        syncRepo,
		mediaRepo:       mediaRepo,
		windowRepo:      windowRepo,
		playlistRepo:    playlistRepo,
		playlistService: playlistService,
		hub:             hub,
		timelineEngine:  timeline.NewEngine(),
	}

	if hub != nil {
		c.bindHub(hub)
	}

	if playlistService != nil {
		playlistService.SetUpdateListener(c.handlePlaylistUpdated)
	}

	return c
}

// SetHub updates the WebSocket Hub reference and establishes registration listeners.
func (c *defaultSyncCoordinator) SetHub(hub websocket.HubInterface) {
	c.mu.Lock()
	c.hub = hub
	c.mu.Unlock()
	c.bindHub(hub)
}

func (c *defaultSyncCoordinator) bindHub(hub websocket.HubInterface) {
	hub.SetWindowRegisterHandler(func(client *websocket.Client, windowNumber int) {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			snapshot, err := c.BuildSnapshotForWindow(ctx, windowNumber)
			if err != nil {
				slog.Error("failed to generate state snapshot for window",
					"window", windowNumber,
					"error", err)
				client.SendEnvelope(websocket.NewEnvelope(websocket.EventError, websocket.ErrorPayload{
					Code:    "SNAPSHOT_FAILED",
					Message: fmt.Sprintf("failed to load initial state for window %d", windowNumber),
				}))
				return
			}

			client.SendEnvelope(websocket.NewEnvelope(websocket.EventStateSnapshot, snapshot))
		}()
	})
}

func (c *defaultSyncCoordinator) handlePlaylistUpdated(windowNumber int, playlist *models.Playlist) {
	if c.hub == nil {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		window, err := c.windowRepo.FindByNumber(ctx, windowNumber)
		if err != nil {
			slog.Error("failed to retrieve window for playlist update broadcast",
				"window", windowNumber, "error", err)
			return
		}

		now := time.Now().UTC()
		playbackState, err := c.timelineEngine.Calculate(window, playlist, now)
		if err != nil {
			slog.Error("failed to calculate playback state for updated playlist",
				"window", windowNumber, "error", err)
			return
		}

		payload := websocket.PlaylistUpdatedPayload{
			WindowNumber:   windowNumber,
			Playlist:       playlist,
			NormalPlayback: playbackState,
			ServerTime:     now,
		}

		c.hub.BroadcastToWindow(windowNumber, websocket.NewEnvelope(websocket.EventPlaylistUpdated, payload))
		slog.Info("broadcasted playlist update to window clients",
			"window", windowNumber,
			"items_count", len(playlist.Items),
			"version", playlist.Version)
	}()
}

func (c *defaultSyncCoordinator) TriggerSync(
	ctx context.Context,
	mediaKey string,
	durationSeconds int,
	leadTimeMs int,
	triggeredBy string,
) (*models.SyncEvent, error) {
	// 1. Validate sync duration constraints
	if durationSeconds < MinSyncDurationSeconds {
		return nil, fmt.Errorf("%w: duration_seconds must be at least %d second",
			ErrInvalidInput, MinSyncDurationSeconds)
	}
	if durationSeconds > MaxSyncDurationSeconds {
		return nil, fmt.Errorf("%w: duration_seconds exceeds maximum limit of %d seconds",
			ErrInvalidInput, MaxSyncDurationSeconds)
	}

	// 2. Validate lead time bounds
	if leadTimeMs < 0 {
		leadTimeMs = DefaultLeadTimeMs
	} else if leadTimeMs > MaxLeadTimeMs {
		leadTimeMs = MaxLeadTimeMs
	}

	// 3. Verify media exists
	media, err := c.mediaRepo.FindByIdOrKey(ctx, mediaKey)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrMediaNotFound
		}
		return nil, fmt.Errorf("failed to verify sync media %s: %w", mediaKey, err)
	}

	// 4. Compute authoritative server timestamps
	now := time.Now().UTC()
	startTime := now.Add(time.Duration(leadTimeMs) * time.Millisecond)
	endTime := startTime.Add(time.Duration(durationSeconds) * time.Second)

	if triggeredBy == "" {
		triggeredBy = "api"
	}

	status := models.SyncStatusScheduled
	if startTime.Before(now) || startTime.Equal(now) {
		status = models.SyncStatusActive
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
		Status:          status,
		TriggeredBy:     triggeredBy,
		CreatedAt:       now,
	}

	if err := event.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidInput, err.Error())
	}

	// 5. Conflict Resolution: "Replace Active Sync" policy
	c.mu.Lock()
	if c.expirationTimer != nil {
		c.expirationTimer.Stop()
		c.expirationTimer = nil
	}
	if c.activeSync != nil {
		oldEventID := c.activeSync.EventID
		// Mark superseded sync as cancelled in background
		go func(id string) {
			updateCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = c.syncRepo.UpdateStatus(updateCtx, id, models.SyncStatusCancelled)
		}(oldEventID)
		slog.Info("previous active sync superseded by new sync request",
			"old_event_id", oldEventID,
			"new_event_id", event.EventID)
	}

	// 6. Persist new sync event
	if err := c.syncRepo.Create(ctx, event); err != nil {
		c.mu.Unlock()
		return nil, fmt.Errorf("failed to persist sync event: %w", err)
	}

	// 7. Schedule authoritative expiration timer
	c.activeSync = event
	untilEnd := time.Until(endTime)
	eventID := event.EventID
	c.expirationTimer = time.AfterFunc(untilEnd, func() {
		c.handleSyncExpiration(eventID)
	})
	c.mu.Unlock()

	slog.Info("authoritative sync event created and scheduled",
		"event_id", event.EventID,
		"media_key", event.MediaKey,
		"start_time", event.StartTime.Format(time.RFC3339Nano),
		"end_time", event.EndTime.Format(time.RFC3339Nano),
		"duration_seconds", event.DurationSeconds,
		"lead_time_ms", leadTimeMs)

	// 8. Broadcast SYNC_STARTED to all connected window displays
	if c.hub != nil {
		payload := websocket.SyncStartedPayload{
			EventID:         event.EventID,
			MediaKey:        event.MediaKey,
			MediaSnapshot:   event.MediaSnapshot,
			DurationSeconds: event.DurationSeconds,
			StartTime:       event.StartTime,
			EndTime:         event.EndTime,
			ServerTime:      time.Now().UTC(),
		}
		c.hub.Broadcast(websocket.NewEnvelope(websocket.EventSyncStarted, payload))
	}

	return event, nil
}

func (c *defaultSyncCoordinator) handleSyncExpiration(eventID string) {
	c.mu.Lock()
	if c.activeSync == nil || c.activeSync.EventID != eventID {
		c.mu.Unlock()
		return
	}
	c.activeSync = nil
	c.expirationTimer = nil
	c.mu.Unlock()

	// Update persistent state to completed
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = c.syncRepo.UpdateStatus(ctx, eventID, models.SyncStatusCompleted)

	slog.Info("sync event duration expired naturally", "event_id", eventID)

	// Broadcast SYNC_ENDED to all connected window displays
	if c.hub != nil {
		payload := websocket.SyncEndedPayload{
			EventID:    eventID,
			Reason:     "EXPIRED",
			ServerTime: time.Now().UTC(),
		}
		c.hub.Broadcast(websocket.NewEnvelope(websocket.EventSyncEnded, payload))
	}
}

func (c *defaultSyncCoordinator) CancelSync(ctx context.Context, eventID string) error {
	c.mu.Lock()
	isCurrent := false
	if c.activeSync != nil && c.activeSync.EventID == eventID {
		isCurrent = true
		if c.expirationTimer != nil {
			c.expirationTimer.Stop()
			c.expirationTimer = nil
		}
		c.activeSync = nil
	}
	c.mu.Unlock()

	err := c.syncRepo.UpdateStatus(ctx, eventID, models.SyncStatusCancelled)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrSyncEventNotFound
		}
		return fmt.Errorf("failed to cancel sync event %s: %w", eventID, err)
	}

	slog.Info("sync event cancelled", "event_id", eventID, "was_current", isCurrent)

	if isCurrent && c.hub != nil {
		payload := websocket.SyncEndedPayload{
			EventID:    eventID,
			Reason:     "CANCELLED",
			ServerTime: time.Now().UTC(),
		}
		c.hub.Broadcast(websocket.NewEnvelope(websocket.EventSyncEnded, payload))
	}

	return nil
}

func (c *defaultSyncCoordinator) GetActiveSync(ctx context.Context) (*models.SyncEvent, error) {
	c.mu.RLock()
	if c.activeSync != nil {
		now := time.Now().UTC()
		if now.Before(c.activeSync.EndTime) {
			eventCopy := *c.activeSync
			c.mu.RUnlock()
			return &eventCopy, nil
		}
	}
	c.mu.RUnlock()

	return c.syncRepo.FindActive(ctx, time.Now().UTC())
}

func (c *defaultSyncCoordinator) GetSyncEvent(ctx context.Context, eventID string) (*models.SyncEvent, error) {
	c.mu.RLock()
	if c.activeSync != nil && c.activeSync.EventID == eventID {
		eventCopy := *c.activeSync
		c.mu.RUnlock()
		return &eventCopy, nil
	}
	c.mu.RUnlock()

	event, err := c.syncRepo.FindByID(ctx, eventID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrSyncEventNotFound
		}
		return nil, fmt.Errorf("failed to retrieve sync event %s: %w", eventID, err)
	}
	return event, nil
}

func (c *defaultSyncCoordinator) BuildSnapshotForWindow(ctx context.Context, windowNumber int) (*websocket.StateSnapshotPayload, error) {
	// 1. Retrieve window entity
	window, err := c.windowRepo.FindByNumber(ctx, windowNumber)
	if err != nil {
		return nil, fmt.Errorf("window %d not found: %w", windowNumber, err)
	}

	// 2. Retrieve window playlist
	playlist, err := c.playlistRepo.FindByWindowNumber(ctx, windowNumber)
	if err != nil {
		return nil, fmt.Errorf("playlist for window %d not found: %w", windowNumber, err)
	}

	// 3. Calculate normal playback state from 5-hour cycle timeline engine
	now := time.Now().UTC()
	normalPlayback, err := c.timelineEngine.Calculate(window, playlist, now)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate playback for window %d: %w", windowNumber, err)
	}

	// 4. Determine current active sync state
	var activeSyncSnap *websocket.ActiveSyncSnapshot
	active, err := c.GetActiveSync(ctx)
	if err == nil && active != nil && now.Before(active.EndTime) {
		isActive := now.After(active.StartTime) || now.Equal(active.StartTime)
		var syncOffsetMs int64 = 0
		if isActive {
			syncOffsetMs = now.Sub(active.StartTime).Milliseconds()
		}

		activeSyncSnap = &websocket.ActiveSyncSnapshot{
			EventID:         active.EventID,
			MediaKey:        active.MediaKey,
			MediaSnapshot:   active.MediaSnapshot,
			DurationSeconds: active.DurationSeconds,
			StartTime:       active.StartTime,
			EndTime:         active.EndTime,
			Status:          active.Status,
			IsActive:        isActive,
			SyncOffsetMs:    syncOffsetMs,
		}
	}

	return &websocket.StateSnapshotPayload{
		WindowNumber:   windowNumber,
		Playlist:       playlist,
		NormalPlayback: normalPlayback,
		ActiveSync:     activeSyncSnap,
		ServerTime:     now,
	}, nil
}

func (c *defaultSyncCoordinator) RecoverActiveSyncOnStartup(ctx context.Context) {
	now := time.Now().UTC()
	active, err := c.syncRepo.FindActive(ctx, now)
	if err != nil || active == nil {
		slog.Info("no in-flight sync event found during startup recovery")
		return
	}

	if now.Before(active.EndTime) {
		c.mu.Lock()
		c.activeSync = active
		untilEnd := time.Until(active.EndTime)
		eventID := active.EventID
		c.expirationTimer = time.AfterFunc(untilEnd, func() {
			c.handleSyncExpiration(eventID)
		})
		c.mu.Unlock()

		slog.Info("recovered active sync event on startup",
			"event_id", active.EventID,
			"media_key", active.MediaKey,
			"remaining", untilEnd.String())
	} else {
		_ = c.syncRepo.UpdateStatus(ctx, active.EventID, models.SyncStatusCompleted)
		slog.Info("marked past sync event completed on startup", "event_id", active.EventID)
	}
}
