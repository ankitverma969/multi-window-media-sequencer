# ==============================================================================
# Multi-Window Media Sequencer — PowerShell API Smoke Test Script
# Usage: .\smoke_test.ps1 [-BaseUrl "http://localhost:8080"]
#
# NOTE: This script cleans up all test data it creates.
#       The item added in Step 5 is deleted in Step 10 (cleanup).
# ==============================================================================

param(
    [string]$BaseUrl = "http://localhost:8080"
)

Write-Host "Running API smoke tests against: $BaseUrl" -ForegroundColor Cyan
Write-Host "--------------------------------------------------"

$Pass = 0
$Fail = 0

function Invoke-ApiTest {
    param(
        [string]$Step,
        [string]$Method,
        [string]$Endpoint,
        [string]$Body = $null
    )
    $url = "$BaseUrl$Endpoint"
    try {
        $headers = @{ "Content-Type" = "application/json" }
        if ($Body) {
            $resp = Invoke-RestMethod -Uri $url -Method $Method -Body $Body -Headers $headers
        } else {
            $resp = Invoke-RestMethod -Uri $url -Method $Method -Headers $headers
        }
        Write-Host "  PASS: $Step" -ForegroundColor Green
        $script:Pass++
        return $resp
    } catch {
        $statusCode = $_.Exception.Response.StatusCode.value__
        Write-Host "  FAIL: $Step - HTTP $statusCode - $($_.Exception.Message)" -ForegroundColor Red
        if ($_.ErrorDetails) {
            Write-Host "        $($_.ErrorDetails.Message)" -ForegroundColor DarkRed
        }
        $script:Fail++
        return $null
    }
}

# 1. Health
Invoke-ApiTest -Step "1. Health Check" -Method "GET" -Endpoint "/health" | Out-Null

# 2. List Windows
Invoke-ApiTest -Step "2. List Windows" -Method "GET" -Endpoint "/api/v1/windows" | Out-Null

# 3. List Media
Invoke-ApiTest -Step "3. List Media Catalog" -Method "GET" -Endpoint "/api/v1/media" | Out-Null

# 4. Get Window 1 Playlist
Invoke-ApiTest -Step "4. Get Window 1 Playlist" -Method "GET" -Endpoint "/api/v1/windows/1/playlist" | Out-Null

# 5. Add Media Item — capture response to extract item_id from data.items[] for cleanup
Write-Host "5. Adding Media M4 to Window 1 (POST /api/v1/windows/1/playlist/items)..." -ForegroundColor Yellow
$addPayload = '{"media_key": "M4", "custom_duration_seconds": 12}'
$addedItemId = $null
try {
    $headers = @{ "Content-Type" = "application/json" }
    $addResp = Invoke-RestMethod -Uri "$BaseUrl/api/v1/windows/1/playlist/items" `
        -Method "POST" -Body $addPayload -Headers $headers
    if ($addResp.success -eq $true) {
        Write-Host "  PASS: 5. Add Media Item" -ForegroundColor Green
        $Pass++

        # The API returns the full updated Playlist in data{}.
        # The newly added item's item_id is inside data.items[].item_id (last entry).
        $items = $addResp.data.items
        if ($items -and $items.Count -gt 0) {
            $addedItemId = $items[$items.Count - 1].item_id
            Write-Host "        Captured item_id for cleanup: $addedItemId" -ForegroundColor DarkGray
        } else {
            Write-Host "        WARNING: No items found in response; cleanup may be incomplete" -ForegroundColor Yellow
        }
    } else {
        Write-Host "  FAIL: 5. Add Media Item - success=false in response" -ForegroundColor Red
        $Fail++
    }
} catch {
    Write-Host "  FAIL: 5. Add Media Item - $($_.Exception.Message)" -ForegroundColor Red
    $Fail++
}

# 6. Get Updated Playlist
Invoke-ApiTest -Step "6. Get Updated Playlist" -Method "GET" -Endpoint "/api/v1/windows/1/playlist" | Out-Null

# 7. Get Playback State
Invoke-ApiTest -Step "7. Get Current Playback State" -Method "GET" -Endpoint "/api/v1/windows/1/playback" | Out-Null

# 8. Trigger Sync
$syncPayload = '{"media_key": "M2", "duration_seconds": 15, "lead_time_ms": 1000, "triggered_by": "powershell_smoke_test"}'
Invoke-ApiTest -Step "8. Trigger Synchronization" -Method "POST" -Endpoint "/api/v1/sync" -Body $syncPayload | Out-Null

# 9. Get Active Sync
Invoke-ApiTest -Step "9. Get Active Synchronization" -Method "GET" -Endpoint "/api/v1/sync/current" | Out-Null

# 10. CLEANUP — Delete the test item added in Step 5
Write-Host "10. Cleanup: Deleting test playlist item..." -ForegroundColor Yellow
if ($addedItemId) {
    try {
        $headers = @{ "Content-Type" = "application/json" }
        $delResp = Invoke-RestMethod -Uri "$BaseUrl/api/v1/windows/1/playlist/$addedItemId" `
            -Method "DELETE" -Headers $headers
        if ($delResp.success -eq $true) {
            Write-Host "  PASS: 10. Cleanup: Delete Test Item (item_id=$addedItemId)" -ForegroundColor Green
            $Pass++
        } else {
            Write-Host "  FAIL: 10. Cleanup: Delete Test Item returned success=false" -ForegroundColor Red
            $Fail++
        }
    } catch {
        Write-Host "  FAIL: 10. Cleanup: Delete Test Item - $($_.Exception.Message)" -ForegroundColor Red
        $Fail++
    }
} else {
    Write-Host "  SKIP: No item_id captured in step 5; manual cleanup may be needed." -ForegroundColor Yellow
}

Write-Host "--------------------------------------------------"
Write-Host "Results: $Pass passed, $Fail failed." -ForegroundColor Cyan
if ($Fail -eq 0) {
    Write-Host "All smoke tests PASSED." -ForegroundColor Green
    exit 0
} else {
    Write-Host "$Fail smoke test(s) FAILED." -ForegroundColor Red
    exit 1
}
