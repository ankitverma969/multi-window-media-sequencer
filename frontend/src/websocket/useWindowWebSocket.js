import { useEffect, useRef, useState, useCallback } from 'react'
import { calculateServerOffset } from '../playback/timeSync'
import { ConnectionStates } from '../playback/playbackMachine'

function getWsBaseUrl() {
  if (import.meta.env.VITE_WS_URL !== undefined && import.meta.env.VITE_WS_URL !== '') {
    return import.meta.env.VITE_WS_URL
  }
  if (typeof window !== 'undefined' && window.location) {
    if (window.location.port === '5173') {
      return 'ws://localhost:8080'
    }
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    return `${protocol}//${window.location.host}`
  }
  return 'ws://localhost:8080'
}

const WS_BASE = getWsBaseUrl()

/**
 * Custom hook managing WebSocket connection for a specific display window.
 */
export function useWindowWebSocket(windowNumber) {
  const [connectionStatus, setConnectionStatus] = useState(ConnectionStates.CONNECTING)
  const [playlist, setPlaylist] = useState(null)
  const [normalPlayback, setNormalPlayback] = useState(null)
  const [activeSync, setActiveSync] = useState(null)
  const [serverOffsetMs, setServerOffsetMs] = useState(0)
  const [lastError, setLastError] = useState(null)

  const socketRef = useRef(null)
  const reconnectTimeoutRef = useRef(null)
  const retryCountRef = useRef(0)
  const isUnmountedRef = useRef(false)

  const connect = useCallback(() => {
    if (!windowNumber || isUnmountedRef.current) return

    // Clean up any existing connection
    if (socketRef.current) {
      socketRef.current.onopen = null
      socketRef.current.onclose = null
      socketRef.current.onerror = null
      socketRef.current.onmessage = null
      try {
        socketRef.current.close()
      } catch {
        // ignore
      }
      socketRef.current = null
    }

    const wsUrl = `${WS_BASE}/ws?window_id=${windowNumber}`
    setConnectionStatus(retryCountRef.current > 0 ? ConnectionStates.RECONNECTING : ConnectionStates.CONNECTING)

    try {
      const ws = new WebSocket(wsUrl)
      socketRef.current = ws

      ws.onopen = () => {
        if (isUnmountedRef.current) return
        setConnectionStatus(ConnectionStates.CONNECTED)
        retryCountRef.current = 0
        setLastError(null)
      }

      ws.onmessage = (event) => {
        if (isUnmountedRef.current) return
        try {
          const envelope = JSON.parse(event.data)
          handleMessage(envelope)
        } catch (err) {
          console.warn('Failed to parse websocket message', err, event.data)
        }
      }

      ws.onerror = (err) => {
        console.warn(`WebSocket error for window ${windowNumber}`, err)
        setLastError('Connection error')
      }

      ws.onclose = (event) => {
        if (isUnmountedRef.current) return
        setConnectionStatus(ConnectionStates.DISCONNECTED)
        socketRef.current = null

        // If not a clean manual close, trigger exponential backoff reconnect
        if (event.code !== 1000) {
          const delay = Math.min(10000, 1000 * Math.pow(1.5, retryCountRef.current))
          retryCountRef.current += 1
          console.log(`Reconnecting window ${windowNumber} in ${delay}ms (attempt ${retryCountRef.current})`)
          reconnectTimeoutRef.current = setTimeout(() => {
            connect()
          }, delay)
        }
      }
    } catch (err) {
      setConnectionStatus(ConnectionStates.DISCONNECTED)
      setLastError(err.message)
    }
  }, [windowNumber])

  const handleMessage = useCallback((envelope) => {
    const { type, payload } = envelope

    switch (type) {
      case 'STATE_SNAPSHOT': {
        if (payload.server_time) {
          setServerOffsetMs(calculateServerOffset(payload.server_time))
        }
        if (payload.playlist) {
          setPlaylist(payload.playlist)
        }
        if (payload.normal_playback) {
          setNormalPlayback(payload.normal_playback)
        }
        setActiveSync(payload.active_sync || null)
        break
      }

      case 'PLAYLIST_UPDATED': {
        if (payload.server_time) {
          setServerOffsetMs(calculateServerOffset(payload.server_time))
        }
        if (payload.playlist) {
          setPlaylist(payload.playlist)
        }
        if (payload.normal_playback) {
          setNormalPlayback(payload.normal_playback)
        }
        break
      }

      case 'SYNC_STARTED': {
        if (payload.server_time) {
          setServerOffsetMs(calculateServerOffset(payload.server_time))
        }
        setActiveSync({
          event_id: payload.event_id,
          media_key: payload.media_key,
          media: payload.media,
          duration_seconds: payload.duration_seconds,
          start_time: payload.start_time,
          end_time: payload.end_time,
          is_active: false,
          sync_offset_ms: 0,
        })
        break
      }

      case 'SYNC_ENDED': {
        if (payload.server_time) {
          setServerOffsetMs(calculateServerOffset(payload.server_time))
        }
        setActiveSync(null)
        break
      }

      case 'ERROR': {
        setLastError(payload?.message || 'Server error')
        break
      }

      case 'PONG': {
        break
      }

      default:
        break
    }
  }, [])

  useEffect(() => {
    isUnmountedRef.current = false
    retryCountRef.current = 0
    connect()

    return () => {
      isUnmountedRef.current = true
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current)
      }
      if (socketRef.current) {
        socketRef.current.close(1000, 'Component unmounted')
        socketRef.current = null
      }
    }
  }, [connect])

  const send = useCallback((type, payload = {}) => {
    if (socketRef.current && socketRef.current.readyState === WebSocket.OPEN) {
      socketRef.current.send(JSON.stringify({ type, payload }))
    }
  }, [])

  return {
    connectionStatus,
    playlist,
    normalPlayback,
    activeSync,
    serverOffsetMs,
    lastError,
    send,
  }
}
