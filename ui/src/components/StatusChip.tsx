import type { DerivedStatus } from '../lib/types'
import { STATUS_META } from '../lib/status'
import { StatusDot } from './StatusDot'

export interface StatusChipProps {
  status: DerivedStatus
  /** Ticket count for this status on a project card (docs/DESIGN.md §3). */
  count: number
  className?: string
}

/**
 * A chip pairing a StatusDot with a label and count, for a project card's
 * per-status breakdown (docs/DESIGN.md §3). The chip surface itself stays
 * neutral (surface/border tokens) — the status colour lives on the dot only,
 * per §2.2's "never reuse them decoratively".
 */
export function StatusChip({ status, count, className = '' }: StatusChipProps) {
  const meta = STATUS_META[status]
  return (
    <span
      className={[
        'inline-flex items-center gap-1.5 rounded-chip border border-border-soft',
        'bg-surface px-2.5 py-1 text-xs text-text-mut',
        className,
      ]
        .filter(Boolean)
        .join(' ')}
    >
      <StatusDot status={status} />
      <span>{meta.label}</span>
      <span className="font-mono text-text">{count}</span>
    </span>
  )
}
