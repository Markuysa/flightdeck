import { ServerCrash, Users } from 'lucide-react'
import { useCallback, useEffect, useState } from 'react'
import { AgentRow } from '../components/AgentRow'
import { Button } from '../components/Button'
import { EmptyState } from '../components/EmptyState'
import { listAgents } from '../lib/api'
import { useFlightDeckEvents } from '../lib/sse'
import type { AgentSession } from '../lib/types'

/** The Agents screen (US-4, docs/DESIGN.md §4.4): one row per session
 * currently working (an in_progress ticket), live-updated over SSE. */
export function Agents() {
  const [sessions, setSessions] = useState<AgentSession[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const refresh = useCallback(async () => {
    try {
      const next = await listAgents()
      setSessions(next)
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load agents.')
    } finally {
      setLoading(false)
    }
  }, [])

  // Deferred a microtask so eslint-plugin-react-hooks' set-state-in-effect
  // rule sees the async-fetch pattern, not a synchronous setState-during-render.
  useEffect(() => {
    Promise.resolve()
      .then(() => refresh())
      .catch(() => {
        // refresh() already turns a failure into `error` state.
      })
  }, [refresh])

  // dispatch.started is published today; board.changed/ci.changed have
  // plumbing but no automatic publisher yet (ticket 008's handoff). Refetch
  // on any of the three, same as Board/Ticket.
  useFlightDeckEvents(() => {
    refresh()
  })

  return (
    <div className="flex flex-col gap-6 p-6">
      <header>
        <h1 className="font-display text-xl font-semibold text-text">Agents</h1>
        <p className="text-sm text-text-mut">Who is working right now, and on what.</p>
      </header>

      {error ? (
        <EmptyState
          icon={ServerCrash}
          title="Couldn't load agents"
          description={error}
          action={
            <Button variant="ghost" onClick={refresh}>
              Try again
            </Button>
          }
        />
      ) : loading ? (
        <p className="py-16 text-center text-sm text-text-mut" aria-live="polite">
          Loading agents…
        </p>
      ) : sessions.length === 0 ? (
        <EmptyState
          icon={Users}
          title="No agents working right now"
          description="Dispatch a ready ticket from its board to start one."
        />
      ) : (
        <ul className="flex flex-col gap-2">
          {sessions.map((session) => (
            <AgentRow key={`${session.project_id}-${session.ticket_id}`} session={session} />
          ))}
        </ul>
      )}
    </div>
  )
}
