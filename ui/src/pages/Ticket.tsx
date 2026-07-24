import { FileQuestion, ServerCrash } from 'lucide-react'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { Button } from '../components/Button'
import { DependencyTrail } from '../components/DependencyTrail'
import { EmptyState } from '../components/EmptyState'
import { HandoffList } from '../components/HandoffList'
import { PRBadge } from '../components/PRBadge'
import { StatusDot } from '../components/StatusDot'
import { TicketActions } from '../components/TicketActions'
import { ApiError, getTicket } from '../lib/api'
import { renderTicketBody } from '../lib/markdown'
import { STATUS_META } from '../lib/status'
import type { DerivedStatus, TicketDetail } from '../lib/types'

/** The Ticket screen (US-3, docs/DESIGN.md §4.3): a ticket's body and
 * acceptance criteria, its dependency trail and upstream handoffs, PR/CI
 * when in review, and the dispatch/approve action area (US-6). */
export function Ticket() {
  const { id, tid } = useParams<{ id: string; tid: string }>()
  const ticketId = tid ? Number(tid) : NaN

  const [detail, setDetail] = useState<TicketDetail | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [notFound, setNotFound] = useState(false)

  const refresh = useCallback(async () => {
    if (!id || Number.isNaN(ticketId)) return
    setLoading(true)
    setError(null)
    setNotFound(false)
    try {
      const next = await getTicket(id, ticketId)
      setDetail(next)
    } catch (err) {
      if (err instanceof ApiError && err.status === 404) {
        setNotFound(true)
      } else {
        setError(err instanceof Error ? err.message : 'Failed to load ticket.')
      }
    } finally {
      setLoading(false)
    }
  }, [id, ticketId])

  // Deferred a microtask so eslint-plugin-react-hooks' set-state-in-effect
  // rule sees the async-fetch pattern, not a synchronous setState-during-render.
  useEffect(() => {
    Promise.resolve()
      .then(() => refresh())
      .catch(() => {
        // refresh() already turns a failure into error/notFound state.
      })
  }, [refresh])

  // Upstream ticket id -> derived status, so DependencyTrail can colour dots
  // without a second request (docs/tickets/010's handoff).
  const statuses = useMemo(() => {
    if (!detail) return {}
    return Object.fromEntries(
      detail.depends_detail.map((t) => [t.id, t.status]),
    ) as Record<number, DerivedStatus>
  }, [detail])

  if (!id || Number.isNaN(ticketId)) return null

  return (
    <div className="flex flex-col gap-6 p-6">
      <div>
        <Link to={`/p/${encodeURIComponent(id)}`} className="text-xs text-text-mut hover:text-text">
          ← Board
        </Link>
      </div>

      {notFound ? (
        <EmptyState
          icon={FileQuestion}
          title="Ticket not found"
          description={`There's no ticket #${tid} in this project's queue.`}
        />
      ) : error ? (
        <EmptyState
          icon={ServerCrash}
          title="Couldn't load the ticket"
          description={error}
          action={
            <Button variant="ghost" onClick={refresh}>
              Try again
            </Button>
          }
        />
      ) : loading || !detail ? (
        <p className="py-16 text-center text-sm text-text-mut" aria-live="polite">
          Loading ticket…
        </p>
      ) : (
        <>
          <header className="flex flex-col gap-3">
            <p className="font-mono text-xs text-text-dim">{id}</p>
            <h1 className="font-display text-xl font-semibold text-text">
              <span className="font-mono text-text-dim">#{detail.id}</span> {detail.title}
            </h1>
            <div className="flex flex-wrap items-center gap-3">
              <span className="rounded-chip border border-border-soft bg-surface px-2 py-0.5 text-[11px] font-medium uppercase tracking-wide text-text-mut">
                {detail.role}
              </span>
              <span className="inline-flex items-center gap-1.5 text-xs text-text-mut">
                <StatusDot status={detail.status} />
                {STATUS_META[detail.status].label}
              </span>
              {detail.depends.length > 0 && (
                <DependencyTrail depends={detail.depends} statuses={statuses} />
              )}
              {detail.status === 'in_review' && detail.pr && <PRBadge pr={detail.pr} />}
            </div>
          </header>

          <section className="rounded-card border border-border-soft bg-surface p-4">
            {renderTicketBody(detail.body)}
          </section>

          {detail.depends_detail.length > 0 && (
            <section className="flex flex-col gap-3">
              <h2 className="font-mono text-[10.5px] uppercase tracking-wide text-text-dim">
                Upstream handoffs
              </h2>
              <HandoffList dependencies={detail.depends_detail} />
            </section>
          )}

          <TicketActions projectId={id} ticketId={detail.id} status={detail.status} />
        </>
      )}
    </div>
  )
}
