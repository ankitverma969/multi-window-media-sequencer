/**
 * Centralized REST API client for the EVA Bharat Media Sequencer backend.
 */

const BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080'

class ApiError extends Error {
  constructor(message, status, code) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
  }
}

async function request(endpoint, options = {}) {
  const url = `${BASE_URL}${endpoint}`
  const headers = {
    'Content-Type': 'application/json',
    ...(options.headers || {}),
  }

  try {
    const response = await fetch(url, { ...options, headers })
    const data = await response.json().catch(() => null)

    if (!response.ok) {
      const errorMessage = data?.error?.message || data?.error || data?.message || `HTTP ${response.status} error`
      const errorCode = data?.error?.code || data?.code || 'UNKNOWN_ERROR'
      throw new ApiError(errorMessage, response.status, errorCode)
    }

    // Unwrap APIResponse envelope if present
    if (data && typeof data === 'object' && data.success === true && 'data' in data) {
      return data.data
    }

    return data
  } catch (err) {
    if (err instanceof ApiError) throw err
    throw new ApiError(err.message || 'Network request failed', 0, 'NETWORK_ERROR')
  }
}

export const api = {
  // Windows
  async getWindows() {
    return request('/api/v1/windows')
  },

  async getWindow(idOrNumber) {
    return request(`/api/v1/windows/${idOrNumber}`)
  },

  // Media catalog
  async getMediaList() {
    return request('/api/v1/media')
  },

  // Playlists
  async getPlaylist(windowNumber) {
    return request(`/api/v1/windows/${windowNumber}/playlist`)
  },

  async addPlaylistItem(windowNumber, mediaKey, durationSeconds) {
    return request(`/api/v1/windows/${windowNumber}/playlist`, {
      method: 'POST',
      body: JSON.stringify({
        media_key: mediaKey,
        custom_duration_seconds: durationSeconds || 0,
      }),
    })
  },

  async removePlaylistItem(windowNumber, itemId) {
    return request(`/api/v1/windows/${windowNumber}/playlist/${itemId}`, {
      method: 'DELETE',
    })
  },

  // Authoritative Playback state
  async getPlaybackState(windowNumber) {
    return request(`/api/v1/windows/${windowNumber}/playback`)
  },

  // Synchronization
  async triggerSync(mediaKey, durationSeconds, leadTimeMs = 1000) {
    return request('/api/v1/sync', {
      method: 'POST',
      body: JSON.stringify({
        media_key: mediaKey,
        duration_seconds: durationSeconds,
        lead_time_ms: leadTimeMs,
      }),
    })
  },

  async getCurrentSync() {
    return request('/api/v1/sync/current')
  },

  async cancelSync(syncId) {
    return request(`/api/v1/sync/${syncId}/cancel`, {
      method: 'POST',
    })
  },

  // Time Sync
  async getServerTime() {
    return request('/api/v1/time')
  },
}

export { ApiError }
