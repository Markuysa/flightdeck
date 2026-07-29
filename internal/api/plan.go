package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Markuysa/flightdeck/internal/core"
	"github.com/Markuysa/flightdeck/internal/plan"
)

// Planner decomposes a goal into a ticket graph. internal/plan's ClaudePlanner
// satisfies it; handler tests fake it, so no test spends a token.
type Planner interface {
	Propose(ctx context.Context, goal string, existing []core.Ticket) (plan.Plan, error)
}

// PlanRequest is POST /api/projects/{id}/plan's body.
type PlanRequest struct {
	Goal string `json:"goal"`
}

// ApplyPlanRequest is POST /api/projects/{id}/plan/apply's body: the plan the
// operator reviewed, sent back verbatim.
//
// The plan round-trips through the browser rather than being held server-side
// between the two calls, which has a consequence worth being explicit about:
// what gets written is what the operator approved, including any edit they made
// — the proposal is a draft, not a token to redeem. Validation runs again on
// this side, so an edited plan is checked exactly as hard as a generated one.
type ApplyPlanRequest struct {
	Plan plan.Plan `json:"plan"`
}

// handleProposePlan implements POST /api/projects/{id}/plan: decompose a goal
// into tickets and return them for review. It writes nothing.
func (s *Server) handleProposePlan(w http.ResponseWriter, r *http.Request) {
	p, ok := s.projectOr404(w, r)
	if !ok {
		return
	}
	if s.planner == nil {
		writeError(w, http.StatusNotImplemented,
			"planning is not configured on this server (set ANTHROPIC_API_KEY)")
		return
	}
	var body PlanRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ctx := r.Context()

	// The queue's current contents go to the planner so it extends the project
	// rather than re-proposing work already there. An unreadable queue is not
	// fatal — planning into an empty project is still useful.
	var existing []core.Ticket
	if tickets, err := s.source.BoardTickets(ctx, p); err == nil {
		existing = make([]core.Ticket, len(tickets))
		for i, t := range tickets {
			existing[i] = t.Ticket
		}
	}

	proposed, err := s.planner.Propose(ctx, body.Goal, existing)
	switch {
	case errors.Is(err, plan.ErrNoAPIKey):
		writeError(w, http.StatusNotImplemented,
			"planning is not configured on this server (set ANTHROPIC_API_KEY)")
		return
	case errors.Is(err, plan.ErrInvalidPlan):
		// The model answered, but with a graph we refuse to write — a cycle or
		// a dangling dependency. 422 rather than 502: nothing upstream failed.
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	case err != nil:
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, proposed)
}

// handleApplyPlan implements POST /api/projects/{id}/plan/apply: write the
// approved plan's tickets into the project's docs/tickets/.
//
// This is the only endpoint that writes to a project's repository, which is why
// it is separate from proposing: a model that could reach this directly would
// be able to redirect every agent the scheduler subsequently dispatches.
func (s *Server) handleApplyPlan(w http.ResponseWriter, r *http.Request) {
	p, ok := s.projectOr404(w, r)
	if !ok {
		return
	}
	var body ApplyPlanRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	applied, err := plan.Apply(p.RepoPath, body.Plan)
	switch {
	case errors.Is(err, plan.ErrInvalidPlan):
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// The board just changed shape on disk; tell every open screen to refetch
	// rather than waiting for the refresher's next tick.
	s.events.Publish(EventBoardChanged, map[string]any{"project_id": p.ID})

	writeJSON(w, http.StatusOK, applied)
}
