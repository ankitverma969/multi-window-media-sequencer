# EVA Bharat: Multi-Window Media Sequencer with Synchronized Playback

A production-grade, server-authoritative multi-window media sequencer and real-time playback synchronization system built for the **EVA Bharat Backend Development Intern Evaluation**.

---

## 1. Production Architecture Overview

```
                           Internet / Client Devices
                                      │
                                      ▼
                            ┌───────────────────┐
                            │    AWS Route 53   │ (DNS)
                            └─────────┬─────────┘
                                      │
                                HTTPS │ (Port 443 / 80)
                                      ▼
                      ┌───────────────────────────────┐
                      │    Nginx Reverse Proxy / SPA  │ (Static Frontend + Proxy)
                      │    - SPA Fallback Routing     │
                      │    - Gzip & Security Headers  │
                      │    - WebSocket Upgrade (WSS)  │
                      └───────┬───────────────┬───────┘
                              │               │
                     REST API │               │ WebSocket (/ws)
             http://localhost:8080    http://localhost:8080 (Upgrade)
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

### Key Architectural Tenets
* **Single Source of Truth**: MongoDB is the sole persistent store for configuration; in-memory state is strictly derived from authoritative wall-clock formulas and MongoDB documents.
* **Predictive Server Timing**: Synchronization events use UTC timestamps with a configurable lead time (default 1000ms), allowing clients to preload assets and begin playback in unison.
* **Wall-Clock Schedule Resumption**: Normal looping sequence is calculated against the 5-hour boundary ($T_{\text{cycle}} = 18{,}000\,\text{s}$). Resuming post-sync snaps to the exact second where the window *should* be, preventing schedule drift.

---

## 2. Prerequisites
* **Local Development**:
  * [Go](https://golang.org/) 1.22+ (tested on Go 1.24)
  * [Node.js](https://nodejs.org/) 20+ and npm 10+
  * [MongoDB Community Server](https://www.mongodb.com/try/download/community) 7.0+ (or Docker)
* **Containerized Deployment**:
  * [Docker](https://www.docker.com/) 24+ and Docker Compose v2+
* **Cloud Deployment (AWS)**:
  * AWS EC2 instance (Ubuntu 24.04 LTS, `t3.small` or `t3.medium` recommended)
  * Elastic IP / Public DNS record
  * MongoDB Atlas M0 Free Cluster or dedicated MongoDB instance

---

## 3. Environment Variables Configuration

Copy `.env.example` to `.env` or set environment variables in your deployment pipeline:

### Configuration Reference Matrix
| Variable | Default (Local) | Production Example | Description |
| :--- | :--- | :--- | :--- |
| `PORT` | `8080` | `8080` | Port on which the Go HTTP server listens. |
| `ENVIRONMENT` | `development` | `production` | Environment tier (`development` or `production`). |
| `MONGODB_URI` | `mongodb://localhost:27017` | `mongodb+srv://user:pass@cluster.mongodb.net/?...` | MongoDB connection URI with credentials and TLS. |
| `MONGODB_DATABASE` | `media_sequencer` | `media_sequencer_prod` | Target MongoDB database name. |
| `CORS_ALLOWED_ORIGINS` | `http://localhost:5173,http://localhost:3000` | `https://app.yourdomain.com` | Whitelist of allowed frontend web origins. |
| `SYNC_LEAD_TIME_MS` | `1000` | `1000` | Milliseconds of lead buffer provided before sync begins. |
| `SEED_ON_STARTUP` | `true` | `false` | When true, populates initial sample windows and playlists. |
| `VITE_API_URL` | *(empty / auto-detected)* | `https://api.yourdomain.com` or `""` (relative) | Base URL for REST endpoints. |
| `VITE_WS_URL` | *(empty / auto-detected)* | `wss://api.yourdomain.com` or `""` (relative) | Base URL for WebSocket endpoint. |

> [!IMPORTANT]
> Never commit `.env` files or credentials to Git. The repository's `.gitignore` explicitly excludes `.env`, `*.env`, and `.env.*`.

---

## 4. Local Development Setup

### 1. Start MongoDB
Ensure MongoDB is running locally on port `27017`:
```bash
# Verify local MongoDB status
mongosh --eval "db.adminCommand('ping')"
```

