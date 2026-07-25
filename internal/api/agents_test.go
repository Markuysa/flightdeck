package api

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Markuysa/flightdeck/internal/core"
)

func TestListAgentsSynthesizesFromInProgressTicketsOnly(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	h := ts.srv.Handler()
	must(t, ts.registry.Add(context.Background(), core.Project{ID: "acme", Name: "Acme Web", RepoPath: "/repos/acme"}))
	must(t, ts.registry.Add(context.Background(), core.Project{ID: "beta", Name: "Beta API", RepoPath: "/repos/beta"}))

	ts.source.setBoard("acme", []core.BoardTicket{
		{Ticket: core.Ticket{ID: 1, Title: "Ready"}, Status: core.StatusReady},
		{Ticket: core.Ticket{ID: 2, Title: "Working"}, Status: core.StatusInProgress, Branch: "claude/002-working"},
	})
	ts.source.setBoard("beta", []core.BoardTicket{
		{Ticket: core.Ticket{ID: 5, Title: "Also working"}, Status: core.StatusInProgress, Branch: "claude/005-also-working"},
	})
	commitTime := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	ts.source.setCommitTime("acme", "claude/002-working", commitTime)
	ts.source.setCommitTime("beta", "claude/005-also-working", commitTime)

	rec := doRequest(t, h, http.MethodGet, "/api/agents", nil, ts.token)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/agents = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	agents := decodeJSON[[]AgentSession](t, rec)
	if len(agents) != 2 {
		t.Fatalf("agents = %+v, want exactly 2 (only in_progress tickets)", agents)
	}

	byTicket := make(map[int]AgentSession, len(agents))
	for _, a := range agents {
		byTicket[a.TicketID] = a
	}
	acme, ok := byTicket[2]
	if !ok {
		t.Fatalf("no agent session for ticket 2: %+v", agents)
	}
	if acme.ProjectID != "acme" || acme.ProjectName != "Acme Web" || acme.Branch != "claude/002-working" {
		t.Errorf("agent for ticket 2 = %+v, want project acme/Acme Web on claude/002-working", acme)
	}
	wantTime := commitTime.Format(time.RFC3339)
	if acme.StartedAt != wantTime || acme.LastActivityAt != wantTime {
		t.Errorf("agent for ticket 2 timestamps = %+v, want both %q", acme, wantTime)
	}
	if acme.SessionURL != "" {
		t.Errorf("agent for ticket 2 session_url = %q, want empty: never dispatched through this server", acme.SessionURL)
	}

	if _, ok := byTicket[5]; !ok {
		t.Errorf("no agent session for ticket 5 (beta project): %+v", agents)
	}
	if _, ok := byTicket[1]; ok {
		t.Errorf("ready ticket 1 should not synthesize an agent session: %+v", agents)
	}
}

func TestListAgentsSkipsProjectWithUnreadableBoard(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	h := ts.srv.Handler()
	must(t, ts.registry.Add(context.Background(), core.Project{ID: "broken", Name: "Broken", RepoPath: "/repos/broken"}))
	ts.source.setBoardErr("broken", &testError{"repo unreadable"})

	rec := doRequest(t, h, http.MethodGet, "/api/agents", nil, ts.token)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/agents with one broken project = %d, want 200 (degrade, don't fail)", rec.Code)
	}
	agents := decodeJSON[[]AgentSession](t, rec)
	if len(agents) != 0 {
		t.Errorf("agents = %+v, want empty", agents)
	}
}

// TestListAgentsFillsSessionURLFromDispatch is ticket 019's core acceptance
// criterion end to end: dispatching a ready ticket records its session URL
// and dispatch time, and once the board reflects that the ticket is now
// in_progress (as it would once the routine pushes a claude/NNN-* branch),
// GET /api/agents surfaces exactly that session_url and started_at — never
// fabricated for a ticket this server did not dispatch.
func TestListAgentsFillsSessionURLFromDispatch(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	h := ts.srv.Handler()
	must(t, ts.registry.Add(context.Background(), core.Project{ID: "acme", Name: "Acme", RepoPath: "/repos/acme"}))

	ts.source.setBoard("acme", []core.BoardTicket{
		{Ticket: core.Ticket{ID: 7, Title: "Fix the thing"}, Status: core.StatusReady},
	})
	const wantSessionURL = "https://routines.example.com/sessions/fromdispatch"
	ts.dispatcher.forProject("acme").sessionURL = wantSessionURL

	before := time.Now()
	dispatchRec := doRequest(t, h, http.MethodPost, "/api/projects/acme/dispatch", DispatchRequest{TicketID: 7}, ts.token)
	after := time.Now()
	if dispatchRec.Code != http.StatusOK {
		t.Fatalf("dispatch = %d, want 200: %s", dispatchRec.Code, dispatchRec.Body.String())
	}

	// Simulate the routine having pushed claude/007-fix-the-thing: the board
	// now derives the ticket as in_progress, exactly as a real ProjectSource
	// would once the branch exists with the file still todo.
	ts.source.setBoard("acme", []core.BoardTicket{
		{Ticket: core.Ticket{ID: 7, Title: "Fix the thing"}, Status: core.StatusInProgress, Branch: "claude/007-fix-the-thing"},
	})

	rec := doRequest(t, h, http.MethodGet, "/api/agents", nil, ts.token)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/agents = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	agents := decodeJSON[[]AgentSession](t, rec)
	if len(agents) != 1 {
		t.Fatalf("agents = %+v, want exactly 1", agents)
	}
	agent := agents[0]
	if agent.SessionURL != wantSessionURL {
		t.Errorf("session_url = %q, want %q", agent.SessionURL, wantSessionURL)
	}
	startedAt, err := time.Parse(time.RFC3339, agent.StartedAt)
	if err != nil {
		t.Fatalf("started_at = %q not RFC3339: %v", agent.StartedAt, err)
	}
	if startedAt.Before(before.Add(-time.Second)) || startedAt.After(after.Add(time.Second)) {
		t.Errorf("started_at = %v, want within a second of the dispatch call [%v, %v]", startedAt, before, after)
	}
}

func TestListAgentsNoProjectsReturnsEmptyArrayNotNull(t *testing.T) {
	t.Parallel()
	ts := newTestServer()

	rec := doRequest(t, ts.srv.Handler(), http.MethodGet, "/api/agents", nil, ts.token)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/agents = %d, want 200", rec.Code)
	}
	if got := rec.Body.String(); got != "[]\n" && got != "[]" {
		t.Errorf("body = %q, want an empty JSON array, not null", got)
	}
}
