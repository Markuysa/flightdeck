import { STATUS_ORDER, statusColor } from '../lib/status'
import type { DerivedStatus, ProjectSummary } from '../lib/types'

export interface FleetSummaryProps {
  projects: ProjectSummary[]
}

/**
 * The fleet in one line: how much work exists, how much is moving, and how
 * much is waiting on a person.
 *
 * The three figures are chosen to answer the only questions worth asking
 * before opening a board. "Waiting on you" is the load-bearing one — it is the
 * sum of the two statuses no agent can clear on its own (needs_attention and
 * in_review), so it is the number that tells an operator whether the fleet
 * needs them right now. A count of `done` is deliberately absent: it grows
 * forever and answers nothing.
 */
export function FleetSummary({ projects }: FleetSummaryProps) {
  const totals = STATUS_ORDER.reduce(
    (acc, status) => {
      acc[status] = projects.reduce((sum, p) => sum + (p.counts[status] ?? 0), 0)
      return acc
    },
    {} as Record<DerivedStatus, number>,
  )

  const queued = totals.ready + totals.blocked
  const working = totals.in_progress
  const needsYou = totals.needs_attention + totals.in_review

  return (
    <dl className="flex flex-wrap items-stretch gap-px overflow-hidden rounded-card border border-border-soft bg-border-soft">
      <Figure
        label="Projects"
        value={projects.length}
        detail={`${projects.filter((p) => p.hasLiveAgent).length} with an agent working`}
      />
      <Figure
        label="Queued"
        value={queued}
        detail={`${totals.ready} ready · ${totals.blocked} blocked`}
        color={totals.ready > 0 ? statusColor('ready') : undefined}
      />
      <Figure
        label="Working"
        value={working}
        color={working > 0 ? statusColor('in_progress') : undefined}
      />
      <Figure
        label="Waiting on you"
        value={needsYou}
        detail={`${totals.in_review} to review · ${totals.needs_attention} stuck`}
        color={needsYou > 0 ? statusColor('needs_attention') : undefined}
      />
    </dl>
  )
}

function Figure({
  label,
  value,
  detail,
  color,
}: {
  label: string
  value: number
  detail?: string
  color?: string
}) {
  return (
    <div className="flex min-w-[9rem] flex-1 flex-col gap-1 bg-surface px-4 py-3">
      <dt className="font-mono text-[10.5px] uppercase tracking-[0.1em] text-text-dim">{label}</dt>
      <dd className="flex items-baseline gap-2">
        <span
          className="font-display text-2xl font-semibold tabular-nums leading-none"
          style={color ? { color } : undefined}
        >
          {value}
        </span>
      </dd>
      {detail && <p className="text-xs text-text-mut">{detail}</p>}
    </div>
  )
}
