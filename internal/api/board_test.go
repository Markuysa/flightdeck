package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/Markuysa/flightdeck/internal/core"
	"github.com/Markuysa/flightdeck/internal/source/git"
)

func TestGetBoardGroupsTicketsByStatus(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	h := ts.srv.Handler()

	must(t, ts.registry.Add(context.Background(), core.Project{ID: "acme", Name: "Acme", RepoPath: "/repos/acme"}))
	ts.source.setBoard("acme", []core.BoardTicket{
		{Ticket: core.Ticket{ID: 1, Title: "Ready one"}, Status: core.StatusReady},
		{Ticket: core.Ticket{ID: 2, Title: "Done one"}, Status: core.StatusDone},
	})

	rec := doRequest(t, h, http.MethodGet, "/api/projects/acme/board", nil, ts.token)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET board = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	board := decodeJSON[map[core.DerivedStatus][]core.BoardTicket](t, rec)

	if len(board[core.StatusReady]) != 1 || board[core.StatusReady][0].ID != 1 {
		t.Errorf("board[ready] = %v, want ticket 1", board[core.StatusReady])
	}
	if len(board[core.StatusDone]) != 1 || board[core.StatusDone][0].ID != 2 {
		t.Errorf("board[done] = %v, want ticket 2", board[core.StatusDone])
	}
	// Every one of the six status keys must be present, even when empty,
	// matching ui/src/lib/types.ts's Board = Record<DerivedStatus, ...>.
	for _, status := range []core.DerivedStatus{
		core.StatusReady, core.StatusInProgress, core.StatusInReview,
		core.StatusBlocked, core.StatusNeedsAttention, core.StatusDone,
	} {
		if _, ok := board[status]; !ok {
			t.Errorf("board missing key %q", status)
		}
	}
}

// TestGetBoardIsComputedPerRequestNeverCached proves the board handler
// never reads a cached/stored value: two requests for the same project see
// whatever the source currently reports, even when it changed between
// calls — the "computed per request" acceptance criterion.
func TestGetBoardIsComputedPerRequestNeverCached(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	h := ts.srv.Handler()
	must(t, ts.registry.Add(context.Background(), core.Project{ID: "acme", Name: "Acme", RepoPath: "/repos/acme"}))

	ts.source.setBoard("acme", []core.BoardTicket{
		{Ticket: core.Ticket{ID: 1}, Status: core.StatusReady},
	})
	first := doRequest(t, h, http.MethodGet, "/api/projects/acme/board", nil, ts.token)
	firstBoard := decodeJSON[map[core.DerivedStatus][]core.BoardTicket](t, first)
	if len(firstBoard[core.StatusReady]) != 1 {
		t.Fatalf("first board[ready] = %v, want one ticket", firstBoard[core.StatusReady])
	}

	ts.source.setBoard("acme", []core.BoardTicket{
		{Ticket: core.Ticket{ID: 1}, Status: core.StatusInProgress, Branch: "claude/001-x"},
	})
	second := doRequest(t, h, http.MethodGet, "/api/projects/acme/board", nil, ts.token)
	secondBoard := decodeJSON[map[core.DerivedStatus][]core.BoardTicket](t, second)
	if len(secondBoard[core.StatusReady]) != 0 {
		t.Errorf("second board[ready] = %v, want empty (status moved on)", secondBoard[core.StatusReady])
	}
	if len(secondBoard[core.StatusInProgress]) != 1 {
		t.Errorf("second board[in_progress] = %v, want one ticket", secondBoard[core.StatusInProgress])
	}
}

func TestGetBoardSourceErrorReturns500(t *testing.T) {
	t.Parallel()
	ts := newTestServer()
	h := ts.srv.Handler()
	must(t, ts.registry.Add(context.Background(), core.Project{ID: "acme", Name: "Acme", RepoPath: "/repos/acme"}))
	ts.source.setBoardErr("acme", errUnreadableRepo)

	rec := doRequest(t, h, http.MethodGet, "/api/projects/acme/board", nil, ts.token)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("GET board with an unreadable repo = %d, want 500", rec.Code)
	}
}

