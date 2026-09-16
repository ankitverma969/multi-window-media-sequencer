/**
 * Playback State Machine constants and definitions.
 */

export const PlaybackStates = {
  IDLE: 'IDLE',
  LOADING: 'LOADING',
  PLAYING: 'PLAYING',
  SYNC_PREPARING: 'SYNC_PREPARING',
  SYNC_PLAYING: 'SYNC_PLAYING',
  BLANK: 'BLANK',
  ERROR: 'ERROR',
}

export const ConnectionStates = {
  CONNECTING: 'CONNECTING',
  CONNECTED: 'CONNECTED',
  RECONNECTING: 'RECONNECTING',
  DISCONNECTED: 'DISCONNECTED',
}
