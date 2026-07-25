import type { DerivedStatus } from '../lib/types'
import { STATUS_META, statusColor } from '../lib/status'
import { StatusDot } from './StatusDot'

export interface StatusChipProps {
  status: DerivedStatus
  /** Ticket count for this status on a project card (docs/DESIGN.md §3). */
  count: number
  className?: string
}

/**
 * A project card's per-status count (docs/DESIGN.md §3): a StatusDot, the
 * label, and the count. The count is set in the §2.2 status colour — it *is*
 * a count of that status, so the colour is the semantic, not decoration; a
 * zero count drops back to the quiet dim tokens so a project's real activity
 * is what reads. The pill surface itself stays neutral.
 */
export function StatusChip({ status, count, className = '' }: StatusChipProps) {
  const meta = STATUS_META[status]
  const empty = count === 0
  return (
    <span
      className={[
        'inline-flex items-center gap-1.5 rounded-chip bg-surface-2 px-2.5 py-1 text-xs',
        empty ? 'text-text-dim' : 'text-text-mut',
        className,
      ]
        .filter(Boolean)
        .join(' ')}
    >
      <StatusDot status={status} className={empty ? 'opacity-40' : ''} />
      <span>{meta.label}</span>
      <span
        className="font-mono font-medium tabular-nums"
        style={{ color: empty ? 'var(--text-dim)' : statusColor(status) }}
      >
        {count}
      </span>
    </span>
  )
}
