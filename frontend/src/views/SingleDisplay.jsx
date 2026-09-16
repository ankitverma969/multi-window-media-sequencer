import React, { useState, useEffect } from 'react'
import { api } from '../api/client'
import { MediaWindow } from '../components/MediaWindow'

export function SingleDisplay({ windowNumber, onBack }) {
  const [windowData, setWindowData] = useState(null)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState(null)

  useEffect(() => {
    const fetchWindow = async () => {
      setIsLoading(true)
      setError(null)
      try {
        const data = await api.getWindow(windowNumber)
        setWindowData(data)
      } catch (err) {
        setError(err.message || `Window ${windowNumber} not found`)
      } finally {
        setIsLoading(false)
      }
    }

    if (windowNumber) {
      fetchWindow()
    }
  }, [windowNumber])

  return (
    <div className="single-display-container">
      <header className="single-display-bar">
        <button className="btn-back-dashboard" onClick={onBack}>
          ⬅ Back to All Windows Grid
        </button>
        <span className="single-display-title">
          Standalone Presentation Display: Window {windowNumber}
        </span>
        <button
          className="btn-fullscreen"
          onClick={() => {
            if (!document.fullscreenElement) {
              document.documentElement.requestFullscreen().catch(() => {})
            } else {
              document.exitFullscreen().catch(() => {})
            }
          }}
        >
          ⛶ Toggle Fullscreen
        </button>
      </header>

      <main className="single-display-viewport">
        {isLoading ? (
          <div className="loading-state">
            <div className="spinner"></div>
            <span>Loading Window {windowNumber} configuration...</span>
          </div>
        ) : error ? (
          <div className="error-state">
            <h3>⚠️ Error Loading Window</h3>
            <p>{error}</p>
            <button className="btn-back-dashboard" onClick={onBack}>
              Return to Grid
            </button>
          </div>
        ) : windowData ? (
          <div className="single-window-wrapper">
            <MediaWindow windowData={windowData} isSelected={true} />
          </div>
        ) : null}
      </main>
    </div>
  )
}
