package timeline

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/eva-bharat/media-sequencer/backend/internal/models"
)

// Helper to construct test playlists quickly
func makeTestPlaylist(windowNumber int, durations []int, types []models.MediaType) *models.Playlist {
	items := make([]models.PlaylistItem, len(durations))
	for i, d := range durations {
		mType := models.MediaTypeVideo
		if i < len(types) && types[i] != "" {
			mType = types[i]
		}
		items[i] = models.PlaylistItem{
			ItemID:          fmt.Sprintf("item-%d-%d", windowNumber, i+1),
			MediaKey:        fmt.Sprintf("M%d", i+1),
			Type:            mType,
			URL:             fmt.Sprintf("https://example.com/m%d.mp4", i+1),
			DurationSeconds: d,
			Order:           i + 1,
		}
	}
	p := &models.Playlist{
		WindowNumber: windowNumber,
		Items:        items,
		Version:      1,
	}
	p.Recalculate()
	return p
}

func makeWindow(windowNumber int, origin time.Time) *models.Window {
	return &models.Window{
		WindowNumber:         windowNumber,
		Name:                 fmt.Sprintf("Window %d", windowNumber),
		CycleDurationSeconds: 18000,
		CycleStartTime:       origin,
		IsActive:             true,
	}
}

// ============================================================================
// TEST GROUP A — NORMAL PLAYBACK
// ============================================================================
func TestGroupA_NormalPlayback(t *testing.T) {
	engine := NewEngine()
	origin := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
	window := makeWindow(1, origin)

	t.Run("one media item", func(t *testing.T) {
		p := makeTestPlaylist(1, []int{30}, nil)

		// At t=0s -> M1, pos=0s, remaining=30s
		st, err := engine.Calculate(window, p, origin)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if st.MediaKey != "M1" || st.PlaybackPosition != 0 || st.TimeRemaining != 30*time.Second {
			t.Errorf("unexpected state at t=0: %+v", st)
		}

		// At t=15s -> M1, pos=15s, remaining=15s
		st, err = engine.Calculate(window, p, origin.Add(15*time.Second))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if st.MediaKey != "M1" || st.PlaybackPosition != 15*time.Second || st.TimeRemaining != 15*time.Second {
			t.Errorf("unexpected state at t=15s: %+v", st)
		}
	})

	t.Run("two media items in correct order", func(t *testing.T) {
		// M1 (30s) -> M2 (60s)
		p := makeTestPlaylist(1, []int{30, 60}, nil)

		// t=10s: should be inside M1 at 10s
		st, err := engine.Calculate(window, p, origin.Add(10*time.Second))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if st.MediaKey != "M1" || st.PlaybackPosition != 10*time.Second {
			t.Errorf("expected M1 at 10s, got %s at %v", st.MediaKey, st.PlaybackPosition)
		}

		// t=30s: exactly at boundary, should be inside M2 at 0s ([start, end) convention)
		st, err = engine.Calculate(window, p, origin.Add(30*time.Second))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if st.MediaKey != "M2" || st.PlaybackPosition != 0 {
			t.Errorf("expected M2 at 0s at boundary 30s, got %s at %v", st.MediaKey, st.PlaybackPosition)
		}

		// t=45s: should be inside M2 at 15s (45 - 30 = 15)
		st, err = engine.Calculate(window, p, origin.Add(45*time.Second))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if st.MediaKey != "M2" || st.PlaybackPosition != 15*time.Second {
			t.Errorf("expected M2 at 15s, got %s at %v", st.MediaKey, st.PlaybackPosition)
		}
	})

	t.Run("multiple media items with accurate offsets", func(t *testing.T) {
		// M1 (10s) -> M2 (20s) -> M3 (30s). Total = 60s
		p := makeTestPlaylist(1, []int{10, 20, 30}, nil)

		// t=5s -> M1, pos=5s
		st, _ := engine.Calculate(window, p, origin.Add(5*time.Second))
		if st.MediaKey != "M1" || st.PlaybackPosition != 5*time.Second {
			t.Errorf("expected M1 at 5s, got %s at %v", st.MediaKey, st.PlaybackPosition)
		}

		// t=25s -> M2 (10..30s), pos = 25 - 10 = 15s
		st, _ = engine.Calculate(window, p, origin.Add(25*time.Second))
		if st.MediaKey != "M2" || st.PlaybackPosition != 15*time.Second {
			t.Errorf("expected M2 at 15s, got %s at %v", st.MediaKey, st.PlaybackPosition)
		}

		// t=50s -> M3 (30..60s), pos = 50 - 30 = 20s
		st, _ = engine.Calculate(window, p, origin.Add(50*time.Second))
		if st.MediaKey != "M3" || st.PlaybackPosition != 20*time.Second {
			t.Errorf("expected M3 at 20s, got %s at %v", st.MediaKey, st.PlaybackPosition)
		}
	})
}

