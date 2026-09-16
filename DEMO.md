# EVA Bharat: Multi-Window Media Sequencer — Evaluator Demo Flow

A concise, step-by-step 3–5 minute demonstration script designed for the EVA Bharat technical evaluation team to verify all core functional requirements in real time.

---

## Prerequisites
* Backend running on `http://localhost:8080` (or containerized on port `8080`).
* Frontend running on `http://localhost:5173` (or containerized on port `5173`).
* MongoDB running on `mongodb://localhost:27017` with seed data loaded.

---

## 3–5 Minute Evaluation Script

### Step 1: Open 2x2 Multi-Window Matrix
1. Open your browser and navigate to: `http://localhost:5173/`.
2. **Observe**:
   - The 2x2 multi-window grid displays 4 distinct outputs:
     - **Window 1**: Front Display
     - **Window 2**: Side Display
     - **Window 3**: Lobby Display
     - **Window 4**: Balcony Display
   - Each window displays a green **CONNECTED** badge indicating live Gorilla WebSocket channel connectivity.

### Step 2: Verify Independent Continuous Playback
1. Observe the viewports for 30–45 seconds.
2. **Observe**:
   - Each window displays its own independent playlist sequence (e.g. Window 1 plays `M1` $\to$ `M2` $\to$ `M3`, Window 2 plays `M4` $\to$ `M5`).
   - When a video or image finishes, the next item transitions seamlessly with **zero artificial gap or freeze**.
   - No window inherits or affects the playback position of another window.

### Step 3: Trigger Real-Time Synchronization Override
1. Scroll to the **⚡ Real-Time Synchronization Console** at the bottom of the screen.
2. Select **M2 — Product Showcase Video (15s)**.
3. Keep default duration at `20` seconds.
4. Click **Trigger Synchronization**.
5. **Observe**:
   - A synchronized broadcast is emitted from the Go backend with a 1000ms lead time.
   - All 4 display cards immediately display the yellow **⚡ SYNC OVERRIDE ACTIVE** banner with a live synchronized countdown.
   - Every window switches to play `M2` simultaneously.

### Step 4: Verify Automatic Expiration & Wall-Clock Resumption
1. Allow the sync countdown to reach `0s` (or click **Cancel Active Sync**).
2. **Observe**:
   - The yellow sync override badge vanishes.
   - Every window automatically reverts to its own normal playlist sequence.
   - Resumption is calculated against the 5-hour wall-clock timeline engine ($T_{\text{cycle}} = 18{,}000\,\text{s}$)—windows resume at the exact second where their schedule should be, preventing drift.
   - **Stored playlists in MongoDB were not mutated or truncated**.

### Step 5: Test Mid-Sync Joining (Late-Connecting Display)
1. In the console, trigger sync for `M2` with `30` seconds duration.
2. Open a new browser tab and navigate directly to: `http://localhost:5173/#/display/3`.
3. **Observe**:
   - The newly connected tab immediately discovers the ongoing sync event during initial WebSocket handshake.
   - It calculates the elapsed sync offset ($\Delta t$) and seeks into `M2` in unison with the other windows, rather than starting from zero.

### Step 6: Test Dynamic Playlist Update
1. Return to the main dashboard (`http://localhost:5173/`).
2. In the **📋 Dynamic Playlist Manager** panel, select **Window 1 (Front Display)**.
3. Select Media Asset **M4 — Tech Animation Video (20s)** and click **Add to Playlist**.
4. **Observe**:
   - An HTTP POST request is sent to `/api/v1/windows/1/playlist`.
   - MongoDB updates the document and increments the version counter.
   - The Go backend broadcasts `PLAYLIST_UPDATED` over WebSocket to Window 1.
   - Window 1 reflects the new playlist item immediately without a full page reload.

### Step 7: Verify Standalone Signage View
1. Navigate to `http://localhost:5173/#/display/2`.
2. **Observe**:
   - The UI switches to dedicated standalone signage mode, displaying only Window 2's output without console overhead.

### Step 8: Verify Database Persistence
1. Stop the Go backend process (`Ctrl+C`).
2. Observe the frontend display status pills transition to yellow **DISCONNECTED — RECONNECTING...**.
3. Restart the Go backend (`go run ./cmd/server`).
4. **Observe**:
   - Frontend automatically reconnects with exponential backoff.
   - All 4 windows reconnect, retrieve persistent state from MongoDB, and resume gapless playback.
   - The newly added `M4` item in Window 1 remains intact.

---

## Summary of Evaluated Criteria
* [x] Continuous looping playback per window
* [x] Authoritative 5-hour cycle behavior without arbitrary blanks
* [x] Dynamic runtime playlist additions via REST and WebSocket
* [x] Instant, reliable synchronization override across all windows
* [x] Resilient recovery across client refreshes, mid-sync joining, and backend restarts
* [x] Persistent MongoDB storage
