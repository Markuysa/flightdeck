// Pure live/idle + relative-time logic for the Agents screen (docs/tickets/012,
// docs/DESIGN.md §3 live indicator). Takes `now` as a parameter rather than
// reading the clock itself, so it unit-tests without faking system time —
// callers (AgentRow) pass `new Date()` at the render boundary.

/** A session counts as active while its last activity is under this age
 * (docs/tickets/012: "5 minutes is fine"). Past it, the live dot stops
 * pulsing — the agent has likely gone idle or the routine has ended. */
export const ACTIVE_THRESHOLD_MS = 5 * 60 * 1000

export interface AgentActivity {
  /** Whether the session is still live right now — feeds StatusDot's `live`. */
  active: boolean
  /** Human-readable age, e.g. "just now", "3m ago", "2h ago", "4d ago". */
  relativeTime: string
}

export function agentActivity(lastActivityAt: string, now: Date): AgentActivity {
  const last = new Date(lastActivityAt)
  // Clamped at 0: a future timestamp (clock skew between agent and browser)
  // reads as "just now" and active, rather than a nonsensical negative age.
  const diffMs = Math.max(0, now.getTime() - last.getTime())
  return {
    active: diffMs < ACTIVE_THRESHOLD_MS,
    relativeTime: relativeTime(diffMs),
  }
}

function relativeTime(diffMs: number): string {
  const minutes = Math.floor(diffMs / 60_000)
  if (minutes < 1) return 'just now'
  if (minutes < 60) return `${minutes}m ago`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours}h ago`
  const days = Math.floor(hours / 24)
  return `${days}d ago`
}