// ============================================================================
// TEST GROUP B — REPETITION
// ============================================================================
func TestGroupB_Repetition(t *testing.T) {
	engine := NewEngine()
	origin := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
	window := makeWindow(1, origin)

	// Playlist: M1 (30s) -> M2 (60s) -> M3 (120s). Total = 210s
	p := makeTestPlaylist(1, []int{30, 60, 120}, nil)

	t.Run("multiple repetitions within cycle", func(t *testing.T) {
		// Iteration 0: t=0s -> M1
		st0, _ := engine.Calculate(window, p, origin)
		if st0.MediaKey != "M1" || st0.SequenceIteration != 0 {
			t.Errorf("iteration 0 failed: %+v", st0)
		}

		// Iteration 1 starts at t=210s -> M1
		st1, _ := engine.Calculate(window, p, origin.Add(210*time.Second))
		if st1.MediaKey != "M1" || st1.SequenceIteration != 1 || st1.PlaybackPosition != 0 {
			t.Errorf("iteration 1 start failed: %+v", st1)
		}

		// Iteration 2: t=420s + 40s = 460s.
		// Within sequence (210s): 460 % 210 = 40s.
		// M1 is 0..30s, M2 is 30..90s. At 40s, it must be M2 at 10s!
		st2, _ := engine.Calculate(window, p, origin.Add(460*time.Second))
		if st2.MediaKey != "M2" || st2.PlaybackPosition != 10*time.Second || st2.SequenceIteration != 2 {
			t.Errorf("iteration 2 mid-play failed: expected M2 at 10s (iter 2), got %s at %v (iter %d)",
				st2.MediaKey, st2.PlaybackPosition, st2.SequenceIteration)
		}
	})

	t.Run("sequence boundary transition", func(t *testing.T) {
		// Just before sequence loop (t = 209s 999ms): still M3
		stBefore, _ := engine.Calculate(window, p, origin.Add(209*time.Second+999*time.Millisecond))
		if stBefore.MediaKey != "M3" {
			t.Errorf("expected M3 right before boundary, got %s", stBefore.MediaKey)
		}

		// Exactly at sequence loop (t = 210s 000ms): snaps to M1 at 0s
		stBoundary, _ := engine.Calculate(window, p, origin.Add(210*time.Second))
		if stBoundary.MediaKey != "M1" || stBoundary.PlaybackPosition != 0 {
			t.Errorf("expected M1 at exactly 210s, got %s at %v", stBoundary.MediaKey, stBoundary.PlaybackPosition)
		}
	})
}

