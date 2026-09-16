# EVA Bharat: Multi-Window Media Sequencer with Synchronized Playback

A production-grade, server-authoritative multi-window media sequencer and real-time playback synchronization system built for the **EVA Bharat Backend Development Intern Evaluation**.

---

## 1. Overview
In distributed digital signage systems, multiple independent display windows continuously loop distinct media playlists across a fixed **5-hour timeline cycle**. When an operator triggers a **Global Synchronization Event** (e.g., displaying an emergency alert, live broadcast, or product showcase), every connected display must immediately switch to that media item in unison. Upon sync expiration, each window seamlessly returns to its own normal playlist sequence without drift and without losing persistent configurations.

---

## 2. Assignment Requirements & Compliance Matrix

| Requirement | Implementation Component | Test & Verification | Status |
| :--- | :--- | :--- | :---: |
| **React Frontend** | `frontend/src/` (React 19 + Vite) | Vitest component tests & browser E2E | **PASS** |
| **Golang Backend** | `backend/cmd/server/main.go` | Go tests (10 packages, 100% pass) | **PASS** |
| **Persistent Storage** | MongoDB 7.0 (`internal/repository`) | Survives backend restarts & process termination | **PASS** |
| **Multiple Display Windows** | 4 independent windows in 2x2 grid | Verified in browser & API smoke tests | **PASS** |
| **Individual Window Playlists**| `internal/service/window_service.go` | Distinct playlists per window | **PASS** |
| **Continuous Playback** | `internal/timeline/engine.go` | Gapless video/image transitions | **PASS** |
| **5-Hour Cycle Engine** | Modulo $18{,}000\,\text{s}$ wall-clock math | Sub-second boundary tests around $18{,}000\,\text{s}$ | **PASS** |
| **Explicit Blank Behavior** | `internal/models/media.go` | Blank appears strictly for configured duration | **PASS** |
| **Dynamic Playlist Updates** | REST POST $\to$ WebSocket Broadcast | Runtime update without page refresh | **PASS** |
| **Synchronized Media Override**| `internal/service/sync_coordinator.go`| All 4 windows display sync asset simultaneously | **PASS** |
| **Sync Duration & Expiry** | UTC timestamp boundaries + timers | Automatic revert on timer expiration | **PASS** |
| **Return to Normal Playlist** | Authoritative wall-clock query | Resumes exact scheduled second; DB untouched | **PASS** |
| **WebSocket Real-Time Updates**| Gorilla WebSocket hub + `writePump` | Thread-safe single-writer channel delivery | **PASS** |
| **Seed Data** | `backend/seeds/seed.go`, `cmd/seed` | 4 windows, 11 media items seeded idempotently | **PASS** |
| **Deployment Configuration** | Multi-stage Docker + Nginx proxy | Dockerfiles, `compose.yml`, healthchecks | **PASS** |
| **Comprehensive Documentation**| `README.md`, `DEMO.md`, test reports | Full setup, API, and deployment documentation | **PASS** |

---

## 3. Architecture

### System Architecture Diagram
```
                           Internet / User Browsers
                                      │
                                      ▼
                            ┌───────────────────┐
                            │    AWS Route 53   │ (DNS Routing)
                            └─────────┬─────────┘
                                      │
                         HTTPS / WSS  │ (Ports 443, 80)
                                      ▼
                      ┌───────────────────────────────┐
                      │    Nginx Reverse Proxy / SPA  │ (Static Frontend + Proxy)
                      │    - SPA Fallback Routing     │
                      │    - Gzip & Security Headers  │
                      │    - WebSocket Upgrade (WSS)  │
                      └───────┬───────────────┬───────┘
                              │               │
                     REST API │               │ WebSocket (/ws)
             http://backend:8080      http://backend:8080 (Upgrade)
                              │               │
                              ▼               ▼
                      ┌───────────────────────────────┐
                      │     Go Backend Container      │ (Golang 1.24)
                      │  - Authoritative 5h Timeline  │
                      │  - Gorilla WS Single-Writer   │
                      │  - Sync Coordinator & Timers  │
                      └───────────────┬───────────────┘
                                      │
                                      ▼ TLS / Auth (Port 27017)
                      ┌───────────────────────────────┐
                      │   MongoDB Persistent Storage  │
                      │  - Atlas Cloud or Docker Vol  │
                      │  - Windows, Playlists, Sync   │
                      └───────────────────────────────┘
```