### 2. Start Go Backend
```bash
cd backend
go run ./cmd/server
```
* The backend binds to `http://localhost:8080`.
* It automatically seeds initial windows, media catalog, and playlists on first launch if `SEED_ON_STARTUP=true`.

### 3. Run Standalone Database Seeder (Optional)
To seed or reset development data manually without restarting the server:
```bash
cd backend
go run ./cmd/seed
```

### 4. Start React Frontend
```bash
cd frontend
npm install
npm run dev
```
Open `http://localhost:5173` in your browser.

---

## 5. Docker & Containerized Production Stack

The repository includes multi-stage production Dockerfiles and a unified `docker-compose.yml`:

### Build & Run the Full Stack Locally
```bash
docker compose up --build -d
```

### Service Map
* **Frontend (Nginx + React SPA)**: `http://localhost:5173`
* **Backend REST API**: `http://localhost:8080/api/v1/health`
* **WebSocket Endpoint**: `ws://localhost:8080/ws`
* **MongoDB Container**: Bound strictly to `127.0.0.1:27017` for security.

### Verify Container Health
```bash
docker compose ps
```
All three containers (`media_sequencer_mongo`, `media_sequencer_backend`, `media_sequencer_frontend`) should report status `healthy`.

---

## 6. AWS Production Deployment Guide

We recommend **Option A: EC2 + Docker Compose + Nginx + MongoDB Atlas** for the EVA Bharat evaluation. It provides full reproducibility, isolated networking, high performance, and minimal operational overhead.

### Architecture Selection Rationale
* **Cost & Simplicity**: Fits within AWS Free Tier (`t3.small` EC2 instance + MongoDB Atlas M0).
* **Self-Contained Networking**: Nginx acts as the single public entry point, terminating SSL, serving optimized static frontend bundles, and proxying `/api` and `/ws` internally to the Go backend.
* **Persistent & Managed Data**: MongoDB Atlas handles clustering, automated backups, and encryption at rest without risking data loss on EC2 volume termination.

### Step-by-Step EC2 Deployment

#### 1. Provision EC2 Instance
* **OS**: Ubuntu 24.04 LTS (x86_64)
* **Instance Type**: `t3.small` (2 vCPU, 2GB RAM)
* **Security Group Rules**:
  * Inbound TCP `22` (SSH from your IP only)
  * Inbound TCP `80` (HTTP from anywhere `0.0.0.0/0`)
  * Inbound TCP `443` (HTTPS from anywhere `0.0.0.0/0`)
  * *(Keep port 8080 and 27017 closed to the public)*