// ============================================================================
// TEST GROUP C — 5-HOUR CYCLE
// ============================================================================
func TestGroupC_FiveHourCycle(t *testing.T) {
	engine := NewEngine()
	origin := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
	window := makeWindow(1, origin)

	// Playlist: M1 (10s) -> M2 (20s). Sequence = 30s.
	// 18,000s / 30s = exactly 600 full iterations per 5-hour cycle!
	p := makeTestPlaylist(1, []int{10, 20}, nil)

	t.Run("one millisecond before 5 hours", func(t *testing.T) {
		// t = 17,999s + 999ms
		// In cycle 0, iteration 599.
		// Within sequence (30s): 17999s 999ms % 30s = 29s 999ms.
		// M1 is 0..10s, M2 is 10..30s. At 29.999s, it is M2 at pos 19s 999ms.
		tBefore := origin.Add(FiveHourCycle - time.Millisecond)
		st, err := engine.Calculate(window, p, tBefore)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if st.CycleNumber != 0 {
			t.Errorf("expected cycle 0, got %d", st.CycleNumber)
		}
		if st.MediaKey != "M2" {
			t.Errorf("expected M2 at end of cycle, got %s", st.MediaKey)
		}
		if st.PlaybackPosition != 19*time.Second+999*time.Millisecond {
			t.Errorf("expected position 19.999s, got %v", st.PlaybackPosition)
		}
	})

	t.Run("exactly at 5 hours", func(t *testing.T) {
		// t = 18,000s -> Cycle 1, iteration 0, M1 at 0s
		tExact := origin.Add(FiveHourCycle)
		st, err := engine.Calculate(window, p, tExact)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if st.CycleNumber != 1 {
			t.Errorf("expected cycle 1, got %d", st.CycleNumber)
		}
		if st.CyclePosition != 0 {
			t.Errorf("expected cycle position 0, got %v", st.CyclePosition)
		}
		if st.MediaKey != "M1" || st.PlaybackPosition != 0 {
			t.Errorf("expected M1 at pos 0 at 5h mark, got %s at %v", st.MediaKey, st.PlaybackPosition)
		}
	})

	t.Run("just after 5 hours", func(t *testing.T) {
		// t = 18,000s + 1ms -> Cycle 1, iteration 0, M1 at 1ms
		tAfter := origin.Add(FiveHourCycle + time.Millisecond)
		st, err := engine.Calculate(window, p, tAfter)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if st.CycleNumber != 1 || st.PlaybackPosition != time.Millisecond {
			t.Errorf("expected cycle 1 at 1ms, got cycle %d at %v", st.CycleNumber, st.PlaybackPosition)
		}
	})

	t.Run("multiple 5-hour cycles (e.g. 3 cycles = 15 hours)", func(t *testing.T) {
		// t = 3 * 18,000s + 15s = 54,015s
		// Cycle 3, cycle position 15s.
		// Within 30s sequence: 15s -> M2 at pos 5s.
		t3Cycles := origin.Add(3*FiveHourCycle + 15*time.Second)
		st, err := engine.Calculate(window, p, t3Cycles)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if st.CycleNumber != 3 {
			t.Errorf("expected cycle number 3, got %d", st.CycleNumber)
		}
		if st.CyclePosition != 15*time.Second {
			t.Errorf("expected cycle position 15s, got %v", st.CyclePosition)
		}
		if st.MediaKey != "M2" || st.PlaybackPosition != 5*time.Second {
			t.Errorf("expected M2 at 5s, got %s at %v", st.MediaKey, st.PlaybackPosition)
		}
	})
}

