package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Markuysa/flightdeck/internal/core"
	"github.com/Markuysa/flightdeck/internal/plan"
	"github.com/Markuysa/flightdeck/internal/registry"
)

// AgentStore is the CRUD surface for per-project agent configuration.
// registry.Store satisfies it structurally.
type AgentStore interface {
	AgentReader
	SetAgent(ctx context.Context, a core.Agent) error
	Agents(ctx context.Context, projectID string) ([]core.Agent, error)
	RemoveAgent(ctx context.Context, projectID, id string) error
}

// SaveAgentRequest is POST /api/projects/{id}/agents' body.
type SaveAgentRequest struct {
	Name   string   `json:"name"`
	Role   string   `json:"role"`
	Prompt string   `json:"prompt"`
	Skills []string `json:"skills"`
}

// handleListAgentConfigs implements GET /api/projects/{id}/agents.
//
// Note the route split: this is the project's configured *specialists*, while
// GET /api/agents (agents.go) is the live sessions currently working. Two
// different things that unavoidably share a word — the paths keep them apart.
func (s *Server) handleListAgentConfigs(w http.ResponseWriter, r *http.Request) {
	p, ok := s.projectOr404(w, r)
	if !ok {
		return
	}
	if s.agentStore == nil {
		writeJSON(w, http.StatusOK, []core.Agent{})
		return
	}
	agents, err := s.agentStore.Agents(r.Context(), p.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list agents")
		return
	}
	writeJSON(w, http.StatusOK, agents)
}

// handleSaveAgent implements POST /api/projects/{id}/agents: configure the
// agent for one role, replacing whatever held that role before.
func (s *Server) handleSaveAgent(w http.ResponseWriter, r *http.Request) {
	p, ok := s.projectOr404(w, r)
	if !ok {
		return
	}
	if s.agentStore == nil {
		writeError(w, http.StatusNotImplemented, "agent configuration is not available on this server")
		return
	}
	var body SaveAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	name := strings.TrimSpace(body.Name)
	role := strings.TrimSpace(body.Role)
	if name == "" || role == "" {
		writeError(w, http.StatusBadRequest, "name and role are required")
		return
	}
	// The role must be one the planner can actually assign, or the agent would
	// be configured for tickets that can never exist.
	if !slices.Contains(plan.Roles, role) {
		writeError(w, http.StatusBadRequest,
			"role must be one of: "+strings.Join(plan.Roles, ", "))
		return
	}

	agent := core.Agent{
		ID:        slugify(name),
		Name:      name,
		Role:      role,
		Prompt:    body.Prompt,
		Skills:    body.Skills,
		ProjectID: p.ID,
	}
	if agent.ID == "" {
		writeError(w, http.StatusBadRequest, "name must contain at least one letter or digit")
		return
	}

	if err := s.agentStore.SetAgent(r.Context(), agent); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save agent")
		return
	}
	writeJSON(w, http.StatusOK, agent)
}

// handleDeleteAgent implements DELETE /api/projects/{id}/agents/{agentID}.
func (s *Server) handleDeleteAgent(w http.ResponseWriter, r *http.Request) {
	p, ok := s.projectOr404(w, r)
	if !ok {
		return
	}
	if s.agentStore == nil {
		writeError(w, http.StatusNotImplemented, "agent configuration is not available on this server")
		return
	}
	err := s.agentStore.RemoveAgent(r.Context(), p.ID, chi.URLParam(r, "agentID"))
	switch {
	case errors.Is(err, registry.ErrAgentNotFound):
		writeError(w, http.StatusNotFound, "agent not found")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "failed to remove agent")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RunSummary is one entry of GET /api/projects/{id}/runs: what this server
// dispatched, when, and how it ended.
//
// This is the timeline the audit called for (stage 4.3). It answers "what has
// the scheduler actually been doing" — a question the board cannot, because the
// board shows current state while this shows attempts, including the ones that
// failed or timed out and left no trace in git.
type RunSummary struct {
	ID         int64  `json:"id"`
	TicketID   int    `json:"ticket_id"`
	Attempt    int    `json:"attempt"`
	State      string `json:"state"`
	SessionURL string `json:"session_url"`
	Detail     string `json:"detail"`
	StartedAt  string `json:"started_at"`
	SettledAt  string `json:"settled_at"`
}

// RunHistoryReader lists a project's dispatch history.
type RunHistoryReader interface {
	ProjectRuns(ctx context.Context, projectID string, limit int) ([]registry.Run, error)
}

// handleListRuns implements GET /api/projects/{id}/runs.
func (s *Server) handleListRuns(w http.ResponseWriter, r *http.Request) {
	p, ok := s.projectOr404(w, r)
	if !ok {
		return
	}
	if s.runHistory == nil {
		writeJSON(w, http.StatusOK, []RunSummary{})
		return
	}
	runs, err := s.runHistory.ProjectRuns(r.Context(), p.ID, 100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list runs")
		return
	}

	out := make([]RunSummary, len(runs))
	for i, run := range runs {
		out[i] = RunSummary{
			ID: run.ID, TicketID: run.TicketID, Attempt: run.Attempt,
			State: string(run.State), SessionURL: run.SessionURL, Detail: run.Detail,
			StartedAt: isoOrEmpty(run.StartedAt), SettledAt: isoOrEmpty(run.SettledAt),
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// isoOrEmpty renders a timestamp as RFC3339, or "" for the zero time — a run
// that has not settled has no settled_at, and "0001-01-01T00:00:00Z" would be
// a lie the UI then has to detect.
func isoOrEmpty(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
