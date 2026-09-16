import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import React from 'react'
import { StatusBadge } from '../StatusBadge'
import { BlankPlayer } from '../BlankPlayer'
import { ImagePlayer } from '../ImagePlayer'
import { VideoPlayer } from '../VideoPlayer'
import { ConnectionStates } from '../../playback/playbackMachine'

describe('StatusBadge component', () => {
  it('renders connected state badge', () => {
    render(<StatusBadge status={ConnectionStates.CONNECTED} type="connection" />)
    expect(screen.getByText('Connected')).toBeInTheDocument()
  })

  it('renders disconnected state badge', () => {
    render(<StatusBadge status={ConnectionStates.DISCONNECTED} type="connection" />)
    expect(screen.getByText('Disconnected')).toBeInTheDocument()
  })

  it('renders sync active badge', () => {
    render(<StatusBadge status={{ isActive: true }} type="sync" />)
    expect(screen.getByText('SYNC ACTIVE')).toBeInTheDocument()
  })

  it('renders normal sequence badge', () => {
    render(<StatusBadge status={{ isActive: false, isScheduled: false }} type="sync" />)
    expect(screen.getByText('NORMAL SEQUENCE')).toBeInTheDocument()
  })
})

describe('BlankPlayer component', () => {
  it('renders configured blank sequence item', () => {
    render(<BlankPlayer isConfiguredBlank={true} durationSeconds={15} />)
    expect(screen.getByText(/Explicit Blank Sequence \(15s\)/i)).toBeInTheDocument()
  })

  it('renders idle message when no playlist is configured', () => {
    render(<BlankPlayer isConfiguredBlank={false} />)
    expect(screen.getByText(/No Playlist Configured — Window Idle/i)).toBeInTheDocument()
  })
})

describe('ImagePlayer component', () => {
  it('renders image with proper alt tag', () => {
    render(
      <ImagePlayer
        src="https://example.com/test.jpg"
        name="Test Image"
        mediaKey="M1"
        durationSeconds={10}
      />
    )
    const img = screen.getByRole('img')
    expect(img).toHaveAttribute('src', 'https://example.com/test.jpg')
    expect(img).toHaveAttribute('alt', 'Test Image')
  })
})

describe('VideoPlayer component', () => {
  it('renders video element with muted and playsInline attributes', () => {
    const { container } = render(
      <VideoPlayer
        src="https://example.com/test.mp4"
        mediaKey="M2"
        isMuted={true}
      />
    )
    const video = container.querySelector('video')
    expect(video).toBeInTheDocument()
    expect(video).toHaveAttribute('src', 'https://example.com/test.mp4')
  })
})
