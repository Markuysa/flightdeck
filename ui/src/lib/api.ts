// Typed client for the frozen contract in docs/ARCHITECTURE.md "API contract
// (frozen for the UI)". One function per row of that table.
//
// Auth: the bearer token is traded once, via createSession(), for a session
// cookie the browser holds and sends automatically (credentials: 'same-origin').
// The token itself is never written to localStorage or a URL, and never kept
// in memory past the createSession() call.
import type {
  AgentSession,
  AutopilotState,
  Board,
  CreateProjectRequest,
  DispatchRequest,
  DispatchResponse,
  Project,
  ProjectSummary,
  TicketDetail,
} from './types'

const BASE = '/api'

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    credentials: 'same-origin',
    ...init,
    headers: {
      'Content-Type': 'application/json',
      ...init.headers,
    },
  })

  if (!res.ok) {
    const body = await res.text().catch(() => '')
    throw new ApiError(res.status, body || res.statusText)
  }

  if (res.status === 204) {
    return undefined as T
  }
  const text = await res.text()
  return (text ? JSON.parse(text) : undefined) as T
}

/** POST /api/session — trade the bearer token for a session cookie. Call
 * once at login; never store the token beyond this call. */
export function createSession(token: string): Promise<void> {
  return request<void>('/session', {
    method: 'POST',
    headers: { Authorization: `Bearer ${token}` },
  })
}

/** GET /api/projects — all registered projects with status counts (US-1). */
export function listProjects(): Promise<ProjectSummary[]> {
  return request<ProjectSummary[]>('/projects')
}

/** POST /api/projects — register a project (US-7). */
export function createProject(body: CreateProjectRequest): Promise<Project> {
  return request<Project>('/projects', {
    method: 'POST',
    body: JSON.stringify(body),
  })
}

/** DELETE /api/projects/{id} — unregister. */
export function deleteProject(id: string): Promise<void> {
  return request<void>(`/projects/${encodeURIComponent(id)}`, { method: 'DELETE' })
}

/** GET /api/projects/{id}/board — derived board, tickets grouped by status (US-2). */
export function getBoard(id: string): Promise<Board> {
  return request<Board>(`/projects/${encodeURIComponent(id)}/board`)
}

/** GET /api/projects/{id}/tickets/{tid} — ticket detail + dependency
 * handoffs + PR/CI (US-3). */
export function getTicket(id: string, ticketId: number): Promise<TicketDetail> {
  return request<TicketDetail>(`/projects/${encodeURIComponent(id)}/tickets/${ticketId}`)
}

/** POST /api/projects/{id}/dispatch — fire the routine for one ticket (US-5). */
export function dispatchTicket(id: string, ticketId: number): Promise<DispatchResponse> {
  const body: DispatchRequest = { ticket_id: ticketId }
  return request<DispatchResponse>(`/projects/${encodeURIComponent(id)}/dispatch`, {
    method: 'POST',
    body: JSON.stringify(body),
  })
}

/** GET /api/projects/{id}/autopilot — read autopilot.json (US-5). */
export function getAutopilot(id: string): Promise<AutopilotState> {
  return request<AutopilotState>(`/projects/${encodeURIComponent(id)}/autopilot`)
}

/** PUT /api/projects/{id}/autopilot — flip autopilot.json (US-5). */
export function setAutopilot(id: string, on: boolean): Promise<AutopilotState> {
  return request<AutopilotState>(`/projects/${encodeURIComponent(id)}/autopilot`, {
    method: 'PUT',
    body: JSON.stringify({ on }),
  })
}

/** POST /api/projects/{id}/tickets/{tid}/approve — merge the ticket's PR;
 * human action only (US-6). */
export function approveTicket(id: string, ticketId: number): Promise<void> {
  return request<void>(
    `/projects/${encodeURIComponent(id)}/tickets/${ticketId}/approve`,
    { method: 'POST' },
  )
}

/** GET /api/agents — live agent sessions (US-4). */
export function listAgents(): Promise<AgentSession[]> {
  return request<AgentSession[]>('/agents')
}
