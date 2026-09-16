import React from 'react'
import { ConnectionStates } from '../playback/playbackMachine'

export function StatusBadge({ status, type = 'connection' }) {
  if (type === 'connection') {
    let colorClass = 'badge-disconnected'
    let label = 'Offline'

    switch (status) {
      case ConnectionStates.CONNECTED:
        colorClass = 'badge-connected'
        label = 'Connected'
        break
      case ConnectionStates.CONNECTING:
        colorClass = 'badge-connecting'
        label = 'Connecting...'
        break
      case ConnectionStates.RECONNECTING:
        colorClass = 'badge-connecting'
        label = 'Reconnecting...'
        break
      default:
        colorClass = 'badge-disconnected'
        label = 'Disconnected'
    }

    return (
      <span className={`status-badge ${colorClass}`} title={`WebSocket: ${label}`}>
        <span className="badge-dot"></span>
        {label}
      </span>
    )
  }

  if (type === 'sync') {
    if (status?.isActive) {
      return (
        <span className="status-badge badge-sync-active" title="Global synchronization active">
          <span className="badge-dot pulse"></span>
          SYNC ACTIVE
        </span>
      )
    }

    if (status?.isScheduled) {
      return (
        <span className="status-badge badge-sync-preparing" title="Preloading synchronized media">
          <span className="badge-dot"></span>
          SYNC PREPARING
        </span>
      )
    }

    return (
      <span className="status-badge badge-normal" title="Normal 5-hour cycle sequence">
        NORMAL SEQUENCE
      </span>
    )
  }

  return null
}
