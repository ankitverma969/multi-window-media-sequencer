import React, { useEffect, useState } from 'react'

export function ImagePlayer({
  src,
  name,
  mediaKey,
  durationSeconds = 10,
  elapsedSeconds = 0,
  onTransition,
  isSync = false,
}) {
  const [hasError, setHasError] = useState(false)
  const [progress, setProgress] = useState(0)

  useEffect(() => {
    setHasError(false)
    const totalMs = durationSeconds * 1000
    const initialElapsedMs = Math.max(0, elapsedSeconds * 1000)
    const remainingMs = Math.max(100, totalMs - initialElapsedMs)

    // Schedule exact transition when duration completes
    const transitionTimer = setTimeout(() => {
      if (onTransition) onTransition()
    }, remainingMs)

    // Update smooth progress bar
    const startTimestamp = Date.now() - initialElapsedMs
    const progressInterval = setInterval(() => {
      const now = Date.now()
      const currentElapsed = now - startTimestamp
      const pct = Math.min(100, (currentElapsed / totalMs) * 100)
      setProgress(pct)
    }, 100)

    return () => {
      clearTimeout(transitionTimer)
      clearInterval(progressInterval)
    }
  }, [src, durationSeconds, elapsedSeconds, onTransition])

  return (
    <div className="player-container image-player-container">
      {!hasError ? (
        <img
          src={src}
          alt={name || mediaKey}
          className="media-content image-content"
          onError={() => setHasError(true)}
        />
      ) : (
        <div className="player-overlay error-overlay">
          <span>⚠️ Failed to load image ({mediaKey})</span>
        </div>
      )}

      {/* Visual countdown progress bar */}
      <div className="image-progress-bar">
        <div className="image-progress-fill" style={{ width: `${progress}%` }}></div>
      </div>

      {isSync && (
        <div className="sync-watermark">
          <span>⚡ SYNC OVERRIDE</span>
        </div>
      )}
    </div>
  )
}
