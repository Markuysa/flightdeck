// Package api implements the frozen REST + SSE contract in
// docs/ARCHITECTURE.md's "API contract (frozen for the UI)" table: it
// composes internal/derive, internal/registry, internal/dispatch, and the
// internal/source/git + internal/source/github readers behind chi routes,
// authenticating a single bearer token traded for a session cookie.
//
// Every response DTO in this file mirrors ui/src/lib/types.ts field for
// field — that file's own header comment says as much ("Mirrors
// internal/core (Go). Keep in lockstep with that package") — so a renamed
// field here is a frontend contract break, not a free refactor.
package api

import (
	"github.com/Markuysa/flightdeck/internal/core"
)

// ProjectSummary is what GET /api/projects returns per project: the
// registration (core.Project, flattened via Go's anonymous-field JSON
// encoding) plus per-status ticket counts, autopilot state, and whether any
// ticket currently has a live agent (US-1).
type ProjectSummary struct {
	core.Project
	Counts       map[core.DerivedStatus]int `json:"counts"`
	Autopilot    bool                       `json:"autopilot"`
	HasLiveAgent bool                       `json:"hasLiveAgent"`
}

// TicketDetail is what GET /api/projects/{id}/tickets/{tid} returns: the
// ticket's own derived state (core.BoardTicket, flattened) plus each
// dependency's own BoardTicket, so the UI can render handoffs and the
// dependency chain without extra round-trips (US-3).
type TicketDetail struct {
	core.BoardTicket
	DependsDetail []core.BoardTicket `json:"depends_detail"`
}

// CreateProjectRequest is POST /api/projects' request body (US-7). GitHub
// is optional: a project registered without it gets Remote == "" and
// always renders with PR/CI state absent. RoutineToken/GitHubToken are also
// optional and, when either is non-empty, are written via
// registry.Store.SetSecrets right after the project is added — they are
// never echoed back in the response (handleCreateProject returns a bare
// core.Project, which carries no secret field by construction).
type CreateProjectRequest struct {
	Name     string `json:"name"`
	RepoPath string `json:"repo_path"`
	// RoutineTriggerID names the Claude routine that implements this
	// project's tickets. Optional: a project registered without it renders
	// its board normally but cannot be dispatched (409 from
	// POST /dispatch), which is the honest answer to "run this ticket" on
	// a project that has no runner wired up.
	RoutineTriggerID string `json:"routine_trigger_id,omitempty"`
	RoutineToken     string `json:"routine_token,omitempty"`
	GitHubToken      string `json:"github_token,omitempty"`
	GitHub           *struct {
		Owner string `json:"owner"`
		Repo  string `json:"repo"`
	} `json:"github,omitempty"`
}

// SetSecretsRequest is PUT /api/projects/{id}/secrets' request body. Either
// field may be omitted or left an empty string to leave that token
// unchanged — handleSetSecrets reads the project's current registry.Secrets
// and overlays only the non-empty fields before writing, so this is
// deliberately not a full replace: there is no way to clear a token back to
// empty through this endpoint (not asked for by the ticket; SetSecrets
// still supports it directly for anyone driving the registry itself).
type SetSecretsRequest struct {
	RoutineToken string `json:"routine_token,omitempty"`
	GitHubToken  string `json:"github_token,omitempty"`
}

// SecretsStatus is GET /api/projects/{id}/secrets' response body: whether
// each token is currently set, never the value (ADR-005).
type SecretsStatus struct {
	RoutineTokenSet bool `json:"routine_token_set"`
	GitHubTokenSet  bool `json:"github_token_set"`
}

// DispatchRequest is POST /api/projects/{id}/dispatch's request body.
type DispatchRequest struct {
	TicketID int `json:"ticket_id"`
	// Notes is an optional per-dispatch refinement the operator types before
	// firing — "use the v2 endpoint", "don't touch the migration". It rides
	// along to the routine in the briefing and is not persisted: it is advice
	// about THIS attempt, not a property of the ticket.
	Notes string `json:"notes,omitempty"`
}

// DispatchResponse is POST /api/projects/{id}/dispatch's response body: the
// routine session URL the frontend opens immediately (US-5).
type DispatchResponse struct {
	SessionURL string `json:"session_url"`
}

// AutopilotState is both GET and PUT /api/projects/{id}/autopilot's body.
type AutopilotState struct {
	On bool `json:"on"`
}

// AgentSession is one entry of GET /api/agents: a ticket currently
// in_progress on a claude/NNN-* branch — an agent working right now (US-4).
//
// Sourcing note (ticket 019 closes the gap ticket 008's handoff documented):
// LastActivityAt is always the ticket's branch tip commit time (see
// gitHubSource.BranchCommitTime in board.go) — the honest v1 activity
// signal, since no source records true heartbeats separately from git
// history. SessionURL and StartedAt come from the server's in-memory
// dispatchSessionStore (sessions.go) when this server itself dispatched the
// ticket (a more accurate StartedAt than the branch's tip commit time); an
// agent whose branch was never dispatched through this server has SessionURL
// == "" and StartedAt falls back to the branch tip commit time too.
type AgentSession struct {
	ProjectID      string `json:"project_id"`
	ProjectName    string `json:"project_name"`
	TicketID       int    `json:"ticket_id"`
	TicketTitle    string `json:"ticket_title"`
	Branch         string `json:"branch"`
	SessionURL     string `json:"session_url"`
	StartedAt      string `json:"started_at"`
	LastActivityAt string `json:"last_activity_at"`
}

// errorBody is the JSON shape every non-2xx handler response uses.
type errorBody struct {
	Error string `json:"error"`
}
