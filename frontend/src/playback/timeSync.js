/**
 * Time synchronization and offset utilities for server-authoritative playback.
 */

/**
 * Calculates clock skew offset between server and browser.
 * serverOffsetMs = serverTime - browserTime
 */
export function calculateServerOffset(serverTimeString) {
  if (!serverTimeString) return 0
  const serverMs = new Date(serverTimeString).getTime()
  if (isNaN(serverMs)) return 0
  return serverMs - Date.now()
}

/**
 * Returns estimated current server time in epoch milliseconds.
 */
export function getEstimatedServerTimeMs(serverOffsetMs = 0) {
  return Date.now() + serverOffsetMs
}

/**
 * Evaluates active sync timeline relative to estimated server clock.
 */
export function evaluateSyncTimeline(startTimeStr, endTimeStr, serverOffsetMs = 0) {
  if (!startTimeStr || !endTimeStr) {
    return { isScheduled: false, isActive: false, isExpired: true, offsetMs: 0, timeUntilStartMs: 0, remainingMs: 0 }
  }

  const startMs = new Date(startTimeStr).getTime()
  const endMs = new Date(endTimeStr).getTime()
  const nowMs = getEstimatedServerTimeMs(serverOffsetMs)

  if (isNaN(startMs) || isNaN(endMs)) {
    return { isScheduled: false, isActive: false, isExpired: true, offsetMs: 0, timeUntilStartMs: 0, remainingMs: 0 }
  }

  if (nowMs < startMs) {
    // Scheduled in future (preparation window)
    return {
      isScheduled: true,
      isActive: false,
      isExpired: false,
      timeUntilStartMs: startMs - nowMs,
      offsetMs: 0,
      remainingMs: endMs - nowMs,
    }
  }

  if (nowMs >= startMs && nowMs < endMs) {
    // Currently active
    return {
      isScheduled: false,
      isActive: true,
      isExpired: false,
      timeUntilStartMs: 0,
      offsetMs: nowMs - startMs,
      remainingMs: endMs - nowMs,
    }
  }

  // Already ended
  return {
    isScheduled: false,
    isActive: false,
    isExpired: true,
    timeUntilStartMs: 0,
    offsetMs: 0,
    remainingMs: 0,
  }
}

/**
 * Calculates current playback seek position (in seconds) within a media asset,
 * looping continuously if total elapsed time exceeds media item duration.
 */
export function calculateMediaOffsetSeconds(elapsedMs, mediaDurationSeconds) {
  if (!mediaDurationSeconds || mediaDurationSeconds <= 0) return 0
  const elapsedSec = Math.max(0, elapsedMs / 1000)
  return elapsedSec % mediaDurationSeconds
}

/**
 * Formats seconds into MM:SS string.
 */
export function formatTime(seconds) {
  if (isNaN(seconds) || seconds < 0) return '00:00'
  const total = Math.floor(seconds)
  const mins = Math.floor(total / 60)
  const secs = total % 60
  return `${String(mins).padStart(2, '0')}:${String(secs).padStart(2, '0')}`
}