// ============================================================================
// TEST GROUP D — BLANK BEHAVIOR (CRITICAL SPECIFICATION REQUIREMENT)
// ============================================================================
func TestGroupD_BlankBehavior(t *testing.T) {
	engine := NewEngine()
	origin := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
	window := makeWindow(1, origin)

	t.Run("PROVE unused cycle time does NOT become blank", func(t *testing.T) {
		// A playlist with M1 (30s) -> M2 (60s) -> M3 (120s). Total = 210s.
		// 5 hours = 18,000s.
		// At t = 10,000s (far beyond the first 210s iteration):
		// IT MUST NOT BE BLANK! It must be repeating M1/M2/M3.
		p := makeTestPlaylist(1, []int{30, 60, 120}, nil)

		st, err := engine.Calculate(window, p, origin.Add(10000*time.Second))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if st.Status == PlaybackStatusBlank {
			t.Fatalf("VIOLATION: Unused cycle time was converted into blank playback!")
		}
		if st.MediaKey != "M1" && st.MediaKey != "M2" && st.MediaKey != "M3" {
			t.Errorf("expected valid configured media, got %s", st.MediaKey)
		}
	})

	t.Run("explicitly configured blank in playlist", func(t *testing.T) {
		// M1 (20s) -> BLANK (10s) -> M2 (30s). Total = 60s.
		durations := []int{20, 10, 30}
		types := []models.MediaType{models.MediaTypeVideo, models.MediaTypeBlank, models.MediaTypeVideo}
		p := makeTestPlaylist(1, durations, types)
		p.Items[1].MediaKey = "BLANK_ITEM"

		// t = 10s: M1 (video)
		st1, _ := engine.Calculate(window, p, origin.Add(10*time.Second))
		if st1.Status != PlaybackStatusNormal || st1.MediaKey != "M1" {
			t.Errorf("expected normal playback of M1, got status=%s key=%s", st1.Status, st1.MediaKey)
		}

		// t = 25s: BLANK_ITEM (20..30s, offset 5s)
		st2, _ := engine.Calculate(window, p, origin.Add(25*time.Second))
		if st2.Status != PlaybackStatusBlank || st2.MediaKey != "BLANK_ITEM" || st2.PlaybackPosition != 5*time.Second {
			t.Errorf("expected PlaybackStatusBlank for BLANK_ITEM at 5s, got status=%s key=%s pos=%v",
				st2.Status, st2.MediaKey, st2.PlaybackPosition)
		}

		// t = 35s: M3 (video, 30..60s, offset 5s)
		st3, _ := engine.Calculate(window, p, origin.Add(35*time.Second))
		if st3.Status != PlaybackStatusNormal || st3.MediaKey != "M3" || st3.PlaybackPosition != 5*time.Second {
			t.Errorf("expected normal playback of M3, got status=%s key=%s pos=%v", st3.Status, st3.MediaKey, st3.PlaybackPosition)
		}
	})

	t.Run("blank at beginning and blank at end", func(t *testing.T) {
		// BLANK_START (10s) -> M1 (20s) -> BLANK_END (10s). Total = 40s.
		durations := []int{10, 20, 10}
		types := []models.MediaType{models.MediaTypeBlank, models.MediaTypeVideo, models.MediaTypeBlank}
		p := makeTestPlaylist(1, durations, types)

		// t = 5s: inside BLANK_START
		stStart, _ := engine.Calculate(window, p, origin.Add(5*time.Second))
		if stStart.Status != PlaybackStatusBlank {
			t.Errorf("expected blank status at start")
		}

		// t = 35s: inside BLANK_END (30..40s)
		stEnd, _ := engine.Calculate(window, p, origin.Add(35*time.Second))
		if stEnd.Status != PlaybackStatusBlank {
			t.Errorf("expected blank status at end")
		}
	})
}

