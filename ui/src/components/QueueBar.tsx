import { STATUS_META, STATUS_ORDER, statusColor } from '../lib/status'
import type { DerivedStatus } from '../lib/types'

export interface QueueBarProps {
  counts: Record<DerivedStatus, number>
  /** Renders at half height, for use inside a dense row. */
  compact?: boolean
  className?: string
}

/**
 * A project's queue as one proportional bar, in pipeline order.
 *
 * This is the densest honest summary of a project that fits on one line, and
 * it is the thing the product is actually about: a queue draining. A row of
 * count chips tells you six numbers you then have to compare; the bar shows
 * the ratio directly — a project that is mostly grey is stuck on dependencies,
 * mostly amber is busy, mostly green is nearly done. You read a whole fleet by
 * scanning shapes rather than reading thirty digits.
 *
 * Segments carry each status's own §2.2 colour, which is the semantic use of
 * those colours (a segment IS that status), not decoration.
 */
export function QueueBar({ counts, compact = false, className = '' }: QueueBarProps) {
  const total = STATUS_ORDER.reduce((sum, status) => sum + (counts[status] ?? 0), 0)

  if (total === 0) {
    return (
      <div
        className={[
          'w-full rounded-chip bg-surface-2',
          compact ? 'h-1' : 'h-1.5',
          className,
        ].join(' ')}
        role="img"
        aria-label="No tickets yet"
      />
    )
  }

  const label = STATUS_ORDER.filter((s) => counts[s] > 0)
    .map((s) => `${counts[s]} ${STATUS_META[s].label.toLowerCase()}`)
    .join(', ')

  return (
    <div
      className={[
        'flex w-full overflow-hidden rounded-chip bg-surface-2',
        compact ? 'h-1' : 'h-1.5',
        className,
      ].join(' ')}
      role="img"
      aria-label={`Queue: ${label}`}
    >
      {STATUS_ORDER.map((status) => {
        const count = counts[status] ?? 0
        if (count === 0) return null
        return (
          <div
            key={status}
            style={{
              backgroundColor: statusColor(status),
              // Percentage rather than flex-grow so a one-ticket segment stays
              // proportional instead of being rounded up to a visible slab —
              // the bar's job is the ratio, and a lie about the ratio is worse
              // than a segment too thin to click.
              width: `${(count / total) * 100}%`,
            }}
          />
        )
      })}
    </div>
  )
}
