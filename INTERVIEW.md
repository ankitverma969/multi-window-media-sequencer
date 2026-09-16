# EVA Bharat Assignment — Comprehensive Interview Preparation Guide

This guide is specifically tailored for technical interviews regarding the **Multi-Window Media Sequencer with Synchronized Playback** assignment for the EVA Bharat Backend Development Intern evaluation.

Every answer, formula, and architecture description in this document is strictly based on the real implementation in this repository.

---

## 1. Explain the Project in 30 Seconds
> "I built a digital signage multi-window media sequencer in Golang, MongoDB, and React. It manages four independent display outputs that continuously loop their own video and image playlists over an authoritative 5-hour cycle. When an operator triggers a global sync event, the Go backend coordinates a simultaneous override across all screens via WebSockets using predictive server timestamps. When the sync timer expires, each screen automatically resumes its exact scheduled playlist position without losing stored configurations or drifting."

---

## 2. Explain the Project in 2 Minutes
> "Digital signage installations across venues like airports or retail centers require screens in different zones to play independent, continuous media loops. However, operators frequently need to trigger an emergency alert or product showcase that plays across every screen simultaneously before reverting back to scheduled content.
>
> In my architecture:
> 1. **Persistent Source of Truth**: MongoDB stores window definitions, media assets, playlists, and sync audit logs. State survives server restarts.
> 2. **Authoritative 5-Hour Cycle Engine**: In Golang, each window's timeline is modeled as an 18,000-second cycle. Rather than incrementing in-memory counters, the engine uses modulo arithmetic against an anchor timestamp:
>    $$\text{offset} = (T_{\text{now}} - T_{\text{start}}) \pmod{18{,}000\,\text{s}}$$
>    $$\text{seqOffset} = \text{offset} \pmod{\text{totalSequenceDuration}}$$
>    This ensures zero drift and guarantees that playlists shorter than 5 hours loop continuously without artificial blank padding.
> 3. **Real-Time Synchronization**: Instead of sending a naive 'Play Now' command, the backend calculates a UTC start time with a 1000ms lead buffer ($T_{\text{start}} = \text{now} + 1000\text{ms}$) and broadcasts it over Gorilla WebSockets. Clients preload the asset and start playback in unison.
> 4. **Resilient Resumption**: When sync expires, windows query the authoritative cycle engine to snap back to the exact second where their regular playlist should be, leaving MongoDB playlist records intact.
> 5. **Direct Architecture**: Communication flows directly between React (port 5173), Go (port 8080), and MongoDB (port 27017) without intermediate reverse proxies."

---

## 3. Problem Statement
The assignment required:
* Multiple display windows operating independently.
* Each window possessing its own distinct looping media list.
* Total play size treated as a fixed 5-hour cycle repeating continuously.
* Blank media to appear **only** when explicitly configured as a playlist item.
* Dynamic runtime playlist updates via REST reflected immediately on active screens.
* Temporary synchronization override showing one selected media item across all windows in unison.
* Clean return to individual playlists post-sync without loss of configuration.
* Full persistence in MongoDB across backend restarts.

---

## 4. System Architecture
```
                  OPERATOR BROWSER / CLIENTS
                              │
               Direct Port Exposure (EC2 SG)
                              │
         ┌────────────────────┴────────────────────┐
         │ (HTTP Port 5173)                        │ (HTTP REST / WebSocket Port 8080)
         ▼                                         ▼
┌──────────────────┐                     ┌───────────────────────────┐
│  React Frontend  │                     │    Go Backend Service     │
│  - 2x2 Grid View │◄──── WebSocket ────►│  - 5-Hour Cycle Engine    │
│  - Standalone /  │      (/ws)          │  - Gorilla WS Hub         │
│  - Time Offset   │                     │  - Authoritative Sync Mgr │
└──────────────────┘                     └─────────────┬─────────────┘
                                                       │
                                        MongoDB Driver │ Port 27017
                                                       ▼
                                         ┌───────────────────────────┐
                                         │      MongoDB Storage      │
                                         │  - Windows, Media         │
                                         │  - Playlists, Sync Events │
                                         └───────────────────────────┘
```
* **No Reverse Proxy**: Direct connection from React to the Go backend on port 8080.
* **CORS Managed in Go**: `internal/middleware/cors.go` explicitly validates request origins.
* **WebSocket Direct Upgrade**: `internal/handlers/ws_handler.go` handles HTTP-to-WS upgrades natively.

