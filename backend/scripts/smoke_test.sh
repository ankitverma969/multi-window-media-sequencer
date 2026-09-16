#!/usr/bin/env bash
# ==============================================================================
# Multi-Window Media Sequencer — API Smoke Test Script
# Usage: ./smoke_test.sh [BASE_URL]
# Example: ./smoke_test.sh http://localhost:8080
# ==============================================================================

BASE_URL="${1:-http://localhost:8080}"
echo "Running API smoke tests against: $BASE_URL"
echo "--------------------------------------------------"

# 1. Health Check
echo "1. Testing Health Check (GET /health)..."
curl -s -w "\nHTTP Status: %{http_code}\n\n" "$BASE_URL/health"

# 2. List Windows
echo "2. Listing Windows (GET /api/v1/windows)..."
curl -s -w "\nHTTP Status: %{http_code}\n\n" "$BASE_URL/api/v1/windows"

# 3. List Media Catalog
echo "3. Listing Media Catalog (GET /api/v1/media)..."
curl -s -w "\nHTTP Status: %{http_code}\n\n" "$BASE_URL/api/v1/media"

# 4. Get Window 1 Playlist
echo "4. Getting Window 1 Playlist (GET /api/v1/windows/1/playlist)..."
curl -s -w "\nHTTP Status: %{http_code}\n\n" "$BASE_URL/api/v1/windows/1/playlist"

# 5. Add Media Item to Window 1
echo "5. Adding Media M4 to Window 1 (POST /api/v1/windows/1/playlist/items)..."
curl -s -X POST "$BASE_URL/api/v1/windows/1/playlist/items" \
  -H "Content-Type: application/json" \
  -d '{"media_key": "M4", "custom_duration_seconds": 12}' \
  -w "\nHTTP Status: %{http_code}\n\n"

# 6. Get Updated Window 1 Playlist
echo "6. Getting Updated Window 1 Playlist (GET /api/v1/windows/1/playlist)..."
curl -s -w "\nHTTP Status: %{http_code}\n\n" "$BASE_URL/api/v1/windows/1/playlist"

# 7. Get Current Playback State of Window 1
echo "7. Getting Current Playback State (GET /api/v1/windows/1/playback)..."
curl -s -w "\nHTTP Status: %{http_code}\n\n" "$BASE_URL/api/v1/windows/1/playback"

# 8. Trigger Synchronized Override
echo "8. Triggering Synchronization for M2 (POST /api/v1/sync)..."
curl -s -X POST "$BASE_URL/api/v1/sync" \
  -H "Content-Type: application/json" \
  -d '{"media_key": "M2", "duration_seconds": 15, "lead_time_ms": 1000, "triggered_by": "smoke_test"}' \
  -w "\nHTTP Status: %{http_code}\n\n"

# 9. Get Active Synchronization State
echo "9. Getting Active Synchronization State (GET /api/v1/sync/current)..."
curl -s -w "\nHTTP Status: %{http_code}\n\n" "$BASE_URL/api/v1/sync/current"

echo "--------------------------------------------------"
echo "Smoke test execution completed."
