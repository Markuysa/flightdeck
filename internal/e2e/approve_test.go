package e2e

import (
	"net/http"
	"testing"

	"github.com/Markuysa/flightdeck/internal/api"
	"github.com/Markuysa/flightdeck/internal/core"
	"github.com/Markuysa/flightdeck/internal/source/git"
)

// TestApproveMergesOnlyTheNamedPROverHTTP is PRD §6's fourth acceptance
// criterion: "Approving a merge merges the PR (GitHub) on human action
// only." With two in_review tickets attached to different PRs, approving
// one calls the stubbed ApproveMerge with only that ticket's PR number —
// never the other's — and a subsequent dispatch of an unrelated ready
// ticket never triggers a merge. Uses the suite's stubbedGitHubSource so
// each in-review ticket can carry its own canned PR number.
func TestApproveMergesOnlyTheNamedPROverHTTP(t *testing.T) {
	t.Parallel()

	repo := git.NewFixtureRepo(t,
		[]git.FixtureTicket{
			{ID: 1, Title: "Foundation", Role: "backend", Status: "done"},
			{ID: 2, Title: "First in review", Role: "backend", Depends: []int{1}, Status: "todo"},
			{ID: 3, Title: "Second in review", Role: "backend", Depends: []int{1}, Status: "todo"},
			{ID: 4, Title: "Ready ticket", Role: "backend", Depends: []int{1}, Status: "todo"},
		},
		[]git.FixtureBranch{
			{
				Name: "claude/002-fixture",
				Tickets: []git.FixtureTicket{
					{ID: 2, Title: "First in review", Role: "backend", Depends: []int{1}, Status: "done"},
				},
			},
			{
				Name: "claude/003-fixture",
				Tickets: []git.FixtureTicket{
					{ID: 3, Title: "Second in review", Role: "backend", Depends: []int{1}, Status: "done"},
				},
			},
		},
	)

	source := newStubbedGitHubSource()
	store := newTempStore(t)
	dispatcher := newStubDispatcher()
	h := newHarness(t, store, source, newStubDispatcherFactory(dispatcher))
	project := h.registerProject("Approve Fixture", repo.Path, nil)

	source.setPRs(project.ID, map[string]core.PRState{
		"claude/002-fixture": {Number: 11, URL: "https://github.com/acme/widgets/pull/11", CI: "green"},
		"claude/003-fixture": {Number: 22, URL: "https://github.com/acme/widgets/pull/22", CI: "green"},
	})

	resp := h.do(http.MethodPost, "/api/projects/"+project.ID+"/tickets/2/approve", nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("approve ticket 2 = %d, want 204: %s", resp.StatusCode, bodyString(t, resp))
	}

	if got := dispatcher.approvedPRNumbers(); len(got) != 1 || got[0] != 11 {
		t.Fatalf("approvedPRNumbers = %v, want exactly [11] (never 22)", got)
	}
	if len(dispatcher.fireCalls) != 0 {
		t.Errorf("approve called Fire %+v times, want zero — approve never dispatches", dispatcher.fireCalls)
	}

	// Dispatching an unrelated ready ticket must never call ApproveMerge —
	// dispatch and merge stay separate code paths.
	dispatchResp := h.do(http.MethodPost, "/api/projects/"+project.ID+"/dispatch", api.DispatchRequest{TicketID: 4})
	if dispatchResp.StatusCode != http.StatusOK {
		t.Fatalf("dispatch of ready ticket 4 = %d, want 200: %s", dispatchResp.StatusCode, bodyString(t, dispatchResp))
	}
	if got := dispatcher.approvedPRNumbers(); len(got) != 1 || got[0] != 11 {
		t.Fatalf("approvedPRNumbers after dispatch = %v, want still exactly [11] — dispatch never merges", got)
	}
}

// TestApproveTicketWithNoPRReturns409OverHTTP guards the boundary the other
// tests assume: a ticket with no PR attached cannot be approved.
func TestApproveTicketWithNoPRReturns409OverHTTP(t *testing.T) {
	t.Parallel()

	repo := git.NewFixtureRepo(t,
		[]git.FixtureTicket{
			{ID: 1, Title: "Foundation", Role: "backend", Status: "done"},
			{ID: 2, Title: "In progress ticket", Role: "backend", Depends: []int{1}, Status: "todo"},
		},
		[]git.FixtureBranch{{Name: "claude/002-fixture"}},
	)

	store := newTempStore(t)
	dispatcher := newStubDispatcher()
	h := newHarness(t, store, api.NewGitHubSource(store), newStubDispatcherFactory(dispatcher))
	project := h.registerProject("No PR Fixture", repo.Path, nil)

	resp := h.do(http.MethodPost, "/api/projects/"+project.ID+"/tickets/2/approve", nil)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("approve a ticket with no PR = %d, want 409: %s", resp.StatusCode, bodyString(t, resp))
	}
	if len(dispatcher.approveCalls) != 0 {
		t.Errorf("approveCalls = %+v, want none called", dispatcher.approveCalls)
	}
}
