import React, { useState, useEffect, useCallback, useMemo } from 'react'
import { useWindowWebSocket } from '../websocket/useWindowWebSocket'
import { evaluateSyncTimeline, formatTime } from '../playback/timeSync'
import { StatusBadge } from './StatusBadge'
import { VideoPlayer } from './VideoPlayer'
import { ImagePlayer } from './ImagePlayer'
import { BlankPlayer } from './BlankPlayer'
import { api } from '../api/client'

export function MediaWindow({ windowData, isSelected = false, onSelect }) {
  const windowNumber = windowData.window_number
  const [isMuted, setIsMuted] = useState(true)

  // Real-time WebSocket hook for this window
  const {
    connectionStatus,
    playlist,
    normalPlayback,
    activeSync,
    serverOffsetMs,
    lastError,
  } = useWindowWebSocket(windowNumber)

  // Local fallback playback state if socket initial snapshot is still loading
  const [localPlayback, setLocalPlayback] = useState(null)

  // Sync timeline evaluation
  const [syncState, setSyncState] = useState({ isActive: false, isScheduled: false, offsetMs: 0, remainingMs: 0 })

  // Regularly re-evaluate sync timeline against server clock
  useEffect(() => {
    if (!activeSync) {
      setSyncState({ isActive: false, isScheduled: false, offsetMs: 0, remainingMs: 0 })
      return
    }

    const updateSync = () => {
      const evaluation = evaluateSyncTimeline(activeSync.start_time, activeSync.end_time, serverOffsetMs)
      setSyncState(evaluation)
    }

    updateSync()
    const timer = setInterval(updateSync, 200)
    return () => clearInterval(timer)
  }, [activeSync, serverOffsetMs])

  // Refresh normal playback state from backend when an item finishes
  const refreshPlayback = useCallback(async () => {
    try {
      const state = await api.getPlaybackState(windowNumber)
      setLocalPlayback(state)
    } catch (err) {
      console.warn(`Failed to refresh playback state for window ${windowNumber}`, err)
    }
  }, [windowNumber])

  // Current authoritative playback object (from websocket or REST fallback)
  const currentNormal = normalPlayback || localPlayback

  // Determine current active item to render
  const renderItem = useMemo(() => {
    // 1. Temporary Synchronization Override takes precedence when active
    if (syncState.isActive && activeSync?.media) {
      const media = activeSync.media
      return {
        isSync: true,
        mediaKey: media.media_key || activeSync.media_key,
        name: media.name || media.media_key,
        type: media.type,
        url: media.url,
        durationSeconds: activeSync.duration_seconds || 15,
        seekOffsetSeconds: Math.max(0, syncState.offsetMs / 1000),
      }
    }

    // 2. Normal playback sequence item
    if (currentNormal?.current_item) {
      const it = currentNormal.current_item
      const posSec = currentNormal.playback_position ? currentNormal.playback_position / 1e9 : 0
      return {
        isSync: false,
        mediaKey: it.media_key,
        name: it.media_key,
        type: it.type,
        url: it.url,
        durationSeconds: it.duration_seconds,
        seekOffsetSeconds: posSec,
      }
    }

    // 3. Fallback to first playlist item if normal playback not yet evaluated
    if (playlist?.items && playlist.items.length > 0) {
      const it = playlist.items[0]
      return {
        isSync: false,
        mediaKey: it.media_key,
        name: it.media_key,
        type: it.type,
        url: it.url,
        durationSeconds: it.duration_seconds,
        seekOffsetSeconds: 0,
      }
    }

    // 4. Blank / Empty
    return null
  }, [syncState, activeSync, currentNormal, playlist])

  return (
    <div
      className={`media-window-card ${isSelected ? 'selected' : ''} ${syncState.isActive ? 'sync-active-card' : ''}`}
      onClick={() => onSelect && onSelect(windowNumber)}
    >
      {/* Header */}
      <div className="window-header">
        <div className="window-identity">
          <span className="window-pill">Window {windowNumber}</span>
          <span className="window-name">{windowData.name}</span>
        </div>

        <div className="window-controls">
          <StatusBadge status={syncState} type="sync" />
          <StatusBadge status={connectionStatus} type="connection" />
          <button
            className={`btn-mute-toggle ${!isMuted ? 'unmuted' : ''}`}
            onClick={(e) => {
              e.stopPropagation()
              setIsMuted(!isMuted)
            }}
            title={isMuted ? 'Click to Unmute' : 'Click to Mute'}
          >
            {isMuted ? '🔇 Muted' : '🔊 Audio On'}
          </button>
        </div>
      </div>

      {/* Media Viewport */}
      <div className="window-viewport">
        {renderItem ? (
          renderItem.type === 'video' ? (
            <VideoPlayer
              key={renderItem.url}
              src={renderItem.url}
              mediaKey={renderItem.mediaKey}
              seekOffsetSeconds={renderItem.seekOffsetSeconds}
              isMuted={isMuted}
              isSync={renderItem.isSync}
              onEnded={refreshPlayback}
              onError={refreshPlayback}
            />
          ) : renderItem.type === 'image' ? (
            <ImagePlayer
              key={renderItem.url}
              src={renderItem.url}
              name={renderItem.name}
              mediaKey={renderItem.mediaKey}
              durationSeconds={renderItem.durationSeconds}
              elapsedSeconds={renderItem.seekOffsetSeconds}
              isSync={renderItem.isSync}
              onTransition={refreshPlayback}
            />
          ) : (
            <BlankPlayer
              isConfiguredBlank={true}
              durationSeconds={renderItem.durationSeconds}
              onTransition={refreshPlayback}
            />
          )
        ) : (
          <BlankPlayer isConfiguredBlank={false} />
        )}
      </div>

      {/* Footer Info */}
      <div className="window-footer">
        <div className="footer-item-info">
          <span className="item-label">Now Playing:</span>
          <span className="item-value">
            {renderItem ? `${renderItem.mediaKey} (${renderItem.type})` : 'Empty Sequence'}
          </span>
        </div>

        <div className="footer-sequence-info">
          {syncState.isActive ? (
            <span className="sync-countdown">
              Ends in: {formatTime(syncState.remainingMs / 1000)}
            </span>
          ) : (
            <span className="playlist-summary">
              Sequence: {playlist?.items?.length || 0} items ({formatTime(playlist?.total_sequence_duration_seconds || 0)})
            </span>
          )}
        </div>
      </div>

      {lastError && (
        <div className="window-error-banner">
          <span>⚠️ {lastError}</span>
        </div>
      )}
    </div>
  )
}