// ============================================================================
// TEST GROUP E — INVALID DATA & BOUNDARIES
// ============================================================================
func TestGroupE_InvalidData(t *testing.T) {
	engine := NewEngine()
	origin := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
	window := makeWindow(1, origin)

	t.Run("empty playlist returns fallback state", func(t *testing.T) {
		emptyPlaylist := &models.Playlist{WindowNumber: 1, Items: []models.PlaylistItem{}}
		st, err := engine.Calculate(window, emptyPlaylist, origin)
		if err != nil {
			t.Fatalf("unexpected error for empty playlist: %v", err)
		}
		if st.Status != PlaybackStatusFallback {
			t.Errorf("expected fallback status, got %s", st.Status)
		}
		if st.MediaKey != "FALLBACK" {
			t.Errorf("expected FALLBACK media key, got %s", st.MediaKey)
		}
	})

	t.Run("nil playlist returns error", func(t *testing.T) {
		_, err := engine.Calculate(window, nil, origin)
		if err != ErrNilPlaylist {
			t.Errorf("expected ErrNilPlaylist, got %v", err)
		}
	})

	t.Run("zero duration item returns error", func(t *testing.T) {
		p := makeTestPlaylist(1, []int{30, 0, 20}, nil)
		_, err := engine.Calculate(window, p, origin)
		if err == nil {
			t.Errorf("expected error for zero duration item")
		}
	})

	t.Run("negative duration item returns error", func(t *testing.T) {
		p := makeTestPlaylist(1, []int{30, -5, 20}, nil)
		_, err := engine.Calculate(window, p, origin)
		if err == nil {
			t.Errorf("expected error for negative duration item")
		}
	})

	t.Run("huge duration (longer than 5 hours)", func(t *testing.T) {
		// Single item of 7 hours = 25,200s
		hugeDuration := 25200
		p := makeTestPlaylist(1, []int{hugeDuration}, nil)

		// At t = 2 hours: inside M1 at 2 hours
		st, err := engine.Calculate(window, p, origin.Add(2*time.Hour))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if st.MediaKey != "M1" || st.PlaybackPosition != 2*time.Hour {
			t.Errorf("expected M1 at 2h, got %s at %v", st.MediaKey, st.PlaybackPosition)
		}

		// At t = 5 hours: 5-hour cycle restarts, so position within cycle is 0,
		// and position in M1 restarts at 0
		stCycle, err := engine.Calculate(window, p, origin.Add(FiveHourCycle))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if stCycle.CycleNumber != 1 || stCycle.PlaybackPosition != 0 {
			t.Errorf("expected cycle 1 at 0s for huge item, got cycle %d at %v",
				stCycle.CycleNumber, stCycle.PlaybackPosition)
		}
	})
}

// ============================================================================
// TEST GROUP F — MULTIPLE WINDOW INDEPENDENCE
// ============================================================================
func TestGroupF_MultipleWindows(t *testing.T) {
	engine := NewEngine()
	origin := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)

	// Window 1: M1 (15s) -> M2 (10s) -> M3 (20s). Total = 45s
	w1 := makeWindow(1, origin)
	p1 := makeTestPlaylist(1, []int{15, 10, 20}, nil)
	p1.Items[0].MediaKey = "W1_M1"
	p1.Items[1].MediaKey = "W1_M2"
	p1.Items[2].MediaKey = "W1_M3"

	// Window 2: M4 (12s) -> M5 (8s). Total = 20s
	w2 := makeWindow(2, origin)
	p2 := makeTestPlaylist(2, []int{12, 8}, nil)
	p2.Items[0].MediaKey = "W2_M4"
	p2.Items[1].MediaKey = "W2_M5"

	// Window 3: M6 (30s) -> M7 (30s). Total = 60s
	w3 := makeWindow(3, origin)
	p3 := makeTestPlaylist(3, []int{30, 30}, nil)
	p3.Items[0].MediaKey = "W3_M6"
	p3.Items[1].MediaKey = "W3_M7"

	// Query at t = 25s:
	// Window 1: 25s % 45s = 25s -> W1_M1(15s), W1_M2(10s) -> W1_M3 at offset 0s
	// Window 2: 25s % 20s = 5s -> W2_M4 at offset 5s
	// Window 3: 25s % 60s = 25s -> W3_M6 at offset 25s
	queryTime := origin.Add(25 * time.Second)

	st1, err1 := engine.Calculate(w1, p1, queryTime)
	st2, err2 := engine.Calculate(w2, p2, queryTime)
	st3, err3 := engine.Calculate(w3, p3, queryTime)

	if err1 != nil || err2 != nil || err3 != nil {
		t.Fatalf("unexpected calculation errors: %v, %v, %v", err1, err2, err3)
	}

	if st1.MediaKey != "W1_M3" || st1.PlaybackPosition != 0 {
		t.Errorf("Window 1 incorrect: expected W1_M3 at 0s, got %s at %v", st1.MediaKey, st1.PlaybackPosition)
	}
	if st2.MediaKey != "W2_M4" || st2.PlaybackPosition != 5*time.Second {
		t.Errorf("Window 2 incorrect: expected W2_M4 at 5s, got %s at %v", st2.MediaKey, st2.PlaybackPosition)
	}
	if st3.MediaKey != "W3_M6" || st3.PlaybackPosition != 25*time.Second {
		t.Errorf("Window 3 incorrect: expected W3_M6 at 25s, got %s at %v", st3.MediaKey, st3.PlaybackPosition)
	}

	// Verify windows did not mutate each other
	if p1.Items[0].MediaKey != "W1_M1" || p2.Items[0].MediaKey != "W2_M4" || p3.Items[0].MediaKey != "W3_M6" {
		t.Errorf("cross-window corruption detected in playlist items")
	}
}

