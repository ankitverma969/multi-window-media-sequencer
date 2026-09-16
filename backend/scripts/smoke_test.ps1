# ==============================================================================
# Multi-Window Media Sequencer — PowerShell API Smoke Test Script
# Usage: .\smoke_test.ps1 [-BaseUrl "http://localhost:8080"]
# ==============================================================================

param(
    [string]$BaseUrl = "http://localhost:8080"
)

Write-Host "Running API smoke tests against: $BaseUrl" -ForegroundColor Cyan
Write-Host "--------------------------------------------------"

function Invoke-ApiTest {
    param(
        [string]$Step,
        [string]$Method,
        [string]$Endpoint,
        [string]$Body = $null
    )
    Write-Host "$Step ($Method $Endpoint)..." -ForegroundColor Yellow
    $url = "$BaseUrl$Endpoint"
    try {
        $headers = @{ "Content-Type" = "application/json" }
        if ($Body) {
            $resp = Invoke-RestMethod -Uri $url -Method $Method -Body $Body -Headers $headers
        } else {
            $resp = Invoke-RestMethod -Uri $url -Method $Method -Headers $headers
        }
        Write-Host "Response:" -ForegroundColor Green
        $resp | ConvertTo-Json -Depth 5
    } catch {
        Write-Host "Error: $($_.Exception.Message)" -ForegroundColor Red
        if ($_.ErrorDetails) {
            Write-Host $_.ErrorDetails.Message -ForegroundColor DarkRed
        }
    }
    Write-Host ""
}

# 1. Health
Invoke-ApiTest -Step "1. Health Check" -Method "GET" -Endpoint "/health"

# 2. List Windows
Invoke-ApiTest -Step "2. List Windows" -Method "GET" -Endpoint "/api/v1/windows"

# 3. List Media
Invoke-ApiTest -Step "3. List Media Catalog" -Method "GET" -Endpoint "/api/v1/media"

# 4. Get Window 1 Playlist
Invoke-ApiTest -Step "4. Get Window 1 Playlist" -Method "GET" -Endpoint "/api/v1/windows/1/playlist"

# 5. Add Media Item
$addPayload = '{"media_key": "M4", "custom_duration_seconds": 12}'
Invoke-ApiTest -Step "5. Add Media M4 to Window 1" -Method "POST" -Endpoint "/api/v1/windows/1/playlist/items" -Body $addPayload

# 6. Get Updated Playlist
Invoke-ApiTest -Step "6. Get Updated Playlist" -Method "GET" -Endpoint "/api/v1/windows/1/playlist"

# 7. Get Playback State
Invoke-ApiTest -Step "7. Get Current Playback State" -Method "GET" -Endpoint "/api/v1/windows/1/playback"

# 8. Trigger Sync
$syncPayload = '{"media_key": "M2", "duration_seconds": 15, "lead_time_ms": 1000, "triggered_by": "powershell_test"}'
Invoke-ApiTest -Step "8. Trigger Synchronization" -Method "POST" -Endpoint "/api/v1/sync" -Body $syncPayload

# 9. Get Active Sync
Invoke-ApiTest -Step "9. Get Active Synchronization" -Method "GET" -Endpoint "/api/v1/sync/current"

Write-Host "--------------------------------------------------"
Write-Host "Smoke tests finished." -ForegroundColor Cyan
