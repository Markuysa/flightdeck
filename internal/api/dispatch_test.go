package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/Markuysa/flightdeck/internal/core"
)

func TestDispatchRejectsNonReadyTicketWith409(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	h := ts.srv.Handler()
	must(t, ts.registry.Add(context.Background(), core.Project{ID: "acme", Name: "Acme", RepoPath: "/repos/acme"}))
	ts.source.setBoard("acme", []core.BoardTicket{
		{Ticket: core.Ticket{ID: 1}, Status: core.StatusBlocked},
	})

	rec := doRequest(t, h, http.MethodPost, "/api/projects/acme/dispatch", DispatchRequest{TicketID: 1}, ts.token)
	if rec.Code != http.StatusConflict {
		t.Fatalf("dispatch of a blocked ticket = %d, want 409", rec.Code)
	}

	fake := ts.dispatcher.forProject("acme")
	if len(fake.firedTicketIDs) != 0 {
		t.Errorf("Fire was called %v times for a non-ready ticket, want zero", fake.firedTicketIDs)
	}
}

func TestDispatchFiresReadyTicketAndReturnsSessionURL(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	h := ts.srv.Handler()
	must(t, ts.registry.Add(context.Background(), core.Project{ID: "acme", Name: "Acme", RepoPath: "/repos/acme"}))
	ts.source.setBoard("acme", []core.BoardTicket{
		{Ticket: core.Ticket{ID: 1}, Status: core.StatusReady},
	})
	ts.dispatcher.forProject("acme").sessionURL = "https://routines.example.com/sessions/abc123"

	rec := doRequest(t, h, http.MethodPost, "/api/projects/acme/dispatch", DispatchRequest{TicketID: 1}, ts.token)
	if rec.Code != http.StatusOK {
		t.Fatalf("dispatch of a ready ticket = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	got := decodeJSON[DispatchResponse](t, rec)
	if got.SessionURL != "https://routines.example.com/sessions/abc123" {
		t.Errorf("session_url = %q, want the routine's session URL", got.SessionURL)
	}

	fake := ts.dispatcher.forProject("acme")
	if len(fake.firedTicketIDs) != 1 || fake.firedTicketIDs[0] != 1 {
		t.Errorf("firedTicketIDs = %v, want exactly [1]", fake.firedTicketIDs)
	}
}

func TestDispatchFireFailureSurfacesErrorAsBadGateway(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	h := ts.srv.Handler()
	must(t, ts.registry.Add(context.Background(), core.Project{ID: "acme", Name: "Acme", RepoPath: "/repos/acme"}))
	ts.source.setBoard("acme", []core.BoardTicket{
		{Ticket: core.Ticket{ID: 1}, Status: core.StatusReady},
	})
	ts.dispatcher.forProject("acme").fireErr = &testError{"routine unreachable"}

	rec := doRequest(t, h, http.MethodPost, "/api/projects/acme/dispatch", DispatchRequest{TicketID: 1}, ts.token)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("dispatch when Fire fails = %d, want 502", rec.Code)
	}
}

func TestDispatchUnknownTicketReturns404(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	h := ts.srv.Handler()
	must(t, ts.registry.Add(context.Background(), core.Project{ID: "acme", Name: "Acme", RepoPath: "/repos/acme"}))
	ts.source.setBoard("acme", []core.BoardTicket{})

	rec := doRequest(t, h, http.MethodPost, "/api/projects/acme/dispatch", DispatchRequest{TicketID: 42}, ts.token)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("dispatch of an unknown ticket = %d, want 404", rec.Code)
	}
}

// TestApproveMergesOnlyTheNamedTicketsPR is the acceptance criterion: with
// two in_review tickets, each attached to a different PR, approving one
// ticket must call ApproveMerge with only that ticket's PR number — never
// the other's.
func TestApproveMergesOnlyTheNamedTicketsPR(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	h := ts.srv.Handler()
	must(t, ts.registry.Add(context.Background(), core.Project{ID: "acme", Name: "Acme", RepoPath: "/repos/acme"}))
	ts.source.setBoard("acme", []core.BoardTicket{
		{
			Ticket: core.Ticket{ID: 1, Title: "First in review"},
			Status: core.StatusInReview,
			Branch: "claude/001-first",
			PR:     &core.PRState{Number: 11, URL: "https://github.com/acme/widgets/pull/11", CI: "green"},
		},
		{
			Ticket: core.Ticket{ID: 2, Title: "Second in review"},
			Status: core.StatusInReview,
			Branch: "claude/002-second",
			PR:     &core.PRState{Number: 22, URL: "https://github.com/acme/widgets/pull/22", CI: "green"},
		},
	})

	rec := doRequest(t, h, http.MethodPost, "/api/projects/acme/tickets/1/approve", nil, ts.token)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("approve ticket 1 = %d, want 204: %s", rec.Code, rec.Body.String())
	}

	fake := ts.dispatcher.forProject("acme")
	if len(fake.approvedPRs) != 1 || fake.approvedPRs[0] != 11 {
		t.Fatalf("approvedPRs = %v, want exactly [11] (never 22)", fake.approvedPRs)
	}
}

