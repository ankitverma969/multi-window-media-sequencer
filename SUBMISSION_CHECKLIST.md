# EVA Bharat: Multi-Window Media Sequencer — Submission Checklist

This checklist documents the verified readiness of all technical requirements for the **EVA Bharat Backend Development Intern Assignment**.

---

## 1. Assignment Core Requirements
- [x] **Assignment requirements reviewed against original PDF**: Confirmed 100% adherence to all scenario constraints.
- [x] **React frontend working**: React 19 + Vite SPA with 2x2 multi-window grid, standalone display routing (`/display/:id`), and dynamic controls.
- [x] **Golang backend working**: Clean architecture Go 1.24 server with standard `net/http` router, structured JSON logging (`log/slog`), and Gorilla WebSockets.
- [x] **MongoDB persistence verified**: Configuration survives backend restarts; verified across process kill/restart cycles.
- [x] **Multiple windows verified**: 4 display windows operate simultaneously and independently.
- [x] **Independent playlists verified**: Each window maintains a separate playlist and computes distinct timeline positions.
- [x] **Continuous playback verified**: Zero artificial pauses between items (image $\to$ image, image $\to$ video, video $\to$ image, video $\to$ video).
- [x] **5-hour cycle tested**: Modulo $18{,}000\,\text{seconds}$ wall-clock formula tested across boundaries ($0$, $1\,\text{ms}$ before end, exact boundary, $1\,\text{ms}$ after boundary).
- [x] **Explicit blank tested**: Blanks appear strictly when explicitly configured as a `BLANK` item; 500-sample regression test proved 0 automatic blanks.
- [x] **Dynamic updates tested**: Adding/removing items via REST updates MongoDB, broadcasts via WebSocket, and reflects on displays without page reloads.
- [x] **Sync tested**: Triggering sync override switches all 4 windows to `M2` in unison with server-authoritative timestamps and lead time buffer.
- [x] **Sync expiration tested**: Automatic timer expiration reverts all displays to their scheduled sequence without losing playlist configurations.
- [x] **Playlist preservation tested**: Underlying MongoDB playlist records remain untouched during and after synchronization.
- [x] **Mid-sync join tested**: Late-connecting display discovers active sync and seeks to the calculated offset.
- [x] **WebSocket reconnect tested**: Exponential backoff reconnects automatically and restores latest state snapshots.
- [x] **Backend restart tested**: Recovers in-flight active sync from MongoDB on boot; persists all window/playlist updates.

---

## 2. Infrastructure & Packaging
- [x] **Docker configuration verified**: Multi-stage production Dockerfiles for Go backend and React frontend (Nginx).
- [x] **Docker Compose verified**: `docker-compose.yml` with healthchecks, dependency ordering, and network isolation.
- [x] **Nginx reverse proxy verified**: WebSocket upgrade support (`Connection: Upgrade`), SPA fallback, gzip compression, and security headers.
- [x] **Environment variables documented**: Root, backend, and frontend `.env.example` templates created with local vs production guidance.
- [x] **Seed data verified**: Idempotent seeding creates 4 display windows and 11 media assets; standalone CLI available via `go run ./cmd/seed`.
- [x] **Smoke test script created**: Automated non-destructive verification in `scripts/smoke_test.ps1`.
- [x] **No secrets committed**: Scanned repository for credentials, API keys, and private tokens (0 found).
- [x] **No temporary junk**: Clean git status; all build artifacts, node_modules, and logs excluded via `.gitignore`.
- [x] **Final tests passing**: 10/10 Go backend packages pass, 15/15 frontend Vitest tests pass.
- [x] **Evaluator documentation complete**: `README.md`, `DEMO.md`, `FINAL_TEST_REPORT.md`, `SUBMISSION_CHECKLIST.md`, and `SUBMISSION_MESSAGE.md`.