---

## 5. Why React?
* **Declarative Component Hierarchy**: Each window is an isolated `<MediaWindow />` maintaining its own player state machine (`IDLE`, `LOADING`, `PLAYING`, `SYNC_OVERRIDE`).
* **Controlled Lifecycle & Cleanups**: Video elements, countdown intervals, and WebSocket listeners are cleanly registered and dismantled via `useEffect` cleanup return functions, avoiding memory leaks.
* **Lightweight Bundle**: Built using Vite, producing optimized ES modules (<250KB total bundle) for rapid rendering on embedded signage hardware.

---

## 6. Why Golang?
* **Concurrency & Goroutines**: Lightweight goroutines allow the WebSocket Hub to handle concurrent client connections and broadcast events without thread overhead.
* **Channel-Based Single-Writer Safety**: Gorilla WebSocket prohibits concurrent writes to the same connection. Go channels (`chan []byte` with capacity 256) inside a dedicated `writePump` serialize writes safely.
* **Strong Typing & Determinism**: High-precision time math (`time.Duration`, `time.Time.UTC()`) guarantees reproducible timeline calculations.
* **Fast Static Binaries**: Compiles to a self-contained Linux binary (`server`) requiring zero external runtime dependencies.

---

## 7. Why MongoDB?
* **Natural Document Modeling**: Signage playlists are hierarchically ordered arrays of items (`[]PlaylistItem`) embedded directly inside the playlist document.
* **Atomic Versioning & Updates**: Playlist modifications atomically increment the `version` field, ensuring optimistic concurrency control.
* **Flexible Schemas for Media Metadata**: Images, videos, and blanks share common attributes while accommodating media-specific attributes.
* **Survives Process Restarts**: All window setups, playlists, and ongoing sync events are persisted on disk.

---

## 8. Why WebSocket?
* **Instant Broadcast Latency**: Polling the server every second for sync events creates high HTTP overhead and introduces a 1–2 second desynchronization window.
* **Bidirectional Communication**: Display windows register upon connection and immediately receive a `STATE_SNAPSHOT` containing the server time and active sync state.
* **Lightweight Heartbeats**: Native ping/pong frames detect dead client connections within seconds.

---

## 9. Why REST API?
* **Separation of Management vs Display**: Management operations (adding media, modifying playlists, triggering sync) follow standard HTTP verbs (`GET`, `POST`, `DELETE`).
* **Standard Status Codes & Validation**: Client errors (malformed JSON, invalid IDs, negative durations) return RFC-7807 structured errors with HTTP 400/404 without crashing WebSocket connections.

---

## 10. How Does the 5-Hour Cycle Work?
* **Cycle Duration Constant**: $T_{\text{cycle}} = 18{,}000\,\text{seconds}$ (`CycleDuration = 18000 * time.Second`).
* **Timeline Anchor**: Each window has an immutable `CycleStartTime` (e.g. UTC midnight).
* **Modulo Math**: The current cycle offset is calculated as:
  $$\text{cycleOffsetSeconds} = (T_{\text{now}} - T_{\text{cycle\_start}}) \pmod{18{,}000}$$
* **Deterministic Repetition**: If a window has 3 items totaling 60 seconds, it calculates:
  $$\text{sequenceOffset} = \text{cycleOffsetSeconds} \pmod{60}$$
  It identifies the exact item and second running right now. When the 60s finishes, it wraps to 0. When the 5-hour cycle finishes, it wraps to cycle 2, position 0.

---