#### 2. Provision MongoDB Atlas Cluster
1. Create a free M0 cluster on [MongoDB Atlas](https://www.mongodb.com/cloud/atlas).
2. Create a database user with read/write permissions to `media_sequencer_prod`.
3. In Network Access, whitelist the Elastic IP of your EC2 instance.
4. Copy the connection string: `mongodb+srv://<user>:<pass>@cluster0.mongodb.net/?retryWrites=true&w=majority`.

#### 3. Setup EC2 Server
Connect via SSH and install Docker:
```bash
sudo apt-get update && sudo apt-get upgrade -y
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh
sudo usermod -aG docker ubuntu
# Log out and log back in for docker group to take effect
```

#### 4. Clone Repository & Configure Secrets
```bash
git clone https://github.com/ankitverma969/multi-window-media-sequencer.git
cd multi-window-media-sequencer

# Create production .env file
cat <<EOF > .env
PORT=8080
ENVIRONMENT=production
MONGODB_URI=mongodb+srv://<user>:<password>@cluster0.abcde.mongodb.net/?retryWrites=true&w=majority
MONGODB_DATABASE=media_sequencer_prod
CORS_ALLOWED_ORIGINS=https://yourdomain.com,http://your-ec2-ip
SYNC_LEAD_TIME_MS=1000
SEED_ON_STARTUP=true
EOF
```

#### 5. Launch Production Stack
```bash
docker compose up -d --build
```

#### 6. Configure HTTPS with Let's Encrypt (Certbot)
If you have pointed a domain to your EC2 IP:
```bash
sudo apt-get install -y certbot python3-certbot-nginx
sudo certbot --nginx -d yourdomain.com
```
Certbot will configure SSL certificates and automatic HTTP $\to$ HTTPS redirection.

---

## 7. Reverse Proxy & WebSocket Configuration

The frontend production container includes an optimized Nginx configuration (`frontend/nginx.conf`):

```nginx
# WebSocket Upgrade Block in frontend/nginx.conf
location /ws {
    proxy_pass http://backend:8080/ws;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "Upgrade";
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_read_timeout 86400s;
    proxy_send_timeout 86400s;
    proxy_buffering off;
}
```

* **Connection: "Upgrade"**: Essential for HTTP/1.1 WebSocket handshakes.
* **`proxy_buffering off;`**: Disables proxy buffer delays to ensure real-time latency.
* **`proxy_read_timeout 86400s;`**: Prevents Nginx from severing idle WebSocket connections after 60s.

---

## 8. Health Check Verification

The `/health` endpoint distinguishes between application liveness and critical storage connectivity:

```bash
# Healthy state (HTTP 200)
curl -i http://localhost:8080/health
```
```json
{
  "status": "ok",
  "database": "connected",
  "environment": "production"
}
```

If MongoDB becomes unreachable:
```json
{
  "error": {
    "code": "DATABASE_UNAVAILABLE",
    "message": "Persistent storage is currently unreachable"
  }
}
```
*(HTTP 503 Service Unavailable)*

---

## 9. Automated Testing & Verification Suite

### Run Backend Tests (10/10 Packages)
```bash
cd backend
go test -v -count=1 ./...
```
* **Coverage**: Config, Database, Models, Repositories, Services, Handlers, Timeline Engine, Utilities, WebSocket Hub, and Integration tests.

### Run Frontend Tests (15/15 Tests)
```bash
cd frontend
npm test -- --run
```
* **Coverage**: Time synchronization formulas, offset calculations, connection recovery, and player component transitions.

### Production Bundle Verification
```bash
cd frontend
npm run build
```
* Builds production-ready gzip assets in under 300ms.

---

## 10. Troubleshooting Guide

| Issue | Root Cause | Solution |
| :--- | :--- | :--- |
| **WebSocket Connection Fails (`400` or `1006`)** | Nginx missing upgrade headers or reverse proxy timeout. | Ensure `proxy_set_header Upgrade $http_upgrade;` and `proxy_set_header Connection "Upgrade";` are in Nginx config. |
| **CORS Error in Browser Console** | Frontend origin not present in `CORS_ALLOWED_ORIGINS`. | Add your frontend domain or IP to the `CORS_ALLOWED_ORIGINS` environment variable in `.env` and restart backend. |
| **MongoDB Connection Timeout** | Network firewall or Atlas IP whitelist blocking connection. | In MongoDB Atlas, verify your server's public IP is added under **Network Access $\to$ IP Access List**. |
| **Video Autoplay Blocked** | Browser security policy restricts unmuted media playback. | Videos default to `muted = true` with `playsInline`. Users can click the audio toggle icon (`🔊 / 🔇`) on each window card. |
| **502 Bad Gateway** | Go backend container is not yet ready or crashed. | Check backend logs via `docker compose logs backend`. Verify MongoDB healthcheck passed. |
| **Schedule Resets After Restart** | State stored in memory rather than DB. | The system stores playlists and sync events in MongoDB. Verify MongoDB volume `mongo_data` is mounted. |

---

## 11. Technical Discussion Points for EVA Bharat Evaluator

1. **Deterministic 5-Hour Cycle Engine**:
   - Rather than accumulating seconds and drifting, the engine uses modulo arithmetic against an immutable anchor timestamp ($T_{\text{start}}$).
   - Guarantees identical sequence position across any server instance regardless of restarts.
2. **Zero Auto-Blank Policy**:
   - A short sequence (e.g. 60 seconds) loops indefinitely within the 5-hour cycle. Blanking is only permitted when explicitly added as a `BLANK` playlist item.
3. **Gorilla WebSocket Single-Writer Safety**:
   - Gorilla WebSocket connections prohibit concurrent writes. Each client has a dedicated `writePump` channel queue, preventing race conditions during mass synchronization broadcasts.
4. **Authoritative Sync Recovery**:
   - On reboot, the Go backend checks MongoDB for active sync events. If a sync is still valid based on its UTC expiration time, it resumes with the calculated remaining duration.
