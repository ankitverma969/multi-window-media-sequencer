import React, { useEffect, useRef, useState } from 'react'

export function VideoPlayer({
  src,
  mediaKey,
  seekOffsetSeconds = 0,
  isMuted = true,
  onEnded,
  onError,
  isSync = false,
}) {
  const videoRef = useRef(null)
  const [isBuffering, setIsBuffering] = useState(true)
  const [autoplayBlocked, setAutoplayBlocked] = useState(false)
  const [hasError, setHasError] = useState(false)
  const hasSeekedRef = useRef(false)

  // Reset seek state when src or seekOffset changes significantly
  useEffect(() => {
    hasSeekedRef.current = false
    setHasError(false)
    setIsBuffering(true)
  }, [src])

  useEffect(() => {
    const video = videoRef.current
    if (!video) return

    video.muted = isMuted
  }, [isMuted])

  const handleLoadedMetadata = () => {
    const video = videoRef.current
    if (!video) return

    if (seekOffsetSeconds > 0 && !hasSeekedRef.current) {
      const duration = video.duration
      if (duration && duration > 0) {
        // If seek offset exceeds duration, loop within video duration
        video.currentTime = seekOffsetSeconds % duration
      } else {
        video.currentTime = seekOffsetSeconds
      }
      hasSeekedRef.current = true
    }

    attemptPlay()
  }

  const attemptPlay = () => {
    const video = videoRef.current
    if (!video) return

    video
      .play()
      .then(() => {
        setAutoplayBlocked(false)
        setIsBuffering(false)
      })
      .catch((err) => {
        console.warn('Autoplay prevented by browser:', err)
        setAutoplayBlocked(true)
        setIsBuffering(false)
      })
  }

  const handleManualPlay = () => {
    const video = videoRef.current
    if (video) {
      video.muted = true
      attemptPlay()
    }
  }

  return (
    <div className="player-container video-player-container">
      <video
        ref={videoRef}
        key={src}
        src={src}
        className="media-content"
        playsInline
        muted={isMuted}
        autoPlay
        preload="auto"
        onLoadedMetadata={handleLoadedMetadata}
        onCanPlay={() => setIsBuffering(false)}
        onWaiting={() => setIsBuffering(true)}
        onPlaying={() => {
          setIsBuffering(false)
          setAutoplayBlocked(false)
        }}
        onEnded={onEnded}
        onError={(e) => {
          console.error('Video playback error for', mediaKey, e)
          setHasError(true)
          setIsBuffering(false)
          if (onError) onError(e)
        }}
      />

      {isBuffering && !hasError && (
        <div className="player-overlay buffering-overlay">
          <div className="spinner"></div>
          <span>Buffering {mediaKey}...</span>
        </div>
      )}

      {autoplayBlocked && (
        <div className="player-overlay autoplay-overlay" onClick={handleManualPlay}>
          <button className="btn-unblock-play">▶ Click to Play</button>
        </div>
      )}

      {hasError && (
        <div className="player-overlay error-overlay">
          <span>⚠️ Video Playback Error ({mediaKey})</span>
        </div>
      )}

      {isSync && (
        <div className="sync-watermark">
          <span>⚡ SYNC OVERRIDE</span>
        </div>
      )}
    </div>
  )
}