## 11. How Is Playback Position Calculated?
Located in `backend/internal/timeline/engine.go`:
```go
cycleOffset := int(elapsed.Seconds()) % int(cycleDuration.Seconds())
seqOffset := cycleOffset % totalDuration

accumulated := 0
for i, item := range playlist.Items {
    if seqOffset >= accumulated && seqOffset < accumulated + item.DurationSeconds {
        return &PlaybackState{
            CurrentItem:   item,
            CurrentIndex:  i,
            ElapsedInItem: seqOffset - accumulated,
            Remaining:     item.DurationSeconds - (seqOffset - accumulated),
            IsBlank:       item.Type == MediaTypeBlank,
        }
    }
    accumulated += item.DurationSeconds
}
```
* Uses half-open intervals $[ \text{start}, \text{end} )$ so that boundary transitions are deterministic to the millisecond.

---

## 12. How Does Synchronization Work?
1. **Operator Trigger**: Operator calls `POST /api/v1/sync` with `media_key: "M2"`, `duration_seconds: 30`, `lead_time_ms: 1000`.
2. **Server Timestamps**: The Go `SyncCoordinator` records:
   $$\text{start\_time} = \text{Now}() + 1000\text{ms}$$
   $$\text{end\_time} = \text{start\_time} + 30\text{s}$$
3. **Broadcast**: Emits `SYNC_STARTED` envelope to all active WebSocket clients.
4. **Client Action**: Windows see the 1000ms lead buffer, preload `M2`, and display a yellow `⚡ SYNC OVERRIDE ACTIVE` status pill.
5. **Simultaneous Play**: When client estimated server time reaches `start_time`, all windows play `M2`.
6. **Expiration**: A Go `time.Timer` fires at `end_time`, writes inactive status to MongoDB, and broadcasts `SYNC_ENDED`.
7. **Resumption**: Each window queries the 5-hour timeline engine and resumes its own playlist.

---

## 13. Why Use Server Time?
* Client device clocks (laptops, wall tablets, signage displays) frequently drift by several seconds or minutes.
* Relying on client local time would cause each screen to trigger sync at different moments.
* The Go backend includes UTC `server_time` in every HTTP response and WebSocket envelope. The frontend computes clock skew:
  $$\Delta t = T_{\text{server}} - T_{\text{local}}$$
  $$T_{\text{estimatedServer}} = T_{\text{local}} + \Delta t$$
  All scheduling decisions are made against $T_{\text{estimatedServer}}$.

---

## 14. What Happens If a Client Joins During Sync?
* Upon connecting, the display receives a `STATE_SNAPSHOT` payload containing the active sync details.
* The client calculates its mid-sync seek offset:
  $$\text{syncOffsetMs} = \max(0, T_{\text{estimatedServer}} - T_{\text{start}})$$
  $$\text{seekSeconds} = (\text{syncOffsetMs} / 1000) \pmod{\text{mediaDuration}}$$
* The video player seeks directly to `seekSeconds`, playing in lockstep with screens that connected earlier.

---

## 15. How Are Playlists Preserved During Sync?
* **Sync is strictly an ephemeral state layer**: It does not overwrite or mutate the `playlists` collection in MongoDB.
* The Go `SyncCoordinator` holds the active sync pointer in memory and logs it to `sync_events`.
* When sync ends, the window references its unchanged playlist and calculates where it should be on the 5-hour cycle.

---

## 16. What Happens When a Playlist Changes?
1. Operator submits `POST /api/v1/windows/:id/playlist` with new item.
2. Go handler validates the media asset exists.
3. MongoDB updates the playlist document, recalculates total duration, and increments `version`.
4. Go backend sends a `PLAYLIST_UPDATED` envelope targeting that specific window.
5. The React player updates its sequence without interrupting or requiring a full page refresh.

---

## 17. How Does MongoDB Persistence Work?
* Windows, Media Catalog, and Playlists are stored in `media_sequencer` database:
  - `windows` collection: metadata, cycle anchors.
  - `media` collection: asset titles, URLs, types, durations.
  - `playlists` collection: embedded item arrays, versions.
  - `sync_events` collection: active and historical sync records.
* **Upsert Immutability**: All upserts use `$setOnInsert` for `_id` and `$set` for mutable fields, avoiding MongoDB immutable `_id` write errors.

---

## 18. How Does WebSocket Reconnection Work?
* Located in `frontend/src/websocket/useWindowWebSocket.js`.
* If a socket closes abnormally (code $\neq 1000$):
  - Component enters `RECONNECTING` state.
  - Triggers exponential backoff: $\text{delay} = \min(10000, 1000 \times 1.5^{\text{retries}})$.
  - On reconnect, server immediately transmits a fresh `STATE_SNAPSHOT`.

