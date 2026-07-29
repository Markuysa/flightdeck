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
  /** The Claude routine that implements this project's tickets. An
   * identifier, not a secret — the routine's bearer token stays server-side
   * (ADR-005). Empty means the project cannot be dispatched. */
  routine_trigger_id: string
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
  /** Optional. Without it the project registers fine and its board renders,
   * but POST /dispatch answers 409 — there is no routine to run. */
  routine_trigger_id?: string
  /** Optional — set via registry.SetSecrets right after registration, never
   * echoed back by POST /api/projects. Omit or leave blank to register
   * without a token. */
  routine_token?: string
  github_token?: string
}

/** PUT /api/projects/{id}/secrets request body. An omitted/empty field
 * leaves that token unchanged (the backend reads-current, overlays, writes). */
export interface SetSecretsRequest {
  routine_token?: string
  github_token?: string
}

/** GET /api/projects/{id}/secrets response: whether each token is
 * currently set, never the value (ADR-005). */
export interface SecretsStatus {
  routine_token_set: boolean
  github_token_set: boolean
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

/** internal/core.Agent — a configured specialist, not a live session. The two
 * unavoidably share a word; `Agent` is the configuration, `AgentSession`
 * (above) is one currently working. */
export interface Agent {
  id: string
  name: string
  role: string
  prompt: string
  skills: string[]
  project_id: string
}

/** The roles a ticket (and therefore an agent) can carry. Mirrors
 * internal/plan.Roles. */
export const ROLES = ['designer', 'frontend', 'backend', 'qa', 'dev'] as const
export type Role = (typeof ROLES)[number]

/** internal/plan.Ticket — one ticket in a proposal, before it exists on disk.
 * `number` is plan-local; real ids are assigned at apply time. */
export interface PlanTicket {
  number: number
  title: string
  role: string
  depends: number[]
  body: string
  acceptance: string[]
  handoff: string
}

/** internal/plan.Plan — a proposed decomposition, for review before applying. */
export interface Plan {
  goal: string
  summary: string
  tickets: PlanTicket[]
  model: string
  created_at: string
}

/** internal/plan.Applied — what applying a plan created. */
export interface Applied {
  files: string[]
  ids: number[]
}

/** internal/api.RunSummary — one dispatch attempt and how it ended. This is
 * history the board cannot show: the board holds current state, runs hold
 * attempts, including the failed and timed-out ones that leave no git trace. */
export interface Run {
  id: number
  ticket_id: number
  attempt: number
  state: 'running' | 'observed' | 'timed_out' | 'failed'
  session_url: string
  detail: string
  started_at: string
  settled_at: string
}

export interface SaveAgentRequest {
  name: string
  role: string
  prompt: string
  skills: string[]
}
