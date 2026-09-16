package timeline

import (
	"errors"
	"fmt"
	"time"

	"github.com/eva-bharat/media-sequencer/backend/internal/models"
)

// FiveHourCycle defines the exact 5-hour window playback cycle (18,000 seconds = 5 * 3600s).
// This constant represents the repeating timeline duration mandated by the specification.
const FiveHourCycle = 18000 * time.Second

// Sentinel errors for timeline calculations.
var (
	ErrNilPlaylist          = errors.New("playlist cannot be nil")
	ErrInvalidItemDuration  = errors.New("playlist item duration must be strictly positive")
	ErrZeroSequenceDuration = errors.New("total playlist sequence duration must be greater than zero")
)

// PlaybackStatus represents the operational playback state of a display window.
type PlaybackStatus string

const (
	// PlaybackStatusNormal indicates active playback of a configured video or image item.
	PlaybackStatusNormal PlaybackStatus = "NORMAL"

	// PlaybackStatusBlank indicates playback of an explicitly configured blank playlist item.
	PlaybackStatusBlank PlaybackStatus = "BLANK"

	// PlaybackStatusFallback indicates fallback state when a window has no configured media items.
	PlaybackStatusFallback PlaybackStatus = "FALLBACK"
)

// PlaybackState encapsulates the exact deterministic state of a display window at a specific timestamp.
type PlaybackState struct {
	WindowNumber      int                  `json:"window_number"`
	Status            PlaybackStatus       `json:"status"`
	CurrentItem       *models.PlaylistItem `json:"current_item,omitempty"`
	MediaKey          string               `json:"media_key"`
	MediaType         models.MediaType     `json:"media_type"`
	MediaURL          string               `json:"media_url"`
	ItemDuration      time.Duration        `json:"item_duration"`
	PlaybackPosition  time.Duration        `json:"playback_position"`   // Offset into current item in [0, ItemDuration)
	TimeRemaining     time.Duration        `json:"time_remaining"`      // Time until next item transition
	CycleDuration     time.Duration        `json:"cycle_duration"`      // Always FiveHourCycle (18,000s)
	CyclePosition     time.Duration        `json:"cycle_position"`      // Position within current 5-hour cycle [0, 18,000s)
	CycleNumber       int64                `json:"cycle_number"`        // Number of 5-hour cycles elapsed since origin
	SequenceIteration int64                `json:"sequence_iteration"`  // Number of sequence iterations within current cycle
	SequenceDuration  time.Duration        `json:"sequence_duration"`   // Total duration of one loop of the playlist
	NextItem          *models.PlaylistItem `json:"next_item,omitempty"` // Next sequential item for preloading
	EvaluatedAt       time.Time            `json:"evaluated_at"`        // Evaluated UTC timestamp
}

// Engine encapsulates the pure timeline calculation logic.
type Engine struct {
	cycleDuration time.Duration
}

// NewEngine creates a new deterministic timeline calculation engine.
// Defaults to FiveHourCycle (18,000 seconds).
func NewEngine() *Engine {
	return &Engine{
		cycleDuration: FiveHourCycle,
	}
}

// NewCustomEngine allows creating an engine with a custom cycle duration for specialized testing.
func NewCustomEngine(cycleDuration time.Duration) (*Engine, error) {
	if cycleDuration <= 0 {
		return nil, errors.New("cycle duration must be strictly positive")
	}
	return &Engine{
		cycleDuration: cycleDuration,
	}, nil
}

// CycleDuration returns the configured cycle duration (18,000s).
func (e *Engine) CycleDuration() time.Duration {
	return e.cycleDuration
}