---

## 19. Concurrency in Go
* **Hub Mutex**: `Hub.clients` map is guarded by `sync.RWMutex` to allow concurrent reads and safe serial registrations.
* **SyncCoordinator Mutex**: `sync.RWMutex` guards the active sync pointer.
* **Atomic Timers**: Preempting an existing sync safely stops (`timer.Stop()`) and drains the previous timer channel.

---

## 20. Race Conditions & Prevention
* **Concurrent WebSocket Writes**: Solved via per-client `writePump` goroutines consuming from a buffered channel (`chan []byte`). Handlers never call `conn.WriteMessage` directly.
* **Playlist Update during Active Sync**: The update modifies MongoDB and notifies the client. The client updates its background playlist state, but remains in `SYNC_OVERRIDE` mode until the sync ends, then resumes with the newly updated playlist.

---

## 21. Error Handling Architecture
* **Go Backend**: Errors are wrapped using `%w` and mapped to domain errors (`ErrNotFound`, `ErrDuplicateKey`, `ErrInvalidInput`). Handlers translate these to clean JSON responses with appropriate HTTP status codes (400, 404, 500).
* **React Frontend**: Media players listen to `<video onError>` and `<img onError>`. If an asset fails, an error card is rendered while the timer continues, preventing player freezing.

---

## 22. Docker Architecture
* **Multi-Stage Go Build**: Uses `golang:1.24-alpine` builder, producing an unprivileged static binary running on `alpine:3.21` under `appuser`.
* **Node Production Preview**: `frontend/Dockerfile` uses `node:20-alpine` to build the app and serves it directly via `npm run preview -- --host 0.0.0.0 --port 5173`.
* **Zero Nginx**: No reverse proxy containers; frontend and backend expose ports directly.

---

## 23. AWS Deployment Approach
```
AWS EC2 Instance
├── React Frontend Container (Port 5173:5173)
├── Go Backend Container     (Port 8080:8080)
└── Security Group Rules:
    - Inbound TCP 22   (SSH)
    - Inbound TCP 5173 (Frontend Web App)
    - Inbound TCP 8080 (REST API & WebSocket)
```
* Persistent storage uses **MongoDB Atlas M0** via `MONGODB_URI` environment variable.

---

## 24. Why No Nginx?
* **Simplicity**: For an internship evaluation, adding Nginx introduces an extra failure point, additional config files (`nginx.conf`), and proxy header complexities (`proxy_pass`, `Upgrade`, timeouts).
* **Direct Communication**: Exposing ports 5173 and 8080 directly through the AWS Security Group provides identical functionality with half the complexity.
* **CORS Managed at Application Layer**: Handled directly in Go middleware where it belongs.

---

## 25. Security Architecture
* **Zero Committed Secrets**: Scanned repository; no passwords, API keys, or AWS credentials in code.
* **CORS Whitelisting**: Restricted to configured origins (`CORS_ALLOWED_ORIGINS`).
* **Database Isolation**: Port 27017 bound strictly to `127.0.0.1` in Docker Compose.
* **Non-Root User**: Backend Docker container runs as unprivileged `appuser`.

---

## 26. Testing Strategy
* **Unit Tests**: Timeline boundary math modulo 18,000s; frontend clock skew and seek calculations.
* **Integration Tests**: Repository CRUD, MongoDB restart survival, WebSocket hub client storms.
* **Smoke Tests**: Automated script (`scripts/smoke_test.ps1`) tests all 8 operational paths.
* **Browser E2E**: Autonomous browser subagent visually verified multi-window sync and video playback.

---

## 27. Edge Cases Handled
1. **Empty Playlist**: Displays explicit placeholder without crashing timeline engine.
2. **Playlist Shorter than 5h**: Loops indefinitely with 0 automatic blanks.
3. **Playlist Longer than 5h**: Truncates cleanly at 18,000s boundary and wraps to cycle 2.
4. **Second Sync Request**: Preempts first sync cleanly; cancels old timer and broadcasts new event.
5. **Backend Restart during Sync**: Queries MongoDB on boot, checks `end_time > now`, and re-arms timer for remaining duration.

