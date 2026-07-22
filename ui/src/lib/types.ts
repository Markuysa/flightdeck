// Mirrors internal/core (Go). Keep in lockstep with that package — it is the
// one place status/shape is allowed to be defined; this file only restates
// it for the compiler.

/** internal/core.DerivedStatus. Never trust any other string here. */
export type DerivedStatus =
  | 'ready'
  | 'in_progress'
  | 'in_review'
  | 'blocked'
  | 'needs_attention'
  | 'done'

/** internal/core.Project */
export interface Project {
  id: string
  name: string
  repo_path: string
  remote: 'github' | ''
  owner: string
  repo: string
}

/** A Project as returned by GET /api/projects: registration plus per-status
 * ticket counts for the Fleet view (US-1). The counts shape is inferred
 * ahead of internal/api (ticket 008) — confirm against its handoff. */
export interface ProjectSummary extends Project {
  counts: Record<DerivedStatus, number>
  autopilot: boolean
  hasLiveAgent: boolean
}

/** internal/core.Ticket */
export interface Ticket {
  id: number
  title: string
  role: string
  depends: number[]
  body: string
  handoff: string
}

/** internal/core.PRState */
export interface PRState {
  number: number
  url: string
  ci: 'pending' | 'green' | 'red' | 'unknown'
}

/** internal/core.BoardTicket — Ticket embeds flatten to top-level JSON
 * fields under Go's default encoding/json behaviour for anonymous fields. */
export interface BoardTicket extends Ticket {
  status: DerivedStatus
  branch: string
  pr: PRState | null
}

/** GET /api/projects/{id}/board response: tickets grouped by derived status. */
export type Board = Record<DerivedStatus, BoardTicket[]>

/** GET /api/projects/{id}/tickets/{tid}: ticket detail plus the dependency
 * trail (each depended-on ticket, with its own derived status) so the UI can
 * render handoffs and a dependency chain without extra round-trips. */
export interface TicketDetail extends BoardTicket {
  depends_detail: BoardTicket[]
}

export interface CreateProjectRequest {
  name: string
  repo_path: string
  github?: { owner: string; repo: string }
}

export interface DispatchRequest {
  ticket_id: number
}

export interface DispatchResponse {
  session_url: string
}

export interface AutopilotState {
  on: boolean
}

/** GET /api/agents: live agent sessions (US-4). Shape inferred ahead of
 * internal/api (ticket 008) — confirm against its handoff. */
export interface AgentSession {
  project_id: string
  project_name: string
  ticket_id: number
  ticket_title: string
  branch: string
  session_url: string
  started_at: string
  last_activity_at: string
}
