import type { BoardTicket, DerivedStatus } from '../lib/types'
import { StatusChip } from './StatusChip'
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
 * One kanban column (docs/DESIGN.md §4.2): a StatusChip-headed list of the
 * tickets in this derived status, fixed at the 300px column width (§2.4).
 */
export function Column({ status, tickets, projectId, statuses }: ColumnProps) {
  return (
    <section className="flex w-[300px] shrink-0 flex-col gap-3 rounded-card border border-border-soft bg-surface p-3">
      <header>
        <StatusChip status={status} count={tickets.length} />
      </header>
      <div className="flex flex-col gap-2">
        {tickets.length === 0 ? (
          <p className="rounded-nested border border-dashed border-border-soft px-3 py-6 text-center text-xs text-text-dim">
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
