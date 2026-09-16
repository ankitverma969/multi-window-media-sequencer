# Media Sequencer Backend — Go & MongoDB

Enterprise backend for the **Multi-Window Media Sequencer with Synchronized Playback** (EVA Bharat Backend Development Intern Evaluation).

---

## 1. Status Overview

| Component | Status | Details |
| :--- | :--- | :--- |
| **Go HTTP Architecture** | **IMPLEMENTED** | Standard Go 1.22+ routing, middleware stack, structured logging, graceful shutdown. |
| **MongoDB Integration** | **IMPLEMENTED** | Official driver (`go.mongodb.org/mongo-driver/v2`), connection pooling, ping checks, index creation. |
| **Domain Models & Repositories** | **IMPLEMENTED** | `Media`, `Window`, `Playlist`, `PlaylistItem`, and `SyncEvent` models with atomic repository operations. |
| **REST API Foundation** | **IMPLEMENTED** | Complete production REST API: `/health`, `/api/v1/time`, `/api/v1/windows`, `/api/v1/media`, `/api/v1/windows/{id}/playlist`, `/api/v1/windows/{id}/playback`, `/api/v1/sync` ([API reference](docs/api_reference.md)). |
| **Middleware & Error Handling** | **IMPLEMENTED** | Strict CORS, JSON envelopes, structured request logging, panic recovery. |
| **Seed Data System** | **IMPLEMENTED** | Candidate-created baseline dataset populating 3 windows and 8 media assets. |
| **Testing Suite** | **IMPLEMENTED** | Pure unit tests for config, models, repositories, services, handlers, middleware, seeds, and timeline engine. |
| **5-Hour Cycle Engine** | **IMPLEMENTED** | Deterministic timeline math engine with continuous repetition, $[start, end)$ boundary convention, and sub-second precision ([documentation](docs/cycle_engine.md)). |
| **WebSocket Hub & Real-Time Sync** | **PLANNED** | Full-duplex WebSocket broadcast hub and client clock synchronization (next milestone). |
| **React Frontend** | **NOT YET IMPLEMENTED** | Multi-window UI & dual-buffer player will follow backend completion. |

---

## 2. Technology Stack

* **Language:** Golang (Go 1.22+)
* **Database:** MongoDB 7.0 (Mandatory persistent source of truth)
* **MongoDB Driver:** Official `go.mongodb.org/mongo-driver/v2`
* **Logging:** Standard library `log/slog` (structured JSON logging)
* **Testing:** Standard `testing` package with clean in-memory mock repositories
* **Containerization:** Multi-stage Dockerfile based on Alpine Linux

---

## 3. Project Structure

```
backend/
├── cmd/
│   └── server/
│       └── main.go              # Application entrypoint, dependency wiring, graceful shutdown
├── internal/
│   ├── config/
│   │   ├── config.go            # Environment variable loader & validator
│   │   └── config_test.go       # Config unit tests
│   ├── database/
│   │   ├── database.go          # MongoDB client connection, ping check, pool management
│   │   └── indexes.go           # Index definitions and initialization
│   ├── handlers/
│   │   ├── health.go            # Liveness & DB connectivity handler
│   │   ├── time.go              # Server time endpoint for client clock sync
│   │   ├── media.go             # Media catalog management
│   │   ├── windows.go           # Display window queries
│   │   ├── playlist.go          # Playlist retrieval and atomic item additions
│   │   ├── routes.go            # Mux route registration and middleware chain
│   │   └── handlers_test.go     # Handler unit tests
│   ├── middleware/
│   │   ├── cors.go              # Production CORS policy with explicit origins
│   │   ├── logger.go            # Structured HTTP request logger
│   │   ├── recovery.go          # Safe panic recovery
│   │   └── middleware_test.go   # Middleware unit tests
│   ├── models/
│   │   ├── models.go            # Domain models: Media, Window, Playlist, SyncEvent
│   │   └── models_test.go       # Domain validations and calculation tests
│   ├── repository/
│   │   ├── repository.go        # Repository interfaces & error sentinels
│   │   ├── mongo_media.go       # MongoDB Media collection repository
│   │   ├── mongo_window.go      # MongoDB Window collection repository
│   │   ├── mongo_playlist.go    # MongoDB Playlist collection repository
│   │   ├── mongo_sync.go        # MongoDB SyncEvent collection repository
│   │   ├── mock_repository.go   # Thread-safe in-memory test mocks
│   │   └── repository_test.go   # Repository contract tests
│   ├── service/
│   │   ├── service.go           # Service interfaces
│   │   ├── window_service.go    # Window business logic
│   │   ├── media_service.go     # Media business logic
│   │   ├── playlist_service.go  # Playlist business logic & asset validation
│   │   └── service_test.go      # Service unit tests
│   └── utils/
│       ├── response.go          # Unified API response & error envelope
│       └── response_test.go     # Envelope unit tests
├── seeds/
│   ├── seed.go                  # Candidate-created baseline dataset
│   └── seed_test.go             # Seed verification tests
├── .env.example                 # Documented environment variables
├── Dockerfile                   # Production multi-stage Alpine Dockerfile
├── go.mod
├── go.sum
└── README.md
```