// Calculate determines the exact active media item and playback position for a window
// at any given query timestamp.
//
// Invariants enforced:
// 1. Half-open interval convention: each item is active during [start, end).
// 2. Unused cycle time does NOT default to blank; sequence continuously repeats.
// 3. Blank occurs ONLY when explicitly configured in the playlist.
// 4. Empty playlist deterministically returns PlaybackStatusFallback.
// 5. Zero or negative item durations return validation errors.
// 6. Independent window isolation: no shared mutable state.
func (e *Engine) Calculate(window *models.Window, playlist *models.Playlist, queryTime time.Time) (*PlaybackState, error) {
	if playlist == nil {
		return nil, ErrNilPlaylist
	}

	windowNumber := 0
	origin := time.Time{}
	if window != nil {
		windowNumber = window.WindowNumber
		origin = window.CycleStartTime
	}

	// If origin is zero, anchor to canonical UTC midnight of the query time
	if origin.IsZero() {
		origin = queryTime.UTC().Truncate(24 * time.Hour)
	}

	// Normalize query time to UTC
	qTimeUTC := queryTime.UTC()
	originUTC := origin.UTC()

	// 1. Handle Empty Playlist
	if len(playlist.Items) == 0 {
		return &PlaybackState{
			WindowNumber:      windowNumber,
			Status:            PlaybackStatusFallback,
			CurrentItem:       nil,
			MediaKey:          "FALLBACK",
			MediaType:         models.MediaTypeBlank,
			MediaURL:          "",
			ItemDuration:      0,
			PlaybackPosition:  0,
			TimeRemaining:     0,
			CycleDuration:     e.cycleDuration,
			CyclePosition:     0,
			CycleNumber:       0,
			SequenceIteration: 0,
			SequenceDuration:  0,
			NextItem:          nil,
			EvaluatedAt:       qTimeUTC,
		}, nil
	}

	// 2. Validate Item Durations & Calculate Total Sequence Duration
	var totalSeqDuration time.Duration
	itemDurations := make([]time.Duration, len(playlist.Items))
	for i, item := range playlist.Items {
		d := getItemDuration(item)
		if d <= 0 {
			return nil, fmt.Errorf("%w: item %s (index %d) has duration %v", ErrInvalidItemDuration, item.ItemID, i, d)
		}
		itemDurations[i] = d
		totalSeqDuration += d
	}

	if totalSeqDuration <= 0 {
		return nil, ErrZeroSequenceDuration
	}

	// 3. Calculate 5-Hour Cycle Position & Cycle Number
	// Δt = (queryTime - origin)
	diff := qTimeUTC.Sub(originUTC)

	var cycleNumber int64
	var cyclePosition time.Duration

	if diff >= 0 {
		cycleNumber = int64(diff / e.cycleDuration)
		cyclePosition = diff % e.cycleDuration
	} else {
		// Negative time handling: query time before origin
		// In Go, (-5) % 10 is -5. Adjust to positive modulo.
		rem := diff % e.cycleDuration
		if rem < 0 {
			rem += e.cycleDuration
			cycleNumber = int64((diff - rem) / e.cycleDuration)
		}
		cyclePosition = rem
	}

	// 4. Calculate Offset Within Repeating Playlist Sequence
	// Under the requirement:
	// "Each window should keep playing its configured list again and again within that cycle.
	//  Blank is only a configured playlist item when included; the rest of the cycle should not become blank playback."
	//
	// The sequence repeats cyclically throughout the 5 hours:
	sequenceIteration := int64(cyclePosition / totalSeqDuration)
	seqOffset := cyclePosition % totalSeqDuration

	// 5. Find Active Item using Half-Open Interval [accumStart, accumStart + duration)
	var accum time.Duration
	var activeIdx int = -1
	var activeItemStart time.Duration

	for i, d := range itemDurations {
		itemStart := accum
		itemEnd := accum + d

		// Half-open interval check: itemStart <= seqOffset < itemEnd
		if seqOffset >= itemStart && seqOffset < itemEnd {
			activeIdx = i
			activeItemStart = itemStart
			break
		}
		accum += d
	}

	// Safety fallback if float/rounding edge occurs at exact boundary
	if activeIdx == -1 {
		activeIdx = len(playlist.Items) - 1
		activeItemStart = totalSeqDuration - itemDurations[activeIdx]
	}

	activeItem := playlist.Items[activeIdx]
	itemDur := itemDurations[activeIdx]
	playbackPos := seqOffset - activeItemStart
	timeRemaining := itemDur - playbackPos

	// Identify Next Sequential Item (for gapless dual-buffer preloading)
	nextIdx := (activeIdx + 1) % len(playlist.Items)
	nextItem := playlist.Items[nextIdx]

	// Determine Status
	status := PlaybackStatusNormal
	if activeItem.Type == models.MediaTypeBlank {
		status = PlaybackStatusBlank
	}

	return &PlaybackState{
		WindowNumber:      windowNumber,
		Status:            status,
		CurrentItem:       &activeItem,
		MediaKey:          activeItem.MediaKey,
		MediaType:         activeItem.Type,
		MediaURL:          activeItem.URL,
		ItemDuration:      itemDur,
		PlaybackPosition:  playbackPos,
		TimeRemaining:     timeRemaining,
		CycleDuration:     e.cycleDuration,
		CyclePosition:     cyclePosition,
		CycleNumber:       cycleNumber,
		SequenceIteration: sequenceIteration,
		SequenceDuration:  totalSeqDuration,
		NextItem:          &nextItem,
		EvaluatedAt:       qTimeUTC,
	}, nil
}

// getItemDuration extracts duration from PlaylistItem with support for DurationMs or DurationSeconds.
func getItemDuration(item models.PlaylistItem) time.Duration {
	if item.DurationMs > 0 {
		return time.Duration(item.DurationMs) * time.Millisecond
	}
	if item.DurationSeconds > 0 {
		return time.Duration(item.DurationSeconds) * time.Second
	}
	return 0
}
