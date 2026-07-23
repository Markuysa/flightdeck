import type { PRState } from '../lib/types'

export interface PRBadgeProps {
  pr: PRState
  className?: string
}

// docs/tickets/004's CI-state -> colour mapping. Looked up via inline
// `var()` rather than a static Tailwind class, since the colour is chosen at
// render time from a dynamic `pr.ci` value (same reasoning as StatusDot).
const CI_META: Record<PRState['ci'], { label: string; colorVar: string }> = {
  pending: { label: 'Pending', colorVar: '--st-progress' },
  green: { label: 'Passing', colorVar: '--st-done' },
  red: { label: 'Failing', colorVar: '--st-attention' },
  unknown: { label: 'Unknown', colorVar: '--text-dim' },
}

/** PR number (linked, mono) and CI state for an in-review ticket
 * (docs/DESIGN.md §4.3). Only rendered when the ticket's derived status is
 * `in_review` and `pr` is non-null — the caller (Ticket.tsx) guards that. */
export function PRBadge({ pr, className = '' }: PRBadgeProps) {
  const ci = CI_META[pr.ci]
  return (
    <span
      className={[
        'inline-flex items-center gap-2 rounded-chip border border-border-soft bg-surface px-2.5 py-1 text-xs',
        className,
      ]
        .filter(Boolean)
        .join(' ')}
    >
      <a
        href={pr.url}
        target="_blank"
        rel="noreferrer"
        className="font-mono text-text-mut underline-offset-2 hover:text-text hover:underline"
      >
        #{pr.number}
      </a>
      <span className="inline-flex items-center gap-1.5" style={{ color: `var(${ci.colorVar})` }}>
        <span
          role="img"
          aria-label={`CI ${ci.label}`}
          className="h-1.5 w-1.5 shrink-0 rounded-chip"
          style={{ backgroundColor: `var(${ci.colorVar})` }}
        />
        {ci.label}
      </span>
    </span>
  )
}
