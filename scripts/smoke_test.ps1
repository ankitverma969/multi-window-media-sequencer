# ==============================================================================
# EVA Bharat Multi-Window Media Sequencer — Production Deployment Smoke Test
# ==============================================================================
param(
    [string]$BackendUrl = "http://localhost:8080",
    [string]$FrontendUrl = "http://localhost:5173"
)

$ErrorActionPreference = "Stop"
Write-Host "==================================================" -ForegroundColor Cyan
Write-Host "Starting EVA Bharat Deployment Smoke Test" -ForegroundColor Cyan
Write-Host "Backend URL:  $BackendUrl"
Write-Host "Frontend URL: $FrontendUrl"
Write-Host "==================================================" -ForegroundColor Cyan

$passed = 0
$failed = 0

function Assert-Step([string]$name, [scriptblock]$action) {
    Write-Host -NoNewline "[TEST] $name... "
    try {
        & $action
        Write-Host "PASS" -ForegroundColor Green
        $script:passed++
    } catch {
        Write-Host "FAIL: $_" -ForegroundColor Red
        $script:failed++
    }
}

# 1. Backend /health reachable
Assert-Step "1. Backend /health endpoint reachable" {
    $res = Invoke-RestMethod -Uri "$BackendUrl/health" -TimeoutSec 5
    if ($res.success -ne $true -or $res.data.status -ne "ok") {
        throw "Health status unexpected: $($res | ConvertTo-Json -Compress)"
    }
}

# 2. MongoDB connection healthy
Assert-Step "2. Persistent MongoDB connectivity verified" {
    $res = Invoke-RestMethod -Uri "$BackendUrl/health" -TimeoutSec 5
    if ($res.data.database -ne "connected") {
        throw "Database status is not connected: $($res.data.database)"
    }
}

# 3. Server time synchronization endpoint
Assert-Step "3. Authoritative server time endpoint responds" {
    $res = Invoke-RestMethod -Uri "$BackendUrl/api/v1/time" -TimeoutSec 5
    if (-not $res.data.server_time_utc -and -not $res.server_time) {
        throw "Missing server_time in response"
    }
}

# 4. Display Windows Catalog
Assert-Step "4. All 4 display windows retrieved from MongoDB" {
    $res = Invoke-RestMethod -Uri "$BackendUrl/api/v1/windows" -TimeoutSec 5
    if ($res.data.Count -lt 4) {
        throw "Expected at least 4 windows, found $($res.data.Count)"
    }
}

# 5. Media Catalog
Assert-Step "5. Media catalog contains assets" {
    $res = Invoke-RestMethod -Uri "$BackendUrl/api/v1/media" -TimeoutSec 5
    if ($res.data.Count -lt 5) {
        throw "Expected at least 5 media items, found $($res.data.Count)"
    }
}

# 6. Window 1 Playlist & 5-hour cycle engine
Assert-Step "6. Window 1 playlist and 5-hour cycle playback calculated" {
    $p = Invoke-RestMethod -Uri "$BackendUrl/api/v1/windows/1/playlist" -TimeoutSec 5
    if (-not $p.data.items -or $p.data.items.Count -eq 0) {
        throw "Window 1 playlist has no items"
    }
    $state = Invoke-RestMethod -Uri "$BackendUrl/api/v1/windows/1/playback" -TimeoutSec 5
    if (-not $state.data.current_item) {
        throw "5-hour timeline engine failed to return current item"
    }
}

# 7. Frontend UI reachable
Assert-Step "7. Frontend HTTP reachable and serving HTML" {
    $html = (Invoke-WebRequest -Uri $FrontendUrl -UseBasicParsing -TimeoutSec 5).Content
    if (-not $html.Contains("<div id=""root""></div>") -and -not $html.Contains("<!doctype html>")) {
        throw "Frontend did not serve valid React HTML entry point"
    }
}

# 8. Trigger Global Sync & Cancel
Assert-Step "8. Real-time synchronization trigger & cancellation" {
    $syncBody = @{
        media_key = "M2"
        duration_seconds = 15
        lead_time_ms = 500
    } | ConvertTo-Json
    $syncRes = Invoke-RestMethod -Uri "$BackendUrl/api/v1/sync" -Method POST -Body $syncBody -ContentType "application/json"
    if ($syncRes.success -ne $true -or -not $syncRes.data.event_id) {
        throw "Sync trigger failed"
    }
    $eventId = $syncRes.data.event_id

    # Verify active sync query
    $current = Invoke-RestMethod -Uri "$BackendUrl/api/v1/sync/current"
    if ($current.data.event_id -ne $eventId) {
        throw "Active sync did not register"
    }

    # Clean up sync immediately
    $cancelRes = Invoke-RestMethod -Uri "$BackendUrl/api/v1/sync/$eventId/cancel" -Method POST
    if ($cancelRes.success -ne $true) {
        throw "Sync cancellation failed"
    }
}

Write-Host "==================================================" -ForegroundColor Cyan
Write-Host "Smoke Test Summary: $passed Passed, $failed Failed" -ForegroundColor $(if ($failed -eq 0) { "Green" } else { "Red" })
Write-Host "==================================================" -ForegroundColor Cyan

if ($failed -gt 0) {
    exit 1
}
