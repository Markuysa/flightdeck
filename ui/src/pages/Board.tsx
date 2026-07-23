import { Kanban, ServerCrash } from 'lucide-react'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useParams } from 'react-router-dom'
import { Button } from '../components/Button'
import { Column } from '../components/Column'
import { EmptyState } from '../components/EmptyState'
import { getBoard } from '../lib/api'
import { useFlightDeckEvents } from '../lib/sse'
import { STATUS_ORDER } from '../lib/status'
import type { Board as BoardData, DerivedStatus } from '../lib/types'

const EMPTY_BOARD: BoardData = {
  ready: [],
  in_progress: [],
  in_review: [],
  needs_attention: [],
  blocked: [],
  done: [],
}

/** The Board screen (US-2, docs/DESIGN.md §4.2): one project as a kanban,
 * columns in pipeline order, live-updated over SSE. */
export function Board() {
  const { id } = useParams<{ id: string }>()
  const [board, setBoard] = useState<BoardData>(EMPTY_BOARD)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const refresh = useCallback(async () => {
    if (!id) return
    try {
      const next = await getBoard(id)
      setBoard(next)
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load board.')
    } finally {
      setLoading(false)
    }
  }, [id])

  // Load once on mount, deferred a microtask so eslint-plugin-react-hooks'
  // set-state-in-effect rule sees the async-fetch pattern (refresh() already
  // awaits before touching state), not a synchronous setState-during-render.
  useEffect(() => {
    Promise.resolve()
      .then(() => refresh())
      .catch(() => {
        // refresh() already turns a failure into `error` state.
      })
  }, [refresh])

  // dispatch.started is published today; board.changed/ci.changed have
  // plumbing but no automatic publisher yet (ticket 008's handoff). Refetch
  // on any of the three — cheap, and correct once a publisher exists.
  useFlightDeckEvents(() => {
    refresh()
  })

  // Upstream ticket id -> derived status, flattened from the same board
  // response so DependencyTrail can colour dots with no extra request.
  const statuses = useMemo(() => {
    const map: Record<number, DerivedStatus> = {}
    for (const status of STATUS_ORDER) {
      for (const ticket of board[status]) {
        map[ticket.id] = ticket.status
      }
    }
    return map
  }, [board])

  const totalTickets = STATUS_ORDER.reduce((sum, status) => sum + board[status].length, 0)

  if (!id) return null

  return (
    <div className="flex flex-col gap-6 p-6">
      <header>
        <h1 className="font-display text-xl font-semibold text-text">Board</h1>
        <p className="font-mono text-xs text-text-dim">{id}</p>
      </header>

      {error ? (
        <EmptyState
          icon={ServerCrash}
          title="Couldn't load the board"
          description={error}
          action={
            <Button variant="ghost" onClick={refresh}>
              Try again
            </Button>
          }
        />
      ) : loading ? (
        <p className="py-16 text-center text-sm text-text-mut" aria-live="polite">
          Loading board…
        </p>
      ) : totalTickets === 0 ? (
        <EmptyState
          icon={Kanban}
          title="No tickets yet"
          description="This project's docs/tickets queue is empty."
        />
      ) : (
        <div className="overflow-x-auto board:overflow-x-visible">
          <div className="flex gap-4 pb-2">
            {STATUS_ORDER.map((status) => (
              <Column
                key={status}
                status={status}
                tickets={board[status]}
                projectId={id}
                statuses={statuses}
              />
            ))}
          </div>
        </div>
      )}
    </div>
  )
}
