package derive

import (
	"context"
	"slices"
	"testing"

	"github.com/Markuysa/flightdeck/internal/core"
	"github.com/Markuysa/flightdeck/internal/source/git"
)

// TestDerive_WithRealGitFixture is the one integration-style test the
// ticket allows: it builds a real temporary git repository via ticket 003's
// git.NewFixtureRepo and reads real ticket/branch state off it with
// TicketsWithStatus and Branches, rather than hand-built maps. Derive itself
// still receives plain Go values and performs no I/O — per-branch
// FileStatus values below are taken directly from the fixture spec this
// test authored (as the ticket permits), not re-read via FileOnBranch, so
// this test does not duplicate internal/source/git's frontmatter parsing.
func TestDerive_WithRealGitFixture(t *testing.T) {
	t.Parallel()

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
			// Identical to main: still "todo" on the branch -> in_progress.
			{Name: "claude/003-in-progress"},
			{
				Name: "claude/004-in-review",
				Tickets: []git.FixtureTicket{
					{ID: 4, Title: "In review ticket", Role: "backend", Depends: []int{1}, Status: "done"},
				},
			},
			{
				Name: "claude/005-needs-attention",
				Tickets: []git.FixtureTicket{
					{ID: 5, Title: "Needs attention ticket", Role: "backend", Depends: []int{1}, Status: "needs-attention"},
				},
			},
		},
	)

	ctx := context.Background()
	metas, err := repo.TicketsWithStatus(ctx)
	if err != nil {
		t.Fatalf("TicketsWithStatus: %v", err)
	}

	tickets := make([]core.Ticket, len(metas))
	mainStatus := make(map[int]string, len(metas))
	for i, m := range metas {
		tickets[i] = m.Ticket
		mainStatus[m.ID] = m.RawStatus
	}

	branchNames, err := repo.Branches(ctx)
	if err != nil {
		t.Fatalf("Branches: %v", err)
	}
	for _, want := range []string{"claude/003-in-progress", "claude/004-in-review", "claude/005-needs-attention"} {
		if !slices.Contains(branchNames, want) {
			t.Fatalf("Branches() = %v, want it to contain %q", branchNames, want)
		}
	}

	// The per-branch FileStatus values here are exactly what this test
	// authored above via FixtureBranch.Tickets (or, for ticket 3, the fact
	// that its branch was left identical to main) — known statically, not
	// re-read through git. Derive receives only these plain values.
	branches := map[int]BranchState{
		3: {Name: "claude/003-in-progress", FileStatus: "todo"},
		4: {Name: "claude/004-in-review", FileStatus: "done"},
		5: {Name: "claude/005-needs-attention", FileStatus: "needs-attention"},
	}

	got := Derive(tickets, mainStatus, branches, nil)

	wantStatus := map[int]core.DerivedStatus{
		1: core.StatusDone,
		2: core.StatusReady,
		3: core.StatusInProgress,
		4: core.StatusInReview,
		5: core.StatusNeedsAttention,
		6: core.StatusBlocked,
	}
	if len(got) != len(wantStatus) {
		t.Fatalf("Derive returned %d BoardTickets, want %d", len(got), len(wantStatus))
	}
	for _, bt := range got {
		want, ok := wantStatus[bt.ID]
		if !ok {
			t.Fatalf("unexpected ticket id %d in output", bt.ID)
		}
		if bt.Status != want {
			t.Errorf("ticket %d (%s): Status = %q, want %q", bt.ID, bt.Title, bt.Status, want)
		}
	}
}
