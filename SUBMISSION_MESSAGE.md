# EVA Bharat Backend Development Intern — Final Submission Message

**Candidate Name:** Ankit Verma  
**Repository:** [https://github.com/ankitverma969/multi-window-media-sequencer](https://github.com/ankitverma969/multi-window-media-sequencer)  
**Evaluation Role:** Backend Development Intern  
**Project Title:** Multi-Window Media Sequencer with Synchronized Playback  

---

## Executive Summary
This submission implements a production-grade, server-authoritative multi-window media sequencer and real-time playback synchronization system designed for digital signage installations. Built strictly adhering to the assignment requirements, the application coordinates multiple independent display windows continuously cycling media over a fixed **5-hour cycle**, supports dynamic runtime playlist modifications, and provides an authoritative synchronization mechanism that temporarily overrides all windows simultaneously before seamlessly resuming normal schedules.

---

## Technology Stack
* **Backend**: Golang (Go 1.24) with Clean Architecture, standard library `net/http` routing, structured JSON logging (`log/slog`), and Gorilla WebSockets (`gorilla/websocket`).
* **Database**: MongoDB 7.0 Community / Atlas with unique index constraints and persistent storage across process restarts.
* **Frontend**: React 19, Vite, and Vanilla CSS with custom modern design system (responsive 2x2 grid, standalone display routing, audio unmute gestures, and server time skew compensation).
* **Containerization**: Multi-stage production Dockerfiles for Go and React + Nginx reverse proxy (`docker-compose.yml`).
* **Testing**: Comprehensive Go unit/integration test suite (10 packages, 100% pass rate) + Frontend Vitest suite (15/15 pass) + Automated deployment smoke test (`scripts/smoke_test.ps1`).

---

## Key Core Features
1. **Authoritative 5-Hour Cycle Engine**:
   - Each window operates on a strict $18{,}000\,\text{second}$ cycle anchored to wall-clock time.
   - Modulo arithmetic prevents time drift across restarts.
   - Shorter playlists repeat continuously; **zero artificial blank padding** is introduced.
2. **Server-Authoritative Synchronization**:
   - Broadcasts synchronized events with a predictive 1000ms lead buffer to allow client preloading.
   - Late-joining displays discover ongoing sync and calculate exact seek offsets.
   - Expiration automatically restores the scheduled sequence without mutating stored playlists.
3. **Dynamic Runtime Playlist Updates**:
   - Items can be added/removed via REST API while media is actively playing.
   - Updates persist in MongoDB, increment the playlist version, and propagate over WebSockets without full-page reloads.
4. **Gorilla WebSocket Single-Writer Safety**:
   - Dedicated per-client `writePump` channel queues prevent concurrent write panics during mass broadcasts.
5. **Production Resiliency**:
   - Automatic exponential backoff on client disconnections.
   - On boot, the Go backend queries MongoDB to recover any in-flight active sync events.

---

## Verified Endpoints & Access
* **Frontend Application**: `http://localhost:5173/`
* **Standalone Display 1 Route**: `http://localhost:5173/#/display/1`
* **Backend Health Check**: `http://localhost:8080/health` (Returns HTTP 200 with DB ping status)
* **REST API Windows**: `http://localhost:8080/api/v1/windows`
* **WebSocket Endpoint**: `ws://localhost:8080/ws?window_id=N`
* **Cloud Deployment Note**: Local production stack is verified 100%. AWS cloud deployment was not executed as AWS credentials were not configured in this local environment (marked as `BLOCKED — NOT VERIFIED`). Copy-pasteable AWS EC2 deployment commands and MongoDB Atlas configuration steps are fully documented in Section 12 of `README.md`.

---

## Documentation Quick Links
* **Master Documentation**: `README.md`
* **3–5 Minute Evaluator Demo Guide**: `DEMO.md`
* **Comprehensive Test Matrix & Evidence**: `FINAL_TEST_REPORT.md`
* **Verified Submission Checklist**: `SUBMISSION_CHECKLIST.md`

Thank you for reviewing this submission!
