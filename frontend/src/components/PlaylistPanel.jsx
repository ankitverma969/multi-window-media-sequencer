import React, { useState, useEffect, useCallback } from 'react'
import { api } from '../api/client'
import { formatTime } from '../playback/timeSync'

export function PlaylistPanel({
  windows = [],
  selectedWindowNumber = 1,
  onSelectWindow,
  mediaList = [],
}) {
  const [playlist, setPlaylist] = useState(null)
  const [isLoading, setIsLoading] = useState(false)
  const [selectedMediaToAdd, setSelectedMediaToAdd] = useState('')
  const [customDuration, setCustomDuration] = useState(30)
  const [isAdding, setIsAdding] = useState(false)
  const [errorMessage, setErrorMessage] = useState(null)

  const fetchPlaylist = useCallback(async (wNum) => {
    setIsLoading(true)
    setErrorMessage(null)
    try {
      const data = await api.getPlaylist(wNum)
      setPlaylist(data)
    } catch (err) {
      setErrorMessage(err.message)
    } finally {
      setIsLoading(false)
    }
  }, [])

  useEffect(() => {
    if (selectedWindowNumber) {
      fetchPlaylist(selectedWindowNumber)
    }
  }, [selectedWindowNumber, fetchPlaylist])

  // Update default duration when media selection changes
  useEffect(() => {
    if (mediaList.length > 0 && !selectedMediaToAdd) {
      setSelectedMediaToAdd(mediaList[0].media_key)
      setCustomDuration(mediaList[0].duration_seconds || 30)
    }
  }, [mediaList, selectedMediaToAdd])

  const handleMediaSelectionChange = (key) => {
    setSelectedMediaToAdd(key)
    const m = mediaList.find((item) => item.media_key === key)
    if (m) {
      setCustomDuration(m.duration_seconds || 30)
    }
  }

  const handleAddItem = async (e) => {
    e.preventDefault()
    if (!selectedMediaToAdd) return

    setIsAdding(true)
    setErrorMessage(null)

    try {
      const updated = await api.addPlaylistItem(selectedWindowNumber, selectedMediaToAdd, Number(customDuration))
      setPlaylist(updated)
    } catch (err) {
      setErrorMessage(err.message || 'Failed to add item to playlist')
    } finally {
      setIsAdding(false)
    }
  }

  const handleRemoveItem = async (itemId) => {
    if (!confirm('Remove this item from the window sequence?')) return

    try {
      const updated = await api.removePlaylistItem(selectedWindowNumber, itemId)
      setPlaylist(updated)
    } catch (err) {
      setErrorMessage(err.message || 'Failed to remove item')
    }
  }

  return (
    <div className="playlist-panel">
      <div className="playlist-panel-header">
        <div className="window-selector-group">
          <label htmlFor="window-select">Configure Window:</label>
          <select
            id="window-select"
            value={selectedWindowNumber}
            onChange={(e) => onSelectWindow && onSelectWindow(Number(e.target.value))}
          >
            {windows.map((w) => (
              <option key={w.window_number} value={w.window_number}>
                Window {w.window_number} ({w.name})
              </option>
            ))}
          </select>
        </div>

        <div className="playlist-summary-metrics">
          <span>Items: {playlist?.items?.length || 0}</span>
          <span>Cycle Sequence: {formatTime(playlist?.total_sequence_duration_seconds || 0)}</span>
          <button className="btn-refresh-playlist" onClick={() => fetchPlaylist(selectedWindowNumber)} title="Refresh">
            🔄
          </button>
        </div>
      </div>

      {errorMessage && <div className="panel-error-alert">{errorMessage}</div>}

      {/* Playlist Items List */}
      <div className="playlist-items-scroll">
        {isLoading ? (
          <div className="loading-state">Loading playlist items...</div>
        ) : playlist?.items && playlist.items.length > 0 ? (
          <table className="playlist-table">
            <thead>
              <tr>
                <th>#</th>
                <th>Media</th>
                <th>Type</th>
                <th>Duration</th>
                <th>Action</th>
              </tr>
            </thead>
            <tbody>
              {playlist.items.map((item, idx) => (
                <tr key={item.item_id || idx} className="playlist-row">
                  <td className="col-order">{item.order || idx + 1}</td>
                  <td className="col-media">
                    <strong>{item.media_key}</strong>
                  </td>
                  <td className="col-type">
                    <span className={`type-tag tag-${item.type}`}>{item.type}</span>
                  </td>
                  <td className="col-duration">{item.duration_seconds}s</td>
                  <td className="col-action">
                    <button
                      className="btn-remove-item"
                      onClick={() => handleRemoveItem(item.item_id)}
                      title="Remove from playlist"
                    >
                      ✕
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        ) : (
          <div className="empty-playlist-state">No items configured for Window {selectedWindowNumber}.</div>
        )}
      </div>

      {/* Add Media Form */}
      <form className="add-item-form" onSubmit={handleAddItem}>
        <h4>➕ Add Media to Window {selectedWindowNumber}</h4>
        <div className="add-form-row">
          <div className="form-group flex-2">
            <label htmlFor="add-media-select">Asset:</label>
            <select
              id="add-media-select"
              value={selectedMediaToAdd}
              onChange={(e) => handleMediaSelectionChange(e.target.value)}
              disabled={isAdding}
            >
              {mediaList.map((m) => (
                <option key={m.id || m.media_key} value={m.media_key}>
                  {m.media_key} — {m.name} ({m.type})
                </option>
              ))}
            </select>
          </div>

          <div className="form-group flex-1">
            <label htmlFor="add-duration-input">Duration (s):</label>
            <input
              id="add-duration-input"
              type="number"
              min="1"
              max="3600"
              value={customDuration}
              onChange={(e) => setCustomDuration(Math.max(1, parseInt(e.target.value) || 1))}
              disabled={isAdding}
            />
          </div>

          <div className="form-group flex-action">
            <label>&nbsp;</label>
            <button type="submit" className="btn-add-item" disabled={isAdding || !selectedMediaToAdd}>
              {isAdding ? 'Adding...' : 'Add to Sequence'}
            </button>
          </div>
        </div>
      </form>
    </div>
  )
}
