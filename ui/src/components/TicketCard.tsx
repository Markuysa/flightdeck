import { Link } from 'react-router-dom'
import type { BoardTicket, DerivedStatus } from '../lib/types'
import { DependencyTrail } from './DependencyTrail'

export interface TicketCardProps {
  ticket: BoardTicket
  projectId: string
  /** Upstream ticket id -> derived status, passed through to
   * DependencyTrail (docs/DESIGN.md §3). */
  statuses: Record<number, DerivedStatus>
}

/**
 * One kanban card (docs/DESIGN.md §4.2): mono id, title, role chip, and the
 * dependency dot-trail. A real link to the ticket route, not a bare div with
 * onClick, so it's reachable and activatable by keyboard.
 */
export function TicketCard({ ticket, projectId, statuses }: TicketCardProps) {
  return (
    <Link
      to={`/p/${encodeURIComponent(projectId)}/t/${ticket.id}`}
      className="flex flex-col gap-2 rounded-card border border-border-soft bg-surface-2 p-3 transition-colors duration-150 hover:border-border"
    >
      <div className="flex items-center justify-between gap-2">
        <span className="font-mono text-xs text-text-dim">#{ticket.id}</span>
        <span className="rounded-chip border border-border-soft bg-surface px-2 py-0.5 text-[11px] font-medium uppercase tracking-wide text-text-mut">
          {ticket.role}
        </span>
      </div>
      <p className="text-sm font-medium leading-snug text-text">{ticket.title}</p>
      <DependencyTrail depends={ticket.depends} statuses={statuses} />
    </Link>
  )
}
