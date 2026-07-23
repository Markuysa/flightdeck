import type { DerivedStatus } from '../lib/types'
import { StatusDot } from './StatusDot'

export interface DependencyTrailProps {
  /** The ticket's `depends`: upstream ticket ids, in file order. */
  depends: number[]
  /** Upstream ticket id -> derived status, so each dot can be coloured
   * without a second request (docs/DESIGN.md §3). The Board screen flattens
   * its six status buckets into this map; the ticket detail screen (011)
   * derives it from `TicketDetail.depends_detail`. */
  statuses: Record<number, DerivedStatus>
  className?: string
}

/**
 * The dependency dot-trail (docs/DESIGN.md §3): a small chain of StatusDots,
 * one per upstream ticket, coloured by that ticket's derived status — so a
 * blocked ticket shows at a glance which upstream is red. Shared verbatim
 * between the Board card (010) and the ticket detail screen (011); keep this
 * component's props stable for both.
 */
export function DependencyTrail({ depends, statuses, className = '' }: DependencyTrailProps) {
  if (depends.length === 0) return null
  return (
    <span className={['inline-flex items-center gap-1', className].filter(Boolean).join(' ')}>
      {depends.map((id) => (
        <StatusDot key={id} status={statuses[id] ?? 'blocked'} className="h-2 w-2" />
      ))}
    </span>
  )
}
