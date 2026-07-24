import { Bot } from 'lucide-react'
import { Link } from 'react-router-dom'
import { agentActivity } from '../lib/agentActivity'
import type { AgentSession } from '../lib/types'
import { StatusDot } from './StatusDot'

export interface AgentRowProps {
  session: AgentSession
  /** Current time, injected so activity/idle is testable without faking the
   * system clock (docs/tickets/012) — defaults to the real clock. */
  now?: Date
}

/**
 * One live agent session (US-4, docs/DESIGN.md §4.4): the working agent, the
 * project and ticket it's on (each linked), its branch, and how long since
 * its last activity. The live dot pulses while active and goes static once
 * the session is idle (docs/DESIGN.md §3 — the app's only motion).
 */
export function AgentRow({ session, now = new Date() }: AgentRowProps) {
  const { active, relativeTime } = agentActivity(session.last_activity_at, now)

  return (
    <li className="flex flex-wrap items-center gap-x-6 gap-y-2 rounded-card border border-border-soft bg-surface p-4">
      <span className="inline-flex items-center gap-2 text-sm text-text">
        <StatusDot status="in_progress" live={active} />
        <Bot className="h-4 w-4 text-text-dim" aria-hidden="true" />
        Agent
      </span>

      <Link
        to={`/p/${encodeURIComponent(session.project_id)}`}
        className="text-sm text-text hover:text-accent"
      >
        {session.project_name}
      </Link>

      <Link
        to={`/p/${encodeURIComponent(session.project_id)}/t/${session.ticket_id}`}
        className="flex items-center gap-2 text-sm text-text hover:text-accent"
      >
        <span className="font-mono text-text-dim">#{session.ticket_id}</span>
        {session.ticket_title}
      </Link>

      <span className="font-mono text-xs text-text-dim">{session.branch}</span>

      <span className="ml-auto font-mono text-xs text-text-dim">{relativeTime}</span>

      {/* session_url is empty in v1 (docs/tickets/008's handoff) — only link
          when a real routine has one. */}
      {session.session_url && (
        <a
          href={session.session_url}
          target="_blank"
          rel="noreferrer"
          className="font-mono text-xs text-accent hover:underline"
        >
          Session
        </a>
      )}
    </li>
  )
}
