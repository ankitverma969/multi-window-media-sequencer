import React, { useState, useEffect } from 'react'
import { api } from '../api/client'
import { MediaWindow } from '../components/MediaWindow'
import { SyncControls } from '../components/SyncControls'
import { PlaylistPanel } from '../components/PlaylistPanel'

export function GridDashboard({ onNavigateDisplay }) {
  const [windows, setWindows] = useState([])
  const [mediaList, setMediaList] = useState([])
  const [selectedWindowNumber, setSelectedWindowNumber] = useState(1)
  const [activeTab, setActiveTab] = useState('sync') // 'sync' | 'playlist'
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState(null)

  const loadData = async () => {
    setIsLoading(true)
    setError(null)
    try {
      const [windowsData, mediaData] = await Promise.all([
        api.getWindows(),
        api.getMediaList(),
      ])
      setWindows(Array.isArray(windowsData) ? windowsData : [])
      setMediaList(Array.isArray(mediaData) ? mediaData : [])
      if (Array.isArray(windowsData) && windowsData.length > 0 && !selectedWindowNumber) {
        setSelectedWindowNumber(windowsData[0].window_number)
      }
    } catch (err) {
      console.error('Failed to load dashboard data:', err)
      setError(err.message || 'Failed to connect to media sequencer backend.')
    } finally {
      setIsLoading(false)
    }
  }

  useEffect(() => {
    loadData()
  }, [])

  return (
    <div className="dashboard-container">
      {/* Top Navigation Bar */}
      <header className="dashboard-header">
        <div className="header-brand">
          <span className="brand-logo">🎬</span>
          <div>
            <h1>EVA Bharat Media Sequencer</h1>
            <p className="brand-subtitle">Server-Authoritative Multi-Window Synchronized Playback</p>
          </div>
        </div>

        <div className="header-actions">
          <div className="display-links">
            <span className="links-label">Dedicated Display Links:</span>
            {windows.map((w) => (
              <button
                key={w.window_number}
                className="btn-display-link"
                onClick={() => onNavigateDisplay(w.window_number)}
                title={`Open full-screen display for Window ${w.window_number}`}
              >
                📺 W{w.window_number}
              </button>
            ))}
          </div>

          <button className="btn-refresh" onClick={loadData} title="Reload initial data">
            🔄 Reload
          </button>
        </div>
      </header>

      {error && (
        <div className="dashboard-error-banner">
          <span>⚠️ {error}</span>
          <button onClick={loadData}>Retry</button>
        </div>
      )}

      {/* Main Multi-Window Grid (2x2) */}
      <main className="windows-grid-section">
        {isLoading && windows.length === 0 ? (
          <div className="dashboard-loading-state">
            <div className="spinner"></div>
            <span>Connecting to Go backend & loading windows...</span>
          </div>
        ) : (
          <div className="windows-grid">
            {windows.map((w) => (
              <MediaWindow
                key={w.window_number}
                windowData={w}
                isSelected={selectedWindowNumber === w.window_number}
                onSelect={(num) => {
                  setSelectedWindowNumber(num)
                  setActiveTab('playlist')
                }}
              />
            ))}
          </div>
        )}
      </main>

      {/* Control Console */}
      <section className="control-console-section">
        <div className="console-tab-header">
          <button
            className={`tab-btn ${activeTab === 'sync' ? 'active' : ''}`}
            onClick={() => setActiveTab('sync')}
          >
            ⚡ Real-Time Synchronization
          </button>
          <button
            className={`tab-btn ${activeTab === 'playlist' ? 'active' : ''}`}
            onClick={() => setActiveTab('playlist')}
          >
            📋 Playlist Manager (Window {selectedWindowNumber})
          </button>
        </div>

        <div className="console-tab-content">
          {activeTab === 'sync' ? (
            <SyncControls mediaList={mediaList} />
          ) : (
            <PlaylistPanel
              windows={windows}
              selectedWindowNumber={selectedWindowNumber}
              onSelectWindow={(num) => setSelectedWindowNumber(num)}
              mediaList={mediaList}
            />
          )}
        </div>
      </section>
    </div>
  )
}
