import React, { useState, useEffect } from 'react'
import { api } from '../api/client'
import { evaluateSyncTimeline } from '../playback/timeSync'

export function SyncControls({ mediaList = [], onSyncTriggered }) {
  const [selectedMediaKey, setSelectedMediaKey] = useState('')
  const [durationSeconds, setDurationSeconds] = useState(15)
  const leadTimeMs = 1000
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [currentSync, setCurrentSync] = useState(null)
  const [statusMessage, setStatusMessage] = useState(null)
  const [syncRemaining, setSyncRemaining] = useState(0)

  // Default selected media to first available
  useEffect(() => {
    if (mediaList.length > 0 && !selectedMediaKey) {
      setSelectedMediaKey(mediaList[0].media_key)
    }
  }, [mediaList, selectedMediaKey])

  // Poll current sync status from backend
  const checkCurrentSync = async () => {
    try {
      const active = await api.getCurrentSync()
      setCurrentSync(active)
    } catch {
      setCurrentSync(null)
    }
  }

  useEffect(() => {
    checkCurrentSync()
    const interval = setInterval(checkCurrentSync, 2000)
    return () => clearInterval(interval)
  }, [])

  // Update countdown timer for active sync
  useEffect(() => {
    if (!currentSync) {
      setSyncRemaining(0)
      return
    }

    const updateCountdown = () => {
      const evalState = evaluateSyncTimeline(currentSync.start_time, currentSync.end_time, 0)
      if (evalState.isActive) {
        setSyncRemaining(Math.ceil(evalState.remainingMs / 1000))
      } else {
        setSyncRemaining(0)
      }
    }

    updateCountdown()
    const timer = setInterval(updateCountdown, 500)
    return () => clearInterval(timer)
  }, [currentSync])

  const handleTriggerSync = async (e) => {
    e.preventDefault()
    if (!selectedMediaKey) return

    setIsSubmitting(true)
    setStatusMessage(null)

    try {
      const event = await api.triggerSync(selectedMediaKey, Number(durationSeconds), Number(leadTimeMs))
      setStatusMessage({ type: 'success', text: `Sync triggered: all windows displaying ${selectedMediaKey}!` })
      setCurrentSync(event)
      if (onSyncTriggered) onSyncTriggered(event)
    } catch (err) {
      setStatusMessage({ type: 'error', text: err.message || 'Failed to trigger synchronization' })
    } finally {
      setIsSubmitting(false)
    }
  }

  const handleCancelSync = async () => {
    if (!currentSync?.event_id) return
    setIsSubmitting(true)

    try {
      await api.cancelSync(currentSync.event_id)
      setStatusMessage({ type: 'info', text: 'Synchronization cancelled. Resuming normal sequences.' })
      setCurrentSync(null)
    } catch (err) {
      setStatusMessage({ type: 'error', text: err.message || 'Failed to cancel sync' })
    } finally {
      setIsSubmitting(false)
    }
  }

  const presets = [10, 15, 30, 60]

  return (
    <div className="sync-controls-panel">
      <div className="sync-panel-header">
        <h3>⚡ Real-Time Synchronization Override</h3>
        <span className="sync-description">
          Forces all windows to display a single media item simultaneously for a configured duration.
        </span>
      </div>

      {currentSync && syncRemaining > 0 && (
        <div className="active-sync-banner">
          <div className="sync-banner-info">
            <span className="badge-pulse">●</span>
            <strong>Active Sync: {currentSync.media_key}</strong>
            <span>({currentSync.media_snapshot?.name || currentSync.media_snapshot?.type})</span>
            <span className="countdown-text">Time remaining: {syncRemaining}s</span>
          </div>
          <button
            className="btn-cancel-sync"
            onClick={handleCancelSync}
            disabled={isSubmitting}
          >
            Cancel Sync
          </button>
        </div>
      )}

      <form className="sync-form" onSubmit={handleTriggerSync}>
        <div className="form-group">
          <label htmlFor="sync-media-select">Selected Media:</label>
          <select
            id="sync-media-select"
            value={selectedMediaKey}
            onChange={(e) => setSelectedMediaKey(e.target.value)}
            disabled={isSubmitting}
          >
            {mediaList.map((m) => (
              <option key={m.id || m.media_key} value={m.media_key}>
                {m.media_key} — {m.name} ({m.type}, {m.duration_seconds}s)
              </option>
            ))}
          </select>
        </div>

        <div className="form-group">
          <label htmlFor="sync-duration-input">Duration (Seconds):</label>
          <div className="duration-input-wrapper">
            <input
              id="sync-duration-input"
              type="number"
              min="1"
              max="3600"
              value={durationSeconds}
              onChange={(e) => setDurationSeconds(Math.max(1, parseInt(e.target.value) || 1))}
              disabled={isSubmitting}
            />
            <div className="duration-presets">
              {presets.map((p) => (
                <button
                  key={p}
                  type="button"
                  className={`btn-preset ${durationSeconds === p ? 'active' : ''}`}
                  onClick={() => setDurationSeconds(p)}
                >
                  {p}s
                </button>
              ))}
            </div>
          </div>
        </div>

        <button
          type="submit"
          className="btn-trigger-sync"
          disabled={isSubmitting || !selectedMediaKey}
        >
          {isSubmitting ? 'Triggering...' : '🚀 SYNC ALL WINDOWS'}
        </button>
      </form>

      {statusMessage && (
        <div className={`sync-status-alert alert-${statusMessage.type}`}>
          {statusMessage.text}
        </div>
      )}
    </div>
  )
}
