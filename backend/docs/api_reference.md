# Multi-Window Media Sequencer — REST API Reference

All API requests and responses use standard JSON encoding. Responses follow a unified envelope:

```json
{
  "success": true,
  "data": { ... },
  "error": null,
  "server_time": "2026-09-16T14:30:00.000Z"
}
```

In the event of an error, `success` is `false`, `data` is `null`, and `error` contains machine-readable error codes:

```json
{
  "success": false,
  "data": null,
  "error": {
    "code": "WINDOW_NOT_FOUND",
    "message": "Window not found"
  },
  "server_time": "2026-09-16T14:30:00.000Z"
}
```

---

## Endpoint Summary

| Category | Method | Path | Description | Success Status |
| :--- | :--- | :--- | :--- | :--- |
| **System** | `GET` | `/health`, `/api/v1/health` | Check service & database health | 200 / 503 |
| **System** | `GET` | `/api/v1/time` | High-precision server UTC timestamp for clock sync | 200 |
| **Windows** | `GET` | `/api/v1/windows` | List all display windows | 200 |
| **Windows** | `GET` | `/api/v1/windows/{id}` | Get single window by number or ObjectID | 200 |
| **Media** | `GET` | `/api/v1/media` | List all available media assets | 200 |
| **Media** | `POST` | `/api/v1/media` | Register a new media asset | 201 |
| **Media** | `GET` | `/api/v1/media/{id}` | Get single media by key or ObjectID | 200 |
| **Playlists** | `GET` | `/api/v1/windows/{id}/playlist` | Get configured playlist for a window | 200 |
| **Playlists** | `POST` | `/api/v1/windows/{id}/playlist` | Dynamically append media item to window | 201 |
| **Playlists** | `PUT` | `/api/v1/windows/{id}/playlist` | Batch update/reorder playlist items | 200 |
| **Playlists** | `DELETE` | `/api/v1/windows/{id}/playlist/{itemId}` | Remove item from window playlist | 200 |
| **Playback** | `GET` | `/api/v1/windows/{id}/playback` | Current deterministic 5-hour playback state | 200 |
| **Sync** | `POST` | `/api/v1/sync` | Trigger global sync override for media item | 201 |
| **Sync** | `GET` | `/api/v1/sync/current` | Inspect currently active synchronization | 200 |
| **Sync** | `GET` | `/api/v1/sync/{event_id}` | Inspect sync event by event_id | 200 |
| **Sync** | `POST` | `/api/v1/sync/{event_id}/cancel` | Cancel an active or scheduled sync override | 200 |

---

## Detailed Endpoint Contracts

### 1. Health Check
* **Endpoint:** `GET /health` or `GET /api/v1/health`
* **Response (200 OK):**
```json
{
  "success": true,
  "data": {
    "status": "ok",
    "database": "connected",
    "environment": "development"
  },
  "error": null,
  "server_time": "2026-09-16T14:30:00.000Z"
}
```
* **Failure (503 Service Unavailable):** Returned if MongoDB ping fails.

---

### 2. High-Precision Time Synchronization
* **Endpoint:** `GET /api/v1/time`
* **Response (200 OK):**
```json
{
  "success": true,
  "data": {
    "server_time_utc": "2026-09-16T14:30:00.123456789Z",
    "server_time_ms": 1789569000123
  },
  "error": null,
  "server_time": "2026-09-16T14:30:00.123456789Z"
}
```

---

### 3. Display Windows
* **List Windows:** `GET /api/v1/windows`
* **Get Single Window:** `GET /api/v1/windows/{id}` (`{id}` accepts window number e.g. `1` or ObjectID hex)
* **Response (200 OK):**
```json
{
  "success": true,
  "data": {
    "id": "650000000000000000000010",
    "window_number": 1,
    "name": "Display Window 1 (Front)",
    "cycle_duration_seconds": 18000,
    "cycle_start_time": "2026-09-16T00:00:00Z",
    "is_active": true,
    "created_at": "2026-09-16T10:00:00Z",
    "updated_at": "2026-09-16T10:00:00Z"
  }
}
```

---

### 4. Dynamic Playlist Management

