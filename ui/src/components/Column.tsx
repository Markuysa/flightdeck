import type { BoardTicket, DerivedStatus } from '../lib/types'
import { STATUS_META, statusColor } from '../lib/status'
import { StatusDot } from './StatusDot'
import { TicketCard } from './TicketCard'

export interface ColumnProps {
  status: DerivedStatus
  tickets: BoardTicket[]
  projectId: string
  /** Upstream ticket id -> derived status, passed through to every card's
   * DependencyTrail (docs/DESIGN.md §3). */
  statuses: Record<number, DerivedStatus>
}

/**
 * One kanban column (docs/DESIGN.md §4.2), fixed at the 300px column width
 * (§2.4). The lane reads as its derived status at a glance: a top accent bar
 * and the status label both carry the §2.2 status colour — the column *is*
 * that status, so this is the status semantic, not decorative reuse.
 */
export function Column({ status, tickets, projectId, statuses }: ColumnProps) {
  const meta = STATUS_META[status]
  const color = statusColor(status)
  return (
    <section className="flex w-[300px] shrink-0 flex-col overflow-hidden rounded-card border border-border-soft bg-surface">
      <div className="h-0.5 w-full shrink-0" style={{ backgroundColor: color }} />
      <header className="flex items-center justify-between gap-2 px-3 pb-2 pt-3">
        <span className="flex min-w-0 items-center gap-2">
          <StatusDot status={status} />
          <span
            className="truncate font-mono text-[10.5px] font-medium uppercase tracking-[0.1em]"
            style={{ color }}
          >
            {meta.label}
          </span>
        </span>
        <span className="shrink-0 font-mono text-xs tabular-nums text-text-mut">
          {tickets.length}
        </span>
      </header>
      <div className="flex min-h-[7rem] flex-1 flex-col gap-2 px-3 pb-3">
        {tickets.length === 0 ? (
          <p className="flex flex-1 items-center justify-center rounded-nested border border-dashed border-border-soft px-3 py-6 text-center text-xs text-text-dim">
            No tickets
          </p>
        ) : (
          tickets.map((ticket) => (
            <TicketCard key={ticket.id} ticket={ticket} projectId={projectId} statuses={statuses} />
          ))
        )}
      </div>
    </section>
  )
}
