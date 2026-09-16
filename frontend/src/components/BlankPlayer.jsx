import React, { useEffect } from 'react'

export function BlankPlayer({
  isConfiguredBlank = false,
  durationSeconds = 10,
  elapsedSeconds = 0,
  onTransition,
}) {
  useEffect(() => {
    if (!isConfiguredBlank || !durationSeconds) return

    const remainingMs = Math.max(100, (durationSeconds - elapsedSeconds) * 1000)
    const timer = setTimeout(() => {
      if (onTransition) onTransition()
    }, remainingMs)

    return () => clearTimeout(timer)
  }, [isConfiguredBlank, durationSeconds, elapsedSeconds, onTransition])

  return (
    <div className="player-container blank-player-container">
      <div className="blank-content">
        <span className="blank-icon">⬛</span>
        <span className="blank-label">
          {isConfiguredBlank
            ? `Explicit Blank Sequence (${durationSeconds}s)`
            : 'No Playlist Configured — Window Idle'}
        </span>
      </div>
    </div>
  )
}