// ============================================================================
// TEST GROUP G — DYNAMIC PLAYLIST UPDATES & TIMELINE EVOLUTION
// ============================================================================
func TestGroupG_DynamicUpdates(t *testing.T) {
	engine := NewEngine()
	origin := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
	window := makeWindow(1, origin)

	// Initial Playlist V1: M1 (30s) -> M2 (30s). Total = 60s.
	pV1 := makeTestPlaylist(1, []int{30, 30}, nil)
	pV1.Version = 1

	// Query at t = 20s (inside M1, pos = 20s)
	tMid := origin.Add(20 * time.Second)
	stV1, _ := engine.Calculate(window, pV1, tMid)
	if stV1.MediaKey != "M1" || stV1.PlaybackPosition != 20*time.Second {
		t.Fatalf("expected M1 at 20s under V1, got %s at %v", stV1.MediaKey, stV1.PlaybackPosition)
	}

	// Dynamic update: Add M3 (40s) -> V2: M1 (30s) -> M2 (30s) -> M3 (40s). Total = 100s.
	pV2 := makeTestPlaylist(1, []int{30, 30, 40}, nil)
	pV2.Version = 2

	// At t = 20s under V2, M1 is still playing at 20s (invariant: adding an item to the tail
	// does not disrupt the currently active item's playback)
	stV2, _ := engine.Calculate(window, pV2, tMid)
	if stV2.MediaKey != "M1" || stV2.PlaybackPosition != 20*time.Second {
		t.Errorf("active item was corrupted by playlist append: got %s at %v", stV2.MediaKey, stV2.PlaybackPosition)
	}

	// But in the new iteration at t = 80s:
	// Under V1: 80 % 60 = 20s -> M1 at 20s.
	// Under V2: 80 % 100 = 80s -> M1(30s) + M2(30s) -> M3 at 20s!
	tLater := origin.Add(80 * time.Second)
	stV2Later, _ := engine.Calculate(window, pV2, tLater)
	if stV2Later.MediaKey != "M3" || stV2Later.PlaybackPosition != 20*time.Second {
		t.Errorf("expected M3 at 20s under updated V2 playlist, got %s at %v", stV2Later.MediaKey, stV2Later.PlaybackPosition)
	}
}

