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
// always renders with PR/CI state absent.
type CreateProjectRequest struct {
	Name     string `json:"name"`
	RepoPath string `json:"repo_path"`
	GitHub   *struct {
		Owner string `json:"owner"`
		Repo  string `json:"repo"`
	} `json:"github,omitempty"`
}

// DispatchRequest is POST /api/projects/{id}/dispatch's request body.
type DispatchRequest struct {
	TicketID int `json:"ticket_id"`
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
// v1 sourcing note (ticket 008's handoff, a documented addition/deviation):
// SessionURL is always "" — no source records a live routine session once
// dispatched in v1 (a session URL is only ever seen once, in a dispatch
// response, and is not persisted anywhere FlightDeck reads back from).
// StartedAt and LastActivityAt are both best-effort filled with the
// ticket's branch tip commit time (see gitHubSource.BranchCommitTime in
// board.go) rather than true session start/heartbeat times, since no
// source in v1 records those separately from the branch's own git history.
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