---

## 4. Persistent Storage & MongoDB Design

MongoDB is the authoritative persistent store for all application data.

### Collections & Indexes

1. **`media`**: Media assets catalog.
   * `media_key` (Unique index): Stable identifier (e.g., `M1`, `M2`).
   * `type`: Standard index for filtering by `video`, `image`, `blank`.
2. **`windows`**: Display window records.
   * `window_number` (Unique index): Display window sequence (1, 2, 3...).
3. **`playlists`**: Authoritative playlist document per window.
   * `window_number` (Unique index)
   * `window_id` (Unique index)
   * `items`: Embedded array of `PlaylistItem` elements, updated atomically via `$set`, `$push`, and `$inc`.
4. **`sync_events`**: Sync override event audit log and active state.
   * `event_id` (Unique index)
   * `status` + `end_time` (Compound index for fast retrieval of active sync)
   * `start_time` (Descending index for timeline queries)

---

## 5. Candidate-Created Seed Data

> **Important Clarification on Assignment Seed Data:**  
> The assignment specification explicitly states: *"Seed data matching the example windows and media lists"* and mentions *"for example M2"*. It does not supply complete asset URLs or durations in the text.  
> To satisfy the evaluation criteria reliably without false attribution, this project initializes **Candidate-Created Development Seed Data** using royalty-free, public-domain assets:

* **Window 1:** $M_1 \text{ (Video, 15s)} \to M_2 \text{ (Image, 10s)} \to M_3 \text{ (Video, 20s)}$
* **Window 2:** $M_4 \text{ (Video, 12s)} \to M_5 \text{ (Image, 8s)} \to \text{BLANK\_10 (Configured Blank, 10s)}$
* **Window 3:** $M_1 \text{ (Video, 15s)} \to M_6 \text{ (Image, 14s)} \to M_7 \text{ (Video, 18s)}$

---

## 6. Environment Configuration

Copy `.env.example` to `.env` or set environment variables:

| Variable | Default | Description |
| :--- | :--- | :--- |
| `PORT` | `8080` | Port for the HTTP server |
| `ENVIRONMENT` | `development` | Runtime environment (`development`, `production`, `test`) |
| `MONGODB_URI` | `mongodb://localhost:27017` | MongoDB connection URI string |
| `MONGODB_DATABASE` | `media_sequencer` | Target MongoDB database name |
| `CORS_ALLOWED_ORIGINS` | `http://localhost:5173,...` | Comma-separated list of allowed origins |
| `SYNC_LEAD_TIME_MS` | `1000` | Pre-buffering lead time before sync playback begins |
| `SEED_ON_STARTUP` | `true` | Automatically populate baseline seed data on startup |

---

## 7. Local Setup & Running

### Option A: Running Natively with Go

1. Ensure MongoDB is running locally on port 27017:
   ```bash
   # If using Docker for MongoDB only:
   docker run -d --name mongo -p 27017:27017 mongo:7.0
   ```
2. Set up environment:
   ```bash
   cp .env.example .env
   ```
3. Run the server:
   ```bash
   go run ./cmd/server
   ```
4. Verify health:
   ```bash
   curl http://localhost:8080/health
   ```

### Option B: Running via Docker Compose

From the project root:
```bash
docker compose up --build
```

---

## 8. Testing

Run all unit tests across all packages:

```bash
go test -v ./...
```

Run race detector (requires CGO or Go race tooling):
```bash
go test -race ./...
```
