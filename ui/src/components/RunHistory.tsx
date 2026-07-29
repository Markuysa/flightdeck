import { useCallback, useEffect, useState } from 'react'
import { listRuns } from '../lib/api'
import type { Run } from '../lib/types'

export interface RunHistoryProps {
  projectId: string
}

/**
 * What this server has actually dispatched, most recent first.
 *
 * The board cannot answer this. A board shows current state derived from git,
 * so a dispatch that failed or timed out before its agent pushed a branch
 * leaves no trace there at all — the ticket simply sits at `ready` and the
 * operator has no way to tell "never started" from "started twice and gave up".
 * This is the only view where those attempts exist.
 */
export function RunHistory({ projectId }: RunHistoryProps) {
  const [runs, setRuns] = useState<Run[]>([])
  const [loaded, setLoaded] = useState(false)

  const refresh = useCallback(async () => {
    try {
      const next = await listRuns(projectId)
      // Defensive: this block is supplementary, and a board that renders
      // without its history is still fully usable — but a board that CRASHES
      // because the history came back in an unexpected shape is not. Never let
      // the optional thing take the essential one down.
      setRuns(Array.isArray(next) ? next : [])
    } catch {
      // History is supplementary: a board that renders without it is still
      // useful, so a failure here stays silent rather than taking the screen.
    } finally {
      setLoaded(true)
    }
  }, [projectId])

  useEffect(() => {
    Promise.resolve()
      .then(() => refresh())
      .catch(() => {})
  }, [refresh])

  if (!loaded || runs.length === 0) return null

  return (
    <section className="flex flex-col gap-3">
      <h2 className="font-mono text-[10.5px] uppercase tracking-[0.1em] text-text-dim">
        Dispatch history
      </h2>
      <ul className="flex flex-col gap-px overflow-hidden rounded-card border border-border-soft bg-border-soft">
        {runs.slice(0, 12).map((run) => (
          <li
            key={run.id}
            className="flex flex-wrap items-center gap-x-3 gap-y-1 bg-surface px-3 py-2 text-xs"
          >
            <RunStateDot state={run.state} />
            <span className="font-mono text-text-dim">#{run.ticket_id}</span>
            <span className="w-16 shrink-0 text-text-mut">{RUN_LABEL[run.state]}</span>
            {run.attempt > 1 && (
              <span className="font-mono text-text-dim">attempt {run.attempt}</span>
            )}
            <span className="min-w-0 flex-1 truncate text-text-dim">{run.detail}</span>
            <time className="shrink-0 font-mono text-text-dim" dateTime={run.started_at}>
              {formatTime(run.started_at)}
            </time>
            {run.session_url && (
              <a
                href={run.session_url}
                target="_blank"
                rel="noreferrer"
                className="shrink-0 text-accent underline-offset-2 hover:underline"
              >
                session
              </a>
            )}
          </li>
        ))}
      </ul>
    </section>
  )
}

const RUN_LABEL: Record<Run['state'], string> = {
  running: 'Running',
  observed: 'Started',
  timed_out: 'Timed out',
  failed: 'Failed',
}

/**
 * A run's state gets a status colour, but the mapping is to what the state
 * MEANS for the operator, not a new palette: a run in flight reads like work in
 * progress, one whose agent picked the work up reads done, and both failure
 * modes read as needing attention — which is exactly what they need.
 */
const RUN_COLOR: Record<Run['state'], string> = {
  running: 'var(--st-progress)',
  observed: 'var(--st-done)',
  timed_out: 'var(--st-attention)',
  failed: 'var(--st-attention)',
}

function RunStateDot({ state }: { state: Run['state'] }) {
  return (
    <span
      aria-hidden
      className="h-1.5 w-1.5 shrink-0 rounded-chip"
      style={{ backgroundColor: RUN_COLOR[state] }}
    />
  )
}

function formatTime(iso: string): string {
  if (!iso) return ''
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return ''
  return date.toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}
