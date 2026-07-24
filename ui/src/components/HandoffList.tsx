import type { BoardTicket } from '../lib/types'
import { StatusDot } from './StatusDot'

export interface HandoffListProps {
  /** `TicketDetail.depends_detail` — each upstream ticket, with its own
   * `handoff` text (docs/tickets/010's handoff on how this map is built). */
  dependencies: BoardTicket[]
  className?: string
}

/** Upstream handoff notes (docs/DESIGN.md §4.3): what each dependency left
 * behind for this ticket to build on. Skips dependencies with no handoff
 * text yet, and renders nothing when none have one. */
export function HandoffList({ dependencies, className = '' }: HandoffListProps) {
  const withHandoff = dependencies.filter((dep) => dep.handoff.trim().length > 0)
  if (withHandoff.length === 0) return null

  return (
    <ul className={['flex flex-col gap-3', className].filter(Boolean).join(' ')}>
      {withHandoff.map((dep) => (
        <li key={dep.id} className="rounded-card border border-border-soft bg-surface p-3">
          <div className="flex items-center gap-2">
            <StatusDot status={dep.status} />
            <span className="font-mono text-xs text-text-dim">#{dep.id}</span>
            <span className="text-sm font-medium text-text">{dep.title}</span>
          </div>
          <p className="mt-2 whitespace-pre-wrap text-sm text-text-mut">{dep.handoff}</p>
        </li>
      ))}
    </ul>
  )
}
