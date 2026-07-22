import type { DerivedStatus } from '../lib/types'
import { STATUS_META } from '../lib/status'

export interface StatusDotProps {
  status: DerivedStatus
  /** Pulses the dot (the live-agent indicator, docs/DESIGN.md §3). Off by
   * default — most uses are a static kanban-card dot. */
  live?: boolean
  className?: string
}

/**
 * A filled dot coloured by derived status (docs/DESIGN.md §2.2, §3). The one
 * status primitive repeated on every card; also doubles as the live-agent
 * indicator when `live` is set.
 */
export function StatusDot({ status, live = false, className = '' }: StatusDotProps) {
  const meta = STATUS_META[status]
  return (
    <span
      role="img"
      aria-label={meta.label}
      title={meta.label}
      className={[
        'inline-block h-2.5 w-2.5 shrink-0 rounded-chip',
        live ? 'animate-live-pulse motion-reduce:animate-none' : '',
        className,
      ]
        .filter(Boolean)
        .join(' ')}
      style={{ backgroundColor: `var(${meta.colorVar})` }}
    />
  )
}
