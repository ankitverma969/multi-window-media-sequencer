import { describe, it, expect, vi } from 'vitest'
import {
  calculateServerOffset,
  getEstimatedServerTimeMs,
  evaluateSyncTimeline,
  calculateMediaOffsetSeconds,
  formatTime,
} from '../timeSync'

describe('timeSync utility', () => {
  it('calculates server offset correctly', () => {
    const fixedNow = 1700000000000
    vi.spyOn(Date, 'now').mockReturnValue(fixedNow)

    // Server is 500ms ahead
    const serverTimeStr = new Date(fixedNow + 500).toISOString()
    const offset = calculateServerOffset(serverTimeStr)
    expect(offset).toBe(500)

    // Invalid string returns 0
    expect(calculateServerOffset(null)).toBe(0)
    expect(calculateServerOffset('invalid')).toBe(0)

    vi.restoreAllMocks()
  })

  it('calculates estimated server time correctly', () => {
    const fixedNow = 1700000000000
    vi.spyOn(Date, 'now').mockReturnValue(fixedNow)

    expect(getEstimatedServerTimeMs(250)).toBe(fixedNow + 250)
    expect(getEstimatedServerTimeMs(-100)).toBe(fixedNow - 100)
    expect(getEstimatedServerTimeMs()).toBe(fixedNow)

    vi.restoreAllMocks()
  })

  it('evaluates sync timeline in future (scheduled)', () => {
    const fixedNow = 1700000000000
    vi.spyOn(Date, 'now').mockReturnValue(fixedNow)

    const startStr = new Date(fixedNow + 1000).toISOString() // starts in 1s
    const endStr = new Date(fixedNow + 16000).toISOString()  // ends in 16s

    const state = evaluateSyncTimeline(startStr, endStr, 0)
    expect(state.isScheduled).toBe(true)
    expect(state.isActive).toBe(false)
    expect(state.isExpired).toBe(false)
    expect(state.timeUntilStartMs).toBe(1000)

    vi.restoreAllMocks()
  })

  it('evaluates sync timeline when currently active', () => {
    const fixedNow = 1700000005000 // 5s after start
    vi.spyOn(Date, 'now').mockReturnValue(fixedNow)

    const startStr = new Date(1700000000000).toISOString()
    const endStr = new Date(1700000015000).toISOString() // 15s duration

    const state = evaluateSyncTimeline(startStr, endStr, 0)
    expect(state.isScheduled).toBe(false)
    expect(state.isActive).toBe(true)
    expect(state.isExpired).toBe(false)
    expect(state.offsetMs).toBe(5000)
    expect(state.remainingMs).toBe(10000)

    vi.restoreAllMocks()
  })

  it('evaluates sync timeline when expired', () => {
    const fixedNow = 1700000020000 // 5s after end
    vi.spyOn(Date, 'now').mockReturnValue(fixedNow)

    const startStr = new Date(1700000000000).toISOString()
    const endStr = new Date(1700000015000).toISOString()

    const state = evaluateSyncTimeline(startStr, endStr, 0)
    expect(state.isActive).toBe(false)
    expect(state.isExpired).toBe(true)

    vi.restoreAllMocks()
  })

  it('calculates loop offset for long sync overrides', () => {
    // 10s video, 25s elapsed into sync
    const offset = calculateMediaOffsetSeconds(25000, 10)
    expect(offset).toBe(5) // 25 % 10 = 5

    // 0 duration
    expect(calculateMediaOffsetSeconds(25000, 0)).toBe(0)
  })

  it('formats seconds into MM:SS correctly', () => {
    expect(formatTime(0)).toBe('00:00')
    expect(formatTime(15)).toBe('00:15')
    expect(formatTime(65)).toBe('01:05')
    expect(formatTime(3600)).toBe('60:00')
    expect(formatTime(-5)).toBe('00:00')
  })
})