### Architectural Tenets
* **Single Source of Truth**: MongoDB is the sole persistent store for configurations. In-memory state is strictly derived from authoritative wall-clock math and MongoDB documents.
* **Server-Authoritative Timing**: The Go backend determines schedule positions. The React frontend renders authoritative state and does not maintain independent drift-prone schedule calculations.

---

## 4. Project Structure
```
multi-window-media-sequencer/
├── backend/
│   ├── cmd/
│   │   ├── server/main.go        # HTTP server, WS hub, and graceful shutdown
│   │   └── seed/main.go          # Standalone seed data CLI tool
│   ├── internal/
│   │   ├── config/               # Environment variable loading & validation
│   │   ├── database/             # MongoDB connection pool & index setup
│   │   ├── handlers/             # HTTP REST controllers & WS upgrade handler
│   │   ├── middleware/           # CORS, structured JSON request logging, recovery
│   │   ├── models/               # Domain entities (Media, Window, Playlist, SyncEvent)
│   │   ├── repository/           # MongoDB repositories (Mongo CRUD with $setOnInsert)
│   │   ├── service/              # Business logic & Authoritative Sync Coordinator
│   │   ├── timeline/             # 5-hour cycle deterministic timeline engine
│   │   ├── utils/                # Standardized JSON response & error wrappers
│   │   └── websocket/            # Gorilla WebSocket Hub with single-writer writePump
│   ├── seeds/                    # Seed data implementation (4 windows, 11 media items)
│   ├── Dockerfile                # Multi-stage production Go binary container
│   └── .env.example              # Backend environment template
├── frontend/
│   ├── src/
│   │   ├── api/client.js         # REST client with relative reverse-proxy routing
│   │   ├── components/           # MediaWindow, VideoPlayer, ImagePlayer, BlankPlayer,
│   │   │                         # StatusBadge, SyncControls, PlaylistPanel
│   │   ├── playback/             # Finite state machine & server clock skew sync
│   │   ├── views/                # GridDashboard (2x2) & SingleDisplay (/display/:id)
│   │   ├── websocket/            # Resilient WebSocket hook with exponential backoff
│   │   ├── App.jsx               # View router
│   │   ├── index.css             # CSS variables & dark design system
│   │   └── App.css               # Component layout & animations
│   ├── Dockerfile                # Multi-stage production container (Node build -> Nginx)
│   ├── nginx.conf                # Nginx reverse proxy with WS upgrade and gzip
│   └── .env.example              # Frontend environment template
├── scripts/
│   └── smoke_test.ps1            # Automated live deployment smoke test
├── docker-compose.yml            # Multi-container local production stack
├── DEMO.md                       # 3–5 minute step-by-step evaluator walkthrough
├── FINAL_TEST_REPORT.md          # Comprehensive QA and verification test matrix
└── README.md                     # Master documentation
```

---

## 5. Playback Model & 5-Hour Cycle Engine

### Core Mathematical Formula
Each window operates under a strict **18,000-second (5-hour)** cycle anchored to `CycleStartTime`:

$$\text{CycleDuration} = 18{,}000\,\text{seconds}$$
$$\text{CycleOffset} = (T_{\text{query}} - T_{\text{cycle\_start}}) \pmod{18{,}000\,\text{s}}$$
$$\text{SequenceOffset} = \text{CycleOffset} \pmod{\text{TotalSequenceDuration}}$$

### Invariants Guaranteed
1. **Zero Artificial Blanks**: If a configured sequence is 60 seconds, it repeats continuously:
   $$M_1 \to M_2 \to M_3 \to M_1 \to M_2 \to M_3 \dots$$
   The engine never pads unused cycle time with blank frames.
2. **Explicit Blank Handling**: Blank media items display strictly for their configured duration and behave as standard sequential playlist elements.
3. **No Boundary Skew**: Wall-clock calculation guarantees that restarting the backend or refreshing the browser returns the exact same item position without drift.

---

## 6. Real-Time Synchronization Design

