# 5-Hour Cycle Timeline Engine — Specification & Architecture

## 1. Executive Summary

This document explains the deterministic mathematical algorithm powering the **Multi-Window Media Sequencer** timeline engine (`internal/timeline`).

The engine computes the exact playback state of any window at any query timestamp without running loops, background sleep timers, or relying on browser APIs. It is a pure function:

$$\text{Calculate}(W, P, T) \longrightarrow \text{PlaybackState}$$

---

## 2. Core Scenario & 5-Hour Cycle Interpretation

The assignment specification states:
> *"Each window has its own media list. The total play size for each window must be treated as 5 hours. Each window should keep playing its configured list again and again within that cycle. Blank is only a configured playlist item when included; the rest of the cycle should not become blank playback by default."*

### Constant Definition
The 5-hour cycle is defined as a strongly typed constant:
```go
const FiveHourCycle = 18000 * time.Second // 18,000 seconds = 5 * 3,600s
```

### Clarification on "No Default Blank"
In many naive digital signage systems, if a sequence total duration is less than the scheduled time block (e.g., 210 seconds out of 5 hours), the player stops at 210s and shows a black screen for the remaining 4 hours and 56 minutes.
**The assignment strictly forbids this.**
Instead, the configured media list loops **continuously and repetitively** until the entire 5-hour boundary is traversed.

---

## 3. Walkthrough Example

Consider a display window with the following configured playlist:
* **$M_1$**: 30 seconds
* **$M_2$**: 60 seconds
* **$M_3$**: 120 seconds
* **Total Sequence Duration ($D_{\text{seq}}$)**: $30 + 60 + 120 = 210\text{ seconds}$

### Timeline Progression

```
Time (seconds):
0               30              90              210             240             300             420...
|--- M1 (30s) ---|--- M2 (60s) ---|--- M3 (120s) ---|--- M1 (30s) ---|--- M2 (60s) ---|--- M3 (120s) ---|...
[   Iteration 0 (0 to 210s)                       ][   Iteration 1 (210 to 420s)                     ]
```

Within the 5-hour cycle ($18{,}000\text{s}$):
$$\frac{18{,}000\text{s}}{210\text{s}} = 85.714\text{ iterations}$$
* The window executes **85 complete iterations** of $M_1 \to M_2 \to M_3$ ($85 \times 210\text{s} = 17{,}850\text{s}$).
* For the remaining $150\text{s}$ of the 5-hour cycle ($17{,}850\text{s}$ to $18{,}000\text{s}$):
  * $17{,}850\text{s}$ to $17{,}880\text{s}$ ($30\text{s}$): Plays $M_1$
  * $17{,}880\text{s}$ to $17{,}940\text{s}$ ($60\text{s}$): Plays $M_2$
  * $17{,}940\text{s}$ to $18{,}000\text{s}$ ($60\text{s}$): Plays the first $60\text{s}$ of $M_3$
* At exactly $18{,}000\text{s}$, Cycle 0 ends and **Cycle 1 begins immediately**, resetting to $M_1$ at offset $0\text{s}$.
* **Result:** Playback never stops, and no unwanted blank screen ever appears.

---

## 4. The Algorithm Step-by-Step

### Step 1: Calculate Elapsed Time in Cycle
Let $T_0$ be the window's `CycleStartTime` in UTC. For any query time $T$:
$$\Delta T = T - T_0$$
```go
cyclePosition := diff % FiveHourCycle
cycleNumber := int64(diff / FiveHourCycle)
```

### Step 2: Calculate Sequence Position
Because the sequence repeats continuously throughout the cycle:
```go
seqOffset := cyclePosition % totalSequenceDuration
sequenceIteration := int64(cyclePosition / totalSequenceDuration)
```

### Step 3: Half-Open Interval Matching $[start, end)$
Each item $k$ occupies a continuous half-open interval $[S_{k-1}, S_k)$ where $S_k = \sum_{i=1}^k d_i$.
* An item is active if:
  $$S_{k-1} \le \text{seqOffset} < S_k$$
* The item's internal playback position is:
  $$\text{PlaybackPosition} = \text{seqOffset} - S_{k-1}$$
* Time remaining until transition:
  $$\text{TimeRemaining} = d_k - \text{PlaybackPosition}$$

---

## 5. Key Edge Cases & Invariants

1. **Exact Boundary Semantics $[start, end)$:**
   * At $t = 30.000\text{s}$, $M_1$ (duration 30s) ends and $M_2$ begins at offset $0.000\text{s}$.
   * There are no ambiguous timestamps where two items compete for playback.
2. **Explicit Configured Blank:**
   * If a playlist item has `type = "blank"` and `duration = 10s`, it plays for exactly 10s, setting `Status: "BLANK"`. It loops just like any video or image item.
3. **Empty Playlist:**
   * Returns `Status: "FALLBACK"` with `MediaKey: "FALLBACK"` and `PlaybackPosition: 0`.
4. **Invalid Durations:**
   * Zero or negative durations return an explicit validation error (`ErrInvalidItemDuration`), preventing division-by-zero or infinite loops.
5. **Multiple Window Isolation:**
   * Window 1, Window 2, and Window 3 have independent playlists, independent durations, and independent calculations. No global mutable state exists.
6. **Sub-Second Precision:**
   * Supports `DurationMs` for millisecond-level precision using Go's standard `time.Duration`.