---

## 28. Browser Autoplay Limitations
* **Problem**: Modern Chromium browsers block programmatic unmuted video autoplay without user interaction.
* **Solution**: Video players default to `muted = true` with `playsInline`. Interactive audio unmute toggles (`🔊 / 🔇`) on each card allow operators to toggle sound per window.

---

## 29. Synchronization Accuracy Limitations
* In browser environments, true millisecond frame sync is limited by:
  1. Client hardware video decoding latency (100–300ms variance).
  2. Operating system scheduling.
  3. Browser rendering frame drops.
* Our system achieves sub-second synchronization using a 1000ms lead buffer and UTC server timestamps.

---

## 30. Tradeoffs
* **Direct Ports vs Reverse Proxy**: Exposing ports 5173 and 8080 directly is simpler and easier to debug, but requires opening two ports in the cloud security group.
* **Wall-Clock vs Paused Resumption**: Resuming at the wall-clock position avoids schedule drift, but means media in progress when sync started will resume mid-item.

---

## 31. What Would You Improve With More Time?
1. **WebRTC for Sub-50ms Video Sync**: Stream unified video canvas rather than individual HTML5 players.
2. **Drag-and-Drop Playlist Reordering**: Add interactive drag-and-drop UI for reordering items.
3. **Automated E2E CI Pipeline**: GitHub Actions running Docker Compose smoke tests on every push.

---

## 32. Likely Interview Questions & Rapid Answers

* **Q: Where is the single source of truth for playback?**
  * *A:* "The Go backend 5-hour cycle engine. React queries it and receives snapshots via WebSocket; it never calculates independent schedule positions."
* **Q: Why did you use modulo arithmetic instead of a ticker loop?**
  * *A:* "A ticker loop drifts over time and resets if the server restarts. Modulo against an anchor timestamp $(T_{\text{now}} - T_{\text{start}}) \pmod{18000}$ is pure, deterministic, and identical across server reboots."
* **Q: How do you prevent race conditions when 100 clients connect to WebSockets?**
  * *A:* "The Hub's client map is protected by a `sync.RWMutex`, and all outgoing messages are queued into per-client buffered channels (`writePump`), ensuring strict single-writer safety."

---

## 33. Difficult Follow-Up Questions

* **Q: What if the MongoDB cluster clock and Go server clock drift?**
  * *A:* "The Go server generates all timestamps (`time.Now().UTC()`) before writing to MongoDB. MongoDB stores the timestamps as BSON dates; it does not generate schedule anchors independently."
* **Q: What happens if a client's network drops for 10 seconds during a 20-second sync?**
  * *A:* "The client's WebSocket closes. Upon reconnecting 10 seconds later, the server's `STATE_SNAPSHOT` informs the client that 10 seconds remain. The client calculates the 10-second offset and seeks into the media."

---

## 34. "Do NOT Say This" Interview Warnings

| Topic | Do NOT Say This ❌ | Say This Instead ✅ |
| :--- | :--- | :--- |
| **Sync Accuracy** | "The screens are mathematically 100% frame-perfect." | "The backend coordinates synchronization using server-authoritative UTC timestamps with a 1000ms lead time. Browser decoding variance may introduce 100–200ms latency." |
| **Blank Behavior** | "The system fills empty cycle time with blank screens." | "The assignment explicitly prohibits automatic blanks. Playlists loop continuously within the 5 hours; blanks appear only if explicitly configured." |
| **Data Storage** | "Sync state is only kept in memory." | "Active sync events and playlists are persisted in MongoDB. If the server crashes, it queries MongoDB on startup and resumes in-flight syncs." |
| **Architecture** | "We need Nginx to make WebSockets work." | "Go's `gorilla/websocket` handles HTTP-to-WS upgrades natively. Direct port exposure simplifies deployment and debugging." |
| **Video Playback** | "Videos always play with full audio automatically." | "Modern browsers enforce strict autoplay policies blocking unmuted sound. Videos play muted by default with explicit audio toggle buttons." |