```
NORMAL SEQUENCE
       │
       ▼ Operator Triggers Sync ("M2", 30s)
[Go Backend SyncCoordinator]
       │
       ├─► Computes: StartTime = Now + 1000ms (Lead Buffer)
       ├─► Computes: EndTime = StartTime + 30s
       ├─► Persists Active Sync to MongoDB
       └─► Broadcasts SYNC_START Envelope via WebSocket Hub
               │
               ▼ (All Connected Displays)
[React Frontend Players]
       │
       ├─► Display Yellow "⚡ SYNC OVERRIDE ACTIVE" Banner
       ├─► Preload & Switch to "M2" Simultaneously
       └─► Run Synchronized Live Countdown
               │
               ▼ (Timer Expires or Operator Cancels)
[Go Backend SyncCoordinator]
       │
       ├─► Broadcasts SYNC_END Envelope
       └─► Marks Sync Inactive in MongoDB
               │
               ▼
[React Frontend Players]
       │
       └─► Query Authoritative 5h Timeline & Resume Normal Sequence
           (Stored Playlists in MongoDB were NEVER Mutated)
```

### Critical Resiliency Features
* **Late-Joining Window**: When a display connects mid-sync, it receives `sync_offset_ms = \max(0, T_{\text{server}} - T_{\text{start}})` and seeks directly into `M2` in unison with other windows.
* **Preemption Policy**: If a new sync is requested while another sync is active, the backend cancels the active timer, supersedes the previous sync, and immediately broadcasts the new event.
* **Crash Recovery**: If the Go backend restarts during an active sync, it queries MongoDB on startup. If the sync has time remaining, it re-arms the timer and continues active sync.

---

## 7. Dynamic Playlist Updates
* Operators can append or remove items from a window's playlist while media is actively playing.
* Updates persist in MongoDB, incrementing the playlist `version`.
* The Go backend emits a `PLAYLIST_UPDATED` WebSocket event.
* The window recalculates its timeline sequence dynamically without requiring a full browser refresh.

---

## 8. MongoDB Data Model

### Collections
1. **`windows`**: Display window metadata, location, cycle duration, and cycle anchor timestamp.
   - Index: `window_number` (Unique).
2. **`media`**: Asset catalog (videos, images, blanks), duration, and URLs.
   - Index: `media_key` (Unique).
3. **`playlists`**: Ordered array of playlist items per window, total duration, and version counter.
   - Index: `window_number` (Unique).
4. **`sync_events`**: Active and historical synchronization audit log with start/end timestamps.
   - Index: `is_active`, `start_time`.

---

## 9. API & WebSocket Documentation

### REST API Endpoints
| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET` | `/health` | Healthcheck and MongoDB ping status. |
| `GET` | `/api/v1/time` | Authoritative UTC server timestamp for clock skew compensation. |
| `GET` | `/api/v1/windows` | List all display windows. |
| `GET` | `/api/v1/windows/:id` | Get window configuration by ID or number. |
| `GET` | `/api/v1/media` | List media catalog assets. |
| `GET` | `/api/v1/windows/:id/playlist` | Get configured playlist for a window. |
| `POST` | `/api/v1/windows/:id/playlist` | Append item to a window's playlist. |
| `DELETE`| `/api/v1/windows/:id/playlist/:itemId`| Remove an item from a window's playlist. |
| `GET` | `/api/v1/windows/:id/playback` | Authoritative playback calculation from 5-hour engine. |
| `POST` | `/api/v1/sync` | Trigger global sync override (`media_key`, `duration_seconds`). |
| `GET` | `/api/v1/sync/current` | Get active synchronization event if in-flight. |
| `POST` | `/api/v1/sync/:id/cancel` | Cancel an active synchronization override immediately. |

### WebSocket Protocol (`/ws?window_id=N`)
Envelopes follow a strictly typed schema:
```json
{
  "type": "STATE_SNAPSHOT | PLAYLIST_UPDATED | SYNC_STARTED | SYNC_ENDED | ERROR | PONG",
  "version": 1,
  "timestamp": "2026-09-16T16:01:44.237Z",
  "payload": { ... }
}
```

---

## 10. Local Setup & Execution

### 1. Start MongoDB
Ensure MongoDB is running locally on port `27017`:
```bash
mongosh --eval "db.adminCommand('ping')"
```

### 2. Run Go Backend
```bash
cd backend
go run ./cmd/server
```
The backend initializes indexes, seeds sample windows/media, and listens on `:8080`.

### 3. Run Standalone Database Seeder (Optional)
```bash
cd backend
go run ./cmd/seed
```

### 4. Run React Frontend
```bash
cd frontend
npm install
npm run dev
```
Open `http://localhost:5173` in any browser.

