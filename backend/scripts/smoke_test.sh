#!/usr/bin/env bash
# ==============================================================================
# Multi-Window Media Sequencer — API Smoke Test Script
# Usage: ./smoke_test.sh [BASE_URL]
# Example: ./smoke_test.sh http://localhost:8080
#
# NOTE: This script cleans up all test data it creates.
#       The item added in Step 5 is deleted in Step 10 (cleanup).
# ==============================================================================

BASE_URL="${1:-http://localhost:8080}"
echo "Running API smoke tests against: $BASE_URL"
echo "--------------------------------------------------"

PASS=0
FAIL=0

check_status() {
  local step="$1"
  local expected="$2"
  local actual="$3"
  if [ "$actual" = "$expected" ]; then
    echo "  PASS: $step (HTTP $actual)"
    PASS=$((PASS + 1))
  else
    echo "  FAIL: $step - expected HTTP $expected, got HTTP $actual"
    FAIL=$((FAIL + 1))
  fi
}

# 1. Health Check
echo "1. Testing Health Check (GET /health)..."
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/health")
check_status "Health Check" "200" "$STATUS"

# 2. List Windows
echo "2. Listing Windows (GET /api/v1/windows)..."
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/api/v1/windows")
check_status "List Windows" "200" "$STATUS"

# 3. List Media Catalog
echo "3. Listing Media Catalog (GET /api/v1/media)..."
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/api/v1/media")
check_status "List Media Catalog" "200" "$STATUS"

# 4. Get Window 1 Playlist
echo "4. Getting Window 1 Playlist (GET /api/v1/windows/1/playlist)..."
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/api/v1/windows/1/playlist")
check_status "Get Window 1 Playlist" "200" "$STATUS"

# 5. Add Media Item to Window 1
# Capture the full response so we can extract item_id from data.items[] for cleanup.
echo "5. Adding Media M4 to Window 1 (POST /api/v1/windows/1/playlist/items)..."
ADD_RESPONSE=$(curl -s -X POST "$BASE_URL/api/v1/windows/1/playlist/items" \
  -H "Content-Type: application/json" \
  -d '{"media_key": "M4", "custom_duration_seconds": 12}')

if echo "$ADD_RESPONSE" | grep -q '"success":true'; then
  echo "  PASS: Add Media Item (HTTP 201)"
  PASS=$((PASS + 1))
  # The API returns the full updated Playlist in data{}.
  # The new item's item_id is inside data.items[].item_id (last entry).
  if command -v python3 &>/dev/null; then
    ITEM_ID=$(echo "$ADD_RESPONSE" | python3 -c "
import sys, json
try:
    resp = json.load(sys.stdin)
    items = resp.get('data', {}).get('items', [])
    if items:
        print(items[-1]['item_id'])
except Exception:
    pass
" 2>/dev/null)
  else
    # Fallback without python: grab last occurrence of item_id in the JSON
    ITEM_ID=$(echo "$ADD_RESPONSE" | grep -oE '"item_id":"[^"]+"' | tail -1 | grep -oE '"[^"]+"$' | tr -d '"')
  fi
  echo "     Extracted item_id for cleanup: ${ITEM_ID:-<not captured>}"
else
  echo "  FAIL: Add Media Item - unexpected response: $ADD_RESPONSE"
  FAIL=$((FAIL + 1))
  ITEM_ID=""
fi

# 6. Get Updated Window 1 Playlist
echo "6. Getting Updated Window 1 Playlist (GET /api/v1/windows/1/playlist)..."
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/api/v1/windows/1/playlist")
check_status "Get Updated Playlist" "200" "$STATUS"

# 7. Get Current Playback State of Window 1
echo "7. Getting Current Playback State (GET /api/v1/windows/1/playback)..."
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/api/v1/windows/1/playback")
check_status "Get Playback State" "200" "$STATUS"

# 8. Trigger Synchronized Override
echo "8. Triggering Synchronization for M2 (POST /api/v1/sync)..."
SYNC_RESPONSE=$(curl -s -X POST "$BASE_URL/api/v1/sync" \
  -H "Content-Type: application/json" \
  -d '{"media_key": "M2", "duration_seconds": 15, "lead_time_ms": 1000, "triggered_by": "smoke_test"}')

if echo "$SYNC_RESPONSE" | grep -q '"success":true'; then
  echo "  PASS: Trigger Sync (HTTP 201)"
  PASS=$((PASS + 1))
else
  echo "  FAIL: Trigger Sync - unexpected response: $SYNC_RESPONSE"
  FAIL=$((FAIL + 1))
fi

# 9. Get Active Synchronization State
echo "9. Getting Active Synchronization State (GET /api/v1/sync/current)..."
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/api/v1/sync/current")
check_status "Get Active Sync" "200" "$STATUS"

# 10. CLEANUP — Delete the test item added in Step 5
echo "10. Cleanup: Deleting test playlist item..."
if [ -n "$ITEM_ID" ]; then
  DEL_STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X DELETE "$BASE_URL/api/v1/windows/1/playlist/$ITEM_ID")
  check_status "Cleanup: Delete Test Item" "200" "$DEL_STATUS"
else
  echo "  SKIP: No item_id captured in step 5; manual cleanup may be needed."
fi

echo "--------------------------------------------------"
echo "Results: $PASS passed, $FAIL failed."
if [ "$FAIL" -eq 0 ]; then
  echo "All smoke tests PASSED."
  exit 0
else
  echo "$FAIL smoke test(s) FAILED."
  exit 1
fi
