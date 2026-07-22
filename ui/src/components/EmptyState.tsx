import type { ComponentType, ReactNode } from 'react'

export interface EmptyStateProps {
  icon: ComponentType<{ className?: string }>
  title: string
  description?: ReactNode
  action?: ReactNode
}

/** A quiet empty/placeholder state: icon, title, optional description and
 * action. Used for "nothing here yet" (no projects, no live agents) and for
 * the route placeholders this ticket ships ahead of tickets 009-012. */
export function EmptyState({ icon: Icon, title, description, action }: EmptyStateProps) {
  return (
    <div className="flex flex-col items-center gap-3 rounded-card border border-border-soft bg-surface px-6 py-16 text-center">
      <Icon className="h-8 w-8 text-text-dim" />
      <p className="font-display text-base font-semibold text-text">{title}</p>
      {description && <p className="max-w-sm text-sm text-text-mut">{description}</p>}
      {action}
    </div>
  )
}
