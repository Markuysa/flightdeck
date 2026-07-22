package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Markuysa/flightdeck/internal/core"
	"github.com/Markuysa/flightdeck/internal/dispatch"
)

// DispatcherFactory builds the core.Dispatcher for one project. The real
// implementation (gitHubDispatcherFactory) sources routine and GitHub
// tokens from the registry per call; handler tests fake this interface
// with an in-memory core.Dispatcher, so no test fires a real routine or
// merges a real PR.
type DispatcherFactory interface {
	Dispatcher(ctx context.Context, p core.Project) (core.Dispatcher, error)
}

type gitHubDispatcherFactory struct {
	secrets SecretsReader
}

// NewDispatcherFactory returns the real DispatcherFactory, sourcing each
// project's routine and GitHub tokens from secrets.
func NewDispatcherFactory(secrets SecretsReader) DispatcherFactory {
	return &gitHubDispatcherFactory{secrets: secrets}
}

func (f *gitHubDispatcherFactory) Dispatcher(ctx context.Context, p core.Project) (core.Dispatcher, error) {
	sec, err := f.secrets.Secrets(ctx, p.ID)
	if err != nil {
		return nil, fmt.Errorf("reading secrets for project %q: %w", p.ID, err)
	}
	return dispatch.New(sec.RoutineToken, sec.GitHubToken), nil
}

// findTicket returns the BoardTicket with id from tickets, if present.
func findTicket(tickets []core.BoardTicket, id int) (core.BoardTicket, bool) {
	for _, t := range tickets {
		if t.ID == id {
			return t, true
		}
	}
	return core.BoardTicket{}, false
}

// handleDispatch implements POST /api/projects/{id}/dispatch (US-5): fires
// the target ticket's routine, but ONLY when its derived status — computed
// fresh for this request, never trusted from a stored value — is "ready".
// A ticket that is not ready is rejected with 409 before Fire is ever
// called.
func (s *Server) handleDispatch(w http.ResponseWriter, r *http.Request) {
	p, ok := s.projectOr404(w, r)
	if !ok {
		return
	}
	var body DispatchRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ctx := r.Context()
	tickets, err := s.source.BoardTickets(ctx, p)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to compute project board")
		return
	}
	target, ok := findTicket(tickets, body.TicketID)
	if !ok {
		writeError(w, http.StatusNotFound, "ticket not found")
		return
	}
	if target.Status != core.StatusReady {
		writeError(w, http.StatusConflict, fmt.Sprintf("ticket %d is not ready (status: %s)", target.ID, target.Status))
		return
	}

	d, err := s.dispatcher.Dispatcher(ctx, p)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to prepare dispatch")
		return
	}
	sessionURL, err := d.Fire(ctx, p, target.ID)
	if err != nil {
		// ARCHITECTURE.md's failure policy for the routine /fire endpoint:
		// "dispatch failure surfaces to the user with the error; never
		// silently retried". dispatch.ErrDispatchFailed never embeds a
		// token — only the Authorization header ever carries one, and
		// internal/dispatch's own tests assert its errors never do — so
		// relaying it here is safe (see secrets_test.go).
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	s.events.Publish(EventDispatchStarted, map[string]any{
		"project_id":  p.ID,
		"ticket_id":   target.ID,
		"session_url": sessionURL,
	})
	s.events.Publish(EventBoardChanged, map[string]any{"project_id": p.ID})

	writeJSON(w, http.StatusOK, DispatchResponse{SessionURL: sessionURL})
}

// handleGetAutopilot implements GET /api/projects/{id}/autopilot.
func (s *Server) handleGetAutopilot(w http.ResponseWriter, r *http.Request) {
	p, ok := s.projectOr404(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	d, err := s.dispatcher.Dispatcher(ctx, p)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read autopilot state")
		return
	}
	on, err := d.Autopilot(ctx, p)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read autopilot state")
		return
	}
	writeJSON(w, http.StatusOK, AutopilotState{On: on})
}

// handleSetAutopilot implements PUT /api/projects/{id}/autopilot.
func (s *Server) handleSetAutopilot(w http.ResponseWriter, r *http.Request) {
	p, ok := s.projectOr404(w, r)
	if !ok {
		return
	}
	var body AutopilotState
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	ctx := r.Context()
	d, err := s.dispatcher.Dispatcher(ctx, p)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to set autopilot state")
		return
	}
	if err := d.SetAutopilot(ctx, p, body.On); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to set autopilot state")
		return
	}
	writeJSON(w, http.StatusOK, AutopilotState{On: body.On})
}

// handleApprove implements POST /api/projects/{id}/tickets/{tid}/approve
// (US-6): merges ONLY the named ticket's pull request, and only because a
// human called this endpoint. Dispatch never merges (handleDispatch never
// calls ApproveMerge), and this never merges any PR but the one attached to
// tid's current derived state.
func (s *Server) handleApprove(w http.ResponseWriter, r *http.Request) {
	p, ok := s.projectOr404(w, r)
	if !ok {
		return
	}
	tid, ok := ticketIDOr400(w, r)
	if !ok {
		return
	}

	ctx := r.Context()
	tickets, err := s.source.BoardTickets(ctx, p)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to compute project board")
		return
	}
	target, ok := findTicket(tickets, tid)
	if !ok {
		writeError(w, http.StatusNotFound, "ticket not found")
		return
	}
	if target.PR == nil {
		writeError(w, http.StatusConflict, fmt.Sprintf("ticket %d has no open pull request to approve", tid))
		return
	}

	d, err := s.dispatcher.Dispatcher(ctx, p)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to prepare merge")
		return
	}
	if err := d.ApproveMerge(ctx, p, target.PR.Number); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	s.events.Publish(EventBoardChanged, map[string]any{"project_id": p.ID})
	w.WriteHeader(http.StatusNoContent)
}