---

## 11. Docker & Production Stack

### Launch Full Stack with One Command
```bash
docker compose up --build -d
```
* **Frontend**: `http://localhost:5173`
* **Backend Health**: `http://localhost:8080/health`
* **MongoDB**: Internal port `27017` (isolated to `127.0.0.1`).

---

## 12. AWS Deployment Instructions (EC2 + Docker + Atlas)

1. **Launch EC2**: Ubuntu 24.04 LTS (`t3.small`). Configure Security Group with ports `22` (SSH), `80` (HTTP), `443` (HTTPS).
2. **MongoDB Atlas**: Create free M0 cluster. Whitelist EC2 IP in Network Access.
3. **Provisioning Script**:
   ```bash
   sudo apt-get update && sudo apt-get upgrade -y
   curl -fsSL https://get.docker.com -o get-docker.sh && sudo sh get-docker.sh
   git clone https://github.com/ankitverma969/multi-window-media-sequencer.git
   cd multi-window-media-sequencer
   cp .env.example .env
   # Edit .env with your MongoDB Atlas URI
   docker compose up -d --build
   ```
4. **SSL (Certbot)**:
   ```bash
   sudo apt-get install -y certbot python3-certbot-nginx
   sudo certbot --nginx -d yourdomain.com
   ```

*Note: For the internship evaluation on this local environment, AWS credentials were not pre-configured. Live deployment status is documented as `BLOCKED — NOT VERIFIED`.*

---

## 13. Live Application URLs
* **Frontend Dashboard**: `http://localhost:5173/`
* **Backend Health**: `http://localhost:8080/health`
* **Standalone Display 1**: `http://localhost:5173/#/display/1`
* **AWS Live Deployment**: `BLOCKED — NOT VERIFIED` *(Requires AWS credentials)*

---

## 14. Testing & Verification

### Run Backend Tests
```bash
cd backend
go test -count=1 ./...
```
*10/10 Go packages passing.*

### Run Frontend Tests
```bash
cd frontend
npm test -- --run
```
*15/15 Vitest tests passing.*

### Run Live Deployment Smoke Test
```bash
powershell -ExecutionPolicy Bypass -File .\scripts\smoke_test.ps1
```
*8/8 checks passing.*

---

## 15. Assumptions & Tradeoffs

1. **Deterministic Wall-Clock Modulo**: A window's position is computed using modulo arithmetic against an anchor timestamp rather than an in-memory counter. This ensures zero drift across backend restarts.
2. **Predictive Lead Time for Video Sync**: Triggering "Play Now" over WebSockets incurs network jitter. Providing a 1000ms lead buffer gives browsers time to preload and start media synchronously.
3. **Muted Autoplay Policy**: Modern Chromium browsers block programmatic unmuted video playback without user gesture. Videos default to muted with explicit interactive unmute toggles on each card.
4. **Wall-Clock Schedule Resumption**: Post-sync resumption snaps to where the window *should* be right now according to its 5-hour cycle, preserving broadcast fidelity.
5. **Preemption Policy**: A second sync request immediately preempts and supersedes any active sync.

---

## 16. Troubleshooting Guide

| Issue | Root Cause | Resolution |
| :--- | :--- | :--- |
| **WebSocket Connection Fails (`1006`)** | Reverse proxy missing upgrade headers. | Verify `Upgrade $http_upgrade` and `Connection "Upgrade"` in `nginx.conf`. |
| **CORS Error in Console** | Frontend origin missing in `CORS_ALLOWED_ORIGINS`. | Add frontend URL to `CORS_ALLOWED_ORIGINS` in `.env`. |
| **Video Playback Paused** | Browser autoplay policy blocked sound. | Click the sound icon (`🔊 / 🔇`) on the window header to unmute. |
| **502 Bad Gateway** | Backend container still starting or DB unreachable. | Check `docker compose logs backend` and verify MongoDB is healthy. |
| **Port Conflict on 5173 or 8080** | Previous process running. | Terminate existing processes on port 8080/5173. |
