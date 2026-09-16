package models

import (
	"errors"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// MediaType represents the valid media types in the sequencer.
type MediaType string

const (
	MediaTypeVideo MediaType = "video"
	MediaTypeImage MediaType = "image"
	MediaTypeBlank MediaType = "blank"
)

// IsValid checks whether the media type is one of the supported types.
func (m MediaType) IsValid() bool {
	switch m {
	case MediaTypeVideo, MediaTypeImage, MediaTypeBlank:
		return true
	default:
		return false
	}
}

// Media represents a registered asset in the media catalog.
type Media struct {
	ID              bson.ObjectID `bson:"_id,omitempty" json:"id"`
	MediaKey        string        `bson:"media_key" json:"media_key"`
	Name            string        `bson:"name" json:"name"`
	Type            MediaType     `bson:"type" json:"type"`
	URL             string        `bson:"url" json:"url"`
	DurationSeconds int           `bson:"duration_seconds" json:"duration_seconds"`
	CreatedAt       time.Time     `bson:"created_at" json:"created_at"`
	UpdatedAt       time.Time     `bson:"updated_at" json:"updated_at"`
}

// Validate ensures media invariants are met.
func (m *Media) Validate() error {
	if strings.TrimSpace(m.MediaKey) == "" {
		return errors.New("media_key is required")
	}
	if strings.TrimSpace(m.Name) == "" {
		return errors.New("name is required")
	}
	if !m.Type.IsValid() {
		return errors.New("invalid media type (must be 'video', 'image', or 'blank')")
	}
	if m.Type != MediaTypeBlank && strings.TrimSpace(m.URL) == "" {
		return errors.New("url is required for non-blank media")
	}
	if m.DurationSeconds <= 0 {
		return errors.New("duration_seconds must be positive")
	}
	return nil
}

// Window represents an independent display output.
type Window struct {
	ID                   bson.ObjectID `bson:"_id,omitempty" json:"id"`
	WindowNumber         int           `bson:"window_number" json:"window_number"`
	Name                 string        `bson:"name" json:"name"`
	CycleDurationSeconds int           `bson:"cycle_duration_seconds" json:"cycle_duration_seconds"` // 18000s = 5 hours
	CycleStartTime       time.Time     `bson:"cycle_start_time" json:"cycle_start_time"`             // Timeline anchor
	IsActive             bool          `bson:"is_active" json:"is_active"`
	CreatedAt            time.Time     `bson:"created_at" json:"created_at"`
	UpdatedAt            time.Time     `bson:"updated_at" json:"updated_at"`
}

// Validate checks window model integrity.
func (w *Window) Validate() error {
	if w.WindowNumber <= 0 {
		return errors.New("window_number must be greater than 0")
	}
	if strings.TrimSpace(w.Name) == "" {
		return errors.New("name is required")
	}
	if w.CycleDurationSeconds <= 0 {
		return errors.New("cycle_duration_seconds must be positive")
	}
	if w.CycleStartTime.IsZero() {
		return errors.New("cycle_start_time must be set")
	}
	return nil
}

// PlaylistItem represents an entry inside a window's sequence.
type PlaylistItem struct {
	ItemID          string    `bson:"item_id" json:"item_id"`
	MediaKey        string    `bson:"media_key" json:"media_key"`
	Type            MediaType `bson:"type" json:"type"`
	URL             string    `bson:"url" json:"url"`
	DurationSeconds int       `bson:"duration_seconds" json:"duration_seconds"`
	Order           int       `bson:"order" json:"order"`
}

// Playlist represents the complete ordered list of media items for a specific window.
type Playlist struct {
	ID                           bson.ObjectID  `bson:"_id,omitempty" json:"id"`
	WindowID                     bson.ObjectID  `bson:"window_id" json:"window_id"`
	WindowNumber                 int            `bson:"window_number" json:"window_number"`
	Items                        []PlaylistItem `bson:"items" json:"items"`
	TotalSequenceDurationSeconds int            `bson:"total_sequence_duration_seconds" json:"total_sequence_duration_seconds"`
	Version                      int64          `bson:"version" json:"version"`
	UpdatedAt                    time.Time      `bson:"updated_at" json:"updated_at"`
}

// Recalculate recalculates total duration and ensures items have sequential ordering.
func (p *Playlist) Recalculate() {
	total := 0
	for i := range p.Items {
		p.Items[i].Order = i + 1
		total += p.Items[i].DurationSeconds
	}
	p.TotalSequenceDurationSeconds = total
	p.Version++
	p.UpdatedAt = time.Now().UTC()
}

// SyncStatus represents the lifecycle of a synchronization event.
type SyncStatus string

const (
	SyncStatusScheduled SyncStatus = "SCHEDULED"
	SyncStatusActive    SyncStatus = "ACTIVE"
	SyncStatusCompleted SyncStatus = "COMPLETED"
	SyncStatusCancelled SyncStatus = "CANCELLED"
)

// MediaSnapshot captures a point-in-time copy of the media to be played in sync.
type MediaSnapshot struct {
	MediaKey string    `bson:"media_key" json:"media_key"`
	Name     string    `bson:"name" json:"name"`
	Type     MediaType `bson:"type" json:"type"`
	URL      string    `bson:"url" json:"url"`
}

// SyncEvent represents a global override where all windows play one media item simultaneously.
type SyncEvent struct {
	ID              bson.ObjectID `bson:"_id,omitempty" json:"id"`
	EventID         string        `bson:"event_id" json:"event_id"`
	MediaKey        string        `bson:"media_key" json:"media_key"`
	MediaSnapshot   MediaSnapshot `bson:"media_snapshot" json:"media_snapshot"`
	DurationSeconds int           `bson:"duration_seconds" json:"duration_seconds"`
	StartTime       time.Time     `bson:"start_time" json:"start_time"`
	EndTime         time.Time     `bson:"end_time" json:"end_time"`
	Status          SyncStatus    `bson:"status" json:"status"`
	TriggeredBy     string        `bson:"triggered_by" json:"triggered_by"`
	CreatedAt       time.Time     `bson:"created_at" json:"created_at"`
}

// Validate checks sync event properties.
func (s *SyncEvent) Validate() error {
	if strings.TrimSpace(s.EventID) == "" {
		return errors.New("event_id is required")
	}
	if strings.TrimSpace(s.MediaKey) == "" {
		return errors.New("media_key is required")
	}
	if s.DurationSeconds <= 0 {
		return errors.New("duration_seconds must be positive")
	}
	if s.StartTime.IsZero() || s.EndTime.IsZero() {
		return errors.New("start_time and end_time must be set")
	}
	if !s.EndTime.After(s.StartTime) {
		return errors.New("end_time must be strictly after start_time")
	}
	return nil
}
