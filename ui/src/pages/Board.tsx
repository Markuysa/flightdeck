import { Kanban, ServerCrash, Sparkles, Users } from 'lucide-react'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { Button } from '../components/Button'
import { Column } from '../components/Column'
import { EmptyState } from '../components/EmptyState'
import { QueueBar } from '../components/QueueBar'
import { RunHistory } from '../components/RunHistory'
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
  const [roleFilter, setRoleFilter] = useState<string | null>(null)

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

  const counts = useMemo(
    () =>
      Object.fromEntries(STATUS_ORDER.map((s) => [s, board[s].length])) as Record<
        DerivedStatus,
        number
      >,
    [board],
  )

  // Roles present on THIS board, not the full role vocabulary: offering a
  // filter for a role with no tickets is a control that can only ever produce
  // an empty board.
  const roles = useMemo(() => {
    const seen = new Set<string>()
    for (const status of STATUS_ORDER) {
      for (const ticket of board[status]) {
        if (ticket.role) seen.add(ticket.role)
      }
    }
    return [...seen].sort()
  }, [board])

  const visible = useMemo(() => {
    if (!roleFilter) return board
    return Object.fromEntries(
      STATUS_ORDER.map((s) => [s, board[s].filter((t) => t.role === roleFilter)]),
    ) as BoardData
  }, [board, roleFilter])

  if (!id) return null

  return (
    <div className="flex flex-col gap-6 p-6">
      <header className="flex flex-col gap-3">
        <nav className="flex items-center gap-1.5 text-xs text-text-dim">
          <Link to="/" className="transition-colors duration-150 hover:text-text">
            Fleet
          </Link>
          <span aria-hidden>/</span>
          <span className="font-mono text-text-mut">{id}</span>
        </nav>

        <div className="flex flex-wrap items-center justify-between gap-3">
          <h1 className="font-display text-xl font-semibold text-text">Board</h1>
          <div className="flex gap-1">
            <Link
              to={`/p/${encodeURIComponent(id)}/plan`}
              className="flex items-center gap-1.5 rounded-nested border border-border-soft px-2.5 py-1.5 text-xs text-text-mut transition-colors duration-150 hover:border-border hover:text-text"
            >
              <Sparkles className="h-3.5 w-3.5" />
              Plan work
            </Link>
            <Link
              to={`/p/${encodeURIComponent(id)}/agents`}
              className="flex items-center gap-1.5 rounded-nested border border-border-soft px-2.5 py-1.5 text-xs text-text-mut transition-colors duration-150 hover:border-border hover:text-text"
            >
              <Users className="h-3.5 w-3.5" />
              Agents
            </Link>
          </div>
        </div>

        {totalTickets > 0 && <QueueBar counts={counts} />}

        {roles.length > 1 && (
          <div className="flex flex-wrap items-center gap-1.5">
            <FilterChip active={roleFilter === null} onClick={() => setRoleFilter(null)}>
              All roles
            </FilterChip>
            {roles.map((role) => (
              <FilterChip
                key={role}
                active={roleFilter === role}
                onClick={() => setRoleFilter(roleFilter === role ? null : role)}
              >
                {role}
              </FilterChip>
            ))}
          </div>
        )}
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
        // The lane strip owns its own horizontal scroll at every width. It
        // used to switch to overflow-x-visible above the `board` breakpoint
        // (1280px), on the assumption that six lanes fit there — they need
        // ~1880px, so between those widths the lanes overflowed and the whole
        // page scrolled sideways, carrying the heading out of view.
        <div className="overflow-x-auto">
          <div className="flex gap-4 pb-2">
            {STATUS_ORDER.map((status) => (
              <Column
                key={status}
                status={status}
                tickets={visible[status]}
                projectId={id}
                statuses={statuses}
              />
            ))}
          </div>
        </div>
      )}

      {!loading && !error && <RunHistory projectId={id} />}
    </div>
  )
}

function FilterChip({
  active,
  onClick,
  children,
}: {
  active: boolean
  onClick: () => void
  children: React.ReactNode
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={active}
      className={[
        'rounded-chip border px-2.5 py-0.5 text-[11px] font-medium uppercase tracking-wide transition-colors duration-150',
        active
          ? 'border-accent bg-accent-soft text-accent'
          : 'border-border-soft text-text-mut hover:border-border hover:text-text',
      ].join(' ')}
    >
      {children}
    </button>
  )
}
