package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Markuysa/flightdeck/internal/core"
	"github.com/Markuysa/flightdeck/internal/derive"
)

// handleListProjects implements GET /api/projects (US-1): every registered
// project plus its derived per-status counts, autopilot state, and whether
// it currently has a live agent. A project whose board or autopilot state
// cannot be read right now (e.g. an unreadable local checkout) degrades to
// zero counts / autopilot=false rather than failing the whole list —
// ARCHITECTURE.md's failure policy for a missing/unreadable repo.
func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projects, err := s.registry.List(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list projects")
		return
	}

	summaries := make([]ProjectSummary, 0, len(projects))
	for _, p := range projects {
		summaries = append(summaries, s.summarize(ctx, p))
	}
	writeJSON(w, http.StatusOK, summaries)
}

func (s *Server) summarize(ctx context.Context, p core.Project) ProjectSummary {
	summary := ProjectSummary{Project: p, Counts: zeroCounts()}

	if tickets, err := s.source.BoardTickets(ctx, p); err == nil {
		for status, list := range derive.Board(tickets) {
			summary.Counts[status] = len(list)
		}
		for _, t := range tickets {
			if t.Status == core.StatusInProgress {
				summary.HasLiveAgent = true
				break
			}
		}
	}

	if d, err := s.dispatcher.Dispatcher(ctx, p); err == nil {
		if on, err := d.Autopilot(ctx, p); err == nil {
			summary.Autopilot = on
		}
	}

	return summary
}

func zeroCounts() map[core.DerivedStatus]int {
	counts := make(map[core.DerivedStatus]int, len(derive.StatusOrder))
	for _, st := range derive.StatusOrder {
		counts[st] = 0
	}
	return counts
}

// handleCreateProject implements POST /api/projects (US-7): registers a
// project from name/repo_path/optional github, generating its stable slug
// ID server-side — the frozen request body carries no id field.
func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	var body CreateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Name == "" || body.RepoPath == "" {
		writeError(w, http.StatusBadRequest, "name and repo_path are required")
		return
	}

	p := core.Project{
		ID:       slugify(body.Name),
		Name:     body.Name,
		RepoPath: body.RepoPath,
	}
	if body.GitHub != nil {
		p.Remote = "github"
		p.Owner = body.GitHub.Owner
		p.Repo = body.GitHub.Repo
	}
	if p.ID == "" {
		writeError(w, http.StatusBadRequest, "name must contain at least one letter or digit")
		return
	}

	if err := s.registry.Add(r.Context(), p); err != nil {
		writeError(w, http.StatusConflict, "a project with this name is already registered")
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// handleDeleteProject implements DELETE /api/projects/{id}.
func (s *Server) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	err := s.registry.Remove(r.Context(), id)
	switch {
	case errors.Is(err, core.ErrProjectNotFound):
		writeError(w, http.StatusNotFound, "project not found")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "failed to remove project")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