// ============================================================================
// TEST GROUP H — HIGH PRECISION & SUB-SECOND MEDIA
// ============================================================================
func TestGroupH_SubSecondAndHighPrecision(t *testing.T) {
	engine := NewEngine()
	origin := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
	window := makeWindow(1, origin)

	// Short media items with millisecond precision:
	// Item 1: 50ms
	// Item 2: 100ms
	// Total = 150ms
	p := &models.Playlist{
		WindowNumber: 1,
		Items: []models.PlaylistItem{
			{
				ItemID:     "sub-1",
				MediaKey:   "MS_50",
				Type:       models.MediaTypeImage,
				DurationMs: 50,
				Order:      1,
			},
			{
				ItemID:     "sub-2",
				MediaKey:   "MS_100",
				Type:       models.MediaTypeImage,
				DurationMs: 100,
				Order:      2,
			},
		},
		Version: 1,
	}

	// t = 20ms: inside MS_50 at 20ms
	st1, err := engine.Calculate(window, p, origin.Add(20*time.Millisecond))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if st1.MediaKey != "MS_50" || st1.PlaybackPosition != 20*time.Millisecond {
		t.Errorf("expected MS_50 at 20ms, got %s at %v", st1.MediaKey, st1.PlaybackPosition)
	}

	// t = 50ms (exact boundary): snaps to MS_100 at 0ms ([start, end) convention)
	st2, err := engine.Calculate(window, p, origin.Add(50*time.Millisecond))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if st2.MediaKey != "MS_100" || st2.PlaybackPosition != 0 {
		t.Errorf("expected MS_100 at 0ms at exact 50ms boundary, got %s at %v", st2.MediaKey, st2.PlaybackPosition)
	}

	// t = 150ms (exact sequence loop): snaps back to MS_50 at 0ms
	st3, err := engine.Calculate(window, p, origin.Add(150*time.Millisecond))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if st3.MediaKey != "MS_50" || st3.PlaybackPosition != 0 {
		t.Errorf("expected MS_50 at 0ms at sequence loop, got %s at %v", st3.MediaKey, st3.PlaybackPosition)
	}
}

// ============================================================================
// INVARIANT / PROPERTY TESTING
// ============================================================================
func TestInvariants_PropertyTesting(t *testing.T) {
	engine := NewEngine()
	origin := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
	window := makeWindow(1, origin)
	p := makeTestPlaylist(1, []int{10, 25, 45}, nil)

	r := rand.New(rand.NewSource(42))

	// Invariant 1..8 check across 1000 random query timestamps
	for i := 0; i < 1000; i++ {
		// Random duration between -10 hours and +50 hours
		randSecs := r.Int63n(60*3600) - (10 * 3600)
		randMs := r.Int63n(1000)
		qTime := origin.Add(time.Duration(randSecs)*time.Second + time.Duration(randMs)*time.Millisecond)

		st, err := engine.Calculate(window, p, qTime)
		if err != nil {
			t.Fatalf("iteration %d: calculation failed: %v", i, err)
		}

		// Invariant 1: Always returns a valid configured media item
		if st.MediaKey != "M1" && st.MediaKey != "M2" && st.MediaKey != "M3" {
			t.Fatalf("Invariant 1 violated: unexpected media %s", st.MediaKey)
		}

		// Invariant 2: Playback position is never negative
		if st.PlaybackPosition < 0 {
			t.Fatalf("Invariant 2 violated: negative playback position %v", st.PlaybackPosition)
		}

		// Invariant 3: Playback position is strictly less than item duration
		if st.PlaybackPosition >= st.ItemDuration {
			t.Fatalf("Invariant 3 violated: playback position %v >= duration %v", st.PlaybackPosition, st.ItemDuration)
		}

		// Invariant 4: No automatically generated blank item appears
		if st.Status == PlaybackStatusBlank {
			t.Fatalf("Invariant 4 violated: unexpected blank item appeared")
		}

		// Invariant 5: Same playlist + same timestamp produces deterministic identical result
		stRepeat, _ := engine.Calculate(window, p, qTime)
		if st.MediaKey != stRepeat.MediaKey || st.PlaybackPosition != stRepeat.PlaybackPosition {
			t.Fatalf("Invariant 5 violated: non-deterministic output")
		}

		// Invariant 7: Cycle position remains within [0, 5 hours)
		if st.CyclePosition < 0 || st.CyclePosition >= FiveHourCycle {
			t.Fatalf("Invariant 7 violated: cycle position %v outside [0, 5h)", st.CyclePosition)
		}
	}
}