func TestApproveTicketWithNoPRReturns409(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	h := ts.srv.Handler()
	must(t, ts.registry.Add(context.Background(), core.Project{ID: "acme", Name: "Acme", RepoPath: "/repos/acme"}))
	ts.source.setBoard("acme", []core.BoardTicket{
		{Ticket: core.Ticket{ID: 1}, Status: core.StatusInProgress, Branch: "claude/001-x"},
	})

	rec := doRequest(t, h, http.MethodPost, "/api/projects/acme/tickets/1/approve", nil, ts.token)
	if rec.Code != http.StatusConflict {
		t.Fatalf("approve a ticket with no PR = %d, want 409", rec.Code)
	}

	fake := ts.dispatcher.forProject("acme")
	if len(fake.approvedPRs) != 0 {
		t.Errorf("approvedPRs = %v, want none called", fake.approvedPRs)
	}
}

// TestDispatchNeverCallsApproveMerge and TestApproveNeverCallsFire together
// assert dispatch and merge stay separate code paths — the same invariant
// internal/dispatch's own tests enforce, now proven at the HTTP layer too.
func TestDispatchNeverCallsApproveMerge(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	h := ts.srv.Handler()
	must(t, ts.registry.Add(context.Background(), core.Project{ID: "acme", Name: "Acme", RepoPath: "/repos/acme"}))
	ts.source.setBoard("acme", []core.BoardTicket{
		{Ticket: core.Ticket{ID: 1}, Status: core.StatusReady},
	})

	doRequest(t, h, http.MethodPost, "/api/projects/acme/dispatch", DispatchRequest{TicketID: 1}, ts.token)

	fake := ts.dispatcher.forProject("acme")
	if len(fake.approvedPRs) != 0 {
		t.Errorf("dispatch called ApproveMerge with %v, want never", fake.approvedPRs)
	}
}

func TestApproveNeverCallsFire(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	h := ts.srv.Handler()
	must(t, ts.registry.Add(context.Background(), core.Project{ID: "acme", Name: "Acme", RepoPath: "/repos/acme"}))
	ts.source.setBoard("acme", []core.BoardTicket{
		{
			Ticket: core.Ticket{ID: 1}, Status: core.StatusInReview, Branch: "claude/001-x",
			PR: &core.PRState{Number: 11, CI: "green"},
		},
	})

	doRequest(t, h, http.MethodPost, "/api/projects/acme/tickets/1/approve", nil, ts.token)

	fake := ts.dispatcher.forProject("acme")
	if len(fake.firedTicketIDs) != 0 {
		t.Errorf("approve called Fire with %v, want never", fake.firedTicketIDs)
	}
}

func TestGetAndSetAutopilot(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	h := ts.srv.Handler()
	must(t, ts.registry.Add(context.Background(), core.Project{ID: "acme", Name: "Acme", RepoPath: "/repos/acme"}))

	getRec := doRequest(t, h, http.MethodGet, "/api/projects/acme/autopilot", nil, ts.token)
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET autopilot = %d, want 200", getRec.Code)
	}
	if got := decodeJSON[AutopilotState](t, getRec); got.On {
		t.Errorf("initial autopilot state = %+v, want off", got)
	}

	putRec := doRequest(t, h, http.MethodPut, "/api/projects/acme/autopilot", AutopilotState{On: true}, ts.token)
	if putRec.Code != http.StatusOK {
		t.Fatalf("PUT autopilot = %d, want 200: %s", putRec.Code, putRec.Body.String())
	}
	if got := decodeJSON[AutopilotState](t, putRec); !got.On {
		t.Errorf("PUT autopilot response = %+v, want on", got)
	}

	getAfterRec := doRequest(t, h, http.MethodGet, "/api/projects/acme/autopilot", nil, ts.token)
	if got := decodeJSON[AutopilotState](t, getAfterRec); !got.On {
		t.Errorf("autopilot state after PUT = %+v, want on", got)
	}
}
