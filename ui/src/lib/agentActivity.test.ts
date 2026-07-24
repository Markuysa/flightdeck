import { describe, expect, it } from 'vitest'
import { ACTIVE_THRESHOLD_MS, agentActivity } from './agentActivity'

describe('agentActivity', () => {
  const now = new Date('2026-07-24T12:00:00Z')

  it('is active and reads "just now" seconds after activity', () => {
    const result = agentActivity('2026-07-24T11:59:50Z', now)
    expect(result.active).toBe(true)
    expect(result.relativeTime).toBe('just now')
  })

  it('is active and reads minutes ago under the threshold', () => {
    const result = agentActivity('2026-07-24T11:57:00Z', now)
    expect(result.active).toBe(true)
    expect(result.relativeTime).toBe('3m ago')
  })

  it('goes idle exactly at the active threshold', () => {
    const atThreshold = new Date(now.getTime() - ACTIVE_THRESHOLD_MS).toISOString()
    const result = agentActivity(atThreshold, now)
    expect(result.active).toBe(false)
    expect(result.relativeTime).toBe('5m ago')
  })

  it('is idle and reads hours ago for an older session', () => {
    const result = agentActivity('2026-07-24T09:30:00Z', now)
    expect(result.active).toBe(false)
    expect(result.relativeTime).toBe('2h ago')
  })

  it('is idle and reads days ago for a stale session', () => {
    const result = agentActivity('2026-07-21T12:00:00Z', now)
    expect(result.active).toBe(false)
    expect(result.relativeTime).toBe('3d ago')
  })

  it('clamps a future timestamp (clock skew) to active/"just now"', () => {
    const result = agentActivity('2026-07-24T12:05:00Z', now)
    expect(result.active).toBe(true)
    expect(result.relativeTime).toBe('just now')
  })
})
