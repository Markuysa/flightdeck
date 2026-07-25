package e2e

import (
	"net/http"
	"testing"

	"github.com/Markuysa/flightdeck/internal/api"
	"github.com/Markuysa/flightdeck/internal/core"
	"github.com/Markuysa/flightdeck/internal/source/git"
)

// TestLyingReadyStatusOnBlockedTicketRendersBlockedOverHTTP is PRD §6's
// second acceptance criterion and the ADR-001 guarantee, proven through the
// FULL STACK — not just internal/derive's own unit test (see that
// package's derive_test.go for the pure-function version of this same
// proof). A ticket file whose frontmatter literally says `status: ready`,
// but has an unmet dependency, must still render `blocked` over the real
// HTTP board: the engine only ever trusts a stored status for the literals
// `done`/`needs-attention`; ready/in_progress/blocked are always computed.
//
// This uses the REAL ProjectSource (api.NewGitHubSource) against a fixture
// with no GitHub remote — no stubbing is needed to prove a pure git-derived
// rule.
func TestLyingReadyStatusOnBlockedTicketRendersBlockedOverHTTP(t *testing.T) {
	t.Parallel()

	repo := git.NewFixtureRepo(t,
		[]git.FixtureTicket{
			{ID: 1, Title: "Unmet dependency", Role: "backend", Status: "todo"},
			// Ticket 2's file lies: its own frontmatter says "status: ready",
			// but it depends on ticket 1, which is not done on main. If the
			// API ever read this literal instead of computing the status,
			// the board would wrongly show ticket 2 as ready.
			{ID: 2, Title: "Lying ticket", Role: "backend", Depends: []int{1}, Status: "ready"},
		},
		nil,
	)

	store := newTempStore(t)
	h := newHarness(t, store, api.NewGitHubSource(store), newStubDispatcherFactory(newStubDispatcher()))
	project := h.registerProject("Lying Status Fixture", repo.Path, nil)

	resp := h.do(http.MethodGet, "/api/projects/"+project.ID+"/board", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET .../board = %d, want 200: %s", resp.StatusCode, bodyString(t, resp))
	}
	board := decodeJSON[map[core.DerivedStatus][]core.BoardTicket](t, resp)

	for _, bt := range board[core.StatusReady] {
		if bt.ID == 2 {
			t.Fatal("ticket 2 (status: ready lie, unmet dependency) appears in board[ready] over HTTP — the ADR-001 guarantee is broken")
		}
	}
	found := false
	for _, bt := range board[core.StatusBlocked] {
		if bt.ID == 2 {
			found = true
		}
	}
	if !found {
		t.Fatalf("board[blocked] = %+v, want ticket 2 present (its literal status: ready must be ignored)", board[core.StatusBlocked])
	}
}
