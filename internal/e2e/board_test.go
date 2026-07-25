package e2e

import (
	"net/http"
	"testing"

	"github.com/Markuysa/flightdeck/internal/api"
	"github.com/Markuysa/flightdeck/internal/core"
	"github.com/Markuysa/flightdeck/internal/source/git"
)

// TestBoardDerivesEveryStatusOverRealHTTP is PRD §6's first acceptance
// criterion, proven end to end: "Registering a local repo with
// docs/tickets/ makes its tickets appear with correct derived statuses,
// verified against a fixture repo with known git state." It boots the REAL
// api.Server behind a REAL HTTP listener, wires the REAL ProjectSource
// (api.NewGitHubSource — real git.Repo + real derive.Derive, no stubbing)
// against a fixture repo with NO GitHub remote, and drives the full flow the
// UI follows: POST /api/session, POST /api/projects, GET .../board.
//
// The fixture carries a ticket in every one of the six derived statuses,
// including both ways a ticket can end up done — status: done directly on
// main, and a merged branch whose status: done change was folded into main
// by the merge itself — the "done (merged branch / main done)" half of the
// ticket's acceptance criterion.
func TestBoardDerivesEveryStatusOverRealHTTP(t *testing.T) {
	t.Parallel()

	repo := git.NewFixtureRepo(t,
		[]git.FixtureTicket{
			{ID: 1, Title: "Foundation", Role: "backend", Status: "done"},
			{ID: 2, Title: "Ready ticket", Role: "backend", Depends: []int{1}, Status: "todo"},
			{ID: 3, Title: "Blocked ticket", Role: "backend", Depends: []int{2}, Status: "todo"},
			{ID: 4, Title: "In progress ticket", Role: "backend", Depends: []int{1}, Status: "todo"},
			{ID: 5, Title: "In review ticket", Role: "backend", Depends: []int{1}, Status: "todo"},
			{ID: 6, Title: "Needs attention ticket", Role: "backend", Depends: []int{1}, Status: "todo"},
			{ID: 7, Title: "Done via merged branch", Role: "backend", Depends: []int{1}, Status: "todo"},
		},
		[]git.FixtureBranch{
			// Untouched: still todo on the branch -> in_progress.
			{Name: "claude/004-fixture"},
			{
				Name: "claude/005-fixture",
				Tickets: []git.FixtureTicket{
					{ID: 5, Title: "In review ticket", Role: "backend", Depends: []int{1}, Status: "done"},
				},
			},
			{
				Name: "claude/006-fixture",
				Tickets: []git.FixtureTicket{
					{ID: 6, Title: "Needs attention ticket", Role: "backend", Depends: []int{1}, Status: "needs-attention"},
				},
			},
			{
				Name: "claude/007-fixture",
				Tickets: []git.FixtureTicket{
					{ID: 7, Title: "Done via merged branch", Role: "backend", Depends: []int{1}, Status: "done"},
				},
				Merged: true,
			},
		},
	)

	store := newTempStore(t)
	h := newHarness(t, store, api.NewGitHubSource(store), newStubDispatcherFactory(newStubDispatcher()))
	project := h.registerProject("Fixture Project", repo.Path, nil)

	resp := h.do(http.MethodGet, "/api/projects/"+project.ID+"/board", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET .../board = %d, want 200: %s", resp.StatusCode, bodyString(t, resp))
	}
	board := decodeJSON[map[core.DerivedStatus][]core.BoardTicket](t, resp)

	wantStatus := map[int]core.DerivedStatus{
		1: core.StatusDone,
		2: core.StatusReady,
		3: core.StatusBlocked,
		4: core.StatusInProgress,
		5: core.StatusInReview,
		6: core.StatusNeedsAttention,
		7: core.StatusDone,
	}
	byID := map[int]core.BoardTicket{}
	for _, tickets := range board {
		for _, bt := range tickets {
			byID[bt.ID] = bt
		}
	}
	for id, want := range wantStatus {
		got, ok := byID[id]
		if !ok {
			t.Errorf("ticket %d missing from the board", id)
			continue
		}
		if got.Status != want {
			t.Errorf("ticket %d derived status = %q, want %q", id, got.Status, want)
		}
	}
	if byID[7].Branch == "" {
		t.Error("ticket 7 (done via a merged branch) has no Branch recorded, want claude/007-fixture — merging does not delete the branch")
	}
	// No GitHub remote was configured, so no PR/CI annotation is possible —
	// the in-review ticket's PR must be nil here (the separate
	// TestBoardAnnotatesInReviewTicketWithPRAndCIStateOverHTTP proves the
	// annotated case via the stubbed GitHub seam).
	if byID[5].PR != nil {
		t.Errorf("ticket 5's PR = %+v, want nil (no GitHub remote configured, no PRReader ever called)", byID[5].PR)
	}
}

// TestBoardAnnotatesInReviewTicketWithPRAndCIStateOverHTTP is the other half
// of PRD §6's second acceptance criterion: "one with an open PR shows in
// review with its live CI state." It uses the suite's stubbedGitHubSource —
// real git + real derive.Derive, canned PRs standing in for GitHub — to
// prove the API surfaces a PR's number, URL and CI state over real HTTP.
func TestBoardAnnotatesInReviewTicketWithPRAndCIStateOverHTTP(t *testing.T) {
	t.Parallel()

	repo := git.NewFixtureRepo(t,
		[]git.FixtureTicket{
			{ID: 1, Title: "Foundation", Role: "backend", Status: "done"},
			{ID: 2, Title: "In review ticket", Role: "backend", Depends: []int{1}, Status: "todo"},
		},
		[]git.FixtureBranch{
			{
				Name: "claude/002-fixture",
				Tickets: []git.FixtureTicket{
					{ID: 2, Title: "In review ticket", Role: "backend", Depends: []int{1}, Status: "done"},
				},
			},
		},
	)

	source := newStubbedGitHubSource()
	store := newTempStore(t)
	h := newHarness(t, store, source, newStubDispatcherFactory(newStubDispatcher()))
	project := h.registerProject("PR Fixture", repo.Path, nil)

	// The stubbed GitHub seam: canned PR/CI state keyed by branch, standing
	// in for a real core.PRReader.OpenPRs call — no GitHub HTTP happens.
	source.setPRs(project.ID, map[string]core.PRState{
		"claude/002-fixture": {Number: 42, URL: "https://github.com/acme/widgets/pull/42", CI: "green"},
	})

	resp := h.do(http.MethodGet, "/api/projects/"+project.ID+"/board", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET .../board = %d, want 200: %s", resp.StatusCode, bodyString(t, resp))
	}
	board := decodeJSON[map[core.DerivedStatus][]core.BoardTicket](t, resp)

	inReview := board[core.StatusInReview]
	if len(inReview) != 1 || inReview[0].ID != 2 {
		t.Fatalf("board[in_review] = %+v, want exactly ticket 2", inReview)
	}
	ticket := inReview[0]
	if ticket.PR == nil {
		t.Fatal("ticket 2's PR is nil, want the stubbed PR annotation")
	}
	if ticket.PR.Number != 42 {
		t.Errorf("ticket 2's PR.Number = %d, want 42", ticket.PR.Number)
	}
	if ticket.PR.CI != "green" {
		t.Errorf("ticket 2's PR.CI = %q, want %q", ticket.PR.CI, "green")
	}
}
