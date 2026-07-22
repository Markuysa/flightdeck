package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/Markuysa/flightdeck/internal/core"
)

// ProjectRegistry is what the projects handlers need to CRUD registered
// projects: registry.Store satisfies it structurally, with no wrapper
// needed. Handler tests fake it in-memory, so no test opens a SQLite file.
type ProjectRegistry interface {
	Add(ctx context.Context, p core.Project) error
	List(ctx context.Context) ([]core.Project, error)
	Get(ctx context.Context, id string) (core.Project, error)
	Remove(ctx context.Context, id string) error
}

// Config wires a Server's dependencies. Token is FLIGHTDECK_TOKEN's value —
// required, compared constant-time against every bearer/session attempt.
type Config struct {
	Token      string
	Registry   ProjectRegistry
	Source     ProjectSource
	Dispatcher DispatcherFactory
	// Events is optional; a fresh Broker is used when nil. Callers that need
	// to Publish board.changed/ci.changed from outside a request (e.g. a
	// future background refresh loop) pass their own and keep a reference.
	Events *Broker
}

// Server serves the frozen REST + SSE contract described in
// docs/ARCHITECTURE.md. Construct one with NewServer and mount Handler().
type Server struct {
	router     chi.Router
	registry   ProjectRegistry
	source     ProjectSource
	dispatcher DispatcherFactory
	events     *Broker
	token      string
	sessions   *sessionStore
}

// NewServer builds a Server from cfg and mounts every route in
// docs/ARCHITECTURE.md's contract table.
func NewServer(cfg Config) *Server {
	s := &Server{
		registry:   cfg.Registry,
		source:     cfg.Source,
		dispatcher: cfg.Dispatcher,
		token:      cfg.Token,
		sessions:   newSessionStore(),
		events:     cfg.Events,
	}
	if s.events == nil {
		s.events = NewBroker()
	}
	s.router = s.buildRouter()
	return s
}

// Handler returns the server's http.Handler, ready to mount.
func (s *Server) Handler() http.Handler { return s.router }

// Events returns the server's event broker so a caller (internal/app, or a
// future background refresh loop) can Publish board.changed/ci.changed
// from outside a request. handleDispatch and handleApprove already publish
// dispatch.started and board.changed from inside their own requests.
func (s *Server) Events() *Broker { return s.events }

func (s *Server) buildRouter() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)

	r.Post("/api/session", s.handleCreateSession)

	r.Group(func(r chi.Router) {
		r.Use(s.requireAuth)
		r.Get("/api/projects", s.handleListProjects)
		r.Post("/api/projects", s.handleCreateProject)
		r.Delete("/api/projects/{id}", s.handleDeleteProject)
		r.Get("/api/projects/{id}/board", s.handleGetBoard)
		r.Get("/api/projects/{id}/tickets/{tid}", s.handleGetTicket)
		r.Post("/api/projects/{id}/dispatch", s.handleDispatch)
		r.Get("/api/projects/{id}/autopilot", s.handleGetAutopilot)
		r.Put("/api/projects/{id}/autopilot", s.handleSetAutopilot)
		r.Post("/api/projects/{id}/tickets/{tid}/approve", s.handleApprove)
		r.Get("/api/agents", s.handleListAgents)
		r.Get("/api/events", s.handleEvents)
	})

	return r
}

// writeJSON writes v as status's JSON body.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON {"error": msg} body. Callers pass either a fixed
// message or an error string already proven token-free (dispatch/merge
// failures — see dispatch.go's doc comments and secrets_test.go) — never an
// unexamined error that might carry a token or other internal detail.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorBody{Error: msg})
}

// projectOr404 loads the {id} path param's project, writing a 404/500 and
// returning ok=false when it cannot be loaded.
func (s *Server) projectOr404(w http.ResponseWriter, r *http.Request) (core.Project, bool) {
	id := chi.URLParam(r, "id")
	p, err := s.registry.Get(r.Context(), id)
	switch {
	case errors.Is(err, core.ErrProjectNotFound):
		writeError(w, http.StatusNotFound, "project not found")
		return core.Project{}, false
	case err != nil:
		writeError(w, http.StatusInternalServerError, "failed to load project")
		return core.Project{}, false
	}
	return p, true
}

// ticketIDOr400 parses the {tid} path param, writing a 400 and returning
// ok=false when it is not an integer.
func ticketIDOr400(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := strconv.Atoi(chi.URLParam(r, "tid"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid ticket id")
		return 0, false
	}
	return id, true
}

var slugPattern = regexp.MustCompile(`[^a-z0-9]+`)

// slugify derives a stable, URL-safe project ID from its display name
// (core.Project.ID's doc comment: "stable slug"). POST /api/projects has no
// id field in its frozen request shape (ui/src/lib/types.ts's
// CreateProjectRequest), so the server generates one from the name.
func slugify(name string) string {
	slug := slugPattern.ReplaceAllString(strings.ToLower(name), "-")
	return strings.Trim(slug, "-")
}
