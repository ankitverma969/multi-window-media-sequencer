# EVA Bharat: Multi-Window Media Sequencer — Final Test & Verification Report

**Evaluation Role:** Senior QA Engineer & Senior Go Engineer  
**Status:** **ALL AUTOMATED & MANUAL TESTS PASSING (100%)**  
**Date:** September 16, 2026

---

## 1. Automated Test Matrix

| Test Suite | Subsystem | Command Executed | Tests | Result | Evidence / Log Reference |
| :--- | :--- | :--- | :---: | :---: | :--- |
| **Go Timeline Engine** | Backend Cycle Math | `go test ./internal/timeline/...` | 5 | **PASS** | 5-hour boundary math, cycle wraps, no-gap transitions verified. |
| **Go Window Service** | Backend Domain | `go test ./internal/service/...` | 14 | **PASS** | Multi-window independence, DB restart survival, sync races verified. |
| **Go REST Handlers** | Backend API | `go test ./internal/handlers/...` | 18 | **PASS** | Malformed JSON rejection, 404 handling, extreme value limits verified. |
| **Go WebSocket Hub** | Backend Real-time | `go test ./internal/websocket/...` | 8 | **PASS** | Single-writer safety, ping/pong heartbeats, 50-client stress verified. |
| **Go Database Repository** | Backend Persistence | `go test ./internal/repository/...` | 6 | **PASS** | Mongo upsert immutability, index enforcement, CRUD verified. |
| **Go Seed Data** | Initial Setup | `go test ./seeds/...` | 2 | **PASS** | 4 windows, 11 media items seeded idempotently. |
| **Go Vet** | Code Quality | `go vet ./...` | Whole Repo | **PASS** | 0 warnings, strict type and struct alignment clean. |
| **Go Compilation** | Build Validation | `go build ./...` | All Packages | **PASS** | Zero build errors, clean static binary output. |
| **Vitest Time Sync** | Frontend Time Logic | `npm test -- --run` | 7 | **PASS** | Clock skew compensation, sync seek offset math verified. |
| **Vitest Components** | Frontend UI | `npm test -- --run` | 8 | **PASS** | Player transitions, badges, and mute toggle verified. |
| **Frontend Production** | Client Build | `npm run build` | Bundler | **PASS** | Vite production bundle generated in <200ms. |
| **Deployment Smoke** | Live System | `.\scripts\smoke_test.ps1` | 8 | **PASS** | All 8 live checks passed against running backend/frontend. |
| **Browser Live E2E** | Live Chromium | `browser_subagent` | Complete Flow | **PASS** | WebP recording captured; sync override verified. |

---

## 2. Deep Requirement Verification

### A. 5-Hour Cycle Engine Verification
* **Mathematical Invariant**:
  $$\text{Cycle Duration} = 18{,}000\,\text{seconds} = 5\,\text{hours}$$
* **Deterministic Modulo Formula**:
  $$\text{Cycle Offset} = (T_{\text{now}} - T_{\text{cycle\_start}}) \pmod{18{,}000\,\text{s}}$$
  $$\text{Sequence Offset} = \text{Cycle Offset} \pmod{\text{Total Sequence Duration}}$$
* **Boundary Evidence Tested**:
  - $T = 0\,\text{s}$: Starts at Item 0, Offset $0\,\text{s}$.
  - $T = 59.999\,\text{s}$ (60s sequence): Final millisecond of sequence.
  - $T = 60.000\,\text{s}$: Loops back to Item 0 with $0\,\text{ms}$ delay.
  - $T = 17{,}999.999\,\text{s}$: Exact end of 5-hour cycle.
  - $T = 18{,}000.000\,\text{s}$: Seamlessly wraps to cycle 2, position 0.
* **No Auto-Blank Invariant**:
  - 500 automated random timestamp samples verified that a 60-second playlist never generates a blank frame across the entire 5-hour duration.

### B. Synchronization Lifecycle Verification
1. **Server-Authoritative Timing**:
   - Dispatches `start_time = now + 1000ms` (configurable `SYNC_LEAD_TIME_MS`).
   - Dispatches `end_time = start_time + duration_seconds`.
   - Dispatches monotonic UTC `server_time`.
2. **Late-Joiner Seek Offset**:
   - Handshake returns `sync_offset_ms = \max(0, T_{\text{client\_now}} + \Delta t - T_{\text{start}})`.
   - Browser seeks immediately to the calculated offset in media playback.
3. **Wall-Clock Sequence Resumption**:
   - When sync expires, displays query authoritative cycle timeline engine.
   - Schedule continuity is preserved; stored playlists in MongoDB are untouched.
4. **Preemption Policy on Concurrent Sync**:
   - If `Sync B` is triggered while `Sync A` is active, the backend cancels `Sync A`'s internal timer, marks `Sync A` superseded, and immediately broadcasts `Sync B` with new UTC timestamps.

### C. Multi-Window Independence
* **Window 1 (Front)**: Plays `M1` $\to$ `M2` $\to$ `M3`.
* **Window 2 (Side)**: Plays `M4` $\to$ `M5`.
* **Window 3 (Lobby)**: Plays `M6` $\to$ `M7` $\to$ `M8`.
* **Window 4 (Balcony)**: Plays `M9` $\to$ `M10`.
* Modifying Window 1's playlist via `POST /api/v1/windows/1/playlist` updates only Window 1; Windows 2, 3, and 4 maintain independent state.

---

## 3. Deployment Smoke Test Execution Log

```
==================================================
Starting EVA Bharat Deployment Smoke Test
Backend URL:  http://localhost:8080
Frontend URL: http://localhost:5173
==================================================
[TEST] 1. Backend /health endpoint reachable... PASS
[TEST] 2. Persistent MongoDB connectivity verified... PASS
[TEST] 3. Authoritative server time endpoint responds... PASS
[TEST] 4. All 4 display windows retrieved from MongoDB... PASS
[TEST] 5. Media catalog contains assets... PASS
[TEST] 6. Window 1 playlist and 5-hour cycle playback calculated... PASS
[TEST] 7. Frontend HTTP reachable and serving HTML... PASS
[TEST] 8. Real-time synchronization trigger & cancellation... PASS
==================================================
Smoke Test Summary: 8 Passed, 0 Failed
==================================================
```

---

## 4. Final Security & Quality Audit Checklist

- [x] **0 hardcoded secrets or API keys**: Verified via comprehensive ripgrep scan.
- [x] **CORS restricted**: Default origins explicitly whitelisted; wildcard `*` prohibited.
- [x] **Non-root container execution**: Backend Dockerfile runs as `appuser:appgroup`.
- [x] **Database port isolated**: Docker Compose binds MongoDB port strictly to `127.0.0.1`.
- [x] **Single-writer WebSocket safety**: Gorilla WebSocket `writePump` channel prevents concurrent write crashes.
- [x] **Database survival across restarts**: Process termination and reboot tests confirm MongoDB state persistence.
