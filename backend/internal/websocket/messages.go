package websocket

import (
	"time"

	"github.com/eva-bharat/media-sequencer/backend/internal/models"
	"github.com/eva-bharat/media-sequencer/backend/internal/timeline"
)

// EventType defines strongly typed real-time event names.
type EventType string

const (
	// Server to Client Events
	EventStateSnapshot   EventType = "STATE_SNAPSHOT"
	EventPlaylistUpdated EventType = "PLAYLIST_UPDATED"
	EventSyncStarted     EventType = "SYNC_STARTED"
	EventSyncEnded       EventType = "SYNC_ENDED"
	EventError           EventType = "ERROR"
	EventPong            EventType = "PONG"

	// Client to Server Events
	EventRegisterWindow EventType = "REGISTER_WINDOW"
	EventPing           EventType = "PING"
)

// Envelope wraps all outbound and inbound WebSocket messages with consistent metadata.
type Envelope struct {
	Type      EventType   `json:"type"`
	Version   int         `json:"version"`
	Timestamp time.Time   `json:"timestamp"`
	Payload   interface{} `json:"payload"`
}

// NewEnvelope creates a standard versioned envelope with UTC timestamp.
func NewEnvelope(eventType EventType, payload interface{}) Envelope {
	return Envelope{
		Type:      eventType,
		Version:   1,
		Timestamp: time.Now().UTC(),
		Payload:   payload,
	}
}

// ClientInboundMessage represents a parsed incoming message from a client.
type ClientInboundMessage struct {
	Type    EventType       `json:"type"`
	Payload RegisterPayload `json:"payload"`
}

// RegisterPayload holds the window assignment requested by a client.
type RegisterPayload struct {
	WindowNumber int `json:"window_number"`
}

// ActiveSyncSnapshot represents active sync information tailored for the client display.
type ActiveSyncSnapshot struct {
	EventID         string               `json:"event_id"`
	MediaKey        string               `json:"media_key"`
	MediaSnapshot   models.MediaSnapshot `json:"media"`
	DurationSeconds int                  `json:"duration_seconds"`
	StartTime       time.Time            `json:"start_time"`
	EndTime         time.Time            `json:"end_time"`
	Status          models.SyncStatus    `json:"status"`
	IsActive        bool                 `json:"is_active"`
	SyncOffsetMs    int64                `json:"sync_offset_ms"`
}

// StateSnapshotPayload is the initial payload delivered immediately upon window connection.
type StateSnapshotPayload struct {
	WindowNumber   int                     `json:"window_number"`
	Playlist       *models.Playlist        `json:"playlist"`
	NormalPlayback *timeline.PlaybackState `json:"normal_playback"`
	ActiveSync     *ActiveSyncSnapshot     `json:"active_sync"`
	ServerTime     time.Time               `json:"server_time"`
}

// SyncStartedPayload describes a newly triggered global synchronization override.
type SyncStartedPayload struct {
	EventID         string               `json:"event_id"`
	MediaKey        string               `json:"media_key"`
	MediaSnapshot   models.MediaSnapshot `json:"media"`
	DurationSeconds int                  `json:"duration_seconds"`
	StartTime       time.Time            `json:"start_time"`
	EndTime         time.Time            `json:"end_time"`
	ServerTime      time.Time            `json:"server_time"`
}

// SyncEndedPayload notifies clients that the synchronization override has concluded.
type SyncEndedPayload struct {
	EventID    string    `json:"event_id"`
	Reason     string    `json:"reason"` // "EXPIRED" or "CANCELLED"
	ServerTime time.Time `json:"server_time"`
}

// PlaylistUpdatedPayload notifies clients of a specific window that their playlist has changed.
type PlaylistUpdatedPayload struct {
	WindowNumber   int                     `json:"window_number"`
	Playlist       *models.Playlist        `json:"playlist"`
	NormalPlayback *timeline.PlaybackState `json:"normal_playback"`
	ServerTime     time.Time               `json:"server_time"`
}

// ErrorPayload sends structured error messages to the client.
type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
