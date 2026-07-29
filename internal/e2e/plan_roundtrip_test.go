package e2e

import (
	"context"
	"testing"

	"github.com/Markuysa/flightdeck/internal/derive"
	"github.com/Markuysa/flightdeck/internal/plan"
	"github.com/Markuysa/flightdeck/internal/source/git"
)

// TestPlannedTicketsParseAndDerive closes the loop between the two halves of
// the product that were built separately: what the planner writes must be
// exactly what the ticket reader parses and the derive engine can act on.
//
// This is the seam most likely to rot. internal/plan renders frontmatter by
// hand and internal/source/git parses it by hand; neither imports the other
// (ADR-004), so nothing but this test notices if one side changes its idea of
// the format. And the failure is not local: a ticket file that does not parse
// fails the WHOLE board read, so a bad plan would take down the board of the
// project it was written into.
func TestPlannedTicketsParseAndDerive(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	ctx := context.Background()

	proposed := plan.Plan{Tickets: []plan.Ticket{
		{
			Number: 1, Title: "Auth scaffold", Role: "backend",
			Body:       "Add session middleware.",
			Acceptance: []string{"go test ./... passes"},
			Handoff:    "Auth middleware is `RequireSession`.",
		},
		{
			Number: 2, Title: "Invoice API", Role: "backend", Depends: []int{1},
			Body:       "CRUD endpoints behind the new auth middleware.",
			Acceptance: []string{"POST /invoices returns 201"},
		},
		{
			Number: 3, Title: "Billing dashboard", Role: "frontend", Depends: []int{2},
			Body: "Render the invoice list.",
		},
	}}

	applied, err := plan.Apply(repo, proposed)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(applied.IDs) != 3 {
		t.Fatalf("applied %d tickets, want 3", len(applied.IDs))
	}

	// The real reader, not a fixture: if the rendered frontmatter is malformed
	// in any way, this errors and names the offending file.
	metas, err := git.NewRepo(repo).TicketsWithStatus(ctx)
	if err != nil {
		t.Fatalf("the ticket reader could not parse what the planner wrote: %v", err)
	}
	if len(metas) != 3 {
		t.Fatalf("parsed %d tickets, want 3", len(metas))
	}

	tickets := make([]struct {
		id      int
		role    string
		depends []int
		handoff string
	}, len(metas))
	mainStatus := make(map[int]string, len(metas))
	for i, m := range metas {
		tickets[i] = struct {
			id      int
			role    string
			depends []int
			handoff string
		}{m.ID, m.Role, m.Depends, m.Handoff}
		mainStatus[m.ID] = m.RawStatus
		if m.RawStatus != "todo" {
			t.Errorf("ticket %d parsed with status %q, want todo — a new ticket is never anything else", m.ID, m.RawStatus)
		}
	}

	if tickets[0].role != "backend" || tickets[2].role != "frontend" {
		t.Errorf("roles round-tripped as %q/%q, want backend/frontend", tickets[0].role, tickets[2].role)
	}
	if tickets[0].handoff == "" {
		t.Error("the ## Handoff section did not round-trip — dependent tickets lose their upstream context")
	}

	// Dependencies must survive as real ids, so the derive engine blocks the
	// right tickets. This is the payoff: the plan's local numbering is gone and
	// the graph still holds.
	if len(tickets[1].depends) != 1 || tickets[1].depends[0] != tickets[0].id {
		t.Errorf("ticket 2 depends = %v, want [%d]", tickets[1].depends, tickets[0].id)
	}

	core, err := git.NewRepo(repo).Tickets(ctx)
	if err != nil {
		t.Fatalf("Tickets: %v", err)
	}
	board := derive.Derive(core, mainStatus, nil, nil)
	if len(board) != 3 {
		t.Fatalf("derived %d tickets, want 3", len(board))
	}

	// Exactly the shape the scheduler needs on a fresh plan: the foundation is
	// startable, everything downstream waits.
	if got := board[0].Status; got != "ready" {
		t.Errorf("first ticket derived %q, want ready — nothing blocks it", got)
	}
	for _, bt := range board[1:] {
		if bt.Status != "blocked" {
			t.Errorf("ticket %d derived %q, want blocked (its dependency has not merged)", bt.ID, bt.Status)
		}
	}
}
