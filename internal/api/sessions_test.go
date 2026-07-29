package api

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Markuysa/flightdeck/internal/core"
)

func readyProject(t *testing.T, ts *testServer, ticketID int) {
	t.Helper()
	must(t, ts.registry.Add(context.Background(), core.Project{ID: "acme", Name: "Acme", RepoPath: "/repos/acme"}))
	ts.source.setBoard("acme", []core.BoardTicket{
		{Ticket: core.Ticket{ID: ticketID}, Status: core.StatusReady},
	})
}

// TestHumanDispatchRecordsARun is the reason dispatch writes to the run store
// at all. Firing does not change the board — the ticket stays `ready` until
// its branch appears — so a human dispatch that left no trace would be
// invisible to the background scheduler, which would fire the very same ticket
// again on its next tick.
func TestHumanDispatchRecordsARun(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	readyProject(t, ts, 1)
	ts.dispatcher.forProject("acme").sessionURL = "https://claude.ai/code/s/abc"

	rec := doRequest(t, ts.srv.Handler(), http.MethodPost, "/api/projects/acme/dispatch", DispatchRequest{TicketID: 1}, ts.token)
	if rec.Code != http.StatusOK {
		t.Fatalf("dispatch = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	runs := ts.runs.all()
	if len(runs) != 1 {
		t.Fatalf("recorded %d runs, want 1: %+v", len(runs), runs)
	}
	if runs[0].TicketID != 1 || runs[0].Attempt != 1 {
		t.Errorf("run = ticket %d attempt %d, want ticket 1 attempt 1", runs[0].TicketID, runs[0].Attempt)
	}
	if runs[0].SessionURL != "https://claude.ai/code/s/abc" {
		t.Errorf("run session URL = %q, want the one Fire returned", runs[0].SessionURL)
	}
}

// TestFailedDispatchRecordsNoRun: a run for work that never started would
// block the scheduler from ever retrying the ticket.
func TestFailedDispatchRecordsNoRun(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	readyProject(t, ts, 1)
	ts.dispatcher.forProject("acme").fireErr = errFake

	rec := doRequest(t, ts.srv.Handler(), http.MethodPost, "/api/projects/acme/dispatch", DispatchRequest{TicketID: 1}, ts.token)
	if rec.Code == http.StatusOK {
		t.Fatalf("dispatch of a failing routine = 200, want an error status")
	}
	if runs := ts.runs.all(); len(runs) != 0 {
		t.Errorf("recorded %+v for a failed dispatch, want nothing", runs)
	}
}

// TestDispatchSucceedsEvenIfTheRunCannotBeRecorded: the routine really is
// running at that point. Reporting the dispatch as failed would be the bigger
// lie, so the API returns the session URL and logs the bookkeeping failure.
func TestDispatchSucceedsEvenIfTheRunCannotBeRecorded(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	readyProject(t, ts, 1)
	ts.runs.startErr = errFake
	ts.dispatcher.forProject("acme").sessionURL = "https://claude.ai/code/s/abc"

	rec := doRequest(t, ts.srv.Handler(), http.MethodPost, "/api/projects/acme/dispatch", DispatchRequest{TicketID: 1}, ts.token)
	if rec.Code != http.StatusOK {
		t.Fatalf("dispatch = %d, want 200 despite the run store failing: %s", rec.Code, rec.Body.String())
	}
	if got := decodeJSON[DispatchResponse](t, rec); got.SessionURL != "https://claude.ai/code/s/abc" {
		t.Errorf("session URL = %q, want it returned anyway", got.SessionURL)
	}
}

// TestRepeatedDispatchNumbersAttempts so retry limits mean something across
// both dispatch paths.
func TestRepeatedDispatchNumbersAttempts(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	readyProject(t, ts, 1)

	for range 3 {
		rec := doRequest(t, ts.srv.Handler(), http.MethodPost, "/api/projects/acme/dispatch", DispatchRequest{TicketID: 1}, ts.token)
		if rec.Code != http.StatusOK {
			t.Fatalf("dispatch = %d, want 200: %s", rec.Code, rec.Body.String())
		}
	}

	runs := ts.runs.all()
	if len(runs) != 3 {
		t.Fatalf("recorded %d runs, want 3", len(runs))
	}
	for i, want := range []int{1, 2, 3} {
		if runs[i].Attempt != want {
			t.Errorf("run %d attempt = %d, want %d", i, runs[i].Attempt, want)
		}
	}
}

// TestAgentsUseTheRecordedRun: the session link now comes from the durable run
// store, which is what lets it survive a restart — the in-memory map this
// replaced forgot every link when the process died.
func TestAgentsUseTheRecordedRun(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	must(t, ts.registry.Add(context.Background(), core.Project{ID: "acme", Name: "Acme", RepoPath: "/repos/acme"}))
	ts.source.setBoard("acme", []core.BoardTicket{
		{Ticket: core.Ticket{ID: 1, Title: "Work"}, Status: core.StatusInProgress, Branch: "claude/001-work"},
	})

	dispatchedAt := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	if _, err := ts.runs.StartRun(context.Background(), "acme", 1, 1, "https://claude.ai/code/s/xyz", dispatchedAt); err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	rec := doRequest(t, ts.srv.Handler(), http.MethodGet, "/api/agents", nil, ts.token)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/agents = %d, want 200", rec.Code)
	}
	agents := decodeJSON[[]AgentSession](t, rec)
	if len(agents) != 1 {
		t.Fatalf("got %d agents, want 1", len(agents))
	}
	if agents[0].SessionURL != "https://claude.ai/code/s/xyz" {
		t.Errorf("session_url = %q, want the recorded run's", agents[0].SessionURL)
	}
	if agents[0].StartedAt != dispatchedAt.Format(time.RFC3339) {
		t.Errorf("started_at = %q, want the run's start time %q", agents[0].StartedAt, dispatchedAt.Format(time.RFC3339))
	}
}

// TestAgentsWithoutARunHaveNoSessionURL: an agent whose branch was never
// dispatched through this server has no link, which is honest rather than
// fabricated.
func TestAgentsWithoutARunHaveNoSessionURL(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	must(t, ts.registry.Add(context.Background(), core.Project{ID: "acme", Name: "Acme", RepoPath: "/repos/acme"}))
	ts.source.setBoard("acme", []core.BoardTicket{
		{Ticket: core.Ticket{ID: 1, Title: "Work"}, Status: core.StatusInProgress, Branch: "claude/001-work"},
	})

	rec := doRequest(t, ts.srv.Handler(), http.MethodGet, "/api/agents", nil, ts.token)
	agents := decodeJSON[[]AgentSession](t, rec)
	if len(agents) != 1 {
		t.Fatalf("got %d agents, want 1", len(agents))
	}
	if agents[0].SessionURL != "" {
		t.Errorf("session_url = %q, want empty for a branch this server never dispatched", agents[0].SessionURL)
	}
}
