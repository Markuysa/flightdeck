import { Trash2 } from 'lucide-react'
import { Link } from 'react-router-dom'
import { STATUS_ORDER } from '../lib/status'
import type { ProjectSummary } from '../lib/types'
import { StatusChip } from './StatusChip'
import { StatusDot } from './StatusDot'

export interface ProjectCardProps {
  project: ProjectSummary
  onRemove: () => void
}

/** One registered project on the Fleet screen (docs/DESIGN.md §4.1): name
 * (with a live-agent dot when a ticket is in progress), a StatusChip per
 * derived status, and autopilot state. */
export function ProjectCard({ project, onRemove }: ProjectCardProps) {
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
        <button
          type="button"
          onClick={onRemove}
          aria-label={`Remove ${project.name}`}
          className="shrink-0 rounded-nested p-1.5 text-text-dim transition-colors duration-150 hover:bg-surface-2 hover:text-text"
        >
          <Trash2 className="h-4 w-4" />
        </button>
      </header>

      <div className="flex flex-wrap gap-1.5">
        {STATUS_ORDER.map((status) => (
          <StatusChip key={status} status={status} count={project.counts[status]} />
        ))}
      </div>

      <footer className="flex items-center justify-between border-t border-border-soft pt-3 text-xs">
        <span className="font-mono uppercase tracking-wide text-text-dim">Autopilot</span>
        <span className={project.autopilot ? 'font-medium text-text' : 'text-text-dim'}>
          {project.autopilot ? 'On' : 'Off'}
        </span>
      </footer>
    </article>
  )
}
