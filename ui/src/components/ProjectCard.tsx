import { KeyRound, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { Link } from 'react-router-dom'
import { setAutopilot } from '../lib/api'
import { STATUS_ORDER } from '../lib/status'
import type { ProjectSummary } from '../lib/types'
import { ManageTokensDialog } from './ManageTokensDialog'
import { StatusChip } from './StatusChip'
import { StatusDot } from './StatusDot'

export interface ProjectCardProps {
  project: ProjectSummary
  onRemove: () => void
}

/** One registered project on the Fleet screen (docs/DESIGN.md §4.1): name
 * (with a live-agent dot when a ticket is in progress), a StatusChip per
 * derived status, autopilot state, and a "manage tokens" affordance (ticket
 * 016) that opens ManageTokensDialog to set/rotate its routine/GitHub
 * tokens after registration. */
export function ProjectCard({ project, onRemove }: ProjectCardProps) {
  const [managingTokens, setManagingTokens] = useState(false)

  // Autopilot toggle (US-5, ticket 017): seeded from the summary, then only
  // ever set from the server's response — never flipped optimistically — so
  // a rejected request leaves the displayed state exactly where it was.
  const [autopilotOn, setAutopilotOn] = useState(project.autopilot)
  const [autopilotPending, setAutopilotPending] = useState(false)
  const [autopilotError, setAutopilotError] = useState<string | null>(null)

  async function handleAutopilotToggle() {
    setAutopilotPending(true)
    setAutopilotError(null)
    try {
      const result = await setAutopilot(project.id, !autopilotOn)
      setAutopilotOn(result.on)
    } catch (err) {
      setAutopilotError(err instanceof Error ? err.message : 'Failed to update autopilot.')
    } finally {
      setAutopilotPending(false)
    }
  }

  return (
    <article className="flex flex-col gap-4 rounded-card border border-border-soft bg-surface p-4 transition-colors duration-150 hover:border-border">
      <header className="flex items-start justify-between gap-2">
        <Link
          to={`/p/${encodeURIComponent(project.id)}`}
          className="flex min-w-0 flex-col gap-1"
        >
          <span className="flex items-center gap-2 font-display text-base font-semibold text-text hover:text-accent">
            {project.hasLiveAgent && <StatusDot status="in_progress" live />}
            <span className="truncate">{project.name}</span>
          </span>
          <span className="truncate font-mono text-xs text-text-dim">{project.repo_path}</span>
        </Link>
        <div className="flex shrink-0 items-center gap-1">
          <button
            type="button"
            onClick={() => setManagingTokens(true)}
            aria-label={`Manage tokens for ${project.name}`}
            className="rounded-nested p-1.5 text-text-dim transition-colors duration-150 hover:bg-surface-2 hover:text-text"
          >
            <KeyRound className="h-4 w-4" />
          </button>
          <button
            type="button"
            onClick={onRemove}
            aria-label={`Remove ${project.name}`}
            className="rounded-nested p-1.5 text-text-dim transition-colors duration-150 hover:bg-surface-2 hover:text-text"
          >
            <Trash2 className="h-4 w-4" />
          </button>
        </div>
      </header>

      <div className="flex flex-wrap gap-1.5">
        {STATUS_ORDER.map((status) => (
          <StatusChip key={status} status={status} count={project.counts[status]} />
        ))}
      </div>

      <footer className="flex items-center justify-between gap-2 border-t border-border-soft pt-3 text-xs">
        <span className="font-mono uppercase tracking-wide text-text-dim">Autopilot</span>
        <div className="flex items-center gap-2">
          {autopilotError && (
            <span role="alert" className="text-st-attention">
              {autopilotError}
            </span>
          )}
          <button
            type="button"
            role="switch"
            aria-checked={autopilotOn}
            aria-label={`Autopilot for ${project.name}`}
            disabled={autopilotPending}
            onClick={handleAutopilotToggle}
            className={[
              'inline-flex h-5 w-9 shrink-0 items-center rounded-chip border-2 border-transparent',
              'transition-colors duration-150 disabled:cursor-not-allowed disabled:opacity-50',
              autopilotOn ? 'bg-accent' : 'bg-surface-2',
            ].join(' ')}
          >
            <span
              className={[
                'h-4 w-4 rounded-chip bg-text transition-transform duration-150 motion-reduce:transition-none',
                autopilotOn ? 'translate-x-4' : 'translate-x-0',
              ].join(' ')}
            />
          </button>
          <span className={autopilotOn ? 'font-medium text-text' : 'text-text-dim'}>
            {autopilotOn ? 'On' : 'Off'}
          </span>
        </div>
      </footer>

      {managingTokens && (
        <ManageTokensDialog
          projectId={project.id}
          projectName={project.name}
          onClose={() => setManagingTokens(false)}
        />
      )}
    </article>
  )
}
