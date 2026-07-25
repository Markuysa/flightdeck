import { Link } from 'react-router-dom'
import type { BoardTicket, DerivedStatus } from '../lib/types'
import { statusColor } from '../lib/status'
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
 * dependency dot-trail. A 2px left edge carries the ticket's own derived
 * status colour (§2.2 — the card's status, not decoration), so a card reads
 * its status even away from its column. A real link, keyboard-activatable.
 */
export function TicketCard({ ticket, projectId, statuses }: TicketCardProps) {
  return (
    <Link
      to={`/p/${encodeURIComponent(projectId)}/t/${ticket.id}`}
      style={{ borderLeftColor: statusColor(ticket.status) }}
      className="flex flex-col gap-2 rounded-card border border-l-2 border-border-soft bg-surface-2 p-3 transition-colors duration-150 hover:border-border"
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