#### Get Playlist
* **Endpoint:** `GET /api/v1/windows/{id}/playlist`
* **Response (200 OK):**
```json
{
  "success": true,
  "data": {
    "id": "650000000000000000000020",
    "window_number": 1,
    "items": [
      {
        "item_id": "b18b4e47-e3e9-46be-91c6-11b33bfaea8c",
        "media_key": "M1",
        "type": "video",
        "url": "https://commondatastorage.googleapis.com/gtv-videos-bucket/sample/BigBuckBunny.mp4",
        "duration_seconds": 15,
        "order": 1
      },
      {
        "item_id": "d03e9f4a-7182-4ca3-bfa2-8b4317fbbcae",
        "media_key": "M2",
        "type": "image",
        "url": "https://images.unsplash.com/photo-1579546929518-9e396f3cc809?w=1200",
        "duration_seconds": 10,
        "order": 2
      }
    ],
    "total_sequence_duration_seconds": 25,
    "version": 2,
    "updated_at": "2026-09-16T14:31:00Z"
  }
}
```

#### Add Item to Playlist
* **Endpoint:** `POST /api/v1/windows/{id}/playlist` or `POST /api/v1/windows/{id}/playlist/items`
* **Request Body:**
```json
{
  "media_key": "M4",
  "custom_duration_seconds": 12
}
```
* **Response (201 Created):** Returns updated playlist with incremented version.

#### Batch Update / Reorder Playlist
* **Endpoint:** `PUT /api/v1/windows/{id}/playlist`
* **Request Body:**
```json
{
  "items": [
    {
      "media_key": "M2",
      "type": "image",
      "url": "https://example.com/m2.jpg",
      "duration_seconds": 10
    },
    {
      "media_key": "M1",
      "type": "video",
      "url": "https://example.com/m1.mp4",
      "duration_seconds": 15
    }
  ]
}
```
* **Response (200 OK):** Returns reordered playlist.

#### Delete Item from Playlist
* **Endpoint:** `DELETE /api/v1/windows/{id}/playlist/{itemId}`
* **Response (200 OK):** Returns playlist with item removed and sequence order recalculated.

---

### 5. Current Playback State (5-Hour Cycle Engine)
* **Endpoint:** `GET /api/v1/windows/{id}/playback`
* **Query Parameters:** `?time=2026-09-16T14:35:00Z` (Optional RFC3339 UTC timestamp, defaults to server now)
* **Response (200 OK):**
```json
{
  "success": true,
  "data": {
    "window_number": 1,
    "status": "NORMAL",
    "media_key": "M1",
    "media_type": "video",
    "media_url": "https://commondatastorage.googleapis.com/gtv-videos-bucket/sample/BigBuckBunny.mp4",
    "item_duration": 15000000000,
    "playback_position": 7000000000,
    "time_remaining": 8000000000,
    "cycle_duration": 18000000000000,
    "cycle_position": 523000000000,
    "cycle_number": 0,
    "sequence_iteration": 34,
    "sequence_duration": 25000000000,
    "evaluated_at": "2026-09-16T14:35:00Z"
  }
}
```

---

### 6. Synchronized Override API

#### Trigger Synchronization
* **Endpoint:** `POST /api/v1/sync`
* **Request Body:**
```json
{
  "media_key": "M2",
  "duration_seconds": 15,
  "lead_time_ms": 1000,
  "triggered_by": "admin_dashboard"
}
```
* **Response (201 Created):**
```json
{
  "success": true,
  "data": {
    "event_id": "sync_9f2a4bc1",
    "media_key": "M2",
    "media_snapshot": {
      "media_key": "M2",
      "name": "Synchronized Promo Banner (M2)",
      "type": "image",
      "url": "https://images.unsplash.com/photo-1579546929518-9e396f3cc809?w=1200"
    },
    "duration_seconds": 15,
    "start_time": "2026-09-16T14:35:01Z",
    "end_time": "2026-09-16T14:35:16Z",
    "status": "SCHEDULED",
    "triggered_by": "admin_dashboard",
    "created_at": "2026-09-16T14:35:00Z"
  }
}
```

#### Get Active Synchronization
* **Endpoint:** `GET /api/v1/sync/current`
* **Response (200 OK):** Returns active sync event or `null` if normal playback is running.

#### Cancel Synchronization
* **Endpoint:** `POST /api/v1/sync/{event_id}/cancel`
* **Response (200 OK):**
```json
{
  "success": true,
  "data": {
    "event_id": "sync_9f2a4bc1",
    "status": "CANCELLED"
  }
}
```