var errUnreadableRepo = &testError{"repo unreadable"}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestGitHubSourceBoardTickets_RealGitFixture is the integration test the
// ticket asks for: it exercises the real ProjectSource (gitHubSource)
// against a real temporary git repository built by ticket 003's
// git.NewFixtureRepo, proving the derive-input assembly (tickets, main
// status, per-branch BranchState via FileOnBranch + git.ParseStatus) works
// end to end. The project has no GitHub remote, so GitHub is "faked/nil"
// per the ticket's instruction — openPRs never runs, keeping this fully
// offline.
func TestGitHubSourceBoardTickets_RealGitFixture(t *testing.T) {
	t.Parallel()

	// git.NewFixtureRepo always names a ticket's file "NNN-fixture.md"
	// (fixture.go's fixtureTicketFilename), regardless of Title — so branch
	// names here follow the real docs/tickets/README.md convention of
	// sharing the ticket file's own NNN-slug ("claude/003-fixture" reads
	// "docs/tickets/003-fixture.md"), matching what ticketFileOnBranch
	// derives from the branch name alone.
	repo := git.NewFixtureRepo(t,
		[]git.FixtureTicket{
			{ID: 1, Title: "Foundation", Role: "backend", Status: "done"},
			{ID: 2, Title: "Ready ticket", Role: "backend", Depends: []int{1}, Status: "todo"},
			{ID: 3, Title: "In progress ticket", Role: "backend", Depends: []int{1}, Status: "todo"},
			{ID: 4, Title: "In review ticket", Role: "backend", Depends: []int{1}, Status: "todo"},
			{ID: 5, Title: "Needs attention ticket", Role: "backend", Depends: []int{1}, Status: "todo"},
			{ID: 6, Title: "Blocked ticket", Role: "backend", Depends: []int{99}, Status: "todo"},
		},
		[]git.FixtureBranch{
			{Name: "claude/003-fixture"},
			{
				Name: "claude/004-fixture",
				Tickets: []git.FixtureTicket{
					{ID: 4, Title: "In review ticket", Role: "backend", Depends: []int{1}, Status: "done"},
				},
			},
			{
				Name: "claude/005-fixture",
				Tickets: []git.FixtureTicket{
					{ID: 5, Title: "Needs attention ticket", Role: "backend", Depends: []int{1}, Status: "needs-attention"},
				},
			},
		},
	)

	source := NewGitHubSource(newFakeRegistry())
	project := core.Project{ID: "fixture", Name: "Fixture", RepoPath: repo.Path}

	got, err := source.BoardTickets(context.Background(), project)
	if err != nil {
		t.Fatalf("BoardTickets: %v", err)
	}

	wantStatus := map[int]core.DerivedStatus{
		1: core.StatusDone,
		2: core.StatusReady,
		3: core.StatusInProgress,
		4: core.StatusInReview,
		5: core.StatusNeedsAttention,
		6: core.StatusBlocked,
	}
	byID := make(map[int]core.BoardTicket, len(got))
	for _, bt := range got {
		byID[bt.ID] = bt
	}
	if len(got) != 6 {
		t.Fatalf("BoardTickets returned %d tickets, want 6", len(got))
	}
	for id, want := range wantStatus {
		if byID[id].Status != want {
			t.Errorf("ticket %d status = %q, want %q", id, byID[id].Status, want)
		}
	}
	if byID[3].Branch != "claude/003-fixture" {
		t.Errorf("ticket 3 Branch = %q, want claude/003-fixture", byID[3].Branch)
	}
	if byID[2].Depends == nil {
		t.Error("ticket 2 Depends is nil, want a non-nil (possibly empty) slice so JSON encodes [] not null")
	}
	if byID[4].PR != nil {
		t.Errorf("ticket 4 PR = %+v, want nil (no GitHub remote configured, no PRReader ever called)", byID[4].PR)
	}
}
