package git

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTicketsParsesFrontmatterBodyAndHandoff(t *testing.T) {
	t.Parallel()
	repo := NewFixtureRepo(t, []FixtureTicket{
		{
			ID:      1,
			Title:   "Foundation",
			Role:    "backend",
			Depends: nil,
			Status:  "done",
			Body:    "Bootstrap the module.\n\n## Handoff\nThe module now exists.\nEverything else builds on it.",
		},
		{
			ID:      2,
			Title:   "Second ticket",
			Role:    "frontend",
			Depends: []int{1},
			Status:  "todo",
			Body:    "No handoff yet, still in progress.",
		},
	}, nil)

	metas, err := repo.TicketsWithStatus(context.Background())
	if err != nil {
		t.Fatalf("TicketsWithStatus: %v", err)
	}
	if len(metas) != 2 {
		t.Fatalf("got %d tickets, want 2", len(metas))
	}

	first := metas[0]
	if first.ID != 1 || first.Title != "Foundation" || first.Role != "backend" {
		t.Errorf("first ticket = %+v, want id 1 Foundation/backend", first.Ticket)
	}
	if len(first.Depends) != 0 {
		t.Errorf("first.Depends = %v, want empty", first.Depends)
	}
	if first.RawStatus != "done" {
		t.Errorf("first.RawStatus = %q, want %q", first.RawStatus, "done")
	}
	if !strings.Contains(first.Body, "Bootstrap the module.") {
		t.Errorf("first.Body = %q, want it to contain the intro paragraph", first.Body)
	}
	if !strings.Contains(first.Body, "## Handoff") {
		t.Errorf("first.Body = %q, want Body to still include the Handoff section", first.Body)
	}
	wantHandoff := "The module now exists.\nEverything else builds on it."
	if first.Handoff != wantHandoff {
		t.Errorf("first.Handoff = %q, want %q", first.Handoff, wantHandoff)
	}

	second := metas[1]
	if second.ID != 2 {
		t.Errorf("second.ID = %d, want 2", second.ID)
	}
	if len(second.Depends) != 1 || second.Depends[0] != 1 {
		t.Errorf("second.Depends = %v, want [1]", second.Depends)
	}
	if second.RawStatus != "todo" {
		t.Errorf("second.RawStatus = %q, want %q", second.RawStatus, "todo")
	}
	if second.Handoff != "" {
		t.Errorf("second.Handoff = %q, want empty (no ## Handoff section)", second.Handoff)
	}
}

func TestTicketsDependsListVariants(t *testing.T) {
	t.Parallel()
	repo := NewFixtureRepo(t, []FixtureTicket{
		{ID: 1, Title: "A", Role: "backend", Depends: nil, Status: "todo"},
		{ID: 5, Title: "B", Role: "backend", Depends: []int{1}, Status: "todo"},
		{ID: 6, Title: "C", Role: "backend", Depends: []int{5, 1}, Status: "todo"},
		{ID: 7, Title: "D", Role: "backend", Depends: []int{5, 6, 7}, Status: "todo"},
	}, nil)

	metas, err := repo.TicketsWithStatus(context.Background())
	if err != nil {
		t.Fatalf("TicketsWithStatus: %v", err)
	}
	if len(metas) != 4 {
		t.Fatalf("got %d tickets, want 4", len(metas))
	}
	// Ordered by id ascending regardless of filename.
	wantIDs := []int{1, 5, 6, 7}
	for i, want := range wantIDs {
		if metas[i].ID != want {
			t.Errorf("metas[%d].ID = %d, want %d", i, metas[i].ID, want)
		}
	}
}

func TestTicketsSkipsREADME(t *testing.T) {
	t.Parallel()
	repo := NewFixtureRepo(t, []FixtureTicket{{ID: 1, Title: "One", Role: "backend", Status: "todo"}}, nil)

	readmePath := filepath.Join(repo.Path, "docs", "tickets", "README.md")
	if err := os.WriteFile(readmePath, []byte("# The ticket queue\n\nNot a ticket."), 0o644); err != nil {
		t.Fatalf("writing README.md: %v", err)
	}

	metas, err := repo.TicketsWithStatus(context.Background())
	if err != nil {
		t.Fatalf("TicketsWithStatus: %v", err)
	}
	if len(metas) != 1 {
		t.Fatalf("got %d tickets, want 1 (README.md should be skipped)", len(metas))
	}
}

func TestTicketsMalformedFrontmatterReportsError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ticketsDir := filepath.Join(dir, "docs", "tickets")
	if err := os.MkdirAll(ticketsDir, 0o755); err != nil {
		t.Fatalf("creating docs/tickets: %v", err)
	}

	badPath := filepath.Join(ticketsDir, "001-broken.md")
	badContent := "---\nid: not-a-number\ntitle: Broken\nrole: backend\ndepends: []\nstatus: todo\n---\nBody.\n"
	if err := os.WriteFile(badPath, []byte(badContent), 0o644); err != nil {
		t.Fatalf("writing malformed ticket: %v", err)
	}

	repo := &Repo{Path: dir, Main: "main"}
	_, err := repo.TicketsWithStatus(context.Background())
	if err == nil {
		t.Fatal("TicketsWithStatus with malformed frontmatter = nil error, want an error")
	}
	if !strings.Contains(err.Error(), "001-broken.md") {
		t.Errorf("error = %q, want it to name the offending file", err.Error())
	}
}

func TestTicketsMissingFrontmatterFenceReportsError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ticketsDir := filepath.Join(dir, "docs", "tickets")
	if err := os.MkdirAll(ticketsDir, 0o755); err != nil {
		t.Fatalf("creating docs/tickets: %v", err)
	}

	badPath := filepath.Join(ticketsDir, "002-no-fence.md")
	if err := os.WriteFile(badPath, []byte("id: 2\ntitle: No fence\n"), 0o644); err != nil {
		t.Fatalf("writing malformed ticket: %v", err)
	}

	repo := &Repo{Path: dir, Main: "main"}
	_, err := repo.TicketsWithStatus(context.Background())
	if err == nil {
		t.Fatal("TicketsWithStatus with no frontmatter fence = nil error, want an error")
	}
	if !strings.Contains(err.Error(), "002-no-fence.md") {
		t.Errorf("error = %q, want it to name the offending file", err.Error())
	}
}

func TestTicketsImplementsCoreTicketReader(t *testing.T) {
	t.Parallel()
	repo := NewFixtureRepo(t, []FixtureTicket{{ID: 1, Title: "One", Role: "backend", Status: "todo"}}, nil)

	tickets, err := repo.Tickets(context.Background())
	if err != nil {
		t.Fatalf("Tickets: %v", err)
	}
	if len(tickets) != 1 || tickets[0].ID != 1 || tickets[0].Title != "One" {
		t.Errorf("Tickets() = %+v, want a single ticket id 1 titled One", tickets)
	}
}
